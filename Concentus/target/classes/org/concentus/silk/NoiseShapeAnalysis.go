package silk

import (
	"math"
)

const (
	SHAPE_WHITE_NOISE_FRACTION = 1e-5
)

type NoiseShapeAnalysis struct{}

func (n *NoiseShapeAnalysis) warped_gain(coefs_Q24 []int32, lambda_Q16 int32, order int) int32 {
	var i int
	var gain_Q24 int32

	lambda_Q16 = -lambda_Q16
	gain_Q24 = coefs_Q24[order-1]
	for i = order - 2; i >= 0; i-- {
		gain_Q24 = SMLAWB(coefs_Q24[i], gain_Q24, lambda_Q16)
	}
	gain_Q24 = SMLAWB(1<<24, gain_Q24, -lambda_Q16)
	return INVERSE32_varQ(gain_Q24, 40)
}

func (n *NoiseShapeAnalysis) limit_warped_coefs(
	coefs_syn_Q24 []int32,
	coefs_ana_Q24 []int32,
	lambda_Q16 int32,
	limit_Q24 int32,
	order int) {

	var i, iter, ind int
	var tmp, maxabs_Q24, chirp_Q16, gain_syn_Q16, gain_ana_Q16 int32
	var nom_Q16, den_Q24 int32

	// Convert to monic coefficients
	lambda_Q16 = -lambda_Q16
	for i = order - 1; i > 0; i-- {
		coefs_syn_Q24[i-1] = SMLAWB(coefs_syn_Q24[i-1], coefs_syn_Q24[i], lambda_Q16)
		coefs_ana_Q24[i-1] = SMLAWB(coefs_ana_Q24[i-1], coefs_ana_Q24[i], lambda_Q16)
	}
	lambda_Q16 = -lambda_Q16
	nom_Q16 = SMLAWB(1<<16, -lambda_Q16, lambda_Q16)
	den_Q24 = SMLAWB(1<<24, coefs_syn_Q24[0], lambda_Q16)
	gain_syn_Q16 = DIV32_varQ(nom_Q16, den_Q24, 24)
	den_Q24 = SMLAWB(1<<24, coefs_ana_Q24[0], lambda_Q16)
	gain_ana_Q16 = DIV32_varQ(nom_Q16, den_Q24, 24)
	for i = 0; i < order; i++ {
		coefs_syn_Q24[i] = SMULWW(gain_syn_Q16, coefs_syn_Q24[i])
		coefs_ana_Q24[i] = SMULWW(gain_ana_Q16, coefs_ana_Q24[i])
	}

	for iter = 0; iter < 10; iter++ {
		// Find maximum absolute value
		maxabs_Q24 = -1
		for i = 0; i < order; i++ {
			tmp = max_int32(abs_int32(coefs_syn_Q24[i]), abs_int32(coefs_ana_Q24[i]))
			if tmp > maxabs_Q24 {
				maxabs_Q24 = tmp
				ind = i
			}
		}
		if maxabs_Q24 <= limit_Q24 {
			// Coefficients are within range - done
			return
		}

		// Convert back to true warped coefficients
		for i = 1; i < order; i++ {
			coefs_syn_Q24[i-1] = SMLAWB(coefs_syn_Q24[i-1], coefs_syn_Q24[i], lambda_Q16)
			coefs_ana_Q24[i-1] = SMLAWB(coefs_ana_Q24[i-1], coefs_ana_Q24[i], lambda_Q16)
		}
		gain_syn_Q16 = INVERSE32_varQ(gain_syn_Q16, 32)
		gain_ana_Q16 = INVERSE32_varQ(gain_ana_Q16, 32)
		for i = 0; i < order; i++ {
			coefs_syn_Q24[i] = SMULWW(gain_syn_Q16, coefs_syn_Q24[i])
			coefs_ana_Q24[i] = SMULWW(gain_ana_Q16, coefs_ana_Q24[i])
		}

		// Apply bandwidth expansion
		chirp_Q16 = int32(0.99*(1<<16)) - DIV32_varQ(
			SMULWB(maxabs_Q24-limit_Q24, SMLABB(int32(0.8*(1<<10)), int32(0.1*(1<<10)), iter)),
			MUL(maxabs_Q24, int32(ind+1)), 22)
		BWExpander_32(coefs_syn_Q24, order, chirp_Q16)
		BWExpander_32(coefs_ana_Q24, order, chirp_Q16)

		// Convert to monic warped coefficients
		lambda_Q16 = -lambda_Q16
		for i = order - 1; i > 0; i-- {
			coefs_syn_Q24[i-1] = SMLAWB(coefs_syn_Q24[i-1], coefs_syn_Q24[i], lambda_Q16)
			coefs_ana_Q24[i-1] = SMLAWB(coefs_ana_Q24[i-1], coefs_ana_Q24[i], lambda_Q16)
		}
		lambda_Q16 = -lambda_Q16
		nom_Q16 = SMLAWB(1<<16, -lambda_Q16, lambda_Q16)
		den_Q24 = SMLAWB(1<<24, coefs_syn_Q24[0], lambda_Q16)
		gain_syn_Q16 = DIV32_varQ(nom_Q16, den_Q24, 24)
		den_Q24 = SMLAWB(1<<24, coefs_ana_Q24[0], lambda_Q16)
		gain_ana_Q16 = DIV32_varQ(nom_Q16, den_Q24, 24)
		for i = 0; i < order; i++ {
			coefs_syn_Q24[i] = SMULWW(gain_syn_Q16, coefs_syn_Q24[i])
			coefs_ana_Q24[i] = SMULWW(gain_ana_Q16, coefs_ana_Q24[i])
		}
	}
	OpusAssert(false)
}

func (n *NoiseShapeAnalysis) noise_shape_analysis(
	psEnc *SilkChannelEncoder,
	psEncCtrl *SilkEncoderControl,
	pitch_res []int16,
	pitch_res_ptr int,
	x []int16,
	x_ptr int) {

	psShapeSt := psEnc.sShape
	var k, i, nSamples, Qnrg, b_Q14 int
	var warping_Q16, scale int32
	var SNR_adj_dB_Q7, HarmBoost_Q16, HarmShapeGain_Q16, Tilt_Q16, tmp32 int32
	var nrg, pre_nrg_Q30, log_energy_Q7, log_energy_prev_Q7, energy_variation_Q7 int32
	var delta_Q16, BWExp1_Q16, BWExp2_Q16, gain_mult_Q16, gain_add_Q16, strength_Q16, b_Q8 int32
	auto_corr := make([]int32, MAX_SHAPE_LPC_ORDER+1)
	refl_coef_Q16 := make([]int32, MAX_SHAPE_LPC_ORDER)
	AR1_Q24 := make([]int32, MAX_SHAPE_LPC_ORDER)
	AR2_Q24 := make([]int32, MAX_SHAPE_LPC_ORDER)
	var x_windowed []int16
	pitch_res_ptr2 := pitch_res_ptr
	x_ptr2 := x_ptr - psEnc.la_shape

	// GAIN CONTROL
	SNR_adj_dB_Q7 = psEnc.SNR_dB_Q7

	// Input quality is the average of the quality in the lowest two VAD bands
	psEncCtrl.input_quality_Q14 = int32(RSHIFT(int64(psEnc.input_quality_bands_Q15[0]+
		psEnc.input_quality_bands_Q15[1]), 2))

	// Coding quality level
	psEncCtrl.coding_quality_Q14 = RSHIFT(Sigmoid_Q15(RSHIFT_ROUND(SNR_adj_dB_Q7-
		int32(20.0*(1<<7)), 4)), 1)

	// Reduce coding SNR during low speech activity
	if psEnc.useCBR == 0 {
		b_Q8 = int32(1*(1<<8)) - psEnc.speech_activity_Q8
		b_Q8 = SMULWB(LSHIFT(b_Q8, 8), b_Q8)
		SNR_adj_dB_Q7 = SMLAWB(SNR_adj_dB_Q7,
			SMULBB(int32(0-TuningParameters.BG_SNR_DECR_dB*(1<<7))>>(4+1), b_Q8),
			SMULWB(int32(1*(1<<14))+psEncCtrl.input_quality_Q14, psEncCtrl.coding_quality_Q14))
	}

	if psEnc.indices.signalType == TYPE_VOICED {
		// Reduce gains for periodic signals
		SNR_adj_dB_Q7 = SMLAWB(SNR_adj_DB_Q7, int32(TuningParameters.HARM_SNR_INCR_dB*(1<<8)), psEnc.LTPCorr_Q15)
	} else {
		// For unvoiced signals and low-quality input
		SNR_adj_dB_Q7 = SMLAWB(SNR_adj_dB_Q7,
			SMLAWB(int32(6.0*(1<<9)), -int32(0.4*(1<<18)), psEnc.SNR_dB_Q7),
			int32(1*(1<<14))-psEncCtrl.input_quality_Q14)
	}

	// SPARSENESS PROCESSING
	if psEnc.indices.signalType == TYPE_VOICED {
		psEnc.indices.quantOffsetType = 0
		psEncCtrl.sparseness_Q8 = 0
	} else {
		// Sparseness measure
		nSamples = LSHIFT(psEnc.fs_kHz, 1)
		energy_variation_Q7 = 0
		log_energy_prev_Q7 = 0
		pitch_res_ptr2 = pitch_res_ptr
		boxed_nrg := &BoxedValueInt32{0}
		boxed_scale := &BoxedValueInt32{0}
		for k = 0; k < SMULBB(SUB_FRAME_LENGTH_MS, psEnc.nb_subfr)/2; k++ {
			sum_sqr_shift(boxed_nrg, boxed_scale, pitch_res, pitch_res_ptr2, nSamples)
			nrg = boxed_nrg.Val
			scale = boxed_scale.Val
			nrg += RSHIFT(int64(nSamples), scale)

			log_energy_Q7 = lin2log(nrg)
			if k > 0 {
				energy_variation_Q7 += abs_int32(log_energy_Q7 - log_energy_prev_Q7)
			}
			log_energy_prev_Q7 = log_energy_Q7
			pitch_res_ptr2 += nSamples
		}

		psEncCtrl.sparseness_Q8 = RSHIFT(Sigmoid_Q15(SMULWB(energy_variation_Q7-
			int32(5.0*(1<<7)), int32(0.1*(1<<16)))), 7)

		if psEncCtrl.sparseness_Q8 > int32(TuningParameters.SPARSENESS_THRESHOLD_QNT_OFFSET*(1<<8)) {
			psEnc.indices.quantOffsetType = 0
		} else {
			psEnc.indices.quantOffsetType = 1
		}

		SNR_adj_dB_Q7 = SMLAWB(SNR_adj_dB_Q7, int32(TuningParameters.SPARSE_SNR_INCR_dB*(1<<15)), 
			psEncCtrl.sparseness_Q8-int32(0.5*(1<<8)))
	}

	// Control bandwidth expansion
	strength_Q16 = SMULWB(psEncCtrl.predGain_Q16, int32(TuningParameters.FIND_PITCH_WHITE_NOISE_FRACTION*(1<<16)))
	BWExp1_Q16 = BWExp2_Q16 = DIV32_varQ(int32(TuningParameters.BANDWIDTH_EXPANSION*(1<<16)),
		SMLAWW(int32(1*(1<<16)), strength_Q16, strength_Q16), 16)
	delta_Q16 = SMULWB(int32(1*(1<<16))-SMULBB(3, psEncCtrl.coding_quality_Q14),
		int32(TuningParameters.LOW_RATE_BANDWIDTH_EXPANSION_DELTA*(1<<16)))
	BWExp1_Q16 = SUB32(BWExp1_Q16, delta_Q16)
	BWExp2_Q16 = ADD32(BWExp2_Q16, delta_Q16)
	BWExp1_Q16 = DIV32_16(LSHIFT(BWExp1_Q16, 14), RSHIFT(BWExp2_Q16, 2))

	if psEnc.warping_Q16 > 0 {
		warping_Q16 = SMLAWB(psEnc.warping_Q16, int32(psEncCtrl.coding_quality_Q14), int32(0.01*(1<<18)))
	} else {
		warping_Q16 = 0
	}

	// Compute noise shaping AR coefs and gains
	x_windowed = make([]int16, psEnc.shapeWinLength)
	for k = 0; k < psEnc.nb_subfr; k++ {
		// Apply window
		flat_part := psEnc.fs_kHz * 3
		slope_part := RSHIFT(psEnc.shapeWinLength-flat_part, 1)

		apply_sine_window(x_windowed, 0, x, x_ptr2, 1, slope_part)
		shift := slope_part
		copy(x_windowed[shift:], x[x_ptr2+shift:x_ptr2+shift+flat_part])
		shift += flat_part
		apply_sine_window(x_windowed, shift, x, x_ptr2+shift, 2, slope_part)

		// Update pointer
		x_ptr2 += psEnc.subfr_length
		scale_boxed := &BoxedValueInt32{scale}
		if psEnc.warping_Q16 > 0 {
			warped_autocorrelation(auto_corr, scale_boxed, x_windowed, warping_Q16, psEnc.shapeWinLength, psEnc.shapingLPCOrder)
		} else {
			autocorr(auto_corr, scale_boxed, x_windowed, psEnc.shapeWinLength, psEnc.shapingLPCOrder+1)
		}
		scale = scale_boxed.Val

		// Add white noise
		auto_corr[0] = ADD32(auto_corr[0], max_int32(SMULWB(RSHIFT(auto_corr[0], 4),
			int32(TuningParameters.SHAPE_WHITE_NOISE_FRACTION*(1<<20))), 1))

		// Calculate reflection coefficients
		nrg = schur64(refl_coef_Q16, auto_corr, psEnc.shapingLPCOrder)
		OpusAssert(nrg >= 0)

		// Convert reflection coefficients to prediction coefficients
		k2a_Q16(AR2_Q24, refl_coef_Q16, psEnc.shapingLPCOrder)

		Qnrg = -scale
		OpusAssert(Qnrg >= -12)
		OpusAssert(Qnrg <= 30)

		if (Qnrg & 1) != 0 {
			Qnrg -= 1
			nrg >>= 1
		}

		tmp32 = SQRT_APPROX(nrg)
		Qnrg >>= 1

		psEncCtrl.Gains_Q16[k] = LSHIFT_SAT32(tmp32, 16-Qnrg)

		if psEnc.warping_Q16 > 0 {
			gain_mult_Q16 = n.warped_gain(AR2_Q24, warping_Q16, psEnc.shapingLPCOrder)
			OpusAssert(psEncCtrl.Gains_Q16[k] >= 0)
			if SMULWW(RSHIFT_ROUND(psEncCtrl.Gains_Q16[k], 1), gain_mult_Q16) >= (math.MaxInt32 >> 1) {
				psEncCtrl.Gains_Q16[k] = math.MaxInt32
			} else {
				psEncCtrl.Gains_Q16[k] = SMULWW(psEncCtrl.Gains_Q16[k], gain_mult_Q16)
			}
		}

		// Bandwidth expansion for synthesis filter shaping
		BWExpander_32(AR2_Q24, psEnc.shapingLPCOrder, BWExp2_Q16)

		// Compute noise shaping filter coefficients
		copy(AR1_Q24, AR2_Q24)

		// Bandwidth expansion for analysis filter shaping
		OpusAssert(BWExp1_Q16 <= int32(1*(1<<16)))
		BWExpander_32(AR1_Q24, psEnc.shapingLPCOrder, BWExp1_Q16)

		// Ratio of prediction gains
				pre_nrg_Q30 = LSHIFT32(SMULWB(pre_nrg_Q30, int32(0.7*(1<<15))), 1)
		psEncCtrl.GainsPre_Q14[k] = int32(0.3*(1<<14)) + DIV32_varQ(pre_nrg_Q30, nrg, 14)

		// Convert to monic warped prediction coefficients and limit absolute values
		n.limit_warped_coefs(AR2_Q24, AR1_Q24, warping_Q16, int32(3.999*(1<<24)), psEnc.shapingLPCOrder)

		// Convert from Q24 to Q13 and store in int16
		for i = 0; i < psEnc.shapingLPCOrder; i++ {
			psEncCtrl.AR1_Q13[k*MAX_SHAPE_LPC_ORDER+i] = int16(SAT16(RSHIFT_ROUND(AR1_Q24[i], 11)))
			psEncCtrl.AR2_Q13[k*MAX_SHAPE_LPC_ORDER+i] = int16(SAT16(RSHIFT_ROUND(AR2_Q24[i], 11)))
		}
	}

	// Gain tweaking
	// Increase gains during low speech activity and put lower limit on gains
	gain_mult_Q16 = log2lin(-SMLAWB(-int32(16.0*(1<<7)), SNR_adj_dB_Q7, int32(0.16*(1<<16))))
	gain_add_Q16 = log2lin(SMLAWB(int32(16.0*(1<<7)), int32(MIN_QGAIN_DB*(1<<7)), int32(0.16*(1<<16))))
	OpusAssert(gain_mult_Q16 > 0)
	for k = 0; k < psEnc.nb_subfr; k++ {
		psEncCtrl.Gains_Q16[k] = SMULWW(psEncCtrl.Gains_Q16[k], gain_mult_Q16)
		OpusAssert(psEncCtrl.Gains_Q16[k] >= 0)
		psEncCtrl.Gains_Q16[k] = ADD_POS_SAT32(psEncCtrl.Gains_Q16[k], gain_add_Q16)
	}

	gain_mult_Q16 = int32(1*(1<<16)) + RSHIFT_ROUND(MLA(int32(TuningParameters.INPUT_TILT*(1<<26)),
		psEncCtrl.coding_quality_Q14, int32(TuningParameters.HIGH_RATE_INPUT_TILT*(1<<12))), 10)
	for k = 0; k < psEnc.nb_subfr; k++ {
		psEncCtrl.GainsPre_Q14[k] = SMULWB(gain_mult_Q16, psEncCtrl.GainsPre_Q14[k])
	}

	// Control low-frequency shaping and noise tilt
	// Less low frequency shaping for noisy inputs
	strength_Q16 = MUL(int32(TuningParameters.LOW_FREQ_SHAPING*(1<<4)), SMLAWB(int32(1*(1<<12)),
		int32(TuningParameters.LOW_QUALITY_LOW_FREQ_SHAPING_DECR*(1<<13)), psEnc.input_quality_bands_Q15[0]-int32(1*(1<<15))))
	strength_Q16 = RSHIFT(MUL(strength_Q16, psEnc.speech_activity_Q8), 8)
	if psEnc.indices.signalType == TYPE_VOICED {
		// Reduce low frequencies quantization noise for periodic signals
		fs_kHz_inv := DIV32_16(int32(0.2*(1<<14)), psEnc.fs_kHz)
		for k = 0; k < psEnc.nb_subfr; k++ {
			b_Q14 = fs_kHz_inv + DIV32_16(int32(3.0*(1<<14)), psEncCtrl.pitchL[k])
			// Pack two coefficients in one int32
			psEncCtrl.LF_shp_Q14[k] = LSHIFT(int32(1*(1<<14))-b_Q14-SMULWB(strength_Q16, b_Q14), 16)
			psEncCtrl.LF_shp_Q14[k] |= (b_Q14 - int32(1*(1<<14)))) & 0xFFFF
		}
		OpusAssert(int32(TuningParameters.HARM_HP_NOISE_COEF*(1<<24)) < int32(0.5*(1<<24)))
		Tilt_Q16 = -int32(TuningParameters.HP_NOISE_COEF*(1<<16)) - SMULWB(int32(1*(1<<16))-int32(TuningParameters.HP_NOISE_COEF*(1<<16)),
			SMULWB(int32(TuningParameters.HARM_HP_NOISE_COEF*(1<<24)), psEnc.speech_activity_Q8))
	} else {
		b_Q14 = DIV32_16(21299, psEnc.fs_kHz)
		// Pack two coefficients in one int32
		psEncCtrl.LF_shp_Q14[0] = LSHIFT(int32(1*(1<<14))-b_Q14-
			SMULWB(strength_Q16, SMULWB(int32(0.6*(1<<16)), b_Q14)), 16)
		psEncCtrl.LF_shp_Q14[0] |= (b_Q14 - int32(1*(1<<14)))) & 0xFFFF
		for k = 1; k < psEnc.nb_subfr; k++ {
			psEncCtrl.LF_shp_Q14[k] = psEncCtrl.LF_shp_Q14[0]
		}
		Tilt_Q16 = -int32(TuningParameters.HP_NOISE_COEF*(1<<16))
	}

	// HARMONIC SHAPING CONTROL
	// Control boosting of harmonic frequencies
	HarmBoost_Q16 = SMULWB(SMULWB(int32(1*(1<<17))-LSHIFT(psEncCtrl.coding_quality_Q14, 3),
		psEnc.LTPCorr_Q15), int32(TuningParameters.LOW_RATE_HARMONIC_BOOST*(1<<16)))

	// More harmonic boost for noisy input signals
	HarmBoost_Q16 = SMLAWB(HarmBoost_Q16,
		int32(1*(1<<16))-LSHIFT(psEncCtrl.input_quality_Q14, 2), int32(TuningParameters.LOW_INPUT_QUALITY_HARMONIC_BOOST*(1<<16)))

	if USE_HARM_SHAPING != 0 && psEnc.indices.signalType == TYPE_VOICED {
		// More harmonic noise shaping for high bitrates or noisy input
		HarmShapeGain_Q16 = SMLAWB(int32(TuningParameters.HARMONIC_SHAPING*(1<<16)),
			int32(1*(1<<16))-SMULWB(int32(1*(1<<18))-LSHIFT(psEncCtrl.coding_quality_Q14, 4),
				psEncCtrl.input_quality_Q14), int32(TuningParameters.HIGH_RATE_OR_LOW_QUALITY_HARMONIC_SHAPING*(1<<16)))

		// Less harmonic noise shaping for less periodic signals
		HarmShapeGain_Q16 = SMULWB(LSHIFT(HarmShapeGain_Q16, 1),
			SQRT_APPROX(LSHIFT(psEnc.LTPCorr_Q15, 15)))
	} else {
		HarmShapeGain_Q16 = 0
	}

	// Smooth over subframes
	for k = 0; k < MAX_NB_SUBFR; k++ {
		psShapeSt.HarmBoost_smth_Q16 = SMLAWB(psShapeSt.HarmBoost_smth_Q16, 
			HarmBoost_Q16-psShapeSt.HarmBoost_smth_Q16, int32(TuningParameters.SUBFR_SMTH_COEF*(1<<16)))
		psShapeSt.HarmShapeGain_smth_Q16 = SMLAWB(psShapeSt.HarmShapeGain_smth_Q16,
			HarmShapeGain_Q16-psShapeSt.HarmShapeGain_smth_Q16, int32(TuningParameters.SUBFR_SMTH_COEF*(1<<16)))
		psShapeSt.Tilt_smth_Q16 = SMLAWB(psShapeSt.Tilt_smth_Q16,
			Tilt_Q16-psShapeSt.Tilt_smth_Q16, int32(TuningParameters.SUBFR_SMTH_COEF*(1<<16)))

		psEncCtrl.HarmBoost_Q14[k] = int32(RSHIFT_ROUND(psShapeSt.HarmBoost_smth_Q16, 2))
		psEncCtrl.HarmShapeGain_Q14[k] = int32(RSHIFT_ROUND(psShapeSt.HarmShapeGain_smth_Q16, 2))
		psEncCtrl.Tilt_Q14[k] = int32(RSHIFT_ROUND(psShapeSt.Tilt_smth_Q16, 2))
	}
}

