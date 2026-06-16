package midi

import "testing"

func TestEventTypeConstants(t *testing.T) {
	if NoteOn != 0x90 {
		t.Errorf("NoteOn = %#x, want 0x90", NoteOn)
	}
	if NoteOff != 0x80 {
		t.Errorf("NoteOff = %#x, want 0x80", NoteOff)
	}
	if CC != 0xB0 {
		t.Errorf("CC = %#x, want 0xB0", CC)
	}
}

func TestEventIsNote(t *testing.T) {
	note := Event{Type: NoteOn, Channel: 0, Key: 60, Value: 100}
	if !note.IsNote() {
		t.Error("NoteOn should be a note event")
	}
	cc := Event{Type: CC, Channel: 0, Key: 64, Value: 127}
	if cc.IsNote() {
		t.Error("CC should not be a note event")
	}
}
