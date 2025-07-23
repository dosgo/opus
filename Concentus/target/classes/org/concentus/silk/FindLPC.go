package silk

// FindLPC finds LPC vector from correlations and converts to NLSF
// This is a translation from Java to idiomatic Go, with optimizations for Go's features
// and memory management.

// silk_find_LPC finds LPC vector from correlations and converts to NLSF
// Parameters:
//
//	psEncC - Encoder state (modified)
//	NLSF_Q15 - Output NLSFs
//	x - Input signal
//	minInvGain_Q30 - Inverse of max prediction gain
func silk_find_LPC(
	psEncC *SilkChannelEncoder,
	NLSF_Q15 []int16,
	x []int16,
	minInvGain_Q30 int,
) {
	const MAX_LPC_ORDER = 16 // Assuming this is the value from SilkConstants

	// Declare variables with more Go-idiomatic names
	var (
		k, subfrLength int
		a_Q16          [MAX_LPC_ORDER]int32
		isInterpLower  int
		shift          int
	)

	// Go doesn't need boxed values - we can use regular variables and return multiple values
	res_nrg0 := 0
	res_nrg1 := 0
	rshift0 := 0
	rshift1 := 0

	// Variables for LSF interpolation
	var (
		a_tmp_Q16        [MAX_LPC_ORDER]int32
		res_nrg_interp   int
		res_nrg          int
		res_tmp_nrg      int
		res_nrg_interp_Q int
		res_nrg_Q        int
		res_tmp_nrg_Q    int
		a_tmp_Q12        [MAX_LPC_ORDER]int16
		NLSF0_Q15        [MAX_LPC_ORDER]int16
	)

	subfrLength = psEncC.subfr_length + psEncC.predictLPCOrder

	// Default: no interpolation
	psEncC.indices.NLSFInterpCoef_Q2 = 4

	// Burg AR analysis for the full frame
	// In Go, we can return multiple values instead of using boxed types
	res_nrg, res_nrg_Q = BurgModified_silk_burg_modified(
		&a_Q16, x, 0, minInvGain_Q30, subfrLength, psEncC.nb_subfr, psEncC.predictLPCOrder,
	)

	// Check if we should use interpolated NLSFs
	if psEncC.useInterpolatedNLSFs != 0 && psEncC.first_frame_after_reset == 0 && psEncC.nb_subfr == MAX_NB_SUBFR {
		var LPC_res []int16

		// Optimal solution for last 10 ms
		res_tmp_nrg, res_tmp_nrg_Q = BurgModified_silk_burg_modified(
			&a_tmp_Q16, x, 2*subfrLength, minInvGain_Q30, subfrLength, 2, psEncC.predictLPCOrder,
		)

		// Subtract residual energy (easier than adding to first 10 ms)
		shift = res_tmp_nrg_Q - res_nrg_Q
		if shift >= 0 {
			if shift < 32 {
				res_nrg -= res_tmp_nrg >> shift
			}
		} else {
			// Equivalent to Java's OpusAssert
			if shift <= -32 {
				panic("shift out of bounds")
			}
			res_nrg = res_nrg>>(-shift) - res_tmp_nrg
			res_nrg_Q = res_tmp_nrg_Q
		}

		// Convert to NLSFs
		A2NLSF(NLSF_Q15, a_tmp_Q16[:], psEncC.predictLPCOrder)

		LPC_res = make([]int16, 2*subfrLength)

		// Search over interpolation indices to find lowest residual energy
		for k = 3; k >= 0; k-- {
			// Interpolate NLSFs for first half
			interpolate(NLSF0_Q15[:], psEncC.prev_NLSFq_Q15[:], NLSF_Q15, k, psEncC.predictLPCOrder)

			// Convert to LPC for residual energy evaluation
			NLSF2A(a_tmp_Q12[:], NLSF0_Q15[:], psEncC.predictLPCOrder)

			// Calculate residual energy with NLSF interpolation
			LPC_analysis_filter(LPC_res, 0, x, 0, a_tmp_Q12[:], 0, 2*subfrLength, psEncC.predictLPCOrder)

			// Calculate sum of squares with shift
			res_nrg0, rshift0 = sum_sqr_shift(LPC_res, psEncC.predictLPCOrder, subfrLength-psEncC.predictLPCOrder)
			res_nrg1, rshift1 = sum_sqr_shift(LPC_res, psEncC.predictLPCOrder+subfrLength, subfrLength-psEncC.predictLPCOrder)

			// Add subframe energies from first half frame
			shift = rshift0 - rshift1
			if shift >= 0 {
				res_nrg1 = res_nrg1 >> shift
				res_nrg_interp_Q = -rshift0
			} else {
				res_nrg0 = res_nrg0 >> (-shift)
				res_nrg_interp_Q = -rshift1
			}
			res_nrg_interp = res_nrg0 + res_nrg1

			// Compare with first half energy without NLSF interpolation
			shift = res_nrg_interp_Q - res_nrg_Q
			if shift >= 0 {
				if (res_nrg_interp >> shift) < res_nrg {
					isInterpLower = 1
				} else {
					isInterpLower = 0
				}
			} else if -shift < 32 {
				if res_nrg_interp < (res_nrg >> (-shift)) {
					isInterpLower = 1
				} else {
					isInterpLower = 0
				}
			} else {
				isInterpLower = 0
			}

			// Determine if current interpolated NLSFs are best so far
			if isInterpLower == 1 {
				// Interpolation has lower residual energy
				res_nrg = res_nrg_interp
				res_nrg_Q = res_nrg_interp_Q
				psEncC.indices.NLSFInterpCoef_Q2 = int8(k)
			}
		}
	}

	if psEncC.indices.NLSFInterpCoef_Q2 == 4 {
		// NLSF interpolation inactive - calculate NLSFs from full frame AR coefficients
		A2NLSF(NLSF_Q15, a_Q16[:], psEncC.predictLPCOrder)
	}

	// Assert equivalent - in Go we'd typically panic or log if this condition fails
	if !(psEncC.indices.NLSFInterpCoef_Q2 == 4 || (psEncC.useInterpolatedNLSFs != 0 && psEncC.first_frame_after_reset == 0 && psEncC.nb_subfr == MAX_NB_SUBFR)) {
		panic("invalid NLSF interpolation state")
	}
}
