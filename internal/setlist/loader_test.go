package setlist

import (
	"os"
	"path/filepath"
	"testing"
)

const testJSON = `{
	"setlist_id": "test1",
	"name": "Test Setlist",
	"profiles": [
		{
			"profile_id": "p1",
			"name": "Piano",
			"soundfont_path": "/sounds/piano.sf2",
			"programs": [{"channel": 0, "bank": 0, "program": 0}],
			"rules": [
				{"rule_id": "r1", "type": "velocity_curve", "range": ["C-1", "C8"], "curve": "exponential", "target_channel": 0}
			],
			"next_profile_trigger": {"type": "cc", "cc_number": 66, "cc_value_min": 64}
		},
		{
			"profile_id": "p2",
			"name": "Rhodes",
			"soundfont_path": "/sounds/rhodes.sf2",
			"programs": [
				{"channel": 0, "bank": 0, "program": 4},
				{"channel": 1, "bank": 0, "program": 32}
			],
			"rules": [
				{"rule_id": "r1", "type": "key_split", "range": ["C-1", "B2"], "target_channel": 1},
				{"rule_id": "r2", "type": "transpose", "range": ["C-1", "B2"], "value": 12, "target_channel": 1}
			],
			"next_profile_trigger": {"type": "cc", "cc_number": 66, "cc_value_min": 64}
		}
	]
}`

func TestLoadSetlist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "setlist.json")
	os.WriteFile(path, []byte(testJSON), 0644)

	sl, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if sl.ID != "test1" {
		t.Errorf("ID = %q", sl.ID)
	}
	if len(sl.Profiles) != 2 {
		t.Fatalf("len(Profiles) = %d, want 2", len(sl.Profiles))
	}
	p := sl.Profiles[0]
	if p.Name != "Piano" {
		t.Errorf("Profiles[0].Name = %q", p.Name)
	}
	if len(p.Rules) != 1 {
		t.Fatalf("len(Rules) = %d", len(p.Rules))
	}
	if p.Rules[0].Type != "velocity_curve" {
		t.Errorf("Rules[0].Type = %q", p.Rules[0].Type)
	}
	if p.NextTrigger == nil {
		t.Fatal("NextTrigger should not be nil")
	}
	if p.NextTrigger.CCNumber != 66 {
		t.Errorf("Trigger.CCNumber = %d", p.NextTrigger.CCNumber)
	}
}

func TestLoadSetlistValidation(t *testing.T) {
	// Empty profiles
	_, err := Load([]byte(`{"setlist_id": "x", "name": "x", "profiles": []}`))
	if err == nil {
		t.Error("empty profiles should fail validation")
	}
}

func TestLoadExampleSetlist(t *testing.T) {
	data, err := os.ReadFile("../../setlists/example_setlist.json")
	if err != nil {
		t.Skip("example setlist not found")
	}
	sl, err := Load(data)
	if err != nil {
		t.Fatalf("example setlist invalid: %v", err)
	}
	if len(sl.Profiles) != 3 {
		t.Errorf("expected 3 profiles, got %d", len(sl.Profiles))
	}
}
