// Package silk implements SILK audio codec functionality.
package silk

// K2A contains functions for converting reflection coefficients to prediction coefficients.

// Step up function, converts reflection coefficients to prediction coefficients (Q15 input)
// A_Q24: Output prediction coefficients [order] Q24
// rc_Q15: Input reflection coefficients [order] Q15
// order: Prediction order
func K2A(A_Q24 []int32, rc_Q15 []int16, order int) {
	// Allocate temporary storage once (on stack for efficiency)
	var Atmp [SILK_MAX_ORDER_LPC]int32

	for k := 0; k < order; k++ {
		// Copy current coefficients to temporary storage
		for n := 0; n < k; n++ {
			Atmp[n] = A_Q24[n]
		}

		// Update coefficients
		for n := 0; n < k; n++ {
			// Equivalent to silk_SMLAWB: A_Q24[n] + ((Atmp[k-n-1] << 1) * rc_Q15[k]) >> 16)
			A_Q24[n] += ((Atmp[k-n-1] << 1) * int32(rc_Q15[k])) >> 16
		}

		// Set new coefficient
		A_Q24[k] = -int32(rc_Q15[k]) << 9
	}
}

// Step up function, converts reflection coefficients to prediction coefficients (Q16 input)
// A_Q24: Output prediction coefficients [order] Q24
// rc_Q16: Input reflection coefficients [order] Q16
// order: Prediction order
func K2AQ16(A_Q24 []int32, rc_Q16 []int32, order int) {
	// Allocate temporary storage once (on stack for efficiency)
	var Atmp [SILK_MAX_ORDER_LPC]int32

	for k := 0; k < order; k++ {
		// Copy current coefficients to temporary storage
		for n := 0; n < k; n++ {
			Atmp[n] = A_Q24[n]
		}

		// Update coefficients
		for n := 0; n < k; n++ {
			// Equivalent to silk_SMLAWW: A_Q24[n] + (Atmp[k-n-1] * rc_Q16[k]) >> 16
			A_Q24[n] += (Atmp[k-n-1] * rc_Q16[k]) >> 16
		}

		// Set new coefficient
		A_Q24[k] = -rc_Q16[k] << 8
	}
}
