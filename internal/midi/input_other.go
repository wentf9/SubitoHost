//go:build !linux
// +build !linux

package midi

import (
	"errors"
)

// Input is a dummy implementation for non-Linux platforms.
type Input struct{}

// ListDevices is a dummy implementation.
func ListDevices() ([]DeviceInfo, error) {
	return nil, errors.New("MIDI input is only supported on Linux in this implementation")
}

// OpenInput is a dummy implementation.
func OpenInput(deviceID int) (*Input, error) {
	return nil, errors.New("MIDI input is only supported on Linux in this implementation")
}

// Poll is a dummy implementation.
func (in *Input) Poll() bool {
	return false
}

// Read is a dummy implementation.
func (in *Input) Read() ([]Event, error) {
	return nil, nil
}

// Close is a dummy implementation.
func (in *Input) Close() error {
	return nil
}
