// internal/synth/fluidsynth.go
package synth

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/wentf9/subitohost/internal/config"
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
			Format:       oto.FormatSignedInt16LE,
			BufferSize:   20 * time.Millisecond,
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

// Synth wraps a pure Go MeltySynth instance and ebitengine/oto/v3 driver.
type Synth struct {
	mu           sync.Mutex
	sampleRate   int
	syn          *meltysynth.Synthesizer
	otoCtx       *oto.Context
	player       *oto.Player
	gain         float64
	activePlayer *Player
	isHeadless   bool
}

// New creates a Synth instance with the given audio configuration.
func New(cfg config.Audio) (*Synth, error) {
	s := &Synth{
		sampleRate: cfg.SampleRate,
		gain:       0.5, // Default master volume in meltysynth.
		isHeadless: false,
	}

	ctx, err := getOtoContext(cfg.SampleRate)
	if err != nil {
		return nil, fmt.Errorf("oto context initialization failed: %w", err)
	}
	s.otoCtx = ctx

	// The player reads from Synth itself, which implements io.Reader
	s.player = ctx.NewPlayer(s)
	s.player.SetBufferSize(2048)
	s.player.Play()

	return s, nil
}

// NewHeadless creates a Synth instance with no audio driver for offline rendering.
func NewHeadless(cfg config.Audio) (*Synth, error) {
	return &Synth{
		sampleRate: cfg.SampleRate,
		gain:       0.5,
		isHeadless: true,
	}, nil
}

// Read implements io.Reader to feed the ebitengine/oto/v3 audio driver.
func (s *Synth) Read(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1 frame = 2 channels * 2 bytes (16-bit PCM) = 4 bytes
	frames := len(p) / 4
	if frames == 0 {
		return 0, nil
	}

	leftBuf := make([]float32, frames)
	rightBuf := make([]float32, frames)

	if s.syn == nil {
		// Fill with silence.
		for i := range p {
			p[i] = 0
		}
		return frames * 4, nil
	}

	if s.activePlayer != nil && atomic.LoadInt32(&s.activePlayer.active) == 1 {
		s.activePlayer.sequencer.Render(leftBuf, rightBuf)
	} else {
		s.syn.Render(leftBuf, rightBuf)
	}

	for i := 0; i < frames; i++ {
		lVal := leftBuf[i]
		if lVal > 1.0 {
			lVal = 1.0
		} else if lVal < -1.0 {
			lVal = -1.0
		}
		rVal := rightBuf[i]
		if rVal > 1.0 {
			rVal = 1.0
		} else if rVal < -1.0 {
			rVal = -1.0
		}

		lInt := int16(lVal * 32767.0)
		rInt := int16(rVal * 32767.0)

		p[i*4] = byte(lInt)
		p[i*4+1] = byte(lInt >> 8)
		p[i*4+2] = byte(rInt)
		p[i*4+3] = byte(rInt >> 8)
	}

	return frames * 4, nil
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.syn == nil {
		return
	}
	// Bank Selection: CC 0x00
	s.syn.ProcessMidiMessage(int32(channel), 0xB0, 0x00, int32(bank))
	// Program Change: 0xC0
	s.syn.ProcessMidiMessage(int32(channel), 0xC0, int32(program), 0)
}

// NoteOn sends a note-on message.
func (s *Synth) NoteOn(channel, key, velocity int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.syn == nil {
		return
	}
	s.syn.NoteOn(int32(channel), int32(key), int32(velocity))
}

// NoteOff sends a note-off message.
func (s *Synth) NoteOff(channel, key int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.syn == nil {
		return
	}
	s.syn.NoteOff(int32(channel), int32(key))
}

// CC sends a control change message.
func (s *Synth) CC(channel, cc, value int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.syn == nil {
		return
	}
	s.syn.ProcessMidiMessage(int32(channel), 0xB0, int32(cc), int32(value))
}

// SetGain sets the master gain (volume) of the synth. Range: 0.0 to 1.0 (recommended).
func (s *Synth) SetGain(gain float64) {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.syn == nil {
		return
	}
	s.syn.NoteOffAll(false)
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
	s.mu.Lock()
	defer s.mu.Unlock()

	frames := len(buf) / 2
	if frames == 0 {
		return nil
	}

	leftBuf := make([]float32, frames)
	rightBuf := make([]float32, frames)

	if s.syn == nil {
		for i := range buf {
			buf[i] = 0
		}
		return nil
	}

	if s.activePlayer != nil && atomic.LoadInt32(&s.activePlayer.active) == 1 {
		s.activePlayer.sequencer.Render(leftBuf, rightBuf)
	} else {
		s.syn.Render(leftBuf, rightBuf)
	}

	for i := 0; i < frames; i++ {
		lVal := leftBuf[i]
		if lVal > 1.0 {
			lVal = 1.0
		} else if lVal < -1.0 {
			lVal = -1.0
		}
		rVal := rightBuf[i]
		if rVal > 1.0 {
			rVal = 1.0
		} else if rVal < -1.0 {
			rVal = -1.0
		}

		buf[i*2] = int16(lVal * 32767.0)
		buf[i*2+1] = int16(rVal * 32767.0)
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
