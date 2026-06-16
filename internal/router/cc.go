package router

import (
	"slices"

	"github.com/wentf9/subitohost/internal/midi"
)

// CCFilterRule drops CC events whose controller number is in BlockCCs.
type CCFilterRule struct {
	BlockCCs []int
}

func (r *CCFilterRule) Apply(ev midi.Event) []midi.Event {
	if ev.Type != midi.CC {
		return []midi.Event{ev}
	}
	if slices.Contains(r.BlockCCs, ev.Key) {
		return nil
	}
	return []midi.Event{ev}
}

// CCMapRule remaps one CC number to another.
type CCMapRule struct {
	FromCC int
	ToCC   int
}

func (r *CCMapRule) Apply(ev midi.Event) []midi.Event {
	if ev.Type == midi.CC && ev.Key == r.FromCC {
		ev.Key = r.ToCC
	}
	return []midi.Event{ev}
}
