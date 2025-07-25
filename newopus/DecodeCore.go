package opus

func silk_decode_core(
	psDec *SilkChannelDecoder,
	psDecCtrl *SilkDecoderControl,
	xq []int16,
	xq_ptr int,
	pulses []int16,
) {
	var i, k, lag, start_idx, sLTP_buf_idx, NLSF_interpolation_flag, signalType int
	var A_Q12 []int16
	B_Q14 := psDecCtrl.LTPCoef_Q14
	var B_Q14_ptr int
	pxq := xq_ptr
	var sLTP []int16
	var sLTP_Q15 []int
	var LTP_pred_Q13, LPC_pred_Q10, Gain_Q10, inv_gain_Q31, gain_adj_Q16, rand_seed, offset_Q10 int
	var pred_lag_ptr int
	pexc_Q14 := 0
	var pres_Q14 []int
	var pres_Q14_ptr int
	var res_Q14 []int
	var sLPC_Q14 []int

	OpusAssert(psDec.prev_gain_Q16 != 0)

	sLTP = make([]int16, psDec.ltp_mem_length)
	sLTP_Q15 = make([]int, psDec.ltp_mem_length+psDec.frame_length)
	res_Q14 = make([]int, psDec.subfr_length)
	sLPC_Q14 = make([]int, psDec.subfr_length+MAX_LPC_ORDER)

	offset_Q10 = int(SilkTables.Quantization_Offsets_Q10[psDec.indices.signalType>>1][psDec.indices.quantOffsetType])

	if psDec.indices.NLSFInterpCoef_Q2 < 1<<2 {
		NLSF_interpolation_flag = 1
	} else {
		NLSF_interpolation_flag = 0
	}

	rand_seed = int(psDec.indices.Seed)
	for i = 0; i < psDec.frame_length; i++ {
		rand_seed = silk_RAND(rand_seed)
		psDec.exc_Q14[i] = int(pulses[i]) << 14
		if psDec.outBuf[i] > 0 {
			psDec.outBuf[i] -= QUANT_LEVEL_ADJUST_Q10 << 4
		} else if psDec.outBuf[i] < 0 {
			psDec.outBuf[i] += QUANT_LEVEL_ADJUST_Q10 << 4
		}
		psDec.exc_Q14[i] += offset_Q10 << 4
		if rand_seed < 0 {
			psDec.exc_Q14[i] = -psDec.exc_Q14[i]
		}
		rand_seed = silk_ADD32_ovflw(rand_seed, int(pulses[i]))
	}

	copy(sLPC_Q14[:MAX_LPC_ORDER], psDec.sLPC_Q14_buf[:MAX_LPC_ORDER])

	pexc_Q14 = 0
	pxq = xq_ptr
	sLTP_buf_idx = psDec.ltp_mem_length
	for k = 0; k < psDec.nb_subfr; k++ {
		pres_Q14 = res_Q14
		pres_Q14_ptr = 0
		A_Q12 = psDecCtrl.PredCoef_Q12[k>>1]
		B_Q14_ptr = k * LTP_ORDER
		signalType = int(psDec.indices.signalType)

		Gain_Q10 = silk_RSHIFT(psDecCtrl.Gains_Q16[k], 6)
		inv_gain_Q31 = silk_INVERSE32_varQ(psDecCtrl.Gains_Q16[k], 47)

		if psDecCtrl.Gains_Q16[k] != psDec.prev_gain_Q16 {
			gain_adj_Q16 = silk_DIV32_varQ(psDec.prev_gain_Q16, psDecCtrl.Gains_Q16[k], 16)
			for i = 0; i < MAX_LPC_ORDER; i++ {
				sLPC_Q14[i] = silk_SMULWW(gain_adj_Q16, sLPC_Q14[i])
			}
		} else {
			gain_adj_Q16 = 1 << 16
		}

		OpusAssert(inv_gain_Q31 != 0)
		psDec.prev_gain_Q16 = psDecCtrl.Gains_Q16[k]

		if psDec.lossCnt != 0 && psDec.prevSignalType == TYPE_VOICED &&
			psDec.indices.signalType != TYPE_VOICED && k < MAX_NB_SUBFR/2 {

			for i := 0; i < LTP_ORDER; i++ {
				B_Q14[B_Q14_ptr+i] = 0
			}
			B_Q14[B_Q14_ptr+(LTP_ORDER/2)] = 4096

			signalType = TYPE_VOICED
			psDecCtrl.pitchL[k] = psDec.lagPrev
		}

		if signalType == TYPE_VOICED {
			lag = psDecCtrl.pitchL[k]

			if k == 0 || (k == 2 && NLSF_interpolation_flag != 0) {
				start_idx = psDec.ltp_mem_length - lag - psDec.LPC_order - LTP_ORDER/2
				OpusAssert(start_idx > 0)

				if k == 2 {
					copy(psDec.outBuf[psDec.ltp_mem_length:], xq[xq_ptr:xq_ptr+2*psDec.subfr_length])
				}

				silk_LPC_analysis_filter(sLTP, start_idx, psDec.outBuf, start_idx+k*psDec.subfr_length, A_Q12, 0, psDec.ltp_mem_length-start_idx, psDec.LPC_order)

				if k == 0 {
					inv_gain_Q31 = silk_LSHIFT(silk_SMULWB(inv_gain_Q31, psDecCtrl.LTP_scale_Q14), 2)
				}
				for i = 0; i < lag+LTP_ORDER/2; i++ {
					sLTP_Q15[sLTP_buf_idx-i-1] = silk_SMULWB(inv_gain_Q31, int(sLTP[psDec.ltp_mem_length-i-1]))
				}
			} else if gain_adj_Q16 != 1<<16 {
				for i = 0; i < lag+LTP_ORDER/2; i++ {
					sLTP_Q15[sLTP_buf_idx-i-1] = silk_SMULWW(gain_adj_Q16, sLTP_Q15[sLTP_buf_idx-i-1])
				}
			}
		}

		if signalType == TYPE_VOICED {
			pred_lag_ptr = sLTP_buf_idx - lag + LTP_ORDER/2
			for i = 0; i < psDec.subfr_length; i++ {
				LTP_pred_Q13 = 2
				LTP_pred_Q13 = silk_SMLAWB(LTP_pred_Q13, sLTP_Q15[pred_lag_ptr], int(B_Q14[B_Q14_ptr]))
				LTP_pred_Q13 = silk_SMLAWB(LTP_pred_Q13, sLTP_Q15[pred_lag_ptr-1], int(B_Q14[B_Q14_ptr+1]))
				LTP_pred_Q13 = silk_SMLAWB(LTP_pred_Q13, sLTP_Q15[pred_lag_ptr-2], int(B_Q14[B_Q14_ptr+2]))
				LTP_pred_Q13 = silk_SMLAWB(LTP_pred_Q13, sLTP_Q15[pred_lag_ptr-3], int(B_Q14[B_Q14_ptr+3]))
				LTP_pred_Q13 = silk_SMLAWB(LTP_pred_Q13, sLTP_Q15[pred_lag_ptr-4], int(B_Q14[B_Q14_ptr+4]))
				pred_lag_ptr++

				pres_Q14[pres_Q14_ptr+i] = silk_ADD_LSHIFT32(psDec.Exc_Q14[pexc_Q14+i], LTP_pred_Q13, 1)

				sLTP_Q15[sLTP_buf_idx] = silk_LSHIFT(pres_Q14[pres_Q14_ptr+i], 1)
				sLTP_buf_idx++
			}
		} else {
			pres_Q14 = psDec.exc_Q14
			pres_Q14_ptr = pexc_Q14
		}

		for i = 0; i < psDec.subfr_length; i++ {
			LPC_pred_Q10 = silk_RSHIFT(int(psDec.LPC_order), 1)
			LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLPC_Q14[MAX_LPC_ORDER+i-1], int(A_Q12[0]))
			LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLPC_Q14[MAX_LPC_ORDER+i-2], int(A_Q12[1]))
			LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLPC_Q14[MAX_LPC_ORDER+i-3], int(A_Q12[2]))
			LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLPC_Q14[MAX_LPC_ORDER+i-4], int(A_Q12[3]))
			LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLPC_Q14[MAX_LPC_ORDER+i-5], int(A_Q12[4]))
			LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLPC_Q14[MAX_LPC_ORDER+i-6], int(A_Q12[5]))
			LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLPC_Q14[MAX_LPC_ORDER+i-7], int(A_Q12[6]))
			LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLPC_Q14[MAX_LPC_ORDER+i-8], int(A_Q12[7]))
			LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLPC_Q14[MAX_LPC_ORDER+i-9], int(A_Q12[8]))
			LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLPC_Q14[MAX_LPC_ORDER+i-10], int(A_Q12[9]))
			if psDec.LPC_order == 16 {
				LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLPC_Q14[MAX_LPC_ORDER+i-11], int(A_Q12[10]))
				LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLPC_Q14[MAX_LPC_ORDER+i-12], int(A_Q12[11]))
				LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLPC_Q14[MAX_LPC_ORDER+i-13], int(A_Q12[12]))
				LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLPC_Q14[MAX_LPC_ORDER+i-14], int(A_Q12[13]))
				LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLPC_Q14[MAX_LPC_ORDER+i-15], int(A_Q12[14]))
				LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLPC_Q14[MAX_LPC_ORDER+i-16], int(A_Q12[15]))
			}

			sLPC_Q14[MAX_LPC_ORDER+i] = silk_ADD_LSHIFT32(pres_Q14[pres_Q14_ptr+i], LPC_pred_Q10, 4)

			xq[pxq+i] = int16(silk_SAT16(silk_RSHIFT_ROUND(silk_SMULWW(sLPC_Q14[MAX_LPC_ORDER+i], Gain_Q10), 8)))
		}

		copy(sLPC_Q14[:MAX_LPC_ORDER], sLPC_Q14[psDec.subfr_length:psDec.subfr_length+MAX_LPC_ORDER])
		pexc_Q14 += psDec.subfr_length
		pxq += psDec.subfr_length
	}

	copy(psDec.sLPC_Q14_buf[:MAX_LPC_ORDER], sLPC_Q14[:MAX_LPC_ORDER])
}
