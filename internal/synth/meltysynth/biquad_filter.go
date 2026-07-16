package meltysynth

import (
	"math"
)

var resonancePeakOffset = float32(1 - 1/math.Sqrt(2))

type biQuadFilter struct {
	synthesizer *Synthesizer
	active      bool
	initialized bool // 用于判断是否是首次设置参数

	a0 float64 // 目标系数 (Target)
	a1 float64
	a2 float64
	a3 float64
	a4 float64

	cA0 float64 // 当前正在平滑过渡的系数 (Current)
	cA1 float64
	cA2 float64
	cA3 float64
	cA4 float64

	x1 float64
	x2 float64
	y1 float64
	y2 float64
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
	bf.initialized = false // 重置初始化状态
}

func (bf *biQuadFilter) setLowPassFilter(cutoffFrequency float32, resonance float32) {
	if !isFinite32(cutoffFrequency) || !isFinite32(resonance) || cutoffFrequency <= 0 || resonance <= 0 {
		bf.active = false
		return
	}
	if cutoffFrequency >= 0.49*float32(bf.synthesizer.SampleRate) {
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
		// A fixed 5 ms time constant makes smoothing independent of block size.
		smoothing := 1 - math.Exp(-1/(0.005*float64(bf.synthesizer.SampleRate)))

		for t := range blockLength {
			// 每处理一个声音样本，滤波器系数就非常平滑地移动一点点
			bf.cA0 += (bf.a0 - bf.cA0) * smoothing
			bf.cA1 += (bf.a1 - bf.cA1) * smoothing
			bf.cA2 += (bf.a2 - bf.cA2) * smoothing
			bf.cA3 += (bf.a3 - bf.cA3) * smoothing
			bf.cA4 += (bf.a4 - bf.cA4) * smoothing

			// 将输入提升为双精度参与高精度运算
			input := float64(block[t])

			// 使用动态渐变的系数 (cA) 而不是固定系数进行滤波
			output := bf.cA0*input + bf.cA1*bf.x1 + bf.cA2*bf.x2 - bf.cA3*bf.y1 - bf.cA4*bf.y2

			// 转换为单精度用于验证和输出
			if math.Abs(output) < 1e-30 {
				output = 0
			}
			out32 := float32(output)

			bf.x2 = bf.x1
			bf.x1 = input
			bf.y2 = bf.y1
			bf.y1 = output

			// 写回音频块
			block[t] = out32
		}

		bf.snapCoefficientsToTarget()

	} else {
		// 非活跃状态下，也要以高精度保存最后两个样本
		bf.x2 = float64(block[blockLength-2])
		bf.x1 = float64(block[blockLength-1])
		bf.y2 = bf.x2
		bf.y1 = bf.x1
	}
}

func isFinite32(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

func (bf *biQuadFilter) snapCoefficientsToTarget() {
	if math.Abs(bf.a0-bf.cA0) < 1e-12 {
		bf.cA0 = bf.a0
	}
	if math.Abs(bf.a1-bf.cA1) < 1e-12 {
		bf.cA1 = bf.a1
	}
	if math.Abs(bf.a2-bf.cA2) < 1e-12 {
		bf.cA2 = bf.a2
	}
	if math.Abs(bf.a3-bf.cA3) < 1e-12 {
		bf.cA3 = bf.a3
	}
	if math.Abs(bf.a4-bf.cA4) < 1e-12 {
		bf.cA4 = bf.a4
	}
}

func (bf *biQuadFilter) setCoefficients(a0 float64, a1 float64, a2 float64, b0 float64, b1 float64, b2 float64) {
	if a0 == 0 || math.IsNaN(a0) || math.IsInf(a0, 0) {
		bf.active = false
		return
	}
	bf.a0 = b0 / a0
	bf.a1 = b1 / a0
	bf.a2 = b2 / a0
	bf.a3 = a1 / a0
	bf.a4 = a2 / a0

	// 初次发声时直接对齐，不进行渐变
	if !bf.initialized {
		bf.cA0 = bf.a0
		bf.cA1 = bf.a1
		bf.cA2 = bf.a2
		bf.cA3 = bf.a3
		bf.cA4 = bf.a4
		bf.initialized = true
	}
}
