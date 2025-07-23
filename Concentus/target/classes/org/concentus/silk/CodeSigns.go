package silk

// CodeSigns handles encoding and decoding of excitation signs in SILK codec.
type CodeSigns struct{}

// encMap converts a pulse value to a sign representation for encoding.
func (cs *CodeSigns) encMap(a int) int {
	return (a >> 15) + 1 // Equivalent to silk_RSHIFT(a, 15) + 1
}

// decMap converts an encoded sign back to a pulse multiplier (-1 or +1).
func (cs *CodeSigns) decMap(a int) int {
	return (a << 1) - 1 // Equivalent to silk_LSHIFT(a, 1) - 1
}

// EncodeSigns encodes signs of excitation pulses.
//
// Parameters:
//   - psRangeEnc: Entropy coder (compressor data structure)
//   - pulses: Pulse signal to encode
//   - length: Length of input
//   - signalType: Signal type
//   - quantOffsetType: Quantization offset type
//   - sumPulses: Sum of absolute pulses per block [MAX_NB_SHELL_BLOCKS]
func (cs *CodeSigns) EncodeSigns(
	psRangeEnc EntropyCoder,
	pulses []byte,
	length int,
	signalType int,
	quantOffsetType int,
	sumPulses []int,
) {
	var icdf [2]int16
	var qPtr int
	signICDF := SilkTables.SignICDF

	// Calculate initial icdf pointer
	i := SMulBB(7, AddLSHIFT(quantOffsetType, signalType, 1))
	icdfPtr := i
	length = (length + (SHELL_CODEC_FRAME_LENGTH / 2)) >> LOG2_SHELL_CODEC_FRAME_LENGTH

	for i := 0; i < length; i++ {
		p := sumPulses[i]
		if p > 0 {
			// Get the appropriate ICDF value based on pulse count
			icdf[0] = signICDF[icdfPtr+min(p&0x1F, 6)]

			// Process each pulse in the current frame
			for j := qPtr; j < qPtr+SHELL_CODEC_FRAME_LENGTH; j++ {
				if pulses[j] != 0 {
					psRangeEnc.EncICDF(cs.encMap(int(pulses[j])), icdf[:], 8)
				}
			}
		}
		qPtr += SHELL_CODEC_FRAME_LENGTH
	}
}

// DecodeSigns decodes signs of excitation pulses.
//
// Parameters:
//   - psRangeDec: Entropy decoder (compressor data structure)
//   - pulses: Pulse signal to decode (modified in-place)
//   - length: Length of input
//   - signalType: Signal type
//   - quantOffsetType: Quantization offset type
//   - sumPulses: Sum of absolute pulses per block [MAX_NB_SHELL_BLOCKS]
func (cs *CodeSigns) DecodeSigns(
	psRangeDec EntropyCoder,
	pulses []int16,
	length int,
	signalType int,
	quantOffsetType int,
	sumPulses []int,
) {
	var icdf [2]int16
	var qPtr int
	icdfTable := SilkTables.SignICDF

	// Calculate initial icdf pointer
	i := SMulBB(7, AddLSHIFT(quantOffsetType, signalType, 1))
	icdfPtr := i
	length = (length + SHELL_CODEC_FRAME_LENGTH/2) >> LOG2_SHELL_CODEC_FRAME_LENGTH

	for i := 0; i < length; i++ {
		p := sumPulses[i]
		if p > 0 {
			// Get the appropriate ICDF value based on pulse count
			icdf[0] = icdfTable[icdfPtr+min(p&0x1F, 6)]

			// Process each pulse in the current frame
			for j := 0; j < SHELL_CODEC_FRAME_LENGTH; j++ {
				if pulses[qPtr+j] > 0 {
					// Attach decoded sign to the pulse
					pulses[qPtr+j] *= int16(cs.decMap(psRangeDec.DecICDF(icdf[:], 8)))
				}
			}
		}
		qPtr += SHELL_CODEC_FRAME_LENGTH
	}
}
