package silk

// FindPitchLags finds pitch lags for the encoder
func FindPitchLags(
	psEnc *ChannelEncoder, // I/O encoder state
	psEncCtrl *EncoderControl, // I/O encoder control
	res []int16, // O residual
	x []int16, // I speech signal
	xPtr int, // I pointer to current position in x
) {
	// ***************************************
	// Set up buffer lengths etc based on Fs
	// ***************************************
	bufLen := psEnc.LaPitch + psEnc.FrameLength + psEnc.LtpMemLength

	// Safety check
	if bufLen < psEnc.PitchLPCWinLength {
		panic("buffer length too small")
	}

	xBuf := xPtr - psEnc.LtpMemLength

	// **********************************
	// Estimate LPC AR coefficients
	// **********************************

	// Calculate windowed signal
	wsig := make([]int16, psEnc.PitchLPCWinLength)

	// First LA_LTP samples
	xBufPtr := xBuf + bufLen - psEnc.PitchLPCWinLength
	wsigPtr := 0
	ApplySineWindow(wsig, wsigPtr, x, xBufPtr, 1, psEnc.LaPitch)

	// Middle un-windowed samples
	wsigPtr += psEnc.LaPitch
	xBufPtr += psEnc.LaPitch
	copy(wsig[wsigPtr:], x[xBufPtr:xBufPtr+(psEnc.PitchLPCWinLength-(psEnc.LaPitch<<1))])

	// Last LA_LTP samples
	wsigPtr += psEnc.PitchLPCWinLength - (psEnc.LaPitch << 1)
	xBufPtr += psEnc.PitchLPCWinLength - (psEnc.LaPitch << 1)
	ApplySineWindow(wsig, wsigPtr, x, xBufPtr, 2, psEnc.LaPitch)

	// Calculate autocorrelation sequence
	autoCorr := make([]int32, MAX_FIND_PITCH_LPC_ORDER+1)
	scale := Autocorrelation(autoCorr, wsig, psEnc.PitchLPCWinLength, psEnc.PitchEstimationLPCOrder+1)

	// Add white noise, as fraction of energy
	whiteNoiseFraction := int32(FIND_PITCH_WHITE_NOISE_FRACTION * (1 << 16))
	autoCorr[0] = SMLAWB(autoCorr[0], autoCorr[0], whiteNoiseFraction) + 1

	// Calculate the reflection coefficients using schur
	rcQ15 := make([]int16, MAX_FIND_PITCH_LPC_ORDER)
	resNrg := Schur(rcQ15, autoCorr, psEnc.PitchEstimationLPCOrder)

	// Prediction gain
	psEncCtrl.PredGainQ16 = DIV32VarQ(autoCorr[0], maxInt32(resNrg, 1), 16)

	// Convert reflection coefficients to prediction coefficients
	AQ24 := make([]int32, MAX_FIND_PITCH_LPC_ORDER)
	K2A(AQ24, rcQ15, psEnc.PitchEstimationLPCOrder)

	// Convert From 32 bit Q24 to 16 bit Q12 coefs
	AQ12 := make([]int16, MAX_FIND_PITCH_LPC_ORDER)
	for i := 0; i < psEnc.PitchEstimationLPCOrder; i++ {
		AQ12[i] = int16(SAT16(RSHIFT(AQ24[i], 12)))
	}

	// Do Bandwidth Expansion (BWE)
	bwExpansion := int32(FIND_PITCH_BANDWIDTH_EXPANSION * (1 << 16))
	BWExpander(AQ12, psEnc.PitchEstimationLPCOrder, bwExpansion)

	// **************************************
	// LPC analysis filtering
	// **************************************
	LPCAnalysisFilter(res, 0, x, xBuf, AQ12, 0, bufLen, psEnc.PitchEstimationLPCOrder)

	if psEnc.Indices.SignalType != TYPE_NO_VOICE_ACTIVITY && psEnc.FirstFrameAfterReset == 0 {
		// Threshold for pitch estimator
		thrhldQ13 := int32(0.6 * (1 << 13))
		thrhldQ13 = SMLABB(thrhldQ13, int32(-0.004*(1<<13)), psEnc.PitchEstimationLPCOrder)
		thrhldQ13 = SMLAWB(thrhldQ13, int32(-0.1*(1<<21)), psEnc.SpeechActivityQ8)
		thrhldQ13 = SMLABB(thrhldQ13, int32(-0.15*(1<<13)), RSHIFT(int32(psEnc.PrevSignalType), 1))
		thrhldQ13 = SMLAWB(thrhldQ13, int32(-0.1*(1<<14)), psEnc.InputTiltQ15)
		thrhldQ13 = SAT16(thrhldQ13)

		// **************************************
		// Call pitch estimator
		// **************************************
		lagIndex := psEnc.Indices.LagIndex
		contourIndex := psEnc.Indices.ContourIndex
		ltpCorrQ15 := psEnc.LTPCorrQ15

		if PitchAnalysisCore(
			res,
			psEncCtrl.PitchL[:],
			&lagIndex,
			&contourIndex,
			&ltpCorrQ15,
			psEnc.PrevLag,
			psEnc.PitchEstimationThresholdQ16,
			int(thrhldQ13),
			psEnc.FsKHz,
			psEnc.PitchEstimationComplexity,
			psEnc.NbSubfr,
		) == 0 {
			psEnc.Indices.SignalType = TYPE_VOICED
		} else {
			psEnc.Indices.SignalType = TYPE_UNVOICED
		}

		psEnc.Indices.LagIndex = lagIndex
		psEnc.Indices.ContourIndex = contourIndex
		psEnc.LTPCorrQ15 = ltpCorrQ15
	} else {
		for i := range psEncCtrl.PitchL {
			psEncCtrl.PitchL[i] = 0
		}
		psEnc.Indices.LagIndex = 0
		psEnc.Indices.ContourIndex = 0
		psEnc.LTPCorrQ15 = 0
	}
}

// Helper functions that would be defined elsewhere in the package

func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
