package opus

func silk_find_pred_coefs(
	psEnc *SilkChannelEncoder,
	psEncCtrl *SilkEncoderControl,
	res_pitch []int16,
	x []int16,
	x_ptr int,
	condCoding int,
) {
	var i int
	invGains_Q16 := make([]int, MAX_NB_SUBFR)
	local_gains := make([]int, MAX_NB_SUBFR)
	Wght_Q15 := make([]int, MAX_NB_SUBFR)
	NLSF_Q15 := make([]int, MAX_LPC_ORDER)
	var x_ptr2, x_pre_ptr int
	var LPC_in_pre []int16
	var tmp, min_gain_Q16, minInvGain_Q30 int
	LTP_corrs_rshift := make([]int, MAX_NB_SUBFR)

	min_gain_Q16 = int(0x7FFFFFFF >> 6)
	for i = 0; i < psEnc.nb_subfr; i++ {
		if psEncCtrl.Gains_Q16[i] < min_gain_Q16 {
			min_gain_Q16 = psEncCtrl.Gains_Q16[i]
		}
	}
	for i = 0; i < psEnc.nb_subfr; i++ {
		OpusAssert(psEncCtrl.Gains_Q16[i] > 0)
		invGains_Q16[i] = silk_DIV32_varQ(min_gain_Q16, psEncCtrl.Gains_Q16[i], 16-2)

		if invGains_Q16[i] < 363 {
			invGains_Q16[i] = 363
		}

		tmp = silk_SMULWB(invGains_Q16[i], invGains_Q16[i])
		Wght_Q15[i] = tmp >> 1

		local_gains[i] = silk_DIV32(1<<16, invGains_Q16[i])
	}

	LPC_in_pre = make([]int16, psEnc.nb_subfr*psEnc.predictLPCOrder+psEnc.frame_length)
	if psEnc.indices.signalType == TYPE_VOICED {
		var WLTP []int

		OpusAssert(psEnc.ltp_mem_length-psEnc.predictLPCOrder >= psEncCtrl.pitchL[0]+LTP_ORDER/2)

		WLTP = make([]int, psEnc.nb_subfr*LTP_ORDER*LTP_ORDER)

		codgain := psEncCtrl.LTPredCodGain_Q7
		silk_find_LTP(psEncCtrl.LTPCoef_Q14, WLTP, &codgain, res_pitch, psEncCtrl.pitchL, Wght_Q15, psEnc.subfr_length, psEnc.nb_subfr, psEnc.ltp_mem_length, LTP_corrs_rshift)
		psEncCtrl.LTPredCodGain_Q7 = codgain

		periodicity := psEnc.indices.PERIndex
		sum_log_gain := psEnc.sum_log_gain_Q7
		silk_quant_LTP_gains(psEncCtrl.LTPCoef_Q14, psEnc.indices.LTPIndex, &periodicity, &sum_log_gain, WLTP, psEnc.mu_LTP_Q9, psEnc.LTPQuantLowComplexity, psEnc.nb_subfr)
		psEnc.indices.PERIndex = periodicity
		psEnc.sum_log_gain_Q7 = sum_log_gain

		silk_LTP_scale_ctrl(psEnc, psEncCtrl, condCoding)

		silk_LTP_analysis_filter(LPC_in_pre, x, x_ptr-psEnc.predictLPCOrder, psEncCtrl.LTPCoef_Q14, psEncCtrl.pitchL, invGains_Q16, psEnc.subfr_length, psEnc.nb_subfr, psEnc.predictLPCOrder)

	} else {
		x_ptr2 = x_ptr - psEnc.predictLPCOrder
		x_pre_ptr = 0
		for i = 0; i < psEnc.nb_subfr; i++ {
			silk_scale_copy_vector16(LPC_in_pre, x_pre_ptr, x, x_ptr2, invGains_Q16[i], psEnc.subfr_length+psEnc.predictLPCOrder)
			x_pre_ptr += psEnc.subfr_length + psEnc.predictLPCOrder
			x_ptr2 += psEnc.subfr_length
		}

		for i := 0; i < psEnc.nb_subfr*LTP_ORDER; i++ {
			psEncCtrl.LTPCoef_Q14[i] = 0
		}
		psEncCtrl.LTPredCodGain_Q7 = 0
		psEnc.sum_log_gain_Q7 = 0
	}

	if psEnc.first_frame_after_reset != 0 {
		minInvGain_Q30 = int((1.0/SilkConstants.MAX_PREDICTION_POWER_GAIN_AFTER_RESET)*float64(1<<30) + 0.5)
	} else {
		minInvGain_Q30 = silk_log2lin(silk_SMLAWB(16<<7, int(psEncCtrl.LTPredCodGain_Q7), int((1.0/3.0)*(1<<16)+0.5)))
		minInvGain_Q30 = silk_DIV32_varQ(minInvGain_Q30, silk_SMULWW(int(SilkConstants.MAX_PREDICTION_POWER_GAIN*(1)+0.5, silk_SMLAWB(int(0.25*(1<<18)+0.5, int(0.75*(1<<18)+0.5, psEncCtrl.coding_quality_Q14)), 14))))
	}

	silk_find_LPC(psEnc, NLSF_Q15, LPC_in_pre, minInvGain_Q30)

	silk_process_NLSFs(psEnc, psEncCtrl.PredCoef_Q12, NLSF_Q15, psEnc.prev_NLSFq_Q15[:])

	silk_residual_energy(psEncCtrl.ResNrg[:], psEncCtrl.ResNrgQ[:], LPC_in_pre, psEncCtrl.PredCoef_Q12, local_gains, psEnc.subfr_length, psEnc.nb_subfr, psEnc.predictLPCOrder)

	copy(psEnc.prev_NLSFq_Q15[:], NLSF_Q15[:MAX_LPC_ORDER])
}
