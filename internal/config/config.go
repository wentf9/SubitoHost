package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/wentf9/subitohost/internal/util"
)

type Config struct {
	ListenAddr string    `json:"listen_addr"`
	Audio      Audio     `json:"audio"`
	MIDI       MIDI      `json:"midi"`
	Recovery   Recovery  `json:"recovery"`
	Recording  Recording `json:"recording"`
}

type Audio struct {
	Backend    string `json:"backend"`
	SampleRate int    `json:"sample_rate"`
	BufferSize int    `json:"buffer_size"`
	Periods    int    `json:"periods"`
}

type MIDI struct {
	AutoConnect       bool   `json:"auto_connect"`
	DeviceNamePattern string `json:"device_name_pattern"`
}

type Recovery struct {
	StateFile  string `json:"state_file"`
	AutoResume bool   `json:"auto_resume"`
}

type Recording struct {
	OutputDir string `json:"output_dir"`
	TriggerCC int    `json:"trigger_cc"`
}

func Default() *Config {
	return &Config{
		ListenAddr: "127.0.0.1:3301",
		Audio: Audio{
			Backend:    "alsa",
			SampleRate: 48000,
			BufferSize: 128,
			Periods:    2,
		},
		MIDI: MIDI{
			AutoConnect: true,
		},
		Recovery: Recovery{
			StateFile:  "~/.config/subitohost/state.json",
			AutoResume: true,
		},
		Recording: Recording{
			OutputDir: "~/recordings",
			TriggerCC: 0,
		},
	}
}

func (c *Config) ExpandPaths() {
	c.Recovery.StateFile = util.ExpandHome(c.Recovery.StateFile)
	c.Recording.OutputDir = util.ExpandHome(c.Recording.OutputDir)
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}
