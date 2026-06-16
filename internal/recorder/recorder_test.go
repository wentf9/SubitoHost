package recorder

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wentf9/subitohost/internal/config"
	"github.com/wentf9/subitohost/internal/midi"
	"github.com/wentf9/subitohost/internal/setlist"
	"github.com/wentf9/subitohost/internal/util"
)

func makeTestProfile() *setlist.Profile {
	return &setlist.Profile{Name: "Test Profile", SFPath: "/fake.sf2"}
}

func TestRecorderStartStop(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.Recording{OutputDir: tmp}
	r := New(cfg, config.Audio{SampleRate: 48000, BufferSize: 128})

	if err := r.Start(makeTestProfile()); err != nil {
		t.Fatal(err)
	}
	midPath, wavPath, err := r.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(midPath); err != nil {
		t.Errorf("mid file not created: %v", err)
	}
	if wavPath == "" {
		t.Error("wavPath should be non-empty")
	}
	if filepath.Ext(midPath) != ".mid" {
		t.Errorf("midPath ext = %q, want .mid", filepath.Ext(midPath))
	}
	if filepath.Ext(wavPath) != ".wav" {
		t.Errorf("wavPath ext = %q, want .wav", filepath.Ext(wavPath))
	}
}

func TestRecorderFeedWritesEvents(t *testing.T) {
	tmp := t.TempDir()
	r := New(config.Recording{OutputDir: tmp}, config.Audio{SampleRate: 48000})

	if err := r.Start(makeTestProfile()); err != nil {
		t.Fatal(err)
	}
	r.Feed(midi.Event{Type: midi.NoteOn, Channel: 0, Key: 60, Value: 100})
	time.Sleep(5 * time.Millisecond)
	r.Feed(midi.Event{Type: midi.NoteOff, Channel: 0, Key: 60, Value: 0})

	midPath, _, err := r.Stop()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(midPath)
	if err != nil {
		t.Fatal(err)
	}
	// An SMF with 2 events is at least ~30 bytes (14-byte header + events + end-of-track)
	if info.Size() < 30 {
		t.Errorf("mid file size %d suspiciously small for 2 events", info.Size())
	}
}

func TestRecorderStopBeforeStart(t *testing.T) {
	r := New(config.Recording{}, config.Audio{})
	_, _, err := r.Stop()
	if err == nil {
		t.Error("Stop before Start should return error")
	}
}

func TestRecorderFeedNoopWhenStopped(t *testing.T) {
	r := New(config.Recording{}, config.Audio{})
	// Should not panic
	r.Feed(midi.Event{Type: midi.NoteOn, Key: 60, Value: 100})
}

func TestSanitizeName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Hello World", "Hello_World"},
		{"Song/1!", "Song1"},
		{"   ", "recording"},
		{"Opening", "Opening"},
	}
	for _, c := range cases {
		got := sanitizeName(c.in)
		if got != c.want {
			t.Errorf("sanitizeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := util.ExpandHome("~/recs"); got != home+"/recs" {
		t.Errorf("expandHome(~/recs) = %q, want %s/recs", got, home)
	}
	if got := util.ExpandHome("/abs"); got != "/abs" {
		t.Errorf("expandHome(/abs) = %q, want /abs", got)
	}
}
