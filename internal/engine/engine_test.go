package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wentf9/subitohost/internal/config"
	"github.com/wentf9/subitohost/internal/midi"
)

func TestRecoverySaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	state := RecoveryState{SetlistPath: "/my/setlist.json", CurrentIndex: 3}
	if err := SaveRecovery(path, state); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadRecovery(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SetlistPath != "/my/setlist.json" || loaded.CurrentIndex != 3 {
		t.Errorf("loaded = %+v", loaded)
	}
}

func TestRecoveryLoadMissing(t *testing.T) {
	_, err := LoadRecovery("/nonexistent/state.json")
	if err == nil {
		t.Error("should return error")
	}
}

func TestRecoveryAtomicity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	SaveRecovery(path, RecoveryState{SetlistPath: "/a.json", CurrentIndex: 0})
	SaveRecovery(path, RecoveryState{SetlistPath: "/b.json", CurrentIndex: 1})
	_, err := os.Stat(path + ".tmp")
	if err == nil {
		t.Error("tmp file should not exist after successful rename")
	}
	loaded, _ := LoadRecovery(path)
	if loaded.CurrentIndex != 1 {
		t.Errorf("CurrentIndex = %d, want 1", loaded.CurrentIndex)
	}
}

func TestInjectNote_Output(t *testing.T) {
	cfg := &config.Config{}
	e := New(cfg)

	ch := e.Subscribe()
	defer e.Unsubscribe(ch)

	e.InjectNote("note_on", 60, 100, "output")
	inj := <-e.injected
	e.processInjectedNote(inj.action, inj.key, inj.vel, inj.target)

	select {
	case ev := <-ch:
		if ev.Type != "note_on" {
			t.Errorf("expected note_on event, got %v", ev.Type)
		}
	default:
		t.Errorf("expected broadcast event")
	}

	midiEv, ok := e.ring.Read()
	if !ok {
		t.Errorf("expected midi event in ring buffer")
	} else if midiEv.Type != midi.NoteOn || midiEv.Key != 60 || midiEv.Value != 100 {
		t.Errorf("unexpected midi event: %+v", midiEv)
	}
}

func TestStartRecordingWithoutSetlist(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{
		Recording: config.Recording{OutputDir: tmp},
		Audio:     config.Audio{SampleRate: 48000, BufferSize: 128},
	}
	e := New(cfg)
	if err := e.StartRecording(); err == nil {
		t.Error("StartRecording without setlist should return error")
	}
	if e.IsRecording() {
		t.Error("IsRecording should be false after failed start")
	}
}

func TestStopRecordingNotRecording(t *testing.T) {
	e := New(&config.Config{})
	_, err := e.StopRecording()
	if err == nil {
		t.Error("StopRecording when not recording should return error")
	}
}

func TestRecordingStatusIdle(t *testing.T) {
	e := New(&config.Config{})
	status, startedAt := e.RecordingStatus()
	if status != "idle" {
		t.Errorf("initial status = %q, want idle", status)
	}
	if !startedAt.IsZero() {
		t.Error("startedAt should be zero when idle")
	}
}
