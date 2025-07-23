package silk

type SilkChannelEncoder struct {
	In_HP_State                   [2]int32
	variable_HP_smth1_Q15         int32
	variable_HP_smth2_Q15         int32
	sLP                           SilkLPState
	sVAD                          SilkVADState
	sNSQ                          SilkNSQState
	prev_NLSFq_Q15                [MAX_LPC_ORDER]int16
	speech_activity_Q8            int32
	allow_bandwidth_switch        int32
	LBRRprevLastGainIndex         byte
	prevSignalType                byte
	prevLag                       int32
	pitch_LPC_win_length          int32
	max_pitch_lag                 int32
	API_fs_Hz                     int32
	prev_API_fs_Hz                int32
	maxInternal_fs_Hz             int32
	minInternal_fs_Hz             int32
	desiredInternal_fs_Hz         int32
	fs_kHz                        int32
	nb_subfr                      int32
	frame_length                  int32
	subfr_length                  int32
	ltp_mem_length                int32
	la_pitch                      int32
	la_shape                      int32
	shapeWinLength                int32
	TargetRate_bps                int32
	PacketSize_ms                 int32
	PacketLoss_perc               int32
	frameCounter                  int32
	Complexity                    int32
	nStatesDelayedDecision        int32
	useInterpolatedNLSFs          int32
	shapingLPCOrder               int32
	predictLPCOrder               int32
	pitchEstimationComplexity     int32
	pitchEstimationLPCOrder       int32
	pitchEstimationThreshold_Q16  int32
	LTPQuantLowComplexity         int32
	mu_LTP_Q9                     int32
	sum_log_gain_Q7               int32
	NLSF_MSVQ_Survivors           int32
	first_frame_after_reset       int32
	controlled_since_last_payload int32
	warping_Q16                   int32
	useCBR                        int32
	prefillFlag                   int32
	pitch_lag_low_bits_iCDF       []int16
	pitch_contour_iCDF            []int16
	psNLSF_CB                     *NLSFCodebook
	input_quality_bands_Q15       [VAD_N_BANDS]int32
	input_tilt_Q15                int32
	SNR_dB_Q7                     int32
	VAD_flags                     [MAX_FRAMES_PER_PACKET]byte
	LBRR_flag                     byte
	LBRR_flags                    [MAX_FRAMES_PER_PACKET]int32
	indices                       SideInfoIndices
	pulses                        [MAX_FRAME_LENGTH]byte
	inputBuf                      [MAX_FRAME_LENGTH + 2]int16
	inputBufIx                    int32
	nFramesPerPacket              int32
	nFramesEncoded                int32
	nChannelsAPI                  int32
	nChannelsInternal             int32
	channelNb                     int32
	frames_since_onset            int32
	ec_prevSignalType             int32
	ec_prevLagIndex               int16
	resampler_state               SilkResamplerState
	useDTX                        int32
	inDTX                         int32
	noSpeechCounter               int32
	useInBandFEC                  int32
	LBRR_enabled                  int32
	LBRR_GainIncreases            int32
	indices_LBRR                  [MAX_FRAMES_PER_PACKET]SideInfoIndices
	pulses_LBRR                   [MAX_FRAMES_PER_PACKET][MAX_FRAME_LENGTH]byte
	sShape                        SilkShapeState
	sPrefilt                      SilkPrefilterState
	x_buf                         [2*MAX_FRAME_LENGTH + LA_SHAPE_MAX]int16
	LTPCorr_Q15                   int32
}

func NewSilkChannelEncoder() *SilkChannelEncoder {
	enc := &SilkChannelEncoder{}
	for c := 0; c < MAX_FRAMES_PER_PACKET; c++ {
		enc.indices_LBRR[c] = NewSideInfoIndices()
	}
	return enc
}

func (enc *SilkChannelEncoder) Reset() {
	enc.In_HP_State = [2]int32{}
	enc.variable_HP_smth1_Q15 = 0
	enc.variable_HP_smth2_Q15 = 0
	enc.sLP.Reset()
	enc.sVAD.Reset()
	enc.sNSQ.Reset()
	enc.prev_NLSFq_Q15 = [MAX_LPC_ORDER]int16{}
	enc.speech_activity_Q8 = 0
	enc.allow_bandwidth_switch = 0
	enc.LBRRprevLastGainIndex = 0
	enc.prevSignalType = 0
	enc.prevLag = 0
	enc.pitch_LPC_win_length = 0
	enc.max_pitch_lag = 0
	enc.API_fs_Hz = 0
	enc.prev_API_fs_Hz = 0
	enc.maxInternal_fs_Hz = 0
	enc.minInternal_fs_Hz = 0
	enc.desiredInternal_fs_Hz = 0
	enc.fs_kHz = 0
	enc.nb_subfr = 0
	enc.frame_length = 0
	enc.subfr_length = 0
	enc.ltp_mem_length = 0
	enc.la_pitch = 0
	enc.la_shape = 0
	enc.shapeWinLength = 0
	enc.TargetRate_bps = 0
	enc.PacketSize_ms = 0
	enc.PacketLoss_perc = 0
	enc.frameCounter = 0
	enc.Complexity = 0
	enc.nStatesDelayedDecision = 0
	enc.useInterpolatedNLSFs = 0
	enc.shapingLPCOrder = 0
	enc.predictLPCOrder = 0
	enc.pitchEstimationComplexity = 0
	enc.pitchEstimationLPCOrder = 0
	enc.pitchEstimationThreshold_Q16 = 0
	enc.LTPQuantLowComplexity = 0
	enc.mu_LTP_Q9 = 0
	enc.sum_log_gain_Q7 = 0
	enc.NLSF_MSVQ_Survivors = 0
	enc.first_frame_after_reset = 0
	enc.controlled_since_last_payload = 0
	enc.warping_Q16 = 0
	enc.useCBR = 0
	enc.prefillFlag = 0
	enc.pitch_lag_low_bits_iCDF = nil
	enc.pitch_contour_iCDF = nil
	enc.psNLSF_CB = nil
	enc.input_quality_bands_Q15 = [VAD_N_BANDS]int32{}
	enc.input_tilt_Q15 = 0
	enc.SNR_dB_Q7 = 0
	enc.VAD_flags = [MAX_FRAMES_PER_PACKET]byte{}
	enc.LBRR_flag = 0
	enc.LBRR_flags = [MAX_FRAMES_PER_PACKET]int32{}
	enc.indices.Reset()
	enc.pulses = [MAX_FRAME_LENGTH]byte{}
	enc.inputBuf = [MAX_FRAME_LENGTH + 2]int16{}
	enc.inputBufIx = 0
	enc.nFramesPerPacket = 0
	enc.nFramesEncoded = 0
	enc.nChannelsAPI = 0
	enc.nChannelsInternal = 0
	enc.channelNb = 0
	enc.frames_since_onset = 0
	enc.ec_prevSignalType = 0
	enc.ec_prevLagIndex = 0
	enc.resampler_state.Reset()
	enc.useDTX = 0
	enc.inDTX = 0
	enc.noSpeechCounter = 0
	enc.useInBandFEC = 0
	enc.LBRR_enabled = 0
	enc.LBRR_GainIncreases = 0
	for c := 0; c < MAX_FRAMES_PER_PACKET; c++ {
		enc.indices_LBRR[c].Reset()
		enc.pulses_LBRR[c] = [MAX_FRAME_LENGTH]byte{}
	}
	enc.sShape.Reset()
	enc.sPrefilt.Reset()
	enc.x_buf = [2*MAX_FRAME_LENGTH + LA_SHAPE_MAX]int16{}
	enc.LTPCorr_Q15 = 0
}

func (enc *SilkChannelEncoder) silk_control_encoder(encControl *EncControlState, TargetRate_bps, allow_bw_switch, channelNb, force_fs_kHz int32) int32 {
	var fs_kHz int32
	ret := SILK_NO_ERROR

	enc.useDTX = encControl.useDTX
	enc.useCBR = encControl.useCBR
	enc.API_fs_Hz = encControl.API_sampleRate
	enc.maxInternal_fs_Hz = encControl.maxInternalSampleRate
	enc.minInternal_fs_Hz = encControl.minInternalSampleRate
	enc.desiredInternal_fs_Hz = encControl.desiredInternalSampleRate
	enc.useInBandFEC = encControl.useInBandFEC
	enc.nChannelsAPI = encControl.nChannelsAPI
	enc.nChannelsInternal = encControl.nChannelsInternal
	enc.allow_bandwidth_switch = allow_bw_switch
	enc.channelNb = channelNb

	if enc.controlled_since_last_payload != 0 && enc.prefillFlag == 0 {
		if enc.API_fs_Hz != enc.prev_API_fs_Hz && enc.fs_kHz > 0 {
			ret = enc.silk_setup_resamplers(enc.fs_kHz)
		}
		return ret
	}

	fs_kHz = enc.silk_control_audio_bandwidth(encControl)
	if force_fs_kHz != 0 {
		fs_kHz = force_fs_kHz
	}

	ret = enc.silk_setup_resamplers(fs_kHz)
	ret = enc.silk_setup_fs(fs_kHz, encControl.payloadSize_ms)
	ret = enc.silk_setup_complexity(encControl.complexity)
	enc.PacketLoss_perc = encControl.packetLossPercentage
	ret = enc.silk_setup_LBRR(TargetRate_bps)

	enc.controlled_since_last_payload = 1
	return ret
}

func (enc *SilkChannelEncoder) silk_setup_resamplers(fs_kHz int32) int32 {
	ret := SILK_NO_ERROR

	if enc.fs_kHz != fs_kHz || enc.prev_API_fs_Hz != enc.API_fs_Hz {
		if enc.fs_kHz == 0 {
			ret += silk_resampler_init(&enc.resampler_state, enc.API_fs_Hz, fs_kHz*1000, 1)
		} else {
			var x_buf_API_fs_Hz []int16
			var temp_resampler_state SilkResamplerState

			buf_length_ms := LSHIFT32(enc.nb_subfr*5, 1) + LA_SHAPE_MS
			old_buf_samples := buf_length_ms * enc.fs_kHz

			temp_resampler_state = SilkResamplerState{}
			ret += silk_resampler_init(&temp_resampler_state, SMULBB(enc.fs_kHz, 1000), enc.API_fs_Hz, 0)

			api_buf_samples := buf_length_ms * DIV32_16(enc.API_fs_Hz, 1000)

			x_buf_API_fs_Hz = make([]int16, api_buf_samples)
			ret += silk_resampler(&temp_resampler_state, x_buf_API_fs_Hz, 0, enc.x_buf[:], 0, old_buf_samples)

			ret += silk_resampler_init(&enc.resampler_state, enc.API_fs_Hz, SMULBB(fs_kHz, 1000), 1)
			ret += silk_resampler(&enc.resampler_state, enc.x_buf[:], 0, x_buf_API_fs_Hz, 0, api_buf_samples)
		}
	}

	enc.prev_API_fs_Hz = enc.API_fs_Hz
	return ret
}

func (enc *SilkChannelEncoder) silk_setup_fs(fs_kHz, PacketSize_ms int32) int32 {
	ret := SILK_NO_ERROR

	if PacketSize_ms != enc.PacketSize_ms {
		if PacketSize_ms != 10 && PacketSize_ms != 20 && PacketSize_ms != 40 && PacketSize_ms != 60 {
			ret = SILK_ENC_PACKET_SIZE_NOT_SUPPORTED
		}
		if PacketSize_ms <= 10 {
			enc.nFramesPerPacket = 1
			if PacketSize_ms == 10 {
				enc.nb_subfr = 2
			} else {
				enc.nb_subfr = 1
			}
			enc.frame_length = SMULBB(PacketSize_ms, fs_kHz)
			enc.pitch_LPC_win_length = SMULBB(FIND_PITCH_LPC_WIN_MS_2_SF, fs_kHz)
			if enc.fs_kHz == 8 {
				enc.pitch_contour_iCDF = silk_pitch_contour_10_ms_NB_iCDF[:]
			} else {
				enc.pitch_contour_iCDF = silk_pitch_contour_10_ms_iCDF[:]
			}
		} else {
			enc.nFramesPerPacket = DIV32_16(PacketSize_ms, MAX_FRAME_LENGTH_MS)
			enc.nb_subfr = MAX_NB_SUBFR
			enc.frame_length = SMULBB(20, fs_kHz)
			enc.pitch_LPC_win_length = SMULBB(FIND_PITCH_LPC_WIN_MS, fs_kHz)
			if enc.fs_kHz == 8 {
				enc.pitch_contour_iCDF = silk_pitch_contour_NB_iCDF[:]
			} else {
				enc.pitch_contour_iCDF = silk_pitch_contour_iCDF[:]
			}
		}
		enc.PacketSize_ms = PacketSize_ms
		enc.TargetRate_bps = 0
	}

	if enc.fs_kHz != fs_kHz {
		enc.sShape.Reset()
		enc.sPrefilt.Reset()
		enc.sNSQ.Reset()
		enc.prev_NLSFq_Q15 = [MAX_LPC_ORDER]int16{}
		enc.sLP.In_LP_State = [2]int32{}
		enc.inputBufIx = 0
		enc.nFramesEncoded = 0
		enc.TargetRate_bps = 0

		enc.prevLag = 100
		enc.first_frame_after_reset = 1
		enc.sPrefilt.lagPrev = 100
		enc.sShape.LastGainIndex = 10
		enc.sNSQ.lagPrev = 100
		enc.sNSQ.prev_gain_Q16 = 65536
		enc.prevSignalType = TYPE_NO_VOICE_ACTIVITY

		enc.fs_kHz = fs_kHz
		if enc.fs_kHz == 8 {
			if enc.nb_subfr == MAX_NB_SUBFR {
				enc.pitch_contour_iCDF = silk_pitch_contour_NB_iCDF[:]
			} else {
				enc.pitch_contour_iCDF = silk_pitch_contour_10_ms_NB_iCDF[:]
			}
		} else if enc.nb_subfr == MAX_NB_SUBFR {
			enc.pitch_contour_iCDF = silk_pitch_contour_iCDF[:]
		} else {
			enc.pitch_contour_iCDF = silk_pitch_contour_10_ms_iCDF[:]
		}

		if enc.fs_kHz == 8 || enc.fs_kHz == 12 {
			enc.predictLPCOrder = MIN_LPC_ORDER
			enc.psNLSF_CB = &silk_NLSF_CB_NB_MB
		} else {
			enc.predictLPCOrder = MAX_LPC_ORDER
			enc.psNLSF_CB = &silk_NLSF_CB_WB
		}

		enc.subfr_length = SUB_FRAME_LENGTH_MS * fs_kHz
		enc.frame_length = SMULBB(enc.subfr_length, enc.nb_subfr)
		enc.ltp_mem_length = SMULBB(LTP_MEM_LENGTH_MS, fs_kHz)
		enc.la_pitch = SMULBB(LA_PITCH_MS, fs_kHz)
		enc.max_pitch_lag = SMULBB(18, fs_kHz)

		if enc.nb_subfr == MAX_NB_SUBFR {
			enc.pitch_LPC_win_length = SMULBB(FIND_PITCH_LPC_WIN_MS, fs_kHz)
		} else {
			enc.pitch_LPC_win_length = SMULBB(FIND_PITCH_LPC_WIN_MS_2_SF, fs_kHz)
		}

		if enc.fs_kHz == 16 {
			enc.mu_LTP_Q9 = SILK_CONST(MU_LTP_QUANT_WB, 9)
			enc.pitch_lag_low_bits_iCDF = silk_uniform8_iCDF[:]
		} else if enc.fs_kHz == 12 {
			enc.mu_LTP_Q9 = SILK_CONST(MU_LTP_QUANT_MB, 9)
			enc.pitch_lag_low_bits_iCDF = silk_uniform6_iCDF[:]
		} else {
			enc.mu_LTP_Q9 = SILK_CONST(MU_LTP_QUANT_NB, 9)
			enc.pitch_lag_low_bits_iCDF = silk_uniform4_iCDF[:]
		}
	}

	return ret
}

func (enc *SilkChannelEncoder) silk_setup_complexity(Complexity int32) int32 {
	ret := SILK_NO_ERROR

	if Complexity < 2 {
		enc.pitchEstimationComplexity = SILK_PE_MIN_COMPLEX
		enc.pitchEstimationThreshold_Q16 = SILK_CONST(0.8, 16)
		enc.pitchEstimationLPCOrder = 6
		enc.shapingLPCOrder = 8
		enc.la_shape = 3 * enc.fs_kHz
		enc.nStatesDelayedDecision = 1
		enc.useInterpolatedNLSFs = 0
		enc.LTPQuantLowComplexity = 1
		enc.NLSF_MSVQ_Survivors = 2
		enc.warping_Q16 = 0
	} else if Complexity < 4 {
		enc.pitchEstimationComplexity = SILK_PE_MID_COMPLEX
		enc.pitchEstimationThreshold_Q16 = SILK_CONST(0.76, 16)
		enc.pitchEstimationLPCOrder = 8
		enc.shapingLPCOrder = 10
		enc.la_shape = 5 * enc.fs_kHz
		enc.nStatesDelayedDecision = 1
		enc.useInterpolatedNLSFs = 0
		enc.LTPQuantLowComplexity = 0
		enc.NLSF_MSVQ_Survivors = 4
		enc.warping_Q16 = 0
	} else if Complexity < 6 {
		enc.pitchEstimationComplexity = SILK_PE_MID_COMPLEX
		enc.pitchEstimationThreshold_Q16 = SILK_CONST(0.74, 16)
		enc.pitchEstimationLPCOrder = 10
		enc.shapingLPCOrder = 12
		enc.la_shape = 5 * enc.fs_kHz
		enc.nStatesDelayedDecision = 2
		enc.useInterpolatedNLSFs = 1
		enc.LTPQuantLowComplexity = 0
		enc.NLSF_MSVQ_Survivors = 8
		enc.warping_Q16 = enc.fs_kHz * SILK_CONST(WARPING_MULTIPLIER, 16)
	} else if Complexity < 8 {
		enc.pitchEstimationComplexity = SILK_PE_MID_COMPLEX
		enc.pitchEstimationThreshold_Q16 = SILK_CONST(0.72, 16)
		enc.pitchEstimationLPCOrder = 12
		enc.shapingLPCOrder = 14
		enc.la_shape = 5 * enc.fs_kHz
		enc.nStatesDelayedDecision = 3
		enc.useInterpolatedNLSFs = 1
		enc.LTPQuantLowComplexity = 0
		enc.NLSF_MSVQ_Survivors = 16
		enc.warping_Q16 = enc.fs_kHz * SILK_CONST(WARPING_MULTIPLIER, 16)
	} else {
		enc.pitchEstimationComplexity = SILK_PE_MAX_COMPLEX
		enc.pitchEstimationThreshold_Q16 = SILK_CONST(0.7, 16)
		enc.pitchEstimationLPCOrder = 16
		enc.shapingLPCOrder = 16
		enc.la_shape = 5 * enc.fs_kHz
		enc.nStatesDelayedDecision = MAX_DEL_DEC_STATES
		enc.useInterpolatedNLSFs = 1
		enc.LTPQuantLowComplexity = 0
		enc.NLSF_MSVQ_Survivors = 32
		enc.warping_Q16 = enc.fs_kHz * SILK_CONST(WARPING_MULTIPLIER, 16)
	}

	enc.pitchEstimationLPCOrder = min_int(enc.pitchEstimationLPCOrder, enc.predictLPCOrder)
	enc.shapeWinLength = SUB_FRAME_LENGTH_MS*enc.fs_kHz + 2*enc.la_shape
	enc.Complexity = Complexity

	return ret
}

func (enc *SilkChannelEncoder) silk_setup_LBRR(TargetRate_bps int32) int32 {
	var LBRR_in_previous_packet int32
	ret := SILK_NO_ERROR
	var LBRR_rate_thres_bps int32

	LBRR_in_previous_packet = enc.LBRR_enabled
	enc.LBRR_enabled = 0
	if enc.useInBandFEC != 0 && enc.PacketLoss_perc > 0 {
		if enc.fs_kHz == 8 {
			LBRR_rate_thres_bps = LBRR_NB_MIN_RATE_BPS
		} else if enc.fs_kHz == 12 {
			LBRR_rate_thres_bps = LBRR_MB_MIN_RATE_BPS
		} else {
			LBRR_rate_thres_bps = LBRR_WB_MIN_RATE_BPS
		}

		LBRR_rate_thres_bps = SMULWB(MUL32(LBRR_rate_thres_bps, 125-min_int32(enc.PacketLoss_perc, 25)), SILK_CONST(0.01, 16))

		if TargetRate_bps > LBRR_rate_thres_bps {
			if LBRR_in_previous_packet == 0 {
				enc.LBRR_GainIncreases = 7
			} else {
				enc.LBRR_GainIncreases = max_int(7-SMULWB(int32(enc.PacketLoss_perc), SILK_CONST(0.4, 16)), 2)
			}
			enc.LBRR_enabled = 1
		}
	}

	return ret
}

func (enc *SilkChannelEncoder) silk_control_audio_bandwidth(encControl *EncControlState) int32 {
	var fs_kHz int32
	var fs_Hz int32

	fs_kHz = enc.fs_kHz
	fs_Hz = SMULBB(fs_kHz, 1000)

	if fs_Hz == 0 {
		fs_Hz = min_int32(enc.desiredInternal_fs_Hz, enc.API_fs_Hz)
		fs_kHz = DIV32_16(fs_Hz, 1000)
	} else if fs_Hz > enc.API_fs_Hz || fs_Hz > enc.maxInternal_fs_Hz || fs_Hz < enc.minInternal_fs_Hz {
		fs_Hz = enc.API_fs_Hz
		fs_Hz = min_int32(fs_Hz, enc.maxInternal_fs_Hz)
		fs_Hz = max_int32(fs_Hz, enc.minInternal_fs_Hz)
		fs_kHz = DIV32_16(fs_Hz, 1000)
	} else {
		if enc.sLP.transition_frame_no >= TRANSITION_FRAMES {
			enc.sLP.mode = 0
		}

		if enc.allow_bandwidth_switch != 0 || encControl.opusCanSwitch != 0 {
			if SMULBB(enc.fs_kHz, 1000) > enc.desiredInternal_fs_Hz {
				if enc.sLP.mode == 0 {
					enc.sLP.transition_frame_no = TRANSITION_FRAMES
					enc.sLP.In_LP_State = [2]int32{}
				}

				if encControl.opusCanSwitch != 0 {
					enc.sLP.mode = 0
					if enc.fs_kHz == 16 {
						fs_kHz = 12
					} else {
						fs_kHz = 8
					}
				} else if enc.sLP.transition_frame_no <= 0 {
					encControl.switchReady = 1
					encControl.maxBits -= encControl.maxBits * 5 / (encControl.payloadSize_ms + 5)
				} else {
					enc.sLP.mode = -2
				}
			} else if SMULBB(enc.fs_kHz, 1000) < enc.desiredInternal_fs_Hz {
				if encControl.opusCanSwitch != 0 {
					if enc.fs_kHz == 8 {
						fs_kHz = 12
					} else {
						fs_kHz = 16
					}
					enc.sLP.transition_frame_no = 0
					enc.sLP.In_LP_State = [2]int32{}
					enc.sLP.mode = 1
				} else if enc.sLP.mode == 0 {
					encControl.switchReady = 1
					encControl.maxBits -= encControl.maxBits * 5 / (encControl.payloadSize_ms + 5)
				} else {
					enc.sLP.mode = 1
				}
			} else if enc.sLP.mode < 0 {
				enc.sLP.mode = 1
			}
		}
	}

	return fs_kHz
}

func (enc *SilkChannelEncoder) silk_control_SNR(TargetRate_bps int32) int32 {
	var k int
	ret := SILK_NO_ERROR
	var frac_Q6 int32
	var rateTable []int32

	TargetRate_bps = LIMIT32(TargetRate_bps, MIN_TARGET_RATE_BPS, MAX_TARGET_RATE_BPS)
	if TargetRate_bps != enc.TargetRate_bps {
		enc.TargetRate_bps = TargetRate_bps

		if enc.fs_kHz == 8 {
			rateTable = silk_TargetRate_table_NB[:]
		} else if enc.fs_kHz == 12 {
			rateTable = silk_TargetRate_table_MB[:]
		} else {
			rateTable = silk_TargetRate_table_WB[:]
		}

		if enc.nb_subfr == 2 {
			TargetRate_bps -= REDUCE_BITRATE_10_MS_BPS
		}

		for k = 1; k < TARGET_RATE_TAB_SZ; k++ {
			if TargetRate_bps <= rateTable[k] {
				frac_Q6 = DIV32(LSHIFT32(TargetRate_bps-rateTable[k-1], 6), rateTable[k]-rateTable[k-1])
				enc.SNR_dB_Q7 = LSHIFT32(int32(silk_SNR_table_Q1[k-1]), 6) + MUL32(frac_Q6, int32(silk_SNR_table_Q1[k]-silk_SNR_table_Q1[k-1]))
				break
			}
		}
	}

	return ret
}

func (enc *SilkChannelEncoder) silk_encode_do_VAD() {
	silk_VAD_GetSA_Q8(enc, enc.inputBuf[:], 1)

	if enc.speech_activity_Q8 < SILK_CONST(SPEECH_ACTIVITY_DTX_THRES, 8) {
		enc.indices.signalType = TYPE_NO_VOICE_ACTIVITY
		enc.noSpeechCounter++
		if enc.noSpeechCounter < NB_SPEECH_FRAMES_BEFORE_DTX {
			enc.inDTX = 0
		} else if enc.noSpeechCounter > MAX_CONSECUTIVE_DTX+NB_SPEECH_FRAMES_BEFORE_DTX {
			enc.noSpeechCounter = NB_SPEECH_FRAMES_BEFORE_DTX
			enc.inDTX = 0
		}
		enc.VAD_flags[enc.nFramesEncoded] = 0
	} else {
		enc.noSpeechCounter = 0
		enc.inDTX = 0
		enc.indices.signalType = TYPE_UNVOICED
		enc.VAD_flags[enc.nFramesEncoded] = 1
	}
}

func (enc *SilkChannelEncoder) silk_encode_frame(pnBytesOut *int32, psRangeEnc *EntropyCoder, condCoding, maxBits, useCBR int32) int32 {
	sEncCtrl := NewSilkEncoderControl()
	var i, iter, maxIter, found_upper, found_lower, ret int32
	var x_frame int32
	sRangeEnc_copy := NewEntropyCoder()
	sRangeEnc_copy2 := NewEntropyCoder()
	sNSQ_copy := NewSilkNSQState()
	sNSQ_copy2 := NewSilkNSQState()
	var nBits, nBits_lower, nBits_upper, gainMult_lower, gainMult_upper int32
	var gainsID, gainsID_lower, gainsID_upper int32
	var gainMult_Q8 int16
	var ec_prevLagIndex_copy int16
	var ec_prevSignalType_copy int32
	var LastGainIndex_copy2 byte
	var seed_copy byte

	LastGainIndex_copy2 = 0
	nBits_lower, nBits_upper, gainMult_lower, gainMult_upper = 0, 0, 0, 0

	enc.indices.Seed = byte(enc.frameCounter & 3)
	enc.frameCounter++

	x_frame = enc.ltp_mem_length

	enc.sLP.silk_LP_variable_cutoff(enc.inputBuf[:], 1, enc.frame_length)

	copy(enc.x_buf[x_frame+LA_SHAPE_MS*enc.fs_kHz:], enc.inputBuf[1:1+enc.frame_length])

	if enc.prefillFlag == 0 {
		var xfw_Q3 []int32
		var res_pitch []int16
		var ec_buf_copy []byte
		var res_pitch_frame int32

		res_pitch = make([]int16, enc.la_pitch+enc.frame_length+enc.ltp_mem_length)
		res_pitch_frame = enc.ltp_mem_length

		silk_find_pitch_lags(enc, sEncCtrl, res_pitch, enc.x_buf[:], x_frame)

		silk_noise_shape_analysis(enc, sEncCtrl, res_pitch, res_pitch_frame, enc.x_buf[:], x_frame)

		silk_find_pred_coefs(enc, sEncCtrl, res_pitch, enc.x_buf[:], x_frame, condCoding)

		silk_process_gains(enc, sEncCtrl, condCoding)

		xfw_Q3 = make([]int32, enc.frame_length)
		silk_prefilter(enc, sEncCtrl, xfw_Q3, enc.x_buf[:], x_frame)

		enc.silk_LBRR_encode(sEncCtrl, xfw_Q3, condCoding)

		maxIter = 6
		gainMult_Q8 = SILK_CONST(1, 8)
		found_lower = 0
		found_upper = 0
		gainsID = silk_gains_ID(enc.indices.GainsIndices[:], enc.nb_subfr)
		gainsID_lower = -1
		gainsID_upper = -1
		sRangeEnc_copy.Assign(psRangeEnc)
		sNSQ_copy.Assign(&enc.sNSQ)
		seed_copy = enc.indices.Seed
		ec_prevLagIndex_copy = enc.ec_prevLagIndex
		ec_prevSignalType_copy = enc.ec_prevSignalType
		ec_buf_copy = make([]byte, 1275)

		for iter = 0; ; iter++ {
			if gainsID == gainsID_lower {
				nBits = nBits_lower
			} else if gainsID == gainsID_upper {
				nBits = nBits_upper
			} else {
				if iter > 0 {
					psRangeEnc.Assign(sRangeEnc_copy)
					enc.sNSQ.Assign(sNSQ_copy)
					enc.indices.Seed = seed_copy
					enc.ec_prevLagIndex = ec_prevLagIndex_copy
					enc.ec_prevSignalType = ec_prevSignalType_copy
				}

				if enc.nStatesDelayedDecision > 1 || enc.warping_Q16 > 0 {
					enc.sNSQ.silk_NSQ_del_dec(
						enc,
						&enc.indices,
						xfw_Q3,
						enc.pulses[:],
						sEncCtrl.PredCoef_Q12[:],
						sEncCtrl.LTPCoef_Q14[:],
						sEncCtrl.AR2_Q13[:],
						sEncCtrl.HarmShapeGain_Q14[:],
						sEncCtrl.Tilt_Q14,
						sEncCtrl.LF_shp_Q14[:],
						sEncCtrl.Gains_Q16[:],
						sEncCtrl.pitchL[:],
						sEncCtrl.Lambda_Q10,
						sEncCtrl.LTP_scale_Q14)
				} else {
					enc.sNSQ.silk_NSQ(
						enc,
						&enc.indices,
						xfw_Q3,
						enc.pulses[:],
						sEncCtrl.PredCoef_Q12[:],
						sEncCtrl.LTPCoef_Q14[:],
						sEncCtrl.AR2_Q13[:],
						sEncCtrl.HarmShapeGain_Q14[:],
						sEncCtrl.Tilt_Q14,
						sEncCtrl.LF_shp_Q14[:],
						sEncCtrl.Gains_Q16[:],
						sEncCtrl.pitchL[:],
						sEncCtrl.Lambda_Q10,
						sEncCtrl.LTP_scale_Q14)
				}

				silk_encode_indices(enc, psRangeEnc, enc.nFramesEncoded, 0, condCoding)

				silk_encode_pulses(psRangeEnc, enc.indices.signalType, enc.indices.quantOffsetType,
					enc.pulses[:], enc.frame_length)

				nBits = psRangeEnc.tell()

				if useCBR == 0 && iter == 0 && nBits <= maxBits {
					break
				}
			}

			if iter == maxIter {
				if found_lower != 0 && (gainsID == gainsID_lower || nBits > maxBits) {
					psRangeEnc.Assign(sRangeEnc_copy2)
					copy(psRangeEnc.buffer[:sRangeEnc_copy2.offs], ec_buf_copy)
					enc.sNSQ.Assign(sNSQ_copy2)
					enc.sShape.LastGainIndex = LastGainIndex_copy2
				}
				break
			}

			if nBits > maxBits {
				if found_lower == 0 && iter >= 2 {
					sEncCtrl.Lambda_Q10 = ADD_RSHIFT32(sEncCtrl.Lambda_Q10, sEncCtrl.Lambda_Q10, 1)
					found_upper = 0
					gainsID_upper = -1
				} else {
					found_upper = 1
					nBits_upper = nBits
					gainMult_upper = int32(gainMult_Q8)
					gainsID_upper = gainsID
				}
			} else if nBits < maxBits-5 {
				found_lower = 1
				nBits_lower = nBits
				gainMult_lower = int32(gainMult_Q8)
				if gainsID != gainsID_lower {
					gainsID_lower = gainsID
					sRangeEnc_copy2.Assign(psRangeEnc)
					copy(ec_buf_copy, psRangeEnc.buffer[:psRangeEnc.offs])
					sNSQ_copy2.Assign(&enc.sNSQ)
					LastGainIndex_copy2 = enc.sShape.LastGainIndex
				}
			} else {
				break
			}

			if (found_lower & found_upper) == 0 {
				gain_factor_Q16 := silk_log2lin(LSHIFT32(nBits-maxBits, 7)/enc.frame_length + SILK_CONST(16, 7))
				gain_factor_Q16 = min_int32(gain_factor_Q16, SILK_CONST(2, 16))
				if nBits > maxBits {
					gain_factor_Q16 = max_int32(gain_factor_Q16, SILK_CONST(1.3, 16))
				}
				gainMult_Q8 = int16(SMULWB(gain_factor_Q16, int32(gainMult_Q8)))
			} else {
				gainMult_Q8 = int16(gainMult_lower + DIV32_16(MUL32(gainMult_upper-gainMult_lower, maxBits-nBits_lower), nBits_upper-nBits_lower))
				if gainMult_Q8 > int16(ADD_RSHIFT32(gainMult_lower, gainMult_upper-gainMult_lower, 2)) {
					gainMult_Q8 = int16(ADD_RSHIFT32(gainMult_lower, gainMult_upper-gainMult_lower, 2))
				} else if gainMult_Q8 < int16(SUB_RSHIFT32(gainMult_upper, gainMult_upper-gainMult_lower, 2)) {
					gainMult_Q8 = int16(SUB_RSHIFT32(gainMult_upper, gainMult_upper-gainMult_lower, 2))
				}
			}

			for i = 0; i < enc.nb_subfr; i++ {
				sEncCtrl.Gains_Q16[i] = LSHIFT_SAT32(SMULWB(sEncCtrl.GainsUnq_Q16[i], int32(gainMult_Q8)), 8)
			}

			enc.sShape.LastGainIndex = sEncCtrl.lastGainIndexPrev
			gainIndex := enc.sShape.LastGainIndex
			silk_gains_quant(enc.indices.GainsIndices[:], sEncCtrl.Gains_Q16[:],
				&gainIndex, boolToInt(condCoding == CODE_CONDITIONALLY), enc.nb_subfr)
			enc.sShape.LastGainIndex = gainIndex

			gainsID = silk_gains_ID(enc.indices.GainsIndices[:], enc.nb_subfr)
		}
	}

	copy(enc.x_buf[:enc.ltp_mem_length+LA_SHAPE_MS*enc.fs_kHz], enc.x_buf[enc.frame_length:])

	if enc.prefillFlag != 0 {
		*pnBytesOut = 0
		return ret
	}

	enc.prevLag = sEncCtrl.pitchL[enc.nb_subfr-1]
	enc.prevSignalType = enc.indices.signalType

	enc.first_frame_after_reset = 0
	*pnBytesOut = RSHIFT32(psRangeEnc.tell()+7, 3)

	return ret
}

func (enc *SilkChannelEncoder) silk_LBRR_encode(thisCtrl *SilkEncoderControl, xfw_Q3 []int32, condCoding int32) {
	TempGains_Q16 := make([]int32, enc.nb_subfr)
	psIndices_LBRR := &enc.indices_LBRR[enc.nFramesEncoded]
	var sNSQ_LBRR SilkNSQState

	if enc.LBRR_enabled != 0 && enc.speech_activity_Q8 > SILK_CONST(LBRR_SPEECH_ACTIVITY_THRES, 8) {
		enc.LBRR_flags[enc.nFramesEncoded] = 1

		sNSQ_LBRR.Assign(&enc.sNSQ)
		psIndices_LBRR.Assign(&enc.indices)

		copy(TempGains_Q16, thisCtrl.Gains_Q16[:enc.nb_subfr])

		if enc.nFramesEncoded == 0 || enc.LBRR_flags[enc.nFramesEncoded-1] == 0 {
			enc.LBRRprevLastGainIndex = enc.sShape.LastGainIndex
			psIndices_LBRR.GainsIndices[0] = byte(min_int(int(psIndices_LBRR.GainsIndices[0])+enc.LBRR_GainIncreases, N_LEVELS_QGAIN-1))
		}

		gainIndex := enc.LBRRprevLastGainIndex
		silk_gains_dequant(thisCtrl.Gains_Q16[:], psIndices_LBRR.GainsIndices[:],
			&gainIndex, boolToInt(condCoding == CODE_CONDITIONALLY), enc.nb_subfr)
		enc.LBRRprevLastGainIndex = gainIndex

		if enc.nStatesDelayedDecision > 1 || enc.warping_Q16 > 0 {
			sNSQ_LBRR.silk_NSQ_del_dec(enc,
				psIndices_LBRR,
				xfw_Q3,
				enc.pulses_LBRR[enc.nFramesEncoded][:],
				thisCtrl.PredCoef_Q12[:],
				thisCtrl.LTPCoef_Q14[:],
				thisCtrl.AR2_Q13[:],
				thisCtrl.HarmShapeGain_Q14[:],
				thisCtrl.Tilt_Q14,
				thisCtrl.LF_shp_Q14[:],
				thisCtrl.Gains_Q16[:],
				thisCtrl.pitchL[:],
				thisCtrl.Lambda_Q10,
				thisCtrl.LTP_scale_Q14)
		} else {
			sNSQ_LBRR.silk_NSQ(enc,
				psIndices_LBRR,
				xfw_Q3,
				enc.pulses_LBRR[enc.nFramesEncoded][:],
				thisCtrl.PredCoef_Q12[:],
				thisCtrl.LTPCoef_Q14[:],
				thisCtrl.AR2_Q13[:],
				thisCtrl.HarmShapeGain_Q14[:],
				thisCtrl.Tilt_Q14,
				thisCtrl.LF_shp_Q14[:],
				thisCtrl.Gains_Q16[:],
				thisCtrl.pitchL[:],
				thisCtrl.Lambda_Q10,
				thisCtrl.LTP_scale_Q14)
		}

		copy(thisCtrl.Gains_Q16[:enc.nb_subfr], TempGains_Q16)
	}
}

// Helper functions
func boolToInt(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

func min_int(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func max_int(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func min_int32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func max_int32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
