package synth

import (
	"testing"

	"github.com/wentf9/subitohost/internal/config"
)

func TestAudioRendererUsesConfiguredChunkSize(t *testing.T) {
	r := newAudioRenderer(config.Audio{SampleRate: 48000, BufferSize: 192, Periods: 3})
	if r.chunkFrames != 192 {
		t.Fatalf("chunkFrames = %d, want 192", r.chunkFrames)
	}
	if len(r.newLeft) != 192 || len(r.oldRight) != 192 {
		t.Fatalf("renderer buffers were not preallocated to configured size")
	}
	if r.fadeTotal != 720 {
		t.Fatalf("fadeTotal = %d, want 720 (15ms at 48kHz)", r.fadeTotal)
	}
}

func TestAudioRendererPublishesAndCrossfades(t *testing.T) {
	r := newAudioRenderer(config.Audio{SampleRate: 1000, BufferSize: 8, Periods: 2})
	first := &Synth{}
	second := &Synth{}
	buf := make([]byte, 8*8)

	r.pending.Store(first)
	if _, err := r.Read(buf); err != nil {
		t.Fatal(err)
	}
	if r.current != first || r.old != nil {
		t.Fatalf("first publish did not become current without a crossfade")
	}

	r.pending.Store(second)
	if _, err := r.Read(buf); err != nil {
		t.Fatal(err)
	}
	if r.current != second || r.old != first {
		t.Fatalf("second publish did not retain old synth for crossfade")
	}
	if r.fadeRemaining != r.fadeTotal-8 {
		t.Fatalf("fadeRemaining = %d, want %d", r.fadeRemaining, r.fadeTotal-8)
	}
	if _, err := r.Read(buf); err != nil {
		t.Fatal(err)
	}
	if r.old != nil || r.fadeRemaining != 0 {
		t.Fatalf("old synth was not retired after crossfade")
	}
}

func TestAudioRendererDefersRapidSwapUntilCurrentFadeCompletes(t *testing.T) {
	r := newAudioRenderer(config.Audio{SampleRate: 1000, BufferSize: 8, Periods: 2})
	first := &Synth{}
	second := &Synth{}
	third := &Synth{}
	buf := make([]byte, 8*8)

	r.pending.Store(first)
	r.Read(buf)
	r.pending.Store(second)
	r.Read(buf)
	r.pending.Store(third)
	r.Read(buf) // finishes first -> second; must not start second -> third yet
	if r.current != second || r.pending.Load() != third {
		t.Fatalf("rapid swap interrupted an in-progress crossfade")
	}
	r.Read(buf)
	if r.current != third || r.old != second {
		t.Fatalf("deferred rapid swap was not started after the prior fade")
	}
}

func TestAudioRendererReadHasNoSteadyStateAllocations(t *testing.T) {
	r := newAudioRenderer(config.Audio{SampleRate: 48000, BufferSize: 128, Periods: 2})
	r.current = &Synth{}
	buf := make([]byte, 128*8)
	if _, err := r.Read(buf); err != nil {
		t.Fatal(err)
	}
	allocs := testing.AllocsPerRun(1000, func() {
		if _, err := r.Read(buf); err != nil {
			panic(err)
		}
	})
	if allocs != 0 {
		t.Fatalf("AudioRenderer.Read allocations = %v, want 0", allocs)
	}
	if r.callbackCount.Load() != 1002 { // warmup + AllocsPerRun warmup + 1000 runs
		t.Fatalf("callback count = %d, want 1002", r.callbackCount.Load())
	}
	if r.callbackMaxNS.Load() == 0 {
		t.Fatal("callback maximum duration was not recorded")
	}
}

func TestNormalizedAudioConfig(t *testing.T) {
	sampleRate, frames, periods := normalizedAudioConfig(config.Audio{})
	if sampleRate != 48000 || frames != 128 || periods != 2 {
		t.Fatalf("defaults = (%d, %d, %d), want (48000, 128, 2)", sampleRate, frames, periods)
	}
}

func TestRuntimeAudioConfigReportsConfiguredDeviceLatency(t *testing.T) {
	runtime := runtimeAudioConfig(config.Audio{SampleRate: 48000, BufferSize: 128, Periods: 2})
	if runtime.SampleRate != 48000 || runtime.FramesPerPeriod != 128 || runtime.Periods != 2 {
		t.Fatalf("unexpected runtime config: %+v", runtime)
	}
	const want = float64(256) * 1000 / 48000
	if runtime.EstimatedLatencyMS != want {
		t.Fatalf("EstimatedLatencyMS = %v, want %v", runtime.EstimatedLatencyMS, want)
	}
}

func TestAudioRendererHandlesLargeSinkQuantum(t *testing.T) {
	r := newAudioRenderer(config.Audio{SampleRate: 48000, BufferSize: 128, Periods: 2})
	r.current = &Synth{}
	buf := make([]byte, 1024*2*4)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(buf) {
		t.Fatalf("Read returned %d bytes, want complete %d-byte sink quantum", n, len(buf))
	}
}

func BenchmarkAudioRendererRead128(b *testing.B) {
	r := newAudioRenderer(config.Audio{SampleRate: 48000, BufferSize: 128, Periods: 2})
	r.current = &Synth{}
	buf := make([]byte, 128*8)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Read(buf)
	}
}
