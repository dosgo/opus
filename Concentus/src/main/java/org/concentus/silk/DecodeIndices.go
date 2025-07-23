# Go Translation of Java Silk Decoder Indices


package silk

// DecodeIndices handles the decoding of side-information parameters from the payload
// following the original Java implementation but adapted to Go idioms.

import (
	"errors"
)

// DecodeIndices decodes the signal type, quantizer offset, gains, LSF indices,
// pitch lags, LTP gains, and seed from the payload.
//
// Key translation decisions:
// 1. Used Go error handling instead of assertions
// 2. Converted Java's byte/short types to Go's uint8/int16 where appropriate
// 3. Replaced Java's array initialization with Go slices
// 4. Used Go constants instead of class constants
// 5. Simplified control flow where possible
// 6. Used Go naming conventions (camelCase instead of PascalCase for variables)
func DecodeIndices(
	psDec *ChannelDecoder,      // I/O State
	psRangeDec *EntropyCoder,   // I/O Compressor data structure
	frameIndex int,             // I Frame number
	decodeLBRR int,             // I Flag indicating LBRR data is being decoded
	condCoding int,             // I The type of conditional coding to use
) error {
	// Temporary buffers for NLSF decoding
	ecIx := make([]int16, psDec.LPC_order)
	predQ8 := make([]int16, psDec.LPC_order)

	// ****************************************
	// Decode signal type and quantizer offset
	// ****************************************
	var ix int
	if decodeLBRR != 0 || psDec.VAD_flags[frameIndex] != 0 {
		ix = psRangeDec.DecICDF(Tables.TypeOffsetVADICDF, 8) + 2
	} else {
		ix = psRangeDec.DecICDF(Tables.TypeOffsetNoVADICDF, 8)
	}
	psDec.Indices.SignalType = uint8(ix >> 1)
	psDec.Indices.QuantOffsetType = uint8(ix & 1)

	// *************
	// Decode gains
	// *************
	// First subframe
	if condCoding == CodeConditionally {
		// Conditional coding
		psDec.Indices.GainsIndices[0] = uint8(psRangeDec.DecICDF(Tables.DeltaGainICDF, 8))
	} else {
		// Independent coding, in two stages: MSB bits followed by 3 LSBs
		psDec.Indices.GainsIndices[0] = uint8(psRangeDec.DecICDF(Tables.GainICDF[psDec.Indices.SignalType], 8) << 3
		psDec.Indices.GainsIndices[0] += uint8(psRangeDec.DecICDF(Tables.Uniform8ICDF, 8))
	}

	// Remaining subframes
	for i := 1; i < psDec.NbSubfr; i++ {
		psDec.Indices.GainsIndices[i] = uint8(psRangeDec.DecICDF(Tables.DeltaGainICDF, 8))
	}

	// *******************
	// Decode LSF Indices
	// *******************
	nVectors := (psDec.Indices.SignalType >> 1) * psDec.PsNLSF_CB.NVectors
	psDec.Indices.NLSFIndices[0] = uint8(psRangeDec.DecICDF(psDec.PsNLSF_CB.CB1ICDF, nVectors, 8))
	
	if err := NLSFUnpack(ecIx, predQ8, psDec.PsNLSF_CB, psDec.Indices.NLSFIndices[0]); err != nil {
		return err
	}

	if psDec.PsNLSF_CB.Order != psDec.LPC_order {
		return errors.New("NLSF codebook order mismatch")
	}

	for i := 0; i < psDec.PsNLSF_CB.Order; i++ {
		ix = psRangeDec.DecICDF(psDec.PsNLSF_CB.ECICDF, int(ecIx[i]), 8)
		switch {
		case ix == 0:
			ix -= psRangeDec.DecICDF(Tables.NLSFExtICDF, 8)
		case ix == 2*NLSFQuantMaxAmplitude:
			ix += psRangeDec.DecICDF(Tables.NLSFExtICDF, 8)
		}
		psDec.Indices.NLSFIndices[i+1] = int8(ix - NLSFQuantMaxAmplitude)
	}

	// Decode LSF interpolation factor
	if psDec.NbSubfr == MaxNbSubfr {
		psDec.Indices.NLSFInterpCoefQ2 = uint8(psRangeDec.DecICDF(Tables.NLSFInterpolationFactorICDF, 8))
	} else {
		psDec.Indices.NLSFInterpCoefQ2 = 4
	}

	if psDec.Indices.SignalType == TypeVoiced {
		// ******************
		// Decode pitch lags
		// ******************
		decodeAbsoluteLagIndex := true
		if condCoding == CodeConditionally && psDec.EcPrevSignalType == TypeVoiced {
			// Decode Delta index
			deltaLagIndex := int16(psRangeDec.DecICDF(Tables.PitchDeltaICDF, 8))
			if deltaLagIndex > 0 {
				deltaLagIndex -= 9
				psDec.Indices.LagIndex = psDec.EcPrevLagIndex + deltaLagIndex
				decodeAbsoluteLagIndex = false
			}
		}

		if decodeAbsoluteLagIndex {
			// Absolute decoding
			psDec.Indices.LagIndex = int16(psRangeDec.DecICDF(Tables.PitchLagICDF, 8)) * int16(psDec.FsKHz>>1)
			psDec.Indices.LagIndex += int16(psRangeDec.DecICDF(psDec.PitchLagLowBitsICDF, 8))
		}
		psDec.EcPrevLagIndex = psDec.Indices.LagIndex

		// Get contour index
		psDec.Indices.ContourIndex = uint8(psRangeDec.DecICDF(psDec.PitchContourICDF, 8))

		// *****************
		// Decode LTP gains
		// *****************
		// Decode PERIndex value
		psDec.Indices.PERIndex = uint8(psRangeDec.DecICDF(Tables.LTPPerIndexICDF, 8))

		for k := 0; k < psDec.NbSubfr; k++ {
			psDec.Indices.LTPIndex[k] = uint8(psRangeDec.DecICDF(Tables.LTPGainICDFPtrs[psDec.Indices.PERIndex], 8))
		}

		// *******************
		// Decode LTP scaling
		// *******************
		if condCoding == CodeIndependently {
			psDec.Indices.LTPScaleIndex = uint8(psRangeDec.DecICDF(Tables.LTPScaleICDF, 8))
		} else {
			psDec.Indices.LTPScaleIndex = 0
		}
	}
	psDec.EcPrevSignalType = psDec.Indices.SignalType

	// ************
	// Decode seed
	// ************
	psDec.Indices.Seed = uint8(psRangeDec.DecICDF(Tables.Uniform4ICDF, 8))

	return nil
}
