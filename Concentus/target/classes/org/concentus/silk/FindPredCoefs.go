package silk

import (
	"math"
)

// silk_find_pred_coefs calculates prediction coefficients for LPC analysis
func silk_find_pred_coefs(
	psEnc *SilkChannelEncoder, // I/O encoder state
	psEncCtrl *SilkEncoderControl, // I/O encoder control
	res_pitch []int16, // I residual from pitch analysis
	x []int16, // I speech signal
	x_ptr int, // I pointer to current position in x
	condCoding int, // I type of conditional coding to use
) {
	const (
		MAX_NB_SUBFR                          = MAX_NB_SUBFR
		MAX_LPC_ORDER                         = MAX_LPC_ORDER
		LTP_ORDER                             = LTP_ORDER
		TYPE_VOICED                           = TYPE_VOICED
		MAX_PREDICTION_POWER_GAIN_AFTER_RESET = MAX_PREDICTION_POWER_GAIN_AFTER_RESET
		MAX_PREDICTION_POWER_GAIN             = MAX_PREDICTION_POWER_GAIN
	)

	var (
		i, tmp, min_gain_Q16, minInvGain_Q30 int
		invGains_Q16                         [MAX_NB_SUBFR]int
		local_gains                          [MAX_NB_SUBFR]int
		Wght_Q15                             [MAX_NB_SUBFR]int
		NLSF_Q15                             [MAX_LPC_ORDER]int16
		x_ptr2, x_pre_ptr                    int
		LTP_corrs_rshift                     [MAX_NB_SUBFR]int
	)

	/* weighting for weighted least squares */
	min_gain_Q16 = math.MaxInt32 >> 6
	for i = 0; i < psEnc.nb_subfr; i++ {
		if psEncCtrl.Gains_Q16[i] < min_gain_Q16 {
			min_gain_Q16 = psEncCtrl.Gains_Q16[i]
		}
	}

	for i = 0; i < psEnc.nb_subfr; i++ {
		/* Divide to Q16 */
		// Assertion: Gains_Q16[i] > 0
		if psEncCtrl.Gains_Q16[i] <= 0 {
			panic("Gains_Q16[i] must be positive")
		}

		/* Invert and normalize gains, and ensure maximum invGains_Q16 is within 16-bit range */
		invGains_Q16[i] = silk_DIV32_varQ(min_gain_Q16, psEncCtrl.Gains_Q16[i], 16-2)

		/* Ensure Wght_Q15 a minimum value 1 */
		invGains_Q16[i] = max(invGains_Q16[i], 363)

		/* Square the inverted gains */
		// Assertion: invGains_Q16[i] == int16(invGains_Q16[i])
		if invGains_Q16[i] != int(int16(invGains_Q16[i])) {
			panic("invGains_Q16 overflow")
		}
		tmp = silk_SMULWB(invGains_Q16[i], invGains_Q16[i])
		Wght_Q15[i] = tmp >> 1

		/* Invert the inverted and normalized gains */
		local_gains[i] = silk_DIV32(1<<16, invGains_Q16[i])
	}

	LPC_in_pre := make([]int16, psEnc.nb_subfr*psEnc.predictLPCOrder+psEnc.frame_length)

	if psEnc.indices.signalType == TYPE_VOICED {
		/* VOICED */
		// Assertion: psEnc.ltp_mem_length-psEnc.predictLPCOrder >= psEncCtrl.pitchL[0]+LTP_ORDER/2
		if psEnc.ltp_mem_length-psEnc.predictLPCOrder < psEncCtrl.pitchL[0]+LTP_ORDER/2 {
			panic("invalid ltp_mem_length")
		}

		WLTP := make([]int, psEnc.nb_subfr*LTP_ORDER*LTP_ORDER)

		/* LTP analysis */
		codgain := psEncCtrl.LTPredCodGain_Q7
		silk_find_LTP(
			psEncCtrl.LTPCoef_Q14[:],
			WLTP,
			&codgain,
			res_pitch,
			psEncCtrl.pitchL[:],
			Wght_Q15[:],
			psEnc.subfr_length,
			psEnc.nb_subfr,
			psEnc.ltp_mem_length,
			LTP_corrs_rshift[:],
		)
		psEncCtrl.LTPredCodGain_Q7 = codgain

		/* Quantize LTP gain parameters */
		periodicity := psEnc.indices.PERIndex
		sum_log_gain := psEnc.sum_log_gain_Q7
		silk_quant_LTP_gains(
			psEncCtrl.LTPCoef_Q14[:],
			psEnc.indices.LTPIndex[:],
			&periodicity,
			&sum_log_gain,
			WLTP,
			psEnc.mu_LTP_Q9,
			psEnc.LTPQuantLowComplexity,
			psEnc.nb_subfr,
		)
		psEnc.indices.PERIndex = periodicity
		psEnc.sum_log_gain_Q7 = sum_log_gain

		/* Control LTP scaling */
		silk_LTP_scale_ctrl(psEnc, psEncCtrl, condCoding)

		/* Create LTP residual */
		silk_LTP_analysis_filter(
			LPC_in_pre,
			x,
			x_ptr-psEnc.predictLPCOrder,
			psEncCtrl.LTPCoef_Q14[:],
			psEncCtrl.pitchL[:],
			invGains_Q16[:],
			psEnc.subfr_length,
			psEnc.nb_subfr,
			psEnc.predictLPCOrder,
		)
	} else {
		/* UNVOICED */
		/* Create signal with prepended subframes, scaled by inverse gains */
		x_ptr2 = x_ptr - psEnc.predictLPCOrder
		x_pre_ptr = 0
		for i = 0; i < psEnc.nb_subfr; i++ {
			silk_scale_copy_vector16(
				LPC_in_pre,
				x_pre_ptr,
				x,
				x_ptr2,
				invGains_Q16[i],
				psEnc.subfr_length+psEnc.predictLPCOrder,
			)
			x_pre_ptr += psEnc.subfr_length + psEnc.predictLPCOrder
			x_ptr2 += psEnc.subfr_length
		}

		// Zero out LTP coefficients
		for i := range psEncCtrl.LTPCoef_Q14 {
			psEncCtrl.LTPCoef_Q14[i] = 0
		}
		psEncCtrl.LTPredCodGain_Q7 = 0
		psEnc.sum_log_gain_Q7 = 0
	}

	/* Limit on total predictive coding gain */
	if psEnc.first_frame_after_reset != 0 {
		minInvGain_Q30 = int((1.0 / MAX_PREDICTION_POWER_GAIN_AFTER_RESET) * (1 << 30))
	} else {
		// Q16 calculation
		minInvGain_Q30 = silk_log2lin(silk_SMLAWB(16<<7, psEncCtrl.LTPredCodGain_Q7, int((1.0/3.0)*(1<<16))))
		minInvGain_Q30 = silk_DIV32_varQ(
			minInvGain_Q30,
			silk_SMULWW(
				int(MAX_PREDICTION_POWER_GAIN*(1<<0)),
				silk_SMLAWB(
					int(0.25*(1<<18)),
					int(0.75*(1<<18)),
					psEncCtrl.coding_quality_Q14,
				),
			),
			14,
		)
	}

	/* LPC_in_pre contains the LTP-filtered input for voiced, and the unfiltered input for unvoiced */
	silk_find_LPC(psEnc, NLSF_Q15[:], LPC_in_pre, minInvGain_Q30)

	/* Quantize LSFs */
	silk_process_NLSFs(psEnc, psEncCtrl.PredCoef_Q12[:], NLSF_Q15[:], psEnc.prev_NLSFq_Q15[:])

	/* Calculate residual energy using quantized LPC coefficients */
	silk_residual_energy(
		psEncCtrl.ResNrg[:],
		psEncCtrl.ResNrgQ[:],
		LPC_in_pre,
		psEncCtrl.PredCoef_Q12[:],
		local_gains[:],
		psEnc.subfr_length,
		psEnc.nb_subfr,
		psEnc.predictLPCOrder,
	)

	/* Copy to prediction struct for use in next frame for interpolation */
	copy(psEnc.prev_NLSFq_Q15[:], NLSF_Q15[:])
}
