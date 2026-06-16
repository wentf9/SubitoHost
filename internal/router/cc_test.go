package router

import (
	"testing"

	"github.com/wentf9/subitohost/internal/midi"
)

func TestCCFilterBlocks(t *testing.T) {
	rule := &CCFilterRule{BlockCCs: []int{64, 66}}

	ev := midi.Event{Type: midi.CC, Channel: 0, Key: 64, Value: 127}
	out := rule.Apply(ev)
	if len(out) != 0 {
		t.Errorf("CC 64 should be filtered, got %d events", len(out))
	}
}

func TestCCFilterPasses(t *testing.T) {
	rule := &CCFilterRule{BlockCCs: []int{64}}

	ev := midi.Event{Type: midi.CC, Channel: 0, Key: 1, Value: 100}
	out := rule.Apply(ev)
	if len(out) != 1 {
		t.Fatalf("CC 1 should pass, got %d events", len(out))
	}
	if out[0].Key != 1 {
		t.Errorf("CC number = %d", out[0].Key)
	}
}

func TestCCFilterIgnoresNotes(t *testing.T) {
	rule := &CCFilterRule{BlockCCs: []int{64}}

	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 64, Value: 100}
	out := rule.Apply(ev)
	if len(out) != 1 {
		t.Error("NoteOn should not be filtered")
	}
}

func TestCCMapRemaps(t *testing.T) {
	rule := &CCMapRule{FromCC: 1, ToCC: 11}

	ev := midi.Event{Type: midi.CC, Channel: 0, Key: 1, Value: 80}
	out := rule.Apply(ev)
	if len(out) != 1 || out[0].Key != 11 {
		t.Errorf("CC should be remapped to 11, got Key=%d", out[0].Key)
	}
	if out[0].Value != 80 {
		t.Errorf("Value should be unchanged, got %d", out[0].Value)
	}
}

func TestCCMapNoMatch(t *testing.T) {
	rule := &CCMapRule{FromCC: 1, ToCC: 11}

	ev := midi.Event{Type: midi.CC, Channel: 0, Key: 7, Value: 80}
	out := rule.Apply(ev)
	if out[0].Key != 7 {
		t.Errorf("non-matching CC should pass through, Key=%d", out[0].Key)
	}
}
