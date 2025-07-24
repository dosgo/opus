package opus

type SilkChannelDecoder struct {
	prev_gain_Q16           int
	exc_Q14                 [MAX_FRAME_LENGTH]int
	sLPC_Q14_buf            [MAX_LPC_ORDER]int
	outBuf                  [MAX_FRAME_LENGTH + 2*MAX_SUB_FRAME_LENGTH]int16
	lagPrev                 int
	LastGainIndex           int8
	fs_kHz                  int
	fs_API_hz               int
	nb_subfr                int
	frame_length            int
	subfr_length            int
	ltp_mem_length          int
	LPC_order               int
	prevNLSF_Q15            [MAX_LPC_ORDER]int16
	first_frame_after_reset int
	pitch_lag_low_bits_iCDF []int16
	pitch_contour_iCDF      []int16
	nFramesDecoded          int
	nFramesPerPacket        int
	ec_prevSignalType       int
	ec_prevLagIndex         int16
	VAD_flags               [MAX_FRAMES_PER_PACKET]int
	LBRR_flag               int
	LBRR_flags              [MAX_FRAMES_PER_PACKET]int
	resampler_state         SilkResamplerState
	psNLSF_CB               *NLSFCodebook
	indices                 SideInfoIndices
	sCNG                    CNGState
	lossCnt                 int
	prevSignalType          int
	sPLC                    PLCStruct
}

func (d *SilkChannelDecoder) Reset() {
	d.prev_gain_Q16 = 0
	d.exc_Q14 = [MAX_FRAME_LENGTH]int{}
	d.sLPC_Q14_buf = [MAX_LPC_ORDER]int{}
	d.outBuf = [MAX_FRAME_LENGTH + 2*MAX_SUB_FRAME_LENGTH]int16{}
	d.lagPrev = 0
	d.LastGainIndex = 0
	d.fs_kHz = 0
	d.fs_API_hz = 0
	d.nb_subfr = 0
	d.frame_length = 0
	d.subfr_length = 0
	d.ltp_mem_length = 0
	d.LPC_order = 0
	d.prevNLSF_Q15 = [MAX_LPC_ORDER]int16{}
	d.first_frame_after_reset = 0
	d.pitch_lag_low_bits_iCDF = nil
	d.pitch_contour_iCDF = nil
	d.nFramesDecoded = 0
	d.nFramesPerPacket = 0
	d.ec_prevSignalType = 0
	d.ec_prevLagIndex = 0
	d.VAD_flags = [MAX_FRAMES_PER_PACKET]int{}
	d.LBRR_flag = 0
	d.LBRR_flags = [MAX_FRAMES_PER_PACKET]int{}
	d.resampler_state.Reset()
	d.psNLSF_CB = nil
	d.indices.Reset()
	d.sCNG.Reset()
	d.lossCnt = 0
	d.prevSignalType = 0
	d.sPLC.Reset()
}

func (d *SilkChannelDecoder) silk_init_decoder() int {
	d.Reset()
	d.first_frame_after_reset = 1
	d.prev_gain_Q16 = 65536
	d.silk_CNG_Reset()
	d.silk_PLC_Reset()
	return 0
}

func (d *SilkChannelDecoder) silk_CNG_Reset() {
	NLSF_step_Q15 := Inlines_silk_DIV32_16(32767, d.LPC_order+1)
	NLSF_acc_Q15 := 0
	for i := 0; i < d.LPC_order; i++ {
		NLSF_acc_Q15 += NLSF_step_Q15
		d.sCNG.CNG_smth_NLSF_Q15[i] = int16(NLSF_acc_Q15)
	}
	d.sCNG.CNG_smth_Gain_Q16 = 0
	d.sCNG.rand_seed = 3176576
}

func (d *SilkChannelDecoder) silk_PLC_Reset() {
	d.sPLC.pitchL_Q8 = Inlines_silk_LSHIFT(d.frame_length, 8-1)
	d.sPLC.prevGain_Q16[0] = 1 << 16
	d.sPLC.prevGain_Q16[1] = 1 << 16
	d.sPLC.subfr_length = 20
	d.sPLC.nb_subfr = 2
}

func (d *SilkChannelDecoder) silk_decoder_set_fs(fs_kHz, fs_API_Hz int) int {
	ret := 0
	Inlines_OpusAssert(fs_kHz == 8 || fs_kHz == 12 || fs_kHz == 16)
	Inlines_OpusAssert(d.nb_subfr == SilkConstants_MAX_NB_SUBFR || d.nb_subfr == SilkConstants_MAX_NB_SUBFR/2)

	subfr_length := Inlines_silk_SMULBB(SilkConstants_SUB_FRAME_LENGTH_MS, fs_kHz)
	frame_length := Inlines_silk_SMULBB(d.nb_subfr, subfr_length)

	if d.fs_kHz != fs_kHz || d.fs_API_hz != fs_API_Hz {
		ret += Resampler_silk_resampler_init(&d.resampler_state, Inlines_silk_SMULBB(fs_kHz, 1000), fs_API_Hz, 0)
		d.fs_API_hz = fs_API_Hz
	}

	if d.fs_kHz != fs_kHz || frame_length != d.frame_length {
		if fs_kHz == 8 {
			if d.nb_subfr == MAX_NB_SUBFR {
				d.pitch_contour_iCDF = silk_pitch_contour_NB_iCDF
			} else {
				d.pitch_contour_iCDF = silk_pitch_contour_10_ms_NB_iCDF
			}
		} else if d.nb_subfr == MAX_NB_SUBFR {
			d.pitch_contour_iCDF = silk_pitch_contour_iCDF
		} else {
			d.pitch_contour_iCDF = silk_pitch_contour_10_ms_iCDF
		}
		if d.fs_kHz != fs_kHz {
			d.ltp_mem_length = Inlines_silk_SMULBB(LTP_MEM_LENGTH_MS, fs_kHz)
			if fs_kHz == 8 || fs_kHz == 12 {
				d.LPC_order = MIN_LPC_ORDER
				d.psNLSF_CB = SilkTables_silk_NLSF_CB_NB_MB
			} else {
				d.LPC_order = MAX_LPC_ORDER
				d.psNLSF_CB = SilkTables_silk_NLSF_CB_WB
			}
			switch fs_kHz {
			case 16:
				d.pitch_lag_low_bits_iCDF = SilkTables_silk_uniform8_iCDF
			case 12:
				d.pitch_lag_low_bits_iCDF = SilkTables_silk_uniform6_iCDF
			case 8:
				d.pitch_lag_low_bits_iCDF = SilkTables_silk_uniform4_iCDF
			default:
				Inlines_OpusAssert(false)
			}
			d.first_frame_after_reset = 1
			d.lagPrev = 100
			d.LastGainIndex = 10
			d.prevSignalType = TYPE_NO_VOICE_ACTIVITY
			d.outBuf = [MAX_FRAME_LENGTH + 2*MAX_SUB_FRAME_LENGTH]int16{}
			d.sLPC_Q14_buf = [MAX_LPC_ORDER]int{}
		}
		d.fs_kHz = fs_kHz
		d.frame_length = frame_length
		d.subfr_length = subfr_length
	}

	Inlines_OpusAssert(d.frame_length > 0 && d.frame_length <= MAX_FRAME_LENGTH)
	return ret
}

func (d *SilkChannelDecoder) silk_decode_frame(psRangeDec *EntropyCoder, pOut []int16, pOut_ptr int, pN *int, lostFlag, condCoding int) int {
	thisCtrl := SilkDecoderControl{}
	L := d.frame_length
	thisCtrl.LTP_scale_Q14 = 0

	Inlines_OpusAssert(L > 0 && L <= MAX_FRAME_LENGTH)
	ret := 0

	if lostFlag == DecoderAPIFlag_FLAG_DECODE_NORMAL || (lostFlag == DecoderAPIFlag_FLAG_DECODE_LBRR && d.LBRR_flags[d.nFramesDecoded] == 1) {
		pulses := make([]int16, (L+SHELL_CODEC_FRAME_LENGTH-1)&^(SHELL_CODEC_FRAME_LENGTH-1))
		DecodeIndices_silk_decode_indices(d, psRangeDec, d.nFramesDecoded, lostFlag, condCoding)
		DecodePulses_silk_decode_pulses(psRangeDec, pulses, d.indices.signalType, d.indices.quantOffsetType, d.frame_length)
		DecodeParameters_silk_decode_parameters(d, &thisCtrl, condCoding)
		DecodeCore_silk_decode_core(d, &thisCtrl, pOut, pOut_ptr, pulses)
		PLC_silk_PLC(d, &thisCtrl, pOut, pOut_ptr, 0)
		d.lossCnt = 0
		d.prevSignalType = d.indices.signalType
		Inlines_OpusAssert(d.prevSignalType >= 0 && d.prevSignalType <= 2)
		d.first_frame_after_reset = 0
	} else {
		PLC_silk_PLC(d, &thisCtrl, pOut, pOut_ptr, 1)
	}

	mv_len := d.ltp_mem_length - d.frame_length
	copy(d.outBuf[:mv_len], d.outBuf[d.frame_length:d.frame_length+mv_len])
	copy(d.outBuf[mv_len:], pOut[pOut_ptr:pOut_ptr+d.frame_length])
	CNG_silk_CNG(d, &thisCtrl, pOut, pOut_ptr, L)
	PLC_silk_PLC_glue_frames(d, pOut, pOut_ptr, L)
	d.lagPrev = thisCtrl.pitchL[d.nb_subfr-1]
	*pN = L
	return ret
}
