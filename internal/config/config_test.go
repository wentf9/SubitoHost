package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{
		"listen_addr": "127.0.0.1:4000",
		"audio": {"backend": "jack", "sample_rate": 44100, "buffer_size": 256, "periods": 2},
		"midi": {"auto_connect": false, "device_name_pattern": "Roland"},
		"recovery": {"state_file": "/tmp/state.json", "auto_resume": false}
	}`
	os.WriteFile(path, []byte(data), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != "127.0.0.1:4000" {
		t.Errorf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.Audio.Backend != "jack" {
		t.Errorf("Audio.Backend = %q", cfg.Audio.Backend)
	}
	if cfg.Audio.SampleRate != 44100 {
		t.Errorf("Audio.SampleRate = %d", cfg.Audio.SampleRate)
	}
	if cfg.MIDI.DeviceNamePattern != "Roland" {
		t.Errorf("MIDI.DeviceNamePattern = %q", cfg.MIDI.DeviceNamePattern)
	}
}

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.ListenAddr != "127.0.0.1:3301" {
		t.Errorf("default ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.Audio.SampleRate != 48000 {
		t.Errorf("default SampleRate = %d", cfg.Audio.SampleRate)
	}
	if cfg.Audio.BufferSize != 128 {
		t.Errorf("default BufferSize = %d", cfg.Audio.BufferSize)
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := Load("/nonexistent/config.json")
	if err == nil {
		t.Error("should return error for missing file")
	}
}

func TestDefaultRecording(t *testing.T) {
	cfg := Default()
	if cfg.Recording.OutputDir != "~/recordings" {
		t.Errorf("Recording.OutputDir = %q, want ~/recordings", cfg.Recording.OutputDir)
	}
	if cfg.Recording.TriggerCC != 0 {
		t.Errorf("Recording.TriggerCC = %d, want 0", cfg.Recording.TriggerCC)
	}
}

func TestLoadRecordingConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"recording": {"output_dir": "/tmp/rec", "trigger_cc": 64}}`), 0644)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Recording.OutputDir != "/tmp/rec" {
		t.Errorf("OutputDir = %q, want /tmp/rec", cfg.Recording.OutputDir)
	}
	if cfg.Recording.TriggerCC != 64 {
		t.Errorf("TriggerCC = %d, want 64", cfg.Recording.TriggerCC)
	}
}
