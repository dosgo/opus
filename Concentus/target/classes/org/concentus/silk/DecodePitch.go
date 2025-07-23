// Package silk implements SILK audio codec functionality.
package silk

// DecodePitch decodes pitch lags from the given indices.
//
// Parameters:
//
//	lagIndex:      Input lag index
//	contourIndex:  Output contour index
//	pitchLags:     Output array for 4 pitch values (will be filled according to nbSubfr)
//	fsKHz:         Sampling frequency in kHz
//	nbSubfr:       Number of sub frames
func DecodePitch(
	lagIndex int16, // I: Lag index
	contourIndex uint8, // O: Contour index
	pitchLags []int32, // O: 4 pitch values (slice must be pre-allocated)
	fsKHz int32, // I: Sampling frequency (kHz)
	nbSubfr int32, // I: Number of sub frames
) {
	var lag, minLag, maxLag int32
	var lagCBPtr [][]uint8

	// Select appropriate codebook based on sampling frequency and number of subframes
	switch {
	case fsKHz == 8:
		if nbSubfr == PE_MAX_NB_SUBFR {
			lagCBPtr = CB_lags_stage2
		} else {
			// Assert nbSubfr == PE_MAX_NB_SUBFR/2
			if nbSubfr != PE_MAX_NB_SUBFR/2 {
				panic("invalid number of subframes for 8kHz")
			}
			lagCBPtr = CB_lags_stage2_10_ms
		}
	case nbSubfr == PE_MAX_NB_SUBFR:
		lagCBPtr = CB_lags_stage3
	default:
		// Assert nbSubfr == PE_MAX_NB_SUBFR/2
		if nbSubfr != PE_MAX_NB_SUBFR/2 {
			panic("invalid number of subframes")
		}
		lagCBPtr = CB_lags_stage3_10_ms
	}

	// Calculate minimum and maximum lag values
	minLag = SMULBB(PE_MIN_LAG_MS, fsKHz)
	maxLag = SMULBB(PE_MAX_LAG_MS, fsKHz)
	lag = minLag + int32(lagIndex)

	// Calculate pitch lags for each subframe
	for k := int32(0); k < nbSubfr; k++ {
		pitchLags[k] = lag + int32(lagCBPtr[k][contourIndex])
		pitchLags[k] = LIMIT(pitchLags[k], minLag, maxLag)
	}
}
