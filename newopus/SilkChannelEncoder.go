package opus

type SilkChannelEncoder struct {
	In_HP_State                   [2]int32
	variable_HP_smth1_Q15         int32
	variable_HP_smth2_Q15         int32
	sLP                           SilkLPState
	sVAD                          SilkVADState
	sNSQ                          SilkNSQState
	prev_NLSFq_Q15                [SilkConstants.MAX_LPC_ORDER]int16
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
	input_quality_bands_Q15       [SilkConstants.VAD_N_BANDS]int32
	input_tilt_Q15                int32
	SNR_dB_Q7                     int32
	VAD_flags                     [SilkConstants.MAX_FRAMES_PER_PACKET]byte
	LBRR_flag                     byte
	LBRR_flags                    [SilkConstants.MAX_FRAMES_PER_PACKET]int32
	indices                       SideInfoIndices
	pulses                        [SilkConstants.MAX_FRAME_LENGTH]byte
	inputBuf                      [SilkConstants.MAX_FRAME_LENGTH + 2]int16
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
	indices_LBRR                  [SilkConstants.MAX_FRAMES_PER_PACKET]*SideInfoIndices
	pulses_LBRR                   [SilkConstants.MAX_FRAMES_PER_PACKET][SilkConstants.MAX_FRAME_LENGTH]byte
	sShape                        SilkShapeState
	sPrefilt                      SilkPrefilterState
	x_buf                         [2*SilkConstants.MAX_FRAME_LENGTH + SilkConstants.LA_SHAPE_MAX]int16
	LTPCorr_Q15                   int32
}

func (s *SilkChannelEncoder) Reset() {
	for i := range s.In_HP_State {
		s.In_HP_State[i] = 0
	}
	s.variable_HP_smth1_Q15 = 0
	s.variable_HP_smth2_Q15 = 0
	s.sLP.Reset()
	s.sVAD.Reset()
	s.sNSQ.Reset()
	for i := range s.prev_NLSFq_Q15 {
		s.prev_NLSFq_Q15[i] = 0
	}
	s.speech_activity_Q8 = 0
	s.allow_bandwidth_switch = 0
	s.LBRRprevLastGainIndex = 0
	s.prevSignalType = 0
	s.prevLag = 0
	s.pitch_LPC_win_length = 0
	s.max_pitch_lag = 0
	s.API_fs_Hz = 0
	s.prev_API_fs_Hz = 0
	s.maxInternal_fs_Hz = 0
	s.minInternal_fs_Hz = 0
	s.desiredInternal_fs_Hz = 0
	s.fs_kHz = 0
	s.nb_subfr = 0
	s.frame_length = 0
	s.subfr_length = 0
	s.ltp_mem_length = 0
	s.la_pitch = 0
	s.la_shape = 0
	s.shapeWinLength = 0
	s.TargetRate_bps = 0
	s.PacketSize_ms = 0
	s.PacketLoss_perc = 0
	s.frameCounter = 0
	s.Complexity = 0
	s.nStatesDelayedDecision = 0
	s.useInterpolatedNLSFs = 0
	s.shapingLPCOrder = 0
	s.predictLPCOrder = 0
	s.pitchEstimationComplexity = 0
	s.pitchEstimationLPCOrder = 0
	s.pitchEstimationThreshold_Q16 = 0
	s.LTPQuantLowComplexity = 0
	s.mu_LTP_Q9 = 0
	s.sum_log_gain_Q7 = 0
	s.NLSF_MSVQ_Survivors = 0
	s.first_frame_after_reset = 0
	s.controlled_since_last_payload = 0
	s.warping_Q16 = 0
	s.useCBR = 0
	s.prefillFlag = 0
	s.pitch_lag_low_bits_iCDF = nil
	s.pitch_contour_iCDF = nil
	s.psNLSF_CB = nil
	for i := range s.input_quality_bands_Q15 {
		s.input_quality_bands_Q15[i] = 0
	}
	s.input_tilt_Q15 = 0
	s.SNR_dB_Q7 = 0
	for i := range s.VAD_flags {
		s.VAD_flags[i] = 0
	}
	s.LBRR_flag = 0
	for i := range s.LBRR_flags {
		s.LBRR_flags[i] = 0
	}
	s.indices.Reset()
	for i := range s.pulses {
		s.pulses[i] = 0
	}
	for i := range s.inputBuf {
		s.inputBuf[i] = 0
	}
	s.inputBufIx = 0
	s.nFramesPerPacket = 0
	s.nFramesEncoded = 0
	s.nChannelsAPI = 0
	s.nChannelsInternal = 0
	s.channelNb = 0
	s.frames_since_onset = 0
	s.ec_prevSignalType = 0
	s.ec_prevLagIndex = 0
	s.resampler_state.Reset()
	s.useDTX = 0
	s.inDTX = 0
	s.noSpeechCounter = 0
	s.useInBandFEC = 0
	s.LBRR_enabled = 0
	s.LBRR_GainIncreases = 0
	for c := 0; c < SilkConstants.MAX_FRAMES_PER_PACKET; c++ {
		s.indices_LBRR[c].Reset()
		for i := range s.pulses_LBRR[c] {
			s.pulses_LBRR[c][i] = 0
		}
	}
	s.sShape.Reset()
	s.sPrefilt.Reset()
	for i := range s.x_buf {
		s.x_buf[i] = 0
	}
	s.LTPCorr_Q15 = 0
}

func (s *SilkChannelEncoder) silk_control_encoder(encControl *EncControlState, TargetRate_bps int32, allow_bw_switch int32, channelNb int32, force_fs_kHz int32) int32 {
	var fs_kHz int32
	ret := SilkError.SILK_NO_ERROR

	s.useDTX = encControl.useDTX
	s.useCBR = encControl.useCBR
	s.API_fs_Hz = encControl.API_sampleRate
	s.maxInternal_fs_Hz = encControl.maxInternalSampleRate
	s.minInternal_fs_Hz = encControl.minInternalSampleRate
	s.desiredInternal_fs_Hz = encControl.desiredInternalSampleRate
	s.useInBandFEC = encControl.useInBandFEC
	s.nChannelsAPI = encControl.nChannelsAPI
	s.nChannelsInternal = encControl.nChannelsInternal
	s.allow_bandwidth_switch = allow_bw_switch
	s.channelNb = channelNb

	if s.controlled_since_last_payload != 0 && s.prefillFlag == 0 {
		if s.API_fs_Hz != s.prev_API_fs_Hz && s.fs_kHz > 0 {
			ret = s.silk_setup_resamplers(s.fs_kHz)
		}
		return ret
	}

	fs_kHz = s.silk_control_audio_bandwidth(encControl)
	if force_fs_kHz != 0 {
		fs_kHz = force_fs_kHz
	}

	ret = s.silk_setup_resamplers(fs_kHz)
	ret = s.silk_setup_fs(fs_kHz, encControl.payloadSize_ms)
	ret = s.silk_setup_complexity(encControl.complexity)
	s.PacketLoss_perc = encControl.packetLossPercentage
	ret = s.silk_setup_LBRR(TargetRate_bps)
	s.controlled_since_last_payload = 1
	return ret
}

func (s *SilkChannelEncoder) silk_setup_resamplers(fs_kHz int32) int32 {
	ret := int32(0)
	if s.fs_kHz != fs_kHz || s.prev_API_fs_Hz != s.API_fs_Hz {
		if s.fs_kHz == 0 {
			ret += silk_resampler_init(&s.resampler_state, s.API_fs_Hz, fs_kHz*1000, 1)
		} else {
			var x_buf_API_fs_Hz []int16
			var temp_resampler_state SilkResamplerState
			api_buf_samples := int32(0)
			old_buf_samples := int32(0)
			buf_length_ms := int32(0)

			buf_length_ms = silk_LSHIFT(s.nb_subfr*5, 1) + SilkConstants.LA_SHAPE_MS
			old_buf_samples = buf_length_ms * s.fs_kHz
			temp_resampler_state.Reset()
			ret += silk_resampler_init(&temp_resampler_state, Silk_SMULBB(s.fs_kHz, 1000), s.API_fs_Hz, 0)
			api_buf_samples = buf_length_ms * Silk_DIV32_16(s.API_fs_Hz, 1000)
			x_buf_API_fs_Hz = make([]int16, api_buf_samples)
			ret += silk_resampler(&temp_resampler_state, x_buf_API_fs_Hz, 0, s.x_buf[:], 0, old_buf_samples)
			ret += silk_resampler_init(&s.resampler_state, s.API_fs_Hz, Silk_SMULBB(fs_kHz, 1000), 1)
			ret += silk_resampler(&s.resampler_state, s.x_buf[:], 0, x_buf_API_fs_Hz, 0, api_buf_samples)
		}
	}
	s.prev_API_fs_Hz = s.API_fs_Hz
	return ret
}

func (s *SilkChannelEncoder) silk_setup_fs(fs_kHz int32, PacketSize_ms int32) int32 {
	ret := SilkError.SILK_NO_ERROR
	if PacketSize_ms != s.PacketSize_ms {
		if PacketSize_ms != 10 && PacketSize_ms != 20 && PacketSize_ms != 40 && PacketSize_ms != 60 {
			ret = SilkError.SILK_ENC_PACKET_SIZE_NOT_SUPPORTED
		}
		if PacketSize_ms <= 10 {
			s.nFramesPerPacket = 1
			if PacketSize_ms == 10 {
				s.nb_subfr = 2
			} else {
				s.nb_subfr = 1
			}
			s.frame_length = silk_SMULBB(PacketSize_ms, fs_kHz)
			s.pitch_LPC_win_length = silk_SMULBB(SilkConstants.FIND_PITCH_LPC_WIN_MS_2_SF, fs_kHz)
			if s.fs_kHz == 8 {
				s.pitch_contour_iCDF = SilkTables.Silk_pitch_contour_10_ms_NB_iCDF
			} else {
				s.pitch_contour_iCDF = SilkTables.Silk_pitch_contour_10_ms_iCDF
			}
		} else {
			s.nFramesPerPacket = silk_DIV32_16(PacketSize_ms, SilkConstants.MAX_FRAME_LENGTH_MS)
			s.nb_subfr = SilkConstants.MAX_NB_SUBFR
			s.frame_length = silk_SMULBB(20, fs_kHz)
			s.pitch_LPC_win_length = silk_SMULBB(SilkConstants.FIND_PITCH_LPC_WIN_MS, fs_kHz)
			if s.fs_kHz == 8 {
				s.pitch_contour_iCDF = SilkTables.Silk_pitch_contour_NB_iCDF
			} else {
				s.pitch_contour_iCDF = SilkTables.Silk_pitch_contour_iCDF
			}
		}
		s.PacketSize_ms = PacketSize_ms
		s.TargetRate_bps = 0
	}

	OpusAssert(fs_kHz == 8 || fs_kHz == 12 || fs_kHz == 16)
	OpusAssert(s.nb_subfr == 2 || s.nb_subfr == 4)
	if s.fs_kHz != fs_kHz {
		s.sShape.Reset()
		s.sPrefilt.Reset()
		s.sNSQ.Reset()
		for i := range s.prev_NLSFq_Q15 {
			s.prev_NLSFq_Q15[i] = 0
		}
		for i := range s.sLP.In_LP_State {
			s.sLP.In_LP_State[i] = 0
		}
		s.inputBufIx = 0
		s.nFramesEncoded = 0
		s.TargetRate_bps = 0
		s.prevLag = 100
		s.first_frame_after_reset = 1
		s.sPrefilt.lagPrev = 100
		s.sShape.LastGainIndex = 10
		s.sNSQ.lagPrev = 100
		s.sNSQ.prev_gain_Q16 = 65536
		s.prevSignalType = SilkConstants.TYPE_NO_VOICE_ACTIVITY
		s.fs_kHz = fs_kHz
		if s.fs_kHz == 8 {
			if s.nb_subfr == SilkConstants.MAX_NB_SUBFR {
				s.pitch_contour_iCDF = SilkTables.Silk_pitch_contour_NB_iCDF
			} else {
				s.pitch_contour_iCDF = SilkTables.Silk_pitch_contour_10_ms_NB_iCDF
			}
		} else if s.nb_subfr == SilkConstants.MAX_NB_SUBFR {
			s.pitch_contour_iCDF = SilkTables.Silk_pitch_contour_iCDF
		} else {
			s.pitch_contour_iCDF = SilkTables.Silk_pitch_contour_10_ms_iCDF
		}
		if s.fs_kHz == 8 || s.fs_kHz == 12 {
			s.predictLPCOrder = SilkConstants.MIN_LPC_ORDER
			s.psNLSF_CB = SilkTables.Silk_NLSF_CB_NB_MB
		} else {
			s.predictLPCOrder = SilkConstants.MAX_LPC_ORDER
			s.psNLSF_CB = SilkTables.Silk_NLSF_CB_WB
		}
		s.subfr_length = SilkConstants.SUB_FRAME_LENGTH_MS * fs_kHz
		s.frame_length = silk_SMULBB(s.subfr_length, s.nb_subfr)
		s.ltp_mem_length = silk_SMULBB(SilkConstants.LTP_MEM_LENGTH_MS, fs_kHz)
		s.la_pitch = silk_SMULBB(SilkConstants.LA_PITCH_MS, fs_kHz)
		s.max_pitch_lag = silk_SMULBB(18, fs_kHz)
		if s.nb_subfr == SilkConstants.MAX_NB_SUBFR {
			s.pitch_LPC_win_length = silk_SMULBB(SilkConstants.FIND_PITCH_LPC_WIN_MS, fs_kHz)
		} else {
			s.pitch_LPC_win_length = silk_SMULBB(SilkConstants.FIND_PITCH_LPC_WIN_MS_2_SF, fs_kHz)
		}
		if s.fs_kHz == 16 {
			s.mu_LTP_Q9 = silk_SMULWB(TuningParameters.MU_LTP_QUANT_WB, 1<<9)
			s.pitch_lag_low_bits_iCDF = SilkTables.Silk_uniform8_iCDF
		} else if s.fs_kHz == 12 {
			s.mu_LTP_Q9 = silk_SMULWB(TuningParameters.MU_LTP_QUANT_MB, 1<<9)
			s.pitch_lag_low_bits_iCDF = SilkTables.Silk_uniform6_iCDF
		} else {
			s.mu_LTP_Q9 = silk_SMULWB(TuningParameters.MU_LTP_QUANT_NB, 1<<9)
			s.pitch_lag_low_bits_iCDF = SilkTables.Silk_uniform4_iCDF
		}
	}
	OpusAssert(s.subfr_length*s.nb_subfr == s.frame_length)
	return ret
}

func (s *SilkChannelEncoder) silk_setup_complexity(Complexity int32) int32 {
	ret := int32(0)
	OpusAssert(Complexity >= 0 && Complexity <= 10)
	if Complexity < 2 {
		s.pitchEstimationComplexity = SilkConstants.SILK_PE_MIN_COMPLEX
		s.pitchEstimationThreshold_Q16 = Silk_SMULWB(0.8, 1<<16)
		s.pitchEstimationLPCOrder = 6
		s.shapingLPCOrder = 8
		s.la_shape = 3 * s.fs_kHz
		s.nStatesDelayedDecision = 1
		s.useInterpolatedNLSFs = 0
		s.LTPQuantLowComplexity = 1
		s.NLSF_MSVQ_Survivors = 2
		s.warping_Q16 = 0
	} else if Complexity < 4 {
		s.pitchEstimationComplexity = SilkConstants.SILK_PE_MID_COMPLEX
		s.pitchEstimationThreshold_Q16 = Silk_SMULWB(0.76, 1<<16)
		s.pitchEstimationLPCOrder = 8
		s.shapingLPCOrder = 10
		s.la_shape = 5 * s.fs_kHz
		s.nStatesDelayedDecision = 1
		s.useInterpolatedNLSFs = 0
		s.LTPQuantLowComplexity = 0
		s.NLSF_MSVQ_Survivors = 4
		s.warping_Q16 = 0
	} else if Complexity < 6 {
		s.pitchEstimationComplexity = SilkConstants.SILK_PE_MID_COMPLEX
		s.pitchEstimationThreshold_Q16 = Silk_SMULWB(0.74, 1<<16)
		s.pitchEstimationLPCOrder = 10
		s.shapingLPCOrder = 12
		s.la_shape = 5 * s.fs_kHz
		s.nStatesDelayedDecision = 2
		s.useInterpolatedNLSFs = 1
		s.LTPQuantLowComplexity = 0
		s.NLSF_MSVQ_Survivors = 8
		s.warping_Q16 = s.fs_kHz * silk_SMULWB(TuningParameters.WARPING_MULTIPLIER, 1<<16)
	} else if Complexity < 8 {
		s.pitchEstimationComplexity = SilkConstants.SILK_PE_MID_COMPLEX
		s.pitchEstimationThreshold_Q16 = silk_SMULWB(0.72, 1<<16)
		s.pitchEstimationLPCOrder = 12
		s.shapingLPCOrder = 14
		s.la_shape = 5 * s.fs_kHz
		s.nStatesDelayedDecision = 3
		s.useInterpolatedNLSFs = 1
		s.LTPQuantLowComplexity = 0
		s.NLSF_MSVQ_Survivors = 16
		s.warping_Q16 = s.fs_kHz * silk_SMULWB(TuningParameters.WARPING_MULTIPLIER, 1<<16)
	} else {
		s.pitchEstimationComplexity = SilkConstants.SILK_PE_MAX_COMPLEX
		s.pitchEstimationThreshold_Q16 = silk_SMULWB(0.7, 1<<16)
		s.pitchEstimationLPCOrder = 16
		s.shapingLPCOrder = 16
		s.la_shape = 5 * s.fs_kHz
		s.nStatesDelayedDecision = SilkConstants.MAX_DEL_DEC_STATES
		s.useInterpolatedNLSFs = 1
		s.LTPQuantLowComplexity = 0
		s.NLSF_MSVQ_Survivors = 32
		s.warping_Q16 = s.fs_kHz * silk_SMULWB(TuningParameters.WARPING_MULTIPLIER, 1<<16)
	}
	s.pitchEstimationLPCOrder = silk_min_int(s.pitchEstimationLPCOrder, s.predictLPCOrder)
	s.shapeWinLength = SilkConstants.SUB_FRAME_LENGTH_MS*s.fs_kHz + 2*s.la_shape
	s.Complexity = Complexity
	OpusAssert(s.pitchEstimationLPCOrder <= SilkConstants.MAX_FIND_PITCH_LPC_ORDER)
	OpusAssert(s.shapingLPCOrder <= SilkConstants.MAX_SHAPE_LPC_ORDER)
	OpusAssert(s.nStatesDelayedDecision <= SilkConstants.MAX_DEL_DEC_STATES)
	OpusAssert(s.warping_Q16 <= 32767)
	OpusAssert(s.la_shape <= SilkConstants.LA_SHAPE_MAX)
	OpusAssert(s.shapeWinLength <= SilkConstants.SHAPE_LPC_WIN_MAX)
	OpusAssert(s.NLSF_MSVQ_Survivors <= SilkConstants.NLSF_VQ_MAX_SURVIVORS)
	return ret
}

func (s *SilkChannelEncoder) silk_setup_LBRR(TargetRate_bps int32) int32 {
	LBRR_in_previous_packet := s.LBRR_enabled
	s.LBRR_enabled = 0
	if s.useInBandFEC != 0 && s.PacketLoss_perc > 0 {
		var LBRR_rate_thres_bps int32
		if s.fs_kHz == 8 {
			LBRR_rate_thres_bps = SilkConstants.LBRR_NB_MIN_RATE_BPS
		} else if s.fs_kHz == 12 {
			LBRR_rate_thres_bps = SilkConstants.LBRR_MB_MIN_RATE_BPS
		} else {
			LBRR_rate_thres_bps = SilkConstants.LBRR_WB_MIN_RATE_BPS
		}
		LBRR_rate_thres_bps = Silk_SMULWB(Silk_MUL(LBRR_rate_thres_bps, 125-Silk_min(s.PacketLoss_perc, 25)), Silk_SMULWB(0.01, 1<<16))
		if TargetRate_bps > LBRR_rate_thres_bps {
			if LBRR_in_previous_packet == 0 {
				s.LBRR_GainIncreases = 7
			} else {
				s.LBRR_GainIncreases = Silk_max_int(7-Silk_SMULWB(s.PacketLoss_perc, Silk_SMULWB(0.4, 1<<16)), 2)
			}
			s.LBRR_enabled = 1
		}
	}
	return SilkError.SILK_NO_ERROR
}

func (s *SilkChannelEncoder) silk_control_audio_bandwidth(encControl *EncControlState) int32 {
	fs_kHz := s.fs_kHz
	fs_Hz := silk_SMULBB(fs_kHz, 1000)
	if fs_Hz == 0 {
		fs_Hz = Silk_min(s.desiredInternal_fs_Hz, s.API_fs_Hz)
		fs_kHz = Silk_DIV32_16(fs_Hz, 1000)
	} else if fs_Hz > s.API_fs_Hz || fs_Hz > s.maxInternal_fs_Hz || fs_Hz < s.minInternal_fs_Hz {
		fs_Hz = s.API_fs_Hz
		fs_Hz = Silk_min(fs_Hz, s.maxInternal_fs_Hz)
		fs_Hz = Silk_max(fs_Hz, s.minInternal_fs_Hz)
		fs_kHz = Silk_DIV32_16(fs_Hz, 1000)
	} else {
		if s.sLP.transition_frame_no >= SilkConstants.TRANSITION_FRAMES {
			s.sLP.mode = 0
		}
		if s.allow_bandwidth_switch != 0 || encControl.opusCanSwitch != 0 {
			if silk_SMULBB(s.fs_kHz, 1000) > s.desiredInternal_fs_Hz {
				if s.sLP.mode == 0 {
					s.sLP.transition_frame_no = SilkConstants.TRANSITION_FRAMES
					for i := range s.sLP.In_LP_State {
						s.sLP.In_LP_State[i] = 0
					}
				}
				if encControl.opusCanSwitch != 0 {
					s.sLP.mode = 0
					if s.fs_kHz == 16 {
						fs_kHz = 12
					} else {
						fs_kHz = 8
					}
				} else if s.sLP.transition_frame_no <= 0 {
					encControl.switchReady = 1
					encControl.maxBits -= encControl.maxBits * 5 / (encControl.payloadSize_ms + 5)
				} else {
					s.sLP.mode = -2
				}
			} else if silk_SMULBB(s.fs_kHz, 1000) < s.desiredInternal_fs_Hz {
				if encControl.opusCanSwitch != 0 {
					if s.fs_kHz == 8 {
						fs_kHz = 12
					} else {
						fs_kHz = 16
					}
					s.sLP.transition_frame_no = 0
					for i := range s.sLP.In_LP_State {
						s.sLP.In_LP_State[i] = 0
					}
					s.sLP.mode = 1
				} else if s.sLP.mode == 0 {
					encControl.switchReady = 1
					encControl.maxBits -= encControl.maxBits * 5 / (encControl.payloadSize_ms + 5)
				} else {
					s.sLP.mode = 1
				}
			} else if s.sLP.mode < 0 {
				s.sLP.mode = 1
			}
		}
	}
	return fs_kHz
}

func (s *SilkChannelEncoder) silk_control_SNR(TargetRate_bps int32) int32 {
	var k int
	ret := SilkError.SILK_NO_ERROR
	var frac_Q6 int32
	var rateTable []int32
	TargetRate_bps = silk_LIMIT(TargetRate_bps, SilkConstants.MIN_TARGET_RATE_BPS, SilkConstants.MAX_TARGET_RATE_BPS)
	if TargetRate_bps != s.TargetRate_bps {
		s.TargetRate_bps = TargetRate_bps
		if s.fs_kHz == 8 {
			rateTable = SilkTables.Silk_TargetRate_table_NB
		} else if s.fs_kHz == 12 {
			rateTable = SilkTables.Silk_TargetRate_table_MB
		} else {
			rateTable = SilkTables.Silk_TargetRate_table_WB
		}
		if s.nb_subfr == 2 {
			TargetRate_bps -= TuningParameters.REDUCE_BITRATE_10_MS_BPS
		}
		for k = 1; k < SilkConstants.TARGET_RATE_TAB_SZ; k++ {
			if TargetRate_bps <= rateTable[k] {
				frac_Q6 = silk_DIV32(silk_LSHIFT(TargetRate_bps-rateTable[k-1], 6), rateTable[k]-rateTable[k-1])
				s.SNR_dB_Q7 = silk_LSHIFT(SilkTables.Silk_SNR_table_Q1[k-1], 6) + silk_MUL(frac_Q6, SilkTables.Silk_SNR_table_Q1[k]-SilkTables.Silk_SNR_table_Q1[k-1])
				break
			}
		}
	}
	return ret
}

func (s *SilkChannelEncoder) silk_encode_do_VAD() {
	silk_VAD_GetSA_Q8(s, s.inputBuf[:], 1)
	if s.speech_activity_Q8 < ailk_SMULWB(TuningParameters.SPEECH_ACTIVITY_DTX_THRES, 1<<8) {
		s.indices.signalType = SilkConstants.TYPE_NO_VOICE_ACTIVITY
		s.noSpeechCounter++
		if s.noSpeechCounter < SilkConstants.NB_SPEECH_FRAMES_BEFORE_DTX {
			s.inDTX = 0
		} else if s.noSpeechCounter > SilkConstants.MAX_CONSECUTIVE_DTX+SilkConstants.NB_SPEECH_FRAMES_BEFORE_DTX {
			s.noSpeechCounter = SilkConstants.NB_SPEECH_FRAMES_BEFORE_DTX
			s.inDTX = 0
		}
		s.VAD_flags[s.nFramesEncoded] = 0
	} else {
		s.noSpeechCounter = 0
		s.inDTX = 0
		s.indices.signalType = SilkConstants.TYPE_UNVOICED
		s.VAD_flags[s.nFramesEncoded] = 1
	}
}

func (s *SilkChannelEncoder) silk_encode_frame(pnBytesOut *int32, psRangeEnc *EntropyCoder, condCoding int32, maxBits int32, useCBR int32) int32 {
	sEncCtrl := NewSilkEncoderControl()
	var iter, maxIter, found_upper, found_lower, ret int32
	var x_frame int32
	sRangeEnc_copy := &EntropyCoder{}
	sRangeEnc_copy2 := &EntropyCoder{}
	sNSQ_copy := &SilkNSQState{}
	sNSQ_copy2 := &SilkNSQState{}
	var nBits, nBits_lower, nBits_upper, gainMult_lower, gainMult_upper int32
	var gainsID, gainsID_lower, gainsID_upper int32
	var gainMult_Q8 int16
	var ec_prevLagIndex_copy int16
	var ec_prevSignalType_copy int32
	var LastGainIndex_copy2 byte
	var seed_copy byte
	nBits_lower, nBits_upper, gainMult_lower, gainMult_upper = 0, 0, 0, 0
	s.indices.Seed = byte(s.frameCounter & 3)
	s.frameCounter++
	x_frame = s.ltp_mem_length
	s.sLP.Silk_LP_variable_cutoff(s.inputBuf[:], 1, s.frame_length)
	copy(s.x_buf[x_frame+SilkConstants.LA_SHAPE_MS*s.fs_kHz:], s.inputBuf[1:1+s.frame_length])
	if s.prefillFlag == 0 {
		var xfw_Q3 []int32
		var res_pitch []int16
		var ec_buf_copy []byte
		var res_pitch_frame int32
		res_pitch = make([]int16, s.la_pitch+s.frame_length+s.ltp_mem_length)
		res_pitch_frame = s.ltp_mem_length
		FindPitchLags.Silk_find_pitch_lags(s, sEncCtrl, res_pitch, s.x_buf[:], x_frame)
		NoiseShapeAnalysis.Silk_noise_shape_analysis(s, sEncCtrl, res_pitch, res_pitch_frame, s.x_buf[:], x_frame)
		FindPredCoefs.Silk_find_pred_coefs(s, sEncCtrl, res_pitch, s.x_buf[:], x_frame, condCoding)
		ProcessGains.Silk_process_gains(s, sEncCtrl, condCoding)
		xfw_Q3 = make([]int32, s.frame_length)
		silk_prefilter(s, sEncCtrl, xfw_Q3, s.x_buf[:], x_frame)
		s.silk_LBRR_encode(sEncCtrl, xfw_Q3, condCoding)
		maxIter = 6
		gainMult_Q8 = int16(silk_SMULWB(1, 1<<8))
		found_lower = 0
		found_upper = 0
		gainsID = GainQuantization.Silk_gains_ID(s.indices.GainsIndices[:], s.nb_subfr)
		gainsID_lower = -1
		gainsID_upper = -1
		*sRangeEnc_copy = *psRangeEnc
		*sNSQ_copy = s.sNSQ
		seed_copy = s.indices.Seed
		ec_prevLagIndex_copy = s.ec_prevLagIndex
		ec_prevSignalType_copy = s.ec_prevSignalType
		ec_buf_copy = make([]byte, 1275)
		for iter = 0; ; iter++ {
			if gainsID == gainsID_lower {
				nBits = nBits_lower
			} else if gainsID == gainsID_upper {
				nBits = nBits_upper
			} else {
				if iter > 0 {
					*psRangeEnc = *sRangeEnc_copy
					s.sNSQ = *sNSQ_copy
					s.indices.Seed = seed_copy
					s.ec_prevLagIndex = ec_prevLagIndex_copy
					s.ec_prevSignalType = ec_prevSignalType_copy
				}
				if s.nStatesDelayedDecision > 1 || s.warping_Q16 > 0 {
					s.sNSQ.Silk_NSQ_del_dec(s, &s.indices, xfw_Q3, s.pulses[:], sEncCtrl.PredCoef_Q12[:], sEncCtrl.LTPCoef_Q14[:], sEncCtrl.AR2_Q13[:], sEncCtrl.HarmShapeGain_Q14, sEncCtrl.Tilt_Q14, sEncCtrl.LF_shp_Q14, sEncCtrl.Gains_Q16[:], sEncCtrl.pitchL[:], sEncCtrl.Lambda_Q10, sEncCtrl.LTP_scale_Q14)
				} else {
					s.sNSQ.Silk_NSQ(s, &s.indices, xfw_Q3, s.pulses[:], sEncCtrl.PredCoef_Q12[:], sEncCtrl.LTPCoef_Q14[:], sEncCtrl.AR2_Q13[:], sEncCtrl.HarmShapeGain_Q14, sEncCtrl.Tilt_Q14, sEncCtrl.LF_shp_Q14, sEncCtrl.Gains_Q16[:], sEncCtrl.pitchL[:], sEncCtrl.Lambda_Q10, sEncCtrl.LTP_scale_Q14)
				}
				EncodeIndices.Silk_encode_indices(s, psRangeEnc, s.nFramesEncoded, 0, condCoding)
				EncodePulses.Silk_encode_pulses(psRangeEnc, s.indices.signalType, s.indices.quantOffsetType, s.pulses[:], s.frame_length)
				nBits = psRangeEnc.Tell()
				if useCBR == 0 && iter == 0 && nBits <= maxBits {
					break
				}
			}
			if iter == maxIter {
				if found_lower != 0 && (gainsID == gainsID_lower || nBits > maxBits) {
					*psRangeEnc = *sRangeEnc_copy2
					copy(psRangeEnc.Buf, ec_buf_copy)
					psRangeEnc.Offs = sRangeEnc_copy2.Offs
					s.sNSQ = *sNSQ_copy2
					s.sShape.LastGainIndex = LastGainIndex_copy2
				}
				break
			}
			if nBits > maxBits {
				if found_lower == 0 && iter >= 2 {
					sEncCtrl.Lambda_Q10 += sEncCtrl.Lambda_Q10 >> 1
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
					*sRangeEnc_copy2 = *psRangeEnc
					copy(ec_buf_copy, psRangeEnc.Buf)
					*sNSQ_copy2 = s.sNSQ
					LastGainIndex_copy2 = s.sShape.LastGainIndex
				}
			} else {
				break
			}
			if (found_lower & found_upper) == 0 {
				gain_factor_Q16 := silk_log2lin(silk_LSHIFT(nBits-maxBits, 7)/s.frame_length + Silk_SMULWB(16, 1<<7))
				if gain_factor_Q16 > silk_SMULWB(2, 1<<16) {
					gain_factor_Q16 = silk_SMULWB(2, 1<<16)
				}
				if nBits > maxBits {
					if gain_factor_Q16 < silk_SMULWB(1.3, 1<<16) {
						gain_factor_Q16 = silk_SMULWB(1.3, 1<<16)
					}
				}
				gainMult_Q8 = int16(silk_SMULWB(gain_factor_Q16, int32(gainMult_Q8)))
			} else {
				gainMult_Q8 = int16(gainMult_lower + silk_DIV32_16(silk_MUL(gainMult_upper-gainMult_lower, maxBits-nBits_lower), nBits_upper-nBits_lower))
				if gainMult_Q8 > int16(gainMult_lower+(gainMult_upper-gainMult_lower)>>2) {
					gainMult_Q8 = int16(gainMult_lower + (gainMult_upper-gainMult_lower)>>2)
				} else if gainMult_Q8 < int16(gainMult_upper-(gainMult_upper-gainMult_lower)>>2) {
					gainMult_Q8 = int16(gainMult_upper - (gainMult_upper-gainMult_lower)>>2)
				}
			}
			for i := int32(0); i < s.nb_subfr; i++ {
				sEncCtrl.Gains_Q16[i] = silk_LSHIFT_SAT32(silk_SMULWB(sEncCtrl.GainsUnq_Q16[i], int32(gainMult_Q8)), 8)
			}
			s.sShape.LastGainIndex = sEncCtrl.lastGainIndexPrev
			GainQuantization.Silk_gains_quant(s.indices.GainsIndices[:], sEncCtrl.Gains_Q16[:], &s.sShape.LastGainIndex, Silk_SMULBB(condCoding, SilkConstants.CODE_CONDITIONALLY), s.nb_subfr)
			gainsID = GainQuantization.Silk_gains_ID(s.indices.GainsIndices[:], s.nb_subfr)
		}
	}
	copy(s.x_buf[:], s.x_buf[s.frame_length:])
	if s.prefillFlag != 0 {
		*pnBytesOut = 0
		return ret
	}
	s.prevLag = sEncCtrl.pitchL[s.nb_subfr-1]
	s.prevSignalType = s.indices.signalType
	s.first_frame_after_reset = 0
	*pnBytesOut = int32(uint32(psRangeEnc.Tell()+7) >> 3)
	return ret
}

func (s *SilkChannelEncoder) silk_LBRR_encode(thisCtrl *SilkEncoderControl, xfw_Q3 []int32, condCoding int32) {
	if s.LBRR_enabled != 0 && s.speech_activity_Q8 > Silk_SMULWB(TuningParameters.LBRR_SPEECH_ACTIVITY_THRES, 1<<8) {
		s.LBRR_flags[s.nFramesEncoded] = 1
		psIndices_LBRR := s.indices_LBRR[s.nFramesEncoded]
		sNSQ_LBRR := &SilkNSQState{}
		*sNSQ_LBRR = s.sNSQ
		*psIndices_LBRR = s.indices
		TempGains_Q16 := make([]int32, s.nb_subfr)
		copy(TempGains_Q16, thisCtrl.Gains_Q16[:])
		if s.nFramesEncoded == 0 || s.LBRR_flags[s.nFramesEncoded-1] == 0 {
			psIndices_LBRR.GainsIndices[0] = byte(Silk_min_int(int(psIndices_LBRR.GainsIndices[0])+s.LBRR_GainIncreases, SilkConstants.N_LEVELS_QGAIN-1))
		}
		gainIndex := s.LBRRprevLastGainIndex
		GainQuantization.Silk_gains_dequant(thisCtrl.Gains_Q16[:], psIndices_LBRR.GainsIndices[:], &gainIndex, Silk_SMULBB(condCoding, SilkConstants.CODE_CONDITIONALLY), s.nb_subfr)
		s.LBRRprevLastGainIndex = gainIndex
		if s.nStatesDelayedDecision > 1 || s.warping_Q16 > 0 {
			sNSQ_LBRR.Silk_NSQ_del_dec(s, psIndices_LBRR, xfw_Q3, s.pulses_LBRR[s.nFramesEncoded][:], thisCtrl.PredCoef_Q12[:], thisCtrl.LTPCoef_Q14[:], thisCtrl.AR2_Q13[:], thisCtrl.HarmShapeGain_Q14, thisCtrl.Tilt_Q14, thisCtrl.LF_shp_Q14, thisCtrl.Gains_Q16[:], thisCtrl.pitchL[:], thisCtrl.Lambda_Q10, thisCtrl.LTP_scale_Q14)
		} else {
			sNSQ_LBRR.Silk_NSQ(s, psIndices_LBRR, xfw_Q3, s.pulses_LBRR[s.nFramesEncoded][:], thisCtrl.PredCoef_Q12[:], thisCtrl.LTPCoef_Q14[:], thisCtrl.AR2_Q13[:], thisCtrl.HarmShapeGain_Q14, thisCtrl.Tilt_Q14, thisCtrl.LF_shp_Q14, thisCtrl.Gains_Q16[:], thisCtrl.pitchL[:], thisCtrl.Lambda_Q10, thisCtrl.LTP_scale_Q14)
		}
		copy(thisCtrl.Gains_Q16[:], TempGains_Q16)
	}
}
