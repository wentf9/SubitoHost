// internal/synth/swap.go
package synth

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wentf9/subitohost/internal/config"
	"github.com/wentf9/subitohost/internal/setlist"
)

// SwapManager handles atomic SoundFont swaps.
type SwapManager struct {
	Active   atomic.Pointer[Synth]
	output   atomic.Pointer[AudioOutput]
	cfg      config.Audio
	gainBits atomic.Uint64
	swapMu   sync.Mutex
}

// NewSwapManager creates a swap manager with the given audio config.
func NewSwapManager(cfg config.Audio) *SwapManager {
	m := &SwapManager{cfg: cfg}
	m.gainBits.Store(math.Float64bits(1))
	return m
}

// Init starts the process-wide audio output without replacing a synth that was
// prepared by a command-line or recovery setlist load before Engine.Start.
func (m *SwapManager) Init() error {
	if m.output.Load() != nil {
		return nil
	}
	output, err := newAudioOutput(m.cfg)
	if err != nil {
		return err
	}
	if !m.output.CompareAndSwap(nil, output) {
		output.Close()
		return nil
	}
	if active := m.Active.Load(); active != nil {
		output.publish(active)
	}
	return nil
}

// SwapTo loads a new SoundFont and programs, atomically replacing the active synth.
// On failure, the old synth continues running.
func (m *SwapManager) SwapTo(profile *setlist.Profile) error {
	m.swapMu.Lock()
	defer m.swapMu.Unlock()

	newSynth, err := New(m.cfg)
	if err != nil {
		return err
	}

	if err := newSynth.LoadSoundFont(profile.SFPath); err != nil {
		newSynth.Close()
		return err
	}

	// Inherit current gain on swap
	newSynth.prepareGain(m.Gain())

	for _, p := range profile.Programs {
		newSynth.prepareProgram(p.Channel, p.Bank, p.Program)
	}

	m.Active.Store(newSynth)
	// Close the small race with a concurrent gain change during SoundFont load.
	newSynth.SetGain(m.Gain())
	if output := m.output.Load(); output != nil {
		output.publish(newSynth)
	}
	return nil
}

// Close shuts down the active synth.
func (m *SwapManager) Close() {
	if s := m.Active.Load(); s != nil {
		s.AllNotesOff()
		s.Close()
	}
	if output := m.output.Swap(nil); output != nil {
		output.Close()
	}
}

// SetGain sets master gain on the active synth and stores it for future swaps.
func (m *SwapManager) SetGain(gain float64) {
	m.gainBits.Store(math.Float64bits(gain))
	if s := m.Active.Load(); s != nil {
		s.SetGain(gain)
	}
}

// Gain returns the current master gain.
func (m *SwapManager) Gain() float64 {
	return math.Float64frombits(m.gainBits.Load())
}

// AudioRuntime returns the effective output buffer configuration.
func (m *SwapManager) AudioRuntime() AudioRuntime {
	if output := m.output.Load(); output != nil {
		renderer := output.renderer
		return AudioRuntime{
			SampleRate:             output.sampleRate,
			FramesPerPeriod:        output.framesPerPeriod,
			Periods:                output.periods,
			EstimatedLatencyMS:     float64(output.estimatedLatency) / float64(time.Millisecond),
			CallbackCount:          renderer.callbackCount.Load(),
			CallbackMaxMS:          float64(renderer.callbackMaxNS.Load()) / float64(time.Millisecond),
			CallbackOverruns:       renderer.callbackOverruns.Load(),
			HardwareXrunsAvailable: false,
		}
	}
	return runtimeAudioConfig(m.cfg)
}

// QueueMetrics returns the active synth command queue pressure.
func (m *SwapManager) QueueMetrics() QueueMetrics {
	return m.Active.Load().QueueMetrics()
}
