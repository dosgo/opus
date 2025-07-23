package silk

// Copyright (c) 2006-2011 Skype Limited. All Rights Reserved
// Ported to Go from Java implementation

import (
	"math"
)

// VAD implements Voice Activity Detection for the SILK codec
type VAD struct {
	AnaState        [2]int32           // Analysis filterbank state [2]
	AnaState1       [2]int32           // Analysis filterbank state [2]
	AnaState2       [2]int32           // Analysis filterbank state [2]
	HPstate         int32              // State of highpass filter
	NL              [VAD_N_BANDS]int32 // Noise level for each band
	inv_NL          [VAD_N_BANDS]int32 // Inverse noise level for each band
	NoiseLevelBias  [VAD_N_BANDS]int32 // Noise level bias
	NrgRatioSmth_Q8 [VAD_N_BANDS]int32 // Smoothed energy-to-noise ratio
	XnrgSubfr       [VAD_N_BANDS]int32 // Subframe energies
	counter         int32              // Frame counter used for initialization
}

// tiltWeights are weighting factors for tilt measure
var tiltWeights = [4]int32{30000, 6000, -12000, -12000}

// Reset initializes the VAD state
func (v *VAD) Reset() {
	v.AnaState = [2]int32{0, 0}
	v.AnaState1 = [2]int32{0, 0}
	v.AnaState2 = [2]int32{0, 0}
	v.HPstate = 0
	for i := range v.NL {
		v.NL[i] = 0
		v.inv_NL[i] = 0
		v.NoiseLevelBias[i] = 0
		v.NrgRatioSmth_Q8[i] = 0
		v.XnrgSubfr[i] = 0
	}
	v.counter = 0
}

// Init initializes the Silk VAD state
func (v *VAD) Init() {
	// Reset state memory
	v.Reset()

	// Initialize noise levels with approx pink noise levels (psd proportional to inverse of frequency)
	for b := 0; b < VAD_N_BANDS; b++ {
		v.NoiseLevelBias[b] = max32(DIV32_16(VAD_NOISE_LEVELS_BIAS, int16(b+1)), 1)
	}

	// Initialize state
	for b := 0; b < VAD_N_BANDS; b++ {
		v.NL[b] = MUL(100, v.NoiseLevelBias[b])
		v.inv_NL[b] = DIV32(math.MaxInt32, v.NL[b])
	}

	v.counter = 15

	// Initialize smoothed energy-to-noise ratio (20 dB SNR)
	for b := 0; b < VAD_N_BANDS; b++ {
		v.NrgRatioSmth_Q8[b] = 100 * 256
	}
}

// GetSA_Q8 gets the speech activity level in Q8 format
func (v *VAD) GetSA_Q8(enc *Encoder, pIn []int16) int32 {
	var (
		SA_Q15, pSNR_dB_Q7, input_tilt int32
		decimated_framelength1         = RSHIFT(int32(enc.frame_length), 1)
		decimated_framelength2         = RSHIFT(int32(enc.frame_length), 2)
		decimated_framelength          = RSHIFT(int32(enc.frame_length), 3)
		X                              = make([]int16, 3*decimated_framelength+decimated_framelength1)
		Xnrg                           = [VAD_N_BANDS]int32{}
		NrgToNoiseRatio_Q8             = [VAD_N_BANDS]int32{}
		X_offset                       = [VAD_N_BANDS]int32{
			0,
			decimated_framelength + decimated_framelength2,
			decimated_framelength + decimated_framelength2 + decimated_framelength,
			decimated_framelength + decimated_framelength2 + decimated_framelength + decimated_framelength2,
		}
	)

	// Safety checks
	if VAD_N_BANDS != 4 {
		panic("VAD_N_BANDS must be 4")
	}
	if MAX_FRAME_LENGTH < enc.frame_length {
		panic("frame_length exceeds MAX_FRAME_LENGTH")
	}
	if enc.frame_length > 512 {
		panic("frame_length exceeds 512")
	}
	if enc.frame_length != 8*RSHIFT(int32(enc.frame_length), 3) {
		panic("frame_length must be multiple of 8")
	}

	/* Filter and Decimate */
	// 0-8 kHz to 0-4 kHz and 4-8 kHz
	AnaFiltBank1(pIn, &v.AnaState, X, X[X_offset[3]:], enc.frame_length)

	// 0-4 kHz to 0-2 kHz and 2-4 kHz
	AnaFiltBank1(X[:X_offset[3]], &v.AnaState1, X, X[X_offset[2]:], decimated_framelength1)

	// 0-2 kHz to 0-1 kHz and 1-2 kHz
	AnaFiltBank1(X[:X_offset[2]], &v.AnaState2, X, X[X_offset[1]:], decimated_framelength2)

	/* HP filter on lowest band (differentiator) */
	X[decimated_framelength-1] = int16(RSHIFT(int32(X[decimated_framelength-1]), 1))
	HPstateTmp := X[decimated_framelength-1]

	for i := decimated_framelength - 1; i > 0; i-- {
		X[i-1] = int16(RSHIFT(int32(X[i-1]), 1))
		X[i] -= X[i-1]
	}

	X[0] -= int16(v.HPstate)
	v.HPstate = int32(HPstateTmp)

	/* Calculate the energy in each band */
	for b := 0; b < VAD_N_BANDS; b++ {
		// Find the decimated framelength in the non-uniformly divided bands
		decimated_framelength = RSHIFT(int32(enc.frame_length), int32(min(VAD_N_BANDS-b, VAD_N_BANDS-1)))

		// Split length into subframe lengths
		dec_subframe_length := RSHIFT(decimated_framelength, VAD_INTERNAL_SUBFRAMES_LOG2)
		dec_subframe_offset := int32(0)

		// Compute energy per sub-frame
		Xnrg[b] = v.XnrgSubfr[b] // initialize with summed energy of last subframe

		for s := 0; s < VAD_INTERNAL_SUBFRAMES; s++ {
			sumSquared := int32(0)

			for i := int32(0); i < dec_subframe_length; i++ {
				x_tmp := RSHIFT(int32(X[X_offset[b]+int(i)+dec_subframe_offset]), 3)
				sumSquared = SMLABB(sumSquared, x_tmp, x_tmp)
			}

			// Add/saturate summed energy of current subframe
			if s < VAD_INTERNAL_SUBFRAMES-1 {
				Xnrg[b] = ADD_POS_SAT32(Xnrg[b], sumSquared)
			} else {
				// Look-ahead subframe
				Xnrg[b] = ADD_POS_SAT32(Xnrg[b], RSHIFT(sumSquared, 1))
			}

			dec_subframe_offset += dec_subframe_length
		}

		v.XnrgSubfr[b] = sumSquared
	}

	/* Noise estimation */
	v.GetNoiseLevels(Xnrg[:])

	/* Signal-plus-noise to noise ratio estimation */
	sumSquared := int32(0)
	input_tilt = 0
	for b := 0; b < VAD_N_BANDS; b++ {
		speech_nrg := Xnrg[b] - v.NL[b]
		if speech_nrg > 0 {
			// Divide with sufficient resolution
			if (Xnrg[b] & 0xFF800000) == 0 {
				NrgToNoiseRatio_Q8[b] = DIV32(LSHIFT(Xnrg[b], 8), v.NL[b]+1)
			} else {
				NrgToNoiseRatio_Q8[b] = DIV32(Xnrg[b], RSHIFT(v.NL[b], 8)+1)
			}

			// Convert to log domain
			SNR_Q7 := Lin2Log(NrgToNoiseRatio_Q8[b]) - 8*128

			// Sum-of-squares
			sumSquared = SMLABB(sumSquared, SNR_Q7, SNR_Q7) // Q14

			// Tilt measure
			if speech_nrg < (1 << 20) {
				// Scale down SNR value for small subband speech energies
				SNR_Q7 = SMULWB(LSHIFT(SQRT_APPROX(speech_nrg), 6), SNR_Q7)
			}
			input_tilt = SMLAWB(input_tilt, tiltWeights[b], SNR_Q7)
		} else {
			NrgToNoiseRatio_Q8[b] = 256
		}
	}

	// Mean-of-squares
	sumSquared = DIV32_16(sumSquared, VAD_N_BANDS) // Q14

	// Root-mean-square approximation, scale to dBs
	pSNR_dB_Q7 = 3 * SQRT_APPROX(sumSquared) // Q7

	/* Speech Probability Estimation */
	SA_Q15 = Sigmoid(SMULWB(VAD_SNR_FACTOR_Q16, pSNR_dB_Q7) - VAD_NEGATIVE_OFFSET_Q5)

	/* Frequency Tilt Measure */
	enc.input_tilt_Q15 = LSHIFT(Sigmoid(input_tilt)-16384, 1)

	/* Scale the sigmoid output based on power levels */
	speech_nrg := int32(0)
	for b := 0; b < VAD_N_BANDS; b++ {
		// Accumulate signal-without-noise energies, higher frequency bands have more weight
		speech_nrg += int32(b+1) * RSHIFT(Xnrg[b]-v.NL[b], 4)
	}

	// Power scaling
	if speech_nrg <= 0 {
		SA_Q15 = RSHIFT(SA_Q15, 1)
	} else if speech_nrg < 32768 {
		if enc.frame_length == 10*int32(enc.fs_kHz) {
			speech_nrg = LSHIFT_SAT32(speech_nrg, 16)
		} else {
			speech_nrg = LSHIFT_SAT32(speech_nrg, 15)
		}

		// square-root
		speech_nrg = SQRT_APPROX(speech_nrg)
		SA_Q15 = SMULWB(32768+speech_nrg, SA_Q15)
	}

	// Copy the resulting speech activity in Q8 (clamped to 255)
	enc.speech_activity_Q8 = min(RSHIFT(SA_Q15, 7), 255)

	/* Energy Level and SNR estimation */
	// Smoothing coefficient
	smooth_coef_Q16 := SMULWB(VAD_SNR_SMOOTH_COEF_Q18, SMULWB(SA_Q15, SA_Q15))

	if enc.frame_length == 10*int32(enc.fs_kHz) {
		smooth_coef_Q16 >>= 1
	}

	for b := 0; b < VAD_N_BANDS; b++ {
		// compute smoothed energy-to-noise ratio per band
		v.NrgRatioSmth_Q8[b] = SMLAWB(v.NrgRatioSmth_Q8[b],
			NrgToNoiseRatio_Q8[b]-v.NrgRatioSmth_Q8[b], smooth_coef_Q16)

		// signal to noise ratio in dB per band
		SNR_Q7 := 3 * (Lin2Log(v.NrgRatioSmth_Q8[b]) - 8*128)

		// quality = sigmoid( 0.25 * ( SNR_dB - 16 ) )
		enc.input_quality_bands_Q15[b] = Sigmoid(RSHIFT(SNR_Q7-16*128, 4))
	}

	return 0
}

// GetNoiseLevels estimates noise levels
func (v *VAD) GetNoiseLevels(pX [VAD_N_BANDS]int32) {
	var min_coef int32

	// Initially faster smoothing
	if v.counter < 1000 { // 1000 = 20 sec
		min_coef = DIV32_16(math.MaxInt32, int16(RSHIFT(v.counter, 4)+1))
	} else {
		min_coef = 0
	}

	for k := 0; k < VAD_N_BANDS; k++ {
		// Get old noise level estimate for current band
		nl := v.NL[k]
		if nl < 0 {
			panic("negative noise level")
		}

		// Add bias
		nrg := ADD_POS_SAT32(pX[k], v.NoiseLevelBias[k])
		if nrg <= 0 {
			panic("non-positive energy")
		}

		// Invert energies
		inv_nrg := DIV32(math.MaxInt32, nrg)
		if inv_nrg < 0 {
			panic("negative inverse energy")
		}

		// Less update when subband energy is high
		var coef int32
		if nrg > LSHIFT(nl, 3) {
			coef = VAD_NOISE_LEVEL_SMOOTH_COEF_Q16 >> 3
		} else if nrg < nl {
			coef = VAD_NOISE_LEVEL_SMOOTH_COEF_Q16
		} else {
			coef = SMULWB(SMULWW(inv_nrg, nl), VAD_NOISE_LEVEL_SMOOTH_COEF_Q16<<1)
		}

		// Initially faster smoothing
		coef = max32(coef, min_coef)

		// Smooth inverse energies
		v.inv_NL[k] = SMLAWB(v.inv_NL[k], inv_nrg-v.inv_NL[k], coef)
		if v.inv_NL[k] < 0 {
			panic("negative inverse noise level")
		}

		// Compute noise level by inverting again
		nl = DIV32(math.MaxInt32, v.inv_NL[k])
		if nl < 0 {
			panic("negative noise level")
		}

		// Limit noise levels (guarantee 7 bits of head room)
		nl = min32(nl, 0x00FFFFFF)

		// Store as part of state
		v.NL[k] = nl
	}

	// Increment frame counter
	v.counter++
}
