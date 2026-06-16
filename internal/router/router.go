package router

import "github.com/wentf9/subitohost/internal/midi"

// Trigger defines a MIDI condition for profile switching.
type Trigger struct {
	CCNumber   int
	CCValueMin int
}

func (t *Trigger) matches(ev midi.Event) bool {
	return ev.Type == midi.CC && ev.Key == t.CCNumber && ev.Value >= t.CCValueMin
}

// ProcessResult contains the output events and whether a profile switch was triggered.
type ProcessResult struct {
	Events    []midi.Event
	Triggered bool
}

// Router processes MIDI events through a rule chain and detects profile switch triggers.
type Router struct {
	rules   []Rule
	trigger *Trigger
}

// New creates a router with the given rules and optional trigger.
func New(rules []Rule, trigger *Trigger) *Router {
	return &Router{rules: rules, trigger: trigger}
}

// Process runs a MIDI event through the rule chain.
// If the event matches the trigger, it is consumed and Triggered is set.
func (r *Router) Process(ev midi.Event) ProcessResult {
	// Check trigger first — consume the event if matched
	if r.trigger != nil && r.trigger.matches(ev) {
		return ProcessResult{Triggered: true}
	}

	// Run through rule chain
	events := []midi.Event{ev}
	for _, rule := range r.rules {
		var next []midi.Event
		for _, e := range events {
			next = append(next, rule.Apply(e)...)
		}
		events = next
		if len(events) == 0 {
			break
		}
	}
	return ProcessResult{Events: events}
}
