package opus

func silk_process_gains(
	psEnc *SilkChannelEncoder,
	psEncCtrl *SilkEncoderControl,
	condCoding int,
) {

	psShapeSt := psEnc.sShape
	var k int
	var s_Q16, InvMaxSqrVal_Q16, gain, gain_squared, ResNrg, ResNrgPart, quant_offset_Q10 int

	/* Gain reduction when LTP coding gain is high */
	if psEnc.indices.signalType == TYPE_VOICED {
		/*s = -0.5f * silk_sigmoid( 0.25f * ( psEncCtrl.LTPredCodGain - 12.0f ) ); */
		s_Q16 = 0 - silk_sigm_Q15(silk_RSHIFT_ROUND(psEncCtrl.LTPredCodGain_Q7-((12.0)*(int64(1)<<(7))+0.5), 4))
		for k = 0; k < psEnc.nb_subfr; k++ {
			psEncCtrl.Gains_Q16[k] = silk_SMLAWB(psEncCtrl.Gains_Q16[k], psEncCtrl.Gains_Q16[k], s_Q16)
		}
	}

	/* Limit the quantized signal */
	/* InvMaxSqrVal = pow( 2.0f, 0.33f * ( 21.0f - SNR_dB ) ) / subfr_length; */
	InvMaxSqrVal_Q16 = silk_DIV32_16(silk_log2lin(
		silk_SMULWB(((int)((21+16/0.33)*(int64(1)<<(7))+0.5))-psEnc.SNR_dB_Q7, ((0.33)*(int64(1)<<(16))+0.5))), psEnc.subfr_length)

	for k = 0; k < psEnc.nb_subfr; k++ {
		/* Soft limit on ratio residual energy and squared gains */
		ResNrg = psEncCtrl.ResNrg[k]
		ResNrgPart = Inlines.silk_SMULWW(ResNrg, InvMaxSqrVal_Q16)
		if psEncCtrl.ResNrgQ[k] > 0 {
			ResNrgPart = Inlines.silk_RSHIFT_ROUND(ResNrgPart, psEncCtrl.ResNrgQ[k])
		} else if ResNrgPart >= Inlines.silk_RSHIFT(Integer.MAX_VALUE, -psEncCtrl.ResNrgQ[k]) {
			ResNrgPart = Integer.MAX_VALUE
		} else {
			ResNrgPart = Inlines.silk_LSHIFT(ResNrgPart, -psEncCtrl.ResNrgQ[k])
		}
		gain = psEncCtrl.Gains_Q16[k]
		gain_squared = Inlines.silk_ADD_SAT32(ResNrgPart, Inlines.silk_SMMUL(gain, gain))
		if gain_squared < Short.MAX_VALUE {
			/* recalculate with higher precision */
			gain_squared = Inlines.silk_SMLAWW(Inlines.silk_LSHIFT(ResNrgPart, 16), gain, gain)
			Inlines.OpusAssert(gain_squared > 0)
			gain = Inlines.silk_SQRT_APPROX(gain_squared)
			/* Q8   */
			gain = Inlines.silk_min(gain, Integer.MAX_VALUE>>8)
			psEncCtrl.Gains_Q16[k] = Inlines.silk_LSHIFT_SAT32(gain, 8)
			/* Q16  */
		} else {
			gain = Inlines.silk_SQRT_APPROX(gain_squared)
			/* Q0   */
			gain = Inlines.silk_min(gain, Integer.MAX_VALUE>>16)
			psEncCtrl.Gains_Q16[k] = Inlines.silk_LSHIFT_SAT32(gain, 16)
			/* Q16  */
		}

	}

	/* Save unquantized gains and gain Index */
	System.arraycopy(psEncCtrl.Gains_Q16, 0, psEncCtrl.GainsUnq_Q16, 0, psEnc.nb_subfr)
	psEncCtrl.lastGainIndexPrev = psShapeSt.LastGainIndex

	/* Quantize gains */
	boxed_lastGainIndex = &BoxedValueByte{psShapeSt.LastGainIndex}
	GainQuantization.silk_gains_quant(psEnc.indices.GainsIndices, psEncCtrl.Gains_Q16,
		boxed_lastGainIndex, boolToInt(condCoding == SilkConstants.CODE_CONDITIONALLY), psEnc.nb_subfr)
	psShapeSt.LastGainIndex = boxed_lastGainIndex.Val

	/* Set quantizer offset for voiced signals. Larger offset when LTP coding gain is low or tilt is high (ie low-pass) */
	if psEnc.indices.signalType == SilkConstants.TYPE_VOICED {
		if psEncCtrl.LTPredCodGain_Q7+Inlines.silk_RSHIFT(psEnc.input_tilt_Q15, 8) > ((int)((1.0)*(int64(1)<<(7)) + 0.5)) {
			psEnc.indices.quantOffsetType = 0
		} else {
			psEnc.indices.quantOffsetType = 1
		}
	}

	/* Quantizer boundary adjustment */
	quant_offset_Q10 = SilkTables.silk_Quantization_Offsets_Q10[psEnc.indices.signalType>>1][psEnc.indices.quantOffsetType]
	psEncCtrl.Lambda_Q10 = ((int)((TuningParameters.LAMBDA_OFFSET)*(int64(1)<<(10)) + 0.5)) +
		silk_SMULBB((int((TuningParameters.LAMBDA_DELAYED_DECISIONS)*(int64(1)<<(10))+0.5)), psEnc.nStatesDelayedDecision)
	+silk_SMULWB(((int)((TuningParameters.LAMBDA_SPEECH_ACT)*(int64(1)<<(18)) + 0.5)), psEnc.speech_activity_Q8)
	+silk_SMULWB(((int)((TuningParameters.LAMBDA_INPUT_QUALITY)*(int64(1)<<(12)) + 0.5)), psEncCtrl.input_quality_Q14)
	+silk_SMULWB(((int)((TuningParameters.LAMBDA_CODING_QUALITY)*(int64(1)<<(12)) + 0.5)), psEncCtrl.coding_quality_Q14)
	+silk_SMULWB(((int)((TuningParameters.LAMBDA_QUANT_OFFSET)*(int64(1)<<(16)) + 0.5)), quant_offset_Q10)

	OpusAssert(psEncCtrl.Lambda_Q10 > 0)
	OpusAssert(psEncCtrl.Lambda_Q10 < (int(2*float64(int64(1)<<(10)) + 0.5)))

}
