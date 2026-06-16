package midi

import "testing"

func TestParseNoteName(t *testing.T) {
	tests := []struct {
		name string
		want int
	}{
		{"C-1", 0},
		{"C#-1", 1},
		{"D-1", 2},
		{"B-1", 11},
		{"C0", 12},
		{"A0", 21},
		{"C4", 60}, // middle C
		{"A4", 69}, // concert A (440Hz)
		{"C8", 108},
		{"G9", 127},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseNoteName(tt.name)
			if err != nil {
				t.Fatalf("ParseNoteName(%q) error: %v", tt.name, err)
			}
			if got != tt.want {
				t.Errorf("ParseNoteName(%q) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func TestParseNoteNameErrors(t *testing.T) {
	bad := []string{"", "X4", "C", "C10", "Cb-2"}
	for _, name := range bad {
		_, err := ParseNoteName(name)
		if err == nil {
			t.Errorf("ParseNoteName(%q) should return error", name)
		}
	}
}

func TestNoteRangeContains(t *testing.T) {
	r, err := ParseNoteRange("C-1", "B2")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Contains(0) {
		t.Error("range should contain note 0 (C-1)")
	}
	if !r.Contains(47) {
		t.Error("range should contain note 47 (B2)")
	}
	if r.Contains(48) {
		t.Error("range should not contain note 48 (C3)")
	}
}
