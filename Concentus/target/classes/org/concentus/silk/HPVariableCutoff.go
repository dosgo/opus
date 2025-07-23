package silk

import (
	"math"
)

// HPVariableCutoff provides high-pass filtering with adaptive cutoff frequency
// based on pitch lag statistics.

// HP_variable_cutoff implements a high-pass filter with cutoff frequency adaptation
// based on pitch lag statistics.
//
// state_Fxx: Encoder states (input/output)
func HP_variable_cutoff(state_Fxx []*ChannelEncoder) {
	// Retrieve primary channel encoder
	psEncC1 := state_Fxx[0]

	// Adaptive cutoff frequency: estimate low end of pitch frequency range
	if psEncC1.PrevSignalType == TypeVoiced {
		// Calculate pitch frequency in Q16 format
		pitch_freq_Hz_Q16 := (int32(psEncC1.Fs_kHz) * 1000 << 16) / int32(psEncC1.PrevLag)

		// Convert to logarithmic scale (Q7), adjusting for Q16 input
		pitch_freq_log_Q7 := lin2log(pitch_freq_Hz_Q16) - (16 << 7)

		// Adjustment based on quality
		quality_Q15 := psEncC1.Input_quality_bands_Q15[0]
		min_cutoff_log := lin2log(Variable_HP_MIN_CUTOFF_HZ<<16) - (16 << 7)
		pitch_freq_log_Q7 = smlawb(
			pitch_freq_log_Q7,
			smulwb(smulwb(-quality_Q15, quality_Q15<<2), pitch_freq_log_Q7-min_cutoff_log),
		)

		// delta_freq = pitch_freq_log - psEnc.variable_HP_smth1
		delta_freq_Q7 := pitch_freq_log_Q7 - int32(psEncC1.Variable_HP_smth1_Q15>>8)

		// Less smoothing for decreasing pitch frequency to track close to minimum
		if delta_freq_Q7 < 0 {
			delta_freq_Q7 *= 3
		}

		// Limit delta to reduce impact of outliers in pitch estimation
		delta_freq_Q7 = limit32(
			delta_freq_Q7,
			-Variable_HP_MAX_DELTA_FREQ<<7,
			Variable_HP_MAX_DELTA_FREQ<<7,
		)

		// Update smoother
		psEncC1.Variable_HP_smth1_Q15 = smlawb(
			psEncC1.Variable_HP_smth1_Q15,
			smulbb(int32(psEncC1.Speech_activity_Q8), delta_freq_Q7),
			Variable_HP_SMTH_COEF1<<16,
		)

		// Limit frequency range
		psEncC1.Variable_HP_smth1_Q15 = limit32(
			psEncC1.Variable_HP_smth1_Q15,
			lin2log(Variable_HP_MIN_CUTOFF_HZ)<<8,
			lin2log(Variable_HP_MAX_CUTOFF_HZ)<<8,
		)
	}
}

// Helper functions that would be defined elsewhere in the package:

// lin2log converts linear value to logarithmic scale (Q7 output)
func lin2log(lin int32) int32 {
	// Implementation depends on specific requirements
	// This would typically involve log2 calculation and scaling
	return int32(math.Log2(float64(lin)) * (1 << 7))
}

// smlawb implements signed multiply accumulate with Q15 and Q0 operands (a + b*c >> 16)
func smlawb(a, b, c int32) int32 {
	return a + (b*c)>>16
}

// smulwb implements signed multiply with Q15 and Q0 operands (a*b >> 16)
func smulwb(a, b int32) int32 {
	return (a * b) >> 16
}

// smulbb implements signed multiply with Q0 operands (a*b)
func smulbb(a, b int32) int32 {
	return a * b
}

// limit32 implements 32-bit value limiting
func limit32(value, min, max int32) int32 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
