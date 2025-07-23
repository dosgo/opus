package silk

import "math"

const (
	NB_ATT = 2
)

var (
	HARM_ATT_Q15              = [2]int16{32440, 31130} // 0.99, 0.95
	PLC_RAND_ATTENUATE_V_Q15  = [2]int16{31130, 26214} // 0.95, 0.8
	PLC_RAND_ATTENUATE_UV_Q15 = [2]int16{32440, 29491} // 0.99, 0.9
)

type PLC struct{}

func (p *PLC) Reset(psDec *SilkChannelDecoder) {
	psDec.sPLC.pitchL_Q8 = Lsh(int32(psDec.frame_length), 8-1)
	psDec.sPLC.prevGain_Q16[0] = 1 << 16
	psDec.sPLC.prevGain_Q16[1] = 1 << 16
	psDec.sPLC.subfr_length = 20
	psDec.sPLC.nb_subfr = 2
}

func (p *PLC) Conceal(psDec *SilkChannelDecoder, psDecCtrl *SilkDecoderControl, frame []int16, frame_ptr int, lost int) {
	if psDec.fs_kHz != psDec.sPLC.fs_kHz {
		p.Reset(psDec)
		psDec.sPLC.fs_kHz = psDec.fs_kHz
	}

	if lost != 0 {
		p.conceal(psDec, psDecCtrl, frame, frame_ptr)
		psDec.lossCnt++
	} else {
		p.update(psDec, psDecCtrl)
	}
}

func (p *PLC) update(psDec *SilkChannelDecoder, psDecCtrl *SilkDecoderControl) {
	var LTP_Gain_Q14, temp_LTP_Gain_Q14 int32
	psPLC := &psDec.sPLC

	psDec.prevSignalType = psDec.indices.signalType
	LTP_Gain_Q14 = 0

	if psDec.indices.signalType == TYPE_VOICED {
		for j := 0; j*psDec.subfr_length < int(psDecCtrl.pitchL[psDec.nb_subfr-1]); j++ {
			if j == int(psDec.nb_subfr) {
				break
			}

			temp_LTP_Gain_Q14 = 0
			for i := 0; i < LTP_ORDER; i++ {
				temp_LTP_Gain_Q14 += int32(psDecCtrl.LTPCoef_Q14[(psDec.nb_subfr-1-int32(j))*LTP_ORDER+i])
			}

			if temp_LTP_Gain_Q14 > LTP_Gain_Q14 {
				LTP_Gain_Q14 = temp_LTP_Gain_Q14
				copy(psPLC.LTPCoef_Q14[:], psDecCtrl.LTPCoef_Q14[(psDec.nb_subfr-1-int32(j))*LTP_ORDER:])
				psPLC.pitchL_Q8 = Lsh(psDecCtrl.pitchL[psDec.nb_subfr-1-int32(j)], 8)
			}
		}

		for i := range psPLC.LTPCoef_Q14 {
			psPLC.LTPCoef_Q14[i] = 0
		}
		psPLC.LTPCoef_Q14[LTP_ORDER/2] = int16(LTP_Gain_Q14)

		if LTP_Gain_Q14 < V_PITCH_GAIN_START_MIN_Q14 {
			scale_Q10 := Div32(Lsh(V_PITCH_GAIN_START_MIN_Q14, 10), max32(LTP_Gain_Q14, 1))
			for i := range psPLC.LTPCoef_Q14 {
				psPLC.LTPCoef_Q14[i] = int16(Rsh(SMulBB(int32(psPLC.LTPCoef_Q14[i]), scale_Q10), 10))
			}
		} else if LTP_Gain_Q14 > V_PITCH_GAIN_START_MAX_Q14 {
			scale_Q14 := Div32(Lsh(V_PITCH_GAIN_START_MAX_Q14, 14), max32(LTP_Gain_Q14, 1))
			for i := range psPLC.LTPCoef_Q14 {
				psPLC.LTPCoef_Q14[i] = int16(Rsh(SMulBB(int32(psPLC.LTPCoef_Q14[i]), scale_Q14), 14))
			}
		}
	} else {
		psPLC.pitchL_Q8 = Lsh(SMulBB(psDec.fs_kHz, 18), 8)
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

func (p *PLC) energy(energy1, shift1, energy2, shift2 *int32, exc_Q14 []int32, prevGain_Q10 []int32, subfr_length, nb_subfr int) {
	exc_buf := make([]int16, 2*subfr_length)
	exc_buf_ptr := 0

	for k := 0; k < 2; k++ {
		for i := 0; i < subfr_length; i++ {
			exc_buf[exc_buf_ptr+i] = int16(SAT16(Rsh(SMulWW(exc_Q14[i+(k+int(nb_subfr)-2)*subfr_length], prevGain_Q10[k]), 8)))
		}
		exc_buf_ptr += subfr_length
	}

	sumSqrShift(energy1, shift1, exc_buf, subfr_length)
	sumSqrShift(energy2, shift2, exc_buf[subfr_length:], subfr_length)
}

func (p *PLC) conceal(psDec *SilkChannelDecoder, psDecCtrl *SilkDecoderControl, frame []int16, frame_ptr int) {
	var lag, idx, sLTP_buf_idx int32
	var rand_seed, harm_Gain_Q15, rand_Gain_Q15, inv_gain_Q30 int32
	var energy1, energy2, shift1, shift2 int32
	var rand_ptr, pred_lag_ptr int32
	var LPC_pred_Q10, LTP_pred_Q12 int32
	var rand_scale_Q14 int16
	var sLPC_Q14_ptr int32

	psPLC := &psDec.sPLC
	prevGain_Q10 := [2]int32{
		Rsh(psPLC.prevGain_Q16[0], 6),
		Rsh(psPLC.prevGain_Q16[1], 6),
	}

	if psDec.first_frame_after_reset != 0 {
		for i := range psPLC.prevLPC_Q12 {
			psPLC.prevLPC_Q12[i] = 0
		}
	}

	p.energy(&energy1, &shift1, &energy2, &shift2, psDec.exc_Q14[:], prevGain_Q10[:], psDec.subfr_length, int(psDec.nb_subfr))

	if Rsh(energy1, shift2) < Rsh(energy2, shift1) {
		rand_ptr = max32(0, int32((psPLC.nb_subfr-1)*psPLC.subfr_length-RAND_BUF_SIZE))
	} else {
		rand_ptr = max32(0, int32(psPLC.nb_subfr*psPLC.subfr_length-RAND_BUF_SIZE))
	}

	B_Q14 := psPLC.LTPCoef_Q14[:]
	rand_scale_Q14 = psPLC.randScale_Q14

	harm_Gain_Q15 = int32(HARM_ATT_Q15[min32(NB_ATT-1, int(psDec.lossCnt))])
	if psDec.prevSignalType == TYPE_VOICED {
		rand_Gain_Q15 = int32(PLC_RAND_ATTENUATE_V_Q15[min32(NB_ATT-1, int(psDec.lossCnt))])
	} else {
		rand_Gain_Q15 = int32(PLC_RAND_ATTENUATE_UV_Q15[min32(NB_ATT-1, int(psDec.lossCnt))])
	}

	bwexpander(psPLC.prevLPC_Q12[:], psDec.LPC_order, BWE_COEF<<16)

	if psDec.lossCnt == 0 {
		rand_scale_Q14 = 1 << 14

		if psDec.prevSignalType == TYPE_VOICED {
			for i := 0; i < LTP_ORDER; i++ {
				rand_scale_Q14 -= B_Q14[i]
			}
			rand_scale_Q14 = max16(3277, rand_scale_Q14)
			rand_scale_Q14 = int16(Rsh(SMulBB(int32(rand_scale_Q14), int32(psPLC.prevLTP_scale_Q14)), 14))
		} else {
			invGain_Q30 := LPCInversePredGain(psPLC.prevLPC_Q12[:], psDec.LPC_order)
			down_scale_Q30 := min32(Rsh(1<<30, LOG2_INV_LPC_GAIN_HIGH_THRES), invGain_Q30)
			down_scale_Q30 = max32(Rsh(1<<30, LOG2_INV_LPC_GAIN_LOW_THRES), down_scale_Q30)
			down_scale_Q30 = Lsh(down_scale_Q30, LOG2_INV_LPC_GAIN_HIGH_THRES)
			rand_Gain_Q15 = Rsh(SMulWB(down_scale_Q30, rand_Gain_Q15), 14)
		}
	}

	rand_seed = psPLC.rand_seed
	lag = RshRound(psPLC.pitchL_Q8, 8)
	sLTP_buf_idx = int32(psDec.ltp_mem_length)

	idx = int32(psDec.ltp_mem_length) - lag - int32(psDec.LPC_order) - int32(LTP_ORDER/2)
	if idx <= 0 {
		panic("idx must be positive")
	}

	sLTP := make([]int16, idx)
	LPC_analysis_filter(sLTP, psDec.outBuf[:idx], psPLC.prevLPC_Q12[:], psDec.LPC_order)

	inv_gain_Q30 = INVERSE32_varQ(int32(psPLC.prevGain_Q16[1]), 46)
	inv_gain_Q30 = min32(inv_gain_Q30, math.MaxInt32>>1)

	sLTP_Q14 := make([]int32, psDec.ltp_mem_length+psDec.frame_length)
	for i := idx + int32(psDec.LPC_order); i < int32(psDec.ltp_mem_length); i++ {
		sLTP_Q14[i] = SMulWB(inv_gain_Q30, int32(sLTP[i]))
	}

	for k := 0; k < int(psDec.nb_subfr); k++ {
		pred_lag_ptr = sLTP_buf_idx - lag + int32(LTP_ORDER/2)
		for i := 0; i < psDec.subfr_length; i++ {
			LTP_pred_Q12 = 2
			LTP_pred_Q12 = SMLAWB(LTP_pred_Q12, sLTP_Q14[pred_lag_ptr], int32(B_Q14[0]))
			LTP_pred_Q12 = SMLAWB(LTP_pred_Q12, sLTP_Q14[pred_lag_ptr-1], int32(B_Q14[1]))
			LTP_pred_Q12 = SMLAWB(LTP_pred_Q12, sLTP_Q14[pred_lag_ptr-2], int32(B_Q14[2]))
			LTP_pred_Q12 = SMLAWB(LTP_pred_Q12, sLTP_Q14[pred_lag_ptr-3], int32(B_Q14[3]))
			LTP_pred_Q12 = SMLAWB(LTP_pred_Q12, sLTP_Q14[pred_lag_ptr-4], int32(B_Q14[4]))
			pred_lag_ptr++

			rand_seed = RAND(rand_seed)
			idx = Rsh(rand_seed, 25) & RAND_BUF_MASK
			sLTP_Q14[sLTP_buf_idx] = Lsh(SMLAWB(LTP_pred_Q12, int32(psDec.exc_Q14[rand_ptr+idx]), int32(rand_scale_Q14)), 2)
			sLTP_buf_idx++
		}

		for j := 0; j < LTP_ORDER; j++ {
			B_Q14[j] = int16(Rsh(SMulBB(harm_Gain_Q15, int32(B_Q14[j])), 15))
		}
		rand_scale_Q14 = int16(Rsh(SMulBB(int32(rand_scale_Q14), rand_Gain_Q15), 15))

		psPLC.pitchL_Q8 = SMLAWB(psPLC.pitchL_Q8, psPLC.pitchL_Q8, PITCH_DRIFT_FAC_Q16)
		psPLC.pitchL_Q8 = min32(psPLC.pitchL_Q8, Lsh(SMulBB(MAX_PITCH_LAG_MS, psDec.fs_kHz), 8))
		lag = RshRound(psPLC.pitchL_Q8, 8)
	}

	sLPC_Q14_ptr = int32(psDec.ltp_mem_length) - int32(MAX_LPC_ORDER)
	copy(sLTP_Q14[sLPC_Q14_ptr:], psDec.sLPC_Q14_buf[:MAX_LPC_ORDER])

	if psDec.LPC_order < 10 {
		panic("LPC_order must be >= 10")
	}

	for i := 0; i < psDec.frame_length; i++ {
		sLPCmaxi := sLPC_Q14_ptr + int32(MAX_LPC_ORDER) + int32(i)
		LPC_pred_Q10 = int32(psDec.LPC_order) >> 1
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-1], int32(psPLC.prevLPC_Q12[0]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-2], int32(psPLC.prevLPC_Q12[1]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-3], int32(psPLC.prevLPC_Q12[2]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-4], int32(psPLC.prevLPC_Q12[3]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-5], int32(psPLC.prevLPC_Q12[4]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-6], int32(psPLC.prevLPC_Q12[5]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-7], int32(psPLC.prevLPC_Q12[6]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-8], int32(psPLC.prevLPC_Q12[7]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-9], int32(psPLC.prevLPC_Q12[8]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-10], int32(psPLC.prevLPC_Q12[9]))

		for j := 10; j < psDec.LPC_order; j++ {
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLTP_Q14[sLPCmaxi-int32(j)-1], int32(psPLC.prevLPC_Q12[j]))
		}

		sLTP_Q14[sLPCmaxi] = AddLsh32(sLTP_Q14[sLPCmaxi], LPC_pred_Q10, 4)
		frame[frame_ptr+i] = int16(SAT16(SAT16(RshRound(SMulWW(sLTP_Q14[sLPCmaxi], prevGain_Q10[1]), 8))))
	}

	copy(psDec.sLPC_Q14_buf[:], sLTP_Q14[sLPC_Q14_ptr+int32(psDec.frame_length):])

	psPLC.rand_seed = rand_seed
	psPLC.randScale_Q14 = rand_scale_Q14
	for i := range psDecCtrl.pitchL {
		psDecCtrl.pitchL[i] = int32(lag)
	}
}

func (p *PLC) GlueFrames(psDec *SilkChannelDecoder, frame []int16, frame_ptr, length int) {
	psPLC := &psDec.sPLC

	if psDec.lossCnt != 0 {
		var conc_e, conc_shift int32
		sumSqrShift(&conc_e, &conc_shift, frame[frame_ptr:], length)
		psPLC.conc_energy = conc_e
		psPLC.conc_energy_shift = conc_shift
		psPLC.last_frame_lost = 1
	} else {

		if psPLC.last_frame_lost != 0 {
			var energy, energy_shift int32
			sumSqrShift(&energy, &energy_shift, frame[frame_ptr:], length)

			if energy_shift > psPLC.conc_energy_shift {
				psPLC.conc_energy = Rsh(psPLC.conc_energy, energy_shift-psPLC.conc_energy_shift)
			} else if energy_shift < psPLC.conc_energy_shift {
				energy = Rsh(energy, psPLC.conc_energy_shift-energy_shift)
			}

			if energy > psPLC.conc_energy {
				var frac_Q24, LZ int32
				var gain_Q16, slope_Q16 int32

				LZ = CLZ32(psPLC.conc_energy)
				LZ = LZ - 1
				psPLC.conc_energy = Lsh(psPLC.conc_energy, LZ)
				energy = Rsh(energy, max32(24-LZ, 0))

				frac_Q24 = Div32(psPLC.conc_energy, max32(energy, 1))
				gain_Q16 = Lsh(SQRT_APPROX(frac_Q24), 4)
				slope_Q16 = Div32_16((1<<16)-gain_Q16, int16(length))
				slope_Q16 = Lsh(slope_Q16, 2)

				for i := frame_ptr; i < frame_ptr+length; i++ {
					frame[i] = int16(SMULWB(gain_Q16, int32(frame[i])))
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
