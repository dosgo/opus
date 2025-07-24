package opus

func silk_find_LPC(
	psEncC *SilkChannelEncoder,
	NLSF_Q15 []int16,
	x []int16,
	minInvGain_Q30 int,
) {
	var k, subfr_length int
	a_Q16 := make([]int, MAX_LPC_ORDER)
	var isInterpLower, shift int
	res_nrg0 := &BoxedValueInt{Val: 0}
	res_nrg1 := &BoxedValueInt{Val: 0}
	rshift0 := &BoxedValueInt{Val: 0}
	rshift1 := &BoxedValueInt{Val: 0}
	scratch_box1 := &BoxedValueInt{Val: 0}
	scratch_box2 := &BoxedValueInt{Val: 0}

	a_tmp_Q16 := make([]int, MAX_LPC_ORDER)
	var res_nrg_interp, res_nrg, res_tmp_nrg int
	var res_nrg_interp_Q, res_nrg_Q, res_tmp_nrg_Q int
	a_tmp_Q12 := make([]int16, MAX_LPC_ORDER)
	NLSF0_Q15 := make([]int16, MAX_LPC_ORDER)

	subfr_length = psEncC.subfr_length + psEncC.predictLPCOrder

	psEncC.indices.NLSFInterpCoef_Q2 = 4

	silk_burg_modified(scratch_box1, scratch_box2, a_Q16, x, 0, minInvGain_Q30, subfr_length, psEncC.nb_subfr, psEncC.predictLPCOrder)
	res_nrg = scratch_box1.Val
	res_nrg_Q = scratch_box2.Val

	if psEncC.useInterpolatedNLSFs != 0 && psEncC.first_frame_after_reset == 0 && psEncC.nb_subfr == MAX_NB_SUBFR {
		var LPC_res []int16

		silk_burg_modified(scratch_box1, scratch_box2, a_tmp_Q16, x, 2*subfr_length, minInvGain_Q30, subfr_length, 2, psEncC.predictLPCOrder)
		res_tmp_nrg = scratch_box1.Val
		res_tmp_nrg_Q = scratch_box2.Val

		shift = res_tmp_nrg_Q - res_nrg_Q
		if shift >= 0 {
			if shift < 32 {
				res_nrg = res_nrg - silk_RSHIFT(res_tmp_nrg, shift)
			}
		} else {
			OpusAssert(shift > -32)
			res_nrg = silk_RSHIFT(res_nrg, -shift) - res_tmp_nrg
			res_nrg_Q = res_tmp_nrg_Q
		}

		silk_A2NLSF(NLSF_Q15, a_tmp_Q16, psEncC.predictLPCOrder)

		LPC_res = make([]int16, 2*subfr_length)

		for k = 3; k >= 0; k-- {
			silk_interpolate(NLSF0_Q15, psEncC.prev_NLSFq_Q15, NLSF_Q15, k, psEncC.predictLPCOrder)

			silk_NLSF2A(a_tmp_Q12, NLSF0_Q15, psEncC.predictLPCOrder)

			silk_LPC_analysis_filter(LPC_res, 0, x, 0, a_tmp_Q12, 0, 2*subfr_length, psEncC.predictLPCOrder)

			silk_sum_sqr_shift(res_nrg0, rshift0, LPC_res, psEncC.predictLPCOrder, subfr_length-psEncC.predictLPCOrder)

			silk_sum_sqr_shift(res_nrg1, rshift1, LPC_res, psEncC.predictLPCOrder+subfr_length, subfr_length-psEncC.predictLPCOrder)

			shift = rshift0.Val - rshift1.Val
			if shift >= 0 {
				res_nrg1.Val = silk_RSHIFT(res_nrg1.Val, shift)
				res_nrg_interp_Q = -rshift0.Val
			} else {
				res_nrg0.Val = silk_RSHIFT(res_nrg0.Val, -shift)
				res_nrg_interp_Q = -rshift1.Val
			}
			res_nrg_interp = silk_ADD32(res_nrg0.Val, res_nrg1.Val)

			shift = res_nrg_interp_Q - res_nrg_Q
			if shift >= 0 {
				if silk_RSHIFT(res_nrg_interp, shift) < res_nrg {
					isInterpLower = 1
				} else {
					isInterpLower = 0
				}
			} else {
				if -shift < 32 {
					if res_nrg_interp < silk_RSHIFT(res_nrg, -shift) {
						isInterpLower = 1
					} else {
						isInterpLower = 0
					}
				} else {
					isInterpLower = 0
				}
			}

			if isInterpLower == 1 {
				res_nrg = res_nrg_interp
				res_nrg_Q = res_nrg_interp_Q
				psEncC.indices.NLSFInterpCoef_Q2 = byte(k)
			}
		}
	}

	if psEncC.indices.NLSFInterpCoef_Q2 == 4 {
		silk_A2NLSF(NLSF_Q15, a_Q16, psEncC.predictLPCOrder)
	}

	OpusAssert(psEncC.indices.NLSFInterpCoef_Q2 == 4 || (psEncC.useInterpolatedNLSFs != 0 && psEncC.first_frame_after_reset == 0 && psEncC.nb_subfr == MAX_NB_SUBFR))
}
