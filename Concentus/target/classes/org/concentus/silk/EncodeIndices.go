package silk

/*
Copyright (c) 2006-2011 Skype Limited. All Rights Reserved
Ported to Go from Java by [Your Name]

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions
are met:

- Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.

- Redistributions in binary form must reproduce the above copyright
notice, this list of conditions and the following disclaimer in the
documentation and/or other materials provided with the distribution.

- Neither the name of Internet Society, IETF or IETF Trust, nor the
names of specific contributors, may be used to endorse or promote
products derived from this software without specific prior written
permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
``AS IS'' AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER
OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL,
EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF
LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

// EncodeIndices encodes side-information parameters to payload
func EncodeIndices(
	psEncC *ChannelEncoder,
	psRangeEnc *EntropyCoder,
	frameIndex int,
	encodeLBRR int,
	condCoding int,
) {
	// Translation notes:
	// 1. Go uses camelCase instead of camelCase for variable names
	// 2. Go doesn't have classes, so we use struct methods
	// 3. Go has stricter type checking, so we ensure proper types
	// 4. Go slices replace Java arrays for more flexible memory management
	// 5. Error handling in Go is explicit, but we maintain assertions as in original

	var (
		i, k, typeOffset       int
		encodeAbsoluteLagIndex int
		deltaLagIndex          int
		ecIx                   = make([]int16, MAX_LPC_ORDER)
		predQ8                 = make([]int16, MAX_LPC_ORDER)
		psIndices              *SideInfoIndices
	)

	// Select appropriate indices based on LBRR flag
	if encodeLBRR != 0 {
		psIndices = &psEncC.IndicesLBRR[frameIndex]
	} else {
		psIndices = &psEncC.Indices
	}

	/*****************************************/
	/* Encode signal type and quantizer offset */
	/*****************************************/
	typeOffset = 2*psIndices.SignalType + psIndices.QuantOffsetType
	opusAssert(typeOffset >= 0 && typeOffset < 6, "typeOffset out of range")
	opusAssert(encodeLBRR == 0 || typeOffset >= 2, "invalid typeOffset for LBRR")

	if encodeLBRR != 0 || typeOffset >= 2 {
		psRangeEnc.EncIcdf(typeOffset-2, silkTypeOffsetVADIcdf[:], 8)
	} else {
		psRangeEnc.EncIcdf(typeOffset, silkTypeOffsetNoVADIcdf[:], 8)
	}

	/*************/
	/* Encode gains */
	/*************/
	// First subframe
	if condCoding == CODE_CONDITIONALLY {
		// Conditional coding
		opusAssert(psIndices.GainsIndices[0] >= 0 &&
			psIndices.GainsIndices[0] < MAX_DELTA_GAIN_QUANT-MIN_DELTA_GAIN_QUANT+1,
			"invalid GainsIndices[0]")
		psRangeEnc.EncIcdf(psIndices.GainsIndices[0], silkDeltaGainIcdf[:], 8)
	} else {
		// Independent coding, in two stages: MSB bits followed by 3 LSBs
		opusAssert(psIndices.GainsIndices[0] >= 0 &&
			psIndices.GainsIndices[0] < N_LEVELS_QGAIN,
			"invalid GainsIndices[0]")
		psRangeEnc.EncIcdf(psIndices.GainsIndices[0]>>3,
			silkGainIcdf[psIndices.SignalType][:], 8)
		psRangeEnc.EncIcdf(psIndices.GainsIndices[0]&7, silkUniform8Icdf[:], 8)
	}

	// Remaining subframes
	for i := 1; i < psEncC.NbSubfr; i++ {
		opusAssert(psIndices.GainsIndices[i] >= 0 &&
			psIndices.GainsIndices[i] < MAX_DELTA_GAIN_QUANT-MIN_DELTA_GAIN_QUANT+1,
			"invalid GainsIndices")
		psRangeEnc.EncIcdf(psIndices.GainsIndices[i], silkDeltaGainIcdf[:], 8)
	}

	/*************/
	/* Encode NLSFs */
	/*************/
	psRangeEnc.EncIcdf(psIndices.NLSFIndices[0],
		psEncC.PsNLSF_CB.CB1Icdf[:],
		(psIndices.SignalType>>1)*psEncC.PsNLSF_CB.NVectors, 8)

	NLSFUnpack(ecIx, predQ8, psEncC.PsNLSF_CB, psIndices.NLSFIndices[0])
	opusAssert(psEncC.PsNLSF_CB.Order == psEncC.PredictLPCOrder, "NLSF order mismatch")

	for i := 0; i < psEncC.PsNLSF_CB.Order; i++ {
		switch {
		case psIndices.NLSFIndices[i+1] >= NLSF_QUANT_MAX_AMPLITUDE:
			psRangeEnc.EncIcdf(2*NLSF_QUANT_MAX_AMPLITUDE,
				psEncC.PsNLSF_CB.EcIcdf[ecIx[i]][:], 8)
			psRangeEnc.EncIcdf(psIndices.NLSFIndices[i+1]-NLSF_QUANT_MAX_AMPLITUDE,
				silkNLSFExtIcdf[:], 8)
		case psIndices.NLSFIndices[i+1] <= -NLSF_QUANT_MAX_AMPLITUDE:
			psRangeEnc.EncIcdf(0,
				psEncC.PsNLSF_CB.EcIcdf[ecIx[i]][:], 8)
			psRangeEnc.EncIcdf(-psIndices.NLSFIndices[i+1]-NLSF_QUANT_MAX_AMPLITUDE,
				silkNLSFExtIcdf[:], 8)
		default:
			psRangeEnc.EncIcdf(psIndices.NLSFIndices[i+1]+NLSF_QUANT_MAX_AMPLITUDE,
				psEncC.PsNLSF_CB.EcIcdf[ecIx[i]][:], 8)
		}
	}

	// Encode NLSF interpolation factor
	if psEncC.NbSubfr == MAX_NB_SUBFR {
		opusAssert(psIndices.NLSFInterpCoefQ2 >= 0 && psIndices.NLSFInterpCoefQ2 < 5,
			"invalid NLSFInterpCoefQ2")
		psRangeEnc.EncIcdf(psIndices.NLSFInterpCoefQ2, silkNLSFInterpolationFactorIcdf[:], 8)
	}

	if psIndices.SignalType == TYPE_VOICED {
		/******************/
		/* Encode pitch lags */
		/******************/
		// Lag index
		encodeAbsoluteLagIndex = 1
		if condCoding == CODE_CONDITIONALLY && psEncC.EcPrevSignalType == TYPE_VOICED {
			// Delta Encoding
			deltaLagIndex = psIndices.LagIndex - psEncC.EcPrevLagIndex

			if deltaLagIndex < -8 || deltaLagIndex > 11 {
				deltaLagIndex = 0
			} else {
				deltaLagIndex += 9
				encodeAbsoluteLagIndex = 0 // Only use delta
			}

			opusAssert(deltaLagIndex >= 0 && deltaLagIndex < 21, "invalid deltaLagIndex")
			psRangeEnc.EncIcdf(deltaLagIndex, silkPitchDeltaIcdf[:], 8)
		}

		if encodeAbsoluteLagIndex != 0 {
			// Absolute encoding
			pitchHighBits := psIndices.LagIndex / (psEncC.FsKHz >> 1)
			pitchLowBits := psIndices.LagIndex - pitchHighBits*(psEncC.FsKHz>>1)

			opusAssert(pitchLowBits < psEncC.FsKHz/2, "invalid pitchLowBits")
			opusAssert(pitchHighBits < 32, "invalid pitchHighBits")

			psRangeEnc.EncIcdf(pitchHighBits, silkPitchLagIcdf[:], 8)
			psRangeEnc.EncIcdf(pitchLowBits, psEncC.PitchLagLowBitsIcdf[:], 8)
		}
		psEncC.EcPrevLagIndex = psIndices.LagIndex

		// Contour index
		opusAssert(psIndices.ContourIndex >= 0, "invalid ContourIndex")
		opusAssert(
			(psIndices.ContourIndex < 34 && psEncC.FsKHz > 8 && psEncC.NbSubfr == 4) ||
				(psIndices.ContourIndex < 11 && psEncC.FsKHz == 8 && psEncC.NbSubfr == 4) ||
				(psIndices.ContourIndex < 12 && psEncC.FsKHz > 8 && psEncC.NbSubfr == 2) ||
				(psIndices.ContourIndex < 3 && psEncC.FsKHz == 8 && psEncC.NbSubfr == 2),
			"ContourIndex out of range for configuration")
		psRangeEnc.EncIcdf(psIndices.ContourIndex, psEncC.PitchContourIcdf[:], 8)

		/*****************/
		/* Encode LTP gains */
		/*****************/
		// PERIndex value
		opusAssert(psIndices.PERIndex >= 0 && psIndices.PERIndex < 3, "invalid PERIndex")
		psRangeEnc.EncIcdf(psIndices.PERIndex, silkLTPPerIndexIcdf[:], 8)

		// Codebook Indices
		for k := 0; k < psEncC.NbSubfr; k++ {
			opusAssert(psIndices.LTPIndex[k] >= 0 &&
				psIndices.LTPIndex[k] < (8<<psIndices.PERIndex),
				"invalid LTPIndex")
			psRangeEnc.EncIcdf(psIndices.LTPIndex[k],
				silkLTPGainIcdfPtrs[psIndices.PERIndex][:], 8)
		}

		/*******************/
		/* Encode LTP scaling */
		/*******************/
		if condCoding == CODE_INDEPENDENTLY {
			opusAssert(psIndices.LTPScaleIndex >= 0 && psIndices.LTPScaleIndex < 3,
				"invalid LTPScaleIndex")
			psRangeEnc.EncIcdf(psIndices.LTPScaleIndex, silkLTPscaleIcdf[:], 8)
		}

		opusAssert(condCoding == 0 || psIndices.LTPScaleIndex == 0,
			"invalid LTPScaleIndex for conditional coding")
	}

	psEncC.EcPrevSignalType = psIndices.SignalType

	/************/
	/* Encode seed */
	/************/
	opusAssert(psIndices.Seed >= 0 && psIndices.Seed < 4, "invalid Seed")
	psRangeEnc.EncIcdf(psIndices.Seed, silkUniform4Icdf[:], 8)
}

// opusAssert is a helper function to replace Java assertions
func opusAssert(condition bool, message string) {
	if !condition {
		panic("Assertion failed: " + message)
	}
}
