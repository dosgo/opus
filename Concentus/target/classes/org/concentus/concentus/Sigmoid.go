package concentus

// Sigmoid provides an approximate sigmoid function implementation.
// This is a direct translation from the original Java code with Go optimizations.
type Sigmoid struct{}

// Lookup tables for sigmoid approximation (Q10 and Q15 fixed-point formats)
var (
	// sigm_LUT_slope_Q10 contains slope values in Q10 fixed-point format
	sigm_LUT_slope_Q10 = []int16{237, 153, 73, 30, 12, 7}

	// sigm_LUT_pos_Q15 contains positive sigmoid values in Q15 fixed-point format
	sigm_LUT_pos_Q15 = []int16{16384, 23955, 28861, 31213, 32178, 32548}

	// sigm_LUT_neg_Q15 contains negative sigmoid values in Q15 fixed-point format
	sigm_LUT_neg_Q15 = []int16{16384, 8812, 3906, 1554, 589, 219}
)

// SigmQ15 computes an approximate sigmoid function in Q15 fixed-point format.
// in_Q5: input value in Q5 fixed-point format (-96 to +96 corresponds to -3 to +3)
// Returns: sigmoid value in Q15 format (0 to 32767)
func (s *Sigmoid) SigmQ15(in_Q5 int32) int32 {
	// Using int32 for calculations to match Java's 32-bit integer behavior

	var ind int32

	if in_Q5 < 0 {
		// Negative input
		in_Q5 = -in_Q5
		if in_Q5 >= 6*32 {
			return 0 // Clip to minimum value
		}

		// Linear interpolation of look up table
		ind = in_Q5 >> 5 // Equivalent to silk_RSHIFT(in_Q5, 5)
		// Calculate: LUT_neg_Q15[ind] - (slope_Q10[ind] * (in_Q5 & 0x1F))
		return int32(sigm_LUT_neg_Q15[ind]) - ((int32(sigm_LUT_slope_Q10[ind]) * (in_Q5 & 0x1F)) >> 10)
	} else if in_Q5 >= 6*32 {
		return 32767 // Clip to maximum value (Q15 format)
	}

	// Positive input case
	// Linear interpolation of look up table
	ind = in_Q5 >> 5
	// Calculate: LUT_pos_Q15[ind] + (slope_Q10[ind] * (in_Q5 & 0x1F))
	return int32(sigm_LUT_pos_Q15[ind]) + ((int32(sigm_LUT_slope_Q10[ind]) * (in_Q5 & 0x1F)) >> 10)
}

// Note: In Go, we don't need a separate Inlines class since bit operations are built-in
