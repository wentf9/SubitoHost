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
	"syscall"
	"time"
)

// Input wraps a Linux raw MIDI device, read via epoll for low-latency event notification.
type Input struct {
	file       *os.File
	fd         int
	epollFd    int
	err        error
	closed     bool
	mu         sync.Mutex
	buf        []Event
	reader     *bufio.Reader
	running    byte
	dataLen    int
	dataBuf    []byte
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
		base := filepath.Base(path)
		var card, device int
		_, err := fmt.Sscanf(base, "midiC%dD%d", &card, &device)
		if err != nil {
			continue
		}

		name, ok := cardNames[card]
		if !ok {
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

		subparts := strings.SplitN(parts[1], "]:", 2)
		var name string
		if len(subparts) >= 2 {
			name = strings.TrimSpace(subparts[1])
		}

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

// OpenInput opens a MIDI input device by ID and sets up epoll for event notification.
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

	fd := int(file.Fd())

	// Set the file descriptor to non-blocking mode so epoll + Read works correctly.
	if err := syscall.SetNonblock(fd, true); err != nil {
		file.Close()
		return nil, fmt.Errorf("set nonblock failed: %w", err)
	}

	epollFd, err := syscall.EpollCreate1(0)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("epoll create failed: %w", err)
	}

	event := &syscall.EpollEvent{
		Events: syscall.EPOLLIN,
		Fd:     int32(fd),
	}
	if err := syscall.EpollCtl(epollFd, syscall.EPOLL_CTL_ADD, fd, event); err != nil {
		syscall.Close(epollFd)
		file.Close()
		return nil, fmt.Errorf("epoll ctl add failed: %w", err)
	}

	inp := &Input{
		file:    file,
		fd:      fd,
		epollFd: epollFd,
		buf:     make([]Event, 0),
		reader:  bufio.NewReader(file),
	}
	return inp, nil
}

// WaitEvent blocks until MIDI data is available or timeout elapses.
// Returns true if data is available to read, false on timeout or error.
func (in *Input) WaitEvent(timeout time.Duration) bool {
	events := make([]syscall.EpollEvent, 1)
	var msTimeout int
	if timeout < 0 {
		msTimeout = -1
	} else {
		msTimeout = int(timeout / time.Millisecond)
	}
	n, err := syscall.EpollWait(in.epollFd, events, msTimeout)
	if err != nil {
		// EINTR is normal when interrupted by signals
		if err == syscall.EINTR {
			return false
		}
		in.mu.Lock()
		if !in.closed {
			in.err = err
		}
		in.mu.Unlock()
		return false
	}
	return n > 0
}

// Poll returns true if there are buffered events available.
func (in *Input) Poll() bool {
	in.mu.Lock()
	defer in.mu.Unlock()
	return len(in.buf) > 0
}

// Read reads available MIDI events. It first drains any already-parsed events
// from the internal buffer, then reads from the device if epoll reported data.
func (in *Input) Read() ([]Event, error) {
	in.mu.Lock()
	if in.err != nil {
		in.mu.Unlock()
		return nil, in.err
	}
	in.mu.Unlock()

	// Parse any available bytes from the device into events
	in.parseAvailable()

	in.mu.Lock()
	defer in.mu.Unlock()
	if len(in.buf) == 0 {
		return nil, nil
	}
	res := in.buf
	in.buf = make([]Event, 0)
	return res, nil
}

// parseAvailable reads all available bytes from the non-blocking fd and parses
// them into MIDI events, buffering them in in.buf.
func (in *Input) parseAvailable() {
	for {
		b, err := in.reader.ReadByte()
		if err != nil {
			// EAGAIN means no more data available (non-blocking fd)
			break
		}
		in.parseByte(b)
	}
}

// parseByte processes a single raw MIDI byte, maintaining running status state.
func (in *Input) parseByte(b byte) {
	if b >= 0x80 {
		// Status byte
		if b >= 0xF0 {
			// System messages
			if b >= 0xF8 {
				// Realtime message, ignore
				return
			}
			// Clear running status for non-realtime system messages
			in.running = 0
			in.dataLen = 0
			in.dataBuf = nil

			// Skip SysEx content
			if b == 0xF0 {
				for {
					sb, err := in.reader.ReadByte()
					if err != nil {
						break
					}
					if sb == 0xF7 {
						break
					}
					if sb >= 0x80 && sb < 0xF8 {
						// Unread and process in next call
						_ = in.reader.UnreadByte()
						break
					}
				}
			}
			return
		}

		// Channel voice message
		in.running = b
		in.dataLen = getDataLen(b)
		in.dataBuf = make([]byte, 0, in.dataLen)
		return
	}

	// Data byte
	if in.running == 0 {
		return
	}

	in.dataBuf = append(in.dataBuf, b)
	if len(in.dataBuf) == in.dataLen {
		status := in.running
		eventType := EventType(status & 0xF0)
		channel := int(status & 0x0F)

		var key, val int
		if in.dataLen >= 1 {
			key = int(in.dataBuf[0])
		}
		if in.dataLen >= 2 {
			val = int(in.dataBuf[1])
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

		in.dataBuf = make([]byte, 0, in.dataLen)
	}
}

func getDataLen(status byte) int {
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

// Close closes the MIDI input stream and epoll fd.
func (in *Input) Close() error {
	in.mu.Lock()
	if in.closed {
		in.mu.Unlock()
		return nil
	}
	in.closed = true
	in.mu.Unlock()

	if in.epollFd >= 0 {
		syscall.Close(in.epollFd)
		in.epollFd = -1
	}
	return in.file.Close()
}
