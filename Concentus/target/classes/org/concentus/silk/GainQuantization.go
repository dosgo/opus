package silk

// Note: Constants like MIN_QGAIN_DB, N_LEVELS_QGAIN, etc. should be defined in a constants package

// GainQuantization handles quantization and dequantization of gain values
type GainQuantization struct{}

// Constants for gain quantization
var (
	offset      = ((MIN_QGAIN_DB*128)/6 + 16*128)
	scaleQ16    = ((65536 * (N_LEVELS_QGAIN - 1)) / (((MAX_QGAIN_DB - MIN_QGAIN_DB) * 128) / 6))
	invScaleQ16 = ((65536 * (((MAX_QGAIN_DB - MIN_QGAIN_DB) * 128) / 6)) / (N_LEVELS_QGAIN - 1))
)

// GainsQuant quantizes gain values with hysteresis, uniform on log scale
//
// Parameters:
//   - ind: output gain indices (slice of bytes)
//   - gainQ16: input/output gains (quantized out) (slice of int32)
//   - prevInd: pointer to last index in previous frame (byte)
//   - conditional: first gain is delta coded if 1 (bool)
//   - nbSubfr: number of subframes (int)
//
// Key translation decisions:
// 1. Changed BoxedValueByte to *byte pointer for more idiomatic Go
// 2. Used slices instead of fixed-size arrays for more flexibility
// 3. Made conditional parameter a bool for better semantics
// 4. Used int32 consistently for Q16 fixed-point values
// 5. Inlined utility functions directly for performance
func (g *GainQuantization) GainsQuant(ind []byte, gainQ16 []int32, prevInd *byte, conditional bool, nbSubfr int) {
	for k := 0; k < nbSubfr; k++ {
		// Convert to log scale, scale, floor()
		ind[k] = byte(SMULWB(scaleQ16, lin2log(gainQ16[k])-offset))

		// Round towards previous quantized gain (hysteresis)
		if ind[k] < *prevInd {
			ind[k]++
		}

		ind[k] = byte(LIMITInt(int(ind[k]), 0, N_LEVELS_QGAIN-1))

		// Compute delta indices and limit
		if k == 0 && !conditional {
			// Full index
			ind[k] = byte(LIMITInt(int(ind[k]), int(*prevInd)+MIN_DELTA_GAIN_QUANT, N_LEVELS_QGAIN-1))
			*prevInd = ind[k]
		} else {
			// Delta index
			ind[k] = ind[k] - *prevInd

			// Double the quantization step size for large gain increases
			doubleStepSizeThreshold := 2*MAX_DELTA_GAIN_QUANT - N_LEVELS_QGAIN + int(*prevInd)
			if int(ind[k]) > doubleStepSizeThreshold {
				ind[k] = byte(doubleStepSizeThreshold + ((int(ind[k]) - doubleStepSizeThreshold + 1) >> 1))
			}

			ind[k] = byte(LIMITInt(int(ind[k]), MIN_DELTA_GAIN_QUANT, MAX_DELTA_GAIN_QUANT))

			// Accumulate deltas
			if int(ind[k]) > doubleStepSizeThreshold {
				*prevInd = byte(int(*prevInd) + (int(ind[k])<<1 - doubleStepSizeThreshold))
			} else {
				*prevInd = byte(int(*prevInd) + int(ind[k]))
			}

			// Shift to make non-negative
			ind[k] -= MIN_DELTA_GAIN_QUANT
		}

		// Scale and convert to linear scale
		gainQ16[k] = log2lin(min32(SMULWB(invScaleQ16, int32(*prevInd))+offset, 3967))
	}
}

// GainsDequant dequantizes gain values, uniform on log scale
//
// Parameters:
//   - gainQ16: output quantized gains (slice of int32)
//   - ind: input gain indices (slice of bytes)
//   - prevInd: pointer to last index in previous frame (byte)
//   - conditional: first gain is delta coded if 1 (bool)
//   - nbSubfr: number of subframes (int)
//
// Key translation decisions:
// 1. Consistent naming with GainsQuant
// 2. Used pointer semantics for prevInd for consistency
// 3. Simplified conditional logic with bool
func (g *GainQuantization) GainsDequant(gainQ16 []int32, ind []byte, prevInd *byte, conditional bool, nbSubfr int) {
	for k := 0; k < nbSubfr; k++ {
		if k == 0 && !conditional {
			// Gain index is not allowed to go down more than 16 steps (~21.8 dB)
			*prevInd = byte(maxInt(int(ind[k]), int(*prevInd)-16))
		} else {
			// Delta index
			indTmp := int(ind[k]) + MIN_DELTA_GAIN_QUANT

			// Accumulate deltas
			doubleStepSizeThreshold := 2*MAX_DELTA_GAIN_QUANT - N_LEVELS_QGAIN + int(*prevInd)
			if indTmp > doubleStepSizeThreshold {
				*prevInd = byte(int(*prevInd) + (indTmp<<1 - doubleStepSizeThreshold))
			} else {
				*prevInd = byte(int(*prevInd) + indTmp)
			}
		}

		*prevInd = byte(LIMITInt(int(*prevInd), 0, N_LEVELS_QGAIN-1))

		// Scale and convert to linear scale
		gainQ16[k] = log2lin(min32(SMULWB(invScaleQ16, int32(*prevInd))+offset, 3967))
	}
}

// GainsID computes unique identifier of gain indices vector
//
// Parameters:
//   - ind: input gain indices (slice of bytes)
//   - nbSubfr: number of subframes (int)
//
// # Returns unique identifier of gains as int32
//
// Key translation decisions:
// 1. Simplified the bit shifting operation
// 2. Used more descriptive variable name
func (g *GainQuantization) GainsID(ind []byte, nbSubfr int) int32 {
	gainsID := int32(0)
	for k := 0; k < nbSubfr; k++ {
		gainsID = (gainsID << 8) | int32(ind[k])
	}
	return gainsID
}
