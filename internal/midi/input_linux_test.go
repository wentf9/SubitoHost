//go:build linux

package midi

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func TestParseRawMIDIPortNameMatchesALSAListing(t *testing.T) {
	proc := "Roland Digital Piano\n\nType: Legacy\nOutput 0\nInput 0\n"
	if got, want := parseRawMIDIPortName(proc), "Roland Digital Piano MIDI 1"; got != want {
		t.Fatalf("parseRawMIDIPortName() = %q, want %q", got, want)
	}
}

func TestParseRawMIDIPortNameEmpty(t *testing.T) {
	if got := parseRawMIDIPortName("\nType: Legacy\n"); got != "" {
		t.Fatalf("parseRawMIDIPortName() = %q, want empty", got)
	}
}

func TestInputReadDrainsAvailableMIDIWithoutBlocking(t *testing.T) {
	in, writer := newPipeInput(t)
	writeMIDIBytes(t, writer, 0x90, 0x34, 0x32, 0x80, 0x34, 0x67)

	type result struct {
		events []Event
		err    error
	}
	done := make(chan result, 1)
	go func() {
		events, err := in.Read()
		done <- result{events: events, err: err}
	}()

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("Read() error = %v", got.err)
		}
		want := []Event{
			{Type: NoteOn, Channel: 0, Key: 0x34, Value: 0x32},
			{Type: NoteOff, Channel: 0, Key: 0x34, Value: 0x67},
		}
		assertEvents(t, got.events, want)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Read() blocked after draining the currently available MIDI bytes")
	}
}

func TestInputReadPreservesRunningStatus(t *testing.T) {
	in, writer := newPipeInput(t)
	writeMIDIBytes(t, writer, 0x90, 0x3C, 0x40, 0x3E, 0x41)

	events, err := in.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	want := []Event{
		{Type: NoteOn, Channel: 0, Key: 0x3C, Value: 0x40},
		{Type: NoteOn, Channel: 0, Key: 0x3E, Value: 0x41},
	}
	assertEvents(t, events, want)
}

func TestInputReadPreservesSysExStateAcrossReads(t *testing.T) {
	in, writer := newPipeInput(t)
	writeMIDIBytes(t, writer, 0xF0, 0x01, 0x02)

	events, err := in.Read()
	if err != nil {
		t.Fatalf("first Read() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("first Read() events = %v, want none", events)
	}

	writeMIDIBytes(t, writer, 0x03, 0xF7, 0x90, 0x40, 0x50)
	events, err = in.Read()
	if err != nil {
		t.Fatalf("second Read() error = %v", err)
	}
	assertEvents(t, events, []Event{
		{Type: NoteOn, Channel: 0, Key: 0x40, Value: 0x50},
	})
}

func newPipeInput(t *testing.T) (*Input, *os.File) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	fd := int(reader.Fd())
	if err := syscall.SetNonblock(fd, true); err != nil {
		_ = reader.Close()
		_ = writer.Close()
		t.Fatalf("SetNonblock() error = %v", err)
	}

	in := &Input{
		file:    reader,
		fd:      fd,
		epollFd: -1,
		buf:     make([]Event, 0),
	}
	t.Cleanup(func() {
		_ = in.Close()
		_ = writer.Close()
	})
	return in, writer
}

func writeMIDIBytes(t *testing.T, writer *os.File, data ...byte) {
	t.Helper()
	if _, err := writer.Write(data); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
}

func assertEvents(t *testing.T, got, want []Event) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
