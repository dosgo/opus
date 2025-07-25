package opus

func silk_InitDecoder(decState *SilkDecoder) int {
	decState.Reset()
	ret := SilkError.SILK_NO_ERROR
	channel_states := decState.channel_state
	for n := 0; n < DECODER_NUM_CHANNELS; n++ {
		ret = channel_states[n].silk_init_decoder()
	}
	decState.sStereo.Reset()
	decState.prev_decode_only_middle = 0
	return ret
}

func silk_Decode(
	psDec *SilkDecoder,
	decControl *DecControlState,
	lostFlag int,
	newPacketFlag int,
	psRangeDec *EntropyCoder,
	samplesOut []int16,
	samplesOut_ptr int,
	nSamplesOut *BoxedValueInt,
) int {
	var i, n, ret int
	decode_only_middle := 0
	LBRR_symbol := 0
	nSamplesOutDec := &BoxedValueInt{Val: 0}
	var samplesOut_tmp []int16
	samplesOut_tmp_ptrs := [2]int{0, 0}
	var samplesOut1_tmp_storage1, samplesOut1_tmp_storage2 []int16
	var samplesOut2_tmp []int16
	MS_pred_Q13 := [2]int{0, 0}
	var resample_out []int16
	resample_out_ptr := 0
	channel_state := psDec.channel_state
	has_side := 0
	stereo_to_mono := 0
	delay_stack_alloc := 0
	nSamplesOut.Val = 0

	if decControl.nChannelsInternal != 1 && decControl.nChannelsInternal != 2 {
		panic("OpusAssert")
	}

	if newPacketFlag != 0 {
		for n = 0; n < decControl.nChannelsInternal; n++ {
			channel_state[n].nFramesDecoded = 0
		}
	}

	if decControl.nChannelsInternal > psDec.nChannelsInternal {
		ret += channel_state[1].silk_init_decoder()
	}

	if decControl.nChannelsInternal == 1 && psDec.nChannelsInternal == 2 && (decControl.internalSampleRate == 1000*channel_state[0].fs_kHz) {
		stereo_to_mono = 1
	} else {
		stereo_to_mono = 0
	}

	if channel_state[0].nFramesDecoded == 0 {
		for n = 0; n < decControl.nChannelsInternal; n++ {
			var fs_kHz_dec int
			if decControl.payloadSize_ms == 0 {
				channel_state[n].nFramesPerPacket = 1
				channel_state[n].nb_subfr = 2
			} else if decControl.payloadSize_ms == 10 {
				channel_state[n].nFramesPerPacket = 1
				channel_state[n].nb_subfr = 2
			} else if decControl.payloadSize_ms == 20 {
				channel_state[n].nFramesPerPacket = 1
				channel_state[n].nb_subfr = 4
			} else if decControl.payloadSize_ms == 40 {
				channel_state[n].nFramesPerPacket = 2
				channel_state[n].nb_subfr = 4
			} else if decControl.payloadSize_ms == 60 {
				channel_state[n].nFramesPerPacket = 3
				channel_state[n].nb_subfr = 4
			} else {
				return SilkError.SILK_DEC_INVALID_FRAME_SIZE
			}
			fs_kHz_dec = (decControl.internalSampleRate >> 10) + 1
			if fs_kHz_dec != 8 && fs_kHz_dec != 12 && fs_kHz_dec != 16 {
				return SilkError.SILK_DEC_INVALID_SAMPLING_FREQUENCY
			}
			ret += channel_state[n].silk_decoder_set_fs(fs_kHz_dec, decControl.API_sampleRate)
		}
	}

	if decControl.nChannelsAPI == 2 && decControl.nChannelsInternal == 2 && (psDec.nChannelsAPI == 1 || psDec.nChannelsInternal == 1) {
		for i := 0; i < 2; i++ {
			psDec.sStereo.pred_prev_Q13[i] = 0
		}
		for i := 0; i < 2; i++ {
			psDec.sStereo.sSide[i] = 0
		}
		channel_state[1].resampler_state = channel_state[0].resampler_state
	}
	psDec.nChannelsAPI = decControl.nChannelsAPI
	psDec.nChannelsInternal = decControl.nChannelsInternal

	if decControl.API_sampleRate > MAX_API_FS_KHZ*1000 || decControl.API_sampleRate < 8000 {
		ret = SilkError.SILK_DEC_INVALID_SAMPLING_FREQUENCY
		return ret
	}

	if lostFlag != FLAG_PACKET_LOST && channel_state[0].nFramesDecoded == 0 {
		for n = 0; n < decControl.nChannelsInternal; n++ {
			for i = 0; i < channel_state[n].nFramesPerPacket; i++ {
				channel_state[n].VAD_flags[i] = psRangeDec.dec_bit_logp(1)
			}
			channel_state[n].LBRR_flag = psRangeDec.dec_bit_logp(1)
		}
		for n = 0; n < decControl.nChannelsInternal; n++ {
			for i := 0; i < MAX_FRAMES_PER_PACKET; i++ {
				channel_state[n].LBRR_flags[i] = 0
			}
			if channel_state[n].LBRR_flag != 0 {
				if channel_state[n].nFramesPerPacket == 1 {
					channel_state[n].LBRR_flags[0] = 1
				} else {
					LBRR_symbol = psRangeDec.dec_icdf(silk_LBRR_flags_iCDF_ptr[channel_state[n].nFramesPerPacket-2], 8) + 1
					for i = 0; i < channel_state[n].nFramesPerPacket; i++ {
						channel_state[n].LBRR_flags[i] = (LBRR_symbol >> i) & 1
					}
				}
			}
		}

		if lostFlag == FLAG_DECODE_NORMAL {
			for i = 0; i < channel_state[0].nFramesPerPacket; i++ {
				for n = 0; n < decControl.nChannelsInternal; n++ {
					if channel_state[n].LBRR_flags[i] != 0 {
						pulses := make([]int16, MAX_FRAME_LENGTH)
						condCoding := 0
						if decControl.nChannelsInternal == 2 && n == 0 {
							silk_stereo_decode_pred(psRangeDec, MS_pred_Q13[:])
							if channel_state[1].LBRR_flags[i] == 0 {
								decodeOnlyMiddleBoxed := &BoxedValueInt{Val: decode_only_middle}
								silk_stereo_decode_mid_only(psRangeDec, *decodeOnlyMiddleBoxed)
								decode_only_middle = decodeOnlyMiddleBoxed.Val
							}
						}
						if i > 0 && (channel_state[n].LBRR_flags[i-1] != 0) {
							condCoding = SilkConstants.CODE_CONDITIONALLY
						} else {
							condCoding = SilkConstants.CODE_INDEPENDENTLY
						}
						silk_decode_indices(channel_state[n], *psRangeDec, i, 1, condCoding)
						silk_decode_pulses(psRangeDec, pulses, channel_state[n].indices.signalType, channel_state[n].indices.quantOffsetType, channel_state[n].frame_length)
					}
				}
			}
		}
	}

	if decControl.nChannelsInternal == 2 {
		if lostFlag == FLAG_DECODE_NORMAL || (lostFlag == FLAG_DECODE_LBRR && channel_state[0].LBRR_flags[channel_state[0].nFramesDecoded] == 1) {
			silk_stereo_decode_pred(psRangeDec, MS_pred_Q13[:])
			if (lostFlag == FLAG_DECODE_NORMAL && channel_state[1].VAD_flags[channel_state[0].nFramesDecoded] == 0) || (lostFlag == FLAG_DECODE_LBRR && channel_state[1].LBRR_flags[channel_state[0].nFramesDecoded] == 0) {
				decodeOnlyMiddleBoxed := &BoxedValueInt{Val: decode_only_middle}
				silk_stereo_decode_mid_only(psRangeDec, *decodeOnlyMiddleBoxed)
				decode_only_middle = decodeOnlyMiddleBoxed.Val
			} else {
				decode_only_middle = 0
			}
		} else {
			for n = 0; n < 2; n++ {
				MS_pred_Q13[n] = psDec.sStereo.pred_prev_Q13[n]
			}
		}
	}

	if decControl.nChannelsInternal == 2 && decode_only_middle == 0 && psDec.prev_decode_only_middle == 1 {
		for i := 0; i < MAX_FRAME_LENGTH+2*MAX_SUB_FRAME_LENGTH; i++ {
			psDec.channel_state[1].outBuf[i] = 0
		}
		for i := 0; i < MAX_LPC_ORDER; i++ {
			psDec.channel_state[1].sLPC_Q14_buf[i] = 0
		}
		psDec.channel_state[1].lagPrev = 100
		psDec.channel_state[1].LastGainIndex = 10
		psDec.channel_state[1].prevSignalType = TYPE_NO_VOICE_ACTIVITY
		psDec.channel_state[1].first_frame_after_reset = 1
	}

	if decControl.internalSampleRate*decControl.nChannelsInternal < decControl.API_sampleRate*decControl.nChannelsAPI {
		delay_stack_alloc = 1
	} else {
		delay_stack_alloc = 0
	}

	if delay_stack_alloc != 0 {
		samplesOut_tmp = samplesOut
		samplesOut_tmp_ptrs[0] = samplesOut_ptr
		samplesOut_tmp_ptrs[1] = samplesOut_ptr + channel_state[0].frame_length + 2
	} else {
		samplesOut1_tmp_storage1 = make([]int16, decControl.nChannelsInternal*(channel_state[0].frame_length+2))
		samplesOut_tmp = samplesOut1_tmp_storage1
		samplesOut_tmp_ptrs[0] = 0
		samplesOut_tmp_ptrs[1] = channel_state[0].frame_length + 2
	}

	if lostFlag == FLAG_DECODE_NORMAL {
		if decode_only_middle == 0 {
			has_side = 1
		} else {
			has_side = 0
		}
	} else {
		if psDec.prev_decode_only_middle == 0 || (decControl.nChannelsInternal == 2 && lostFlag == FLAG_DECODE_LBRR && channel_state[1].LBRR_flags[channel_state[1].nFramesDecoded] == 1) {
			has_side = 1
		} else {
			has_side = 0
		}
	}

	for n = 0; n < decControl.nChannelsInternal; n++ {
		if n == 0 || has_side != 0 {
			FrameIndex := channel_state[0].nFramesDecoded - n
			condCoding := 0
			if FrameIndex <= 0 {
				condCoding = SilkConstants.CODE_INDEPENDENTLY
			} else if lostFlag == FLAG_DECODE_LBRR {
				if channel_state[n].LBRR_flags[FrameIndex-1] != 0 {
					condCoding = SilkConstants.CODE_CONDITIONALLY
				} else {
					condCoding = SilkConstants.CODE_INDEPENDENTLY
				}
			} else if n > 0 && psDec.prev_decode_only_middle != 0 {
				condCoding = CODE_INDEPENDENTLY_NO_LTP_SCALING
			} else {
				condCoding = CODE_CONDITIONALLY
			}
			ret += channel_state[n].silk_decode_frame(psRangeDec, samplesOut_tmp, samplesOut_tmp_ptrs[n]+2, nSamplesOutDec, lostFlag, condCoding)
		} else {
			start := samplesOut_tmp_ptrs[n] + 2
			for i := start; i < start+nSamplesOutDec.Val; i++ {
				samplesOut_tmp[i] = 0
			}
		}
		channel_state[n].nFramesDecoded++
	}

	if decControl.nChannelsAPI == 2 && decControl.nChannelsInternal == 2 {
		silk_stereo_MS_to_LR(psDec.sStereo, samplesOut_tmp, samplesOut_tmp_ptrs[0], samplesOut_tmp, samplesOut_tmp_ptrs[1], MS_pred_Q13[:], channel_state[0].fs_kHz, nSamplesOutDec.Val)
	} else {
		copy(samplesOut_tmp[samplesOut_tmp_ptrs[0]:samplesOut_tmp_ptrs[0]+2], psDec.sStereo.sMid[:2])
		copy(psDec.sStereo.sMid[:2], samplesOut_tmp[samplesOut_tmp_ptrs[0]+nSamplesOutDec.Val:samplesOut_tmp_ptrs[0]+nSamplesOutDec.Val+2])
	}

	nSamplesOut.Val = (nSamplesOutDec.Val * decControl.API_sampleRate) / (channel_state[0].fs_kHz * 1000)

	if decControl.nChannelsAPI == 2 {
		samplesOut2_tmp = make([]int16, nSamplesOut.Val)
		resample_out = samplesOut2_tmp
		resample_out_ptr = 0
	} else {
		resample_out = samplesOut
		resample_out_ptr = samplesOut_ptr
	}

	if delay_stack_alloc != 0 {
		length := decControl.nChannelsInternal * (channel_state[0].frame_length + 2)
		samplesOut1_tmp_storage2 = make([]int16, length)
		copy(samplesOut1_tmp_storage2, samplesOut[samplesOut_ptr:samplesOut_ptr+length])
		samplesOut_tmp = samplesOut1_tmp_storage2
		samplesOut_tmp_ptrs[0] = 0
		samplesOut_tmp_ptrs[1] = channel_state[0].frame_length + 2
	}

	minChannels := decControl.nChannelsAPI
	if decControl.nChannelsInternal < minChannels {
		minChannels = decControl.nChannelsInternal
	}
	for n = 0; n < minChannels; n++ {
		ret += silk_resampler(channel_state[n].resampler_state, resample_out, resample_out_ptr, samplesOut_tmp, samplesOut_tmp_ptrs[n]+1, nSamplesOutDec.Val)

		if decControl.nChannelsAPI == 2 {
			for i = 0; i < nSamplesOut.Val; i++ {
				samplesOut[samplesOut_ptr+n+2*i] = resample_out[resample_out_ptr+i]
			}
		}
	}

	if decControl.nChannelsAPI == 2 && decControl.nChannelsInternal == 1 {
		if stereo_to_mono != 0 {
			ret += silk_resampler(channel_state[1].resampler_state, resample_out, resample_out_ptr, samplesOut_tmp, samplesOut_tmp_ptrs[0]+1, nSamplesOutDec.Val)
			for i = 0; i < nSamplesOut.Val; i++ {
				samplesOut[samplesOut_ptr+1+2*i] = resample_out[resample_out_ptr+i]
			}
		} else {
			for i = 0; i < nSamplesOut.Val; i++ {
				samplesOut[samplesOut_ptr+1+2*i] = samplesOut[samplesOut_ptr+2*i]
			}
		}
	}

	if channel_state[0].prevSignalType == TYPE_VOICED {
		mult_tab := [3]int{6, 4, 3}
		decControl.prevPitchLag = channel_state[0].lagPrev * mult_tab[(channel_state[0].fs_kHz-8)>>2]
	} else {
		decControl.prevPitchLag = 0
	}

	if lostFlag == FLAG_PACKET_LOST {
		for i = 0; i < psDec.nChannelsInternal; i++ {
			psDec.channel_state[i].LastGainIndex = 10
		}
	} else {
		psDec.prev_decode_only_middle = decode_only_middle
	}

	return ret
}
