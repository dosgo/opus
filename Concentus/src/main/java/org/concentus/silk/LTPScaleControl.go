package silk

// LTPScaleControl handles LTP (Long-Term Prediction) state scaling calculations.
// This is a direct translation from Java to Go with idiomatic Go practices applied.

// LTP_scale_ctrl calculates the LTP state scaling factor based on packet loss and coding conditions.
//
// Parameters:
//   - psEnc: Pointer to the encoder state (modified in place)
//   - psEncCtrl: Pointer to the encoder control (modified in place)
//   - condCoding: The type of conditional coding being used (CODE_INDEPENDENTLY or other)
//
// Translation Notes:
// 1. Go doesn't have classes, so we use receiver functions on struct types
// 2. Java's byte type becomes uint8 in Go
// 3. Constants are defined in the silk package
// 4. Inline functions are implemented as package-level functions
// 5. Go prefers explicit error handling, but since original code doesn't have any, we omit it
func (sc *LTPScaleControl) LTP_scale_ctrl(psEnc *SilkChannelEncoder, psEncCtrl *SilkEncoderControl, condCoding int) {
	var round_loss int

	if condCoding == CODE_INDEPENDENTLY {
		// Only scale if first frame in packet
		round_loss = psEnc.PacketLoss_perc + psEnc.nFramesPerPacket

		// The original Java code uses several nested operations:
		// 1. Multiply round_loss with LTPredCodGain_Q7 (SMULBB)
		// 2. Multiply result with 0.1 in Q9 format (SMULWB)
		// 3. Limit the result between 0 and 2
		// We implement this as direct arithmetic in Go
		ltpScale := LIMIT(
			SMULWB(
				SMULBB(round_loss, psEncCtrl.LTPredCodGain_Q7),
				int(0.1*(1<<9)+0.5), // Equivalent to SILK_CONST(0.1f, 9)
			),
			0,
			2,
		)
		psEnc.Indices.LTP_scaleIndex = uint8(ltpScale)
	} else {
		// Default is minimum scaling
		psEnc.Indices.LTP_scaleIndex = 0
	}

	// Set the scaling factor from the precomputed table
	psEncCtrl.LTP_scale_Q14 = LTPScales_table_Q14[psEnc.Indices.LTP_scaleIndex]
}

// SMULBB implements signed multiply of two 16-bit values (from original Java code)
func SMULBB(a, b int) int {
	return a * b
}

// SMULWB implements signed multiply of a 32-bit value with a 16-bit value, with rounding (from original Java code)
func SMULWB(a32, b16 int) int {
	return (a32*b16 + (1 << 14)) >> 15
}

// LIMIT bounds a value between min and max (inclusive)
func LIMIT(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
