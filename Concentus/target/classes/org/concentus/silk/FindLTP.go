package silk

import (
	"math"
)

const (
	// Head room for correlations
	ltpCorrsHeadRoom = 2
)

// FindLTP finds linear prediction coefficients and weights
//
// Parameters:
//
//	b_Q14: Output LTP coefficients [MAX_NB_SUBFR * LTP_ORDER]
//	WLTP: Output weight for LTP quantization [MAX_NB_SUBFR * LTP_ORDER * LTP_ORDER]
//	LTPredCodGain_Q7: Output LTP coding gain
//	r_lpc: Input residual signal after LPC signal + state for first 10 ms
//	lag: Input LTP lags [MAX_NB_SUBFR]
//	Wght_Q15: Input weights [MAX_NB_SUBFR]
//	subfr_length: Input subframe length
//	nb_subfr: Input number of subframes
//	mem_offset: Input number of samples in LTP memory
//	corr_rshifts: Output right shifts applied to correlations [MAX_NB_SUBFR]
func FindLTP(
	b_Q14 []int16,
	WLTP []int32,
	LTPredCodGain_Q7 *int32,
	r_lpc []int16,
	lag []int32,
	Wght_Q15 []int32,
	subfr_length int32,
	nb_subfr int32,
	mem_offset int32,
	corr_rshifts []int32,
) {
	var (
		i, k, lshift int32
		r_ptr        = mem_offset
		lag_ptr      int32
		b_Q14_ptr    int32
		WLTP_ptr     int32
		regu         int32
		g_Q26        int32
		wd           int32
		m_Q12        int32
	)

	// Temporary arrays
	b_Q16 := make([]int32, LTP_ORDER)
	delta_b_Q14 := make([]int32, LTP_ORDER)
	d_Q14 := make([]int32, MAX_NB_SUBFR)
	nrg := make([]int32, MAX_NB_SUBFR)
	w := make([]int32, MAX_NB_SUBFR)
	Rr := make([]int32, LTP_ORDER)
	rr := make([]int32, MAX_NB_SUBFR)

	var (
		temp32, denom32       int32
		extra_shifts          int32
		rr_shifts, maxRshifts int32
		maxRshifts_wxtra      int32
		LZs                   int32
		LPC_res_nrg           int32
		LPC_LTP_res_nrg       int32
		div_Q16               int32
		max_abs_d_Q14         int32
		max_w_bits            int32
		WLTP_max              int32
	)

	b_Q14_ptr = 0
	WLTP_ptr = 0
	for k = 0; k < nb_subfr; k++ {
		lag_ptr = r_ptr - (lag[k] + LTP_ORDER/2)

		// Calculate sum of squares and shift
		rr_val, rr_shift := SumSqrShift(r_lpc, r_ptr, subfr_length)
		rr[k] = rr_val
		rr_shifts = rr_shift

		// Assure headroom
		LZs = CLZ32(rr[k])
		if LZs < ltpCorrsHeadRoom {
			rr[k] = RSHIFT_ROUND(rr[k], ltpCorrsHeadRoom-LZs)
			rr_shifts += (ltpCorrsHeadRoom - LZs)
		}
		corr_rshifts[k] = rr_shifts

		// Calculate correlation matrix
		shifts := corr_rshifts[k]
		CorrMatrix(r_lpc, lag_ptr, subfr_length, LTP_ORDER, ltpCorrsHeadRoom, WLTP[WLTP_ptr:], &shifts)
		corr_rshifts[k] = shifts

		// Calculate correlation vector
		CorrVector(r_lpc, lag_ptr, r_lpc, r_ptr, subfr_length, LTP_ORDER, Rr, corr_rshifts[k])

		if corr_rshifts[k] > rr_shifts {
			rr[k] = RSHIFT(rr[k], corr_rshifts[k]-rr_shifts)
		}
		Assert(rr[k] >= 0, "rr[k] must be non-negative")

		// Regularization
		regu = 1
		regu = SMLAWB(regu, rr[k], int32((LTP_DAMPING/3)*(1<<16)+0.5))
		regu = SMLAWB(regu, MatrixGet(WLTP, WLTP_ptr, 0, 0, LTP_ORDER), int32((LTP_DAMPING/3)*(1<<16)+0.5))
		regu = SMLAWB(regu, MatrixGet(WLTP, WLTP_ptr, LTP_ORDER-1, LTP_ORDER-1, LTP_ORDER), int32((LTP_DAMPING/3)*(1<<16)+0.5))
		RegularizeCorrelations(WLTP[WLTP_ptr:], rr, k, regu, LTP_ORDER)

		// Solve linear system
		SolveLDL(WLTP[WLTP_ptr:], LTP_ORDER, Rr, b_Q16)

		// Limit and store in Q14
		fitLTP(b_Q16, b_Q14[b_Q14_ptr:])

		// Calculate residual energy
		nrg[k] = ResidualEnergy16Covar(b_Q14[b_Q14_ptr:], WLTP[WLTP_ptr:], Rr, rr[k], LTP_ORDER, 14)

		// Calculate weight
		extra_shifts = MinInt(corr_rshifts[k], ltpCorrsHeadRoom)
		denom32 = LSHIFT_SAT32(SMULWB(nrg[k], Wght_Q15[k]), 1+extra_shifts) +
			RSHIFT(SMULWB(subfr_length, 655), corr_rshifts[k]-extra_shifts)
		denom32 = MaxInt32(denom32, 1)
		Assert(int64(Wght_Q15[k])<<16 < math.MaxInt32, "Wght_Q15 too large")

		temp32 = DIV32(LSHIFT(Wght_Q15[k], 16), denom32)
		temp32 = RSHIFT(temp32, 31+corr_rshifts[k]-extra_shifts-26)

		// Limit temp to prevent overflow
		WLTP_max = 0
		for i = WLTP_ptr; i < WLTP_ptr+(LTP_ORDER*LTP_ORDER); i++ {
			WLTP_max = MaxInt32(WLTP[i], WLTP_max)
		}
		lshift = CLZ32(WLTP_max) - 1 - 3 // keep 3 bits free
		Assert(26-18+lshift >= 0, "shift underflow")
		if 26-18+lshift < 31 {
			temp32 = MinInt32(temp32, LSHIFT(1, 26-18+lshift))
		}

		// Scale vector
		ScaleVector32Q26Lshift18(WLTP[WLTP_ptr:], temp32, LTP_ORDER*LTP_ORDER)

		w[k] = MatrixGet(WLTP, WLTP_ptr, LTP_ORDER/2, LTP_ORDER/2, LTP_ORDER)
		Assert(w[k] >= 0, "w[k] must be non-negative")

		r_ptr += subfr_length
		b_Q14_ptr += LTP_ORDER
		WLTP_ptr += (LTP_ORDER * LTP_ORDER)
	}

	// Find maximum right shift
	maxRshifts = 0
	for k = 0; k < nb_subfr; k++ {
		maxRshifts = MaxInt(corr_rshifts[k], maxRshifts)
	}

	// Compute LTP coding gain
	if LTPredCodGain_Q7 != nil {
		LPC_res_nrg = 0
		LPC_LTP_res_nrg = 0
		Assert(ltpCorrsHeadRoom >= 2, "headroom too small")

		for k = 0; k < nb_subfr; k++ {
			LPC_res_nrg = ADD32(LPC_res_nrg, RSHIFT(ADD32(SMULWB(rr[k], Wght_Q15[k]), 1), 1+(maxRshifts-corr_rshifts[k])))
			LPC_LTP_res_nrg = ADD32(LPC_LTP_res_nrg, RSHIFT(ADD32(SMULWB(nrg[k], Wght_Q15[k]), 1), 1+(maxRshifts-corr_rshifts[k])))
		}
		LPC_LTP_res_nrg = MaxInt32(LPC_LTP_res_nrg, 1)

		div_Q16 = DIV32VarQ(LPC_res_nrg, LPC_LTP_res_nrg, 16)
		*LTPredCodGain_Q7 = int32(SMULBB(3, Lin2Log(div_Q16)-(16<<7)))

		Assert(*LTPredCodGain_Q7 == int32(SAT16(MUL(3, Lin2Log(div_Q16)-(16<<7)))), "coding gain overflow")
	}

	// Smoothing
	b_Q14_ptr = 0
	for k = 0; k < nb_subfr; k++ {
		d_Q14[k] = 0
		for i = b_Q14_ptr; i < b_Q14_ptr+LTP_ORDER; i++ {
			d_Q14[k] += int32(b_Q14[i])
		}
		b_Q14_ptr += LTP_ORDER
	}

	// Find maximum absolute value and bits used
	max_abs_d_Q14 = 0
	max_w_bits = 0
	for k = 0; k < nb_subfr; k++ {
		max_abs_d_Q14 = MaxInt32(max_abs_d_Q14, Abs32(d_Q14[k]))
		max_w_bits = MaxInt32(max_w_bits, 32-CLZ32(w[k])+corr_rshifts[k]-maxRshifts)
	}

	Assert(max_abs_d_Q14 <= (5<<15), "d_Q14 too large")

	// Calculate extra shifts needed
	extra_shifts = max_w_bits + 32 - CLZ32(max_abs_d_Q14) - 14
	extra_shifts -= (32 - 1 - 2 + maxRshifts)
	extra_shifts = MaxInt(extra_shifts, 0)

	maxRshifts_wxtra = maxRshifts + extra_shifts

	temp32 = RSHIFT(262, maxRshifts+extra_shifts) + 1
	wd = 0
	for k = 0; k < nb_subfr; k++ {
		temp32 = ADD32(temp32, RSHIFT(w[k], maxRshifts_wxtra-corr_rshifts[k]))
		wd = ADD32(wd, LSHIFT(SMULWW(RSHIFT(w[k], maxRshifts_wxtra-corr_rshifts[k]), d_Q14[k]), 2))
	}
	m_Q12 = DIV32VarQ(wd, temp32, 12)

	// Apply smoothing
	b_Q14_ptr = 0
	for k = 0; k < nb_subfr; k++ {
		if 2-corr_rshifts[k] > 0 {
			temp32 = RSHIFT(w[k], 2-corr_rshifts[k])
		} else {
			temp32 = LSHIFT_SAT32(w[k], corr_rshifts[k]-2)
		}

		g_Q26 = MUL(
			DIV32(
				int32((LTP_SMOOTHING)*(1<<26)+0.5),
				RSHIFT(int32((LTP_SMOOTHING)*(1<<26)+0.5), 10)+temp32),
			LSHIFT_SAT32(SUB_SAT32(m_Q12, RSHIFT(d_Q14[k], 2)), 4))

		temp32 = 0
		for i = 0; i < LTP_ORDER; i++ {
			delta_b_Q14[i] = MaxInt16(b_Q14[b_Q14_ptr+i], 1638) // 0.1_Q0
			temp32 += delta_b_Q14[i]
		}
		temp32 = DIV32(g_Q26, temp32)
		for i = 0; i < LTP_ORDER; i++ {
			b_Q14[b_Q14_ptr+i] = int16(LIMIT_32(int32(b_Q14[b_Q14_ptr+i])+SMULWB(LSHIFT_SAT32(temp32, 4), delta_b_Q14[i]), -16000, 28000))
		}
		b_Q14_ptr += LTP_ORDER
	}
}

// fitLTP limits LTP coefficients to 16-bit range
func fitLTP(LTP_coefs_Q16 []int32, LTP_coefs_Q14 []int16) {
	for i := 0; i < LTP_ORDER; i++ {
		LTP_coefs_Q14[i] = int16(SAT16(RSHIFT_ROUND(LTP_coefs_Q16[i], 2)))
	}
}
