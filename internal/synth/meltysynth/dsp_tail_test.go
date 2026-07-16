package meltysynth

import (
	"math"
	"testing"
)

func TestOscillatorEndTailHasFixedLengthAcrossBlocks(t *testing.T) {
	s := &Synthesizer{SampleRate: 48000}
	o := newOscillator(s)
	data := []int16{10000, 10000, 10000}
	o.start(data, loop_NoLoop, 48000, 0, 3, 0, 0, 60, 0, 0, 100)

	first := make([]float32, 4)
	if !o.process(first, 60) {
		t.Fatal("oscillator ended before emitting its de-click tail")
	}
	if o.endTailPosition != 1 {
		t.Fatalf("tail position after first block = %d, want 1", o.endTailPosition)
	}

	middle := make([]float32, 32)
	o.process(middle, 60)
	if o.endTailPosition != 33 {
		t.Fatalf("tail position after second block = %d, want 33", o.endTailPosition)
	}
	last := make([]float32, 31)
	o.process(last, 60)
	if o.endTailPosition != oscillatorEndTailSamples || !o.finished {
		t.Fatalf("tail did not finish after exactly %d samples", oscillatorEndTailSamples)
	}
	if last[len(last)-1] != 0 {
		t.Fatalf("final tail sample = %v, want 0", last[len(last)-1])
	}

	after := make([]float32, 8)
	if o.process(after, 60) {
		t.Fatal("finished oscillator remained active for an extra block")
	}
	for i, sample := range after {
		if sample != 0 {
			t.Fatalf("post-tail sample %d = %v, want 0", i, sample)
		}
	}
}

func TestOscillatorEndTailBoundsAdjacentJump(t *testing.T) {
	s := &Synthesizer{SampleRate: 48000}
	o := newOscillator(s)
	o.start([]int16{12000, 12000, 12000}, loop_NoLoop, 48000, 0, 3, 0, 0, 60, 0, 0, 100)
	output := make([]float32, 3+oscillatorEndTailSamples)
	o.process(output, 60)

	var maximumJump float64
	for i := 1; i < len(output); i++ {
		maximumJump = math.Max(maximumJump, math.Abs(float64(output[i]-output[i-1])))
	}
	if maximumJump > 0.01 {
		t.Fatalf("maximum adjacent sample jump = %v, want <= 0.01", maximumJump)
	}
}

func TestVoiceFadeTailPersistsAcrossBlocks(t *testing.T) {
	s := &Synthesizer{SampleRate: 48000, BlockSize: 64}
	vc := newVoiceCollection(s, 2)
	v := vc.voices[0]
	for i := range v.block {
		v.block[i] = 0.5
	}
	v.currentMixGainLeft = 1
	v.currentMixGainRight = 0.5
	vc.addFadeTail(v)

	left := make([]float32, 64)
	right := make([]float32, 64)
	vc.renderFadeTails(left, right, 1)
	if !vc.fadeTails[0].active || vc.fadeTails[0].position != 64 {
		t.Fatalf("voice tail did not persist after its first block")
	}
	left2 := make([]float32, 64)
	right2 := make([]float32, 64)
	vc.renderFadeTails(left2, right2, 1)
	if vc.fadeTails[0].active {
		t.Fatalf("voice tail exceeded configured %d-sample duration", vc.fadeTailSamples)
	}
	lastNonZero := int(vc.fadeTailSamples) - 64 - 2
	firstZero := lastNonZero + 1
	if left2[lastNonZero] == 0 || left2[firstZero] != 0 {
		t.Fatalf("voice tail boundary does not match configured duration")
	}
}

func TestVoiceFadeTailBoundsAdjacentJumpAndReachesSilence(t *testing.T) {
	s := &Synthesizer{SampleRate: 48000, BlockSize: 128}
	vc := newVoiceCollection(s, 1)
	v := vc.voices[0]
	for i := range v.block {
		v.block[i] = 0.5
	}
	v.currentMixGainLeft = 1
	vc.addFadeTail(v)
	left := make([]float32, 128)
	right := make([]float32, 128)
	vc.renderFadeTails(left, right, 1)

	previous := float32(0.5)
	var maximumJump float64
	for _, sample := range left {
		maximumJump = math.Max(maximumJump, math.Abs(float64(sample-previous)))
		previous = sample
	}
	if maximumJump > 0.01 {
		t.Fatalf("voice-steal tail maximum adjacent jump = %v, want <= 0.01", maximumJump)
	}
	for i := int(vc.fadeTailSamples) - 1; i < len(left); i++ {
		if left[i] != 0 {
			t.Fatalf("voice-steal tail did not reach exact silence at sample %d", i)
		}
	}
}

func TestEveryVoiceReusePathCreatesIndependentTail(t *testing.T) {
	s := &Synthesizer{SampleRate: 48000, BlockSize: 64}
	vc := newVoiceCollection(s, 1)
	region := &InstrumentRegion{}
	region.gs[gen_ExclusiveClass] = 7
	v := vc.requestNew(region, 0)
	v.exclusiveClass = 7
	v.channel = 0
	v.noteGain = 1
	v.currentMixGainLeft = 1
	for i := range v.block {
		v.block[i] = 0.25
	}
	if reused := vc.requestNew(region, 0); reused != v {
		t.Fatal("exclusive-class path did not reuse matching voice")
	}
	if !vc.fadeTails[0].active {
		t.Fatal("exclusive-class reuse did not create a separate fade tail")
	}

	for i := range vc.fadeTails {
		vc.fadeTails[i].active = false
	}
	region.gs[gen_ExclusiveClass] = 0
	if reused := vc.requestNew(region, 0); reused != v {
		t.Fatal("polyphony-limit path did not select existing voice")
	}
	if !vc.fadeTails[0].active {
		t.Fatal("polyphony-limit reuse did not create a separate fade tail")
	}
}

func TestBiquadRejectsInvalidParameters(t *testing.T) {
	s := &Synthesizer{SampleRate: 48000}
	for _, test := range []struct {
		cutoff    float32
		resonance float32
	}{
		{cutoff: 0, resonance: 1},
		{cutoff: float32(math.NaN()), resonance: 1},
		{cutoff: 1000, resonance: float32(math.Inf(1))},
		{cutoff: 24000, resonance: 1},
	} {
		filter := newBiQuadFilter(s)
		filter.setLowPassFilter(test.cutoff, test.resonance)
		if filter.active {
			t.Fatalf("invalid filter parameters remained active: %+v", test)
		}
	}
}

func TestBiquadFlushesInternalDenormalState(t *testing.T) {
	s := &Synthesizer{SampleRate: 48000}
	filter := newBiQuadFilter(s)
	filter.setLowPassFilter(1000, 1)
	filter.x1, filter.x2 = 1e-310, -1e-310
	filter.y1, filter.y2 = 1e-310, -1e-310
	block := make([]float32, 16)
	filter.process(block)
	if filter.x1 != 0 || filter.x2 != 0 || filter.y1 != 0 || filter.y2 != 0 {
		t.Fatalf("internal denormal state survived: x=(%g,%g) y=(%g,%g)",
			filter.x1, filter.x2, filter.y1, filter.y2)
	}
}

func TestBiquadSmoothingIsIndependentOfBlockPartition(t *testing.T) {
	s := &Synthesizer{SampleRate: 48000}
	whole := newBiQuadFilter(s)
	partitioned := newBiQuadFilter(s)
	for _, filter := range []*biQuadFilter{whole, partitioned} {
		filter.setLowPassFilter(500, 1)
		filter.setLowPassFilter(5000, 1)
	}
	whole.process(make([]float32, 64))
	for i := 0; i < 4; i++ {
		partitioned.process(make([]float32, 16))
	}
	for _, pair := range [][2]float64{
		{whole.cA0, partitioned.cA0},
		{whole.cA1, partitioned.cA1},
		{whole.cA2, partitioned.cA2},
		{whole.cA3, partitioned.cA3},
		{whole.cA4, partitioned.cA4},
	} {
		if math.Abs(pair[0]-pair[1]) > 1e-12 {
			t.Fatalf("coefficient smoothing depends on block partition: %g != %g", pair[0], pair[1])
		}
	}
}
