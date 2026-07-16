package meltysynth

import (
	"math"
)

// In this class, fixed-point numbers are used for speed-up.
// A fixed-point number is expressed by Int64, whose lower 24 bits represent the fraction part,
// and the rest represent the integer part.
// For clarity, fixed-point number variables have a suffix "_fp".

const fracBits int32 = 24
const fracUnit int64 = 1 << fracBits
const oscillatorEndTailSamples = 64

type oscillator struct {
	synthesizer      *Synthesizer
	data             []int16
	loopMode         int32
	sampleRate       int32
	sampleStart      int32
	sampleEnd        int32
	startLoop        int32
	endLoop          int32
	rootKey          int32
	tune             float32
	pitchChangeScale float32
	sampleRateRatio  float32
	looping          bool
	position_fp      int64
	lastSample       float32
	previousSample   float32
	endTailPosition  int32
	endTailActive    bool
	finished         bool
}

func newOscillator(s *Synthesizer) *oscillator {
	result := new(oscillator)
	result.synthesizer = s
	return result
}

func (o *oscillator) start(data []int16, loopMode int32, sampleRate int32, start int32, end int32, startLoop int32, endLoop int32, rootKey int32, coarseTune int32, fineTune int32, scaleTuning int32) {
	o.data = data
	o.loopMode = loopMode
	o.sampleRate = sampleRate
	o.sampleStart = start
	o.sampleEnd = end
	o.startLoop = startLoop
	o.endLoop = endLoop
	o.rootKey = rootKey

	o.tune = float32(coarseTune) + float32(0.01)*float32(fineTune)
	o.pitchChangeScale = float32(0.01) * float32(scaleTuning)
	o.sampleRateRatio = float32(sampleRate) / float32(o.synthesizer.SampleRate)

	switch loopMode {
	case loop_NoLoop:
		o.looping = false
	case loop_Continuous, loop_LoopUntilNoteOff:
		o.looping = true
	default:
		o.looping = false
	}

	o.position_fp = int64(start) << fracBits
	o.lastSample = 0
	o.previousSample = 0
	o.endTailPosition = 0
	o.endTailActive = false
	o.finished = false
}

func (o *oscillator) release() {
	if o.loopMode == loop_LoopUntilNoteOff {
		o.looping = false
	}
}

func (o *oscillator) process(block []float32, pitch float32) bool {
	pitchChange := o.pitchChangeScale*(pitch-float32(o.rootKey)) + o.tune
	pitchRatio := float64(o.sampleRateRatio) * math.Pow(float64(2), float64(pitchChange)/float64(12))
	return o.fillBlock(block, pitchRatio)
}

func (o *oscillator) fillBlock(block []float32, pitchRatio float64) bool {
	pitchRatio_fp := int64(float64(fracUnit) * pitchRatio)

	if o.looping {
		return o.fillBlock_Continuous(block, pitchRatio_fp)
	} else {
		return o.fillBlock_NoLoop(block, pitchRatio_fp)
	}
}

func (o *oscillator) fillBlock_NoLoop(block []float32, pitchRatio_fp int64) bool {
	if o.finished {
		clear(block)
		return false
	}

	blockLength := len(block)

	for t := 0; t < blockLength; t++ {
		if o.endTailActive {
			o.writeEndTail(block[t:])
			return true
		}
		index := int32(o.position_fp >> fracBits)
		if index >= o.sampleEnd {
			o.endTailActive = true
			o.endTailPosition = 0
			o.writeEndTail(block[t:])
			return true
		}

		idx0 := index - 1
		if idx0 < o.sampleStart {
			idx0 = index
		}
		idx1 := index
		idx2 := index + 1
		if idx2 >= o.sampleEnd {
			idx2 = index
		}
		idx3 := index + 2
		if idx3 >= o.sampleEnd {
			idx3 = idx2
		}

		x0 := float64(o.data[idx0])
		x1 := float64(o.data[idx1])
		x2 := float64(o.data[idx2])
		x3 := float64(o.data[idx3])

		a_fp := o.position_fp & (fracUnit - 1)
		tVal := float64(a_fp) / float64(fracUnit)

		// Cubic Hermite Interpolation (Float64 Precision)
		val := x1 + 0.5*tVal*(x2-x0+tVal*(2.0*x0-5.0*x1+4.0*x2-x3+tVal*(3.0*(x1-x2)+x3-x0)))
		sample := float32(val * (1.0 / 32768.0))
		block[t] = sample
		o.previousSample = o.lastSample
		o.lastSample = sample

		o.position_fp += pitchRatio_fp
	}

	return true
}

// writeEndTail continues the final local slope under a raised-cosine window.
// Unlike repeating the final sample, this preserves the waveform direction at
// the boundary. State persists across blocks and always lasts exactly 64
// samples, independent of where the sample ended within a render block.
func (o *oscillator) writeEndTail(block []float32) {
	slope := o.lastSample - o.previousSample
	for i := range block {
		if o.endTailPosition >= oscillatorEndTailSamples {
			clear(block[i:])
			o.endTailActive = false
			o.finished = true
			return
		}
		step := o.endTailPosition + 1
		phase := float64(step) / oscillatorEndTailSamples
		window := float32(0.5 * (1 + math.Cos(math.Pi*phase)))
		predicted := o.lastSample + slope*float32(step)
		predicted = calcClamp(predicted, -1, 1)
		block[i] = predicted * window
		o.endTailPosition++
	}
	if o.endTailPosition >= oscillatorEndTailSamples {
		o.endTailActive = false
		o.finished = true
	}
}

func (o *oscillator) fillBlock_Continuous(block []float32, pitchRatio_fp int64) bool {
	blockLength := len(block)

	endLoop_fp := int64(o.endLoop) << fracBits

	loopLength := int32(o.endLoop - o.startLoop)
	loopLength_fp := int64(loopLength) << fracBits

	for t := 0; t < blockLength; t++ {
		for o.position_fp >= endLoop_fp {
			o.position_fp -= loopLength_fp
		}

		index1 := int32(o.position_fp >> fracBits)

		idx0 := index1 - 1
		if idx0 < o.startLoop {
			idx0 += loopLength
		}
		idx1 := index1
		idx2 := index1 + 1
		if idx2 >= o.endLoop {
			idx2 -= loopLength
		}
		idx3 := index1 + 2
		if idx3 >= o.endLoop {
			idx3 -= loopLength
		}

		x0 := float64(o.data[idx0])
		x1 := float64(o.data[idx1])
		x2 := float64(o.data[idx2])
		x3 := float64(o.data[idx3])

		a_fp := o.position_fp & (fracUnit - 1)
		tVal := float64(a_fp) / float64(fracUnit)

		// Cubic Hermite Interpolation (Float64 Precision)
		val := x1 + 0.5*tVal*(x2-x0+tVal*(2.0*x0-5.0*x1+4.0*x2-x3+tVal*(3.0*(x1-x2)+x3-x0)))
		block[t] = float32(val * (1.0 / 32768.0))

		o.position_fp += pitchRatio_fp
	}

	return true
}
