package opus
import "math"

const (
	MAX_STABILIZE_LOOPS  = 20
	QA                   = 16
	BIN_DIV_STEPS_A2NLSF = 3
	MAX_ITERATIONS_A2NLSF = 30
)

func silk_NLSF_VQ(err_Q26 []int32, in_Q15 []int16, pCB_Q8 []int16, K int, LPC_order int) {
	var diff_Q15, sum_error_Q30, sum_error_Q26 int32
	pCB_idx := 0

	Inlines_OpusAssert(err_Q26 != nil)
	Inlines_OpusAssert(LPC_order <= 16)
	Inlines_OpusAssert((LPC_order & 1) == 0)

	for i := 0; i < K; i++ {
		sum_error_Q26 = 0
		for m := 0; m < LPC_order; m += 2 {
			diff_Q15 = Inlines_silk_SUB_LSHIFT32(in_Q15[m], pCB_Q8[pCB_idx], 7)
			sum_error_Q30 = Inlines_silk_SMULBB(diff_Q15, diff_Q15)
			diff_Q15 = Inlines_silk_SUB_LSHIFT32(in_Q15[m+1], pCB_Q8[pCB_idx+1], 7)
			sum_error_Q30 = Inlines_silk_SMLABB(sum_error_Q30, diff_Q15, diff_Q15)
			sum_error_Q26 = Inlines_silk_ADD_RSHIFT32(sum_error_Q26, sum_error_Q30, 4)
			Inlines_OpusAssert(sum_error_Q26 >= 0)
			Inlines_OpusAssert(sum_error_Q30 >= 0)
			pCB_idx += 2
		}
		err_Q26[i] = sum_error_Q26
	}
}

func silk_NLSF_VQ_weights_laroia(pNLSFW_Q_OUT []int16, pNLSF_Q15 []int16, D int) {
	var tmp1_int, tmp2_int int32

	Inlines_OpusAssert(pNLSFW_Q_OUT != nil)
	Inlines_OpusAssert(D > 0)
	Inlines_OpusAssert((D & 1) == 0)

	tmp1_int = int32(Inlines_silk_max_int(int(pNLSF_Q15[0]), 1))
	tmp1_int = Inlines_silk_DIV32(1<<(15+SilkConstants_NLSF_W_Q), tmp1_int)
	tmp2_int = int32(Inlines_silk_max_int(int(pNLSF_Q15[1]-pNLSF_Q15[0]), 1))
	tmp2_int = Inlines_silk_DIV32(1<<(15+SilkConstants_NLSF_W_Q), tmp2_int)
	pNLSFW_Q_OUT[0] = int16(Inlines_silk_min_int(tmp1_int+tmp2_int, math.MaxInt16))
	Inlines_OpusAssert(pNLSFW_Q_OUT[0] > 0)

	for k := 1; k < D-1; k += 2 {
		tmp1_int = int32(Inlines_silk_max_int(int(pNLSF_Q15[k+1]-pNLSF_Q15[k]), 1))
		tmp1_int = Inlines_silk_DIV32(1<<(15+SilkConstants_NLSF_W_Q), tmp1_int)
		pNLSFW_Q_OUT[k] = int16(Inlines_silk_min_int(tmp1_int+tmp2_int, math.MaxInt16))
		Inlines_OpusAssert(pNLSFW_Q_OUT[k] > 0)

		tmp2_int = int32(Inlines_silk_max_int(int(pNLSF_Q15[k+2]-pNLSF_Q15[k+1]), 1))
		tmp2_int = Inlines_silk_DIV32(1<<(15+SilkConstants_NLSF_W_Q), tmp2_int)
		pNLSFW_Q_OUT[k+1] = int16(Inlines_silk_min_int(tmp1_int+tmp2_int, math.MaxInt16))
		Inlines_OpusAssert(pNLSFW_Q_OUT[k+1] > 0)
	}

	tmp1_int = int32(Inlines_silk_max_int(int((1<<15)-int(pNLSF_Q15[D-1]), 1))
	tmp1_int = Inlines_silk_DIV32(1<<(15+SilkConstants_NLSF_W_Q), tmp1_int)
	pNLSFW_Q_OUT[D-1] = int16(Inlines_silk_min_int(tmp1_int+tmp2_int, math.MaxInt16))
	Inlines_OpusAssert(pNLSFW_Q_OUT[D-1] > 0)
}

func silk_NLSF_residual_dequant(x_Q10 []int16, indices []byte, indices_ptr int, pred_coef_Q8 []int16, quant_step_size_Q16 int32, order int16) {
	var pred_Q10, out_Q10 int32

	out_Q10 = 0
	for i := int(order) - 1; i >= 0; i-- {
		pred_Q10 = Inlines_silk_RSHIFT(int32(Inlines_silk_SMULBB(out_Q10, pred_coef_Q8[i])), 8)
		out_Q10 = int32(indices[indices_ptr+i]) << 10
		if out_Q10 > 0 {
			out_Q10 -= int32(SilkConstants_NLSF_QUANT_LEVEL_ADJ * (1 << 10))
		} else if out_Q10 < 0 {
			out_Q10 += int32(SilkConstants_NLSF_QUANT_LEVEL_ADJ * (1 << 10))
		}
		out_Q10 = Inlines_silk_SMLAWB(pred_Q10, out_Q10, quant_step_size_Q16)
		x_Q10[i] = int16(out_Q10)
	}
}

func silk_NLSF_unpack(ec_ix []int16, pred_Q8 []int16, psNLSF_CB *NLSFCodebook, CB1_index int) {
	var entry int16
	ec_sel_ptr := CB1_index * psNLSF_CB.order / 2

	for i := 0; i < psNLSF_CB.order; i += 2 {
		entry = psNLSF_CB.ec_sel[ec_sel_ptr]
		ec_sel_ptr++
		ec_ix[i] = int16(Inlines_silk_SMULBB(int32((entry>>1)&7), int32(2*SilkConstants_NLSF_QUANT_MAX_AMPLITUDE+1)))
		pred_Q8[i] = psNLSF_CB.pred_Q8[i+int(entry&1)*(psNLSF_CB.order-1)]
		ec_ix[i+1] = int16(Inlines_silk_SMULBB(int32((entry>>5)&7), int32(2*SilkConstants_NLSF_QUANT_MAX_AMPLITUDE+1)))
		pred_Q8[i+1] = psNLSF_CB.pred_Q8[i+int((entry>>4)&1)*(psNLSF_CB.order-1)+1]
	}
}

func silk_NLSF_stabilize(NLSF_Q15 []int16, NDeltaMin_Q15 []int16, L int) {
	var I, k, loops int
	var center_freq_Q15 int16
	var diff_Q15, min_diff_Q15, min_center_Q15, max_center_Q15 int32

	Inlines_OpusAssert(int(NDeltaMin_Q15[L]) >= 1)

	for loops = 0; loops < MAX_STABILIZE_LOOPS; loops++ {
		min_diff_Q15 = int32(NLSF_Q15[0] - NDeltaMin_Q15[0])
		I = 0

		for i := 1; i <= L-1; i++ {
			diff_Q15 = int32(NLSF_Q15[i] - (NLSF_Q15[i-1] + NDeltaMin_Q15[i]))
			if diff_Q15 < min_diff_Q15 {
				min_diff_Q15 = diff_Q15
				I = i
			}
		}

		diff_Q15 = (1 << 15) - int32(NLSF_Q15[L-1]+int32(NDeltaMin_Q15[L])
		if diff_Q15 < min_diff_Q15 {
			min_diff_Q15 = diff_Q15
			I = L
		}

		if min_diff_Q15 >= 0 {
			return
		}

		if I == 0 {
			NLSF_Q15[0] = NDeltaMin_Q15[0]
		} else if I == L {
			NLSF_Q15[L-1] = int16((1 << 15) - int(NDeltaMin_Q15[L]))
		} else {
			min_center_Q15 = 0
			for k = 0; k < I; k++ {
				min_center_Q15 += int32(NDeltaMin_Q15[k])
			}
			min_center_Q15 += int32(Inlines_silk_RSHIFT(int32(NDeltaMin_Q15[I]), 1))

			max_center_Q15 = 1 << 15
			for k = L; k > I; k-- {
				max_center_Q15 -= int32(NDeltaMin_Q15[k])
			}
			max_center_Q15 -= int32(Inlines_silk_RSHIFT(int32(NDeltaMin_Q15[I]), 1))

			center_freq_Q15 = int16(Inlines_silk_LIMIT_32(
				Inlines_silk_RSHIFT_ROUND(int32(NLSF_Q15[I-1])+int32(NLSF_Q15[I]), 1),
				min_center_Q15, max_center_Q15))
			NLSF_Q15[I-1] = center_freq_Q15 - int16(Inlines_silk_RSHIFT(int32(NDeltaMin_Q15[I]), 1))
			NLSF_Q15[I] = NLSF_Q15[I-1] + NDeltaMin_Q15[I]
		}
	}

	if loops == MAX_STABILIZE_LOOPS {
		Sort_silk_insertion_sort_increasing_all_values_int16(NLSF_Q15, L)
		NLSF_Q15[0] = int16(Inlines_silk_max_int(int(NLSF_Q15[0]), int(NDeltaMin_Q15[0])))
		for i := 1; i < L; i++ {
			NLSF_Q15[i] = int16(Inlines_silk_max_int(int(NLSF_Q15[i]), int(NLSF_Q15[i-1])+int(NDeltaMin_Q15[i])))
		}
		NLSF_Q15[L-1] = int16(Inlines_silk_min_int(int(NLSF_Q15[L-1]), (1<<15)-int(NDeltaMin_Q15[L])))
		for i := L - 2; i >= 0; i-- {
			NLSF_Q15[i] = int16(Inlines_silk_min_int(int(NLSF_Q15[i]), int(NLSF_Q15[i+1])-int(NDeltaMin_Q15[i+1])))
		}
	}
}

func silk_NLSF_decode(pNLSF_Q15 []int16, NLSFIndices []byte, psNLSF_CB *NLSFCodebook) {
	pred_Q8 := make([]int16, psNLSF_CB.order)
	ec_ix := make([]int16, psNLSF_CB.order)
	res_Q10 := make([]int16, psNLSF_CB.order)
	W_tmp_QW := make([]int16, psNLSF_CB.order)
	var W_tmp_Q9, NLSF_Q15_tmp int32

	pCB_element := int(NLSFIndices[0]) * psNLSF_CB.order
	for i := 0; i < psNLSF_CB.order; i++ {
		pNLSF_Q15[i] = int16(psNLSF_CB.CB1_NLSF_Q8[pCB_element+i] << 7)
	}

	silk_NLSF_unpack(ec_ix, pred_Q8, psNLSF_CB, int(NLSFIndices[0]))
	silk_NLSF_residual_dequant(res_Q10, NLSFIndices, 1, pred_Q8, psNLSF_CB.quantStepSize_Q16, int16(psNLSF_CB.order))
	silk_NLSF_VQ_weights_laroia(W_tmp_QW, pNLSF_Q15, psNLSF_CB.order)

	for i := 0; i < psNLSF_CB.order; i++ {
		W_tmp_Q9 = Inlines_silk_SQRT_APPROX(int32(W_tmp_QW[i]) << (18 - SilkConstants_NLSF_W_Q))
		NLSF_Q15_tmp = int32(pNLSF_Q15[i]) + Inlines_silk_DIV32_16(int32(res_Q10[i])<<14, int16(W_tmp_Q9))
		pNLSF_Q15[i] = int16(Inlines_silk_LIMIT(NLSF_Q15_tmp, 0, 32767))
	}

	silk_NLSF_stabilize(pNLSF_Q15, psNLSF_CB.deltaMin_Q15, psNLSF_CB.order)
}

func silk_NLSF_del_dec_quant(indices []byte, x_Q10 []int16, w_Q5 []int16, pred_coef_Q8 []int16, ec_ix []int16, ec_rates_Q5 []int16, quant_step_size_Q16 int32, inv_quant_step_size_Q6 int16, mu_Q20 int32, order int16) int32 {
	const NLSF_QUANT_DEL_DEC_STATES = 128
	const MAX_LPC_ORDER = 16
	const NLSF_QUANT_MAX_AMPLITUDE_EXT = 10
	const NLSF_QUANT_MAX_AMPLITUDE = 16
	const NLSF_QUANT_DEL_DEC_STATES_LOG2 = 7

	var i, j, nStates, ind_tmp, ind_min_max, ind_max_min, in_Q10, res_Q10 int32
	var pred_Q10, diff_Q10, out0_Q10, out1_Q10, rate0_Q5, rate1_Q5 int32
	var RD_tmp_Q25, min_Q25, min_max_Q25, max_min_Q25, pred_coef_Q16 int32
	ind_sort := make([]int, NLSF_QUANT_DEL_DEC_STATES)
	ind := make([][]byte, NLSF_QUANT_DEL_DEC_STATES)
	for i := range ind {
		ind[i] = make([]byte, MAX_LPC_ORDER)
	}

	prev_out_Q10 := make([]int16, 2*NLSF_QUANT_DEL_DEC_STATES)
	RD_Q25 := make([]int32, 2*NLSF_QUANT_DEL_DEC_STATES)
	RD_min_Q25 := make([]int32, NLSF_QUANT_DEL_DEC_STATES)
	RD_max_Q25 := make([]int32, NLSF_QUANT_DEL_DEC_STATES)
	var rates_Q5 int32

	out0_Q10_table := make([]int32, 2*NLSF_QUANT_MAX_AMPLITUDE_EXT)
	out1_Q10_table := make([]int32, 2*NLSF_QUANT_MAX_AMPLITUDE_EXT)

	for i := -NLSF_QUANT_MAX_AMPLITUDE_EXT; i < NLSF_QUANT_MAX_AMPLITUDE_EXT; i++ {
		out0_Q10 = int32(i) << 10
		out1_Q10 = out0_Q10 + 1024

		if i > 0 {
			out0_Q10 -= int32(SilkConstants_NLSF_QUANT_LEVEL_ADJ * (1 << 10))
			out1_Q10 -= int32(SilkConstants_NLSF_QUANT_LEVEL_ADJ * (1 << 10))
		} else if i == 0 {
			out1_Q10 -= int32(SilkConstants_NLSF_QUANT_LEVEL_ADJ * (1 << 10))
		} else if i == -1 {
			out0_Q10 += int32(SilkConstants_NLSF_QUANT_LEVEL_ADJ * (1 << 10))
		} else {
			out0_Q10 += int32(SilkConstants_NLSF_QUANT_LEVEL_ADJ * (1 << 10))
			out1_Q10 += int32(SilkConstants_NLSF_QUANT_LEVEL_ADJ * (1 << 10))
		}

		out0_Q10_table[i+NLSF_QUANT_MAX_AMPLITUDE_EXT] = Inlines_silk_SMULWB(out0_Q10, quant_step_size_Q16)
		out1_Q10_table[i+NLSF_QUANT_MAX_AMPLITUDE_EXT] = Inlines_silk_SMULWB(out1_Q10, quant_step_size_Q16)
	}

	Inlines_OpusAssert((NLSF_QUANT_DEL_DEC_STATES & (NLSF_QUANT_DEL_DEC_STATES - 1)) == 0)

	nStates = 1
	RD_Q25[0] = 0
	prev_out_Q10[0] = 0

	ord := int(order)
	for i := ord - 1; ; i-- {
		pred_coef_Q16 = int32(pred_coef_Q8[i]) << 8
		in_Q10 = int32(x_Q10[i])

		for j := 0; j < nStates; j++ {
			pred_Q10 = Inlines_silk_SMULWB(pred_coef_Q16, int32(prev_out_Q10[j]))
			res_Q10 = in_Q10 - pred_Q10
			ind_tmp = Inlines_silk_SMULWB(int32(inv_quant_step_size_Q6), res_Q10)
			ind_tmp = Inlines_silk_LIMIT(ind_tmp, -NLSF_QUANT_MAX_AMPLITUDE_EXT, NLSF_QUANT_MAX_AMPLITUDE_EXT-1)
			ind[j][i] = byte(ind_tmp)
			rates_Q5 = int32(ec_ix[i]) + ind_tmp

			out0_Q10 = out0_Q10_table[ind_tmp+NLSF_QUANT_MAX_AMPLITUDE_EXT]
			out1_Q10 = out1_Q10_table[ind_tmp+NLSF_QUANT_MAX_AMPLITUDE_EXT]

			out0_Q10 += pred_Q10
			out1_Q10 += pred_Q10
			prev_out_Q10[j] = int16(out0_Q10)
			prev_out_Q10[j+nStates] = int16(out1_Q10)

			if ind_tmp+1 >= NLSF_QUANT_MAX_AMPLITUDE {
				if ind_tmp+1 == NLSF_QUANT_MAX_AMPLITUDE {
					rate0_Q5 = int32(ec_rates_Q5[rates_Q5+NLSF_QUANT_MAX_AMPLITUDE])
					rate1_Q5 = 280
				} else {
					rate0_Q5 = Inlines_silk_SMLABB(280-(43*NLSF_QUANT_MAX_AMPLITUDE), 43, ind_tmp)
					rate1_Q5 = rate0_Q5 + 43
				}
			} else if ind_tmp <= -NLSF_QUANT_MAX_AMPLITUDE {
				if ind_tmp == -NLSF_QUANT_MAX_AMPLITUDE {
					rate0_Q5 = 280
					rate1_Q5 = int32(ec_rates_Q5[rates_Q5+1+NLSF_QUANT_MAX_AMPLITUDE])
				} else {
					rate0_Q5 = Inlines_silk_SMLABB(280-43*NLSF_QUANT_MAX_AMPLITUDE, -43, ind_tmp)
					rate1_Q5 = rate0_Q5 - 43
				}
			} else {
				rate0_Q5 = int32(ec_rates_Q5[rates_Q5+NLSF_QUANT_MAX_AMPLITUDE])
				rate1_Q5 = int32(ec_rates_Q5[rates_Q5+1+NLSF_QUANT_MAX_AMPLITUDE])
			}

			RD_tmp_Q25 = RD_Q25[j]
			diff_Q10 = in_Q10 - out0_Q10
			RD_Q25[j] = Inlines_silk_SMLABB(Inlines_silk_MLA(RD_tmp_Q25, Inlines_silk_SMULBB(diff_Q10, diff_Q10), int32(w_Q5[i])), mu_Q20, rate0_Q5)
			diff_Q10 = in_Q10 - out1_Q10
			RD_Q25[j+nStates] = Inlines_silk_SMLABB(Inlines_silk_MLA(RD_tmp_Q25, Inlines_silk_SMULBB(diff_Q10, diff_Q10), int32(w_Q5[i])), mu_Q20, rate1_Q5)
		}

		if nStates <= (NLSF_QUANT_DEL_DEC_STATES >> 1) {
			for j := 0; j < nStates; j++ {
				ind[j+nStates][i] = ind[j][i] + 1
			}
			nStates <<= 1
			for j := nStates; j < NLSF_QUANT_DEL_DEC_STATES; j++ {
				copy(ind[j], ind[j-nStates])
			}
		} else if i > 0 {
			for j := 0; j < NLSF_QUANT_DEL_DEC_STATES; j++ {
				if RD_Q25[j] > RD_Q25[j+NLSF_QUANT_DEL_DEC_STATES] {
					RD_max_Q25[j] = RD_Q25[j]
					RD_min_Q25[j] = RD_Q25[j+NLSF_QUANT_DEL_DEC_STATES]
					RD_Q25[j] = RD_min_Q25[j]
					RD_Q25[j+NLSF_QUANT_DEL_DEC_STATES] = RD_max_Q25[j]
					out0_Q10 = int32(prev_out_Q10[j])
					prev_out_Q10[j] = prev_out_Q10[j+NLSF_QUANT_DEL_DEC_STATES]
					prev_out_Q10[j+NLSF_QUANT_DEL_DEC_STATES] = int16(out0_Q10)
					ind_sort[j] = j + NLSF_QUANT_DEL_DEC_STATES
				} else {
					RD_min_Q25[j] = RD_Q25[j]
					RD_max_Q25[j] = RD_Q25[j+NLSF_QUANT_DEL_DEC_STATES]
					ind_sort[j] = j
				}
			}

			for {
				min_max_Q25 = math.MaxInt32
				max_min_Q25 = 0
				ind_min_max = 0
				ind_max_min = 0

				for j := 0; j < NLSF_QUANT_DEL_DEC_STATES; j++ {
					if min_max_Q25 > RD_max_Q25[j] {
						min_max_Q25 = RD_max_Q25[j]
						ind_min_max = j
					}
					if max_min_Q25 < RD_min_Q25[j] {
						max_min_Q25 = RD_min_Q25[j]
						ind_max_min = j
					}
				}

				if min_max_Q25 >= max_min_Q25 {
					break
				}

				ind_sort[ind_max_min] = ind_sort[ind_min_max] ^ NLSF_QUANT_DEL_DEC_STATES
				RD_Q25[ind_max_min] = RD_Q25[ind_min_max+NLSF_QUANT_DEL_DEC_STATES]
				prev_out_Q10[ind_max_min] = prev_out_Q10[ind_min_max+NLSF_QUANT_DEL_DEC_STATES]
				RD_min_Q25[ind_max_min] = 0
				RD_max_Q25[ind_min_max] = math.MaxInt32
				copy(ind[ind_max_min], ind[ind_min_max])
			}

			for j := 0; j < NLSF_QUANT_DEL_DEC_STATES; j++ {
				x := ind_sort[j] >> NLSF_QUANT_DEL_DEC_STATES_LOG2
				ind[j][i] += byte(x)
			}
		} else {
			break
		}
	}

	ind_tmp = 0
	min_Q25 = math.MaxInt32
	for j := 0; j < 2*NLSF_QUANT_DEL_DEC_STATES; j++ {
		if min_Q25 > RD_Q25[j] {
			min_Q25 = RD_Q25[j]
			ind_tmp = j
		}
	}

	for j := 0; j < ord; j++ {
		indices[j] = ind[ind_tmp&(NLSF_QUANT_DEL_DEC_STATES-1)][j]
		Inlines_OpusAssert(int(indices[j]) >= -NLSF_QUANT_MAX_AMPLITUDE_EXT)
		Inlines_OpusAssert(int(indices[j]) <= NLSF_QUANT_MAX_AMPLITUDE_EXT)
	}

	indices[0] += byte(ind_tmp >> NLSF_QUANT_DEL_DEC_STATES_LOG2)
	Inlines_OpusAssert(int(indices[0]) <= NLSF_QUANT_MAX_AMPLITUDE_EXT)
	Inlines_OpusAssert(min_Q25 >= 0)
	return min_Q25
}

func silk_NLSF_encode(NLSFIndices []byte, pNLSF_Q15 []int16, psNLSF_CB *NLSFCodebook, pW_QW []int16, NLSF_mu_Q20 int32, nSurvivors int, signalType int) int32 {
	const NLSF_VQ_MAX_SURVIVORS = 16
	const MAX_LPC_ORDER = 16

	var i, s, ind1, prob_Q8, bits_q7 int
	var W_tmp_Q9 int32
	err_Q26 := make([]int32, psNLSF_CB.nVectors)
	RD_Q25 := make([]int32, nSurvivors)
	tempIndices1 := make([]int, nSurvivors)
	tempIndices2 := make([][]byte, nSurvivors)
	for i := range tempIndices2 {
		tempIndices2[i] = make([]byte, MAX_LPC_ORDER)
	}
	res_Q15 := make([]int16, psNLSF_CB.order)
	res_Q10 := make([]int16, psNLSF_CB.order)
	NLSF_tmp_Q15 := make([]int16, psNLSF_CB.order)
	W_tmp_QW := make([]int16, psNLSF_CB.order)
	W_adj_Q5 := make([]int16, psNLSF_CB.order)
	pred_Q8 := make([]int16, psNLSF_CB.order)
	ec_ix := make([]int16, psNLSF_CB.order)

	Inlines_OpusAssert(nSurvivors <= NLSF_VQ_MAX_SURVIVORS)
	Inlines_OpusAssert(signalType >= 0 && signalType <= 2)
	Inlines_OpusAssert(NLSF_mu_Q20 <= 32767 && NLSF_mu_Q20 >= 0)

	silk_NLSF_stabilize(pNLSF_Q15, psNLSF_CB.deltaMin_Q15, psNLSF_CB.order)
	silk_NLSF_VQ(err_Q26, pNLSF_Q15, psNLSF_CB.CB1_NLSF_Q8, psNLSF_CB.nVectors, psNLSF_CB.order)
	Sort_silk_insertion_sort_increasing(err_Q26, tempIndices1, psNLSF_CB.nVectors, nSurvivors)

	for s := 0; s < nSurvivors; s++ {
		ind1 = tempIndices1[s]
		pCB_element := ind1 * psNLSF_CB.order
		for i := 0; i < psNLSF_CB.order; i++ {
			NLSF_tmp_Q15[i] = int16(psNLSF_CB.CB1_NLSF_Q8[pCB_element+i] << 7)
			res_Q15[i] = pNLSF_Q15[i] - NLSF_tmp_Q15[i]
		}

		silk_NLSF_VQ_weights_laroia(W_tmp_QW, NLSF_tmp_Q15, psNLSF_CB.order)
		for i := 0; i < psNLSF_CB.order; i++ {
			W_tmp_Q9 = Inlines_silk_SQRT_APPROX(int32(W_tmp_QW[i]) << (18 - SilkConstants_NLSF_W_Q))
			res_Q10[i] = int16(Inlines_silk_RSHIFT(int32(Inlines_silk_SMULBB(int32(res_Q15[i]), int16(W_tmp_Q9)), 14))
		}

		for i := 0; i < psNLSF_CB.order; i++ {
			W_adj_Q5[i] = int16(Inlines_silk_DIV32_16(int32(pW_QW[i])<<5, int16(W_tmp_QW[i])))
		}

		silk_NLSF_unpack(ec_ix, pred_Q8, psNLSF_CB, ind1)
		RD_Q25[s] = silk_NLSF_del_dec_quant(
			tempIndices2[s], res_Q10, W_adj_Q5, pred_Q8, ec_ix, psNLSF_CB.ec_Rates_Q5,
			psNLSF_CB.quantStepSize_Q16, psNLSF_CB.invQuantStepSize_Q6, NLSF_mu_Q20, int16(psNLSF_CB.order))

		iCDF_ptr := (signalType >> 1) * psNLSF_CB.nVectors
		if ind1 == 0 {
			prob_Q8 = 256 - psNLSF_CB.CB1_iCDF[iCDF_ptr+ind1]
		} else {
			prob_Q8 = psNLSF_CB.CB1_iCDF[iCDF_ptr+ind1-1] - psNLSF_CB.CB1_iCDF[iCDF_ptr+ind1]
		}

		bits_q7 = (8 << 7) - Inlines_silk_lin2log(prob_Q8)
		RD_Q25[s] = Inlines_silk_SMLABB(RD_Q25[s], int32(bits_q7), NLSF_mu_Q20>>2)
	}

	bestIndex := make([]int, 1)
	Sort_silk_insertion_sort_increasing(RD_Q25, bestIndex, nSurvivors, 1)
	NLSFIndices[0] = byte(tempIndices1[bestIndex[0]])
	copy(NLSFIndices[1:], tempIndices2[bestIndex[0]])

	silk_NLSF_decode(pNLSF_Q15, NLSFIndices, psNLSF_CB)
	return RD_Q25[bestIndex[0]]
}

func silk_NLSF2A_find_poly(o []int32, cLSF []int32, cLSF_ptr int, dd int) {
	for k := 1; k < dd; k++ {
		ftmp := cLSF[cLSF_ptr+2*k]
		o[k+1] = (o[k-1] << 1) - int32(Inlines_silk_RSHIFT_ROUND64(Inlines_silk_SMULL(ftmp, o[k]), QA))
		for n := k; n > 1; n-- {
			o[n] += o[n-2] - int32(Inlines_silk_RSHIFT_ROUND64(Inlines_silk_SMULL(ftmp, o[n-1]), QA))
		}
		o[1] -= ftmp
	}
}

var ordering16 = []byte{0, 15, 8, 7, 4, 11, 12, 3, 2, 13, 10, 5, 6, 9, 14, 1}
var ordering10 = []byte{0, 9, 6, 3, 4, 5, 8, 1, 2, 7}

func silk_NLSF2A(a_Q12 []int16, NLSF []int16, d int) {
	const LSF_COS_TAB_SZ = 128
	const SILK_MAX_ORDER_LPC = 16
	const MAX_LPC_STABILIZE_ITERATIONS = 16
	const MAX_PREDICTION_POWER_GAIN = 10000

	var ordering []byte
	if d == 16 {
		ordering = ordering16
	} else {
		ordering = ordering10
	}

	Inlines_OpusAssert(LSF_COS_TAB_SZ == 128)
	Inlines_OpusAssert(d == 10 || d == 16)

	cos_LSF_QA := make([]int32, d)
	for k := 0; k < d; k++ {
		Inlines_OpusAssert(int(NLSF[k]) >= 0)
		f_int := int(NLSF[k]) >> (15 - 7)
		f_frac := int(NLSF[k]) - (f_int << (15 - 7))
		Inlines_OpusAssert(f_int >= 0)
		Inlines_OpusAssert(f_int < LSF_COS_TAB_SZ)
		cos_val := SilkTables_silk_LSFCosTab_Q12[f_int]
		delta := SilkTables_silk_LSFCosTab_Q12[f_int+1] - cos_val
		cos_LSF_QA[ordering[k]] = int32(Inlines_silk_RSHIFT_ROUND(int64(cos_val)<<8+int64(delta)*int64(f_frac), 20-QA))
	}

	dd := d / 2
	P := make([]int32, dd+1)
	Q := make([]int32, dd+1)
	a32_QA1 := make([]int32, d)

	P[0] = 1 << QA
	P[1] = -cos_LSF_QA[0]
	silk_NLSF2A_find_poly(P, cos_LSF_QA, 0, dd)

	Q[0] = 1 << QA
	Q[1] = -cos_LSF_QA[1]
	silk_NLSF2A_find_poly(Q, cos_LSF_QA, 1, dd)

	for k := 0; k < dd; k++ {
		Ptmp := P[k+1] + P[k]
		Qtmp := Q[k+1] - Q[k]
		a32_QA1[k] = -Qtmp - Ptmp
		a32_QA1[d-k-1] = Qtmp - Ptmp
	}

	for i := 0; i < 10; i++ {
		maxabs := int32(0)
		idx := 0
		for k := 0; k < d; k++ {
			absval := Inlines_silk_abs(a32_QA1[k])
			if absval > maxabs {
				maxabs = absval
				idx = k
			}
		}
		maxabs = int32(Inlines_silk_RSHIFT_ROUND(int64(maxabs), int64(QA+1-12)))

		if maxabs > math.MaxInt16 {
			maxabs = Inlines_silk_min_int32(maxabs, 163838)
			sc_Q16 := int32((0.999*65536.0)+0.5) - Inlines_silk_DIV32(int32(maxabs-math.MaxInt16)<<14, Inlines_silk_RSHIFT32(Inlines_silk_MUL(maxabs, int32(idx+1)), 2))
			Filters_silk_bwexpander_32(a32_QA1, d, sc_Q16)
		} else {
			break
		}
	}

	if i := 10; i == 10 {
		for k := 0; k < d; k++ {
			a_Q12[k] = int16(Inlines_silk_SAT16(int32(Inlines_silk_RSHIFT_ROUND(int64(a32_QA1[k]), int64(QA+1-12))))
			a32_QA1[k] = int32(a_Q12[k]) << (QA + 1 - 12)
		}
	} else {
		for k := 0; k < d; k++ {
			a_Q12[k] = int16(Inlines_silk_RSHIFT_ROUND(int64(a32_QA1[k]), int64(QA+1-12)))
		}
	}

	for i := 0; i < MAX_LPC_STABILIZE_ITERATIONS; i++ {
		if Filters_silk_LPC_inverse_pred_gain(a_Q12, d) < int32((1.0/MAX_PREDICTION_POWER_GAIN)*1073741824.0+0.5) {
			Filters_silk_bwexpander_32(a32_QA1, d, 65536-int32(2<<i))
			for k := 0; k < d; k++ {
				a_Q12[k] = int16(Inlines_silk_RSHIFT_ROUND(int64(a32_QA1[k]), int64(QA+1-12)))
			}
		} else {
			break
		}
	}
}

func silk_A2NLSF_trans_poly(p []int32, dd int) {
	for k := 2; k <= dd; k++ {
		for n := dd; n > k; n-- {
			p[n-2] -= p[n]
		}
		p[k-2] -= p[k] << 1
	}
}

func silk_A2NLSF_eval_poly(p []int32, x int32, dd int) int32 {
	x_Q16 := x << 4
	y32 := p[dd]
	if dd == 8 {
		y32 = Inlines_silk_SMLAWW(p[7], y32, x_Q16)
		y32 = Inlines_silk_SMLAWW(p[6], y32, x_Q16)
		y32 = Inlines_silk_SMLAWW(p[5], y32, x_Q16)
		y32 = Inlines_silk_SMLAWW(p[4], y32, x_Q16)
		y32 = Inlines_silk_SMLAWW(p[3], y32, x_Q16)
		y32 = Inlines_silk_SMLAWW(p[2], y32, x_Q16)
		y32 = Inlines_silk_SMLAWW(p[1], y32, x_Q16)
		y32 = Inlines_silk_SMLAWW(p[0], y32, x_Q16)
	} else {
		for n := dd - 1; n >= 0; n-- {
			y32 = Inlines_silk_SMLAWW(p[n], y32, x_Q16)
		}
	}
	return y32
}

func silk_A2NLSF_init(a_Q16 []int32, P []int32, Q []int32, dd int) {
	P[dd] = 1 << 16
	Q[dd] = 1 << 16
	for k := 0; k < dd; k++ {
		P[k] = -a_Q16[dd-k-1] - a_Q16[dd+k]
		Q[k] = -a_Q16[dd-k-1] + a_Q16[dd+k]
	}
	for k := dd; k > 0; k-- {
		P[k-1] -= P[k]
		Q[k-1] += Q[k]
	}
	silk_A2NLSF_trans_poly(P, dd)
	silk_A2NLSF_trans_poly(Q, dd)
}

func silk_A2NLSF(NLSF []int16, a_Q16 []int32, d int) {
	const SILK_MAX_ORDER_LPC = 16
	const LSF_COS_TAB_SZ = 128

	dd := d / 2
	P := make([]int32, SILK_MAX_ORDER_LPC/2+1)
	Q := make([]int32, SILK_MAX_ORDER_LPC/2+1)
	PQ := [2][]int32{P, Q}

	silk_A2NLSF_init(a_Q16, P, Q, dd)

	p := P
	xlo := SilkTables_silk_LSFCosTab_Q12[0]
	ylo := silk_A2NLSF_eval_poly(p, xlo, dd)

	root_ix := 0
	if ylo < 0 {
		NLSF[0] = 0
		p = Q
		ylo = silk_A2NLSF_eval_poly(p, xlo, dd)
		root_ix = 1
	}

	k := 1
	i := 0
	thr := 0
	for {
		xhi := SilkTables_silk_LSFCosTab_Q12[k]
		yhi := silk_A2NLSF_eval_poly(p, xhi, dd)

		if (ylo <= 0 && yhi >= int32(thr)) || (ylo >= 0 && yhi <= -int32(thr)) {
			if yhi == 0 {
				thr = 1
			} else {
				thr = 0
			}

			ffrac := -256
			for m := 0; m < BIN_DIV_STEPS_A2NLSF; m++ {
				xmid := (xlo + xhi) / 2
				ymid := silk_A2NLSF_eval_poly(p, xmid, dd)

				if (ylo <= 0 && ymid >= 0) || (ylo >= 0 && ymid <= 0) {
					xhi = xmid
					yhi = ymid
				} else {
					xlo = xmid
					ylo = ymid
					ffrac += 128 >> m
				}
			}

			if Inlines_silk_abs(int32(ylo)) < 65536 {
				den := ylo - yhi
				nom := (ylo << (8 - BIN_DIV_STEPS_A2NLSF)) + den/2
				if den != 0 {
					ffrac += int(nom / den)
				}
			} else {
				ffrac += int(ylo / ((ylo - yhi) >> (8 - BIN_DIV_STEPS_A2NLSF)))
			}

			NLSF[root_ix] = int16(Inlines_silk_min_32(int32(k)<<8+int32(ffrac), math.MaxInt16))
			Inlines_OpusAssert(int(NLSF[root_ix]) >= 0)

			root_ix++
			if root_ix >= d {
				break
			}

			p = PQ[root_ix&1]
			xlo = SilkTables_silk_LSFCosTab_Q12[k-1]
			ylo = int32(1-(root_ix&2)) << 12
		} else {
			k++
			xlo = xhi
			ylo = yhi
			thr = 0

			if k > LSF_COS_TAB_SZ {
				i++
				if i > MAX_ITERATIONS_A2NLSF {
					NLSF[0] = int16(((1 << 15) / (d + 1)))
					for k := 1; k < d; k++ {
						NLSF[k] = int16((k + 1) * int(NLSF[0]))
					}
					return
				}

				Filters_silk_bwexpander_32(a_Q16, d, 65536-int32(10+i)*int32(i))
				silk_A2NLSF_init(a_Q16, P, Q, dd)
				p = P
				xlo = SilkTables_silk_LSFCosTab_Q12[0]
				ylo = silk_A2NLSF_eval_poly(p, xlo, dd)
				if ylo < 0 {
					NLSF[0] = 0
					p = Q
					ylo = silk_A2NLSF_eval_poly(p, xlo, dd)
					root_ix = 1
				} else {
					root_ix = 0
				}
				k = 1
			}
		}
	}
}

func silk_process_NLSFs(psEncC *SilkChannelEncoder, PredCoef_Q12 [][]int16, pNLSF_Q15 []int16, prev_NLSFq_Q15 []int16) {
	const MAX_PREDICTION_POWER_GAIN = 10000
	const MAX_LPC_ORDER = 16
	const NLSF_MSVQ_SURVIVORS = 16

	var i int
	var doInterpolate bool
	var NLSF_mu_Q20, i_sqr_Q15 int32
	pNLSF0_temp_Q15 := make([]int16, MAX_LPC_ORDER)
	pNLSFW_QW := make([]int16, MAX_LPC_ORDER)
	pNLSFW0_temp_QW := make([]int16, MAX_LPC_ORDER)

	Inlines_OpusAssert(psEncC.speech_activity_Q8 >= 0)
	Inlines_OpusAssert(psEncC.speech_activity_Q8 <= 256)
	Inlines_OpusAssert(psEncC.useInterpolatedNLSFs == 1 || psEncC.indices.NLSFInterpCoef_Q2 == 4)

	NLSF_mu_Q20 = Inlines_silk_SMLAWB(31457, -26843, int32(psEncC.speech_activity_Q8))
	if psEncC.nb_subfr == 2 {
		NLSF_mu_Q20 += NLSF_mu_Q20 >> 1
	}

	Inlines_OpusAssert(NLSF_mu_Q20 > 0)
	Inlines_OpusAssert(NLSF_mu_Q20 <= 52428)

	silk_NLSF_VQ_weights_laroia(pNLSFW_QW, pNLSF_Q15, psEncC.predictLPCOrder)

	doInterpolate = (psEncC.useInterpolatedNLSFs == 1) && (psEncC.indices.NLSFInterpCoef_Q2 < 4)
	if doInterpolate {
		Inlines_silk_interpolate(pNLSF0_temp_Q15, prev_NLSFq_Q15, pNLSF_Q15, psEncC.indices.NLSFInterpCoef_Q2, psEncC.predictLPCOrder)
		silk_NLSF_VQ_weights_laroia(pNLSFW0_temp_QW, pNLSF0_temp_Q15, psEncC.predictLPCOrder)
		i_sqr_Q15 = int32(psEncC.indices.NLSFInterpCoef_Q2*psEncC.indices.NLSFInterpCoef_Q2) << 11
		for i := 0; i < psEncC.predictLPCOrder; i++ {
			pNLSFW_QW[i] = int16(Inlines_silk_SMLAWB(int32(pNLSFW_QW[i])>>1, int32(pNLSFW0_temp_QW[i]), i_sqr_Q15))
			Inlines_OpusAssert(pNLSFW_QW[i] > 0)
		}
	}

	silk_NLSF_encode(
		psEncC.indices.NLSFIndices, pNLSF_Q15, psEncC.psNLSF_CB, pNLSFW_QW,
		NLSF_mu_Q20, NLSF_MSVQ_SURVIVORS, psEncC.indices.signalType,
	)

	silk_NLSF2A(PredCoef_Q12[1], pNLSF_Q15, psEncC.predictLPCOrder)

	if doInterpolate {
		Inlines_silk_interpolate(pNLSF0_temp_Q15, prev_NLSFq_Q15, pNLSF_Q15, psEncC.indices.NLSFInterpCoef_Q2, psEncC.predictLPCOrder)
		silk_NLSF2A(PredCoef_Q12[0], pNLSF0_temp_Q15, psEncC.predictLPCOrder)
	} else {
		copy(PredCoef_Q12[0], PredCoef_Q12[1][:psEncC.predictLPCOrder])
	}
}