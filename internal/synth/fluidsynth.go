// internal/synth/fluidsynth.go
package synth

import (
	"fmt"
	"log"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/wentf9/subitohost/internal/config"
	"github.com/wentf9/subitohost/internal/ringbuf"
	"github.com/wentf9/subitohost/internal/synth/meltysynth"
)

var (
	otoCtx     *oto.Context
	otoCtxOnce sync.Once
	otoCtxErr  error
)

func getOtoContext(sampleRate int) (*oto.Context, error) {
	var err error
	otoCtxOnce.Do(func() {
		op := &oto.NewContextOptions{
			SampleRate:   sampleRate,
			ChannelCount: 2,
			Format:       oto.FormatFloat32LE,
			BufferSize:   30 * time.Millisecond,
		}
		var ready chan struct{}
		otoCtx, ready, err = oto.NewContext(op)
		if err == nil {
			<-ready
		} else {
			otoCtxErr = err
		}
	})
	if otoCtxErr != nil {
		return nil, otoCtxErr
	}
	return otoCtx, err
}

// synthEvent kinds for the lock-free event queue.
const (
	evNoteOn        uint8 = 0
	evNoteOff       uint8 = 1
	evCC            uint8 = 2
	evProgramChange uint8 = 3
	evAllNotesOff   uint8 = 4
	evSetGain       uint8 = 5
)

// synthEvent is the carrier for MIDI events in the lock-free queue.
// data1/data2 semantics depend on kind:
//
//	NoteOn:        data1=key, data2=velocity
//	NoteOff:       data1=key, data2=0
//	CC:            data1=cc, data2=value
//	ProgramChange: data1=bank, data2=program
//	AllNotesOff:   data1=0, data2=0
//	SetGain:       data1=float64 bits, data2=0
type synthEvent struct {
	kind    uint8
	channel int32
	data1   int32
	data2   int32
}

// Synth wraps a pure Go MeltySynth instance and ebitengine/oto/v3 driver.
type Synth struct {
	mu           sync.Mutex // protects syn, player, activePlayer, gain, muted
	sampleRate   int
	syn          *meltysynth.Synthesizer
	otoCtx       *oto.Context
	player       *oto.Player
	gain         float64
	activePlayer *Player
	isHeadless   bool

	// evQueue is a lock-free SPSC queue: MIDI thread produces, audio Read
	// thread consumes. nil in headless mode (direct calls instead).
	evQueue *ringbuf.Buffer[synthEvent]

	// muted suppresses audio output (used during synth swap).
	muted bool

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
		isHeadless: false,
		evQueue:    ringbuf.New[synthEvent](512),
	}
	s.initSoftClipLUT()

	ctx, err := getOtoContext(cfg.SampleRate)
	if err != nil {
		return nil, fmt.Errorf("oto context initialization failed: %w", err)
	}
	s.otoCtx = ctx

	// The player reads from Synth itself, which implements io.Reader
	s.player = ctx.NewPlayer(s)
	s.player.SetBufferSize(8192)
	s.player.Play()

	return s, nil
}

// NewHeadless creates a Synth instance with no audio driver for offline rendering.
func NewHeadless(cfg config.Audio) (*Synth, error) {
	s := &Synth{
		sampleRate: cfg.SampleRate,
		gain:       0.5,
		isHeadless: true,
	}
	s.initSoftClipLUT()
	return s, nil
}

// initSoftClipLUT populates the soft clipping lookup table.
// Input range [-1.5, 1.5] maps to LUT indices [0, 4095].
// Linear region |x| < 1.0 uses x - x^3/3 (tanh approximation).
// Saturation region uses a soft asymptotic curve.
func (s *Synth) initSoftClipLUT() {
	for i := 0; i < 4096; i++ {
		x := float64(i-2048) / 2048.0 * 1.5
		var v float64
		if x > 1.0 {
			t := x - 1.0
			v = 1.0 - 1.0/(3.0*t*t+1.0)
		} else if x < -1.0 {
			t := -x - 1.0
			v = -(1.0 - 1.0/(3.0*t*t+1.0))
		} else {
			v = x - x*x*x/3.0
		}
		s.softClipLUT[i] = float32(v)
	}
}

// softClip applies soft clipping using the LUT.
func softClip(lut *[4096]float32, x float32) float32 {
	idx := int((x/1.5)*2048 + 2048)
	if idx < 0 {
		return lut[0]
	}
	if idx >= 4096 {
		return lut[4095]
	}
	return lut[idx]
}

// drainEvents processes all queued MIDI events under the lock.
// Called from Read/WriteS16 before rendering.
func (s *Synth) drainEvents() {
	if s.evQueue == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.syn == nil {
		return
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
			s.syn.ProcessMidiMessage(ev.channel, 0xC0, ev.data2, 0)     // program
		case evAllNotesOff:
			s.syn.NoteOffAll(false)
		case evSetGain:
			newGain := math.Float64frombits(uint64(ev.data1))
			s.gain = newGain
			s.syn.MasterVolume = float32(newGain)
		}
	}
}

// Read implements io.Reader to feed the ebitengine/oto/v3 audio driver.
func (s *Synth) Read(p []byte) (n int, err error) {
	// 1 frame = 2 channels * 4 bytes (32-bit Float LE) = 8 bytes
	frames := len(p) / 8
	if frames == 0 {
		return 0, nil
	}

	// Check muted flag (set during synth swap)
	s.mu.Lock()
	muted := s.muted
	s.mu.Unlock()
	if muted {
		for i := range p {
			p[i] = 0
		}
		return frames * 8, nil
	}

	// Drain queued events before rendering
	s.drainEvents()

	leftBuf := make([]float32, frames)
	rightBuf := make([]float32, frames)

	s.mu.Lock()
	syn := s.syn
	activePlayer := s.activePlayer
	s.mu.Unlock()

	if syn == nil {
		for i := range p {
			p[i] = 0
		}
		return frames * 8, nil
	}

	if activePlayer != nil && atomic.LoadInt32(&activePlayer.active) == 1 {
		activePlayer.sequencer.Render(leftBuf, rightBuf)
	} else {
		syn.Render(leftBuf, rightBuf)
	}

	// DC blocker + soft clip + encode to float32 LE
	for i := 0; i < frames; i++ {
		xL := leftBuf[i]
		yL := xL - s.dcLeftX + 0.999*s.dcLeftY
		s.dcLeftX = xL
		s.dcLeftY = yL

		xR := rightBuf[i]
		yR := xR - s.dcRightX + 0.999*s.dcRightY
		s.dcRightX = xR
		s.dcRightY = yR

		lVal := softClip(&s.softClipLUT, yL)
		rVal := softClip(&s.softClipLUT, yR)

		lBits := math.Float32bits(lVal)
		rBits := math.Float32bits(rVal)

		p[i*8] = byte(lBits)
		p[i*8+1] = byte(lBits >> 8)
		p[i*8+2] = byte(lBits >> 16)
		p[i*8+3] = byte(lBits >> 24)
		p[i*8+4] = byte(rBits)
		p[i*8+5] = byte(rBits >> 8)
		p[i*8+6] = byte(rBits >> 16)
		p[i*8+7] = byte(rBits >> 24)
	}

	return frames * 8, nil
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
	s.syn.MasterVolume = float32(s.gain)

	return nil
}

// ProgramChange selects a bank and program on a channel.
func (s *Synth) ProgramChange(channel, bank, program int) {
	if s.evQueue != nil {
		s.evQueue.Write(synthEvent{
			kind:    evProgramChange,
			channel: int32(channel),
			data1:   int32(bank),
			data2:   int32(program),
		})
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

// NoteOn sends a note-on message.
func (s *Synth) NoteOn(channel, key, velocity int) {
	if s.evQueue != nil {
		s.evQueue.Write(synthEvent{
			kind:    evNoteOn,
			channel: int32(channel),
			data1:   int32(key),
			data2:   int32(velocity),
		})
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
		s.evQueue.Write(synthEvent{
			kind:    evNoteOff,
			channel: int32(channel),
			data1:   int32(key),
		})
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
		s.evQueue.Write(synthEvent{
			kind:    evCC,
			channel: int32(channel),
			data1:   int32(cc),
			data2:   int32(value),
		})
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
	if s.evQueue != nil {
		s.evQueue.Write(synthEvent{
			kind:  evSetGain,
			data1: int32(math.Float64bits(gain)),
		})
		return
	}
	// Headless fallback
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gain = gain
	if s.syn != nil {
		s.syn.MasterVolume = float32(gain)
	}
}

// Gain returns the current master gain.
func (s *Synth) Gain() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gain
}

// AllNotesOff sends All Notes Off on all channels.
func (s *Synth) AllNotesOff() {
	if s.evQueue != nil {
		s.evQueue.Write(synthEvent{kind: evAllNotesOff})
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

// DrainAndMute drains the event queue and mutes audio output.
// Used during synth swap to prevent overlapping audio from the old synth.
func (s *Synth) DrainAndMute() {
	// Drain any pending NoteOff events to ensure clean silence
	s.drainEvents()
	s.mu.Lock()
	s.muted = true
	s.mu.Unlock()
}

// Close releases resources.
func (s *Synth) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.player != nil {
		s.player.Close()
		s.player = nil
	}
}

// WriteS16 renders PCM samples for offline rendering.
func (s *Synth) WriteS16(buf []int16) error {
	// Drain queued events (headless mode: queue is nil, no-op)
	s.drainEvents()

	frames := len(buf) / 2
	if frames == 0 {
		return nil
	}

	leftBuf := make([]float32, frames)
	rightBuf := make([]float32, frames)

	s.mu.Lock()
	syn := s.syn
	activePlayer := s.activePlayer
	s.mu.Unlock()

	if syn == nil {
		for i := range buf {
			buf[i] = 0
		}
		return nil
	}

	if activePlayer != nil && atomic.LoadInt32(&activePlayer.active) == 1 {
		activePlayer.sequencer.Render(leftBuf, rightBuf)
	} else {
		syn.Render(leftBuf, rightBuf)
	}

	for i := 0; i < frames; i++ {
		// DC Blocker filter (R = 0.999)
		xL := leftBuf[i]
		yL := xL - s.dcLeftX + 0.999*s.dcLeftY
		s.dcLeftX = xL
		s.dcLeftY = yL

		xR := rightBuf[i]
		yR := xR - s.dcRightX + 0.999*s.dcRightY
		s.dcRightX = xR
		s.dcRightY = yR

		lVal := softClip(&s.softClipLUT, yL)
		rVal := softClip(&s.softClipLUT, yR)

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
