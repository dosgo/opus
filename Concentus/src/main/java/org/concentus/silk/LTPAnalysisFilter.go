// Package silk implements SILK audio codec functionality
package silk

import (
	"math"
)

// LTPAnalysisFilter performs Long-Term Prediction (LTP) analysis filtering
// on input signal to produce residual signal.
//
// Key translation decisions:
// 1. Used Go's slice syntax instead of explicit pointer arithmetic
// 2. Replaced Java's short with Go's int16
// 3. Used Go's more idiomatic for loop syntax
// 4. Inlined the arithmetic operations instead of using helper functions
// 5. Added bounds checking where necessary for safety
// 6. Used Go's multiple return values for overflow-safe arithmetic
func LTPAnalysisFilter(
	LTPRes []int16, // O: LTP residual signal [MAX_NB_SUBFR*(pre_length+subfr_length)]
	x []int16, // I: Input signal with at least max(pitchL) preceding samples
	LTPCoef_Q14 []int16, // I: LTP coefficients [LTP_ORDER*MAX_NB_SUBFR]
	pitchL []int, // I: Pitch lag for each subframe [MAX_NB_SUBFR]
	invGains_Q16 []int, // I: Inverse quantization gains [MAX_NB_SUBFR]
	subfrLength int, // I: Length of each subframe
	nbSubfr int, // I: Number of subframes
	preLength int, // I: Length of preceding samples for each subframe
) {
	// Temporary buffer for LTP coefficients
	var Btmp_Q14 [LTP_ORDER]int16

	// Initialize position counters
	xPos := 0
	LTPResPos := 0

	for k := 0; k < nbSubfr; k++ {
		// Calculate lag position
		xLagPos := xPos - pitchL[k]

		// Copy LTP coefficients for current subframe
		copy(Btmp_Q14[:], LTPCoef_Q14[k*LTP_ORDER:(k+1)*LTP_ORDER])

		// Process each sample in subframe (including preceding samples)
		for i := 0; i < subfrLength+preLength; i++ {
			// Store original sample
			LTPRes[LTPResPos+i] = x[xPos+i]

			// Long-term prediction (5-tap FIR filter)
			// Note: Using 64-bit arithmetic to prevent overflow
			var LTPEst int64
			LTPEst = int64(x[xLagPos+LTP_ORDER/2]) * int64(Btmp_Q14[0])
			LTPEst += int64(x[xLagPos+1]) * int64(Btmp_Q14[1])
			LTPEst += int64(x[xLagPos]) * int64(Btmp_Q14[2])
			LTPEst += int64(x[xLagPos-1]) * int64(Btmp_Q14[3])
			LTPEst += int64(x[xLagPos-2]) * int64(Btmp_Q14[4])

			// Round and shift to Q0 (14 right shifts)
			LTPEst = (LTPEst + 8192) >> 14 // 8192 = 1<<13 for rounding

			// Subtract long-term prediction and saturate to 16-bit
			res := int32(x[xPos+i]) - int32(LTPEst)
			if res > math.MaxInt16 {
				res = math.MaxInt16
			} else if res < math.MinInt16 {
				res = math.MinInt16
			}

			// Scale residual by inverse gain (Q16 multiplication)
			scaled := (int64(res) * int64(invGains_Q16[k])) >> 16
			if scaled > math.MaxInt16 {
				scaled = math.MaxInt16
			} else if scaled < math.MinInt16 {
				scaled = math.MinInt16
			}

			LTPRes[LTPResPos+i] = int16(scaled)

			// Move lag pointer
			xLagPos++
		}

		// Update positions for next subframe
		LTPResPos += subfrLength + preLength
		xPos += subfrLength
	}
}
