package router

import (
	"testing"

	"github.com/wentf9/subitohost/internal/midi"
)

func TestVelocityLinear(t *testing.T) {
	r, _ := midi.ParseNoteRange("C-1", "C8")
	rule := &VelocityCurveRule{Range: r, Channel: -1, Curve: CurveLinear}

	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 60, Value: 80}
	out := rule.Apply(ev)
	if out[0].Value != 80 {
		t.Errorf("linear: Value = %d, want 80", out[0].Value)
	}
}

func TestVelocityExponential(t *testing.T) {
	r, _ := midi.ParseNoteRange("C-1", "C8")
	rule := &VelocityCurveRule{Range: r, Channel: -1, Curve: CurveExponential}

	// Soft touch: 64 → (64/127)^2 * 127 ≈ 32
	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 60, Value: 64}
	out := rule.Apply(ev)
	if out[0].Value != 32 {
		t.Errorf("exponential(64) = %d, want 32", out[0].Value)
	}

	// Full velocity stays at 127
	ev.Value = 127
	out = rule.Apply(ev)
	if out[0].Value != 127 {
		t.Errorf("exponential(127) = %d, want 127", out[0].Value)
	}

	// Zero stays at 0
	ev.Value = 0
	out = rule.Apply(ev)
	if out[0].Value != 0 {
		t.Errorf("exponential(0) = %d, want 0", out[0].Value)
	}
}

func TestVelocityLogarithmic(t *testing.T) {
	r, _ := midi.ParseNoteRange("C-1", "C8")
	rule := &VelocityCurveRule{Range: r, Channel: -1, Curve: CurveLogarithmic}

	// Soft touch: 64 → sqrt(64/127) * 127 ≈ 90
	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 60, Value: 64}
	out := rule.Apply(ev)
	if out[0].Value != 90 {
		t.Errorf("logarithmic(64) = %d, want 90", out[0].Value)
	}
}

func TestVelocityNoteOffIgnored(t *testing.T) {
	r, _ := midi.ParseNoteRange("C-1", "C8")
	rule := &VelocityCurveRule{Range: r, Channel: -1, Curve: CurveExponential}

	ev := midi.Event{Type: midi.NoteOff, Channel: 0, Key: 60, Value: 64}
	out := rule.Apply(ev)
	if out[0].Value != 64 {
		t.Errorf("NoteOff velocity should not change, got %d", out[0].Value)
	}
}
