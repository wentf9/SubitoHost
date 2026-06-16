package recorder

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wentf9/subitohost/internal/midi"
)

func TestSMFWriterHeader(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.mid")

	w := NewSMFWriter()
	if err := w.Flush(path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data[0:4]) != "MThd" {
		t.Errorf("chunk type = %q, want MThd", string(data[0:4]))
	}
	if binary.BigEndian.Uint32(data[4:8]) != 6 {
		t.Errorf("header length = %d, want 6", binary.BigEndian.Uint32(data[4:8]))
	}
	if binary.BigEndian.Uint16(data[8:10]) != 0 {
		t.Errorf("format = %d, want 0", binary.BigEndian.Uint16(data[8:10]))
	}
	if binary.BigEndian.Uint16(data[12:14]) != 480 {
		t.Errorf("ticks = %d, want 480", binary.BigEndian.Uint16(data[12:14]))
	}
	if string(data[14:18]) != "MTrk" {
		t.Errorf("track chunk = %q, want MTrk", string(data[14:18]))
	}
}

func TestSMFWriterEndOfTrack(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.mid")

	w := NewSMFWriter()
	w.AddEvent(midi.Event{Type: midi.NoteOn, Channel: 0, Key: 60, Value: 100})
	time.Sleep(5 * time.Millisecond)
	w.AddEvent(midi.Event{Type: midi.NoteOff, Channel: 0, Key: 60, Value: 0})

	if err := w.Flush(path); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	trackLen := binary.BigEndian.Uint32(data[18:22])
	if trackLen == 0 {
		t.Error("track length should be > 0 with events")
	}
	// Last 3 bytes of track must be the end-of-track meta event: FF 2F 00
	last3 := data[22+trackLen-3 : 22+trackLen]
	if last3[0] != 0xFF || last3[1] != 0x2F || last3[2] != 0x00 {
		t.Errorf("end-of-track = % X, want FF 2F 00", last3)
	}
}

func TestEncodeVarLen(t *testing.T) {
	cases := []struct {
		in  uint32
		out []byte
	}{
		{0, []byte{0x00}},
		{127, []byte{0x7F}},
		{128, []byte{0x81, 0x00}},
		{255, []byte{0x81, 0x7F}},
		{16383, []byte{0xFF, 0x7F}},
		{16384, []byte{0x81, 0x80, 0x00}},
	}
	for _, c := range cases {
		got := encodeVarLen(c.in)
		if string(got) != string(c.out) {
			t.Errorf("encodeVarLen(%d) = % X, want % X", c.in, got, c.out)
		}
	}
}
