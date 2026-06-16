package router

import "github.com/wentf9/subitohost/internal/midi"

// Rule processes a single MIDI event and returns zero or more output events.
type Rule interface {
	Apply(ev midi.Event) []midi.Event
}
