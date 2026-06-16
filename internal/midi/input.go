// internal/midi/input.go
package midi

import (
	"fmt"
	"strings"
)

// DeviceInfo describes a MIDI device.
type DeviceInfo struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	IsInput bool   `json:"is_input"`
	Path    string `json:"-"` // Linux device node path, e.g. /dev/snd/midiC3D0
}

// FindInputByName returns the first input device whose name contains pattern.
func FindInputByName(pattern string) (*DeviceInfo, error) {
	devices, err := ListDevices()
	if err != nil {
		return nil, err
	}
	for _, d := range devices {
		if d.IsInput && strings.Contains(strings.ToLower(d.Name), strings.ToLower(pattern)) {
			return &d, nil
		}
	}
	return nil, fmt.Errorf("no MIDI input matching %q", pattern)
}
