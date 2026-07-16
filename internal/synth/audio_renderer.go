package synth

import (
	"fmt"
	"log"
	"math"
	"sync/atomic"
	"time"

	"github.com/wentf9/subitohost/internal/config"
)

type audioDevice interface {
	Close()
}

// AudioRenderer is the process-wide PCM source. Its mutable rendering state is
// owned exclusively by the audio reader goroutine; producers only publish pending
// fully initialized synths through an atomic pointer.
type AudioRenderer struct {
	pending atomic.Pointer[Synth]
	current *Synth
	old     *Synth

	fadeTotal        int
	fadeRemaining    int
	chunkFrames      int
	oldLeft          []float32
	oldRight         []float32
	newLeft          []float32
	newRight         []float32
	sampleRate       int
	callbackCount    atomic.Uint64
	callbackMaxNS    atomic.Uint64
	callbackOverruns atomic.Uint64
}

// AudioOutput owns the only platform audio stream in the process.
type AudioOutput struct {
	renderer         *AudioRenderer
	device           audioDevice
	sampleRate       int
	framesPerPeriod  int
	periods          int
	estimatedLatency time.Duration
}

func normalizedAudioConfig(cfg config.Audio) (sampleRate, frames, periods int) {
	sampleRate = cfg.SampleRate
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	frames = cfg.BufferSize
	if frames <= 0 {
		frames = 128
	}
	periods = cfg.Periods
	if periods <= 0 {
		periods = 2
	}
	return
}

func newAudioRenderer(cfg config.Audio) *AudioRenderer {
	sampleRate, frames, _ := normalizedAudioConfig(cfg)
	fadeFrames := sampleRate * 15 / 1000
	return &AudioRenderer{
		fadeTotal:   max(fadeFrames, 1),
		chunkFrames: frames,
		sampleRate:  sampleRate,
		oldLeft:     make([]float32, frames),
		oldRight:    make([]float32, frames),
		newLeft:     make([]float32, frames),
		newRight:    make([]float32, frames),
	}
}

func newAudioOutput(cfg config.Audio) (*AudioOutput, error) {
	sampleRate, frames, periods := normalizedAudioConfig(cfg)
	deviceBuffer := time.Duration(int64(time.Second) * int64(frames*periods) / int64(sampleRate))
	renderer := newAudioRenderer(cfg)
	device, estimatedLatency, backend, err := openPlatformAudioDevice(renderer, sampleRate, frames, periods, deviceBuffer)
	if err != nil {
		return nil, fmt.Errorf("audio output initialization failed: %w", err)
	}

	output := &AudioOutput{
		renderer:         renderer,
		device:           device,
		sampleRate:       sampleRate,
		framesPerPeriod:  frames,
		periods:          periods,
		estimatedLatency: estimatedLatency,
	}
	log.Printf("audio output: backend=%s sample_rate=%d frames_per_period=%d periods=%d estimated_latency=%s",
		backend, sampleRate, frames, periods, output.estimatedLatency)
	return output, nil
}

// AudioRuntime describes the buffers that are actually configured at runtime.
type AudioRuntime struct {
	SampleRate             int     `json:"sample_rate"`
	FramesPerPeriod        int     `json:"frames_per_period"`
	Periods                int     `json:"periods"`
	EstimatedLatencyMS     float64 `json:"estimated_latency_ms"`
	CallbackCount          uint64  `json:"callback_count"`
	CallbackMaxMS          float64 `json:"callback_max_ms"`
	CallbackOverruns       uint64  `json:"callback_overruns"`
	HardwareXrunsAvailable bool    `json:"hardware_xruns_available"`
}

func runtimeAudioConfig(cfg config.Audio) AudioRuntime {
	sampleRate, frames, periods := normalizedAudioConfig(cfg)
	deviceFrames := frames * periods
	return AudioRuntime{
		SampleRate:         sampleRate,
		FramesPerPeriod:    frames,
		Periods:            periods,
		EstimatedLatencyMS: float64(deviceFrames) * 1000 / float64(sampleRate),
	}
}

func (o *AudioOutput) publish(s *Synth) {
	o.renderer.pending.Store(s)
}

func (o *AudioOutput) Close() {
	if o.device != nil {
		o.device.Close()
	}
}

func (r *AudioRenderer) beginPendingSwap() {
	// Finish the current transition before accepting another one. A newer
	// pending profile may replace an older pending profile atomically, but the
	// currently audible pair is never hard-dropped halfway through a fade.
	if r.old != nil && r.fadeRemaining > 0 {
		return
	}
	next := r.pending.Swap(nil)
	if next == nil || next == r.current {
		return
	}
	if r.current == nil {
		r.current = next
		return
	}
	r.old = r.current
	r.current = next
	r.fadeRemaining = r.fadeTotal
}

// Read implements io.Reader without heap allocation, locking, logging, or IO.
func (r *AudioRenderer) Read(p []byte) (int, error) {
	started := time.Now()
	r.beginPendingSwap()
	frames := len(p) / 8
	writtenFrames := 0
	for writtenFrames < frames {
		count := min(frames-writtenFrames, r.chunkFrames)
		newLeft := r.newLeft[:count]
		newRight := r.newRight[:count]
		if r.current == nil {
			clear(newLeft)
			clear(newRight)
		} else {
			r.current.renderInto(newLeft, newRight)
		}

		if r.old != nil && r.fadeRemaining > 0 {
			oldLeft := r.oldLeft[:count]
			oldRight := r.oldRight[:count]
			r.old.renderInto(oldLeft, oldRight)
			for i := 0; i < count; i++ {
				progress := float64(r.fadeTotal-r.fadeRemaining) / float64(max(r.fadeTotal-1, 1))
				oldGain, newGain := math.Sincos(progress * math.Pi / 2)
				// Sincos returns sin then cos; swap them for old/new gains.
				newLeft[i] = oldLeft[i]*float32(newGain) + newLeft[i]*float32(oldGain)
				newRight[i] = oldRight[i]*float32(newGain) + newRight[i]*float32(oldGain)
				newLeft[i] = finiteHardClip(newLeft[i])
				newRight[i] = finiteHardClip(newRight[i])
				r.fadeRemaining--
				if r.fadeRemaining == 0 {
					r.old = nil
					break
				}
			}
		}

		for i := 0; i < count; i++ {
			offset := (writtenFrames + i) * 8
			leftBits := math.Float32bits(newLeft[i])
			rightBits := math.Float32bits(newRight[i])
			p[offset] = byte(leftBits)
			p[offset+1] = byte(leftBits >> 8)
			p[offset+2] = byte(leftBits >> 16)
			p[offset+3] = byte(leftBits >> 24)
			p[offset+4] = byte(rightBits)
			p[offset+5] = byte(rightBits >> 8)
			p[offset+6] = byte(rightBits >> 16)
			p[offset+7] = byte(rightBits >> 24)
		}
		writtenFrames += count
	}
	elapsed := time.Since(started)
	r.callbackCount.Add(1)
	elapsedNS := uint64(elapsed)
	for maximum := r.callbackMaxNS.Load(); elapsedNS > maximum; maximum = r.callbackMaxNS.Load() {
		if r.callbackMaxNS.CompareAndSwap(maximum, elapsedNS) {
			break
		}
	}
	deadline := time.Duration(int64(time.Second) * int64(max(frames, 1)) / int64(r.sampleRate))
	if elapsed > deadline {
		r.callbackOverruns.Add(1)
	}
	return writtenFrames * 8, nil
}

func finiteHardClip(value float32) float32 {
	if math.IsNaN(float64(value)) {
		return 0
	}
	if value > 1 {
		return 1
	}
	if value < -1 {
		return -1
	}
	return value
}
