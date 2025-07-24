package opus

const (
	NB_ATT = 2
)

var (
	HARM_ATT_Q15              = [2]int16{32440, 31130}
	PLC_RAND_ATTENUATE_V_Q15  = [2]int16{31130, 26214}
	PLC_RAND_ATTENUATE_UV_Q15 = [2]int16{32440, 29491}
)

func silk_PLC_Reset(psDec *SilkChannelDecoder) {
	psDec.sPLC.pitchL_Q8 = int(int(psDec.frame_length) << (8 - 1))
	psDec.sPLC.prevGain_Q16[0] = int((1) * (1 << 16))
	psDec.sPLC.prevGain_Q16[1] = int((1) * (1 << 16))
	psDec.sPLC.subfr_length = 20
	psDec.sPLC.nb_subfr = 2
}

func silk_PLC(psDec *SilkChannelDecoder, psDecCtrl *SilkDecoderControl, frame []int16, frame_ptr int, lost int) {
	if psDec.fs_kHz != psDec.sPLC.fs_kHz {
		silk_PLC_Reset(psDec)
		psDec.sPLC.fs_kHz = psDec.fs_kHz
	}

	if lost != 0 {
		silk_PLC_conceal(psDec, psDecCtrl, frame, frame_ptr)
		psDec.lossCnt++
	} else {
		silk_PLC_update(psDec, psDecCtrl)
	}
}

func silk_PLC_update(psDec *SilkChannelDecoder, psDecCtrl *SilkDecoderControl) {
	var LTP_Gain_Q14, temp_LTP_Gain_Q14 int32
	psPLC := &psDec.sPLC

	psDec.prevSignalType = int(psDec.indices.signalType)
	LTP_Gain_Q14 = 0
	if psDec.indices.signalType == TYPE_VOICED {
		for j := 0; j*psDec.subfr_length < int(psDecCtrl.pitchL[psDec.nb_subfr-1]); j++ {
			if j == psDec.nb_subfr {
				break
			}
			temp_LTP_Gain_Q14 = 0
			for i := 0; i < LTP_ORDER; i++ {
				temp_LTP_Gain_Q14 += int32(psDecCtrl.LTPCoef_Q14[(psDec.nb_subfr-1-j)*LTP_ORDER+i])
			}
			if temp_LTP_Gain_Q14 > LTP_Gain_Q14 {
				LTP_Gain_Q14 = temp_LTP_Gain_Q14
				copy(psPLC.LTPCoef_Q14[:], psDecCtrl.LTPCoef_Q14[(psDec.nb_subfr-1-j)*LTP_ORDER:])
				psPLC.pitchL_Q8 = int32(psDecCtrl.pitchL[psDec.nb_subfr-1-j]) << 8
			}
		}

		for i := range psPLC.LTPCoef_Q14 {
			psPLC.LTPCoef_Q14[i] = 0
		}
		psPLC.LTPCoef_Q14[LTP_ORDER/2] = int16(LTP_Gain_Q14)

		if LTP_Gain_Q14 < V_PITCH_GAIN_START_MIN_Q14 {
			var scale_Q10 int32
			tmp := int32(V_PITCH_GAIN_START_MIN_Q14) << 10
			if LTP_Gain_Q14 > 0 {
				scale_Q10 = tmp / LTP_Gain_Q14
			} else {
				scale_Q10 = tmp
			}
			for i := 0; i < LTP_ORDER; i++ {
				psPLC.LTPCoef_Q14[i] = int16((int32(psPLC.LTPCoef_Q14[i]) * scale_Q10 >> 10))
			}
		} else if LTP_Gain_Q14 > V_PITCH_GAIN_START_MAX_Q14 {
			var scale_Q14 int32
			tmp := int32(V_PITCH_GAIN_START_MAX_Q14) << 14
			if LTP_Gain_Q14 > 0 {
				scale_Q14 = tmp / LTP_Gain_Q14
			} else {
				scale_Q14 = tmp
			}
			for i := 0; i < LTP_ORDER; i++ {
				psPLC.LTPCoef_Q14[i] = int16((int32(psPLC.LTPCoef_Q14[i]) * scale_Q14 >> 14))
			}
		}
	} else {
		psPLC.pitchL_Q8 = int32(psDec.fs_kHz*18) << 8
		for i := range psPLC.LTPCoef_Q14 {
			psPLC.LTPCoef_Q14[i] = 0
		}
	}

	copy(psPLC.prevLPC_Q12[:], psDecCtrl.PredCoef_Q12[1][:psDec.LPC_order])
	psPLC.prevLTP_scale_Q14 = int16(psDecCtrl.LTP_scale_Q14)

	copy(psPLC.prevGain_Q16[:], psDecCtrl.Gains_Q16[psDec.nb_subfr-2:])
	psPLC.subfr_length = psDec.subfr_length
	psPLC.nb_subfr = psDec.nb_subfr
}

func silk_PLC_energy(energy1, shift1, energy2, shift2 *BoxedValueInt, exc_Q14 []int32, prevGain_Q10 []int32, subfr_length, nb_subfr int) {
	exc_buf := make([]int16, 2*subfr_length)
	exc_buf_ptr := 0

	for k := 0; k < 2; k++ {
		for i := 0; i < subfr_length; i++ {
			idx := i + (k+nb_subfr-2)*subfr_length
			if idx < len(exc_Q14) {
				tmp := (exc_Q14[idx] * prevGain_Q10[k]) >> 8
				if tmp > 32767 {
					tmp = 32767
				} else if tmp < -32768 {
					tmp = -32768
				}
				exc_buf[exc_buf_ptr+i] = int16(tmp)
			}
		}
		exc_buf_ptr += subfr_length
	}

	silk_sum_sqr_shift(energy1, shift1, exc_buf[:subfr_length])
	silk_sum_sqr_shift(energy2, shift2, exc_buf[subfr_length:subfr_length*2])
}

func silk_PLC_conceal(psDec *SilkChannelDecoder, psDecCtrl *SilkDecoderControl, frame []int16, frame_ptr int) {
	var lag, idx, sLTP_buf_idx int
	var rand_seed, harm_Gain_Q15, rand_Gain_Q15, inv_gain_Q30 int32
	var rand_scale_Q14 int16
	var LPC_pred_Q10, LTP_pred_Q12 int32
	energy1 := &BoxedValueInt{0}
	energy2 := &BoxedValueInt{0}
	shift1 := &BoxedValueInt{0}
	shift2 := &BoxedValueInt{0}

	psPLC := &psDec.sPLC
	prevGain_Q10 := [2]int32{
		psPLC.prevGain_Q16[0] >> 6,
		psPLC.prevGain_Q16[1] >> 6,
	}

	if psDec.first_frame_after_reset != 0 {
		for i := range psPLC.prevLPC_Q12 {
			psPLC.prevLPC_Q12[i] = 0
		}
	}

	silk_PLC_energy(energy1, shift1, energy2, shift2, psDec.exc_Q14, prevGain_Q10[:], psDec.subfr_length, psDec.nb_subfr)

	energy1_shifted := energy1.Val >> shift2.Val
	energy2_shifted := energy2.Val >> shift1.Val
	if energy1_shifted < energy2_shifted {
		rand_ptr := max(0, (psPLC.nb_subfr-1)*psPLC.subfr_length-RAND_BUF_SIZE)
	} else {
		rand_ptr := max(0, psPLC.nb_subfr*psPLC.subfr_length-RAND_BUF_SIZE)
	}

	B_Q14 := psPLC.LTPCoef_Q14[:]
	rand_scale_Q14 = psPLC.randScale_Q14

	harm_idx := min(NB_ATT-1, int(psDec.lossCnt))
	harm_Gain_Q15 = int32(HARM_ATT_Q15[harm_idx])
	if psDec.prevSignalType == TYPE_VOICED {
		rand_Gain_Q15 = int32(PLC_RAND_ATTENUATE_V_Q15[harm_idx])
	} else {
		rand_Gain_Q15 = int32(PLC_RAND_ATTENUATE_UV_Q15[harm_idx])
	}

	silk_bwexpander(psPLC.prevLPC_Q12[:], psDec.LPC_order, BWE_COEF_Q16)

	if psDec.lossCnt == 0 {
		rand_scale_Q14 = 1 << 14
		if psDec.prevSignalType == TYPE_VOICED {
			sum := int32(0)
			for i := 0; i < LTP_ORDER; i++ {
				sum += int32(B_Q14[i])
			}
			rand_scale_Q14 -= int16(sum)
			if rand_scale_Q14 < 3277 {
				rand_scale_Q14 = 3277
			}
			rand_scale_Q14 = int16((int32(rand_scale_Q14) * int32(psPLC.prevLTP_scale_Q14) >> 14))
		} else {
			invGain_Q30 := silk_LPC_inverse_pred_gain(psPLC.prevLPC_Q12[:], psDec.LPC_order)
			down_scale_Q30 := min(int32(1<<30)>>LOG2_INV_LPC_GAIN_HIGH_THRES, invGain_Q30)
			down_scale_Q30 = max(int32(1<<30)>>LOG2_INV_LPC_GAIN_LOW_THRES, down_scale_Q30)
			down_scale_Q30 <<= LOG2_INV_LPC_GAIN_HIGH_THRES
			rand_Gain_Q15 = int32(int64(down_scale_Q30) * int64(rand_Gain_Q15) >> 14)
		}
	}

	rand_seed = psPLC.rand_seed
	lag = int((psPLC.pitchL_Q8 + 128) >> 8)
	sLTP_buf_idx = psDec.ltp_mem_length

	idx = psDec.ltp_mem_length - lag - psDec.LPC_order - LTP_ORDER/2
	silk_LPC_analysis_filter(psDec.outBuf[idx:], psDec.outBuf[idx:], psPLC.prevLPC_Q12[:], psDec.ltp_mem_length-idx, psDec.LPC_order)

	inv_gain_Q30 = silk_INVERSE32_varQ(psPLC.prevGain_Q16[1], 46)
	if inv_gain_Q30 > (1<<30)-1 {
		inv_gain_Q30 = (1 << 30) - 1
	}
	for i := idx + psDec.LPC_order; i < psDec.ltp_mem_length; i++ {
		sLTP_Q14[i] = silk_SMULWB(inv_gain_Q30, int32(psDec.outBuf[i]))
	}

	for k := 0; k < psDec.nb_subfr; k++ {
		pred_lag_ptr := sLTP_buf_idx - lag + LTP_ORDER/2
		for i := 0; i < psDec.subfr_length; i++ {
			LTP_pred_Q12 = 2
			LTP_pred_Q12 = silk_SMLAWB(LTP_pred_Q12, sLTP_Q14[pred_lag_ptr], int32(B_Q14[0]))
			LTP_pred_Q12 = silk_SMLAWB(LTP_pred_Q12, sLTP_Q14[pred_lag_ptr-1], int32(B_Q14[1]))
			LTP_pred_Q12 = silk_SMLAWB(LTP_pred_Q12, sLTP_Q14[pred_lag_ptr-2], int32(B_Q14[2]))
			LTP_pred_Q12 = silk_SMLAWB(LTP_pred_Q12, sLTP_Q14[pred_lag_ptr-3], int32(B_Q14[3]))
			LTP_pred_Q12 = silk_SMLAWB(LTP_pred_Q12, sLTP_Q14[pred_lag_ptr-4], int32(B_Q14[4]))
			pred_lag_ptr++

			rand_seed = silk_RAND(rand_seed)
			idx = int(rand_seed>>25) & RAND_BUF_MASK
			sLTP_Q14[sLTP_buf_idx] = (LTP_pred_Q12 + silk_SMULWB(int32(psDec.exc_Q14[rand_ptr+idx]), int32(rand_scale_Q14))<<2)
			sLTP_buf_idx++
		}

		for j := 0; j < LTP_ORDER; j++ {
			B_Q14[j] = int16((int32(B_Q14[j]) * harm_Gain_Q15 >> 15))
		}
		rand_scale_Q14 = int16(int32(rand_scale_Q14) * rand_Gain_Q15 >> 15)

		psPLC.pitchL_Q8 = int32(int64(psPLC.pitchL_Q8) + (int64(psPLC.pitchL_Q8) * int64(PITCH_DRIFT_FAC_Q16) >> 16))
		max_pitch := int32(MAX_PITCH_LAG_MS*psDec.fs_kHz) << 8
		if psPLC.pitchL_Q8 > max_pitch {
			psPLC.pitchL_Q8 = max_pitch
		}
		lag = int((psPLC.pitchL_Q8 + 128) >> 8)
	}

	sLPC_Q14_ptr := psDec.ltp_mem_length - MAX_LPC_ORDER
	copy(sLTP_Q14[sLPC_Q14_ptr:], psDec.sLPC_Q14_buf[:MAX_LPC_ORDER])

	for i := 0; i < psDec.frame_length; i++ {
		sLPCmaxi := sLPC_Q14_ptr + MAX_LPC_ORDER + i
		LPC_pred_Q10 = int32(psDec.LPC_order) >> 1
		LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-1], int32(psPLC.prevLPC_Q12[0]))
		LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-2], int32(psPLC.prevLPC_Q12[1]))
		LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-3], int32(psPLC.prevLPC_Q12[2]))
		LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-4], int32(psPLC.prevLPC_Q12[3]))
		LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-5], int32(psPLC.prevLPC_Q12[4]))
		LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-6], int32(psPLC.prevLPC_Q12[5]))
		LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-7], int32(psPLC.prevLPC_Q12[6]))
		LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-8], int32(psPLC.prevLPC_Q12[7]))
		LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-9], int32(psPLC.prevLPC_Q12[8]))
		LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-10], int32(psPLC.prevLPC_Q12[9]))
		for j := 10; j < psDec.LPC_order; j++ {
			LPC_pred_Q10 = silk_SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-j-1], int32(psPLC.prevLPC_Q12[j]))
		}

		sLTP_Q14[sLPCmaxi] += LPC_pred_Q10 << 4

		sample := silk_SMULWW(sLTP_Q14[sLPCmaxi], prevGain_Q10[1])
		sample = (sample + 128) >> 8
		if sample > 32767 {
			sample = 32767
		} else if sample < -32768 {
			sample = -32768
		}
		frame[frame_ptr+i] = int16(sample)
	}

	copy(psDec.sLPC_Q14_buf[:], sLTP_Q14[sLPC_Q14_ptr+psDec.frame_length:])

	psPLC.rand_seed = rand_seed
	psPLC.randScale_Q14 = rand_scale_Q14
	for i := 0; i < MAX_NB_SUBFR; i++ {
		psDecCtrl.pitchL[i] = int32(lag)
	}
}

func silk_PLC_glue_frames(psDec *SilkChannelDecoder, frame []int16, frame_ptr int, length int) {
	var energy_shift, energy BoxedValueInt
	psPLC := &psDec.sPLC

	if psDec.lossCnt != 0 {
		silk_sum_sqr_shift(&energy, &energy_shift, frame[frame_ptr:frame_ptr+length])
		psPLC.conc_energy = energy.Val
		psPLC.conc_energy_shift = energy_shift.Val
		psPLC.last_frame_lost = 1
	} else {
		if psPLC.last_frame_lost != 0 {
			silk_sum_sqr_shift(&energy, &energy_shift, frame[frame_ptr:frame_ptr+length])

			if energy_shift.Val > psPLC.conc_energy_shift {
				psPLC.conc_energy >>= uint(energy_shift.Val - psPLC.conc_energy_shift)
			} else if energy_shift.Val < psPLC.conc_energy_shift {
				energy.Val >>= uint(psPLC.conc_energy_shift - energy_shift.Val)
			}

			if energy.Val > psPLC.conc_energy {
				var frac_Q24, LZ int32
				var gain_Q16, slope_Q16 int32

				LZ = int32(silk_CLZ32(uint32(psPLC.conc_energy)))
				LZ--
				psPLC.conc_energy <<= LZ
				energy.Val >>= max(0, 24-int(LZ))

				frac_Q24 = psPLC.conc_energy / max(energy.Val, 1)

				gain_Q16 = int32(silk_SQRT_APPROX(uint32(frac_Q24))) << 4
				slope_Q16 = ((1 << 16) - gain_Q16) / int32(length)
				slope_Q16 <<= 2

				for i := frame_ptr; i < frame_ptr+length; i++ {
					frame[i] = int16((int32(frame[i]) * gain_Q16) >> 16)
					gain_Q16 += slope_Q16
					if gain_Q16 > (1 << 16) {
						break
					}
				}
			}
		}
		psPLC.last_frame_lost = 0
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
