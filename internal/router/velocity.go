package router

import (
	"math"

	"github.com/wentf9/subitohost/internal/midi"
)

type CurveType int

const (
	CurveLinear      CurveType = iota
	CurveExponential           // (v/127)^2 * 127 — compresses dynamics
	CurveLogarithmic           // sqrt(v/127) * 127 — expands dynamics
)

// VelocityCurveRule transforms NoteOn velocity using a curve function.
// Only NoteOn events are affected; NoteOff passes through unchanged.
type VelocityCurveRule struct {
	Range   midi.NoteRange
	Channel int // -1 = any
	Curve   CurveType
}

func (r *VelocityCurveRule) Apply(ev midi.Event) []midi.Event {
	if ev.Type != midi.NoteOn || !r.Range.Contains(ev.Key) {
		return []midi.Event{ev}
	}
	if r.Channel >= 0 && ev.Channel != r.Channel {
		return []midi.Event{ev}
	}
	ev.Value = r.apply(ev.Value)
	return []midi.Event{ev}
}

func (r *VelocityCurveRule) apply(v int) int {
	f := float64(v) / 127.0
	var out float64
	switch r.Curve {
	case CurveLinear:
		out = f
	case CurveExponential:
		out = f * f
	case CurveLogarithmic:
		out = math.Sqrt(f)
	default:
		out = f
	}
	result := int(math.Round(out * 127.0))
	if result < 0 {
		return 0
	}
	if result > 127 {
		return 127
	}
	return result
}
