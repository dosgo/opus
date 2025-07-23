package concentus

import (
	"math"
)

// VQ_WMat_EC performs entropy constrained matrix-weighted VQ for 5-element vectors.
// This is a direct translation from Java to Go with idiomatic Go practices applied.
func VQ_WMat_EC(
	ind *byte, // O: index of best codebook vector
	rateDistQ14 *int32, // O: best weighted quant error + mu*rate
	gainQ7 *int32, // O: sum of absolute LTP coefficients
	inQ14 []int16, // I: input vector to be quantized
	inQ14Ptr int, // I: input vector pointer (not needed in Go slices)
	WQ18 []int32, // I: weighting matrix
	WQ18Ptr int, // I: weighting matrix pointer (not needed in Go slices)
	cbQ7 [][]byte, // I: codebook
	cbGainQ7 []int16, // I: codebook effective gain
	clQ5 []int16, // I: code length for each codebook vector
	muQ9 int32, // I: tradeoff between weighted error and rate
	maxGainQ7 int32, // I: maximum sum of absolute LTP coefficients
	L int, // I: number of vectors in codebook
) {
	// Initialize with maximum possible value
	*rateDistQ14 = math.MaxInt32

	// Loop over codebook
	for k := 0; k < L; k++ {
		cbRowQ7 := cbQ7[k] // Go slices make pointer arithmetic unnecessary
		gainTmpQ7 := int32(cbGainQ7[k])

		// Calculate difference vector (input - codebook entry)
		diffQ14 := [5]int16{
			inQ14[inQ14Ptr] - int16(cbRowQ7[0]<<7),
			inQ14[inQ14Ptr+1] - int16(cbRowQ7[1]<<7),
			inQ14[inQ14Ptr+2] - int16(cbRowQ7[2]<<7),
			inQ14[inQ14Ptr+3] - int16(cbRowQ7[3]<<7),
			inQ14[inQ14Ptr+4] - int16(cbRowQ7[4]<<7),
		}

		// Weighted rate
		sum1Q14 := SMULBB(muQ9, int32(clQ5[k]))

		// Penalty for too large gain
		penalty := gainTmpQ7 - maxGainQ7
		if penalty < 0 {
			penalty = 0
		}
		sum1Q14 = ADD_LSHIFT32(sum1Q14, penalty, 10)

		// The following assertions would be replaced with proper error handling in production code
		// assert(sum1Q14 >= 0)

		// Matrix multiplication with weighting matrix W_Q18
		// Each section corresponds to a row in the matrix

		// First row of W_Q18
		sum2Q16 := SMULWB(WQ18[WQ18Ptr+1], int32(diffQ14[1]))
		sum2Q16 = SMLAWB(sum2Q16, WQ18[WQ18Ptr+2], int32(diffQ14[2]))
		sum2Q16 = SMLAWB(sum2Q16, WQ18[WQ18Ptr+3], int32(diffQ14[3]))
		sum2Q16 = SMLAWB(sum2Q16, WQ18[WQ18Ptr+4], int32(diffQ14[4]))
		sum2Q16 = LSHIFT32(sum2Q16, 1)
		sum2Q16 = SMLAWB(sum2Q16, WQ18[WQ18Ptr], int32(diffQ14[0]))
		sum1Q14 = SMLAWB(sum1Q14, sum2Q16, int32(diffQ14[0]))

		// Second row of W_Q18
		sum2Q16 = SMULWB(WQ18[WQ18Ptr+7], int32(diffQ14[2]))
		sum2Q16 = SMLAWB(sum2Q16, WQ18[WQ18Ptr+8], int32(diffQ14[3]))
		sum2Q16 = SMLAWB(sum2Q16, WQ18[WQ18Ptr+9], int32(diffQ14[4]))
		sum2Q16 = LSHIFT32(sum2Q16, 1)
		sum2Q16 = SMLAWB(sum2Q16, WQ18[WQ18Ptr+6], int32(diffQ14[1]))
		sum1Q14 = SMLAWB(sum1Q14, sum2Q16, int32(diffQ14[1]))

		// Third row of W_Q18
		sum2Q16 = SMULWB(WQ18[WQ18Ptr+13], int32(diffQ14[3]))
		sum2Q16 = SMLAWB(sum2Q16, WQ18[WQ18Ptr+14], int32(diffQ14[4]))
		sum2Q16 = LSHIFT32(sum2Q16, 1)
		sum2Q16 = SMLAWB(sum2Q16, WQ18[WQ18Ptr+12], int32(diffQ14[2]))
		sum1Q14 = SMLAWB(sum1Q14, sum2Q16, int32(diffQ14[2]))

		// Fourth row of W_Q18
		sum2Q16 = SMULWB(WQ18[WQ18Ptr+19], int32(diffQ14[4]))
		sum2Q16 = LSHIFT32(sum2Q16, 1)
		sum2Q16 = SMLAWB(sum2Q16, WQ18[WQ18Ptr+18], int32(diffQ14[3]))
		sum1Q14 = SMLAWB(sum1Q14, sum2Q16, int32(diffQ14[3]))

		// Last row of W_Q18
		sum2Q16 = SMULWB(WQ18[WQ18Ptr+24], int32(diffQ14[4]))
		sum1Q14 = SMLAWB(sum1Q14, sum2Q16, int32(diffQ14[4]))

		// assert(sum1Q14 >= 0)

		// Find best match
		if sum1Q14 < *rateDistQ14 {
			*rateDistQ14 = sum1Q14
			*ind = byte(k)
			*gainQ7 = gainTmpQ7
		}
	}
}

// Helper functions that replicate the Java Inlines class operations

// SMULBB implements signed multiply (16x16->32) of two bottom 16-bit values
func SMULBB(a, b int32) int32 {
	return int32(int16(a)) * int32(int16(b))
}

// SMULWB implements signed multiply (32x16->32) with rounding
func SMULWB(a, b int32) int32 {
	return (a * (b >> 16)) + ((a * (b & 0xFFFF)) >> 16)
}

// SMLAWB implements multiply-accumulate (32x16->32) with rounding
func SMLAWB(a, b, c int32) int32 {
	return a + ((b * (c >> 16)) + (b*(c&0xFFFF))>>16)
}

// ADD_LSHIFT32 implements addition with left shift
func ADD_LSHIFT32(a, b int32, shift int) int32 {
	return a + (b << shift)
}

// LSHIFT32 implements logical left shift
func LSHIFT32(a int32, shift int) int32 {
	return a << shift
}
