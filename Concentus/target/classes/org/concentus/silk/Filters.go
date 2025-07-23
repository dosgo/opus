package silk

// Filters contains various signal processing functions for SILK codec
type Filters struct{}

// silk_warped_LPC_analysis_filter performs warped linear prediction analysis filtering
// state: I/O State [order + 1]
// res_Q2: O Residual signal [length]
// coef_Q13: I Coefficients [order]
// input: I Input signal [length]
// lambda_Q16: I Warping factor
// length: I Length of input signal
// order: I Filter order (must be even)
func (f *Filters) WarpedLPCAnalysisFilter(
	state []int32, // I/O State [order + 1]
	res_Q2 []int32, // O Residual signal [length]
	coef_Q13 []int16, // I Coefficients [order]
	input []int16, // I Input signal [length]
	lambda_Q16 int16, // I Warping factor
	length int, // I Length of input signal
	order int, // I Filter order (must be even)
) {
	// Order must be even
	if (order & 1) != 0 {
		panic("order must be even")
	}

	for n := 0; n < length; n++ {
		// Output of lowpass section
		tmp2 := SMLAWB(state[0], state[1], lambda_Q16)
		state[0] = int32(input[n]) << 14

		// Output of allpass section
		tmp1 := SMLAWB(state[1], state[2]-tmp2, lambda_Q16)
		state[1] = tmp2

		acc_Q11 := int32(order >> 1)
		acc_Q11 = SMLAWB(acc_Q11, tmp2, coef_Q13[0])

		// Loop over allpass sections
		for i := 2; i < order; i += 2 {
			// Output of allpass section
			tmp2 = SMLAWB(state[i], state[i+1]-tmp1, lambda_Q16)
			state[i] = tmp1
			acc_Q11 = SMLAWB(acc_Q11, tmp1, coef_Q13[i-1])

			// Output of allpass section
			tmp1 = SMLAWB(state[i+1], state[i+2]-tmp2, lambda_Q16)
			state[i+1] = tmp2
			acc_Q11 = SMLAWB(acc_Q11, tmp2, coef_Q13[i])
		}

		state[order] = tmp1
		acc_Q11 = SMLAWB(acc_Q11, tmp1, coef_Q13[order-1])
		res_Q2[n] = (int32(input[n]) << 2) - RSHIFT_ROUND(acc_Q11, 9)
	}
}

// Prefilter performs pre-filtering for encoder
func (f *Filters) Prefilter(
	psEnc *SilkChannelEncoder,
	psEncCtrl *SilkEncoderControl,
	xw_Q3 []int32, // O Weighted signal
	x []int16, // I Speech signal
) {
	P := psEnc.sPrefilt
	var lag int32
	var tmp_32 int32
	var AR1_shp_Q13 int32
	var HarmShapeGain_Q12, Tilt_Q14 int32
	var HarmShapeFIRPacked_Q12, LF_shp_Q14 int32

	// Set up temporary buffers
	x_filt_Q12 := make([]int32, psEnc.subfr_length)
	st_res_Q2 := make([]int32, psEnc.subfr_length)
	B_Q10 := [2]int16{}

	pxw_Q3 := 0
	lag = P.lagPrev

	for k := 0; k < psEnc.nb_subfr; k++ {
		// Update Variables that change per sub frame
		if psEnc.indices.signalType == TYPE_VOICED {
			lag = psEncCtrl.pitchL[k]
		}

		// Noise shape parameters
		HarmShapeGain_Q12 = SMULWB(int32(psEncCtrl.HarmShapeGain_Q14[k]), 16384-psEncCtrl.HarmBoost_Q14[k])
		if HarmShapeGain_Q12 < 0 {
			panic("HarmShapeGain_Q12 must be >= 0")
		}
		HarmShapeFIRPacked_Q12 = RSHIFT(HarmShapeGain_Q12, 2)
		HarmShapeFIRPacked_Q12 |= LSHIFT(RSHIFT(HarmShapeGain_Q12, 1), 16)
		Tilt_Q14 = psEncCtrl.Tilt_Q14[k]
		LF_shp_Q14 = psEncCtrl.LF_shp_Q14[k]
		AR1_shp_Q13 = int32(k) * MAX_SHAPE_LPC_ORDER

		// Short term FIR filtering
		f.WarpedLPCAnalysisFilter(
			P.sAR_shp,
			st_res_Q2,
			psEncCtrl.AR1_Q13[AR1_shp_Q13:],
			x[psEnc.subfr_length*k:],
			int16(psEnc.warping_Q16),
			psEnc.subfr_length,
			psEnc.shapingLPCOrder,
		)

		// Reduce (mainly) low frequencies during harmonic emphasis
		B_Q10[0] = int16(RSHIFT_ROUND(psEncCtrl.GainsPre_Q14[k], 4))
		tmp_32 = SMLABB(int32((INPUT_TILT)*(1<<26)+0.5), psEncCtrl.HarmBoost_Q14[k], HarmShapeGain_Q12)
		tmp_32 = SMLABB(tmp_32, psEncCtrl.coding_quality_Q14, int32((HIGH_RATE_INPUT_TILT)*(1<<12)+0.5))
		tmp_32 = SMULWB(tmp_32, -psEncCtrl.GainsPre_Q14[k])
		tmp_32 = RSHIFT_ROUND(tmp_32, 14)
		B_Q10[1] = int16(SAT16(tmp_32))

		x_filt_Q12[0] = MLA(MUL(st_res_Q2[0], int32(B_Q10[0])), int32(P.sHarmHP_Q2), int32(B_Q10[1]))
		for j := 1; j < psEnc.subfr_length; j++ {
			x_filt_Q12[j] = MLA(MUL(st_res_Q2[j], int32(B_Q10[0])), st_res_Q2[j-1], int32(B_Q10[1]))
		}
		P.sHarmHP_Q2 = st_res_Q2[psEnc.subfr_length-1]

		f.Prefilt(
			P,
			x_filt_Q12,
			xw_Q3[pxw_Q3:],
			HarmShapeFIRPacked_Q12,
			Tilt_Q14,
			LF_shp_Q14,
			lag,
			psEnc.subfr_length,
		)

		pxw_Q3 += psEnc.subfr_length
	}

	P.lagPrev = psEncCtrl.pitchL[psEnc.nb_subfr-1]
}

// Prefilt is the prefilter for finding quantizer input signal
func (f *Filters) Prefilt(
	P *SilkPrefilterState, // I/O state
	st_res_Q12 []int32, // I short term residual signal
	xw_Q3 []int32, // O prefiltered signal
	HarmShapeFIRPacked_Q12 int32, // I Harmonic shaping coefficients
	Tilt_Q14 int32, // I Tilt shaping coefficient
	LF_shp_Q14 int32, // I Low-frequency shaping coefficients
	lag int32, // I Lag for harmonic shaping
	length int, // I Length of signals
) {
	var n_LTP_Q12, n_Tilt_Q10, n_LF_Q10 int32
	var sLF_MA_shp_Q12, sLF_AR_shp_Q12 int32

	// To speed up use temp variables instead of using the struct
	LTP_shp_buf := P.sLTP_shp[:]
	LTP_shp_buf_idx := P.sLTP_shp_buf_idx
	sLF_AR_shp_Q12 = P.sLF_AR_shp_Q12
	sLF_MA_shp_Q12 = P.sLF_MA_shp_Q12

	for i := 0; i < length; i++ {
		if lag > 0 {
			// unrolled loop
			if HARM_SHAPE_FIR_TAPS != 3 {
				panic("HARM_SHAPE_FIR_TAPS must be 3")
			}
			idx := lag + LTP_shp_buf_idx
			n_LTP_Q12 = SMULBB(int32(LTP_shp_buf[(idx-HARM_SHAPE_FIR_TAPS/2-1)&LTP_MASK]), HarmShapeFIRPacked_Q12)
			n_LTP_Q12 = SMLABT(n_LTP_Q12, int32(LTP_shp_buf[(idx-HARM_SHAPE_FIR_TAPS/2)&LTP_MASK]), HarmShapeFIRPacked_Q12)
			n_LTP_Q12 = SMLABB(n_LTP_Q12, int32(LTP_shp_buf[(idx-HARM_SHAPE_FIR_TAPS/2+1)&LTP_MASK]), HarmShapeFIRPacked_Q12)
		} else {
			n_LTP_Q12 = 0
		}

		n_Tilt_Q10 = SMULWB(sLF_AR_shp_Q12, Tilt_Q14)
		n_LF_Q10 = SMLAWB(SMULWT(sLF_AR_shp_Q12, LF_shp_Q14), sLF_MA_shp_Q12, LF_shp_Q14)

		sLF_AR_shp_Q12 = SUB32(st_res_Q12[i], LSHIFT(n_Tilt_Q10, 2))
		sLF_MA_shp_Q12 = SUB32(sLF_AR_shp_Q12, LSHIFT(n_LF_Q10, 2))

		LTP_shp_buf_idx = (LTP_shp_buf_idx - 1) & LTP_MASK
		LTP_shp_buf[LTP_shp_buf_idx] = int16(SAT16(RSHIFT_ROUND(sLF_MA_shp_Q12, 12)))

		xw_Q3[i] = RSHIFT_ROUND(SUB32(sLF_MA_shp_Q12, n_LTP_Q12), 9)
	}

	// Copy temp variables back to state
	P.sLF_AR_shp_Q12 = sLF_AR_shp_Q12
	P.sLF_MA_shp_Q12 = sLF_MA_shp_Q12
	P.sLTP_shp_buf_idx = LTP_shp_buf_idx
}

// BiquadAlt implements a second order ARMA filter (alternative implementation)
func (f *Filters) BiquadAlt(
	input []int16,
	B_Q28 []int32,
	A_Q28 []int32,
	S []int32,
	output []int16,
	len int,
	stride int,
) {
	// DIRECT FORM II TRANSPOSED (uses 2 element state vector)
	var inval, A0_U_Q28, A0_L_Q28, A1_U_Q28, A1_L_Q28, out32_Q14 int32

	// Negate A_Q28 values and split in two parts
	A0_L_Q28 = (-A_Q28[0]) & 0x00003FFF // lower part
	A0_U_Q28 = RSHIFT(-A_Q28[0], 14)    // upper part
	A1_L_Q28 = (-A_Q28[1]) & 0x00003FFF // lower part
	A1_U_Q28 = RSHIFT(-A_Q28[1], 14)    // upper part

	for k := 0; k < len; k++ {
		// S[0], S[1]: Q12
		inval = int32(input[k*stride])
		out32_Q14 = LSHIFT(SMLAWB(S[0], B_Q28[0], inval), 2)

		S[0] = S[1] + RSHIFT_ROUND(SMULWB(out32_Q14, A0_L_Q28), 14)
		S[0] = SMLAWB(S[0], out32_Q14, A0_U_Q28)
		S[0] = SMLAWB(S[0], B_Q28[1], inval)

		S[1] = RSHIFT_ROUND(SMULWB(out32_Q14, A1_L_Q28), 14)
		S[1] = SMLAWB(S[1], out32_Q14, A1_U_Q28)
		S[1] = SMLAWB(S[1], B_Q28[2], inval)

		// Scale back to Q0 and saturate
		output[k*stride] = int16(SAT16(RSHIFT(out32_Q14+(1<<14)-1, 14)))
	}
}

// AnaFiltBank1 splits signal into two decimated bands using first-order allpass filters
func (f *Filters) AnaFiltBank1(
	input []int16,
	S []int32,
	outL []int16,
	outH []int16,
	N int,
) {
	N2 := N >> 1
	var in32, X, Y, out_1, out_2 int32

	// Internal variables and state are in Q10 format
	for k := 0; k < N2; k++ {
		// Convert to Q10
		in32 = int32(input[2*k]) << 10

		// All-pass section for even input sample
		Y = SUB32(in32, S[0])
		X = SMLAWB(Y, Y, A_fb1_21)
		out_1 = ADD32(S[0], X)
		S[0] = ADD32(in32, X)

		// Convert to Q10
		in32 = int32(input[2*k+1]) << 10

		// All-pass section for odd input sample, and add to output of previous section
		Y = SUB32(in32, S[1])
		X = SMULWB(Y, A_fb1_20)
		out_2 = ADD32(S[1], X)
		S[1] = ADD32(in32, X)

		// Add/subtract, convert back to int16 and store to output
		outL[k] = int16(SAT16(RSHIFT_ROUND(ADD32(out_2, out_1), 11)))
		outH[k] = int16(SAT16(RSHIFT_ROUND(SUB32(out_2, out_1), 11)))
	}
}

// Bwexpander32 performs chirp (bandwidth expand) LP AR filter
func (f *Filters) Bwexpander32(
	ar []int32,
	d int,
	chirp_Q16 int32,
) {
	chirp_minus_one_Q16 := chirp_Q16 - 65536

	for i := 0; i < d-1; i++ {
		ar[i] = SMULWW(chirp_Q16, ar[i])
		chirp_Q16 += RSHIFT_ROUND(MUL(chirp_Q16, chirp_minus_one_Q16), 16)
	}

	ar[d-1] = SMULWW(chirp_Q16, ar[d-1])
}

// LPInterpolateFilterTaps interpolates elliptic/Cauer filter taps

func LPInterpolateFilterTaps(B_Q28 []int32, A_Q28 []int32, ind int, fac_Q16 int32) {
	if ind < TRANSITION_INT_NUM-1 {
		if fac_Q16 > 0 {
			if fac_Q16 < 32768 {
				for nb := 0; nb < TRANSITION_NB; nb++ {
					B_Q28[nb] = SMLAWB(
						silk_Transition_LP_B_Q28[ind][nb],
						silk_Transition_LP_B_Q28[ind+1][nb]-silk_Transition_LP_B_Q28[ind][nb],
						fac_Q16,
					)
				}
				for na := 0; na < TRANSITION_NA; na++ {
					A_Q28[na] = SMLAWB(
						silk_Transition_LP_A_Q28[ind][na],
						silk_Transition_LP_A_Q28[ind+1][na]-silk_Transition_LP_A_Q28[ind][na],
						fac_Q16,
					)
				}
			} else {
				fac := fac_Q16 - (1 << 16)
				for nb := 0; nb < TRANSITION_NB; nb++ {
					B_Q28[nb] = SMLAWB(
						Transition_LP_B_Q28[ind+1][nb],
						Transition_LP_B_Q28[ind+1][nb]-Transition_LP_B_Q28[ind][nb],
						fac,
					)
				}
				for na := 0; na < TRANSITION_NA; na++ {
					A_Q28[na] = SMLAWB(
						Transition_LP_A_Q28[ind+1][na],
						Transition_LP_A_Q28[ind+1][na]-Transition_LP_A_Q28[ind][na],
						fac,
					)
				}
			}
		} else {
			copy(B_Q28, Transition_LP_B_Q28[ind][:])
			copy(A_Q28, Transition_LP_A_Q28[ind][:])
		}
	} else {
		copy(B_Q28, Transition_LP_B_Q28[TRANSITION_INT_NUM-1][:])
		copy(A_Q28, Transition_LP_A_Q28[TRANSITION_INT_NUM-1][:])
	}
}

func silk_LPC_analysis_filter(output []int16, output_ptr int, input []int16, input_ptr int, B []int16, B_ptr int, len int, d int) {
	mem := make([]int16, SILK_MAX_ORDER_LPC)
	num := make([]int16, SILK_MAX_ORDER_LPC)

	for j := 0; j < d; j++ {
		num[j] = -B[B_ptr+j]
	}
	for j := 0; j < d; j++ {
		mem[j] = input[input_ptr+d-j-1]
	}

	// 需要实现 celt_fir 函数
	celt_fir(input[input_ptr+d:], num, output[output_ptr+d:], len-d, d, mem)

	for j := output_ptr; j < output_ptr+d; j++ {
		output[j] = 0
	}
}

func LPC_inverse_pred_gain_QA(A_QA [][]int32, order int) int32 {
	Anew_QA := A_QA[order&1]
	invGain_Q30 := int32(1 << 30)

	for k := order - 1; k > 0; k-- {
		if Anew_QA[k] > A_LIMIT || Anew_QA[k] < -A_LIMIT {
			return 0
		}

		rc_Q31 := -LSHIFT(Anew_QA[k], 31-QA)
		rc_mult1_Q30 := (1 << 30) - SMMUL(rc_Q31, rc_Q31)
		mult2Q := 32 - CLZ32(abs32(rc_mult1_Q30))
		rc_mult2 := INVERSE32_varQ(rc_mult1_Q30, mult2Q+30)

		invGain_Q30 = LSHIFT(SMMUL(invGain_Q30, rc_mult1_Q30), 2)

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

	rc_Q31 := -LSHIFT(Anew_QA[0], 31-QA)
	rc_mult1_Q30 := (1 << 30) - SMMUL(rc_Q31, rc_Q31)
	invGain_Q30 = LSHIFT(SMMUL(invGain_Q30, rc_mult1_Q30), 2)

	return invGain_Q30
}

func silk_LPC_inverse_pred_gain(A_Q12 []int16, order int) int32 {
	Anew_QA := make([][]int32, 2)
	for i := range Anew_QA {
		Anew_QA[i] = make([]int32, order)
	}

	DC_resp := int32(0)
	for k := 0; k < order; k++ {
		DC_resp += int32(A_Q12[k])
		Anew_QA[0][k] = int32(A_Q12[k]) << (QA - 12)
	}

	if DC_resp >= 4096 {
		return 0
	}

	return LPC_inverse_pred_gain_QA([][]int32{Anew_QA[0], make([]int32, order)}, order)
}

func SMLAWB(a, b int32, c int16) int32 {
	return a + int32((int64(b)*int64(c))>>16)
}

func SMULWB(a, b int32) int32 {
	return int32((int64(a) * int64(b)) >> 16)
}

func RSHIFT(x, n int32) int32 {
	return x >> n
}

func LSHIFT(x, n int32) int32 {
	return x << n
}

func RSHIFT_ROUND(x, n int32) int32 {
	if n > 0 {
		return (x + (1 << (n - 1))) >> n
	}
	return x
}

func SAT16(x int32) int16 {
	if x > 32767 {
		return 32767
	}
	if x < -32768 {
		return -32768
	}
	return int16(x)
}

func MLA(a, b, c int32) int32 {
	return a + b*c
}

func MUL(a, b int32) int32 {
	return a * b
}

func SMLABB(a, b, c int32) int32 {
	return a + b*int32(c)
}

func SMLABT(a, b int32, c int16) int32 {
	return a + b*int32(c)
}

func SMULWT(a, b int32) int32 {
	return (a * (b >> 16)) << 1
}

func SMMUL(a, b int32) int32 {
	return int32((int64(a) * int64(b)) >> 32)
}

func CLZ32(x int32) int {
	if x == 0 {
		return 32
	}
	n := 0
	if x <= 0x0000FFFF {
		n += 16
		x <<= 16
	}
	if x <= 0x00FFFFFF {
		n += 8
		x <<= 8
	}
	if x <= 0x0FFFFFFF {
		n += 4
		x <<= 4
	}
	if x <= 0x3FFFFFFF {
		n += 2
		x <<= 2
	}
	if x <= 0x7FFFFFFF {
		n++
	}
	return n
}

func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

func INVERSE32_varQ(x int32, Q int) int32 {
	return int32((int64(1) << (Q + 31)) / int64(x+(1<<(Q-16))))
}

func MUL32_FRAC_Q(a, b int32, Q int) int32 {
	return int32((int64(a) * int64(b)) >> Q)
}
