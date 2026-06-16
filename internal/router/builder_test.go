package router

import (
	"testing"

	"github.com/wentf9/subitohost/internal/midi"
	"github.com/wentf9/subitohost/internal/setlist"
)

func intPtr(v int) *int { return &v }

func TestBuildKeySplit(t *testing.T) {
	cfg := setlist.RuleConfig{
		ID:            "r1",
		Type:          "key_split",
		Range:         [2]string{"C-1", "B2"},
		TargetChannel: intPtr(1),
	}
	rule, err := BuildRule(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 36, Value: 100}
	out := rule.Apply(ev)
	if out[0].Channel != 1 {
		t.Errorf("Channel = %d", out[0].Channel)
	}
}

func TestBuildTranspose(t *testing.T) {
	cfg := setlist.RuleConfig{
		ID:            "r2",
		Type:          "transpose",
		Range:         [2]string{"C-1", "B2"},
		Value:         intPtr(12),
		TargetChannel: intPtr(1),
	}
	rule, err := BuildRule(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ev := midi.Event{Type: midi.NoteOn, Channel: 1, Key: 36, Value: 100}
	out := rule.Apply(ev)
	if out[0].Key != 48 {
		t.Errorf("Key = %d", out[0].Key)
	}
}

func TestBuildVelocityCurve(t *testing.T) {
	cfg := setlist.RuleConfig{
		ID:    "r3",
		Type:  "velocity_curve",
		Range: [2]string{"C-1", "C8"},
		Curve: "exponential",
	}
	rule, err := BuildRule(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 60, Value: 64}
	out := rule.Apply(ev)
	if out[0].Value != 32 {
		t.Errorf("Value = %d, want 32", out[0].Value)
	}
}

func TestBuildChordExpand(t *testing.T) {
	cfg := setlist.RuleConfig{
		ID:        "r4",
		Type:      "chord_expand",
		Range:     [2]string{"C-1", "C8"},
		Intervals: []int{0, 7, 12},
	}
	rule, err := BuildRule(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ev := midi.Event{Type: midi.NoteOn, Channel: 0, Key: 60, Value: 100}
	out := rule.Apply(ev)
	if len(out) != 3 {
		t.Fatalf("expected 3, got %d", len(out))
	}
}

func TestBuildUnknownType(t *testing.T) {
	cfg := setlist.RuleConfig{Type: "unknown"}
	_, err := BuildRule(cfg)
	if err == nil {
		t.Error("unknown type should return error")
	}
}

func TestBuildRulesChordExpandMustBeLast(t *testing.T) {
	cfgs := []setlist.RuleConfig{
		{ID: "r1", Type: "chord_expand", Range: [2]string{"C-1", "C8"}, Intervals: []int{0, 12}},
		{ID: "r2", Type: "transpose", Range: [2]string{"C-1", "C8"}, Value: intPtr(1)},
	}
	_, err := BuildRules(cfgs)
	if err == nil {
		t.Error("chord_expand not last should return error")
	}
}
