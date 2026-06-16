package router

import (
	"testing"

	"github.com/wentf9/subitohost/internal/midi"
)

func TestTransposeUp(t *testing.T) {
	r, _ := midi.ParseNoteRange("C-1", "B2")
	rule := &TransposeRule{Range: r, Channel: -1, Semitones: 12}

	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 36, Value: 100}
	out := rule.Apply(ev)
	if len(out) != 1 || out[0].Key != 48 {
		t.Errorf("Key = %d, want 48", out[0].Key)
	}
}

func TestTransposeDown(t *testing.T) {
	r, _ := midi.ParseNoteRange("C-1", "C8")
	rule := &TransposeRule{Range: r, Channel: -1, Semitones: -12}

	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 60, Value: 100}
	out := rule.Apply(ev)
	if out[0].Key != 48 {
		t.Errorf("Key = %d, want 48", out[0].Key)
	}
}

func TestTransposeClamp(t *testing.T) {
	r, _ := midi.ParseNoteRange("C-1", "C8")
	rule := &TransposeRule{Range: r, Channel: -1, Semitones: 100}

	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 100, Value: 100}
	out := rule.Apply(ev)
	if out[0].Key != 127 {
		t.Errorf("Key = %d, want clamped to 127", out[0].Key)
	}
}

func TestTransposeChannelFilter(t *testing.T) {
	r, _ := midi.ParseNoteRange("C-1", "C8")
	rule := &TransposeRule{Range: r, Channel: 1, Semitones: 12}

	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 60, Value: 100}
	out := rule.Apply(ev)
	if out[0].Key != 60 {
		t.Errorf("wrong channel should not transpose, Key = %d", out[0].Key)
	}

	ev.Channel = 1
	out = rule.Apply(ev)
	if out[0].Key != 72 {
		t.Errorf("Key = %d, want 72", out[0].Key)
	}
}
