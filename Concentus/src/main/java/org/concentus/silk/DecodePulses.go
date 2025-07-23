package silk

import (
	"errors"
)

// DecodePulses decodes quantization indices of excitation signal.
// This is a direct translation from Java to Go with idiomatic Go practices.
func DecodePulses(
	psRangeDec *EntropyCoder, // I/O Compressor data structure
	pulses []int16, // O Excitation signal
	signalType int, // I Signal type
	quantOffsetType int, // I Quantization offset type
	frameLength int, // I Frame length
) error {
	// Validate inputs
	if psRangeDec == nil || pulses == nil {
		return errors.New("nil input parameters")
	}
	if len(pulses) < frameLength {
		return errors.New("pulses slice too small for frame length")
	}

	// ******************
	// Decode rate level
	// ******************
	rateLevelIndex := psRangeDec.DecICDF(Tables.RateLevelsICDF[signalType>>1], 8)

	// Calculate number of shell blocks
	if 1<<Constants.Log2ShellCodecFrameLength != Constants.ShellCodecFrameLength {
		return errors.New("shell codec frame length constant mismatch")
	}

	iter := frameLength >> Constants.Log2ShellCodecFrameLength
	if iter*Constants.ShellCodecFrameLength < frameLength {
		if frameLength != 12*10 { // Make sure only happens for 10 ms @ 12 kHz
			return errors.New("unexpected frame length")
		}
		iter++
	}

	// ************************************************
	// Sum-Weighted-Pulses Decoding
	// ************************************************
	sumPulses := make([]int, Constants.MaxNbShellBlocks)
	nLShifts := make([]int, Constants.MaxNbShellBlocks)

	for i := 0; i < iter; i++ {
		nLShifts[i] = 0
		sumPulses[i] = psRangeDec.DecICDF(Tables.PulsesPerBlockICDF[rateLevelIndex], 8)

		// LSB indication
		for sumPulses[i] == Constants.SilkMaxPulses+1 {
			nLShifts[i]++
			// When we've already got 10 LSBs, we shift the table to not allow (SILK_MAX_PULSES + 1)
			tableIndex := Constants.NRateLevels - 1
			shiftFlag := 0
			if nLShifts[i] == 10 {
				shiftFlag = 1
			}
			sumPulses[i] = psRangeDec.DecICDF(Tables.PulsesPerBlockICDF[tableIndex], shiftFlag, 8)
		}
	}

	// ************************************************
	// Shell decoding
	// ************************************************
	for i := 0; i < iter; i++ {
		offset := i * Constants.ShellCodecFrameLength
		if sumPulses[i] > 0 {
			if err := ShellDecoder(pulses[offset:], psRangeDec, sumPulses[i]); err != nil {
				return err
			}
		} else {
			// Zero out the pulses for this block
			for j := 0; j < Constants.ShellCodecFrameLength; j++ {
				pulses[offset+j] = 0
			}
		}
	}

	// ************************************************
	// LSB Decoding
	// ************************************************
	for i := 0; i < iter; i++ {
		if nLShifts[i] > 0 {
			nLS := nLShifts[i]
			pulsesPtr := i * Constants.ShellCodecFrameLength
			for k := 0; k < Constants.ShellCodecFrameLength; k++ {
				absQ := int(pulses[pulsesPtr+k])
				for j := 0; j < nLS; j++ {
					absQ <<= 1
					absQ += psRangeDec.DecICDF(Tables.LsbICDF, 8)
				}
				pulses[pulsesPtr+k] = int16(absQ)
			}
			// Mark the number of pulses non-zero for sign decoding
			sumPulses[i] |= nLS << 5
		}
	}

	// *************************************
	// Decode and add signs to pulse signal
	// *************************************
	return DecodeSigns(psRangeDec, pulses, frameLength, signalType, quantOffsetType, sumPulses)
}
