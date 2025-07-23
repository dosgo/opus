package silk

// DecodeParameters decodes parameters from the payload according to the Silk codec specification.
// This is a direct translation from the Java implementation with Go idioms applied.
func DecodeParameters(
	psDec *ChannelDecoder,      // I/O State
	psDecCtrl *DecoderControl,  // I/O Decoder control
	condCoding int,             // I The type of conditional coding to use
) {
	lpcOrder := psDec.LPC_order
	nbSubfr := psDec.nb_subfr

	// Dequantize gains
	lastGainIndex := psDec.LastGainIndex
	GainsDequant(
		psDecCtrl.Gains_Q16[:],
		psDec.indices.GainsIndices[:],
		&lastGainIndex,
		BoolToInt(condCoding == CODE_CONDITIONALLY),
		nbSubfr,
	)
	psDec.LastGainIndex = lastGainIndex

	// *****************
	// Decode NLSFs
	// *****************
	var pNLSF_Q15 [MAX_LPC_ORDER]int16
	NLSFDecode(
		pNLSF_Q15[:lpcOrder],
		psDec.indices.NLSFIndices[:],
		psDec.psNLSF_CB,
	)

	// Convert NLSF parameters to AR prediction filter coefficients
	NLSF2A(
		psDecCtrl.PredCoef_Q12[1][:lpcOrder],
		pNLSF_Q15[:lpcOrder],
		lpcOrder,
	)

	// If just reset (e.g., because internal Fs changed), do not allow interpolation
	// improves the case of packet loss in the first frame after a switch
	if psDec.first_frame_after_reset == 1 {
		psDec.indices.NLSFInterpCoef_Q2 = 4
	}

	var pNLSF0_Q15 [MAX_LPC_ORDER]int16
	if psDec.indices.NLSFInterpCoef_Q2 < 4 {
		// Calculation of the interpolated NLSF0 vector from the interpolation factor,
		// the previous NLSF1, and the current NLSF1
		for i := 0; i < lpcOrder; i++ {
			pNLSF0_Q15[i] = int16(
				int(psDec.prevNLSF_Q15[i]) + 
				((int(psDec.indices.NLSFInterpCoef_Q2) * 
					(int(pNLSF_Q15[i]) - int(psDec.prevNLSF_Q15[i]))) >> 2,
			)
		}

		// Convert NLSF parameters to AR prediction filter coefficients
		NLSF2A(
			psDecCtrl.PredCoef_Q12[0][:lpcOrder],
			pNLSF0_Q15[:lpcOrder],
			lpcOrder,
		)
	} else {
		// Copy LPC coefficients for first half from second half
		copy(psDecCtrl.PredCoef_Q12[0][:lpcOrder], psDecCtrl.PredCoef_Q12[1][:lpcOrder])
	}

	// Update previous NLSF values
	copy(psDec.prevNLSF_Q15[:lpcOrder], pNLSF_Q15[:lpcOrder])

	// After a packet loss do BWE of LPC coefs
	if psDec.lossCnt != 0 {
		BWExpander(
			psDecCtrl.PredCoef_Q12[0][:lpcOrder], 
			lpcOrder, 
			BWE_AFTER_LOSS_Q16,
		)
		BWExpander(
			psDecCtrl.PredCoef_Q12[1][:lpcOrder], 
			lpcOrder, 
			BWE_AFTER_LOSS_Q16,
		)
	}

	if psDec.indices.signalType == TYPE_VOICED {
		// *******************
		// Decode pitch lags
		// *******************

		// Decode pitch values
		DecodePitch(
			psDec.indices.lagIndex[:],
			psDec.indices.contourIndex[:],
			psDecCtrl.pitchL[:],
			psDec.fs_kHz,
			nbSubfr,
		)

		// Decode Codebook Index
		cbk_ptr_Q7 := LTP_vq_ptrs_Q7[psDec.indices.PERIndex] // set pointer to start of codebook

		for k := 0; k < nbSubfr; k++ {
			ix := psDec.indices.LTPIndex[k]
			for i := 0; i < LTP_ORDER; i++ {
				psDecCtrl.LTPCoef_Q14[k*LTP_ORDER+i] = int16(cbk_ptr_Q7[ix][i] << 7)
			}
		}

		// ********************
		// Decode LTP scaling
		// ********************
		ix := psDec.indices.LTP_scaleIndex
		psDecCtrl.LTP_scale_Q14 = LTPScales_table_Q14[ix]
	} else {
		// Unvoiced case - zero out pitch and LTP coefficients
		for i := range psDecCtrl.pitchL {
			psDecCtrl.pitchL[i] = 0
		}
		for i := range psDecCtrl.LTPCoef_Q14 {
			psDecCtrl.LTPCoef_Q14[i] = 0
		}
		psDec.indices.PERIndex = 0
		psDecCtrl.LTP_scale_Q14 = 0
	}
}

// BoolToInt converts a boolean condition to 1 (true) or 0 (false)
func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}