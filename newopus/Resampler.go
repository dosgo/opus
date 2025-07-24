package opus

const (
	USE_silk_resampler_copy                   = 0
	USE_silk_resampler_private_up2_HQ_wrapper = 1
	USE_silk_resampler_private_IIR_FIR        = 2
	USE_silk_resampler_private_down_FIR       = 3
	ORDER_FIR                                 = 4
)

func rateID(R int) int {
	v := R >> 12
	if R > 16000 {
		v--
	}
	if R > 24000 {
		v >>= 1
	}
	return v - 1
}

func silk_resampler_init(S *SilkResamplerState, Fs_Hz_in int, Fs_Hz_out int, forEnc int) int {
	S.Reset()

	if forEnc != 0 {
		if (Fs_Hz_in != 8000 && Fs_Hz_in != 12000 && Fs_Hz_in != 16000 && Fs_Hz_in != 24000 && Fs_Hz_in != 48000) ||
			(Fs_Hz_out != 8000 && Fs_Hz_out != 12000 && Fs_Hz_out != 16000) {
			OpusAssert(false)
			return -1
		}
		S.inputDelay = delay_matrix_enc[rateID(Fs_Hz_in)][rateID(Fs_Hz_out)]
	} else {
		if (Fs_Hz_in != 8000 && Fs_Hz_in != 12000 && Fs_Hz_in != 16000) ||
			(Fs_Hz_out != 8000 && Fs_Hz_out != 12000 && Fs_Hz_out != 16000 && Fs_Hz_out != 24000 && Fs_Hz_out != 48000) {
			OpusAssert(false)
			return -1
		}
		S.inputDelay = delay_matrix_dec[rateID(Fs_Hz_in)][rateID(Fs_Hz_out)]
	}

	S.Fs_in_kHz = silk_DIV32_16(Fs_Hz_in, 1000)
	S.Fs_out_kHz = silk_DIV32_16(Fs_Hz_out, 1000)
	S.batchSize = S.Fs_in_kHz * RESAMPLER_MAX_BATCH_SIZE_MS

	up2x := 0
	if Fs_Hz_out > Fs_Hz_in {
		if Fs_Hz_out == silk_MUL(Fs_Hz_in, 2) {
			S.resampler_function = USE_silk_resampler_private_up2_HQ_wrapper
		} else {
			S.resampler_function = USE_silk_resampler_private_IIR_FIR
			up2x = 1
		}
	} else if Fs_Hz_out < Fs_Hz_in {
		S.resampler_function = USE_silk_resampler_private_down_FIR
		if silk_MUL(Fs_Hz_out, 4) == silk_MUL(Fs_Hz_in, 3) {
			S.FIR_Fracs = 3
			S.FIR_Order = RESAMPLER_DOWN_ORDER_FIR0
			S.Coefs = silk_Resampler_3_4_COEFS
		} else if silk_MUL(Fs_Hz_out, 3) == silk_MUL(Fs_Hz_in, 2) {
			S.FIR_Fracs = 2
			S.FIR_Order = RESAMPLER_DOWN_ORDER_FIR0
			S.Coefs = silk_Resampler_2_3_COEFS
		} else if silk_MUL(Fs_Hz_out, 2) == Fs_Hz_in {
			S.FIR_Fracs = 1
			S.FIR_Order = RESAMPLER_DOWN_ORDER_FIR1
			S.Coefs = silk_Resampler_1_2_COEFS
		} else if silk_MUL(Fs_Hz_out, 3) == Fs_Hz_in {
			S.FIR_Fracs = 1
			S.FIR_Order = RESAMPLER_DOWN_ORDER_FIR2
			S.Coefs = silk_Resampler_1_3_COEFS
		} else if silk_MUL(Fs_Hz_out, 4) == Fs_Hz_in {
			S.FIR_Fracs = 1
			S.FIR_Order = RESAMPLER_DOWN_ORDER_FIR2
			S.Coefs = silk_Resampler_1_4_COEFS
		} else if silk_MUL(Fs_Hz_out, 6) == Fs_Hz_in {
			S.FIR_Fracs = 1
			S.FIR_Order = RESAMPLER_DOWN_ORDER_FIR2
			S.Coefs = silk_Resampler_1_6_COEFS
		} else {
			OpusAssert(false)
			return -1
		}
	} else {
		S.resampler_function = USE_silk_resampler_copy
	}

	S.invRatio_Q16 = silk_LSHIFT32(silk_DIV32(silk_LSHIFT32(Fs_Hz_in, 14+up2x), Fs_Hz_out), 2)
	for silk_SMULWW(S.invRatio_Q16, Fs_Hz_out) < silk_LSHIFT32(Fs_Hz_in, up2x) {
		S.invRatio_Q16++
	}
	return 0
}

func silk_resampler(S *SilkResamplerState, output []int16, output_ptr int, input []int16, input_ptr int, inLen int) int {
	OpusAssert(inLen >= S.Fs_in_kHz)
	OpusAssert(S.inputDelay <= S.Fs_in_kHz)

	nSamples := S.Fs_in_kHz - S.inputDelay
	delayBufPtr := S.delayBuf

	copy(delayBufPtr[S.inputDelay:S.inputDelay+nSamples], input[input_ptr:input_ptr+nSamples])

	switch S.resampler_function {
	case USE_silk_resampler_private_up2_HQ_wrapper:
		silk_resampler_private_up2_HQ(S.sIIR[:], output, output_ptr, delayBufPtr, 0, S.Fs_in_kHz)
		silk_resampler_private_up2_HQ(S.sIIR[:], output, output_ptr+S.Fs_out_kHz, input, input_ptr+nSamples, inLen-S.Fs_in_kHz)
	case USE_silk_resampler_private_IIR_FIR:
		silk_resampler_private_IIR_FIR(S, output, output_ptr, delayBufPtr, 0, S.Fs_in_kHz)
		silk_resampler_private_IIR_FIR(S, output, output_ptr+S.Fs_out_kHz, input, input_ptr+nSamples, inLen-S.Fs_in_kHz)
	case USE_silk_resampler_private_down_FIR:
		silk_resampler_private_down_FIR(S, output, output_ptr, delayBufPtr, 0, S.Fs_in_kHz)
		silk_resampler_private_down_FIR(S, output, output_ptr+S.Fs_out_kHz, input, input_ptr+nSamples, inLen-S.Fs_in_kHz)
	default:
		copy(output[output_ptr:output_ptr+S.Fs_in_kHz], delayBufPtr[:S.Fs_in_kHz])
		copy(output[output_ptr+S.Fs_out_kHz:output_ptr+S.Fs_out_kHz+(inLen-S.Fs_in_kHz)], input[input_ptr+nSamples:input_ptr+nSamples+(inLen-S.Fs_in_kHz)])
	}

	copy(delayBufPtr[:S.inputDelay], input[input_ptr+inLen-S.inputDelay:input_ptr+inLen])
	return SILK_NO_ERROR
}

func silk_resampler_down2(S []int32, output []int16, input []int16, inLen int) {
	len2 := silk_RSHIFT32(inLen, 1)
	OpusAssert(silk_resampler_down2_0 > 0)
	OpusAssert(silk_resampler_down2_1 < 0)

	for k := 0; k < len2; k++ {
		in32 := silk_LSHIFT(int32(input[2*k]), 10)
		Y := silk_SUB32(in32, S[0])
		X := silk_SMLAWB(Y, Y, silk_resampler_down2_1)
		out32 := silk_ADD32(S[0], X)
		S[0] = silk_ADD32(in32, X)

		in32 = silk_LSHIFT(int32(input[2*k+1]), 10)
		Y = silk_SUB32(in32, S[1])
		X = silk_SMULWB(Y, silk_resampler_down2_0)
		out32 = silk_ADD32(out32, S[1])
		out32 = silk_ADD32(out32, X)
		S[1] = silk_ADD32(in32, X)

		output[k] = int16(silk_SAT16(silk_RSHIFT_ROUND(out32, 11)))
	}
}

func silk_resampler_down2_3(S []int32, output []int16, input []int16, inLen int) {
	buf := make([]int32, RESAMPLER_MAX_BATCH_SIZE_IN+ORDER_FIR)
	input_ptr := 0
	output_ptr := 0

	copy(buf[:ORDER_FIR], S[:ORDER_FIR])

	for {
		nSamplesIn := silk_min(inLen, RESAMPLER_MAX_BATCH_SIZE_IN)
		silk_resampler_private_AR2(S, 0, buf, ORDER_FIR, input, input_ptr, silk_Resampler_2_3_COEFS_LQ, nSamplesIn)

		buf_ptr := 0
		counter := nSamplesIn
		for counter > 2 {
			res_Q6 := silk_SMULWB(buf[buf_ptr], silk_Resampler_2_3_COEFS_LQ[2])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+1], silk_Resampler_2_3_COEFS_LQ[3])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+2], silk_Resampler_2_3_COEFS_LQ[5])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+3], silk_Resampler_2_3_COEFS_LQ[4])
			output[output_ptr] = int16(silk_SAT16(silk_RSHIFT_ROUND(res_Q6, 6)))
			output_ptr++

			res_Q6 = silk_SMULWB(buf[buf_ptr+1], silk_Resampler_2_3_COEFS_LQ[4])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+2], silk_Resampler_2_3_COEFS_LQ[5])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+3], silk_Resampler_2_3_COEFS_LQ[3])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+4], silk_Resampler_2_3_COEFS_LQ[2])
			output[output_ptr] = int16(silk_SAT16(silk_RSHIFT_ROUND(res_Q6, 6)))
			output_ptr++

			buf_ptr += 3
			counter -= 3
		}

		input_ptr += nSamplesIn
		inLen -= nSamplesIn

		if inLen > 0 {
			copy(buf[:ORDER_FIR], buf[nSamplesIn:nSamplesIn+ORDER_FIR])
		} else {
			break
		}
	}

	copy(S[:ORDER_FIR], buf[nSamplesIn:nSamplesIn+ORDER_FIR])
}

func silk_resampler_private_AR2(S []int32, S_ptr int, out_Q8 []int32, out_Q8_ptr int, input []int16, input_ptr int, A_Q14 []int16, len int) {
	for k := 0; k < len; k++ {
		out32 := silk_ADD_LSHIFT32(S[S_ptr], int32(input[input_ptr+k]), 8)
		out_Q8[out_Q8_ptr+k] = out32
		out32 = silk_LSHIFT(out32, 2)
		S[S_ptr] = silk_SMLAWB(S[S_ptr+1], out32, int32(A_Q14[0]))
		S[S_ptr+1] = silk_SMULWB(out32, int32(A_Q14[1]))
	}
}

func silk_resampler_private_down_FIR_INTERPOL(output []int16, output_ptr int, buf []int32, FIR_Coefs []int16, FIR_Coefs_ptr int, FIR_Order int, FIR_Fracs int, max_index_Q16 int, index_increment_Q16 int) int {
	switch FIR_Order {
	case RESAMPLER_DOWN_ORDER_FIR0:
		for index_Q16 := 0; index_Q16 < max_index_Q16; index_Q16 += index_increment_Q16 {
			buf_ptr := silk_RSHIFT(index_Q16, 16)
			interpol_ind := silk_SMULWB(index_Q16&0xFFFF, FIR_Fracs)
			interpol_ptr := FIR_Coefs_ptr + (RESAMPLER_DOWN_ORDER_FIR0/2)*interpol_ind
			res_Q6 := silk_SMULWB(buf[buf_ptr+0], FIR_Coefs[interpol_ptr+0])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+1], FIR_Coefs[interpol_ptr+1])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+2], FIR_Coefs[interpol_ptr+2])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+3], FIR_Coefs[interpol_ptr+3])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+4], FIR_Coefs[interpol_ptr+4])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+5], FIR_Coefs[interpol_ptr+5])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+6], FIR_Coefs[interpol_ptr+6])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+7], FIR_Coefs[interpol_ptr+7])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+8], FIR_Coefs[interpol_ptr+8])
			interpol_ptr = FIR_Coefs_ptr + (RESAMPLER_DOWN_ORDER_FIR0/2)*(FIR_Fracs-1-interpol_ind)
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+17], FIR_Coefs[interpol_ptr+0])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+16], FIR_Coefs[interpol_ptr+1])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+15], FIR_Coefs[interpol_ptr+2])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+14], FIR_Coefs[interpol_ptr+3])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+13], FIR_Coefs[interpol_ptr+4])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+12], FIR_Coefs[interpol_ptr+5])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+11], FIR_Coefs[interpol_ptr+6])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+10], FIR_Coefs[interpol_ptr+7])
			res_Q6 = silk_SMLAWB(res_Q6, buf[buf_ptr+9], FIR_Coefs[interpol_ptr+8])
			output[output_ptr] = int16(silk_SAT16(silk_RSHIFT_ROUND(res_Q6, 6)))
			output_ptr++
		}
	case RESAMPLER_DOWN_ORDER_FIR1:
		for index_Q16 := 0; index_Q16 < max_index_Q16; index_Q16 += index_increment_Q16 {
			buf_ptr := silk_RSHIFT(index_Q16, 16)
			res_Q6 := silk_SMULWB(silk_ADD32(buf[buf_ptr+0], buf[buf_ptr+23]), FIR_Coefs[FIR_Coefs_ptr+0])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+1], buf[buf_ptr+22]), FIR_Coefs[FIR_Coefs_ptr+1])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+2], buf[buf_ptr+21]), FIR_Coefs[FIR_Coefs_ptr+2])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+3], buf[buf_ptr+20]), FIR_Coefs[FIR_Coefs_ptr+3])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+4], buf[buf_ptr+19]), FIR_Coefs[FIR_Coefs_ptr+4])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+5], buf[buf_ptr+18]), FIR_Coefs[FIR_Coefs_ptr+5])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+6], buf[buf_ptr+17]), FIR_Coefs[FIR_Coefs_ptr+6])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+7], buf[buf_ptr+16]), FIR_Coefs[FIR_Coefs_ptr+7])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+8], buf[buf_ptr+15]), FIR_Coefs[FIR_Coefs_ptr+8])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+9], buf[buf_ptr+14]), FIR_Coefs[FIR_Coefs_ptr+9])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+10], buf[buf_ptr+13]), FIR_Coefs[FIR_Coefs_ptr+10])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+11], buf[buf_ptr+12]), FIR_Coefs[FIR_Coefs_ptr+11])
			output[output_ptr] = int16(silk_SAT16(silk_RSHIFT_ROUND(res_Q6, 6)))
			output_ptr++
		}
	case RESAMPLER_DOWN_ORDER_FIR2:
		for index_Q16 := 0; index_Q16 < max_index_Q16; index_Q16 += index_increment_Q16 {
			buf_ptr := silk_RSHIFT(index_Q16, 16)
			res_Q6 := silk_SMULWB(silk_ADD32(buf[buf_ptr+0], buf[buf_ptr+35]), FIR_Coefs[FIR_Coefs_ptr+0])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+1], buf[buf_ptr+34]), FIR_Coefs[FIR_Coefs_ptr+1])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+2], buf[buf_ptr+33]), FIR_Coefs[FIR_Coefs_ptr+2])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+3], buf[buf_ptr+32]), FIR_Coefs[FIR_Coefs_ptr+3])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+4], buf[buf_ptr+31]), FIR_Coefs[FIR_Coefs_ptr+4])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+5], buf[buf_ptr+30]), FIR_Coefs[FIR_Coefs_ptr+5])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+6], buf[buf_ptr+29]), FIR_Coefs[FIR_Coefs_ptr+6])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+7], buf[buf_ptr+28]), FIR_Coefs[FIR_Coefs_ptr+7])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+8], buf[buf_ptr+27]), FIR_Coefs[FIR_Coefs_ptr+8])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+9], buf[buf_ptr+26]), FIR_Coefs[FIR_Coefs_ptr+9])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+10], buf[buf_ptr+25]), FIR_Coefs[FIR_Coefs_ptr+10])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+11], buf[buf_ptr+24]), FIR_Coefs[FIR_Coefs_ptr+11])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+12], buf[buf_ptr+23]), FIR_Coefs[FIR_Coefs_ptr+12])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+13], buf[buf_ptr+22]), FIR_Coefs[FIR_Coefs_ptr+13])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+14], buf[buf_ptr+21]), FIR_Coefs[FIR_Coefs_ptr+14])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+15], buf[buf_ptr+20]), FIR_Coefs[FIR_Coefs_ptr+15])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+16], buf[buf_ptr+19]), FIR_Coefs[FIR_Coefs_ptr+16])
			res_Q6 = silk_SMLAWB(res_Q6, silk_ADD32(buf[buf_ptr+17], buf[buf_ptr+18]), FIR_Coefs[FIR_Coefs_ptr+17])
			output[output_ptr] = int16(silk_SAT16(silk_RSHIFT_ROUND(res_Q6, 6)))
			output_ptr++
		}
	default:
		OpusAssert(false)
	}
	return output_ptr
}

func silk_resampler_private_down_FIR(S *SilkResamplerState, output []int16, output_ptr int, input []int16, input_ptr int, inLen int) {
	buf := make([]int32, S.batchSize+S.FIR_Order)
	copy(buf[:S.FIR_Order], S.sFIR_i32[:S.FIR_Order])

	index_increment_Q16 := S.invRatio_Q16
	for {
		nSamplesIn := silk_min(inLen, S.batchSize)
		silk_resampler_private_AR2(S.sIIR[:], 0, buf, S.FIR_Order, input, input_ptr, S.Coefs, nSamplesIn)

		max_index_Q16 := silk_LSHIFT32(nSamplesIn, 16)
		output_ptr = silk_resampler_private_down_FIR_INTERPOL(output, output_ptr, buf, S.Coefs, 2, S.FIR_Order, S.FIR_Fracs, max_index_Q16, index_increment_Q16)

		input_ptr += nSamplesIn
		inLen -= nSamplesIn

		if inLen > 1 {
			copy(buf[:S.FIR_Order], buf[nSamplesIn:nSamplesIn+S.FIR_Order])
		} else {
			break
		}
	}

	copy(S.sFIR_i32[:S.FIR_Order], buf[nSamplesIn:nSamplesIn+S.FIR_Order])
}

func silk_resampler_private_IIR_FIR_INTERPOL(output []int16, output_ptr int, buf []int16, max_index_Q16 int, index_increment_Q16 int) int {
	for index_Q16 := 0; index_Q16 < max_index_Q16; index_Q16 += index_increment_Q16 {
		table_index := silk_SMULWB(index_Q16&0xFFFF, 12)
		buf_ptr := index_Q16 >> 16

		res_Q15 := silk_SMULBB(int32(buf[buf_ptr]), int32(silk_resampler_frac_FIR_12[table_index][0]))
		res_Q15 = silk_SMLABB(res_Q15, int32(buf[buf_ptr+1]), int32(silk_resampler_frac_FIR_12[table_index][1]))
		res_Q15 = silk_SMLABB(res_Q15, int32(buf[buf_ptr+2]), int32(silk_resampler_frac_FIR_12[table_index][2]))
		res_Q15 = silk_SMLABB(res_Q15, int32(buf[buf_ptr+3]), int32(silk_resampler_frac_FIR_12[table_index][3]))
		res_Q15 = silk_SMLABB(res_Q15, int32(buf[buf_ptr+4]), int32(silk_resampler_frac_FIR_12[11-table_index][3]))
		res_Q15 = silk_SMLABB(res_Q15, int32(buf[buf_ptr+5]), int32(silk_resampler_frac_FIR_12[11-table_index][2]))
		res_Q15 = silk_SMLABB(res_Q15, int32(buf[buf_ptr+6]), int32(silk_resampler_frac_FIR_12[11-table_index][1]))
		res_Q15 = silk_SMLABB(res_Q15, int32(buf[buf_ptr+7]), int32(silk_resampler_frac_FIR_12[11-table_index][0]))
		output[output_ptr] = int16(silk_SAT16(silk_RSHIFT_ROUND(res_Q15, 15)))
		output_ptr++
	}
	return output_ptr
}

func silk_resampler_private_IIR_FIR(S *SilkResamplerState, output []int16, output_ptr int, input []int16, input_ptr int, inLen int) {
	buf := make([]int16, 2*S.batchSize+RESAMPLER_ORDER_FIR_12)
	copy(buf[:RESAMPLER_ORDER_FIR_12], S.sFIR_i16[:RESAMPLER_ORDER_FIR_12])

	index_increment_Q16 := S.invRatio_Q16
	for {
		nSamplesIn := silk_min(inLen, S.batchSize)
		silk_resampler_private_up2_HQ(S.sIIR[:], buf, RESAMPLER_ORDER_FIR_12, input, input_ptr, nSamplesIn)

		max_index_Q16 := silk_LSHIFT32(nSamplesIn, 16+1)
		output_ptr = silk_resampler_private_IIR_FIR_INTERPOL(output, output_ptr, buf, max_index_Q16, index_increment_Q16)
		input_ptr += nSamplesIn
		inLen -= nSamplesIn

		if inLen > 0 {
			copy(buf[:RESAMPLER_ORDER_FIR_12], buf[nSamplesIn<<1:nSamplesIn<<1+RESAMPLER_ORDER_FIR_12])
		} else {
			break
		}
	}

	copy(S.sFIR_i16[:RESAMPLER_ORDER_FIR_12], buf[nSamplesIn<<1:nSamplesIn<<1+RESAMPLER_ORDER_FIR_12])
}

func silk_resampler_private_up2_HQ(S []int32, output []int16, output_ptr int, input []int16, input_ptr int, len int) {
	OpusAssert(silk_resampler_up2_hq_0[0] > 0)
	OpusAssert(silk_resampler_up2_hq_0[1] > 0)
	OpusAssert(silk_resampler_up2_hq_0[2] < 0)
	OpusAssert(silk_resampler_up2_hq_1[0] > 0)
	OpusAssert(silk_resampler_up2_hq_1[1] > 0)
	OpusAssert(silk_resampler_up2_hq_1[2] < 0)

	for k := 0; k < len; k++ {
		in32 := silk_LSHIFT(int32(input[input_ptr+k]), 10)
		Y := silk_SUB32(in32, S[0])
		X := silk_SMULWB(Y, silk_resampler_up2_hq_0[0])
		out32_1 := silk_ADD32(S[0], X)
		S[0] = silk_ADD32(in32, X)

		Y = silk_SUB32(out32_1, S[1])
		X = silk_SMULWB(Y, silk_resampler_up2_hq_0[1])
		out32_2 := silk_ADD32(S[1], X)
		S[1] = silk_ADD32(out32_1, X)

		Y = silk_SUB32(out32_2, S[2])
		X = silk_SMLAWB(Y, Y, silk_resampler_up2_hq_0[2])
		out32_1 = silk_ADD32(S[2], X)
		S[2] = silk_ADD32(out32_2, X)
		output[output_ptr+2*k] = int16(silk_SAT16(silk_RSHIFT_ROUND(out32_1, 10)))

		Y = silk_SUB32(in32, S[3])
		X = silk_SMULWB(Y, silk_resampler_up2_hq_1[0])
		out32_1 = silk_ADD32(S[3], X)
		S[3] = silk_ADD32(in32, X)

		Y = silk_SUB32(out32_1, S[4])
		X = silk_SMULWB(Y, silk_resampler_up2_hq_1[1])
		out32_2 = silk_ADD32(S[4], X)
		S[4] = silk_ADD32(out32_1, X)

		Y = silk_SUB32(out32_2, S[5])
		X = silk_SMLAWB(Y, Y, silk_resampler_up2_hq_1[2])
		out32_1 = silk_ADD32(S[5], X)
		S[5] = silk_ADD32(out32_2, X)
		output[output_ptr+2*k+1] = int16(silk_SAT16(silk_RSHIFT_ROUND(out32_1, 10)))
	}
}
