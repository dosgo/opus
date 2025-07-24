package opus
import (
	"math"
)

func silk_process_gains(
	psEnc *SilkChannelEncoder,
	psEncCtrl *SilkEncoderControl,
	condCoding int,
) {
	psShapeSt := psEnc.sShape
	var k int
	var s_Q16, InvMaxSqrVal_Q16, gain, gain_squared, ResNrg, ResNrgPart, quant_offset_Q10 int

	if psEnc.indices.signalType == TYPE_VOICED {
		s_Q16 = 0 - silk_sigm_Q15(Inlines.silk_RSHIFT_ROUND(psEncCtrl.LTPredCodGain_Q7-((12.0*128)+0.5), 4))
		for k = 0; k < psEnc.nb_subfr; k++ {
			psEncCtrl.Gains_Q16[k] = Inlines.silk_SMLAWB(psEncCtrl.Gains_Q16[k], psEncCtrl.Gains_Q16[k], s_Q16)
		}
	}

	InvMaxSqrVal_Q16 = Inlines.silk_DIV32_16(
		Inlines.silk_log2lin(
			Inlines.silk_SMULWB(
				((21+16/0.33)*128)+0.5-psEnc.SNR_dB_Q7,
				(0.33*65536)+0.5,
			),
		),
		psEnc.subfr_length,
	)

	for k = 0; k < psEnc.nb_subfr; k++ {
		ResNrg = psEncCtrl.ResNrg[k]
		ResNrgPart = Inlines.silk_SMULWW(ResNrg, InvMaxSqrVal_Q16)
		if psEncCtrl.ResNrgQ[k] > 0 {
			ResNrgPart = Inlines.silk_RSHIFT_ROUND(ResNrgPart, psEncCtrl.ResNrgQ[k])
		} else {
			if ResNrgPart >= Inlines.silk_RSHIFT(math.MaxInt32, -psEncCtrl.ResNrgQ[k]) {
				ResNrgPart = math.MaxInt32
			} else {
				ResNrgPart = Inlines.silk_LSHIFT(ResNrgPart, -psEncCtrl.ResNrgQ[k])
			}
		}
		gain = psEncCtrl.Gains_Q16[k]
		gain_squared = Inlines.silk_ADD_SAT32(ResNrgPart, Inlines.silk_SMMUL(gain, gain))
		if gain_squared < math.MaxInt16 {
			gain_squared = Inlines.silk_SMLAWW(Inlines.silk_LSHIFT(ResNrgPart, 16), gain, gain)
			if gain_squared <= 0 {
				panic("gain_squared should be positive")
			}
			gain = Inlines.silk_SQRT_APPROX(gain_squared)
			if gain > math.MaxInt32>>8 {
				gain = math.MaxInt32 >> 8
			}
			psEncCtrl.Gains_Q16[k] = Inlines.silk_LSHIFT_SAT32(gain, 8)
		} else {
			gain = Inlines.silk_SQRT_APPROX(gain_squared)
			if gain > math.MaxInt32>>16 {
				gain = math.MaxInt32 >> 16
			}
			psEncCtrl.Gains_Q16[k] = Inlines.silk_LSHIFT_SAT32(gain, 16)
		}
	}

	copy(psEncCtrl.GainsUnq_Q16[:], psEncCtrl.Gains_Q16[:psEnc.nb_subfr])
	psEncCtrl.lastGainIndexPrev = psShapeSt.LastGainIndex

	lastGainIndex := psShapeSt.LastGainIndex
	GainQuantization.silk_gains_quant(
		psEnc.indices.GainsIndices[:],
		psEncCtrl.Gains_Q16[:psEnc.nb_subfr],
		&lastGainIndex,
		boolToInt(condCoding == CODE_CONDITIONALLY),
		psEnc.nb_subfr,
	)
	psShapeSt.LastGainIndex = lastGainIndex

	if psEnc.indices.signalType == TYPE_VOICED {
		if psEncCtrl.LTPredCodGain_Q7+Inlines.silk_RSHIFT(psEnc.input_tilt_Q15, 8) > (1.0*128)+0.5 {
			psEnc.indices.quantOffsetType = 0
		} else {
			psEnc.indices.quantOffsetType = 1
		}
	}

	quant_offset_Q10 = SilkTables.Silk_Quantization_Offsets_Q10[psEnc.indices.signalType>>1][psEnc.indices.quantOffsetType]
	psEncCtrl.Lambda_Q10 = (TuningParameters.LAMBDA_OFFSET * 1024) +
		Inlines.silk_SMULBB((TuningParameters.LAMBDA_DELAYED_DECISIONS*1024), psEnc.nStatesDelayedDecision) +
		Inlines.silk_SMULWB((TuningParameters.LAMBDA_SPEECH_ACT*262144), psEnc.speech_activity_Q8) +
		Inlines.silk_SMULWB((TuningParameters.LAMBDA_INPUT_QUALITY*4096), psEncCtrl.input_quality_Q14) +
		Inlines.silk_SMULWB((TuningParameters.LAMBDA_CODING_QUALITY*4096), psEncCtrl.coding_quality_Q14) +
		Inlines.silk_SMULWB((TuningParameters.LAMBDA_QUANT_OFFSET*65536), quant_offset_Q10)

	if psEncCtrl.Lambda_Q10 <= 0 || psEncCtrl.Lambda_Q10 >= (2*1024) {
		panic("Lambda_Q10 out of range")
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}