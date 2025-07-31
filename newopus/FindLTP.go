package opus

func silk_find_LTP(b_Q14 []int16, WLTP []int, LTPredCodGain_Q7 BoxedValueInt, r_lpc []int16, lag []int, Wght_Q15 []int, subfr_length int, nb_subfr int, mem_offset int, corr_rshifts []int) {
	const LTP_CORRS_HEAD_ROOM = 2

	var i, k, lshift int
	r_ptr := mem_offset
	b_Q14_ptr := 0
	WLTP_ptr := 0

	b_Q16 := make([]int, LTP_ORDER)
	delta_b_Q14 := make([]int, LTP_ORDER)
	d_Q14 := make([]int, MAX_NB_SUBFR)
	nrg := make([]int, MAX_NB_SUBFR)
	w := make([]int, MAX_NB_SUBFR)
	var g_Q26, WLTP_max, max_abs_d_Q14, max_w_bits int
	var temp32, denom32, extra_shifts int
	var rr_shifts, maxRshifts, maxRshifts_wxtra, LZs int
	var LPC_res_nrg, LPC_LTP_res_nrg, div_Q16 int
	Rr := make([]int, LTP_ORDER)
	rr := make([]int, MAX_NB_SUBFR)
	var wd, m_Q12 int

	for k = 0; k < nb_subfr; k++ {
		lag_ptr := r_ptr - int(lag[k]) - LTP_ORDER/2
		boxed_rr := BoxedValueInt{0}
		boxed_rr_shift := BoxedValueInt{0}
		//silk_sum_sqr_shift5(&rr_val, &rr_shift, r_lpc[r_ptr:], subfr_length)
		silk_sum_sqr_shift5(&boxed_rr, &boxed_rr_shift, r_lpc, r_ptr, subfr_length)
		rr[k] = boxed_rr.Val
		rr_shifts = int(boxed_rr_shift.Val)

		LZs = silk_CLZ32(rr[k])
		if LZs < LTP_CORRS_HEAD_ROOM {
			rr[k] = silk_RSHIFT_ROUND(rr[k], LTP_CORRS_HEAD_ROOM-LZs)
			rr_shifts += LTP_CORRS_HEAD_ROOM - LZs
		}
		corr_rshifts[k] = int(rr_shifts)

		boxed_shifts := BoxedValueInt{corr_rshifts[k]}
		CorrelateMatrix.silk_corrMatrix(r_lpc, lag_ptr, subfr_length, SilkConstants.LTP_ORDER, LTP_CORRS_HEAD_ROOM, WLTP, WLTP_ptr, &boxed_shifts)

		corr_rshifts[k] = boxed_shifts.Val

		//	CorrelateMatrix.silk_corrVector(r_lpc[lag_ptr:], r_lpc[r_ptr:], subfr_length, LTP_ORDER, Rr, int(corr_rshifts[k]))
		CorrelateMatrix.silk_corrVector(r_lpc, lag_ptr, r_lpc, r_ptr, subfr_length, SilkConstants.LTP_ORDER, Rr, corr_rshifts[k])
		if int(corr_rshifts[k]) > rr_shifts {
			rr[k] = silk_RSHIFT(rr[k], int(corr_rshifts[k])-rr_shifts)
		}

		regu := int(1)
		regu = silk_SMLAWB(regu, rr[k], SILK_CONST(TuningParameters.LTP_DAMPING/3, 16))
		regu = silk_SMLAWB(regu, MatrixGetPtr(WLTP, WLTP_ptr, 0, 0, LTP_ORDER), SILK_CONST(TuningParameters.LTP_DAMPING/3, 16))
		regu = silk_SMLAWB(regu, MatrixGetPtr(WLTP, WLTP_ptr, LTP_ORDER-1, LTP_ORDER-1, LTP_ORDER), SILK_CONST(TuningParameters.LTP_DAMPING/3, 16))

		silk_regularize_correlations(WLTP, WLTP_ptr, rr, k, regu, SilkConstants.LTP_ORDER)
		silk_solve_LDL(WLTP, WLTP_ptr, SilkConstants.LTP_ORDER, Rr, b_Q16)

		silk_fit_LTP(b_Q16, b_Q14, b_Q14_ptr)

		nrg[k] = silk_residual_energy16_covar(b_Q14, b_Q14_ptr, WLTP, WLTP_ptr, Rr, rr[k], SilkConstants.LTP_ORDER, 14)
		extra_shifts = silk_min_int(corr_rshifts[k], LTP_CORRS_HEAD_ROOM)
		denom32 = silk_LSHIFT_SAT32(silk_SMULWB(nrg[k], Wght_Q15[k]), 1+extra_shifts) + silk_RSHIFT(silk_SMULWB(int(subfr_length), 655), int(corr_rshifts[k])-extra_shifts)
		if denom32 < 1 {
			denom32 = 1
		}
		temp32 = silk_DIV32(silk_LSHIFT(int(Wght_Q15[k]), 16), denom32)
		temp32 = silk_RSHIFT(temp32, 31+int(corr_rshifts[k])-extra_shifts-26)

		WLTP_max = 0
		for i = WLTP_ptr; i < WLTP_ptr+LTP_ORDER*LTP_ORDER; i++ {
			if WLTP[i] > WLTP_max {
				WLTP_max = WLTP[i]
			}
		}
		lshift = silk_CLZ32(WLTP_max) - 1 - 3
		if 26-18+lshift < 31 {
			max_val := int(1) << (26 - 18 + lshift)
			if temp32 > max_val {
				temp32 = max_val
			}
		}

		silk_scale_vector32_Q26_lshift_18(WLTP, WLTP_ptr, temp32, SilkConstants.LTP_ORDER*SilkConstants.LTP_ORDER)

		//w[k] = MatrixGet(WLTP, WLTP_ptr, LTP_ORDER/2, LTP_ORDER/2, LTP_ORDER)
		w[k] = MatrixGetPtr(WLTP, WLTP_ptr, SilkConstants.LTP_ORDER/2, SilkConstants.LTP_ORDER/2, SilkConstants.LTP_ORDER)
		r_ptr += subfr_length
		b_Q14_ptr += LTP_ORDER
		WLTP_ptr += LTP_ORDER * LTP_ORDER
	}

	maxRshifts = 0
	for k = 0; k < nb_subfr; k++ {
		shift := int(corr_rshifts[k])
		if shift > maxRshifts {
			maxRshifts = shift
		}
	}

	if &LTPredCodGain_Q7 != nil {
		LPC_res_nrg = 0
		LPC_LTP_res_nrg = 0
		for k = 0; k < nb_subfr; k++ {
			LPC_res_nrg += silk_RSHIFT(silk_SMULWB(rr[k], Wght_Q15[k])+1, 1+(maxRshifts-int(corr_rshifts[k])))
			LPC_LTP_res_nrg += silk_RSHIFT(silk_SMULWB(nrg[k], Wght_Q15[k])+1, 1+(maxRshifts-int(corr_rshifts[k])))
		}
		if LPC_LTP_res_nrg < 1 {
			LPC_LTP_res_nrg = 1
		}
		div_Q16 = silk_DIV32_varQ(LPC_res_nrg, LPC_LTP_res_nrg, 16)
		LTPredCodGain_Q7.Val = int(silk_SMULBB(3, silk_lin2log(div_Q16)-(16<<7)))
	}

	b_Q14_ptr = 0
	for k = 0; k < nb_subfr; k++ {
		d_Q14[k] = 0
		for i = 0; i < LTP_ORDER; i++ {
			d_Q14[k] += int(b_Q14[b_Q14_ptr+i])
		}
		b_Q14_ptr += LTP_ORDER
	}

	max_abs_d_Q14 = 0
	max_w_bits = 0
	for k = 0; k < nb_subfr; k++ {
		abs_d := d_Q14[k]
		if abs_d < 0 {
			abs_d = -abs_d
		}
		if abs_d > max_abs_d_Q14 {
			max_abs_d_Q14 = abs_d
		}
		bits := 32 - silk_CLZ32(w[k]) + int(corr_rshifts[k]) - maxRshifts
		if bits > max_w_bits {
			max_w_bits = bits
		}
	}

	extra_shifts = max_w_bits + 32 - silk_CLZ32(max_abs_d_Q14) - 14
	extra_shifts -= 32 - 1 - 2 + maxRshifts
	if extra_shifts < 0 {
		extra_shifts = 0
	}
	maxRshifts_wxtra = maxRshifts + extra_shifts

	temp32 = silk_RSHIFT(262, maxRshifts_wxtra) + 1
	wd = 0
	for k = 0; k < nb_subfr; k++ {
		w_shifted := w[k] >> (maxRshifts_wxtra - int(corr_rshifts[k]))
		temp32 += w_shifted
		wd += silk_LSHIFT(silk_SMULWW(w_shifted, d_Q14[k]), 2)
	}
	m_Q12 = silk_DIV32_varQ(wd, int(temp32), 12)

	b_Q14_ptr = 0
	for k = 0; k < nb_subfr; k++ {
		var temp32 int
		if 2-int(corr_rshifts[k]) > 0 {
			temp32 = w[k] >> (2 - int(corr_rshifts[k]))
		} else {
			temp32 = silk_LSHIFT_SAT32(w[k], int(corr_rshifts[k])-2)
		}

		g_Q26 = silk_MUL(
			silk_DIV32(
				SILK_CONST(TuningParameters.LTP_SMOOTHING, 26),
				silk_RSHIFT(SILK_CONST(TuningParameters.LTP_SMOOTHING, 26), 10)+temp32,
			),
			silk_LSHIFT_SAT32(m_Q12-silk_RSHIFT(d_Q14[k], 2), 4),
		)

		temp32 = 0
		for i = 0; i < LTP_ORDER; i++ {
			delta_b_Q14[i] = int(silk_max_16(b_Q14[b_Q14_ptr+i], 1638))
			temp32 += delta_b_Q14[i]
		}
		temp32 = silk_DIV32(g_Q26, temp32)
		for i = 0; i < LTP_ORDER; i++ {
			sum := int(b_Q14[b_Q14_ptr+i]) + silk_SMULWB(silk_LSHIFT_SAT32(temp32, 4), delta_b_Q14[i])
			if sum < -16000 {
				sum = -16000
			} else if sum > 28000 {
				sum = 28000
			}
			b_Q14[b_Q14_ptr+i] = int16(sum)
		}
		b_Q14_ptr += LTP_ORDER
	}
}

func silk_fit_LTP(LTP_coefs_Q16 []int, LTP_coefs_Q14 []int16, LTP_coefs_Q14_ptr int) {
	for i := 0; i < LTP_ORDER; i++ {
		val := silk_RSHIFT_ROUND(int(LTP_coefs_Q16[i]), 2)
		if val < -32768 {
			val = -32768
		} else if val > 32767 {
			val = 32767
		}
		LTP_coefs_Q14[LTP_coefs_Q14_ptr+i] = int16(val)
	}
}
