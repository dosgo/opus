package concentus

import "math/bits"

// Constants
const (
	QC                  = 10
	QS                  = 14
	MAX_SHAPE_LPC_ORDER = 16 // Assuming this value from SilkConstants
)

// BoxedValueInt is a container for an integer that needs to be modified by reference
type BoxedValueInt struct {
	Val int
}

// Compute autocorrelation
func SilkAutocorr(
	results []int, // O: Result (length correlationCount)
	scale *BoxedValueInt, // O: Scaling of the correlation vector
	inputData []int16, // I: Input data to correlate
	inputDataSize int, // I: Length of input
	correlationCount int, // I: Number of correlation taps to compute
) {
	corrCount := min(inputDataSize, correlationCount)
	scale.Val = celtAutocorr(inputData, results, corrCount-1, inputDataSize)
}

// Internal autocorrelation function for int16 input
func celtAutocorr(
	x []int16, // in: [0...n-1] samples x
	ac []int, // out: [0...lag-1] ac values
	lag int,
	n int,
) int {
	fastN := n - lag
	shift := 0
	var xptr []int16

	// Assert equivalent - Go doesn't have assertions in production code
	if n <= 0 {
		panic("n must be positive")
	}
	xptr = x

	// Calculate initial shift value
	{
		ac0 := 1 + (n << 7)
		if (n & 1) != 0 {
			ac0 += shr32(mult16_16(int32(xptr[0]), int32(xptr[0]), 9))
		}
		for i := (n & 1); i < n; i += 2 {
			ac0 += shr32(mult16_16(int32(xptr[i]), int32(xptr[i])), 9)
			ac0 += shr32(mult16_16(int32(xptr[i+1]), int32(xptr[i+1])), 9)
		}
		shift = celtIlog2(ac0) - 30 + 10
		shift = shift / 2

		if shift > 0 {
			xx := make([]int16, n)
			for i := 0; i < n; i++ {
				xx[i] = int16(pshr32(int32(xptr[i]), shift))
			}
			xptr = xx
		} else {
			shift = 0
		}
	}

	// Compute pitch correlation
	pitchXcorr(xptr, xptr, ac, fastN, lag+1)

	// Compute remaining correlations
	for k := 0; k <= lag; k++ {
		d := int32(0)
		for i := k + fastN; i < n; i++ {
			d = mac16_16(d, xptr[i], xptr[i-k])
		}
		ac[k] += int(d)
	}

	// Final adjustments
	shift = 2 * shift
	if shift <= 0 {
		ac[0] += shl32(1, -shift)
	}

	if ac[0] < 268435456 {
		shift2 := 29 - ecIlog(ac[0])
		for i := 0; i <= lag; i++ {
			ac[i] = shl32(ac[i], shift2)
		}
		shift -= shift2
	} else if ac[0] >= 536870912 {
		shift2 := 1
		if ac[0] >= 1073741824 {
			shift2++
		}
		for i := 0; i <= lag; i++ {
			ac[i] = shr32(ac[i], shift2)
		}
		shift += shift2
	}

	return shift
}

// Autocorrelations for a warped frequency axis
func SilkWarpedAutocorrelation(
	corr []int, // O: Result [order + 1]
	scale *BoxedValueInt, // O: Scaling of the correlation vector
	input []int16, // I: Input data to correlate
	warpingQ16 int, // I: Warping coefficient
	length int, // I: Length of input
	order int, // I: Correlation order (even)
) {
	// Order must be even
	if (order & 1) != 0 {
		panic("order must be even")
	}
	if 2*QS-QC < 0 {
		panic("2*QS-QC must be >= 0")
	}

	var (
		tmp1QS, tmp2QS int32
		stateQS        = make([]int32, MAX_SHAPE_LPC_ORDER+1)
		corrQC         = make([]int64, MAX_SHAPE_LPC_ORDER+1)
	)

	// Loop over samples
	for n := 0; n < length; n++ {
		tmp1QS = int32(input[n]) << QS

		// Loop over allpass sections
		for i := 0; i < order; i += 2 {
			// Output of allpass section
			tmp2QS = smlawb(stateQS[i], stateQS[i+1]-tmp1QS, warpingQ16)
			stateQS[i] = tmp1QS
			corrQC[i] += smull(tmp1QS, stateQS[0]) >> (2*QS - QC)

			// Output of allpass section
			tmp1QS = smlawb(stateQS[i+1], stateQS[i+2]-tmp2QS, warpingQ16)
			stateQS[i+1] = tmp2QS
			corrQC[i+1] += smull(tmp2QS, stateQS[0]) >> (2*QS - QC)
		}
		stateQS[order] = tmp1QS
		corrQC[order] += smull(tmp1QS, stateQS[0]) >> (2*QS - QC)
	}

	lsh := clz64(corrQC[0]) - 35
	lsh = limit(lsh, -12-QC, 30-QC)
	scale.Val = -(QC + lsh)

	if lsh >= 0 {
		for i := 0; i < order+1; i++ {
			corr[i] = int(corrQC[i] << uint(lsh))
		}
	} else {
		for i := 0; i < order+1; i++ {
			corr[i] = int(corrQC[i] >> uint(-lsh))
		}
	}
}

// Helper functions that were in Inlines class in Java

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func mult16_16(a, b int32) int32 {
	return a * b
}

func shr32(a int32, shift int) int32 {
	return a >> uint(shift)
}

func shl32(a, shift int) int {
	return a << uint(shift)
}

func pshr32(a int32, shift int) int32 {
	if shift > 0 {
		return (a + (1 << uint(shift-1))) >> uint(shift)
	}
	return a
}

func mac16_16(acc int32, a, b int16) int32 {
	return acc + int32(a)*int32(b)
}

func smlawb(a, b, c int32) int32 {
	return a + ((b * c) >> 16)
}

func smull(a, b int32) int64 {
	return int64(a) * int64(b)
}

func celtIlog2(x int) int {
	if x <= 0 {
		return 0
	}
	return 31 - bits.LeadingZeros32(uint32(x))
}

func ecIlog(x int) int {
	if x <= 0 {
		return 0
	}
	return 31 - bits.LeadingZeros32(uint32(x))
}

func clz64(x int64) int {
	return bits.LeadingZeros64(uint64(x))
}

func limit(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// pitchXcorr is assumed to be implemented elsewhere
func pitchXcorr(x, y []int16, ac []int, len, lag int) {
	// Implementation would go here
	// This is a placeholder for the actual pitch correlation computation
}
