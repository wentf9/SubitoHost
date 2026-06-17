package meltysynth

import (
	"math"
)

var resonancePeakOffset = float32(1 - 1/math.Sqrt(2))

type biQuadFilter struct {
	synthesizer *Synthesizer
	active      bool
	a0          float64
	a1          float64
	a2          float64
	a3          float64
	a4          float64
	x1          float64
	x2          float64
	y1          float64
	y2          float64
}

func newBiQuadFilter(s *Synthesizer) *biQuadFilter {
	result := new(biQuadFilter)
	result.synthesizer = s
	return result
}

func (bf *biQuadFilter) clearBuffer() {
	bf.x1 = 0
	bf.x2 = 0
	bf.y1 = 0
	bf.y2 = 0
}

func (bf *biQuadFilter) setLowPassFilter(cutoffFrequency float32, resonance float32) {
	if cutoffFrequency >= 0.499*float32(bf.synthesizer.SampleRate) {
		bf.active = false
		return
	}
	bf.active = true

	// This equation gives the Q value which makes the desired resonance peak.
	// The error of the resultant peak height is less than 3%.
	q := resonance - resonancePeakOffset/(1+6*(resonance-1))

	w := 2 * math.Pi * float64(cutoffFrequency) / float64(bf.synthesizer.SampleRate)
	cosw := math.Cos(w)
	alpha := math.Sin(w) / float64(2*q)

	b0 := (1 - cosw) / 2
	b1 := 1 - cosw
	b2 := (1 - cosw) / 2
	a0 := 1 + alpha
	a1 := -2 * cosw
	a2 := 1 - alpha

	bf.setCoefficients(a0, a1, a2, b0, b1, b2)
}

func (bf *biQuadFilter) process(block []float32) {
	blockLength := len(block)

	if bf.active {
		for t := range blockLength {
			// 将输入提升为双精度参与高精度运算
			input := float64(block[t])
			output := bf.a0*input + bf.a1*bf.x1 + bf.a2*bf.x2 - bf.a3*bf.y1 - bf.a4*bf.y2

			// 转换为单精度用于验证和输出
			out32 := float32(output)

			// 将 897988541 (1.0e-6) 替换为 8388608 (1.175e-38)
			// 这既能阻挡真正的非正规数引发的 CPU 掉速，又能消除音频过零时的交越失真
			if (math.Float32bits(out32) & 0x7FFFFFFF) < 8388608 {
				out32 = 0
				output = 0 // 同步清零内部的 float64 状态，彻底切断反馈底噪
			}

			bf.x2 = bf.x1
			bf.x1 = input
			bf.y2 = bf.y1
			bf.y1 = output

			// 写回音频块
			block[t] = out32
		}
	} else {
		// 非活跃状态下，也要以高精度保存最后两个样本
		bf.x2 = float64(block[blockLength-2])
		bf.x1 = float64(block[blockLength-1])
		bf.y2 = bf.x2
		bf.y1 = bf.x1
	}
}

func (bf *biQuadFilter) setCoefficients(a0 float64, a1 float64, a2 float64, b0 float64, b1 float64, b2 float64) {
	bf.a0 = b0 / a0
	bf.a1 = b1 / a0
	bf.a2 = b2 / a0
	bf.a3 = a1 / a0
	bf.a4 = a2 / a0
}
