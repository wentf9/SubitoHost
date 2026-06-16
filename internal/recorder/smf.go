package recorder

import (
	"encoding/binary"
	"os"
	"time"

	"github.com/wentf9/subitohost/internal/midi"
)

const (
	smfPPQN  = 480    // ticks per quarter note
	smfTempo = 500000 // µs per quarter note at 120 BPM
)

type smfEvent struct {
	ts time.Duration
	ev midi.Event
}

// SMFWriter buffers timestamped MIDI events and serializes to SMF format 0.
type SMFWriter struct {
	start  time.Time
	events []smfEvent
}

// NewSMFWriter creates an SMFWriter; all AddEvent calls are timestamped from now.
func NewSMFWriter() *SMFWriter {
	return &SMFWriter{start: time.Now()}
}

// AddEvent appends a MIDI event timestamped relative to NewSMFWriter.
func (w *SMFWriter) AddEvent(ev midi.Event) {
	w.events = append(w.events, smfEvent{ts: time.Since(w.start), ev: ev})
}

// Flush serializes buffered events to a format-0 SMF file at path.
func (w *SMFWriter) Flush(path string) error {
	track := buildTrack(w.events)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	writeBytes := func(b []byte) error {
		_, err := f.Write(b)
		return err
	}
	writeBE := func(v any) error {
		return binary.Write(f, binary.BigEndian, v)
	}

	// MThd header chunk
	if err := writeBytes([]byte("MThd")); err != nil {
		return err
	}
	if err := writeBE(uint32(6)); err != nil {
		return err
	}
	if err := writeBE(uint16(0)); err != nil { // format 0
		return err
	}
	if err := writeBE(uint16(1)); err != nil { // 1 track
		return err
	}
	if err := writeBE(uint16(smfPPQN)); err != nil {
		return err
	}

	// MTrk track chunk
	if err := writeBytes([]byte("MTrk")); err != nil {
		return err
	}
	if err := writeBE(uint32(len(track))); err != nil {
		return err
	}
	if err := writeBytes(track); err != nil {
		return err
	}
	return nil
}

func buildTrack(events []smfEvent) []byte {
	var buf []byte
	var prevTicks int64
	for _, e := range events {
		b := midiBytes(e.ev)
		if len(b) == 0 {
			continue // skip unsupported event types entirely
		}
		ticks := e.ts.Microseconds() * smfPPQN / smfTempo
		delta := ticks - prevTicks
		if delta < 0 {
			delta = 0
		}
		prevTicks = ticks
		buf = append(buf, encodeVarLen(uint32(delta))...)
		buf = append(buf, b...)
	}
	// End-of-track meta event: delta=0, FF 2F 00
	buf = append(buf, 0x00, 0xFF, 0x2F, 0x00)
	return buf
}

func midiBytes(ev midi.Event) []byte {
	status := byte(ev.Type) | byte(ev.Channel)
	switch ev.Type {
	case midi.NoteOn:
		return []byte{status, byte(ev.Key), byte(ev.Value)}
	case midi.NoteOff:
		return []byte{status, byte(ev.Key), 0}
	case midi.CC:
		return []byte{status, byte(ev.Key), byte(ev.Value)}
	default:
		return nil
	}
}

// encodeVarLen encodes v as an SMF variable-length quantity.
// v must be <= 0x0FFFFFFF (SMF allows at most 4 bytes per VLQ value).
func encodeVarLen(v uint32) []byte {
	if v < 0x80 {
		return []byte{byte(v)}
	}
	var buf []byte
	for v > 0 {
		buf = append([]byte{byte(v & 0x7F)}, buf...)
		v >>= 7
	}
	for i := 0; i < len(buf)-1; i++ {
		buf[i] |= 0x80
	}
	return buf
}
