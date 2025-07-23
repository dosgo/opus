package celt

import (
	"math"
	"math/bits"
)

// CeltCommon contains common CELT functions and constants
type CeltCommon struct{}

// invTable is a table of 6*64/x, trained on real data to minimize the average error
var invTable = [128]int16{
	255, 255, 156, 110, 86, 70, 59, 51, 45, 40, 37, 33, 31, 28, 26, 25,
	23, 22, 21, 20, 19, 18, 17, 16, 16, 15, 15, 14, 13, 13, 12, 12,
	12, 12, 11, 11, 11, 10, 10, 10, 9, 9, 9, 9, 9, 9, 8, 8,
	8, 8, 8, 7, 7, 7, 7, 7, 7, 6, 6, 6, 6, 6, 6, 6,
	6, 6, 6, 6, 6, 6, 6, 6, 6, 5, 5, 5, 5, 5, 5, 5,
	5, 5, 5, 5, 5, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4,
	4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 2,
}

// ComputeVBR computes the variable bitrate target
func (c *CeltCommon) ComputeVBR(mode *CeltMode, analysis *AnalysisInfo, baseTarget, LM, bitrate, lastCodedBands, C, intensity,
	constrainedVBR, stereoSaving, totBoost, tfEstimate, pitchChange, maxDepth int,
	variableDuration OpusFramesize, lfe, hasSurroundMask, surroundMasking, temporalVBR int) int {
	// The target rate in 8th bits per frame
	target := baseTarget
	nbEBands := mode.NbEBands
	eBands := mode.EBands

	codedBands := lastCodedBands
	if codedBands == 0 {
		codedBands = nbEBands
	}
	codedBins := eBands[codedBands] << LM
	if C == 2 {
		codedBins += eBands[min(intensity, codedBands)] << LM
	}

	if analysis.Enabled && analysis.Valid != 0 && analysis.Activity < 0.4 {
		target -= int(float32(codedBins<<EntropyCoderBitres) * (0.4 - analysis.Activity))
	}

	// Stereo savings
	if C == 2 {
		codedStereoBands := min(intensity, codedBands)
		codedStereoDof := (eBands[codedStereoBands] << LM) - codedStereoBands
		// Maximum fraction of the bits we can save if the signal is mono.
		maxFrac := div32_16(mult16_16(int16(0.8*32767.5), codedStereoDof), codedBins)
		stereoSaving = min16(stereoSaving, int16(1.0*255.5))
		target -= int(min32(mult16_32_q15(maxFrac, int32(target)),
			shr32(mult16_16(stereoSaving-int16(0.1*255.5), (codedStereoDof<<EntropyCoderBitres)), 8)))
	}

	// Boost the rate according to dynalloc (minus the dynalloc average for calibration).
	target += totBoost - (16 << LM)

	// Apply transient boost, compensating for average boost.
	var tfCalibration int16
	if variableDuration == OpusFramesizeVariable {
		tfCalibration = int16(0.02 * 16383.5)
	} else {
		tfCalibration = int16(0.04 * 16383.5)
	}
	target += int(shr32(mult16_32_q15(int16(tfEstimate)-tfCalibration, int32(target)), 1))

	// Apply tonality boost
	if analysis.Enabled && analysis.Valid != 0 && lfe == 0 {
		tonal := max16(0, analysis.Tonality-0.15) - 0.09
		tonalTarget := target + int(float32(codedBins<<EntropyCoderBitres)*1.2*float32(tonal))
		if pitchChange != 0 {
			tonalTarget += int(float32(codedBins<<EntropyCoderBitres) * 0.8)
		}
		target = tonalTarget
	}

	if hasSurroundMask != 0 && lfe == 0 {
		surroundTarget := target + int(shr32(mult16_16(int16(surroundMasking), codedBins<<EntropyCoderBitres), DBShift))
		target = max(target/4, surroundTarget)
	}

	{
		bins := eBands[nbEBands-2] << LM
		floorDepth := int(shr32(mult16_16((C*bins<<EntropyCoderBitres), maxDepth), DBShift))
		floorDepth = max(floorDepth, target>>2)
		target = min(target, floorDepth)
	}

	if (hasSurroundMask == 0 || lfe != 0) && (constrainedVBR != 0 || bitrate < 64000) {
		rateFactor := max16(0, int16(bitrate-32000))
		if constrainedVBR != 0 {
			rateFactor = min16(rateFactor, int16(0.67*32767.5))
		}
		target = baseTarget + int(mult16_32_q15(rateFactor, int32(target-baseTarget)))
	}

	if hasSurroundMask == 0 && tfEstimate < int(int16(0.2*16383.5)) {
		amount := mult16_16_q15(int16(0.0000031*1073741823.5), int16(max(0, min(32000, 96000-bitrate))))
		tvbrFactor := shr32(mult16_16(int16(temporalVBR), amount), DBShift)
		target += int(mult16_32_q15(tvbrFactor, int32(target)))
	}

	// Don't allow more than doubling the rate
	target = min(2*baseTarget, target)

	return target
}

// TransientAnalysis performs transient analysis on input signal
func (c *CeltCommon) TransientAnalysis(input [][]int32, length, channels int, tfEstimate, tfChan *int) int {
	isTransient := 0
	maskMetric := 0
	tfChanVal := 0
	tmp := make([]int32, length)

	length2 := length / 2
	for ch := 0; ch < channels; ch++ {
		var mem0, mem1 int32
		unmask := 0
		// High-pass filter
		for i := 0; i < length; i++ {
			x := shr32(input[ch][i], SigShift)
			y := add32(mem0, x)
			mem0 = mem1 + y - shl32(x, 1)
			mem1 = x - shr32(y, 1)
			tmp[i] = extract16(shr32(y, 2))
		}

		// First few samples are bad because we don't propagate the memory
		for i := 0; i < 12; i++ {
			tmp[i] = 0
		}

		// Normalize tmp to max range
		shift := 14 - celtILog2(1+celtMaxAbs32(tmp, 0, length))
		if shift != 0 {
			for i := 0; i < length; i++ {
				tmp[i] = shl16(tmp[i], shift)
			}
		}

		mean := int32(0)
		mem0 = 0
		// Forward pass to compute the post-echo threshold
		for i := 0; i < length2; i++ {
			x2 := psHR32(mult16_16(tmp[2*i], tmp[2*i])+mult16_16(tmp[2*i+1], tmp[2*i+1]), 16)
			mean += x2
			tmp[i] = mem0 + psHR32(x2-mem0, 4)
			mem0 = tmp[i]
		}

		mem0 = 0
		maxE := int32(0)
		// Backward pass to compute the pre-echo threshold
		for i := length2 - 1; i >= 0; i-- {
			tmp[i] = mem0 + psHR32(tmp[i]-mem0, 3)
			mem0 = tmp[i]
			maxE = max16(maxE, mem0)
		}

		// Compute frame energy
		mean = mult16_16(celtSqrt(mean), celtSqrt(mult16_16(maxE, int32(length2>>1))))
		norm := shl32(int32(length2), 6+14) / (Epsilon + shr32(mean, 1))

		// Compute harmonic mean
		for i := 12; i < length2-5; i += 4 {
			id := max32(0, min32(127, mult16_32_q15((tmp[i]+Epsilon), norm)))
			unmask += int(invTable[id])
		}

		// Normalize
		unmask = 64 * unmask * 4 / (6 * (length2 - 17))
		if unmask > maskMetric {
			tfChanVal = ch
			maskMetric = unmask
		}
	}

	if maskMetric > 200 {
		isTransient = 1
	}

	// Arbitrary metric for VBR boost
	tfMax := max16(0, int32(celtSqrt(27*int32(maskMetric))-42))
	*tfEstimate = int(celtSqrt(max32(0, shl32(mult16_16(int16(0.0069*16383.5), min16(163, int16(tfMax))), 14)-int32(0.139*268435455.5))))
	*tfChan = tfChanVal

	return isTransient
}

// Helper functions that need to be implemented
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min16(a, b int16) int16 {
	if a < b {
		return a
	}
	return b
}

func max16(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func div32_16(a, b int32) int32 {
	return a / b
}

func mult16_16(a, b int16) int32 {
	return int32(a) * int32(b)
}

func mult16_32_q15(a int16, b int32) int32 {
	return (int32(a) * b) >> 15
}

func shr32(a, shift int32) int32 {
	return a >> shift
}

func shl32(a, shift int32) int32 {
	return a << shift
}

func shl16(a int32, shift int) int32 {
	return a << shift
}

func add32(a, b int32) int32 {
	return a + b
}

func sub32(a, b int32) int32 {
	return a - b
}

func extract16(a int32) int16 {
	return int16(a)
}

func psHR32(a, shift int32) int32 {
	return (a + (1 << (shift - 1))) >> shift
}

func celtILog2(x int32) int {
	return 31 - bits.LeadingZeros32(uint32(x))
}

func celtMaxAbs32(x []int32, offset, len int) int32 {
	max := int32(0)
	for i := offset; i < offset+len; i++ {
		abs := x[i]
		if abs < 0 {
			abs = -abs
		}
		if abs > max {
			max = abs
		}
	}
	return max
}

func celtSqrt(x int32) int32 {
	return int32(math.Sqrt(float64(x)))
}

// Constants
const (
	EntropyCoderBitres = 3
	DBShift            = 8
	SigShift           = 12
	Epsilon            = 1
)
