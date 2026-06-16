package router

import (
	"testing"

	"github.com/wentf9/subitohost/internal/midi"
)

func TestChordExpandNoteOn(t *testing.T) {
	r, _ := midi.ParseNoteRange("C-1", "C8")
	rule := &ChordExpandRule{Range: r, Channel: -1, Intervals: []int{0, 7, 12}}

	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 60, Value: 100}
	out := rule.Apply(ev)
	if len(out) != 3 {
		t.Fatalf("expected 3 events, got %d", len(out))
	}
	wantKeys := []int{60, 67, 72}
	for i, want := range wantKeys {
		if out[i].Key != want {
			t.Errorf("out[%d].Key = %d, want %d", i, out[i].Key, want)
		}
		if out[i].Type != midi.NoteOn {
			t.Errorf("out[%d].Type = %d, want NoteOn", i, out[i].Type)
		}
		if out[i].Value != 100 {
			t.Errorf("out[%d].Value = %d, want 100", i, out[i].Value)
		}
	}
}

func TestChordExpandNoteOff(t *testing.T) {
	r, _ := midi.ParseNoteRange("C-1", "C8")
	rule := &ChordExpandRule{Range: r, Channel: -1, Intervals: []int{0, 7, 12}}

	ev := midi.Event{Type: midi.NoteOff, Channel: 0, Key: 60, Value: 0}
	out := rule.Apply(ev)
	if len(out) != 3 {
		t.Fatalf("NoteOff should also expand, got %d", len(out))
	}
	if out[1].Key != 67 || out[2].Key != 72 {
		t.Errorf("NoteOff keys = %d, %d", out[1].Key, out[2].Key)
	}
}

func TestChordExpandClipsHighNotes(t *testing.T) {
	r, _ := midi.ParseNoteRange("C-1", "C8")
	rule := &ChordExpandRule{Range: r, Channel: -1, Intervals: []int{0, 7, 12}}

	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 125, Value: 100}
	out := rule.Apply(ev)
	// 125+0=125 (ok), 125+7=132 (skip), 125+12=137 (skip)
	if len(out) != 1 {
		t.Fatalf("expected 1 event (others out of range), got %d", len(out))
	}
	if out[0].Key != 125 {
		t.Errorf("Key = %d", out[0].Key)
	}
}

func TestChordExpandCCPassthrough(t *testing.T) {
	r, _ := midi.ParseNoteRange("C-1", "C8")
	rule := &ChordExpandRule{Range: r, Channel: -1, Intervals: []int{0, 7, 12}}

	ev := midi.Event{Type: midi.CC, Channel: 0, Key: 64, Value: 127}
	out := rule.Apply(ev)
	if len(out) != 1 {
		t.Error("CC should pass through")
	}
}

func TestChordExpandChannelFilter(t *testing.T) {
	r, _ := midi.ParseNoteRange("C-1", "C8")
	rule := &ChordExpandRule{Range: r, Channel: 1, Intervals: []int{0, 7, 12}}

	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 60, Value: 100}
	out := rule.Apply(ev)
	if len(out) != 1 {
		t.Errorf("wrong channel should not expand, got %d events", len(out))
	}
}
