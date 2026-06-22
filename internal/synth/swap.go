// internal/synth/swap.go
package synth

import (
	"log"
	"sync/atomic"
	"time"

	"github.com/wentf9/subitohost/internal/config"
	"github.com/wentf9/subitohost/internal/setlist"
)

// SwapManager handles atomic SoundFont swaps.
type SwapManager struct {
	Active atomic.Pointer[Synth]
	cfg    config.Audio
	gain   float64 // current master gain
}

// NewSwapManager creates a swap manager with the given audio config.
func NewSwapManager(cfg config.Audio) *SwapManager {
	return &SwapManager{cfg: cfg, gain: 1.0}
}

// Init creates the first synth instance.
func (m *SwapManager) Init() error {
	s, err := New(m.cfg)
	if err != nil {
		return err
	}
	m.Active.Store(s)
	return nil
}

// SwapTo loads a new SoundFont and programs, atomically replacing the active synth.
// On failure, the old synth continues running.
func (m *SwapManager) SwapTo(profile *setlist.Profile) error {
	newSynth, err := New(m.cfg)
	if err != nil {
		return err
	}

	if err := newSynth.LoadSoundFont(profile.SFPath); err != nil {
		newSynth.Close()
		return err
	}

	// Inherit current gain on swap
	newSynth.SetGain(m.gain)

	for _, p := range profile.Programs {
		newSynth.ProgramChange(p.Channel, p.Bank, p.Program)
	}

	old := m.Active.Load()
	if old != nil {
		// Mute old synth and drain its event queue to prevent overlapping audio
		old.DrainAndMute()
		// Wait for one oto buffer cycle (~21ms) to let ALSA drain old data
		time.Sleep(25 * time.Millisecond)
	}

	m.Active.Store(newSynth)

	if old != nil {
		// Close old player now that its output is muted and drained
		go func() {
			time.Sleep(50 * time.Millisecond)
			old.Close()
			log.Printf("closed old synth after swap to %q", profile.Name)
		}()
	}
	return nil
}

// Close shuts down the active synth.
func (m *SwapManager) Close() {
	if s := m.Active.Load(); s != nil {
		s.AllNotesOff()
		s.Close()
	}
}

// SetGain sets master gain on the active synth and stores it for future swaps.
func (m *SwapManager) SetGain(gain float64) {
	m.gain = gain
	if s := m.Active.Load(); s != nil {
		s.SetGain(gain)
	}
}

// Gain returns the current master gain.
func (m *SwapManager) Gain() float64 {
	return m.gain
}
