package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type RecoveryState struct {
	SetlistPath  string `json:"setlist_path"`
	CurrentIndex int    `json:"current_index"`
}

func SaveRecovery(path string, state RecoveryState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	return os.Rename(tmp, path)
}

func LoadRecovery(path string) (*RecoveryState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state RecoveryState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}
