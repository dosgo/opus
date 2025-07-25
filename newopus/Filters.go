package opus

import "math"

func silk_warped_LPC_analysis_filter(
	state []int,
	res_Q2 []int,
	coef_Q13 []int16,
	coef_Q13_ptr int,
	input []int16,
	input_ptr int,
	lambda_Q16 int16,
	length int,
	order int) {

	for n := 0; n < length; n++ {
		tmp2 := silk_SMLAWB(state[0], state[1], int(lambda_Q16))
		state[0] = silk_LSHIFT(int(input[input_ptr+n]), 14)
		tmp1 := silk_SMLAWB(state[1], state[2]-tmp2, int(lambda_Q16))
		state[1] = tmp2
		acc_Q11 := int(order >> 1)
		acc_Q11 = silk_SMLAWB(acc_Q11, tmp2, int(coef_Q13[coef_Q13_ptr]))
		for i := 2; i < order; i += 2 {
			tmp2 = silk_SMLAWB(state[i], state[i+1]-tmp1, int(lambda_Q16))
			state[i] = tmp1
			acc_Q11 = silk_SMLAWB(acc_Q11, tmp1, int(coef_Q13[coef_Q13_ptr+i-1]))
			tmp1 = silk_SMLAWB(state[i+1], state[i+2]-tmp2, int(lambda_Q16))
			state[i+1] = tmp2
			acc_Q11 = silk_SMLAWB(acc_Q11, tmp2, int(coef_Q13[coef_Q13_ptr+i]))
		}
		state[order] = tmp1
		acc_Q11 = silk_SMLAWB(acc_Q11, tmp1, int(coef_Q13[coef_Q13_ptr+order-1]))
		res_Q2[n] = silk_LSHIFT(int(input[input_ptr+n]), 2) - silk_RSHIFT_ROUND(acc_Q11, 9)
	}
}

func silk_prefilter(
	psEnc *SilkChannelEncoder,
	psEncCtrl *SilkEncoderControl,
	xw_Q3 []int,
	x []int16,
	x_ptr int) {

	P := &psEnc.sPrefilt
	var lag int
	px := x_ptr
	pxw_Q3 := 0
	lag = P.lagPrev
	x_filt_Q12 := make([]int, psEnc.subfr_length)
	st_res_Q2 := make([]int, psEnc.subfr_length)
	for k := 0; k < psEnc.nb_subfr; k++ {
		if psEnc.indices.signalType == TYPE_VOICED {
			lag = psEncCtrl.pitchL[k]
		}
		HarmShapeGain_Q12 := silk_SMULWB(int(psEncCtrl.HarmShapeGain_Q14[k]), 16384-int(psEncCtrl.HarmBoost_Q14[k]))
		HarmShapeFIRPacked_Q12 := silk_RSHIFT(HarmShapeGain_Q12, 2)
		HarmShapeFIRPacked_Q12 |= silk_LSHIFT(int(silk_RSHIFT(HarmShapeGain_Q12, 1)), 16)
		Tilt_Q14 := psEncCtrl.Tilt_Q14[k]
		LF_shp_Q14 := psEncCtrl.LF_shp_Q14[k]
		AR1_shp_Q13 := k * MAX_SHAPE_LPC_ORDER
		silk_warped_LPC_analysis_filter(P.sAR_shp[:], st_res_Q2, psEncCtrl.AR1_Q13[:], AR1_shp_Q13, x, px, int16(psEnc.warping_Q16), psEnc.subfr_length, psEnc.shapingLPCOrder)
		B_Q10 := [2]int16{int16(silk_RSHIFT_ROUND(int(psEncCtrl.GainsPre_Q14[k]), 4)), 0}
		tmp_32 := silk_SMLABB(int(TuningParameters.INPUT_TILT), int(psEncCtrl.HarmBoost_Q14[k]), HarmShapeGain_Q12)
		//tmp_32 = silk_SMLABB(tmp_32, int(psEncCtrl.coding_quality_Q14), HIGH_RATE_INPUT_TILT_Q12)
		tmp_32 = silk_SMLABB(tmp_32, psEncCtrl.coding_quality_Q14, int((TuningParameters.HIGH_RATE_INPUT_TILT)*(1<<(12))+0.5))
		/* Q26 */

		tmp_32 = silk_SMULWB(tmp_32, -int(psEncCtrl.GainsPre_Q14[k]))
		tmp_32 = silk_RSHIFT_ROUND(tmp_32, 14)
		B_Q10[1] = int16(silk_SAT16(tmp_32))
		x_filt_Q12[0] = silk_MLA(silk_MUL(st_res_Q2[0], int(B_Q10[0])), int(P.sHarmHP_Q2), int(B_Q10[1]))
		for j := 1; j < psEnc.subfr_length; j++ {
			x_filt_Q12[j] = silk_MLA(silk_MUL(st_res_Q2[j], int(B_Q10[0])), st_res_Q2[j-1], int(B_Q10[1]))
		}
		P.sHarmHP_Q2 = int(st_res_Q2[psEnc.subfr_length-1])
		silk_prefilt(P, x_filt_Q12, xw_Q3, pxw_Q3, HarmShapeFIRPacked_Q12, Tilt_Q14, LF_shp_Q14, lag, psEnc.subfr_length)
		px += psEnc.subfr_length
		pxw_Q3 += psEnc.subfr_length
	}
	P.lagPrev = psEncCtrl.pitchL[psEnc.nb_subfr-1]
}

func silk_prefilt(
	P *SilkPrefilterState,
	st_res_Q12 []int,
	xw_Q3 []int,
	xw_Q3_ptr int,
	HarmShapeFIRPacked_Q12 int,
	Tilt_Q14 int,
	LF_shp_Q14 int,
	lag int,
	length int) {

	var n_LTP_Q12, n_Tilt_Q10, n_LF_Q10 int
	LTP_shp_buf := P.sLTP_shp[:]
	LTP_shp_buf_idx := P.sLTP_shp_buf_idx
	sLF_AR_shp_Q12 := P.sLF_AR_shp_Q12
	sLF_MA_shp_Q12 := P.sLF_MA_shp_Q12
	for i := 0; i < length; i++ {
		if lag > 0 {
			idx := lag + LTP_shp_buf_idx
			n_LTP_Q12 = silk_SMULBB(int(LTP_shp_buf[(idx-HARM_SHAPE_FIR_TAPS/2-1)&LTP_MASK]), HarmShapeFIRPacked_Q12)
			n_LTP_Q12 = silk_SMLABT(n_LTP_Q12, int(LTP_shp_buf[(idx-HARM_SHAPE_FIR_TAPS/2)&LTP_MASK]), HarmShapeFIRPacked_Q12)
			n_LTP_Q12 = silk_SMLABB(n_LTP_Q12, int(LTP_shp_buf[(idx-HARM_SHAPE_FIR_TAPS/2+1)&LTP_MASK]), HarmShapeFIRPacked_Q12)
		} else {
			n_LTP_Q12 = 0
		}
		n_Tilt_Q10 = silk_SMULWB(sLF_AR_shp_Q12, Tilt_Q14)
		n_LF_Q10 = silk_SMLAWB(silk_SMULWT(sLF_AR_shp_Q12, LF_shp_Q14), sLF_MA_shp_Q12, LF_shp_Q14)
		sLF_AR_shp_Q12 = st_res_Q12[i] - silk_LSHIFT(n_Tilt_Q10, 2)
		sLF_MA_shp_Q12 = sLF_AR_shp_Q12 - silk_LSHIFT(n_LF_Q10, 2)
		LTP_shp_buf_idx = (LTP_shp_buf_idx - 1) & LTP_MASK
		LTP_shp_buf[LTP_shp_buf_idx] = int16(silk_SAT16(silk_RSHIFT_ROUND(sLF_MA_shp_Q12, 12)))
		xw_Q3[xw_Q3_ptr+i] = silk_RSHIFT_ROUND(sLF_MA_shp_Q12-n_LTP_Q12, 9)
	}
	P.sLF_AR_shp_Q12 = sLF_AR_shp_Q12
	P.sLF_MA_shp_Q12 = sLF_MA_shp_Q12
	P.sLTP_shp_buf_idx = LTP_shp_buf_idx
}

func silk_biquad_alt(
	input []int16,
	input_ptr int,
	B_Q28 []int,
	A_Q28 []int,
	S []int,
	output []int16,
	output_ptr int,
	len int,
	stride int) {

	A0_L_Q28 := (-A_Q28[0]) & 0x00003FFF
	A0_U_Q28 := silk_RSHIFT(-A_Q28[0], 14)
	A1_L_Q28 := (-A_Q28[1]) & 0x00003FFF
	A1_U_Q28 := silk_RSHIFT(-A_Q28[1], 14)
	for k := 0; k < len; k++ {
		inval := int(input[input_ptr+k*stride])
		out32_Q14 := silk_LSHIFT(silk_SMLAWB(S[0], B_Q28[0], inval), 2)
		S[0] = S[1] + silk_RSHIFT_ROUND(silk_SMULWB(out32_Q14, A0_L_Q28), 14)
		S[0] = silk_SMLAWB(S[0], out32_Q14, A0_U_Q28)
		S[0] = silk_SMLAWB(S[0], B_Q28[1], inval)
		S[1] = silk_RSHIFT_ROUND(silk_SMULWB(out32_Q14, A1_L_Q28), 14)
		S[1] = silk_SMLAWB(S[1], out32_Q14, A1_U_Q28)
		S[1] = silk_SMLAWB(S[1], B_Q28[2], inval)
		output[output_ptr+k*stride] = int16(silk_SAT16(silk_RSHIFT(out32_Q14+(1<<14)-1, 14)))
	}
}

func silk_biquad_alt_ptr(
	input []int16,
	input_ptr int,
	B_Q28 []int,
	A_Q28 []int,
	S []int,
	S_ptr int,
	output []int16,
	output_ptr int,
	len int,
	stride int) {

	A0_L_Q28 := (-A_Q28[0]) & 0x00003FFF
	A0_U_Q28 := silk_RSHIFT(-A_Q28[0], 14)
	A1_L_Q28 := (-A_Q28[1]) & 0x00003FFF
	A1_U_Q28 := silk_RSHIFT(-A_Q28[1], 14)
	for k := 0; k < len; k++ {
		inval := int(input[input_ptr+k*stride])
		s0 := S[S_ptr]
		s1 := S[S_ptr+1]
		out32_Q14 := silk_LSHIFT(silk_SMLAWB(s0, B_Q28[0], inval), 2)
		s0 = s1 + silk_RSHIFT_ROUND(silk_SMULWB(out32_Q14, A0_L_Q28), 14)
		s0 = silk_SMLAWB(s0, out32_Q14, A0_U_Q28)
		s0 = silk_SMLAWB(s0, B_Q28[1], inval)
		s1 = silk_RSHIFT_ROUND(silk_SMULWB(out32_Q14, A1_L_Q28), 14)
		s1 = silk_SMLAWB(s1, out32_Q14, A1_U_Q28)
		s1 = silk_SMLAWB(s1, B_Q28[2], inval)
		S[S_ptr] = s0
		S[S_ptr+1] = s1
		output[output_ptr+k*stride] = int16(silk_SAT16(silk_RSHIFT(out32_Q14+(1<<14)-1, 14)))
	}
}

const (
	A_fb1_20 = 5394 << 1
	A_fb1_21 = -24290
)

func silk_ana_filt_bank_1(
	input []int16,
	input_ptr int,
	S []int,
	outL []int16,
	outH []int16,
	outH_ptr int,
	N int) {

	N2 := N >> 1
	for k := 0; k < N2; k++ {
		in32 := silk_LSHIFT(int(input[input_ptr+2*k]), 10)
		Y := in32 - S[0]
		X := silk_SMLAWB(Y, Y, A_fb1_21)
		out_1 := S[0] + X
		S[0] = in32 + X
		in32 = silk_LSHIFT(int(input[input_ptr+2*k+1]), 10)
		Y = in32 - S[1]
		X = silk_SMULWB(Y, A_fb1_20)
		out_2 := S[1] + X
		S[1] = in32 + X
		outL[k] = int16(silk_SAT16(silk_RSHIFT_ROUND(out_2+out_1, 11)))
		outH[outH_ptr+k] = int16(silk_SAT16(silk_RSHIFT_ROUND(out_2-out_1, 11)))
	}
}

func silk_LP_interpolate_filter_taps(
	B_Q28 []int,
	A_Q28 []int,
	ind int,
	fac_Q16 int) {

	if ind < TRANSITION_INT_NUM-1 {
		if fac_Q16 > 0 {
			if fac_Q16 < 32768 {
				for nb := 0; nb < TRANSITION_NB; nb++ {
					B_Q28[nb] = silk_SMLAWB(
						silk_Transition_LP_B_Q28[ind][nb],
						silk_Transition_LP_B_Q28[ind+1][nb]-silk_Transition_LP_B_Q28[ind][nb],
						fac_Q16)
				}
				for na := 0; na < TRANSITION_NA; na++ {
					A_Q28[na] = silk_SMLAWB(
						silk_Transition_LP_A_Q28[ind][na],
						silk_Transition_LP_A_Q28[ind+1][na]-silk_Transition_LP_A_Q28[ind][na],
						fac_Q16)
				}
			} else {
				fac_Q16_minus_one := fac_Q16 - (1 << 16)
				for nb := 0; nb < TRANSITION_NB; nb++ {
					B_Q28[nb] = silk_SMLAWB(
						silk_Transition_LP_B_Q28[ind+1][nb],
						silk_Transition_LP_B_Q28[ind+1][nb]-silk_Transition_LP_B_Q28[ind][nb],
						fac_Q16_minus_one)
				}
				for na := 0; na < TRANSITION_NA; na++ {
					A_Q28[na] = silk_SMLAWB(
						silk_Transition_LP_A_Q28[ind+1][na],
						silk_Transition_LP_A_Q28[ind+1][na]-silk_Transition_LP_A_Q28[ind][na],
						fac_Q16_minus_one)
				}
			}
		} else {
			copy(B_Q28, silk_Transition_LP_B_Q28[ind][:TRANSITION_NB])
			copy(A_Q28, silk_Transition_LP_A_Q28[ind][:TRANSITION_NA])
		}
	} else {
		copy(B_Q28, silk_Transition_LP_B_Q28[TRANSITION_INT_NUM-1][:TRANSITION_NB])
		copy(A_Q28, silk_Transition_LP_A_Q28[TRANSITION_INT_NUM-1][:TRANSITION_NA])
	}
}

func silk_LPC_analysis_filter(
	output []int16,
	output_ptr int,
	input []int16,
	input_ptr int,
	B []int16,
	B_ptr int,
	len int,
	d int) {

	mem := make([]int16, SILK_MAX_ORDER_LPC)
	num := make([]int16, SILK_MAX_ORDER_LPC)
	for j := 0; j < d; j++ {
		num[j] = -B[B_ptr+j]
	}
	for j := 0; j < d; j++ {
		mem[j] = input[input_ptr+d-j-1]
	}
	celt_fir(input[input_ptr+d:], num, output[output_ptr+d:], len-d, d, mem)
	for j := output_ptr; j < output_ptr+d; j++ {
		output[j] = 0
	}
}

const A_LIMIT = int(math.Round(0.99975 * float64(1<<QA)))

func LPC_inverse_pred_gain_QA(
	A_QA [2][]int,
	order int) int {

	Anew_QA := A_QA[order&1]
	invGain_Q30 := int(1 << 30)
	for k := order - 1; k > 0; k-- {
		if Anew_QA[k] > A_LIMIT || Anew_QA[k] < -A_LIMIT {
			return 0
		}
		rc_Q31 := -silk_LSHIFT(Anew_QA[k], 31-QA)
		rc_mult1_Q30 := (1 << 30) - silk_SMMUL(rc_Q31, rc_Q31)
		mult2Q := 32 - silk_CLZ32(silk_abs(rc_mult1_Q30))
		rc_mult2 := silk_INVERSE32_varQ(rc_mult1_Q30, mult2Q+30)
		invGain_Q30 = silk_LSHIFT(silk_SMMUL(invGain_Q30, rc_mult1_Q30), 2)
		Aold_QA := Anew_QA
		Anew_QA = A_QA[k&1]
		for n := 0; n < k; n++ {
			tmp_QA := Aold_QA[n] - MUL32_FRAC_Q(Aold_QA[k-n-1], rc_Q31, 31)
			Anew_QA[n] = MUL32_FRAC_Q(tmp_QA, rc_mult2, mult2Q)
		}
	}
	if Anew_QA[0] > A_LIMIT || Anew_QA[0] < -A_LIMIT {
		return 0
	}
	rc_Q31 := -silk_LSHIFT(Anew_QA[0], 31-QA)
	rc_mult1_Q30 := (1 << 30) - silk_SMMUL(rc_Q31, rc_Q31)
	invGain_Q30 = silk_LSHIFT(silk_SMMUL(invGain_Q30, rc_mult1_Q30), 2)
	return invGain_Q30
}

func silk_LPC_inverse_pred_gain(A_Q12 []int16, order int) int {
	Atmp_QA := [2][]int{
		make([]int, order),
		make([]int, order),
	}
	Anew_QA := Atmp_QA[order&1]
	DC_resp := 0
	for k := 0; k < order; k++ {
		DC_resp += int(A_Q12[k])
		Anew_QA[k] = silk_LSHIFT32(int(A_Q12[k]), QA-12)
	}
	if DC_resp >= 4096 {
		return 0
	}
	return LPC_inverse_pred_gain_QA(Atmp_QA, order)
}
