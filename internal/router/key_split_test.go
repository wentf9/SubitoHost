package router

import (
	"testing"

	"github.com/wentf9/subitohost/internal/midi"
)

func TestKeySplitInRange(t *testing.T) {
	r, _ := midi.ParseNoteRange("C-1", "B2")
	rule := &KeySplitRule{Range: r, TargetChannel: 1}

	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 36, Value: 100}
	out := rule.Apply(ev)
	if len(out) != 1 {
		t.Fatalf("len = %d", len(out))
	}
	if out[0].Channel != 1 {
		t.Errorf("Channel = %d, want 1", out[0].Channel)
	}
	if out[0].Key != 36 || out[0].Value != 100 {
		t.Error("Key/Value should be unchanged")
	}
}

func TestKeySplitOutOfRange(t *testing.T) {
	r, _ := midi.ParseNoteRange("C-1", "B2")
	rule := &KeySplitRule{Range: r, TargetChannel: 1}

	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 60, Value: 100}
	out := rule.Apply(ev)
	if len(out) != 1 || out[0].Channel != 0 {
		t.Errorf("out-of-range note should pass through unchanged, got channel %d", out[0].Channel)
	}
}

func TestKeySplitCCIgnored(t *testing.T) {
	r, _ := midi.ParseNoteRange("C-1", "C8")
	rule := &KeySplitRule{Range: r, TargetChannel: 1}

	ev := midi.Event{Type: midi.CC, Channel: 0, Key: 64, Value: 127}
	out := rule.Apply(ev)
	if len(out) != 1 || out[0].Channel != 0 {
		t.Error("CC events should pass through unchanged")
	}
}
