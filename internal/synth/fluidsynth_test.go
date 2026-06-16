// internal/synth/fluidsynth_test.go
//go:build cgo && integration

package synth

import (
	"testing"

	"github.com/wentf9/subitohost/internal/config"
)

func TestNewSynth(t *testing.T) {
	cfg := config.Audio{
		Backend:    "pulseaudio",
		SampleRate: 48000,
		BufferSize: 128,
		Periods:    2,
	}
	s, err := New(cfg)
	if err != nil {
		t.Skipf("FluidSynth not available: %v", err)
	}
	defer s.Close()

	// Just verify it doesn't panic
	s.NoteOn(0, 60, 100)
	s.NoteOff(0, 60)
	s.AllNotesOff()
}
