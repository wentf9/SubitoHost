package router

import (
	"testing"

	"github.com/wentf9/subitohost/internal/midi"
)

func TestRouterChain(t *testing.T) {
	splitRange, _ := midi.ParseNoteRange("C-1", "B2")
	transposeRange, _ := midi.ParseNoteRange("C-1", "B2")

	r := New(
		[]Rule{
			&KeySplitRule{Range: splitRange, TargetChannel: 1},
			&TransposeRule{Range: transposeRange, Channel: 1, Semitones: 12},
		},
		nil,
	)

	// Low note: split to ch1, then transposed
	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 36, Value: 100}
	result := r.Process(ev)
	if result.Triggered {
		t.Error("should not trigger")
	}
	if len(result.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Events))
	}
	if result.Events[0].Channel != 1 || result.Events[0].Key != 48 {
		t.Errorf("event = %+v, want ch=1 key=48", result.Events[0])
	}

	// High note: passes through unchanged
	ev = midi.Event{Type: midi.NoteOn, Channel: 0, Key: 60, Value: 100}
	result = r.Process(ev)
	if result.Events[0].Channel != 0 || result.Events[0].Key != 60 {
		t.Errorf("high note = %+v, want ch=0 key=60", result.Events[0])
	}
}

func TestRouterTriggerDetection(t *testing.T) {
	trigger := &Trigger{CCNumber: 66, CCValueMin: 64}
	r := New(nil, trigger)

	// Matching CC event
	ev := midi.Event{Type: midi.CC, Channel: 0, Key: 66, Value: 127}
	result := r.Process(ev)
	if !result.Triggered {
		t.Error("should trigger")
	}
	if len(result.Events) != 0 {
		t.Error("triggered event should be consumed")
	}

	// Below threshold
	ev = midi.Event{Type: midi.CC, Channel: 0, Key: 66, Value: 10}
	result = r.Process(ev)
	if result.Triggered {
		t.Error("below threshold should not trigger")
	}
	if len(result.Events) != 1 {
		t.Error("non-trigger CC should pass through")
	}
}

func TestRouterNoRules(t *testing.T) {
	r := New(nil, nil)
	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 60, Value: 100}
	result := r.Process(ev)
	if len(result.Events) != 1 || result.Events[0] != ev {
		t.Error("no rules should pass through unchanged")
	}
}
