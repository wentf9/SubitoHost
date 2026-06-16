package router

import "github.com/wentf9/subitohost/internal/midi"

// ChordExpandRule expands a single note into a chord based on Intervals.
// Intervals are semitone offsets from the root, e.g. [0, 7, 12] = root + 5th + octave.
// Notes that would exceed MIDI range (0-127) are omitted.
type ChordExpandRule struct {
	Range     midi.NoteRange
	Channel   int // -1 = any
	Intervals []int
}

func (r *ChordExpandRule) Apply(ev midi.Event) []midi.Event {
	if !ev.IsNote() || !r.Range.Contains(ev.Key) {
		return []midi.Event{ev}
	}
	if r.Channel >= 0 && ev.Channel != r.Channel {
		return []midi.Event{ev}
	}
	result := make([]midi.Event, 0, len(r.Intervals))
	for _, interval := range r.Intervals {
		note := ev.Key + interval
		if note < 0 || note > 127 {
			continue
		}
		out := ev
		out.Key = note
		result = append(result, out)
	}
	if len(result) == 0 {
		return []midi.Event{ev} // fallback: return original if all intervals out of range
	}
	return result
}
