package schur

import (
	"math"
)

const (
	// SILK_MAX_ORDER_LPC is the maximum order for Linear Predictive Coding
	SILK_MAX_ORDER_LPC = 16
)

// Schur computes reflection coefficients (16-bit fixed point) and residual energy
// This is the faster but less accurate version of the algorithm.
func Schur(rcQ15 []int16, c []int32, order int) int32 {
	// Validate input order
	if !(order == 6 || order == 8 || order == 10 || order == 12 || order == 14 || order == 16) {
		panic("invalid order, must be one of 6,8,10,12,14,16")
	}

	// Initialize correlation matrix with two columns
	C := make([][2]int32, SILK_MAX_ORDER_LPC+1)

	// Get number of leading zeros
	lz := clz32(c[0])

	// Copy correlations and adjust level to Q30
	switch {
	case lz < 2:
		// lz must be 1, so shift one to the right
		for k := 0; k < order+1; k++ {
			C[k][0], C[k][1] = c[k]>>1, c[k]>>1
		}
	case lz > 2:
		// Shift to the left
		lz -= 2
		for k := 0; k < order+1; k++ {
			C[k][0], C[k][1] = c[k]<<lz, c[k]<<lz
		}
	default:
		// No need to shift
		for k := 0; k < order+1; k++ {
			C[k][0], C[k][1] = c[k], c[k]
		}
	}

	for k := 0; k < order; k++ {
		// Check for unstable reflection coefficient
		if abs32(C[k+1][0]) >= C[0][1] {
			if C[k+1][0] > 0 {
				rcQ15[k] = int16(-(int32(0.99*(1<<15) + 0.5)))
			} else {
				rcQ15[k] = int16(int32(0.99*(1<<15) + 0.5))
			}
			k++
			break
		}

		// Get reflection coefficient
		denom := max32(C[0][1]>>15, 1)
		rcTmpQ15 := -div32_16(C[k+1][0], denom)

		// Clip (shouldn't happen for properly conditioned inputs)
		rcTmpQ15 = sat16(rcTmpQ15)

		// Store
		rcQ15[k] = int16(rcTmpQ15)

		// Update correlations
		for n := 0; n < order-k; n++ {
			Ctmp1 := C[n+k+1][0]
			Ctmp2 := C[n][1]
			C[n+k+1][0] = smlabb(Ctmp1, Ctmp2<<1, rcTmpQ15)
			C[n][1] = smlabb(Ctmp2, Ctmp1<<1, rcTmpQ15)
		}
	}

	// Zero out remaining coefficients if we exited early
	for ; k < order; k++ {
		rcQ15[k] = 0
	}

	// Return residual energy
	return max32(1, C[0][1])
}

// Schur64 computes reflection coefficients (32-bit fixed point) and residual energy
// This is the slower but more accurate version of the algorithm.
func Schur64(rcQ16 []int32, c []int32, order int) int32 {
	// Validate input order
	if !(order == 6 || order == 8 || order == 10 || order == 12 || order == 14 || order == 16) {
		panic("invalid order, must be one of 6,8,10,12,14,16")
	}

	// Check for invalid input
	if c[0] <= 0 {
		for i := range rcQ16 {
			rcQ16[i] = 0
		}
		return 0
	}

	// Initialize correlation matrix with two columns
	C := make([][2]int32, SILK_MAX_ORDER_LPC+1)
	for k := 0; k < order+1; k++ {
		C[k][0], C[k][1] = c[k], c[k]
	}

	for k := 0; k < order; k++ {
		// Check for unstable reflection coefficient
		if abs32(C[k+1][0]) >= C[0][1] {
			if C[k+1][0] > 0 {
				rcQ16[k] = -int32(0.99*(1<<16) + 0.5)
			} else {
				rcQ16[k] = int32(0.99*(1<<16) + 0.5)
			}
			k++
			break
		}

		// Get reflection coefficient: divide two Q30 values and get result in Q31
		rcTmpQ31 := div32VarQ(-C[k+1][0], C[0][1], 31)

		// Save the output (rounding from Q31 to Q16)
		rcQ16[k] = roundShift32(rcTmpQ31, 15)

		// Update correlations
		for n := 0; n < order-k; n++ {
			Ctmp1 := C[n+k+1][0]
			Ctmp2 := C[n][1]

			// Multiply and add the highest int32
			C[n+k+1][0] = Ctmp1 + smmul(Ctmp2<<1, rcTmpQ31)
			C[n][1] = Ctmp2 + smmul(Ctmp1<<1, rcTmpQ31)
		}
	}

	// Zero out remaining coefficients if we exited early
	for ; k < order; k++ {
		rcQ16[k] = 0
	}

	// Return residual energy
	return max32(1, C[0][1])
}

// Helper functions

// clz32 counts leading zeros in a 32-bit integer
func clz32(x int32) int {
	if x == 0 {
		return 32
	}
	return 31 - int(math.Floor(math.Log2(float64(x))))
}

// abs32 returns absolute value of a 32-bit integer
func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// max32 returns the maximum of two 32-bit integers
func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

// div32_16 divides a 32-bit integer by a 16-bit integer
func div32_16(a int32, b int32) int32 {
	return a / b
}

// sat16 saturates a 32-bit integer to 16-bit range
func sat16(x int32) int32 {
	if x > 32767 {
		return 32767
	}
	if x < -32768 {
		return -32768
	}
	return x
}

// smlabb performs multiply-accumulate operation (a + b * c)
func smlabb(a, b, c int32) int32 {
	return a + (b * c >> 15)
}

// div32VarQ divides two fixed-point numbers with variable Q format
func div32VarQ(a, b int32, q int) int32 {
	// Scale numerator to 64 bits to avoid overflow
	a64 := int64(a) << 32
	b64 := int64(b)

	// Perform division and scale back to desired Q format
	return int32((a64 / b64) >> (32 - q))
}

// roundShift32 rounds and shifts a 32-bit integer
func roundShift32(x int32, shift int) int32 {
	round := int32(1 << (shift - 1))
	return (x + round) >> shift
}

// smmul performs signed multiply returning most significant 32 bits
func smmul(a, b int32) int32 {
	return int32((int64(a) * int64(b)) >> 32)
}
