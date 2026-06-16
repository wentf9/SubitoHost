package router

import "github.com/wentf9/subitohost/internal/midi"

// TransposeRule shifts note numbers by Semitones.
// Channel: -1 = apply to all channels, >= 0 = only apply on that channel.
type TransposeRule struct {
	Range     midi.NoteRange
	Channel   int
	Semitones int
}

func (r *TransposeRule) Apply(ev midi.Event) []midi.Event {
	if !ev.IsNote() || !r.Range.Contains(ev.Key) {
		return []midi.Event{ev}
	}
	if r.Channel >= 0 && ev.Channel != r.Channel {
		return []midi.Event{ev}
	}
	ev.Key = clamp(ev.Key+r.Semitones, 0, 127)
	return []midi.Event{ev}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
