package silk

// DecodeCore performs inverse NSQ operation (LTP + LPC) for Silk decoding
func DecodeCore(
	psDec *ChannelDecoder, // I/O Decoder state
	psDecCtrl *DecoderControl, // I Decoder control
	xq []int16, // O Decoded speech
	xqPtr int, // Offset in xq array
	pulses []int16, // I Pulse signal [MAX_FRAME_LENGTH]
) {
	// Validate input
	if psDec.prevGainQ16 == 0 {
		panic("prevGainQ16 should not be zero")
	}

	// Allocate temporary buffers
	sLTP := make([]int16, psDec.ltpMemLength)
	sLTPQ15 := make([]int32, psDec.ltpMemLength+psDec.frameLength)
	resQ14 := make([]int32, psDec.subfrLength)
	sLPCQ14 := make([]int32, psDec.subfrLength+MaxLPCOrder)

	// Get quantization offset
	offsetQ10 := QuantizationOffsetsQ10[psDec.indices.SignalType>>1][psDec.indices.QuantOffsetType]

	// Determine NLSF interpolation flag
	NLSFInterpolationFlag := 0
	if psDec.indices.NLSFInterpCoefQ2 < 1<<2 {
		NLSFInterpolationFlag = 1
	}

	// Decode excitation
	randSeed := psDec.indices.Seed
	for i := 0; i < psDec.frameLength; i++ {
		randSeed = RAND(randSeed)
		psDec.excQ14[i] = int32(pulses[i]) << 14

		// Apply quantization adjustment
		if psDec.excQ14[i] > 0 {
			psDec.excQ14[i] -= int32(QuantLevelAdjustQ10) << 4
		} else if psDec.excQ14[i] < 0 {
			psDec.excQ14[i] += int32(QuantLevelAdjustQ10) << 4
		}

		// Add offset and random sign
		psDec.excQ14[i] += int32(offsetQ10) << 4
		if randSeed < 0 {
			psDec.excQ14[i] = -psDec.excQ14[i]
		}

		randSeed = ADD32_ovflw(randSeed, int32(pulses[i]))
	}

	// Copy LPC state
	copy(sLPCQ14[:MaxLPCOrder], psDec.sLPCQ14Buf[:])

	pexcQ14 := 0
	pxq := xqPtr
	sLTPBufIdx := psDec.ltpMemLength

	// Loop over subframes
	for k := 0; k < psDec.nbSubfr; k++ {
		presQ14 := resQ14
		presQ14Ptr := 0
		A_Q12 := psDecCtrl.PredCoefQ12[k>>1]
		B_Q14_ptr := k * LTPOrder
		signalType := psDec.indices.SignalType

		Gain_Q10 := int32(psDecCtrl.GainsQ16[k] >> 6)
		inv_gain_Q31 := INVERSE32_varQ(int32(psDecCtrl.GainsQ16[k]), 47)

		// Calculate gain adjustment factor
		var gain_adj_Q16 int32
		if psDecCtrl.GainsQ16[k] != psDec.prevGainQ16 {
			gain_adj_Q16 = DIV32_varQ(int32(psDec.prevGainQ16), int32(psDecCtrl.GainsQ16[k]), 16)

			// Scale short term state
			for i := 0; i < MaxLPCOrder; i++ {
				sLPCQ14[i] = SMULWW(gain_adj_Q16, sLPCQ14[i])
			}
		} else {
			gain_adj_Q16 = 1 << 16
		}

		// Save inv_gain
		if inv_gain_Q31 == 0 {
			panic("inv_gain_Q31 should not be zero")
		}
		psDec.prevGainQ16 = psDecCtrl.GainsQ16[k]

		// Handle transition from voiced PLC to unvoiced normal decoding
		if psDec.lossCnt != 0 && psDec.prevSignalType == TypeVoiced &&
			psDec.indices.SignalType != TypeVoiced && k < MaxNbSubfr/2 {

			for i := 0; i < LTPOrder; i++ {
				psDecCtrl.LTPCoefQ14[B_Q14_ptr+i] = 0
			}
			psDecCtrl.LTPCoefQ14[B_Q14_ptr+(LTPOrder/2)] = int16(float32(0.25)*float32(1<<14) + 0.5)

			signalType = TypeVoiced
			psDecCtrl.PitchL[k] = psDec.lagPrev
		}

		if signalType == TypeVoiced {
			// Voiced processing
			lag := psDecCtrl.PitchL[k]

			// Re-whitening
			if k == 0 || (k == 2 && NLSFInterpolationFlag != 0) {
				// Rewhiten with new A coefs
				startIdx := psDec.ltpMemLength - lag - psDec.LPCOrder - LTPOrder/2
				if startIdx <= 0 {
					panic("startIdx should be positive")
				}

				if k == 2 {
					copy(psDec.outBuf[psDec.ltpMemLength:], xq[xqPtr:xqPtr+2*psDec.subfrLength])
				}

				LPC_analysis_filter(sLTP, startIdx, psDec.outBuf, startIdx+k*psDec.subfrLength,
					A_Q12, 0, psDec.ltpMemLength-startIdx, psDec.LPCOrder)

				// After rewhitening the LTP state is unscaled
				if k == 0 {
					// Do LTP downscaling to reduce inter-packet dependency
					inv_gain_Q31 = SMULWB(inv_gain_Q31, int32(psDecCtrl.LTPScaleQ14)) << 2
				}
				for i := 0; i < lag+LTPOrder/2; i++ {
					sLTPQ15[sLTPBufIdx-i-1] = SMULWB(inv_gain_Q31, int32(sLTP[psDec.ltpMemLength-i-1]))
				}
			} else if gain_adj_Q16 != 1<<16 {
				// Update LTP state when Gain changes
				for i := 0; i < lag+LTPOrder/2; i++ {
					sLTPQ15[sLTPBufIdx-i-1] = SMULWW(gain_adj_Q16, sLTPQ15[sLTPBufIdx-i-1])
				}
			}

			// Long-term prediction
			pred_lag_ptr := sLTPBufIdx - lag + LTPOrder/2
			for i := 0; i < psDec.subfrLength; i++ {
				// Unrolled LTP prediction
				LTP_pred_Q13 := int32(2)
				LTP_pred_Q13 = SMLAWB(LTP_pred_Q13, sLTPQ15[pred_lag_ptr], int32(psDecCtrl.LTPCoefQ14[B_Q14_ptr]))
				LTP_pred_Q13 = SMLAWB(LTP_pred_Q13, sLTPQ15[pred_lag_ptr-1], int32(psDecCtrl.LTPCoefQ14[B_Q14_ptr+1]))
				LTP_pred_Q13 = SMLAWB(LTP_pred_Q13, sLTPQ15[pred_lag_ptr-2], int32(psDecCtrl.LTPCoefQ14[B_Q14_ptr+2]))
				LTP_pred_Q13 = SMLAWB(LTP_pred_Q13, sLTPQ15[pred_lag_ptr-3], int32(psDecCtrl.LTPCoefQ14[B_Q14_ptr+3]))
				LTP_pred_Q13 = SMLAWB(LTP_pred_Q13, sLTPQ15[pred_lag_ptr-4], int32(psDecCtrl.LTPCoefQ14[B_Q14_ptr+4]))
				pred_lag_ptr++

				// Generate LPC excitation
				presQ14[presQ14Ptr+i] = ADD_LSHIFT32(psDec.excQ14[pexcQ14+i], LTP_pred_Q13, 1)

				// Update states
				sLTPQ15[sLTPBufIdx] = int32(presQ14[presQ14Ptr+i]) << 1
				sLTPBufIdx++
			}
		} else {
			// Unvoiced processing
			presQ14 = psDec.excQ14
			presQ14Ptr = pexcQ14
		}

		// Short-term prediction
		for i := 0; i < psDec.subfrLength; i++ {
			if psDec.LPCOrder != 10 && psDec.LPCOrder != 16 {
				panic("LPC order must be 10 or 16")
			}

			// LPC prediction
			LPC_pred_Q10 := int32(psDec.LPCOrder >> 1)
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLPCQ14[MaxLPCOrder+i-1], int32(A_Q12[0]))
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLPCQ14[MaxLPCOrder+i-2], int32(A_Q12[1]))
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLPCQ14[MaxLPCOrder+i-3], int32(A_Q12[2]))
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLPCQ14[MaxLPCOrder+i-4], int32(A_Q12[3]))
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLPCQ14[MaxLPCOrder+i-5], int32(A_Q12[4]))
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLPCQ14[MaxLPCOrder+i-6], int32(A_Q12[5]))
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLPCQ14[MaxLPCOrder+i-7], int32(A_Q12[6]))
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLPCQ14[MaxLPCOrder+i-8], int32(A_Q12[7]))
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLPCQ14[MaxLPCOrder+i-9], int32(A_Q12[8]))
			LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLPCQ14[MaxLPCOrder+i-10], int32(A_Q12[9]))

			if psDec.LPCOrder == 16 {
				LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLPCQ14[MaxLPCOrder+i-11], int32(A_Q12[10]))
				LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLPCQ14[MaxLPCOrder+i-12], int32(A_Q12[11]))
				LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLPCQ14[MaxLPCOrder+i-13], int32(A_Q12[12]))
				LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLPCQ14[MaxLPCOrder+i-14], int32(A_Q12[13]))
				LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLPCQ14[MaxLPCOrder+i-15], int32(A_Q12[14]))
				LPC_pred_Q10 = SMLAWB(LPC_pred_Q10, sLPCQ14[MaxLPCOrder+i-16], int32(A_Q12[15]))
			}

			// Add prediction to LPC excitation
			sLPCQ14[MaxLPCOrder+i] = ADD_LSHIFT32(presQ14[presQ14Ptr+i], LPC_pred_Q10, 4)

			// Scale with gain and store result
			scaled := SMULWW(sLPCQ14[MaxLPCOrder+i], Gain_Q10)
			xq[pxq+i] = int16(SAT16(RSHIFT_ROUND(scaled, 8)))
		}

		// Update LPC filter state
		copy(sLPCQ14[:MaxLPCOrder], sLPCQ14[psDec.subfrLength:])
		pexcQ14 += psDec.subfrLength
		pxq += psDec.subfrLength
	}

	// Save LPC state
	copy(psDec.sLPCQ14Buf[:], sLPCQ14[:MaxLPCOrder])
}
