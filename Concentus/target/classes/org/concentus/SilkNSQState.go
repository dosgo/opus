
package silk

import (
	"math"
)

// NSQState represents the noise shaping quantization state
type NSQState struct {
	// Buffer for quantized output signal
	xq            [2 * MaxFrameLength]int16
	sLTP_shp_Q14  [2 * MaxFrameLength]int32
	sLPC_Q14      [MaxSubFrameLength + NSQLPCBufLength]int32
	sAR2_Q14      [MaxShapeLPCOrder]int32
	sLF_AR_shp_Q14 int32
	lagPrev       int32
	sLTP_buf_idx  int32
	sLTP_shp_buf_idx int32
	rand_seed     int32
	prev_gain_Q16 int32
	rewhite_flag  int32
}

// Reset resets the NSQ state
func (s *NSQState) Reset() {
	for i := range s.xq {
		s.xq[i] = 0
	}
	for i := range s.sLTP_shp_Q14 {
		s.sLTP_shp_Q14[i] = 0
	}
	for i := range s.sLPC_Q14 {
		s.sLPC_Q14[i] = 0
	}
	for i := range s.sAR2_Q14 {
		s.sAR2_Q14[i] = 0
	}
	s.sLF_AR_shp_Q14 = 0
	s.lagPrev = 0
	s.sLTP_buf_idx = 0
	s.sLTP_shp_buf_idx = 0
	s.rand_seed = 0
	s.prev_gain_Q16 = 0
	s.rewhite_flag = 0
}

// Assign copies another NSQ state to this one
func (s *NSQState) Assign(other *NSQState) {
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

// NSQDelDecStruct represents the delayed decision structure
type NSQDelDecStruct struct {
	sLPC_Q14      [MaxSubFrameLength + NSQLPCBufLength]int32
	RandState     [DecisionDelay]int32
	Q_Q10         [DecisionDelay]int32
	Xq_Q14        [DecisionDelay]int32
	Pred_Q15      [DecisionDelay]int32
	Shape_Q14     [DecisionDelay]int32
	sAR2_Q14      []int32
	LF_AR_Q14     int32
	Seed          int32
	SeedInit      int32
	RD_Q10        int32
}

// NewNSQDelDecStruct creates a new NSQDelDecStruct
func NewNSQDelDecStruct(shapingOrder int) *NSQDelDecStruct {
	return &NSQDelDecStruct{
		sAR2_Q14: make([]int32, shapingOrder),
	}
}

// PartialCopyFrom copies partial state from another NSQDelDecStruct
func (d *NSQDelDecStruct) PartialCopyFrom(other *NSQDelDecStruct, q14Offset int) {
	copy(d.sLPC_Q14[q14Offset:], other.sLPC_Q14[q14Offset:MaxSubFrameLength+NSQLPCBufLength-q14Offset])
	copy(d.RandState[:], other.RandState[:])
	copy(d.Q_Q10[:], other.Q_Q10[:])
	copy(d.Xq_Q14[:], other.Xq_Q14[:])
	copy(d.Pred_Q15[:], other.Pred_Q15[:])
	copy(d.Shape_Q14[:], other.Shape_Q14[:])
	copy(d.sAR2_Q14, other.sAR2_Q14)
	d.LF_AR_Q14 = other.LF_AR_Q14
	d.Seed = other.Seed
	d.SeedInit = other.SeedInit
	d.RD_Q10 = other.RD_Q10
}

// Assign copies another NSQDelDecStruct to this one
func (d *NSQDelDecStruct) Assign(other *NSQDelDecStruct) {
	d.PartialCopyFrom(other, 0)
}

// NSQSampleStruct represents a sample structure
type NSQSampleStruct struct {
	Q_Q10        int32
	RD_Q10       int32
	xq_Q14       int32
	LF_AR_Q14    int32
	sLTP_shp_Q14 int32
	LPC_exc_Q14  int32
}

// Assign copies another NSQSampleStruct to this one
func (s *NSQSampleStruct) Assign(other *NSQSampleStruct) {
	s.Q_Q10 = other.Q_Q10
	s.RD_Q10 = other.RD_Q10
	s.xq_Q14 = other.xq_Q14
	s.LF_AR_Q14 = other.LF_AR_Q14
	s.sLTP_shp_Q14 = other.sLTP_shp_Q14
	s.LPC_exc_Q14 = other.LPC_exc_Q14
}

// NSQ performs noise shaping quantization
func (s *NSQState) NSQ(
	psEncC *ChannelEncoder,
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

	// Set unvoiced lag to the previous one, overwrite later for voiced
	lag = s.lagPrev

	// Assert prev_gain_Q16 != 0
	if s.prev_gain_Q16 == 0 {
		panic("prev_gain_Q16 should not be 0")
	}

	offset_Q10 = QuantizationOffsets_Q10[psIndices.signalType>>1][psIndices.quantOffsetType]

	if psIndices.NLSFInterpCoef_Q2 == 4 {
		LSF_interpolation_flag = 0
	} else {
		LSF_interpolation_flag = 1
	}

	sLTP_Q15 = make([]int32, psEncC.ltp_mem_length+psEncC.frame_length)
	sLTP = make([]int16, psEncC.ltp_mem_length+psEncC.frame_length)
	x_sc_Q10 = make([]int32, psEncC.subfr_length)
	// Set up pointers to start of sub frame
	s.sLTP_shp_buf_idx = psEncC.ltp_mem_length
	s.sLTP_buf_idx = psEncC.ltp_mem_length
	pxq = psEncC.ltp_mem_length

	for k = 0; k < psEncC.nb_subfr; k++ {
		A_Q12 = ((k >> 1) | (1 - LSF_interpolation_flag))
		B_Q14 = k * LTPOrder // opt: does this indicate a partitioned array?
		AR_shp_Q13 = k * MaxShapeLPCOrder // opt: same here

		// Noise shape parameters
		if HarmShapeGain_Q14[k] < 0 {
			panic("HarmShapeGain_Q14 should be >= 0")
		}
		HarmShapeFIRPacked_Q14 = RSHIFT(HarmShapeGain_Q14[k], 2)
		HarmShapeFIRPacked_Q14 |= LSHIFT(int32(RSHIFT(HarmShapeGain_Q14[k], 1)), 16)

		s.rewhite_flag = 0
		if psIndices.signalType == TypeVoiced {
			// Voiced
			lag = pitchL[k]

			// Re-whitening
			if (k & (3 - LSHIFT(LSF_interpolation_flag, 1))) == 0 {
				// Rewhiten with new A coefs
				start_idx = psEncC.ltp_mem_length - lag - psEncC.predictLPCOrder - LTPOrder/2
				if start_idx <= 0 {
					panic("start_idx should be > 0")
				}

				LPC_analysis_filter(sLTP, start_idx, s.xq[:], start_idx+k*psEncC.subfr_length,
					PredCoef_Q12[A_Q12], 0, psEncC.ltp_mem_length-start_idx, psEncC.predictLPCOrder)

				s.rewhite_flag = 1
				s.sLTP_buf_idx = psEncC.ltp_mem_length
			}
		}

		s.nsq_scale_states(psEncC, x_Q3, x_Q3_ptr, x_sc_Q10, sLTP, sLTP_Q15, k, LTP_scale_Q14, Gains_Q16, pitchL, psIndices.signalType)

		s.noise_shape_quantizer(
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
			psEncC.predictLPCOrder,
		)

		x_Q3_ptr += psEncC.subfr_length
		pulses_ptr += psEncC.subfr_length
		pxq += psEncC.subfr_length
	}

	// Update lagPrev for next frame
	s.lagPrev = pitchL[psEncC.nb_subfr-1]

	// Save quantized speech and noise shaping signals
	copy(s.xq[:psEncC.ltp_mem_length], s.xq[psEncC.frame_length:psEncC.frame_length+psEncC.ltp_mem_length])
	copy(s.sLTP_shp_Q14[:psEncC.ltp_mem_length], s.sLTP_shp_Q14[psEncC.frame_length:psEncC.frame_length+psEncC.ltp_mem_length])
}

// noise_shape_quantizer performs noise shaping quantization
func (s *NSQState) noise_shape_quantizer(
	signalType int32,
	x_sc_Q10 []int32,
	pulses []byte,
	pulses_ptr int,
	xq []int16,
	xq_ptr int32,
	sLTP_Q15 []int32,
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

	shp_lag_ptr = s.sLTP_shp_buf_idx - lag + HarmShapeFirTaps/2
	pred_lag_ptr = s.sLTP_buf_idx - lag + LTPOrder/2
	Gain_Q10 = RSHIFT(Gain_Q16, 6)

	// Set up short term AR state
	psLPC_Q14 = NSQLPCBufLength - 1

	for i = 0; i < length; i++ {
		// Generate dither
		s.rand_seed = RAND(s.rand_seed)

		// Short-term prediction
		if predictLPCOrder != 10 && predictLPCOrder != 16 {
			panic("predictLPCOrder should be 10 or 16")
		}
		// Avoids introducing a bias because SMLAWB() always rounds to -inf
		LPC_pred_Q10 = RSHIFT(predictLPCOrder, 1)
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-0], int32(a_Q12[0]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-1], int32(a_Q12[1]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-2], int32(a_Q12[2]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-3], int32(a_Q12[3]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-4], int32(a_Q12[4]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-5], int32(a_Q12[5]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-6], int32(a_Q12[6]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-7], int32(a_Q12[7]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-8], int32(a_Q12[8]))
		LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-9], int32(a_Q12[9]))
		if predictLPCOrder == 16 {
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-10], int32(a_Q12[10]))
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-11], int32(a_Q12[11]))
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-12], int32(a_Q12[12]))
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-13], int32(a_Q12[13]))
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-14], int32(a_Q12[14]))
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, s.sLPC_Q14[psLPC_Q14-15], int32(a_Q12[15]))
		}

		// Long-term prediction
		if signalType == TypeVoiced {
			// Unrolled loop
			// Avoids introducing a bias because SMLAWB() always rounds to -inf
			LTP_pred_Q13 = 2
			LTP_pred_Q13 = SMLAWB(LTP_pred_Q13, sLTP_Q15[pred_lag_ptr], int32(b_Q14[b_Q14_ptr+0]))
			LTP_pred_Q13 = SMLAWB(LTP_pred_Q13, sLTP_Q15[pred_lag_ptr-1], int32(b_Q14[b_Q14_ptr+1]))
			LTP_pred_Q13 = SMLAWB(LTP_pred_Q13, sLTP_Q15[pred_lag_ptr-2], int32(b_Q14[b_Q14_ptr+2]))
			LTP_pred_Q13 = SMLAWB(LTP_pred_Q13, sLTP_Q15[pred_lag_ptr-3], int32(b_Q14[b_Q14_ptr+3]))
			LTP_pred_Q13 = SMLAWB(LTP_pred_Q13, sLTP_Q15[pred_lag_ptr-4], int32(b_Q14[b_Q14_ptr+4]))
			pred_lag_ptr += 1
		} else {
			LTP_pred_Q13 = 0
		}

		// Noise shape feedback
		if (shapingLPCOrder & 1) != 0 {
			panic("shapingLPCOr