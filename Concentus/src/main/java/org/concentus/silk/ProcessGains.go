package silk

import (
	"math"
)

// ProcessGains processes the gains for the SILK encoder
func ProcessGains(
	psEnc *ChannelEncoder, // I/O Encoder state
	psEncCtrl *EncoderControl, // I/O Encoder control
	condCoding int, // I The type of conditional coding to use
) {
	psShapeSt := psEnc.sShape
	var (
		s_Q16, InvMaxSqrVal_Q16, gain, gain_squared, ResNrg, ResNrgPart, quant_offset_Q10 int
	)

	// Gain reduction when LTP coding gain is high
	if psEnc.indices.signalType == TYPE_VOICED {
		// s = -0.5f * silk_sigmoid(0.25f * (psEncCtrl.LTPredCodGain - 12.0f))
		s_Q16 = 0 - SigmoidQ15(RSHIFT_ROUND(psEncCtrl.LTPredCodGain_Q7-SILK_CONST(12.0, 7), 4))
		for k := 0; k < psEnc.nb_subfr; k++ {
			psEncCtrl.Gains_Q16[k] = SMLAWB(psEncCtrl.Gains_Q16[k], psEncCtrl.Gains_Q16[k], s_Q16)
		}
	}

	// Limit the quantized signal
	// InvMaxSqrVal = pow(2.0f, 0.33f * (21.0f - SNR_dB)) / subfr_length
	InvMaxSqrVal_Q16 = DIV32_16(
		Log2lin(
			SMULWB(
				SILK_CONST(21+16/0.33, 7)-psEnc.SNR_dB_Q7,
				SILK_CONST(0.33, 16),
			),
		),
		psEnc.subfr_length,
	)

	for k := 0; k < psEnc.nb_subfr; k++ {
		// Soft limit on ratio residual energy and squared gains
		ResNrg = psEncCtrl.ResNrg[k]
		ResNrgPart = SMULWW(ResNrg, InvMaxSqrVal_Q16)
		if psEncCtrl.ResNrgQ[k] > 0 {
			ResNrgPart = RSHIFT_ROUND(ResNrgPart, psEncCtrl.ResNrgQ[k])
		} else if ResNrgPart >= RSHIFT(math.MaxInt32, -psEncCtrl.ResNrgQ[k]) {
			ResNrgPart = math.MaxInt32
		} else {
			ResNrgPart = LSHIFT(ResNrgPart, -psEncCtrl.ResNrgQ[k])
		}

		gain = psEncCtrl.Gains_Q16[k]
		gain_squared = ADD_SAT32(ResNrgPart, SMMUL(gain, gain))
		if gain_squared < math.MaxInt16 {
			// Recalculate with higher precision
			gain_squared = SMLAWB(LSHIFT(ResNrgPart, 16), gain, gain)
			OpusAssert(gain_squared > 0)
			gain = SQRT_APPROX(gain_squared) // Q8
			gain = min(gain, math.MaxInt32>>8)
			psEncCtrl.Gains_Q16[k] = LSHIFT_SAT32(gain, 8) // Q16
		} else {
			gain = SQRT_APPROX(gain_squared) // Q0
			gain = min(gain, math.MaxInt32>>16)
			psEncCtrl.Gains_Q16[k] = LSHIFT_SAT32(gain, 16) // Q16
		}
	}

	// Save unquantized gains and gain Index
	copy(psEncCtrl.GainsUnq_Q16[:], psEncCtrl.Gains_Q16[:psEnc.nb_subfr])
	psEncCtrl.lastGainIndexPrev = psShapeSt.LastGainIndex

	// Quantize gains
	var lastGainIndex byte = psShapeSt.LastGainIndex
	GainsQuant(
		psEnc.indices.GainsIndices[:],
		psEncCtrl.Gains_Q16[:psEnc.nb_subfr],
		&lastGainIndex,
		boolToInt(condCoding == CODE_CONDITIONALLY),
		psEnc.nb_subfr,
	)
	psShapeSt.LastGainIndex = lastGainIndex

	// Set quantizer offset for voiced signals. Larger offset when LTP coding gain is low or tilt is high (ie low-pass)
	if psEnc.indices.signalType == TYPE_VOICED {
		if psEncCtrl.LTPredCodGain_Q7+RSHIFT(psEnc.input_tilt_Q15, 8) > SILK_CONST(1.0, 7) {
			psEnc.indices.quantOffsetType = 0
		} else {
			psEnc.indices.quantOffsetType = 1
		}
	}

	// Quantizer boundary adjustment
	quant_offset_Q10 = QuantizationOffsetsQ10[psEnc.indices.signalType>>1][psEnc.indices.quantOffsetType]
	psEncCtrl.Lambda_Q10 = SILK_CONST(LAMBDA_OFFSET, 10) +
		SMULBB(SILK_CONST(LAMBDA_DELAYED_DECISIONS, 10), psEnc.nStatesDelayedDecision) +
		SMULWB(SILK_CONST(LAMBDA_SPEECH_ACT, 18), psEnc.speech_activity_Q8) +
		SMULWB(SILK_CONST(LAMBDA_INPUT_QUALITY, 12), psEncCtrl.input_quality_Q14) +
		SMULWB(SILK_CONST(LAMBDA_CODING_QUALITY, 12), psEncCtrl.coding_quality_Q14) +
		SMULWB(SILK_CONST(LAMBDA_QUANT_OFFSET, 16), quant_offset_Q10)

	OpusAssert(psEncCtrl.Lambda_Q10 > 0)
	OpusAssert(psEncCtrl.Lambda_Q10 < SILK_CONST(2, 10))
}

// Helper function to convert bool to int
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
