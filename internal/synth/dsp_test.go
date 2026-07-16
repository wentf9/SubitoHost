package synth

import (
	"math"
	"testing"

	"github.com/wentf9/subitohost/internal/config"
	"github.com/wentf9/subitohost/internal/ringbuf"
)

func TestSetGainPreservesFloat64Value(t *testing.T) {
	s, err := NewHeadless(config.Audio{SampleRate: 48000, BufferSize: 128, Periods: 2})
	if err != nil {
		t.Fatal(err)
	}
	for _, gain := range []float64{0, 0.1, 0.5, 1} {
		s.SetGain(gain)
		if got := s.Gain(); got != gain {
			t.Errorf("Gain() = %v after SetGain(%v)", got, gain)
		}
	}
}

func TestRealtimeSetGainPreservesFloat64Value(t *testing.T) {
	s := &Synth{evQueue: ringbuf.New[synthEvent](8)}
	for _, gain := range []float64{0, 0.1, 0.5, 1} {
		s.SetGain(gain)
		if got := s.Gain(); got != gain {
			t.Errorf("Gain() = %v after SetGain(%v)", got, gain)
		}
	}
}

func TestCriticalCommandPressureTriggersEmergencyFallback(t *testing.T) {
	s := &Synth{evQueue: ringbuf.New[synthEvent](2)}
	s.NoteOn(0, 60, 100)
	s.NoteOn(0, 61, 100)
	s.NoteOff(0, 60)

	metrics := s.QueueMetrics()
	if metrics.HighWater != 2 || metrics.Dropped != 1 {
		t.Fatalf("queue metrics = %+v, want high_water=2 dropped=1", metrics)
	}
	if !s.emergencyAllNotesOff.Load() {
		t.Fatal("dropped NoteOff did not request emergency AllNotesOff")
	}
}

func TestSoftClipContinuousMonotonicSymmetricAndFinite(t *testing.T) {
	var s Synth
	s.initSoftClipLUT()
	previous := float32(-2)
	for i := -10000; i <= 10000; i++ {
		x := float32(i) / 5000
		y := softClip(&s.softClipLUT, x)
		if math.IsNaN(float64(y)) || math.IsInf(float64(y), 0) {
			t.Fatalf("softClip(%v) is not finite", x)
		}
		if y < previous {
			t.Fatalf("softClip is not monotonic at %v: %v < %v", x, y, previous)
		}
		negative := softClip(&s.softClipLUT, -x)
		if math.Abs(float64(y+negative)) > 0.001 {
			t.Fatalf("softClip is not symmetric at %v: f(x)=%v f(-x)=%v", x, y, negative)
		}
		previous = y
	}
	left := softClip(&s.softClipLUT, 1-1e-4)
	right := softClip(&s.softClipLUT, 1+1e-4)
	if jump := math.Abs(float64(right - left)); jump > 0.001 {
		t.Fatalf("softClip discontinuity around 1: jump=%v", jump)
	}
}
