package silk

// Import necessary packages
import (
	"math"
)

// EncodePulses provides functionality for encoding quantization indices of excitation
type EncodePulses struct{}

// combineAndCheck combines pairs of pulses and checks against a maximum value.
// This is a helper function used during pulse encoding.
//
// Parameters:
//
//	pulsesComb: (output) combined pulses array
//	pulsesCombPtr: offset in pulsesComb array
//	pulsesIn: (input) source pulses array
//	pulsesInPtr: offset in pulsesIn array
//	maxPulses: maximum allowed sum of pulses
//	len: number of output values
//
// Returns:
//
//	0 if successful, 1 if any sum exceeds maxPulses
func combineAndCheck(pulsesComb []int, pulsesCombPtr int, pulsesIn []int, pulsesInPtr int, maxPulses int, length int) int {
	for k := 0; k < length; k++ {
		k2p := 2*k + pulsesInPtr
		sum := pulsesIn[k2p] + pulsesIn[k2p+1]
		if sum > maxPulses {
			return 1
		}
		pulsesComb[pulsesCombPtr+k] = sum
	}
	return 0
}

// combineAndCheckNoPtr is a simplified version of combineAndCheck without pointer offsets
func combineAndCheckNoPtr(pulsesComb, pulsesIn []int, maxPulses int, length int) int {
	for k := 0; k < length; k++ {
		sum := pulsesIn[2*k] + pulsesIn[2*k+1]
		if sum > maxPulses {
			return 1
		}
		pulsesComb[k] = sum
	}
	return 0
}

// EncodePulses encodes quantization indices of excitation
//
// Parameters:
//
//	psRangeEnc: compressor data structure (pointer to entropy coder)
//	signalType: signal type
//	quantOffsetType: quantization offset type
//	pulses: quantization indices (as bytes)
//	frameLength: frame length
func (ep *EncodePulses) EncodePulses(
	psRangeEnc *EntropyCoder,
	signalType int,
	quantOffsetType int,
	pulses []byte,
	frameLength int,
) {
	var (
		i, k, j, iter, bit, nLS, scaleDown int
		absQ, minSumBitsQ5, sumBitsQ5      int
		rateLevelIndex                     int = 0
	)

	// Prepare for shell coding
	// Calculate number of shell blocks
	opusAssert(1<<LOG2_SHELL_CODEC_FRAME_LENGTH == SHELL_CODEC_FRAME_LENGTH, "frame length assertion failed")
	iter = frameLength >> LOG2_SHELL_CODEC_FRAME_LENGTH
	if iter*SHELL_CODEC_FRAME_LENGTH < frameLength {
		opusAssert(frameLength == 12*10, "unexpected frame length")
		// Make sure only happens for 10 ms @ 12 kHz
		iter++
		// Zero-pad the pulses array
		for i := frameLength; i < SHELL_CODEC_FRAME_LENGTH; i++ {
			pulses[i] = 0
		}
	}

	// Take the absolute value of the pulses
	absPulses := make([]int, iter*SHELL_CODEC_FRAME_LENGTH)
	opusAssert((SHELL_CODEC_FRAME_LENGTH&3) == 0, "frame length must be multiple of 4")

	// Process pulses in batches of 4 for efficiency
	for i := 0; i < iter*SHELL_CODEC_FRAME_LENGTH; i += 4 {
		absPulses[i+0] = int(math.Abs(float64(pulses[i+0])))
		absPulses[i+1] = int(math.Abs(float64(pulses[i+1])))
		absPulses[i+2] = int(math.Abs(float64(pulses[i+2])))
		absPulses[i+3] = int(math.Abs(float64(pulses[i+3])))
	}

	// Calculate sum pulses per shell code frame
	sumPulses := make([]int, iter)
	nRshifts := make([]int, iter)
	absPulsesPtr := 0
	pulsesComb := make([]int, 8) // Temporary buffer for combining pulses

	for i := 0; i < iter; i++ {
		nRshifts[i] = 0

		for {
			// 1+1 -> 2
			scaleDown = combineAndCheck(pulsesComb, 0, absPulses, absPulsesPtr, maxPulsesTable[0], 8)
			// 2+2 -> 4
			scaleDown += combineAndCheckNoPtr(pulsesComb, pulsesComb, maxPulsesTable[1], 4)
			// 4+4 -> 8
			scaleDown += combineAndCheckNoPtr(pulsesComb, pulsesComb, maxPulsesTable[2], 2)
			// 8+8 -> 16
			scaleDown += combineAndCheck(sumPulses, i, pulsesComb, 0, maxPulsesTable[3], 1)

			if scaleDown != 0 {
				// We need to downscale the quantization signal
				nRshifts[i]++
				for k := absPulsesPtr; k < absPulsesPtr+SHELL_CODEC_FRAME_LENGTH; k++ {
					absPulses[k] >>= 1
				}
			} else {
				// Break out of loop and go to next shell coding frame
				break
			}
		}
		absPulsesPtr += SHELL_CODEC_FRAME_LENGTH
	}

	// Rate level selection
	// Find rate level that leads to fewest bits for coding of pulses per block info
	minSumBitsQ5 = math.MaxInt32
	for k := 0; k < N_RATE_LEVELS-1; k++ {
		nBitsPtr := pulsesPerBlockBitsQ5[k]
		sumBitsQ5 := rateLevelsBitsQ5[signalType>>1][k]
		for i := 0; i < iter; i++ {
			if nRshifts[i] > 0 {
				sumBitsQ5 += nBitsPtr[SILK_MAX_PULSES+1]
			} else {
				sumBitsQ5 += nBitsPtr[sumPulses[i]]
			}
		}
		if sumBitsQ5 < minSumBitsQ5 {
			minSumBitsQ5 = sumBitsQ5
			rateLevelIndex = k
		}
	}

	psRangeEnc.EncIcdf(rateLevelIndex, rateLevelsICDF[signalType>>1], 8)

	// Sum-Weighted-Pulses Encoding
	for i := 0; i < iter; i++ {
		if nRshifts[i] == 0 {
			psRangeEnc.EncIcdf(sumPulses[i], pulsesPerBlockICDF[rateLevelIndex], 8)
		} else {
			psRangeEnc.EncIcdf(SILK_MAX_PULSES+1, pulsesPerBlockICDF[rateLevelIndex], 8)
			for k := 0; k < nRshifts[i]-1; k++ {
				psRangeEnc.EncIcdf(SILK_MAX_PULSES+1, pulsesPerBlockICDF[N_RATE_LEVELS-1], 8)
			}
			psRangeEnc.EncIcdf(sumPulses[i], pulsesPerBlockICDF[N_RATE_LEVELS-1], 8)
		}
	}

	// Shell Encoding
	for i := 0; i < iter; i++ {
		if sumPulses[i] > 0 {
			shellEncoder(psRangeEnc, absPulses, i*SHELL_CODEC_FRAME_LENGTH)
		}
	}

	// LSB Encoding
	for i := 0; i < iter; i++ {
		if nRshifts[i] > 0 {
			pulsesPtr := i * SHELL_CODEC_FRAME_LENGTH
			nLS := nRshifts[i] - 1
			for k := 0; k < SHELL_CODEC_FRAME_LENGTH; k++ {
				absQ = int(math.Abs(float64(pulses[pulsesPtr+k])))
				for j := nLS; j > 0; j-- {
					bit = (absQ >> j) & 1
					psRangeEnc.EncIcdf(bit, lsbICDF, 8)
				}
				bit = absQ & 1
				psRangeEnc.EncIcdf(bit, lsbICDF, 8)
			}
		}
	}

	// Encode signs
	encodeSigns(psRangeEnc, pulses, frameLength, signalType, quantOffsetType, sumPulses)
}

// opusAssert is a helper function for assertions
func opusAssert(condition bool, message string) {
	if !condition {
		panic(message)
	}
}
