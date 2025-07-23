package concentus

import "math"

type SilkNSQState struct {
	xq               [2 * MAX_FRAME_LENGTH]int16
	sLTP_shp_Q14     [2 * MAX_FRAME_LENGTH]int32
	sLPC_Q14         [MAX_SUB_FRAME_LENGTH + NSQ_LPC_BUF_LENGTH]int32
	sAR2_Q14         [MAX_SHAPE_LPC_ORDER]int32
	sLF_AR_shp_Q14   int32
	lagPrev          int32
	sLTP_buf_idx     int32
	sLTP_shp_buf_idx int32
	rand_seed        int32
	prev_gain_Q16    int32
	rewhite_flag     int32
}

func NewSilkNSQState() *SilkNSQState {
	return &SilkNSQState{}
}

func (s *SilkNSQState) Reset() {
	s.xq = [2 * MAX_FRAME_LENGTH]int16{}
	s.sLTP_shp_Q14 = [2 * MAX_FRAME_LENGTH]int32{}
	s.sLPC_Q14 = [MAX_SUB_FRAME_LENGTH + NSQ_LPC_BUF_LENGTH]int32{}
	s.sAR2_Q14 = [MAX_SHAPE_LPC_ORDER]int32{}
	s.sLF_AR_shp_Q14 = 0
	s.lagPrev = 0
	s.sLTP_buf_idx = 0
	s.sLTP_shp_buf_idx = 0
	s.rand_seed = 0
	s.prev_gain_Q16 = 0
	s.rewhite_flag = 0
}

func (s *SilkNSQState) Assign(other *SilkNSQState) {
	s.sLF_AR_shp_Q14 = other.sLF_AR_shp_Q14
	s.lagPrev = other.lagPrev
	s.sLTP_buf_idx = other.sLTP_buf_idx
	s.sLTP_shp_buf_idx = other.sLTP_shp_buf_idx
	s.rand_seed = other.rand_seed
	s.prev_gain_Q16 = other.prev_gain_Q16
	s.rewhite_flag = other.rewhite_flag
	copy(s.xq[:], other.xq[:])
	copy(s.sLTP_shp_Q14[:], other.sLTP_shp_Q14[:])
	copy(s.sLPC_Q14[:], other.sLPC_Q14[:])
	copy(s.sAR2_Q14[:], other.sAR2_Q14[:])
}

type nsq_del_dec_struct struct {
	sLPC_Q14  [MAX_SUB_FRAME_LENGTH + NSQ_LPC_BUF_LENGTH]int32
	RandState [DECISION_DELAY]int32
	Q_Q10     [DECISION_DELAY]int32
	Xq_Q14    [DECISION_DELAY]int32
	Pred_Q15  [DECISION_DELAY]int32
	Shape_Q14 [DECISION_DELAY]int32
	sAR2_Q14  []int32
	LF_AR_Q14 int32
	Seed      int32
	SeedInit  int32
	RD_Q10    int32
}

func newNSQDelDecStruct(shapingOrder int) *nsq_del_dec_struct {
	return &nsq_del_dec_struct{
		sAR2_Q14: make([]int32, shapingOrder),
	}
}

func (s *nsq_del_dec_struct) PartialCopyFrom(other *nsq_del_dec_struct, q14Offset int) {
	copy(s.sLPC_Q14[q14Offset:], other.sLPC_Q14[q14Offset:])
	copy(s.RandState[:], other.RandState[:])
	copy(s.Q_Q10[:], other.Q_Q10[:])
	copy(s.Xq_Q14[:], other.Xq_Q14[:])
	copy(s.Pred_Q15[:], other.Pred_Q15[:])
	copy(s.Shape_Q14[:], other.Shape_Q14[:])
	copy(s.sAR2_Q14, other.sAR2_Q14)
	s.LF_AR_Q14 = other.LF_AR_Q14
	s.Seed = other.Seed
	s.SeedInit = other.SeedInit
	s.RD_Q10 = other.RD_Q10
}

func (s *nsq_del_dec_struct) Assign(other *nsq_del_dec_struct) {
	s.PartialCopyFrom(other, 0)
}

type nsq_sample_struct struct {
	Q_Q10        int32
	RD_Q10       int32
	xq_Q14       int32
	LF_AR_Q14    int32
	sLTP_shp_Q14 int32
	LPC_exc_Q14  int32
}

func (s *nsq_sample_struct) Assign(other *nsq_sample_struct) {
	s.Q_Q10 = other.Q_Q10
	s.RD_Q10 = other.RD_Q10
	s.xq_Q14 = other.xq_Q14
	s.LF_AR_Q14 = other.LF_AR_Q14
	s.sLTP_shp_Q14 = other.sLTP_shp_Q14
	s.LPC_exc_Q14 = other.LPC_exc_Q14
}

func (s *SilkNSQState) silk_NSQ(
	psEncC *SilkChannelEncoder,
	psIndices *SideInfoIndices,
	x_Q3 []int32,
	pulses []byte,
	PredCoef_Q12 [][]int16,
	LTPCoef_Q14 []int16,
	AR2_Q13 []int16,
	HarmShapeGain_Q14 []int32,
	Tilt_Q14 []int32,
	LF_shp_Q14 []int32,
	Gains_Q16 []int32,
	pitchL []int32,
	Lambda_Q10 int32,
	LTP_scale_Q14 int32,
) {
	var k, lag, start_idx, LSF_interpolation_flag int32
	var A_Q12, B_Q14, AR_shp_Q13 int32
	var pxq int32
	var sLTP_Q15 []int32
	var sLTP []int16
	var HarmShapeFIRPacked_Q14 int32
	var offset_Q10 int32
	var x_sc_Q10 []int32
	pulses_ptr := 0
	x_Q3_ptr := 0

	s.rand_seed = int32(psIndices.Seed)

	lag = s.lagPrev

	offset_Q10 = silk_Quantization_Offsets_Q10[psIndices.signalType>>1][psIndices.quantOffsetType]

	if psIndices.NLSFInterpCoef_Q2 == 4 {
		LSF_interpolation_flag = 0
	} else {
		LSF_interpolation_flag = 1
	}

	sLTP_Q15 = make([]int32, psEncC.ltp_mem_length+psEncC.frame_length)
	sLTP = make([]int16, psEncC.ltp_mem_length+psEncC.frame_length)
	x_sc_Q10 = make([]int32, psEncC.subfr_length)

	s.sLTP_shp_buf_idx = psEncC.ltp_mem_length
	s.sLTP_buf_idx = psEncC.ltp_mem_length
	pxq = psEncC.ltp_mem_length

	for k = 0; k < psEncC.nb_subfr; k++ {
		A_Q12 = ((k >> 1) | (1 - LSF_interpolation_flag))
		B_Q14 = k * LTP_ORDER
		AR_shp_Q13 = k * MAX_SHAPE_LPC_ORDER

		HarmShapeFIRPacked_Q14 = RSHIFT32(HarmShapeGain_Q14[k], 2)
		HarmShapeFIRPacked_Q14 |= LSHIFT32(RSHIFT32(HarmShapeGain_Q14[k], 1), 16)

		s.rewhite_flag = 0
		if psIndices.signalType == TYPE_VOICED {
			lag = pitchL[k]

			if (k & (3 - LSHIFT32(LSF_interpolation_flag, 1))) == 0 {
				start_idx = psEncC.ltp_mem_length - lag - psEncC.predictLPCOrder - LTP_ORDER/2
				silk_LPC_analysis_filter(sLTP, start_idx, s.xq[:], start_idx+k*psEncC.subfr_length,
					PredCoef_Q12[A_Q12], 0, psEncC.ltp_mem_length-start_idx, psEncC.predictLPCOrder)

				s.rewhite_flag = 1
				s.sLTP_buf_idx = psEncC.ltp_mem_length
			}
		}

		s.silk_nsq_scale_states(psEncC, x_Q3, x_Q3_ptr, x_sc_Q10, sLTP, sLTP_Q15, k, LTP_scale_Q14, Gains_Q16, pitchL, psIndices.signalType)

		s.silk_noise_shape_quantizer(
			psIndices.signalType,
			x_sc_Q10,
			pulses,
			pulses_ptr,
			s.xq[:],
			pxq,
			sLTP_Q15,
			PredCoef_Q12[A_Q12],
			LTPCoef_Q14,
			B_Q14,
			AR2_Q13,
			AR_shp_Q13,
			lag,
			HarmShapeFIRPacked_Q14,
			Tilt_Q14[k],
			LF_shp_Q14[k],
			Gains_Q16[k],
			Lambda_Q10,
			offset_Q10,
			psEncC.subfr_length,
			psEncC.shapingLPCOrder,
			psEncC.predictLPCOrder)

		x_Q3_ptr += psEncC.subfr_length
		pulses_ptr += psEncC.subfr_length
		pxq += psEncC.subfr_length
	}

	s.lagPrev = pitchL[psEncC.nb_subfr-1]

	copy(s.xq[:psEncC.ltp_mem_length], s.xq[psEncC.frame_length:])
	copy(s.sLTP_shp_Q14[:psEncC.ltp_mem_length], s.sLTP_shp_Q14[psEncC.frame_length:])
}

func (s *SilkNSQState) silk_noise_shape_quantizer(
	signalType int32,
	x_sc_Q10 []int32,
	pulses []byte,
	pulses_ptr int,
	xq []int16,
	xq_ptr int,
	sLTP_Q15 []int32,
	a_Q12 []int16,
	b_Q14 []int16,
	b_Q14_ptr int,
	AR_shp_Q13 []int16,
	AR_shp_Q13_ptr int,
	lag int32,
	HarmShapeFIRPacked_Q14 int32,
	Tilt_Q14 int32,
	LF_shp_Q14 int32,
	Gain_Q16 int32,
	Lambda_Q10 int32,
	offset_Q10 int32,
	length int32,
	shapingLPCOrder int32,
	predictLPCOrder int32,
) {
	var i, j int32
	var LTP_pred_Q13, LPC_pred_Q10, n_AR_Q12, n_LTP_Q13 int32
	var n_LF_Q12, r_Q10, rr_Q10, q1_Q0, q1_Q10, q2_Q10, rd1_Q20, rd2_Q20 int32
	var exc_Q14, LPC_exc_Q14, xq_Q14, Gain_Q10 int32
	var tmp1, tmp2, sLF_AR_shp_Q14 int32
	var psLPC_Q14 int32
	var shp_lag_ptr, pred_lag_ptr int32

	shp_lag_ptr = s.sLTP_shp_buf_idx - lag + HARM_SHAPE_FIR_TAPS/2
	pred_lag_ptr = s.sLTP_buf_idx - lag + LTP_ORDER/2
	Gain_Q10 = RSHIFT32(Gain_Q16, 6)

	psLPC_Q14 = NSQ_LPC_BUF_LENGTH - 1

	for i = 0; i < length; i++ {
		s.rand_seed = silk_RAND(s.rand_seed)

		LPC_pred_Q10 = RSHIFT32(predictLPCOrder, 1)
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-0], a_Q12[0])
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-1], a_Q12[1])
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-2], a_Q12[2])
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-3], a_Q12[3])
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-4], a_Q12[4])
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-5], a_Q12[5])
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-6], a_Q12[6])
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-7], a_Q12[7])
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-8], a_Q12[8])
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-9], a_Q12[9])
		if predictLPCOrder == 16 {
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-10], a_Q12[10])
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-11], a_Q12[11])
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-12], a_Q12[12])
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-13], a_Q12[13])
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-14], a_Q12[14])
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-15], a_Q12[15])
		}

		if signalType == TYPE_VOICED {
			LTP_pred_Q13 = 2
			LTP_pred_Q13 = SMLAWB(LTP_pred_Q13, sLTP_Q15[pred_lag_ptr], b_Q14[b_Q14_ptr])
			LTP_pred_Q13 = SMLAWB(LTP_pred_Q13, sLTP_Q15[pred_lag_ptr-1], b_Q14[b_Q14_ptr+1])
			LTP_pred_Q13 = SMLAWB(LTP_pred_Q13, sLTP_Q15[pred_lag_ptr-2], b_Q14[b_Q14_ptr+2])
			LTP_pred_Q13 = SMLAWB(LTP_pred_Q13, sLTP_Q15[pred_lag_ptr-3], b_Q14[b_Q14_ptr+3])
			LTP_pred_Q13 = SMLAWB(LTP_pred_Q13, sLTP_Q15[pred_lag_ptr-4], b_Q14[b_Q14_ptr+4])
			pred_lag_ptr += 1
		} else {
			LTP_pred_Q13 = 0
		}

		tmp2 = s.sLPC_Q14[psLPC_Q14]
		tmp1 = s.sAR2_Q14[0]
		s.sAR2_Q14[0] = tmp2
		n_AR_Q12 = RSHIFT32(shapingLPCOrder, 1)
		n_AR_Q12 = SMLAWB(n_AR_Q12, tmp2, AR_shp_Q13[AR_shp_Q13_ptr])
		for j = 2; j < shapingLPCOrder; j += 2 {
			tmp2 = s.sAR2_Q14[j-1]
			s.sAR2_Q14[j-1] = tmp1
			n_AR_Q12 = SMLAWB(n_AR_Q12, tmp1, AR_shp_Q13[AR_shp_Q13_ptr+j-1])
			tmp1 = s.sAR2_Q14[j+0]
			s.sAR2_Q14[j+0] = tmp2
			n_AR_Q12 = SMLAWB(n_AR_Q12, tmp2, AR_shp_Q13[AR_shp_Q13_ptr+j])
		}
		s.sAR2_Q14[shapingLPCOrder-1] = tmp1
		n_AR_Q12 = SMLAWB(n_AR_Q12, tmp1, AR_shp_Q13[AR_shp_Q13_ptr+shapingLPCOrder-1])

		n_AR_Q12 = LSHIFT32(n_AR_Q12, 1)
		n_AR_Q12 = SMLAWB(n_AR_Q12, s.sLF_AR_shp_Q14, Tilt_Q14)

		n_LF_Q12 = SMULWB(s.sLTP_shp_Q14[s.sLTP_shp_buf_idx-1], LF_shp_Q14)
		n_LF_Q12 = SMLAWT(n_LF_Q12, s.sLF_AR_shp_Q14, LF_shp_Q14)

		tmp1 = SUB32(LSHIFT32(LPC_pred_Q10, 2), n_AR_Q12)
		tmp1 = SUB32(tmp1, n_LF_Q12)
		if lag > 0 {
			n_LTP_Q13 = SMULWB(ADD32(s.sLTP_shp_Q14[shp_lag_ptr], s.sLTP_shp_Q14[shp_lag_ptr-2]), HarmShapeFIRPacked_Q14)
			n_LTP_Q13 = SMLAWT(n_LTP_Q13, s.sLTP_shp_Q14[shp_lag_ptr-1], HarmShapeFIRPacked_Q14)
			n_LTP_Q13 = LSHIFT(n_LTP_Q13, 1)
			shp_lag_ptr += 1

			tmp2 = SUB32(LTP_pred_Q13, n_LTP_Q13)
			tmp1 = ADD_LSHIFT32(tmp2, tmp1, 1)
			tmp1 = RSHIFT_ROUND(tmp1, 3)
		} else {
			tmp1 = RSHIFT_ROUND(tmp1, 2)
		}

		r_Q10 = SUB32(x_sc_Q10[i], tmp1)
		if s.rand_seed < 0 {
			r_Q10 = -r_Q10
		}
		r_Q10 = LIMIT_32(r_Q10, -(31 << 10), 30<<10)

		q1_Q10 = SUB32(r_Q10, offset_Q10)
		q1_Q0 = RSHIFT32(q1_Q10, 10)
		if q1_Q0 > 0 {
			q1_Q10 = SUB32(LSHIFT32(q1_Q0, 10), QUANT_LEVEL_ADJUST_Q10)
			q1_Q10 = ADD32(q1_Q10, offset_Q10)
			q2_Q10 = ADD32(q1_Q10, 1024)
			rd1_Q20 = SMULBB(q1_Q10, Lambda_Q10)
			rd2_Q20 = SMULBB(q2_Q10, Lambda_Q10)
		} else if q1_Q0 == 0 {
			q1_Q10 = offset_Q10
			q2_Q10 = ADD32(q1_Q10, 1024-QUANT_LEVEL_ADJUST_Q10)
			rd1_Q20 = SMULBB(q1_Q10, Lambda_Q10)
			rd2_Q20 = SMULBB(q2_Q10, Lambda_Q10)
		} else if q1_Q0 == -1 {
			q2_Q10 = offset_Q10
			q1_Q10 = SUB32(q2_Q10, 1024-QUANT_LEVEL_ADJUST_Q10)
			rd1_Q20 = SMULBB(-q1_Q10, Lambda_Q10)
			rd2_Q20 = SMULBB(q2_Q10, Lambda_Q10)
		} else {
			q1_Q10 = ADD32(LSHIFT32(q1_Q0, 10), QUANT_LEVEL_ADJUST_Q10)
			q1_Q10 = ADD32(q1_Q10, offset_Q10)
			q2_Q10 = ADD32(q1_Q10, 1024)
			rd1_Q20 = SMULBB(-q1_Q10, Lambda_Q10)
			rd2_Q20 = SMULBB(-q2_Q10, Lambda_Q10)
		}
		rr_Q10 = SUB32(r_Q10, q1_Q10)
		rd1_Q20 = SMLABB(rd1_Q20, rr_Q10, rr_Q10)
		rr_Q10 = SUB32(r_Q10, q2_Q10)
		rd2_Q20 = SMLABB(rd2_Q20, rr_Q10, rr_Q10)

		if rd2_Q20 < rd1_Q20 {
			q1_Q10 = q2_Q10
		}

		pulses[pulses_ptr+i] = byte(RSHIFT_ROUND(q1_Q10, 10))

		exc_Q14 = LSHIFT32(q1_Q10, 4)
		if s.rand_seed < 0 {
			exc_Q14 = -exc_Q14
		}

		LPC_exc_Q14 = ADD_LSHIFT32(exc_Q14, LTP_pred_Q13, 1)
		xq_Q14 = ADD_LSHIFT32(LPC_exc_Q14, LPC_pred_Q10, 4)

		xq[xq_ptr+i] = int16(SAT16(RSHIFT_ROUND(SMULWW(xq_Q14, Gain_Q10), 8)))

		psLPC_Q14 += 1
		s.sLPC_Q14[psLPC_Q14] = xq_Q14
		sLF_AR_shp_Q14 = SUB_LSHIFT32(xq_Q14, n_AR_Q12, 2)
		s.sLF_AR_shp_Q14 = sLF_AR_shp_Q14

		s.sLTP_shp_Q14[s.sLTP_shp_buf_idx] = SUB_LSHIFT32(sLF_AR_shp_Q14, n_LF_Q12, 2)
		sLTP_Q15[s.sLTP_buf_idx] = LSHIFT32(LPC_exc_Q14, 1)
		s.sLTP_shp_buf_idx++
		s.sLTP_buf_idx++

		s.rand_seed = ADD32_ovflw(s.rand_seed, int32(pulses[pulses_ptr+i]))
	}
}

func (s *SilkNSQState) silk_nsq_scale_states(
	psEncC *SilkChannelEncoder,
	x_Q3 []int32,
	x_Q3_ptr int32,
	x_sc_Q10 []int32,
	sLTP []int16,
	sLTP_Q15 []int32,
	subfr int32,
	LTP_scale_Q14 int32,
	Gains_Q16 []int32,
	pitchL []int32,
	signal_type int32,
) {
	var i, lag int32
	var gain_adj_Q16, inv_gain_Q31, inv_gain_Q23 int32

	lag = pitchL[subfr]
	inv_gain_Q31 = INVERSE32_varQ(max_int32(Gains_Q16[subfr], 1), 47)

	if Gains_Q16[subfr] != s.prev_gain_Q16 {
		gain_adj_Q16 = DIV32_varQ(s.prev_gain_Q16, Gains_Q16[subfr], 16)
	} else {
		gain_adj_Q16 = 1 << 16
	}

	inv_gain_Q23 = RSHIFT_ROUND(inv_gain_Q31, 8)
	for i = 0; i < psEncC.subfr_length; i++ {
		x_sc_Q10[i] = SMULWW(x_Q3[x_Q3_ptr+i], inv_gain_Q23)
	}

	s.prev_gain_Q16 = Gains_Q16[subfr]

	if s.rewhite_flag != 0 {
		if subfr == 0 {
			inv_gain_Q31 = LSHIFT32(SMULWB(inv_gain_Q31, LTP_scale_Q14), 2)
		}
		for i = s.sLTP_buf_idx - lag - LTP_ORDER/2; i < s.sLTP_buf_idx; i++ {
			sLTP_Q15[i] = SMULWB(inv_gain_Q31, sLTP[i])
		}
	}

	if gain_adj_Q16 != (1 << 16) {
		for i = s.sLTP_shp_buf_idx - psEncC.ltp_mem_length; i < s.sLTP_shp_buf_idx; i++ {
			s.sLTP_shp_Q14[i] = SMULWW(gain_adj_Q16, s.sLTP_shp_Q14[i])
		}

		if signal_type == TYPE_VOICED && s.rewhite_flag == 0 {
			for i = s.sLTP_buf_idx - lag - LTP_ORDER/2; i < s.sLTP_buf_idx; i++ {
				sLTP_Q15[i] = SMULWW(gain_adj_Q16, sLTP_Q15[i])
			}
		}

		s.sLF_AR_shp_Q14 = SMULWW(gain_adj_Q16, s.sLF_AR_shp_Q14)

		for i = 0; i < NSQ_LPC_BUF_LENGTH; i++ {
			s.sLPC_Q14[i] = SMULWW(gain_adj_Q16, s.sLPC_Q14[i])
		}
		for i = 0; i < MAX_SHAPE_LPC_ORDER; i++ {
			s.sAR2_Q14[i] = SMULWW(gain_adj_Q16, s.sAR2_Q14[i])
		}
	}
}

func (s *SilkNSQState) silk_NSQ_del_dec(
	psEncC *SilkChannelEncoder,
	psIndices *SideInfoIndices,
	x_Q3 []int32,
	pulses []byte,
	PredCoef_Q12 [][]int16,
	LTPCoef_Q14 []int16,
	AR2_Q13 []int16,
	HarmShapeGain_Q14 []int32,
	Tilt_Q14 []int32,
	LF_shp_Q14 []int32,
	Gains_Q16 []int32,
	pitchL []int32,
	Lambda_Q10 int32,
	LTP_scale_Q14 int32,
) {
	var i, k, lag, start_idx, LSF_interpolation_flag, Winner_ind, subfr int32
	var last_smple_idx, smpl_buf_idx, decisionDelay int32
	var A_Q12 int32
	var pulses_ptr, pxq int32
	var sLTP_Q15 []int32
	var sLTP []int16
	var HarmShapeFIRPacked_Q14 int32
	var offset_Q10, RDmin_Q10, Gain_Q10 int32
	var x_sc_Q10, delayedGain_Q10 []int32
	var x_Q3_ptr int32
	var psDelDec []*nsq_del_dec_struct
	var psDD *nsq_del_dec_struct

	lag = s.lagPrev

	psDelDec = make([]*nsq_del_dec_struct, psEncC.nStatesDelayedDecision)
	for k = 0; k < psEncC.nStatesDelayedDecision; k++ {
		psDelDec[k] = newNSQDelDecStruct(psEncC.shapingLPCOrder)
		psDD = psDelDec[k]
		psDD.Seed = (k + int32(psIndices.Seed)) & 3
		psDD.SeedInit = psDD.Seed
		psDD.RD_Q10 = 0
		psDD.LF_AR_Q14 = s.sLF_AR_shp_Q14
		psDD.Shape_Q14[0] = s.sLTP_shp_Q14[psEncC.ltp_mem_length-1]
		copy(psDD.sLPC_Q14[:], s.sLPC_Q14[:])
		copy(psDD.sAR2_Q14, s.sAR2_Q14[:])
	}

	offset_Q10 = silk_Quantization_Offsets_Q10[psIndices.signalType>>1][psIndices.quantOffsetType]
	smpl_buf_idx = 0

	decisionDelay = min_int(DECISION_DELAY, psEncC.subfr_length)

	if psIndices.signalType == TYPE_VOICED {
		for k = 0; k < psEncC.nb_subfr; k++ {
			decisionDelay = min_int(decisionDelay, pitchL[k]-LTP_ORDER/2-1)
		}
	} else if lag > 0 {
		decisionDelay = min_int(decisionDelay, lag-LTP_ORDER/2-1)
	}

	if psIndices.NLSFInterpCoef_Q2 == 4 {
		LSF_interpolation_flag = 0
	} else {
		LSF_interpolation_flag = 1
	}

	sLTP_Q15 = make([]int32, psEncC.ltp_mem_length+psEncC.frame_length)
	sLTP = make([]int16, psEncC.ltp_mem_length+psEncC.frame_length)
	x_sc_Q10 = make([]int32, psEncC.subfr_length)
	delayedGain_Q10 = make([]int32, DECISION_DELAY)

	pxq = psEncC.ltp_mem_length
	s.sLTP_shp_buf_idx = psEncC.ltp_mem_length
	s.sLTP_buf_idx = psEncC.ltp_mem_length
	subfr = 0

	for k = 0; k < psEncC.nb_subfr; k++ {
		A_Q12 = ((k >> 1) | (1 - LSF_interpolation_flag))

		HarmShapeFIRPacked_Q14 = RSHIFT32(HarmShapeGain_Q14[k], 2)
		HarmShapeFIRPacked_Q14 |= LSHIFT32(RSHIFT32(HarmShapeGain_Q14[k], 1), 16)

		s.rewhite_flag = 0
		if psIndices.signalType == TYPE_VOICED {
			lag = pitchL[k]

			if (k & (3 - LSHIFT32(LSF_interpolation_flag, 1))) == 0 {
				if k == 2 {
					RDmin_Q10 = psDelDec[0].RD_Q10
					Winner_ind = 0
					for i = 1; i < psEncC.nStatesDelayedDecision; i++ {
						if psDelDec[i].RD_Q10 < RDmin_Q10 {
							RDmin_Q10 = psDelDec[i].RD_Q10
							Winner_ind = i
						}
					}
					for i = 0; i < psEncC.nStatesDelayedDecision; i++ {
						if i != Winner_ind {
							psDelDec[i].RD_Q10 += (math.MaxInt32 >> 4)
						}
					}

					psDD = psDelDec[Winner_ind]
					last_smple_idx = smpl_buf_idx + decisionDelay
					for i = 0; i < decisionDelay; i++ {
						last_smple_idx = (last_smple_idx - 1) & DECISION_DELAY_MASK
						pulses[pulses_ptr+i-decisionDelay] = byte(RSHIFT_ROUND(psDD.Q_Q10[last_smple_idx], 10))
						s.xq[pxq+i-decisionDelay] = int16(SAT16(RSHIFT_ROUND(
							SMULWW(psDD.Xq_Q14[last_smple_idx], Gains_Q16[1]), 14)))
						s.sLTP_shp_Q14[s.sLTP_shp_buf_idx-decisionDelay+i] = psDD.Shape_Q14[last_smple_idx]
					}

					subfr = 0
				}

				start_idx = psEncC.ltp_mem_length - lag - psEncC.predictLPCOrder - LTP_ORDER/2
				silk_LPC_analysis_filter(sLTP, start_idx, s.xq[:], start_idx+k*psEncC.subfr_length,
					PredCoef_Q12[A_Q12], 0, psEncC.ltp_mem_length-start_idx, psEncC.predictLPCOrder)

				s.sLTP_buf_idx = psEncC.ltp_mem_length
				s.rewhite_flag = 1
			}
		}

		s.silk_nsq_del_dec_scale_states(
			psEncC,
			psDelDec,
			x_Q3,
			x_Q3_ptr,
			x_sc_Q10,
			sLTP,
			sLTP_Q15,
			k,
			psEncC.nStatesDelayedDecision,
			LTP_scale_Q14,
			Gains_Q16,
			pitchL,
			psIndices.signalType,
			decisionDelay)

		smpl_buf_idx = s.silk_noise_shape_quantizer_del_dec(
			psDelDec,
			psIndices.signalType,
			x_sc_Q10,
			pulses,
			pulses_ptr,
			s.xq[:],
			pxq,
			sLTP_Q15,
			delayedGain_Q10,
			PredCoef_Q12[A_Q12],
			LTPCoef_Q14,
			k*LTP_ORDER,
			AR2_Q13,
			k*MAX_SHAPE_LPC_ORDER,
			lag,
			HarmShapeFIRPacked_Q14,
			Tilt_Q14[k],
			LF_shp_Q14[k],
			Gains_Q16[k],
			Lambda_Q10,
			offset_Q10,
			psEncC.subfr_length,
			subfr,
			psEncC.shapingLPCOrder,
			psEncC.predictLPCOrder,
			psEncC.warping_Q16,
			psEncC.nStatesDelayedDecision,
			smpl_buf_idx,
			decisionDelay)

		x_Q3_ptr += psEncC.subfr_length
		pulses_ptr += psEncC.subfr_length
		pxq += psEncC.subfr_length
		subfr++
	}

	RDmin_Q10 = psDelDec[0].RD_Q10
	Winner_ind = 0
	for k = 1; k < psEncC.nStatesDelayedDecision; k++ {
		if psDelDec[k].RD_Q10 < RDmin_Q10 {
			RDmin_Q10 = psDelDec[k].RD_Q10
			Winner_ind = k
		}
	}

	psDD = psDelDec[Winner_ind]
	psIndices.Seed = byte(psDD.SeedInit)
	last_smple_idx = smpl_buf_idx + decisionDelay
	Gain_Q10 = RSHIFT32(Gains_Q16[psEncC.nb_subfr-1], 6)
	for i = 0; i < decisionDelay; i++ {
		last_smple_idx = (last_smple_idx - 1) & DECISION_DELAY_MASK
		pulses[pulses_ptr+i-decisionDelay] = byte(RSHIFT_ROUND(psDD.Q_Q10[last_smple_idx], 10))
		s.xq[pxq+i-decisionDelay] = int16(SAT16(RSHIFT_ROUND(
			SMULWW(psDD.Xq_Q14[last_smple_idx], Gain_Q10), 8)))
		s.sLTP_shp_Q14[s.sLTP_shp_buf_idx-decisionDelay+i] = psDD.Shape_Q14[last_smple_idx]
	}
	copy(s.sLPC_Q14[:], psDD.sLPC_Q14[psEncC.subfr_length:])
	copy(s.sAR2_Q14[:], psDD.sAR2_Q14)

	s.sLF_AR_shp_Q14 = psDD.LF_AR_Q14
	s.lagPrev = pitchL[psEncC.nb_subfr-1]

	copy(s.xq[:psEncC.ltp_mem_length], s.xq[psEncC.frame_length:])
	copy(s.sLTP_shp_Q14[:psEncC.ltp_mem_length], s.sLTP_shp_Q14[psEncC.frame_length:])
}

func (s *SilkNSQState) silk_noise_shape_quantizer_del_dec(
	psDelDec []*nsq_del_dec_struct,
	signalType int32,
	x_Q10 []int32,
	pulses []byte,
	pulses_ptr int32,
	xq []int16,
	xq_ptr int32,
	sLTP_Q15 []int32,
	delayedGain_Q10 []int32,
	a_Q12 []int16,
	b_Q14 []int16,
	b_Q14_ptr int32,
	AR_shp_Q13 []int16,
	AR_shp_Q13_ptr int32,
	lag int32,
	HarmShapeFIRPacked_Q14 int32,
	Tilt_Q14 int32,
	LF_shp_Q14 int32,
	Gain_Q16 int32,
	Lambda_Q10 int32,
	offset_Q10 int32,
	length int32,
	subfr int32,
	shapingLPCOrder int32,
	predictLPCOrder int32,
	warping_Q16 int32,
	nStatesDelayedDecision int32,
	smpl_buf_idx int32,
	decisionDelay int32,
) int32 {
	var i, j, k, Winner_ind, RDmin_ind, RDmax_ind, last_smple_idx int32
	var Winner_rand_state int32
	var LTP_pred_Q14, LPC_pred_Q14, n_AR_Q14, n_LTP_Q14 int32
	var n_LF_Q14, r_Q10, rr_Q10, rd1_Q10, rd2_Q10, RDmin_Q10, RDmax_Q10 int32
	var q1_Q0, q1_Q10, q2_Q10, exc_Q14, LPC_exc_Q14, xq_Q14, Gain_Q10 int32
	var tmp1, tmp2, sLF_AR_shp_Q14 int32
	var pred_lag_ptr, shp_lag_ptr, psLPC_Q14 int32
	var sampleStates []*nsq_sample_struct
	var psDD *nsq_del_dec_struct
	var SS_left, SS_right int32

	sampleStates = make([]*nsq_sample_struct, 2*nStatesDelayedDecision)
	for i := range sampleStates {
		sampleStates[i] = &nsq_sample_struct{}
	}

	shp_lag_ptr = s.sLTP_shp_buf_idx - lag + HARM_SHAPE_FIR_TAPS/2
	pred_lag_ptr = s.sLTP_buf_idx - lag + LTP_ORDER/2
	Gain_Q10 = RSHIFT32(Gain_Q16, 6)

	for i = 0; i < length; i++ {
		if signalType == TYPE_VOICED {
			LTP_pred_Q14 = 2
			LTP_pred_Q14 = SMLAWB(LTP_pred_Q14, sLTP_Q15[pred_lag_ptr], b_Q14[b_Q14_ptr+0])
			LTP_pred_Q14 = SMLAWB(LTP_pred_Q14, sLTP_Q15[pred_lag_ptr-1], b_Q14[b_Q14_ptr+1])
			LTP_pred_Q14 = SMLAWB(LTP_pred_Q14, sLTP_Q15[pred_lag_ptr-2], b_Q14[b_Q14_ptr+2])
			LTP_pred_Q14 = SMLAWB(LTP_pred_Q14, sLTP_Q15[pred_lag_ptr-3], b_Q14[b_Q14_ptr+3])
			LTP_pred_Q14 = SMLAWB(LTP_pred_Q14, sLTP_Q15[pred_lag_ptr-4], b_Q14[b_Q14_ptr+4])
			LTP_pred_Q14 = LSHIFT32(LTP_pred_Q14, 1)
			pred_lag_ptr += 1
		} else {
			LTP_pred_Q14 = 0
		}

		if lag > 0 {
			n_LTP_Q14 = SMULWB(ADD32(s.sLTP_shp_Q14[shp_lag_ptr], s.sLTP_shp_Q14[shp_lag_ptr-2]), HarmShapeFIRPacked_Q14)
			n_LTP_Q14 = SMLAWT(n_LTP_Q14, s.sLTP_shp_Q14[shp_lag_ptr-1], HarmShapeFIRPacked_Q14)
			n_LTP_Q14 = SUB_LSHIFT32(LTP_pred_Q14, n_LTP_Q14, 2)
			shp_lag_ptr += 1
		} else {
			n_LTP_Q14 = 0
		}

		for k = 0; k < nStatesDelayedDecision; k++ {
			psDD = psDelDec[k]
			SS_left = 2 * k
			SS_right = SS_left + 1

			psDD.Seed = silk_RAND(psDD.Seed)
			psLPC_Q14 = NSQ_LPC_BUF_LENGTH - 1 + i

			LPC_pred_Q14 = RSHIFT32(predictLPCOrder, 1)
			LPC_pred_Q14 = SMLAWB(LPC_pred_Q14, psDD.sLPC_Q14[psLPC_Q14], a_Q12[0])
			LPC_pred_Q14 = SMLAWB(LPC_pred_Q14, psDD.sLPC_Q14[psLPC_Q14-1], a_Q12[1])
			LPC_pred_Q14 = SMLAWB(LPC_pred_Q14, psDD.sLPC_Q14[psLPC_Q14-2], a_Q12[2])
			LPC_pred_Q14 = SMLAWB(LPC_pred_Q14, psDD.sLPC_Q14[psLPC_Q14-3], a_Q12[3])
			LPC_pred_Q14 = SMLAWB(LPC_pred_Q14, psDD.sLPC_Q14[psLPC_Q14-4], a_Q12[4])
			LPC_pred_Q14 = SMLAWB(LPC_pred_Q14, psDD.sLPC_Q14[psLPC_Q14-5], a_Q12[5])
			LPC_pred_Q14 = SMLAWB(LPC_pred_Q14, psDD.sLPC_Q14[psLPC_Q14-6], a_Q12[6])
			LPC_pred_Q14 = SMLAWB(LPC_pred_Q14, psDD.sLPC_Q14[psLPC_Q14-7], a_Q12[7])
			LPC_pred_Q14 = SMLAWB(LPC_pred_Q14, psDD.sLPC_Q14[psLPC_Q14-8], a_Q12[8])
			LPC_pred_Q14 = SMLAWB(LPC_pred_Q14, psDD.sLPC_Q14[psLPC_Q14-9], a_Q12[9])
			if predictLPCOrder == 16 {
				LPC_pred_Q14 = SMLAWB(LPC_pred_Q14, psDD.sLPC_Q14[psLPC_Q14-10], a_Q12[10])
				LPC_pred_Q14 = SMLAWB(LPC_pred_Q14, psDD.sLPC_Q14[psLPC_Q14-11], a_Q12[11])
				LPC_pred_Q14 = SMLAWB(LPC_pred_Q14, psDD.sLPC_Q14[psLPC_Q14-12], a_Q12[12])
				LPC_pred_Q14 = SMLAWB(LPC_pred_Q14, psDD.sLPC_Q14[psLPC_Q14-13], a_Q12[13])
				LPC_pred_Q14 = SMLAWB(LPC_pred_Q14, psDD.sLPC_Q14[psLPC_Q14-14], a_Q12[14])
				LPC_pred_Q14 = SMLAWB(LPC_pred_Q14, psDD.sLPC_Q14[psLPC_Q14-15], a_Q12[15])
			}
			LPC_pred_Q14 = LSHIFT32(LPC_pred_Q14, 4)

			tmp2 = SMLAWB(psDD.sLPC_Q14[psLPC_Q14], psDD.sAR2_Q14[0], warping_Q16)
			tmp1 = SMLAWB(psDD.sAR2_Q14[0], psDD.sAR2_Q14[1]-tmp2, warping_Q16)
			psDD.sAR2_Q14[0] = tmp2
			n_AR_Q14 = RSHIFT32(shapingLPCOrder, 1)
			n_AR_Q14 = SMLAWB(n_AR_Q14, tmp2, AR_shp_Q13[AR_shp_Q13_ptr])

			for j = 2; j < shapingLPCOrder; j += 2 {
				tmp2 = SMLAWB(psDD.sAR2_Q14[j-1], psDD.sAR2_Q14[j+0]-tmp1, warping_Q16)
				psDD.sAR2_Q14[j-1] = tmp1
				n_AR_Q14 = SMLAWB(n_AR_Q14, tmp1, AR_shp_Q13[AR_shp_Q13_ptr+j-1])
				tmp1 = SMLAWB(psDD.sAR2_Q14[j+0], psDD.sAR2_Q14[j+1]-tmp2, warping_Q16)
				psDD.sAR2_Q14[j+0] = tmp2
				n_AR_Q14 = SMLAWB(n_AR_Q14, tmp2, AR_shp_Q13[AR_shp_Q13_ptr+j])
			}
			psDD.sAR2_Q14[shapingLPCOrder-1] = tmp1
			n_AR_Q14 = SMLAWB(n_AR_Q14, tmp1, AR_shp_Q13[AR_shp_Q13_ptr+shapingLPCOrder-1])

			n_AR_Q14 = LSHIFT32(n_AR_Q14, 1)
			n_AR_Q14 = SMLAWB(n_AR_Q14, psDD.LF_AR_Q14, Tilt_Q14)
			n_AR_Q14 = LSHIFT32(n_AR_Q14, 2)

			n_LF_Q14 = SMULWB(psDD.Shape_Q14[smpl_buf_idx], LF_shp_Q14)
			n_LF_Q14 = SMLAWT(n_LF_Q14, psDD.LF_AR_Q14, LF_shp_Q14)
			n_LF_Q14 = LSHIFT32(n_LF_Q14, 2)

			tmp1 = ADD32(n_AR_Q14, n_LF_Q14)
			tmp2 = ADD32(n_LTP_Q14, LPC_pred_Q14)
			tmp1 = SUB32(tmp2, tmp1)
			tmp1 = RSHIFT_ROUND(tmp1, 4)

			r_Q10 = SUB32(x_Q10[i], tmp1)
			if psDD.Seed < 0 {
				r_Q10 = -r_Q10
			}
			r_Q10 = LIMIT_32(r_Q10, -(31 << 10), 30<<10)

			q1_Q10 = SUB32(r_Q10, offset_Q10)
			q1_Q0 = RSHIFT32(q1_Q10, 10)
			if q1_Q0 > 0 {
				q1_Q10 = SUB32(LSHIFT32(q1_Q0, 10), QUANT_LEVEL_ADJUST_Q10)
				q1_Q10 = ADD32(q1_Q10, offset_Q10)
				q2_Q10 = ADD32(q1_Q10, 1024)
				rd1_Q10 = SMULBB(q1_Q10, Lambda_Q10)
				rd2_Q10 = SMULBB(q2_Q10, Lambda_Q10)
			} else if q1_Q0 == 0 {
				q1_Q10 = offset_Q10
				q2_Q10 = ADD32(q1_Q10, 1024-QUANT_LEVEL_ADJUST_Q10)
				rd1_Q10 = SMULBB(q1_Q10, Lambda_Q10)
				rd2_Q10 = SMULBB(q2_Q10, Lambda_Q10)
			} else if q1_Q0 == -1 {
				q2_Q10 = offset_Q10
				q1_Q10 = SUB32(q2_Q10, 1024-QUANT_LEVEL_ADJUST_Q10)
				rd1_Q10 = SMULBB(-q1_Q10, Lambda_Q10)
				rd2_Q10 = SMULBB(q2_Q10, Lambda_Q10)
			} else {
				q1_Q10 = ADD32(LSHIFT32(q1_Q0, 10), QUANT_LEVEL_ADJUST_Q10)
				q1_Q10 = ADD32(q1_Q10, offset_Q10)
				q2_Q10 = ADD32(q1_Q10, 1024)
				rd1_Q10 = SMULBB(-q1_Q10, Lambda_Q10)
				rd2_Q10 = SMULBB(-q2_Q10, Lambda_Q10)
			}
			rr_Q10 = SUB32(r_Q10, q1_Q10)
			rd1_Q10 = RSHIFT32(SMLABB(rd1_Q10, rr_Q10, rr_Q10), 10)
			rr_Q10 = SUB32(r_Q10, q2_Q10)
			rd2_Q10 = RSHIFT32(SMLABB(rd2_Q10, rr_Q10, rr_Q10), 10)

			if rd1_Q10 < rd2_Q10 {
				sampleStates[SS_left].RD_Q10 = ADD32(psDD.RD_Q10, rd1_Q10)
				sampleStates[SS_right].RD_Q10 = ADD32(psDD.RD_Q10, rd2_Q10)
				sampleStates[SS_left].Q_Q10 = q1_Q10
				sampleStates[SS_right].Q_Q10 = q2_Q10
			} else {
				sampleStates[SS_left].RD_Q10 = ADD32(psDD.RD_Q10, rd2_Q10)
				sampleStates[SS_right].RD_Q10 = ADD32(psDD.RD_Q10, rd1_Q10)
				sampleStates[SS_left].Q_Q10 = q2_Q10
				sampleStates[SS_right].Q_Q10 = q1_Q10
			}

			exc_Q14 = LSHIFT32(sampleStates[SS_left].Q_Q10, 4)
			if psDD.Seed < 0 {
				exc_Q14 = -exc_Q14
			}

			LPC_exc_Q14 = ADD32(exc_Q14, LTP_pred_Q14)
			xq_Q14 = ADD32(LPC_exc_Q14, LPC_pred_Q14)

			sLF_AR_shp_Q14 = SUB32(xq_Q14, n_AR_Q14)
			sampleStates[SS_left].sLTP_shp_Q14 = SUB32(sLF_AR_shp_Q14, n_LF_Q14)
			sampleStates[SS_left].LF_AR_Q14 = sLF_AR_shp_Q14
			sampleStates[SS_left].LPC_exc_Q14 = LPC_exc_Q14
			sampleStates[SS_left].xq_Q14 = xq_Q14

			exc_Q14 = LSHIFT32(sampleStates[SS_right].Q_Q10, 4)
			if psDD.Seed < 0 {
				exc_Q14 = -exc_Q14
			}

			LPC_exc_Q14 = ADD32(exc_Q14, LTP_pred_Q14)
			xq_Q14 = ADD32(LPC_exc_Q14, LPC_pred_Q14)

			sLF_AR_shp_Q14 = SUB32(xq_Q14, n_AR_Q14)
			sampleStates[SS_right].sLTP_shp_Q14 = SUB32(sLF_AR_shp_Q14, n_LF_Q14)
			sampleStates[SS_right].LF_AR_Q14 = sLF_AR_shp_Q14
			sampleStates[SS_right].LPC_exc_Q14 = LPC_exc_Q14
			sampleStates[SS_right].xq_Q14 = xq_Q14
		}

		smpl_buf_idx = (smpl_buf_idx - 1) & DECISION_DELAY_MASK
		last_smple_idx = (smpl_buf_idx + decisionDelay) & DECISION_DELAY_MASK

		RDmin_Q10 = sampleStates[0].RD_Q10
		Winner_ind = 0
		for k = 1; k < nStatesDelayedDecision; k++ {
			if sampleStates[k*2].RD_Q10 < RDmin_Q10 {
				RDmin_Q10 = sampleStates[k*2].RD_Q10
				Winner_ind = k
			}
		}

		Winner_rand_state = psDelDec[Winner_ind].RandState[last_smple_idx]
		for k = 0; k < nStatesDelayedDecision; k++ {
			if psDelDec[k].RandState[last_smple_idx] != Winner_rand_state {
				k2 := k * 2
				sampleStates[k2].RD_Q10 = ADD32(sampleStates[k2].RD_Q10, math.MaxInt32>>4)
				sampleStates[k2+1].RD_Q10 = ADD32(sampleStates[k2+1].RD_Q10, math.MaxInt32>>4)
			}
		}

		RDmin_Q10 = sampleStates[1].RD_Q10
		RDmax_ind = 0
		RDmin_ind = 0
		for k = 1; k < nStatesDelayedDecision; k++ {
			k2 := k * 2
			if sampleStates[k2].RD_Q10 > RDmax_Q10 {
				RDmax_Q10 = sampleStates[k2].RD_Q10
				RDmax_ind = k
			}
			if sampleStates[k2+1].RD_Q10 < RDmin_Q10 {
				RDmin_Q10 = sampleStates[k2+1].RD_Q10
				RDmin_ind = k
			}
		}

		if RDmin_Q10 < RDmax_Q10 {
			psDelDec[RDmax_ind].PartialCopyFrom(psDelDec[RDmin_ind], int(i))
			sampleStates[RDmax_ind*2].Assign(sampleStates[RDmin_ind*2+1])
		}

		psDD = psDelDec[Winner_ind]
		if subfr > 0 || i >= decisionDelay {
			pulses[pulses_ptr+i-decisionDelay] = byte(RSHIFT_ROUND(psDD.Q_Q10[last_smple_idx], 10))
			xq[xq_ptr+i-decisionDelay] = int16(SAT16(RSHIFT_ROUND(
				SMULWW(psDD.Xq_Q14[last_smple_idx], delayedGain_Q10[last_smple_idx]), 8)))
			s.sLTP_shp_Q14[s.sLTP_shp_buf_idx-decisionDelay] = psDD.Shape_Q14[last_smple_idx]
			sLTP_Q15[s.sLTP_buf_idx-decisionDelay] = psDD.Pred_Q15[last_smple_idx]
		}
		s.sLTP_shp_buf_idx++
		s.sLTP_buf_idx++

		for k = 0; k < nStatesDelayedDecision; k++ {
			psDD = psDelDec[k]
			SS_left = k * 2
			psDD.LF_AR_Q14 = sampleStates[SS_left].LF_AR_Q14
			psDD.sLPC_Q14[NSQ_LPC_BUF_LENGTH+i] = sampleStates[SS_left].xq_Q14
			psDD.Xq_Q14[smpl_buf_idx] = sampleStates[SS_left].xq_Q14
			psDD.Q_Q10[smpl_buf_idx] = sampleStates[SS_left].Q_Q10
			psDD.Pred_Q15[smpl_buf_idx] = LSHIFT32(sampleStates[SS_left].LPC_exc_Q14, 1)
			psDD.Shape_Q14[smpl_buf_idx] = sampleStates[SS_left].sLTP_shp_Q14
			psDD.Seed = ADD32_ovflw(psDD.Seed, RSHIFT_ROUND(sampleStates[SS_left].Q_Q10, 10))
			psDD.RandState[smpl_buf_idx] = psDD.Seed
			psDD.RD_Q10 = sampleStates[SS_left].RD_Q10
		}
		delayedGain_Q10[smpl_buf_idx] = Gain_Q10
	}

	for k = 0; k < nStatesDelayedDecision; k++ {
		psDD = psDelDec[k]
		copy(psDD.sLPC_Q14[:], psDD.sLPC_Q14[length:])
	}

	return smpl_buf_idx
}

func (s *SilkNSQState) silk_nsq_del_dec_scale_states(
	psEncC *SilkChannelEncoder,
	psDelDec []*nsq_del_dec_struct,
	x_Q3 []int32,
	x_Q3_ptr int32,
	x_sc_Q10 []int32,
	sLTP []int16,
	sLTP_Q15 []int32,
	subfr int32,
	nStatesDelayedDecision int32,
	LTP_scale_Q14 int32,
	Gains_Q16 []int32,
	pitchL []int32,
	signal_type int32,
	decisionDelay int32,
) {
	var i, k, lag int32
	var gain_adj_Q16, inv_gain_Q31, inv_gain_Q23 int32
	var psDD *nsq_del_dec_struct

	lag = pitchL[subfr]
	inv_gain_Q31 = INVERSE32_varQ(max_int32(Gains_Q16[subfr], 1), 47)

	if Gains_Q16[subfr] != s.prev_gain_Q16 {
		gain_adj_Q16 = DIV32_varQ(s.prev_gain_Q16, Gains_Q16[subfr], 16)
	} else {
		gain_adj_Q16 = 1 << 16
	}

	inv_gain_Q23 = RSHIFT_ROUND(inv_gain_Q31, 8)
	for i = 0; i < psEncC.subfr_length; i++ {
		x_sc_Q10[i] = SMULWW(x_Q3[x_Q3_ptr+i], inv_gain_Q23)
	}

	s.prev_gain_Q16 = Gains_Q16[subfr]

	if s.rewhite_flag != 0 {
		if subfr == 0 {
			inv_gain_Q31 = LSHIFT32(SMULWB(inv_gain_Q31, LTP_scale_Q14), 2)
		}
		for i = s.sLTP_buf_idx - lag - LTP_ORDER/2; i < s.sLTP_buf_idx; i++ {
			sLTP_Q15[i] = SMULWB(inv_gain_Q31, sLTP[i])
		}
	}

	if gain_adj_Q16 != (1 << 16) {
		for i = s.sLTP_shp_buf_idx - psEncC.ltp_mem_length; i < s.sLTP_shp_buf_idx; i++ {
			s.sLTP_shp_Q14[i] = SMULWW(gain_adj_Q16, s.sLTP_shp_Q14[i])
		}

		if signal_type == TYPE_VOICED && s.rewhite_flag == 0 {
			for i = s.sLTP_buf_idx - lag - LTP_ORDER/2; i < s.sLTP_buf_idx-decisionDelay; i++ {
				sLTP_Q15[i] = SMULWW(gain_adj_Q16, sLTP_Q15[i])
			}
		}

		for k = 0; k < nStatesDelayedDecision; k++ {
			psDD = psDelDec[k]
			psDD.LF_AR_Q14 = SMULWW(gain_adj_Q16, psDD.LF_AR_Q14)

			for i = 0; i < NSQ_LPC_BUF_LENGTH; i++ {
				psDD.sLPC_Q14[i] = SMULWW(gain_adj_Q16, psDD.sLPC_Q14[i])
			}
			for i = 0; i < psEncC.shapingLPCOrder; i++ {
				psDD.sAR2_Q14[i] = SMULWW(gain_adj_Q16, psDD.sAR2_Q14[i])
			}
			for i = 0; i < DECISION_DELAY; i++ {
				psDD.Pred_Q15[i] = SMULWW(gain_adj_Q16, psDD.Pred_Q15[i])
				psDD.Shape_Q14[i] = SMULWW(gain_adj_Q16, psDD.Shape_Q14[i])
			}
		}
	}
}
