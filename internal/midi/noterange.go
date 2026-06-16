package midi

import (
	"fmt"
	"strconv"
	"strings"
)

var semitones = map[byte]int{
	'C': 0, 'D': 2, 'E': 4, 'F': 5, 'G': 7, 'A': 9, 'B': 11,
}

// ParseNoteName converts a note name like "C4", "C#-1", "Bb3" to a MIDI note number (0-127).
// Convention: C-1 = 0, C4 = 60 (middle C), G9 = 127.
func ParseNoteName(name string) (int, error) {
	if len(name) < 2 {
		return 0, fmt.Errorf("invalid note name: %q", name)
	}

	letter := strings.ToUpper(name)[0]
	base, ok := semitones[letter]
	if !ok {
		return 0, fmt.Errorf("invalid note letter: %c", letter)
	}

	rest := name[1:]
	if len(rest) > 0 && rest[0] == '#' {
		base++
		rest = rest[1:]
	} else if len(rest) > 0 && rest[0] == 'b' {
		base--
		rest = rest[1:]
	}

	octave, err := strconv.Atoi(rest)
	if err != nil {
		return 0, fmt.Errorf("invalid octave in %q: %v", name, err)
	}

	note := (octave+1)*12 + base
	if note < 0 || note > 127 {
		return 0, fmt.Errorf("note %q out of MIDI range (0-127): %d", name, note)
	}
	return note, nil
}

// NoteRange represents an inclusive range of MIDI note numbers.
type NoteRange struct {
	Low  int
	High int
}

// ParseNoteRange creates a NoteRange from two note names.
func ParseNoteRange(low, high string) (NoteRange, error) {
	l, err := ParseNoteName(low)
	if err != nil {
		return NoteRange{}, fmt.Errorf("low bound: %w", err)
	}
	h, err := ParseNoteName(high)
	if err != nil {
		return NoteRange{}, fmt.Errorf("high bound: %w", err)
	}
	if l > h {
		return NoteRange{}, fmt.Errorf("low (%d) > high (%d)", l, h)
	}
	return NoteRange{Low: l, High: h}, nil
}

// Contains returns true if note is within the range (inclusive).
func (r NoteRange) Contains(note int) bool {
	return note >= r.Low && note <= r.High
}
