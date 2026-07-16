// internal/synth/fluidsynth.go
package synth

import (
	"fmt"
	"log"
	"math"
	"os"
	"sync"
	"sync/atomic"

	"github.com/wentf9/subitohost/internal/config"
	"github.com/wentf9/subitohost/internal/ringbuf"
	"github.com/wentf9/subitohost/internal/synth/meltysynth"
)

// synthEvent kinds for the lock-free event queue.
const (
	evNoteOn        uint8 = 0
	evNoteOff       uint8 = 1
	evCC            uint8 = 2
	evProgramChange uint8 = 3
	evAllNotesOff   uint8 = 4
)

// synthEvent is the carrier for MIDI events in the lock-free queue.
// data1/data2 semantics depend on kind:
//
//	NoteOn:        data1=key, data2=velocity
//	NoteOff:       data1=key, data2=0
//	CC:            data1=cc, data2=value
//	ProgramChange: data1=bank, data2=program
//	AllNotesOff:   data1=0, data2=0
type synthEvent struct {
	kind    uint8
	channel int32
	data1   int32
	data2   int32
}

// Synth wraps one pure-Go MeltySynth rendering state. Realtime Synth values do
// not own audio devices or Oto players.
type Synth struct {
	mu            sync.Mutex // protects offline player setup only
	sampleRate    int
	syn           *meltysynth.Synthesizer
	gain          float64
	gainTarget    float64
	gainStep      float64
	gainRemaining int
	activePlayer  *Player
	isHeadless    bool

	// evQueue is a bounded MPSC queue. The audio callback is its sole consumer.
	evQueue              *ringbuf.Buffer[synthEvent]
	targetGain           atomic.Uint64
	emergencyAllNotesOff atomic.Bool
	queueDropped         atomic.Uint64
	queueHighWater       atomic.Uint64

	// DC blocker state (only accessed in Read/WriteS16, no lock needed).
	dcLeftX  float32
	dcLeftY  float32
	dcRightX float32
	dcRightY float32

	// softClipLUT is a lookup table for tanh-like soft clipping.
	softClipLUT [4096]float32
}

// New creates a Synth instance with the given audio configuration.
func New(cfg config.Audio) (*Synth, error) {
	s := &Synth{
		sampleRate: cfg.SampleRate,
		gain:       0.5, // Default master volume in meltysynth.
		gainTarget: 0.5,
		isHeadless: false,
		evQueue:    ringbuf.New[synthEvent](512),
	}
	s.targetGain.Store(math.Float64bits(s.gain))
	s.initSoftClipLUT()

	return s, nil
}

// NewHeadless creates a Synth instance with no audio driver for offline rendering.
func NewHeadless(cfg config.Audio) (*Synth, error) {
	s := &Synth{
		sampleRate: cfg.SampleRate,
		gain:       0.5,
		gainTarget: 0.5,
		isHeadless: true,
	}
	s.targetGain.Store(math.Float64bits(s.gain))
	s.initSoftClipLUT()
	return s, nil
}

// initSoftClipLUT populates the soft clipping lookup table.
// Input range [-1.5, 1.5] maps to LUT indices [0, 4095].
// The tanh transfer is continuous, monotonic, symmetric, and finite.
func (s *Synth) initSoftClipLUT() {
	for i := 0; i < 4096; i++ {
		x := float64(i-2048) / 2048.0 * 1.5
		s.softClipLUT[i] = float32(math.Tanh(x))
	}
}

// softClip applies soft clipping using the LUT.
func softClip(lut *[4096]float32, x float32) float32 {
	position := (x/1.5)*2048 + 2048
	idx := int(math.Floor(float64(position)))
	if idx <= 0 {
		return lut[0]
	}
	if idx >= 4095 {
		return lut[4095]
	}
	fraction := position - float32(idx)
	return lut[idx] + fraction*(lut[idx+1]-lut[idx])
}

// drainEvents processes all queued MIDI events under the lock.
// Called from Read/WriteS16 before rendering.
func (s *Synth) drainEvents() {
	if s.evQueue == nil {
		return
	}
	if s.syn == nil {
		return
	}
	if s.emergencyAllNotesOff.Swap(false) {
		s.syn.NoteOffAll(false)
	}
	for {
		ev, ok := s.evQueue.Read()
		if !ok {
			break
		}
		switch ev.kind {
		case evNoteOn:
			s.syn.NoteOn(ev.channel, ev.data1, ev.data2)
		case evNoteOff:
			s.syn.NoteOff(ev.channel, ev.data1)
		case evCC:
			s.syn.ProcessMidiMessage(ev.channel, 0xB0, ev.data1, ev.data2)
		case evProgramChange:
			s.syn.ProcessMidiMessage(ev.channel, 0xB0, 0x00, ev.data1) // bank
			s.syn.ProcessMidiMessage(ev.channel, 0xC0, ev.data2, 0)    // program
		case evAllNotesOff:
			s.syn.NoteOffAll(false)
		}
	}
}

// renderInto is called only by the process audio renderer (or the single
// offline renderer goroutine). The supplied buffers are reused by the caller.
func (s *Synth) renderInto(left, right []float32) {
	s.drainEvents()
	if s.syn == nil {
		clear(left)
		clear(right)
		return
	}

	if s.activePlayer != nil && atomic.LoadInt32(&s.activePlayer.active) == 1 {
		s.activePlayer.sequencer.Render(left, right)
	} else {
		s.syn.Render(left, right)
	}

	target := math.Float64frombits(s.targetGain.Load())
	if target != s.gainTarget {
		s.gainTarget = target
		s.gainRemaining = max(s.sampleRate/100, 1) // 10 ms
		s.gainStep = (target - s.gain) / float64(s.gainRemaining)
	}
	for i := range left {
		if s.gainRemaining > 0 {
			s.gain += s.gainStep
			s.gainRemaining--
			if s.gainRemaining == 0 {
				s.gain = s.gainTarget
			}
		}
		xL := left[i]
		yL := xL - s.dcLeftX + 0.999*s.dcLeftY
		s.dcLeftX = xL
		s.dcLeftY = yL

		xR := right[i]
		yR := xR - s.dcRightX + 0.999*s.dcRightY
		s.dcRightX = xR
		s.dcRightY = yR

		left[i] = softClip(&s.softClipLUT, yL*float32(s.gain))
		right[i] = softClip(&s.softClipLUT, yR*float32(s.gain))
	}
}

// LoadSoundFont loads a .sf2 file into the synth.
func (s *Synth) LoadSoundFont(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open soundfont file failed: %w", err)
	}
	defer f.Close()

	sf, err := meltysynth.NewSoundFont(f)
	if err != nil {
		return fmt.Errorf("parse soundfont failed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Low latency BlockSize: 64 samples per block
	settings := meltysynth.NewSynthesizerSettings(int32(s.sampleRate))
	settings.BlockSize = 64
	syn, err := meltysynth.NewSynthesizer(sf, settings)
	if err != nil {
		return fmt.Errorf("create synthesizer failed: %w", err)
	}

	s.syn = syn
	s.syn.MasterVolume = 1

	return nil
}

// ProgramChange selects a bank and program on a channel.
func (s *Synth) ProgramChange(channel, bank, program int) {
	if s.evQueue != nil {
		ev := synthEvent{
			kind:    evProgramChange,
			channel: int32(channel),
			data1:   int32(bank),
			data2:   int32(program),
		}
		s.enqueue(ev, false)
		return
	}
	// Headless fallback
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.syn == nil {
		return
	}
	s.syn.ProcessMidiMessage(int32(channel), 0xB0, 0x00, int32(bank))
	s.syn.ProcessMidiMessage(int32(channel), 0xC0, int32(program), 0)
}

// prepareProgram configures an unpublished synth without queueing the command.
func (s *Synth) prepareProgram(channel, bank, program int) {
	s.syn.ProcessMidiMessage(int32(channel), 0xB0, 0x00, int32(bank))
	s.syn.ProcessMidiMessage(int32(channel), 0xC0, int32(program), 0)
}

// NoteOn sends a note-on message.
func (s *Synth) NoteOn(channel, key, velocity int) {
	if s.evQueue != nil {
		ev := synthEvent{
			kind:    evNoteOn,
			channel: int32(channel),
			data1:   int32(key),
			data2:   int32(velocity),
		}
		s.enqueue(ev, false)
		return
	}
	// Headless fallback
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.syn == nil {
		return
	}
	s.syn.NoteOn(int32(channel), int32(key), int32(velocity))
}

// NoteOff sends a note-off message.
func (s *Synth) NoteOff(channel, key int) {
	if s.evQueue != nil {
		ev := synthEvent{
			kind:    evNoteOff,
			channel: int32(channel),
			data1:   int32(key),
		}
		s.enqueue(ev, true)
		return
	}
	// Headless fallback
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.syn == nil {
		return
	}
	s.syn.NoteOff(int32(channel), int32(key))
}

// CC sends a control change message.
func (s *Synth) CC(channel, cc, value int) {
	if s.evQueue != nil {
		ev := synthEvent{
			kind:    evCC,
			channel: int32(channel),
			data1:   int32(cc),
			data2:   int32(value),
		}
		s.enqueue(ev, false)
		return
	}
	// Headless fallback
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.syn == nil {
		return
	}
	s.syn.ProcessMidiMessage(int32(channel), 0xB0, int32(cc), int32(value))
}

// SetGain sets the master gain (volume) of the synth. Range: 0.0 to 1.0 (recommended).
func (s *Synth) SetGain(gain float64) {
	s.targetGain.Store(math.Float64bits(gain))
}

// prepareGain configures an unpublished synth without creating a startup ramp.
func (s *Synth) prepareGain(gain float64) {
	s.gain = gain
	s.gainTarget = gain
	s.gainStep = 0
	s.gainRemaining = 0
	s.targetGain.Store(math.Float64bits(gain))
}

// Gain returns the current master gain.
func (s *Synth) Gain() float64 {
	return math.Float64frombits(s.targetGain.Load())
}

// AllNotesOff sends All Notes Off on all channels.
func (s *Synth) AllNotesOff() {
	if s.evQueue != nil {
		s.enqueue(synthEvent{kind: evAllNotesOff}, true)
		return
	}
	// Headless fallback
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.syn == nil {
		return
	}
	s.syn.NoteOffAll(false)
}

// Close releases resources.
func (s *Synth) Close() {}

// WriteS16 renders PCM samples for offline rendering.
func (s *Synth) WriteS16(buf []int16) error {
	frames := len(buf) / 2
	if frames == 0 {
		return nil
	}

	leftBuf := make([]float32, frames)
	rightBuf := make([]float32, frames)

	s.renderInto(leftBuf, rightBuf)

	for i := 0; i < frames; i++ {
		lVal := leftBuf[i]
		rVal := rightBuf[i]

		var lInt, rInt int16
		if lVal >= 0 {
			lInt = int16(lVal*32767.0 + 0.5)
		} else {
			lInt = int16(lVal*32767.0 - 0.5)
		}
		if rVal >= 0 {
			rInt = int16(rVal*32767.0 + 0.5)
		} else {
			rInt = int16(rVal*32767.0 - 0.5)
		}

		buf[i*2] = lInt
		buf[i*2+1] = rInt
	}

	return nil
}

// Player wraps a MeltySynth MidiFileSequencer attached to a Synth.
type Player struct {
	synth     *Synth
	sequencer *meltysynth.MidiFileSequencer
	midiFile  *meltysynth.MidiFile
	active    int32
}

// OpenPlayer creates a MIDI player on s and loads the given .mid file.
func (s *Synth) OpenPlayer(midPath string) (*Player, error) {
	f, err := os.Open(midPath)
	if err != nil {
		return nil, fmt.Errorf("open mid file failed: %w", err)
	}
	defer f.Close()

	mf, err := meltysynth.NewMidiFile(f)
	if err != nil {
		return nil, fmt.Errorf("parse midi file failed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.syn == nil {
		return nil, fmt.Errorf("synthesizer not loaded (load SoundFont first)")
	}

	seq := meltysynth.NewMidiFileSequencer(s.syn)
	p := &Player{
		synth:     s,
		sequencer: seq,
		midiFile:  mf,
	}

	s.activePlayer = p
	return p, nil
}

// Play starts MIDI playback on the player.
func (p *Player) Play() error {
	p.sequencer.Play(p.midiFile, false)
	atomic.StoreInt32(&p.active, 1)
	return nil
}

// IsDone returns true when MIDI playback has finished.
func (p *Player) IsDone() bool {
	return p.sequencer.IsDone()
}

// Close releases the player.
func (p *Player) Close() {
	atomic.StoreInt32(&p.active, 0)
	p.sequencer.Stop()

	p.synth.mu.Lock()
	if p.synth.activePlayer == p {
		p.synth.activePlayer = nil
	}
	p.synth.mu.Unlock()
}

// logDroppedEvent logs when the event queue is full.
func (s *Synth) logDroppedEvent(ev synthEvent) {
	switch ev.kind {
	case evNoteOn:
		log.Printf("warning: synth event queue full, NoteOn dropped (ch=%d key=%d vel=%d)",
			ev.channel, ev.data1, ev.data2)
	case evNoteOff:
		log.Printf("warning: synth event queue full, NoteOff dropped (ch=%d key=%d)",
			ev.channel, ev.data1)
	default:
		log.Printf("warning: synth event queue full, event kind=%d dropped", ev.kind)
	}
}

func (s *Synth) enqueue(ev synthEvent, critical bool) bool {
	if s.evQueue.Write(ev) {
		depth := uint64(s.evQueue.Len())
		for maximum := s.queueHighWater.Load(); depth > maximum; maximum = s.queueHighWater.Load() {
			if s.queueHighWater.CompareAndSwap(maximum, depth) {
				break
			}
		}
		return true
	}
	s.queueDropped.Add(1)
	if critical {
		s.emergencyAllNotesOff.Store(true)
	}
	s.logDroppedEvent(ev)
	return false
}

// QueueMetrics reports realtime synth command queue pressure.
type QueueMetrics struct {
	Depth     int    `json:"depth"`
	Capacity  int    `json:"capacity"`
	HighWater uint64 `json:"high_water"`
	Dropped   uint64 `json:"dropped"`
}

func (s *Synth) QueueMetrics() QueueMetrics {
	if s == nil || s.evQueue == nil {
		return QueueMetrics{}
	}
	return QueueMetrics{
		Depth:     s.evQueue.Len(),
		Capacity:  s.evQueue.Cap(),
		HighWater: s.queueHighWater.Load(),
		Dropped:   s.queueDropped.Load(),
	}
}
