package setlist

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadFile reads and parses a setlist JSON file.
func LoadFile(path string) (*Setlist, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read setlist: %w", err)
	}
	return Load(data)
}

// Load parses setlist JSON bytes.
func Load(data []byte) (*Setlist, error) {
	var sl Setlist
	if err := json.Unmarshal(data, &sl); err != nil {
		return nil, fmt.Errorf("parse setlist: %w", err)
	}
	if err := validate(&sl); err != nil {
		return nil, err
	}
	return &sl, nil
}

func validate(sl *Setlist) error {
	if len(sl.Profiles) == 0 {
		return fmt.Errorf("setlist %q has no profiles", sl.ID)
	}
	for i, p := range sl.Profiles {
		if p.SFPath == "" {
			return fmt.Errorf("profile %d (%q): soundfont_path is required", i, p.ID)
		}
		if len(p.Programs) == 0 {
			return fmt.Errorf("profile %d (%q): at least one program is required", i, p.ID)
		}
	}
	return nil
}
