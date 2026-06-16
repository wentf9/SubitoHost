package midi

// EventType represents the MIDI status byte (high nibble).
type EventType byte

const (
	NoteOff EventType = 0x80
	NoteOn  EventType = 0x90
	CC      EventType = 0xB0
)

// Event is a parsed MIDI event.
// For NoteOn/NoteOff: Key = note number (0-127), Value = velocity (0-127).
// For CC: Key = controller number (0-127), Value = controller value (0-127).
type Event struct {
	Type    EventType
	Channel int
	Key     int
	Value   int
}

// IsNote returns true if the event is NoteOn or NoteOff.
func (e Event) IsNote() bool {
	return e.Type == NoteOn || e.Type == NoteOff
}
