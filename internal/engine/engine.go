package engine

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wentf9/subitohost/internal/config"
	"github.com/wentf9/subitohost/internal/midi"
	"github.com/wentf9/subitohost/internal/recorder"
	"github.com/wentf9/subitohost/internal/ringbuf"
	"github.com/wentf9/subitohost/internal/router"
	"github.com/wentf9/subitohost/internal/setlist"
	"github.com/wentf9/subitohost/internal/synth"
)

type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type Engine struct {
	cfg          *config.Config
	mu           sync.Mutex // protects state and setlistPath
	state        *setlist.State
	swap         *synth.SwapManager
	ring         *ringbuf.Buffer[midi.Event]
	router       atomic.Pointer[router.Router]
	input        *midi.Input
	subsMu       sync.Mutex // protects subs
	subs         []chan Event
	cancel       context.CancelFunc
	setlistPath  string
	injected     chan injectedEvent
	rec          atomic.Pointer[recorder.Recorder]
	recMu        sync.Mutex // protects recStatus, recStartedAt
	recStatus    string     // "idle", "recording", "rendering"
	recStartedAt time.Time
}

type injectedEvent struct {
	action string
	key    int
	vel    int
	target string
}

func New(cfg *config.Config) *Engine {
	return &Engine{
		cfg:       cfg,
		ring:      ringbuf.New[midi.Event](1024),
		swap:      synth.NewSwapManager(cfg.Audio),
		injected:  make(chan injectedEvent, 256),
		recStatus: "idle",
	}
}

func (e *Engine) LoadSetlist(path string, startIndex int) error {
	sl, err := setlist.LoadFile(path)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.setlistPath = path
	e.state = setlist.NewState(sl, setlist.WrapAround)
	if startIndex > 0 {
		if _, err := e.state.GoTo(startIndex); err != nil {
			e.mu.Unlock()
			return fmt.Errorf("recover to index %d: %w", startIndex, err)
		}
	}
	e.mu.Unlock()
	return e.activateCurrentProfile()
}

func (e *Engine) activateCurrentProfile() error {
	profile := e.state.Current()
	rules, err := router.BuildRules(profile.Rules)
	if err != nil {
		return fmt.Errorf("build rules: %w", err)
	}
	var trigger *router.Trigger
	if profile.NextTrigger != nil {
		trigger = &router.Trigger{
			CCNumber:   profile.NextTrigger.CCNumber,
			CCValueMin: profile.NextTrigger.CCValueMin,
		}
	}
	e.router.Store(router.New(rules, trigger))
	if err := e.swap.SwapTo(profile); err != nil {
		return fmt.Errorf("swap synth: %w", err)
	}
	if e.cfg.Recovery.AutoResume {
		if err := SaveRecovery(e.cfg.Recovery.StateFile, RecoveryState{
			SetlistPath:  e.setlistPath,
			CurrentIndex: e.state.Index(),
		}); err != nil {
			log.Printf("warning: failed to save recovery state: %v", err)
		}
	}
	e.Broadcast(Event{Type: "profile_switch", Data: map[string]interface{}{
		"index": e.state.Index(),
		"name":  profile.Name,
	}})
	log.Printf("activated profile [%d] %q", e.state.Index(), profile.Name)
	// Auto-split active recording on profile switch: SMF cannot hot-swap soundfonts.
	if e.IsRecording() {
		log.Printf("profile switch during recording: auto-splitting")
		if _, err := e.StopRecording(); err != nil {
			log.Printf("auto-split stop: %v", err)
		}
		if err := e.StartRecording(); err != nil {
			log.Printf("auto-split start: %v", err)
		}
	}
	return nil
}

func (e *Engine) ConnectMIDI(deviceID int) error {
	if e.input != nil {
		e.input.Close()
	}
	inp, err := midi.OpenInput(deviceID)
	if err != nil {
		return err
	}
	e.input = inp
	return nil
}

func (e *Engine) AutoConnectMIDI() error {
	dev, err := midi.FindInputByName(e.cfg.MIDI.DeviceNamePattern)
	if err != nil {
		return err
	}
	return e.ConnectMIDI(dev.ID)
}

func (e *Engine) Start() error {
	if err := e.swap.Init(); err != nil {
		return fmt.Errorf("init synth: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	go e.midiLoop(ctx)
	go e.audioLoop(ctx)
	return nil
}

func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	if e.IsRecording() {
		if _, err := e.StopRecording(); err != nil {
			log.Printf("stop recording on shutdown: %v", err)
		}
	}
	if e.input != nil {
		e.input.Close()
	}
	e.swap.Close()
}

func (e *Engine) NextProfile() error {
	e.mu.Lock()
	_, err := e.state.Next()
	e.mu.Unlock()
	if err != nil {
		return err
	}
	return e.activateCurrentProfile()
}

func (e *Engine) PrevProfile() error {
	e.mu.Lock()
	_, err := e.state.Prev()
	e.mu.Unlock()
	if err != nil {
		return err
	}
	return e.activateCurrentProfile()
}

func (e *Engine) GoToProfile(index int) error {
	e.mu.Lock()
	_, err := e.state.GoTo(index)
	e.mu.Unlock()
	if err != nil {
		return err
	}
	return e.activateCurrentProfile()
}

func (e *Engine) State() *setlist.State {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}
func (e *Engine) Config() *config.Config { return e.cfg }
func (e *Engine) SetGain(gain float64)   { e.swap.SetGain(gain) }
func (e *Engine) Gain() float64          { return e.swap.Gain() }

// StartRecording starts a new recording session for the current profile.
func (e *Engine) StartRecording() error {
	e.mu.Lock()
	if e.state == nil {
		e.mu.Unlock()
		return fmt.Errorf("no setlist loaded")
	}
	profile := e.state.Current()
	e.mu.Unlock()

	e.recMu.Lock()
	if e.recStatus != "idle" {
		e.recMu.Unlock()
		return fmt.Errorf("already recording")
	}
	r := recorder.New(e.cfg.Recording, e.cfg.Audio)
	if err := r.Start(profile); err != nil {
		e.recMu.Unlock()
		return fmt.Errorf("start recorder: %w", err)
	}
	e.rec.Store(r)
	e.recStatus = "recording"
	e.recStartedAt = time.Now()
	startedAt := e.recStartedAt
	e.recMu.Unlock()

	e.Broadcast(Event{Type: "record_start", Data: map[string]interface{}{
		"started_at": startedAt.Format(time.RFC3339),
	}})
	log.Printf("recording started")
	return nil
}

// StopRecording stops event buffering, flushes the SMF file, and launches
// offline WAV rendering in a background goroutine.
// Returns the .mid file path.
func (e *Engine) StopRecording() (midPath string, err error) {
	r := e.rec.Load()
	if r == nil {
		return "", fmt.Errorf("not recording")
	}
	e.rec.Store(nil)
	e.recMu.Lock()
	e.recStatus = "rendering"
	e.recStartedAt = time.Time{}
	e.recMu.Unlock()

	mid, wav, err := r.Stop()
	if err != nil {
		e.recMu.Lock()
		e.recStatus = "idle"
		e.recMu.Unlock()
		return "", fmt.Errorf("stop recorder: %w", err)
	}

	e.Broadcast(Event{Type: "record_rendering", Data: map[string]interface{}{"mid": mid}})
	log.Printf("recording stopped, rendering to %s", wav)

	go recorder.RenderOffline(mid, wav, r.Profile(), e.cfg.Audio, func(evType string, data map[string]interface{}) {
		if evType == "record_stop" {
			e.recMu.Lock()
			e.recStatus = "idle"
			e.recMu.Unlock()
		}
		e.Broadcast(Event{Type: evType, Data: data})
	})

	return mid, nil
}

// IsRecording returns true while events are being buffered.
func (e *Engine) IsRecording() bool {
	return e.rec.Load() != nil
}

// RecordingStatus returns current status ("idle", "recording", "rendering")
// and the time recording started (zero if idle).
func (e *Engine) RecordingStatus() (status string, startedAt time.Time) {
	e.recMu.Lock()
	defer e.recMu.Unlock()
	return e.recStatus, e.recStartedAt
}

func (e *Engine) Subscribe() chan Event {
	ch := make(chan Event, 64)
	e.subsMu.Lock()
	e.subs = append(e.subs, ch)
	e.subsMu.Unlock()
	return ch
}

func (e *Engine) Unsubscribe(ch chan Event) {
	e.subsMu.Lock()
	for i, s := range e.subs {
		if s == ch {
			e.subs = append(e.subs[:i], e.subs[i+1:]...)
			close(ch)
			break
		}
	}
	e.subsMu.Unlock()
}

func (e *Engine) Broadcast(ev Event) {
	e.subsMu.Lock()
	subs := make([]chan Event, len(e.subs))
	copy(subs, e.subs)
	e.subsMu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (e *Engine) BroadcastNote(eventType string, key, vel int, source string) {
	e.Broadcast(Event{
		Type: eventType,
		Data: map[string]interface{}{
			"key":    key,
			"vel":    vel,
			"source": source,
		},
	})
}

func (e *Engine) midiLoop(ctx context.Context) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Use a short timeout for epoll wait so we can periodically check
	// the injected channel and context cancellation.
	// The epoll wait itself provides event-driven wakeups for MIDI input,
	// eliminating the 1ms ticker jitter.
	const pollTimeout = 10 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return
		case inj := <-e.injected:
			e.processInjectedNote(inj.action, inj.key, inj.vel, inj.target)
			continue
		default:
		}

		if e.input == nil {
			// No input device, just wait for injected events or context
			select {
			case <-ctx.Done():
				return
			case inj := <-e.injected:
				e.processInjectedNote(inj.action, inj.key, inj.vel, inj.target)
			case <-time.After(pollTimeout):
			}
			continue
		}

		// Block until MIDI data arrives or timeout
		if !e.input.WaitEvent(pollTimeout) {
			// Timeout or error: check for injected events
			select {
			case <-ctx.Done():
				return
			case inj := <-e.injected:
				e.processInjectedNote(inj.action, inj.key, inj.vel, inj.target)
			default:
			}
			continue
		}

		events, err := e.input.Read()
		if err != nil {
			log.Printf("MIDI read error: %v", err)
			e.Broadcast(Event{Type: "midi_disconnect"})
			e.input = nil
			go e.reconnectMIDI(ctx)
			continue
		}

		for _, ev := range events {
			if ev.Type == midi.NoteOn {
				e.BroadcastNote("note_on", ev.Key, ev.Value, "input")
			} else if ev.Type == midi.NoteOff {
				e.BroadcastNote("note_off", ev.Key, 0, "input")
			}
		}

		r := e.router.Load()
		if r == nil {
			continue
		}
		for _, ev := range events {
			// CC recording trigger: consume event without forwarding to router
			if e.cfg.Recording.TriggerCC > 0 &&
				ev.Type == midi.CC &&
				ev.Key == e.cfg.Recording.TriggerCC &&
				ev.Value >= 1 {
				go func() {
					if e.IsRecording() {
						if _, err := e.StopRecording(); err != nil {
							log.Printf("CC stop recording: %v", err)
						}
					} else {
						if err := e.StartRecording(); err != nil {
							log.Printf("CC start recording: %v", err)
						}
					}
				}()
				continue
			}
			result := r.Process(ev)
			if result.Triggered {
				go func() {
					if err := e.NextProfile(); err != nil {
						log.Printf("profile switch error: %v", err)
					}
				}()
				continue
			}
			for _, out := range result.Events {
				if out.Type == midi.NoteOn {
					e.BroadcastNote("note_on", out.Key, out.Value, "output")
				} else if out.Type == midi.NoteOff {
					e.BroadcastNote("note_off", out.Key, 0, "output")
				}
				e.ringWrite(out)
			}
		}
	}
}

func (e *Engine) audioLoop(ctx context.Context) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Pre-allocate batch buffer to drain ring buffer in bulk.
	// This ensures chords (multiple NoteOn) are processed in a single iteration,
	// getting enqueued to the synth's event queue together and rendered in the same block.
	batch := make([]midi.Event, 256)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			s := e.swap.Active.Load()
			if s == nil {
				time.Sleep(time.Millisecond)
				continue
			}

			// Bulk drain: read all available events from ring buffer
			n := 0
			for n < len(batch) {
				ev, ok := e.ring.Read()
				if !ok {
					break
				}
				batch[n] = ev
				n++
			}
			if n == 0 {
				time.Sleep(100 * time.Microsecond)
				continue
			}

			// Process all events in this batch
			for i := 0; i < n; i++ {
				ev := batch[i]
				switch ev.Type {
				case midi.NoteOn:
					s.NoteOn(ev.Channel, ev.Key, ev.Value)
				case midi.NoteOff:
					s.NoteOff(ev.Channel, ev.Key)
				case midi.CC:
					s.CC(ev.Channel, ev.Key, ev.Value)
				}
				if rec := e.rec.Load(); rec != nil {
					rec.Feed(ev)
				}
			}
		}
	}
}

func (e *Engine) reconnectMIDI(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.AutoConnectMIDI(); err == nil {
				log.Println("MIDI reconnected")
				e.Broadcast(Event{Type: "midi_reconnect"})
				return
			}
		}
	}
}

func (e *Engine) InjectNote(action string, key, vel int, target string) {
	select {
	case e.injected <- injectedEvent{action: action, key: key, vel: vel, target: target}:
	default:
		log.Println("warning: injected note dropped (buffer full)")
	}
}

// ringWrite writes a MIDI event to the ring buffer.
// NoteOff events are guaranteed delivery (spin-wait with timeout) to prevent stuck notes.
// NoteOn/CC events are dropped with a warning if the buffer is full.
func (e *Engine) ringWrite(ev midi.Event) {
	if ev.Type == midi.NoteOff {
		deadline := time.Now().Add(50 * time.Millisecond)
		for !e.ring.Write(ev) {
			if time.Now().After(deadline) {
				log.Printf("warning: NoteOff (ch=%d key=%d) dropped after 50ms timeout",
					ev.Channel, ev.Key)
				return
			}
			runtime.Gosched()
		}
		return
	}
	if !e.ring.Write(ev) {
		log.Printf("warning: %d (ch=%d key=%d val=%d) dropped (ring buffer full)",
			ev.Type, ev.Channel, ev.Key, ev.Value)
	}
}

func (e *Engine) processInjectedNote(action string, key, vel int, target string) {
	evType := midi.NoteOn
	if action == "note_off" || vel == 0 {
		evType = midi.NoteOff
	}

	ev := midi.Event{Type: evType, Key: key, Value: vel, Channel: 0}

	if target == "input" {
		if evType == midi.NoteOn {
			e.BroadcastNote("note_on", key, vel, "input")
		} else {
			e.BroadcastNote("note_off", key, 0, "input")
		}
		r := e.router.Load()
		if r == nil {
			return
		}
		result := r.Process(ev)
		if result.Triggered {
			go func() {
				if err := e.NextProfile(); err != nil {
					log.Printf("profile switch error: %v", err)
				}
			}()
			return
		}
		for _, out := range result.Events {
			if out.Type == midi.NoteOn {
				e.BroadcastNote("note_on", out.Key, out.Value, "output")
			} else if out.Type == midi.NoteOff {
				e.BroadcastNote("note_off", out.Key, 0, "output")
			}
			e.ringWrite(out)
		}
	} else if target == "output" {
		if evType == midi.NoteOn {
			e.BroadcastNote("note_on", key, vel, "output")
		} else {
			e.BroadcastNote("note_off", key, 0, "output")
		}
		e.ringWrite(ev)
	}
}
