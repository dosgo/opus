package concentus

const (
	USE_silk_resampler_copy = iota
	USE_silk_resampler_private_up2_HQ_wrapper
	USE_silk_resampler_private_IIR_FIR
	USE_silk_resampler_private_down_FIR
	ORDER_FIR = 4
)

func rateID(R int) int {
	return ((((R >> 12) - btoi(R > 16000)) >> btoi(R > 24000)) - 1)
}

func silk_resampler_init(S *SilkResamplerState, Fs_Hz_in, Fs_Hz_out, forEnc int) int {
	var up2x int

	// Clear state
	S.Reset()

	// Input checking
	if forEnc != 0 {
		if (Fs_Hz_in != 8000 && Fs_Hz_in != 12000 && Fs_Hz_in != 16000 && Fs_Hz_in != 24000 && Fs_Hz_in != 48000) ||
			(Fs_Hz_out != 8000 && Fs_Hz_out != 12000 && Fs_Hz_out != 16000) {
			return -1
		}
		S.inputDelay = SilkTables.delay_matrix_enc[rateID(Fs_Hz_in)][rateID(Fs_Hz_out)]
	} else {
		if (Fs_Hz_in != 8000 && Fs_Hz_in != 12000 && Fs_Hz_in != 16000) ||
			(Fs_Hz_out != 8000 && Fs_Hz_out != 12000 && Fs_Hz_out != 16000 && Fs_Hz_out != 24000 && Fs_Hz_out != 48000) {
			return -1
		}
		S.inputDelay = SilkTables.delay_matrix_dec[rateID(Fs_Hz_in)][rateID(Fs_Hz_out)]
	}

	S.Fs_in_kHz = Fs_Hz_in / 1000
	S.Fs_out_kHz = Fs_Hz_out / 1000

	// Number of samples processed per batch
	S.batchSize = S.Fs_in_kHz * RESAMPLER_MAX_BATCH_SIZE_MS

	// Find resampler with the right sampling ratio
	up2x = 0
	if Fs_Hz_out > Fs_Hz_in {
		// Upsample
		if Fs_Hz_out == 2*Fs_Hz_in {
			// Fs_out : Fs_in = 2 : 1
			S.resampler_function = USE_silk_resampler_private_up2_HQ_wrapper
		} else {
			// Default resampler
			S.resampler_function = USE_silk_resampler_private_IIR_FIR
			up2x = 1
		}
	} else if Fs_Hz_out < Fs_Hz_in {
		// Downsample
		S.resampler_function = USE_silk_resampler_private_down_FIR
		if 4*Fs_Hz_out == 3*Fs_Hz_in {
			// Fs_out : Fs_in = 3 : 4
			S.FIR_Fracs = 3
			S.FIR_Order = RESAMPLER_DOWN_ORDER_FIR0
			S.Coefs = SilkTables.silk_Resampler_3_4_COEFS
		} else if 3*Fs_Hz_out == 2*Fs_Hz_in {
			// Fs_out : Fs_in = 2 : 3
			S.FIR_Fracs = 2
			S.FIR_Order = RESAMPLER_DOWN_ORDER_FIR0
			S.Coefs = SilkTables.silk_Resampler_2_3_COEFS
		} else if 2*Fs_Hz_out == Fs_Hz_in {
			// Fs_out : Fs_in = 1 : 2
			S.FIR_Fracs = 1
			S.FIR_Order = RESAMPLER_DOWN_ORDER_FIR1
			S.Coefs = SilkTables.silk_Resampler_1_2_COEFS
		} else if 3*Fs_Hz_out == Fs_Hz_in {
			// Fs_out : Fs_in = 1 : 3
			S.FIR_Fracs = 1
			S.FIR_Order = RESAMPLER_DOWN_ORDER_FIR2
			S.Coefs = SilkTables.silk_Resampler_1_3_COEFS
		} else if 4*Fs_Hz_out == Fs_Hz_in {
			// Fs_out : Fs_in = 1 : 4
			S.FIR_Fracs = 1
			S.FIR_Order = RESAMPLER_DOWN_ORDER_FIR2
			S.Coefs = SilkTables.silk_Resampler_1_4_COEFS
		} else if 6*Fs_Hz_out == Fs_Hz_in {
			// Fs_out : Fs_in = 1 : 6
			S.FIR_Fracs = 1
			S.FIR_Order = RESAMPLER_DOWN_ORDER_FIR2
			S.Coefs = SilkTables.silk_Resampler_1_6_COEFS
		} else {
			// None available
			return -1
		}
	} else {
		// Input and output sampling rates are equal: copy
		S.resampler_function = USE_silk_resampler_copy
	}

	// Ratio of input/output samples
	S.invRatio_Q16 = (Fs_Hz_in << (14 + up2x)) / Fs_Hz_out << 2

	// Make sure the ratio is rounded up
	for SMULWW(S.invRatio_Q16, Fs_Hz_out) < (Fs_Hz_in << up2x) {
		S.invRatio_Q16++
	}

	return 0
}

func silk_resampler(S *SilkResamplerState, output []int16, output_ptr int, input []int16, input_ptr int, inLen int) int {
	var nSamples int

	// Need at least 1 ms of input data
	if inLen < S.Fs_in_kHz {
		return SILK_NO_ERROR
	}
	// Delay can't exceed the 1 ms of buffering
	if S.inputDelay > S.Fs_in_kHz {
		return SILK_NO_ERROR
	}

	nSamples = S.Fs_in_kHz - S.inputDelay

	// Copy to delay buffer
	copy(S.delayBuf[S.inputDelay:], input[input_ptr:input_ptr+nSamples])

	switch S.resampler_function {
	case USE_silk_resampler_private_up2_HQ_wrapper:
		silk_resampler_private_up2_HQ(S.sIIR[:], output, output_ptr, S.delayBuf, 0, S.Fs_in_kHz)
		silk_resampler_private_up2_HQ(S.sIIR[:], output, output_ptr+S.Fs_out_kHz, input, input_ptr+nSamples, inLen-S.Fs_in_kHz)
	case USE_silk_resampler_private_IIR_FIR:
		silk_resampler_private_IIR_FIR(S, output, output_ptr, S.delayBuf, 0, S.Fs_in_kHz)
		silk_resampler_private_IIR_FIR(S, output, output_ptr+S.Fs_out_kHz, input, input_ptr+nSamples, inLen-S.Fs_in_kHz)
	case USE_silk_resampler_private_down_FIR:
		silk_resampler_private_down_FIR(S, output, output_ptr, S.delayBuf, 0, S.Fs_in_kHz)
		silk_resampler_private_down_FIR(S, output, output_ptr+S.Fs_out_kHz, input, input_ptr+nSamples, inLen-S.Fs_in_kHz)
	default:
		copy(output[output_ptr:], S.delayBuf[:S.Fs_in_kHz])
		copy(output[output_ptr+S.Fs_out_kHz:], input[input_ptr+nSamples:input_ptr+inLen])
	}

	// Copy to delay buffer
	copy(S.delayBuf[:S.inputDelay], input[input_ptr+inLen-S.inputDelay:input_ptr+inLen])

	return SILK_NO_ERROR
}

func silk_resampler_down2(S []int32, output []int16, input []int16, inLen int) {
	len2 := inLen / 2

	for k := 0; k < len2; k++ {
		// Convert to Q10
		in32 := int32(input[2*k]) << 10

		// All-pass section for even input sample
		Y := in32 - S[0]
		X := SMLAWB(Y, Y, SilkTables.silk_resampler_down2_1)
		out32 := S[0] + X
		S[0] = in32 + X

		// Convert to Q10
		in32 = int32(input[2*k+1]) << 10

		// All-pass section for odd input sample, and add to output of previous section
		Y = in32 - S[1]
		X = SMULWB(Y, SilkTables.silk_resampler_down2_0)
		out32 += S[1]
		out32 += X
		S[1] = in32 + X

		// Add, convert back to int16 and store to output
		output[k] = int16(SAT16(RSHIFT_ROUND(out32, 11)))
	}
}

func silk_resampler_down2_3(S []int32, output []int16, input []int16, inLen int) {
	var nSamplesIn, counter, res_Q6 int
	buf := make([]int32, RESAMPLER_MAX_BATCH_SIZE_IN+ORDER_FIR)
	input_ptr := 0
	output_ptr := 0

	// Copy buffered samples to start of buffer
	copy(buf[:ORDER_FIR], S[:ORDER_FIR])

	// Iterate over blocks of frameSizeIn input samples
	for {
		nSamplesIn = min(inLen, RESAMPLER_MAX_BATCH_SIZE_IN)

		// Second-order AR filter (output in Q8)
		silk_resampler_private_AR2(S, 0, buf, ORDER_FIR, input, input_ptr,
			SilkTables.silk_Resampler_2_3_COEFS_LQ, nSamplesIn)

		// Interpolate filtered signal
		buf_ptr := 0
		counter = nSamplesIn
		for counter > 2 {
			// Inner product
			res_Q6 = SMULWB(buf[buf_ptr], SilkTables.silk_Resampler_2_3_COEFS_LQ[2])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+1], SilkTables.silk_Resampler_2_3_COEFS_LQ[3])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+2], SilkTables.silk_Resampler_2_3_COEFS_LQ[5])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+3], SilkTables.silk_Resampler_2_3_COEFS_LQ[4])

			// Scale down, saturate and store in output array
			output[output_ptr] = int16(SAT16(RSHIFT_ROUND(res_Q6, 6)))
			output_ptr++

			res_Q6 = SMULWB(buf[buf_ptr+1], SilkTables.silk_Resampler_2_3_COEFS_LQ[4])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+2], SilkTables.silk_Resampler_2_3_COEFS_LQ[5])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+3], SilkTables.silk_Resampler_2_3_COEFS_LQ[3])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+4], SilkTables.silk_Resampler_2_3_COEFS_LQ[2])

			// Scale down, saturate and store in output array
			output[output_ptr] = int16(SAT16(RSHIFT_ROUND(res_Q6, 6)))
			output_ptr++

			buf_ptr += 3
			counter -= 3
		}

		input_ptr += nSamplesIn
		inLen -= nSamplesIn

		if inLen > 0 {
			// More iterations to do; copy last part of filtered signal to beginning of buffer
			copy(buf[:ORDER_FIR], buf[nSamplesIn:nSamplesIn+ORDER_FIR])
		} else {
			break
		}
	}

	// Copy last part of filtered signal to the state for the next call
	copy(S[:ORDER_FIR], buf[nSamplesIn:nSamplesIn+ORDER_FIR])
}

func silk_resampler_private_AR2(S []int32, S_ptr int, out_Q8 []int32, out_Q8_ptr int, input []int16, input_ptr int, A_Q14 []int16, len int) {
	for k := 0; k < len; k++ {
		out32 := S[S_ptr] + (int32(input[input_ptr+k]) << 8)
		out_Q8[out_Q8_ptr+k] = out32
		out32 <<= 2
		S[S_ptr] = S[S_ptr+1] + SMLAWB(0, out32, A_Q14[0])
		S[S_ptr+1] = SMULWB(out32, A_Q14[1])
	}
}

func silk_resampler_private_down_FIR_INTERPOL(output []int16, output_ptr int, buf []int32, FIR_Coefs []int16, FIR_Coefs_ptr, FIR_Order, FIR_Fracs, max_index_Q16, index_increment_Q16 int) int {
	switch FIR_Order {
	case RESAMPLER_DOWN_ORDER_FIR0:
		for index_Q16 := 0; index_Q16 < max_index_Q16; index_Q16 += index_increment_Q16 {
			// Integer part gives pointer to buffered input
			buf_ptr := index_Q16 >> 16

			// Fractional part gives interpolation coefficients
			interpol_ind := SMULWB(index_Q16&0xFFFF, FIR_Fracs)

			// Inner product
			interpol_ptr := FIR_Coefs_ptr + (RESAMPLER_DOWN_ORDER_FIR0/2)*interpol_ind
			res_Q6 := SMULWB(buf[buf_ptr+0], FIR_Coefs[interpol_ptr+0])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+1], FIR_Coefs[interpol_ptr+1])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+2], FIR_Coefs[interpol_ptr+2])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+3], FIR_Coefs[interpol_ptr+3])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+4], FIR_Coefs[interpol_ptr+4])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+5], FIR_Coefs[interpol_ptr+5])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+6], FIR_Coefs[interpol_ptr+6])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+7], FIR_Coefs[interpol_ptr+7])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+8], FIR_Coefs[interpol_ptr+8])
			interpol_ptr = FIR_Coefs_ptr + (RESAMPLER_DOWN_ORDER_FIR0/2)*(FIR_Fracs-1-interpol_ind)
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+17], FIR_Coefs[interpol_ptr+0])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+16], FIR_Coefs[interpol_ptr+1])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+15], FIR_Coefs[interpol_ptr+2])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+14], FIR_Coefs[interpol_ptr+3])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+13], FIR_Coefs[interpol_ptr+4])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+12], FIR_Coefs[interpol_ptr+5])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+11], FIR_Coefs[interpol_ptr+6])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+10], FIR_Coefs[interpol_ptr+7])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+9], FIR_Coefs[interpol_ptr+8])

			// Scale down, saturate and store in output array
			output[output_ptr] = int16(SAT16(RSHIFT_ROUND(res_Q6, 6)))
			output_ptr++
		}
	case RESAMPLER_DOWN_ORDER_FIR1:
		for index_Q16 := 0; index_Q16 < max_index_Q16; index_Q16 += index_increment_Q16 {
			buf_ptr := index_Q16 >> 16

			res_Q6 := SMULWB(buf[buf_ptr+0]+buf[buf_ptr+23], FIR_Coefs[FIR_Coefs_ptr+0])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+1]+buf[buf_ptr+22], FIR_Coefs[FIR_Coefs_ptr+1])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+2]+buf[buf_ptr+21], FIR_Coefs[FIR_Coefs_ptr+2])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+3]+buf[buf_ptr+20], FIR_Coefs[FIR_Coefs_ptr+3])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+4]+buf[buf_ptr+19], FIR_Coefs[FIR_Coefs_ptr+4])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+5]+buf[buf_ptr+18], FIR_Coefs[FIR_Coefs_ptr+5])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+6]+buf[buf_ptr+17], FIR_Coefs[FIR_Coefs_ptr+6])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+7]+buf[buf_ptr+16], FIR_Coefs[FIR_Coefs_ptr+7])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+8]+buf[buf_ptr+15], FIR_Coefs[FIR_Coefs_ptr+8])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+9]+buf[buf_ptr+14], FIR_Coefs[FIR_Coefs_ptr+9])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+10]+buf[buf_ptr+13], FIR_Coefs[FIR_Coefs_ptr+10])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+11]+buf[buf_ptr+12], FIR_Coefs[FIR_Coefs_ptr+11])

			output[output_ptr] = int16(SAT16(RSHIFT_ROUND(res_Q6, 6)))
			output_ptr++
		}
	case RESAMPLER_DOWN_ORDER_FIR2:
		for index_Q16 := 0; index_Q16 < max_index_Q16; index_Q16 += index_increment_Q16 {
			buf_ptr := index_Q16 >> 16

			res_Q6 := SMULWB(buf[buf_ptr+0]+buf[buf_ptr+35], FIR_Coefs[FIR_Coefs_ptr+0])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+1]+buf[buf_ptr+34], FIR_Coefs[FIR_Coefs_ptr+1])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+2]+buf[buf_ptr+33], FIR_Coefs[FIR_Coefs_ptr+2])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+3]+buf[buf_ptr+32], FIR_Coefs[FIR_Coefs_ptr+3])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+4]+buf[buf_ptr+31], FIR_Coefs[FIR_Coefs_ptr+4])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+5]+buf[buf_ptr+30], FIR_Coefs[FIR_Coefs_ptr+5])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+6]+buf[buf_ptr+29], FIR_Coefs[FIR_Coefs_ptr+6])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+7]+buf[buf_ptr+28], FIR_Coefs[FIR_Coefs_ptr+7])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+8]+buf[buf_ptr+27], FIR_Coefs[FIR_Coefs_ptr+8])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+9]+buf[buf_ptr+26], FIR_Coefs[FIR_Coefs_ptr+9])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+10]+buf[buf_ptr+25], FIR_Coefs[FIR_Coefs_ptr+10])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+11]+buf[buf_ptr+24], FIR_Coefs[FIR_Coefs_ptr+11])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+12]+buf[buf_ptr+23], FIR_Coefs[FIR_Coefs_ptr+12])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+13]+buf[buf_ptr+22], FIR_Coefs[FIR_Coefs_ptr+13])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+14]+buf[buf_ptr+21], FIR_Coefs[FIR_Coefs_ptr+14])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+15]+buf[buf_ptr+20], FIR_Coefs[FIR_Coefs_ptr+15])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+16]+buf[buf_ptr+19], FIR_Coefs[FIR_Coefs_ptr+16])
			res_Q6 = SMLAWB(res_Q6, buf[buf_ptr+17]+buf[buf_ptr+18], FIR_Coefs[FIR_Coefs_ptr+17])

			output[output_ptr] = int16(SAT16(RSHIFT_ROUND(res_Q6, 6)))
			output_ptr++
		}
	default:
		return output_ptr
	}

	return output_ptr
}

func silk_resampler_private_down_FIR(S *SilkResamplerState, output []int16, output_ptr int, input []int16, input_ptr int, inLen int) {
	var nSamplesIn int
	var max_index_Q16, index_increment_Q16 int32
	buf := make([]int32, S.batchSize+S.FIR_Order)

	// Copy buffered samples to start of buffer
	copy(buf[:S.FIR_Order], S.sFIR_i32[:S.FIR_Order])

	// Iterate over blocks of frameSizeIn input samples
	index_increment_Q16 = S.invRatio_Q16
	for {
		nSamplesIn = min(inLen, S.batchSize)

		// Second-order AR filter (output in Q8)
		silk_resampler_private_AR2(S.sIIR[:], 0, buf, S.FIR_Order, input, input_ptr, S.Coefs, nSamplesIn)

		max_index_Q16 = int32(nSamplesIn) << 16

		// Interpolate filtered signal
		output_ptr = silk_resampler_private_down_FIR_INTERPOL(output, output_ptr, buf, S.Coefs, 2, S.FIR_Order,
			S.FIR_Fracs, max_index_Q16, index_increment_Q16)

		input_ptr += nSamplesIn
		inLen -= nSamplesIn

		if inLen > 1 {
			// More iterations to do; copy last part of filtered signal to beginning of buffer
			copy(buf[:S.FIR_Order], buf[nSamplesIn:nSamplesIn+S.FIR_Order])
		} else {
			break
		}
	}

	// Copy last part of filtered signal to the state for the next call
	copy(S.sFIR_i32[:S.FIR_Order], buf[nSamplesIn:nSamplesIn+S.FIR_Order])
}

func silk_resampler_private_IIR_FIR_INTERPOL(output []int16, output_ptr int, buf []int16, max_index_Q16, index_increment_Q16 int32) int {
	for index_Q16 := int32(0); index_Q16 < max_index_Q16; index_Q16 += index_increment_Q16 {
		table_index := SMULWB(index_Q16&0xFFFF, 12)
		buf_ptr := index_Q16 >> 16

		res_Q15 := SMULBB(int32(buf[buf_ptr]), int32(SilkTables.silk_resampler_frac_FIR_12[table_index][0]))
		res_Q15 = SMLABB(res_Q15, int32(buf[buf_ptr+1]), int32(SilkTables.silk_resampler_frac_FIR_12[table_index][1]))
		res_Q15 = SMLABB(res_Q15, int32(buf[buf_ptr+2]), int32(SilkTables.silk_resampler_frac_FIR_12[table_index][2]))
		res_Q15 = SMLABB(res_Q15, int32(buf[buf_ptr+3]), int32(SilkTables.silk_resampler_frac_FIR_12[table_index][3]))
		res_Q15 = SMLABB(res_Q15, int32(buf[buf_ptr+4]), int32(SilkTables.silk_resampler_frac_FIR_12[11-table_index][3]))
		res_Q15 = SMLABB(res_Q15, int32(buf[buf_ptr+5]), int32(SilkTables.silk_resampler_frac_FIR_12[11-table_index][2]))
		res_Q15 = SMLABB(res_Q15, int32(buf[buf_ptr+6]), int32(SilkTables.silk_resampler_frac_FIR_12[11-table_index][1]))
		res_Q15 = SMLABB(res_Q15, int32(buf[buf_ptr+7]), int32(SilkTables.silk_resampler_frac_FIR_12[11-table_index][0]))

		output[output_ptr] = int16(SAT16(RSHIFT_ROUND(res_Q15, 15)))
		output_ptr++
	}
	return output_ptr
}

func silk_resampler_private_IIR_FIR(S *SilkResamplerState, output []int16, output_ptr int, input []int16, input_ptr int, inLen int) {
	var nSamplesIn int
	var max_index_Q16, index_increment_Q16 int32
	buf := make([]int16, 2*S.batchSize+RESAMPLER_ORDER_FIR_12)

	// Copy buffered samples to start of buffer
	copy(buf[:RESAMPLER_ORDER_FIR_12], S.sFIR_i16[:RESAMPLER_ORDER_FIR_12])

	// Iterate over blocks of frameSizeIn input samples
	index_increment_Q16 = S.invRatio_Q16
	for {
		nSamplesIn = min(inLen, S.batchSize)

		// Upsample 2x
		silk_resampler_private_up2_HQ(S.sIIR[:], buf, RESAMPLER_ORDER_FIR_12, input, input_ptr, nSamplesIn)

		max_index_Q16 = int32(nSamplesIn) << (16 + 1) // +1 because 2x upsampling
		output_ptr = silk_resampler_private_IIR_FIR_INTERPOL(output, output_ptr, buf, max_index_Q16, index_increment_Q16)
		input_ptr += nSamplesIn
		inLen -= nSamplesIn

		if inLen > 0 {
			// More iterations to do; copy last part of filtered signal to beginning of buffer
			copy(buf[:RESAMPLER_ORDER_FIR_12], buf[nSamplesIn<<1:(nSamplesIn<<1)+RESAMPLER_ORDER_FIR_12])
		} else {
			break
		}
	}

	// Copy last part of filtered signal to the state for the next call
	copy(S.sFIR_i16[:RESAMPLER_ORDER_FIR_12], buf[nSamplesIn<<1:(nSamplesIn<<1)+RESAMPLER_ORDER_FIR_12])
}

func silk_resampler_private_up2_HQ(S []int32, output []int16, output_ptr int, input []int16, input_ptr int, len int) {
	for k := 0; k < len; k++ {
		// Convert to Q10
		in32 := int32(input[input_ptr+k]) << 10

		// First all-pass section for even output sample
		Y := in32 - S[0]
		X := SMULWB(Y, SilkTables.silk_resampler_up2_hq_0[0])
		out32_1 := S[0] + X
		S[0] = in32 + X

		// Second all-pass section for even output sample
		Y = out32_1 - S[1]
		X = SMULWB(Y, SilkTables.silk_resampler_up2_hq_0[1])
		out32_2 := S[1] + X
		S[1] = out32_1 + X

		// Third all-pass section for even output sample
		Y = out32_2 - S[2]
		X = SMLAWB(Y, Y, SilkTables.silk_resampler_up2_hq_0[2])
		out32_1 = S[2] + X
		S[2] = out32_2 + X

		// Apply gain in Q15, convert back to int16 and store to output
		output[output_ptr+(2*k)] = int16(SAT16(RSHIFT_ROUND(out32_1, 10)))

		// First all-pass section for odd output sample
		Y = in32 - S[3]
		X = SMULWB(Y, SilkTables.silk_resampler_up2_hq_1[0])
		out32_1 = S[3] + X
		S[3] = in32 + X

		// Second all-pass section for odd output sample
		Y = out32_1 - S[4]
		X = SMULWB(Y, SilkTables.silk_resampler_up2_hq_1[1])
		out32_2 = S[4] + X
		S[4] = out32_1 + X

		// Third all-pass section for odd output sample
		Y = out32_2 - S[5]
		X = SMLAWB(Y, Y, SilkTables.silk_resampler_up2_hq_1[2])
		out32_1 = S[5] + X
		S[5] = out32_2 + X

		// Apply gain in Q15, convert back to int16 and store to output
		output[output_ptr+(2*k)+1] = int16(SAT16(RSHIFT_ROUND(out32_1, 10)))
	}
}

// Helper functions
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
