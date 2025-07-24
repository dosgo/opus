package opus

func silk_find_pitch_lags(psEnc *SilkChannelEncoder, psEncCtrl *SilkEncoderControl, res []int16, x []int16, x_ptr int) {
	var buf_len, i, scale int
	var thrhld_Q13, res_nrg int
	var x_buf, x_buf_ptr int
	var Wsig []int16
	var Wsig_ptr int
	var auto_corr [MAX_FIND_PITCH_LPC_ORDER + 1]int32
	var rc_Q15 [MAX_FIND_PITCH_LPC_ORDER]int16
	var A_Q24 [MAX_FIND_PITCH_LPC_ORDER]int32
	var A_Q12 [MAX_FIND_PITCH_LPC_ORDER]int16

	buf_len = psEnc.la_pitch + psEnc.frame_length + psEnc.ltp_mem_length

	OpusAssert(buf_len >= psEnc.pitch_LPC_win_length)

	x_buf = x_ptr - psEnc.ltp_mem_length

	Wsig = make([]int16, psEnc.pitch_LPC_win_length)

	x_buf_ptr = x_buf + buf_len - psEnc.pitch_LPC_win_length
	Wsig_ptr = 0
	silk_apply_sine_window(Wsig, Wsig_ptr, x, x_buf_ptr, 1, psEnc.la_pitch)

	Wsig_ptr += psEnc.la_pitch
	x_buf_ptr += psEnc.la_pitch
	copy(Wsig[Wsig_ptr:], x[x_buf_ptr:x_buf_ptr+(psEnc.pitch_LPC_win_length-silk_LSHIFT(psEnc.la_pitch, 1))])

	Wsig_ptr += psEnc.pitch_LPC_win_length - silk_LSHIFT(psEnc.la_pitch, 1)
	x_buf_ptr += psEnc.pitch_LPC_win_length - silk_LSHIFT(psEnc.la_pitch, 1)
	silk_apply_sine_window(Wsig, Wsig_ptr, x, x_buf_ptr, 2, psEnc.la_pitch)

	boxed_scale := new(int)
	*boxed_scale = 0
	silk_autocorr(auto_corr[:], boxed_scale, Wsig, psEnc.pitch_LPC_win_length, psEnc.pitchEstimationLPCOrder+1)
	scale = *boxed_scale

	auto_corr[0] = Silk_SMLAWB(auto_corr[0], auto_corr[0], int32((TuningParameters.FIND_PITCH_WHITE_NOISE_FRACTION)*(1<<16)+0.5)) + 1

	res_nrg = silk_schur(rc_Q15[:], auto_corr[:], psEnc.pitchEstimationLPCOrder)

	if res_nrg < 1 {
		res_nrg = 1
	}
	psEncCtrl.predGain_Q16 = silk_DIV32_varQ(auto_corr[0], int32(res_nrg), 16)

	K2A.Silk_k2a(A_Q24[:], rc_Q15[:], psEnc.pitchEstimationLPCOrder)

	for i = 0; i < psEnc.pitchEstimationLPCOrder; i++ {
		A_Q12[i] = int16(silk_SAT16(int32(silk_RSHIFT_ROUND(A_Q24[i], 12))))
	}

	silk_bwexpander(A_Q12[:], psEnc.pitchEstimationLPCOrder, int32((TuningParameters.FIND_PITCH_BANDWIDTH_EXPANSION)*(1<<16)+0.5))

	silk_LPC_analysis_filter(res, 0, x, x_buf, A_Q12[:], 0, buf_len, psEnc.pitchEstimationLPCOrder)

	if psEnc.indices.signalType != SilkConstants.TYPE_NO_VOICE_ACTIVITY && psEnc.first_frame_after_reset == 0 {
		thrhld_Q13 = int((0.6 * (1 << 13)) + 0.5)
		thrhld_Q13 = silk_SMLABB(int32(thrhld_Q13), int32((-0.004*(1<<13))+0.5, int32(psEnc.pitchEstimationLPCOrder)))
		thrhld_Q13 = int(silk_SMLAWB(int32(thrhld_Q13), int32((-0.1*(1<<21))+0.5, int32(psEnc.speech_activity_Q8))))
		thrhld_Q13 = silk_SMLABB(int32(thrhld_Q13), int32((-0.15*(1<<13))+0.5, int32(Silk_RSHIFT(int32(psEnc.prevSignalType), 1))))
		thrhld_Q13 = int(silk_SMLAWB(int32(thrhld_Q13), int32((-0.1*(1<<14))+0.5, int32(psEnc.input_tilt_Q15))))
		thrhld_Q13 = int(silk_SAT16(int(thrhld_Q13)))

		lagIndex := psEnc.indices.lagIndex
		contourIndex := psEnc.indices.contourIndex
		LTPcorr_Q15 := psEnc.LTPCorr_Q15
		if silk_pitch_analysis_core(res, psEncCtrl.pitchL[:], &lagIndex, &contourIndex, &LTPcorr_Q15, psEnc.prevLag, psEnc.pitchEstimationThreshold_Q16, thrhld_Q13, psEnc.fs_kHz, psEnc.pitchEstimationComplexity, psEnc.nb_subfr) == 0 {
			psEnc.indices.signalType = SilkConstants.TYPE_VOICED
		} else {
			psEnc.indices.signalType = SilkConstants.TYPE_UNVOICED
		}

		psEnc.indices.lagIndex = lagIndex
		psEnc.indices.contourIndex = contourIndex
		psEnc.LTPCorr_Q15 = LTPcorr_Q15
	} else {
		for i := range psEncCtrl.pitchL {
			psEncCtrl.pitchL[i] = 0
		}
		psEnc.indices.lagIndex = 0
		psEnc.indices.contourIndex = 0
		psEnc.LTPCorr_Q15 = 0
	}
}
