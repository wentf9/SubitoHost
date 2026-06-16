package router

import "github.com/wentf9/subitohost/internal/midi"

// KeySplitRule routes notes within Range to TargetChannel.
// Notes outside the range and non-note events pass through unchanged.
type KeySplitRule struct {
	Range         midi.NoteRange
	TargetChannel int
}

func (r *KeySplitRule) Apply(ev midi.Event) []midi.Event {
	if ev.IsNote() && r.Range.Contains(ev.Key) {
		ev.Channel = r.TargetChannel
	}
	return []midi.Event{ev}
}
