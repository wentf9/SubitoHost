// internal/midi/input_test.go
//go:build cgo && integration

package midi

import "testing"

func TestListDevices(t *testing.T) {
	devices, err := ListDevices()
	if err != nil {
		t.Skipf("portmidi not available: %v", err)
	}
	t.Logf("found %d MIDI devices", len(devices))
	for _, d := range devices {
		dir := "output"
		if d.IsInput {
			dir = "input"
		}
		t.Logf("  [%d] %s (%s)", d.ID, d.Name, dir)
	}
}
