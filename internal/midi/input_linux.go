//go:build linux
// +build linux

package midi

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Input wraps a Linux raw MIDI device.
type Input struct {
	file       *os.File
	eventsChan chan Event
	err        error
	closed     bool
	mu         sync.Mutex
	buf        []Event
}

// ListDevices returns all available MIDI devices by scanning /dev/snd/midiC*D*.
func ListDevices() ([]DeviceInfo, error) {
	paths, err := filepath.Glob("/dev/snd/midiC*D*")
	if err != nil {
		return nil, err
	}

	cardNames := parseCards()

	devices := make([]DeviceInfo, 0, len(paths))
	for i, path := range paths {
		// Extract card and device numbers.
		// e.g. /dev/snd/midiC3D0 -> card 3, device 0
		base := filepath.Base(path)
		var card, device int
		_, err := fmt.Sscanf(base, "midiC%dD%d", &card, &device)
		if err != nil {
			continue
		}

		name, ok := cardNames[card]
		if !ok {
			// Try to read card ID from sysfs.
			sysPath := fmt.Sprintf("/sys/class/sound/card%d/id", card)
			if data, err := os.ReadFile(sysPath); err == nil {
				name = strings.TrimSpace(string(data))
			} else {
				name = fmt.Sprintf("ALSA MIDI Card %d Dev %d", card, device)
			}
		} else {
			name = fmt.Sprintf("%s (MIDI %d:%d)", name, card, device)
		}

		devices = append(devices, DeviceInfo{
			ID:      i,
			Name:    name,
			IsInput: true,
			Path:    path,
		})
	}
	return devices, nil
}

// parseCards parses /proc/asound/cards to get friendly names.
func parseCards() map[int]string {
	cardNames := make(map[int]string)
	content, err := os.ReadFile("/proc/asound/cards")
	if err != nil {
		return cardNames
	}

	lines := strings.Split(string(content), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		// Match lines like " 3 [M2             ]: USB-Audio - MiniFuse 2"
		if !strings.Contains(line, "[") || !strings.Contains(line, "]") {
			continue
		}
		parts := strings.SplitN(line, "[", 2)
		if len(parts) < 2 {
			continue
		}
		cardNumStr := strings.TrimSpace(parts[0])
		cardNum, err := strconv.Atoi(cardNumStr)
		if err != nil {
			continue
		}

		// First line after colon.
		subparts := strings.SplitN(parts[1], "]:", 2)
		var name string
		if len(subparts) >= 2 {
			name = strings.TrimSpace(subparts[1])
		}

		// Try to see if there is a second line with a long name.
		if i+1 < len(lines) {
			nextLine := lines[i+1]
			if strings.HasPrefix(nextLine, "  ") || strings.HasPrefix(nextLine, "\t") {
				nextLineTrim := strings.TrimSpace(nextLine)
				if idx := strings.Index(nextLineTrim, " at "); idx != -1 {
					longName := nextLineTrim[:idx]
					if longName != "" {
						name = longName
					}
				}
			}
		}

		if name == "" {
			name = fmt.Sprintf("Card %d", cardNum)
		}
		cardNames[cardNum] = name
	}
	return cardNames
}

// OpenInput opens a MIDI input device by ID.
func OpenInput(deviceID int) (*Input, error) {
	devices, err := ListDevices()
	if err != nil {
		return nil, err
	}
	if deviceID < 0 || deviceID >= len(devices) {
		return nil, fmt.Errorf("invalid device ID %d (total %d devices)", deviceID, len(devices))
	}
	dev := devices[deviceID]

	file, err := os.Open(dev.Path)
	if err != nil {
		return nil, fmt.Errorf("open raw midi device %s failed: %w", dev.Path, err)
	}

	inp := &Input{
		file: file,
		buf:  make([]Event, 0),
	}
	go inp.readLoop()
	return inp, nil
}

// Poll returns true if there are events available.
func (in *Input) Poll() bool {
	in.mu.Lock()
	defer in.mu.Unlock()
	return len(in.buf) > 0
}

// Read reads available MIDI events, converting them to Event structs.
func (in *Input) Read() ([]Event, error) {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.err != nil {
		return nil, in.err
	}
	if len(in.buf) == 0 {
		return nil, nil
	}
	res := in.buf
	in.buf = make([]Event, 0)
	return res, nil
}

// Close closes the MIDI input stream.
func (in *Input) Close() error {
	in.mu.Lock()
	if in.closed {
		in.mu.Unlock()
		return nil
	}
	in.closed = true
	in.mu.Unlock()
	return in.file.Close()
}

// readLoop runs in background, parses raw MIDI bytes.
func (in *Input) readLoop() {
	reader := bufio.NewReader(in.file)

	var runningStatus byte
	var expectedDataLen int
	var dataBuffer []byte

	getDataLen := func(status byte) int {
		op := status & 0xF0
		switch op {
		case 0x80, 0x90, 0xA0, 0xB0, 0xE0:
			return 2
		case 0xC0, 0xD0:
			return 1
		default:
			return 0
		}
	}

	for {
		b, err := reader.ReadByte()
		if err != nil {
			in.mu.Lock()
			if !in.closed {
				in.err = err
			}
			in.mu.Unlock()
			return
		}

		if b >= 0x80 {
			// Status byte.
			if b >= 0xF0 {
				// System messages.
				if b >= 0xF8 {
					// Realtime message. Ignore and continue.
					continue
				}
				// Clear running status for non-realtime system messages.
				runningStatus = 0
				expectedDataLen = 0
				dataBuffer = nil

				// Skip SysEx content.
				if b == 0xF0 {
					for {
						sb, sErr := reader.ReadByte()
						if sErr != nil {
							break
						}
						if sb == 0xF7 {
							break
						}
						if sb >= 0x80 && sb < 0xF8 {
							_ = reader.UnreadByte()
							break
						}
					}
				}
				continue
			}

			// Channel voice messages.
			runningStatus = b
			expectedDataLen = getDataLen(b)
			dataBuffer = make([]byte, 0, expectedDataLen)
		} else {
			// Data byte.
			if runningStatus == 0 {
				continue
			}

			dataBuffer = append(dataBuffer, b)
			if len(dataBuffer) == expectedDataLen {
				status := runningStatus
				eventType := EventType(status & 0xF0)
				channel := int(status & 0x0F)

				var key, val int
				if expectedDataLen >= 1 {
					key = int(dataBuffer[0])
				}
				if expectedDataLen >= 2 {
					val = int(dataBuffer[1])
				}

				switch eventType {
				case NoteOn, NoteOff, CC:
					in.mu.Lock()
					in.buf = append(in.buf, Event{
						Type:    eventType,
						Channel: channel,
						Key:     key,
						Value:   val,
					})
					in.mu.Unlock()
				}

				// Prepare for next message using the same running status.
				dataBuffer = make([]byte, 0, expectedDataLen)
			}
		}
	}
}
