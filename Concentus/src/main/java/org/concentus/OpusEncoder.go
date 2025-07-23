package opus
import (
	"errors"
	"math"
)

type OpusApplication int

const (
	OPUS_APPLICATION_UNIMPLEMENTED OpusApplication = iota
	OPUS_APPLICATION_VOIP
	OPUS_APPLICATION_AUDIO
	OPUS_APPLICATION_RESTRICTED_LOWDELAY
)

type OpusSignal int

const (
	OPUS_SIGNAL_UNKNOWN OpusSignal = iota
	OPUS_SIGNAL_AUTO
	OPUS_SIGNAL_VOICE
	OPUS_SIGNAL_MUSIC
)

type OpusBandwidth int

const (
	OPUS_BANDWIDTH_UNKNOWN OpusBandwidth = iota
	OPUS_BANDWIDTH_NARROWBAND
	OPUS_BANDWIDTH_MEDIUMBAND
	OPUS_BANDWIDTH_WIDEBAND
	OPUS_BANDWIDTH_SUPERWIDEBAND
	OPUS_BANDWIDTH_FULLBAND
	OPUS_BANDWIDTH_AUTO
)

type OpusMode int

const (
	MODE_UNKNOWN OpusMode = iota
	MODE_AUTO
	MODE_SILK_ONLY
	MODE_HYBRID
	MODE_CELT_ONLY
)

type OpusFramesize int

const (
	OPUS_FRAMESIZE_UNKNOWN OpusFramesize = iota
	OPUS_FRAMESIZE_ARG
	OPUS_FRAMESIZE_2_5_MS
	OPUS_FRAMESIZE_5_MS
	OPUS_FRAMESIZE_10_MS
	OPUS_FRAMESIZE_20_MS
	OPUS_FRAMESIZE_40_MS
	OPUS_FRAMESIZE_60_MS
	OPUS_FRAMESIZE_VARIABLE
)

type EncControlState struct {
	nChannelsAPI          int
	nChannelsInternal     int
	API_sampleRate        int
	maxInternalSampleRate int
	minInternalSampleRate int
	desiredInternalSampleRate int
	payloadSize_ms        int
	bitRate               int
	packetLossPercentage  int
	complexity            int
	useInBandFEC          int
	useDTX                int
	useCBR                int
	reducedDependency     int
	allowBandwidthSwitch  int
	opusCanSwitch         int
	internalSampleRate    int
	toMono                int
	stereoWidth_Q14       int
	inWBmodeWithoutVariableLP int
	switchReady           int
}

func (e *EncControlState) Reset() {
	*e = EncControlState{}
}

type TonalityAnalysisState struct {
	tonality             []float32
	tonality_slope       []float32
	noisiness            []float32
	window               []float32
	analysis_buf         []float32
	peak_rate            float32
	prev_tonality        float32
	prev_tonality_slope  float32
	prev_noisiness       float32
	emph_coeff           float32
	arch                 int
	Fs                   int
	frame_size           int
	subframe_size        int
	read_pos             int
	read_subframe        int
	remaining            int
	band_energy          []float32
	low_energy_avg       float32
	high_energy_avg      float32
	energy_avg           float32
	enabled              bool
}

func (t *TonalityAnalysisState) Reset() {
	t.enabled = false
}

type StereoWidthState struct {
	mem                  float32
	coeff                float32
	smoothed_width       float32
	max_freq             float32
}

func (s *StereoWidthState) Reset() {
	s.mem = 0
	s.coeff = 0
	s.smoothed_width = 0
	s.max_freq = 0
}

type OpusEncoder struct {
	silk_mode            EncControlState
	application          OpusApplication
	channels             int
	delay_compensation  int
	force_channels       int
	signal_type          OpusSignal
	user_bandwidth       OpusBandwidth
	max_bandwidth        OpusBandwidth
	user_forced_mode     OpusMode
	voice_ratio          int
	Fs                   int
	use_vbr              int
	vbr_constraint       int
	variable_duration    OpusFramesize
	bitrate_bps          int
	user_bitrate_bps     int
	lsb_depth            int
	encoder_buffer       int
	lfe                  int
	analysis             TonalityAnalysisState
	stream_channels      int
	hybrid_stereo_width_Q14 int16
	variable_HP_smth2_Q15 int
	prev_HB_gain         int
	hp_mem               [4]int
	mode                 OpusMode
	prev_mode            OpusMode
	prev_channels        int
	prev_framesize       int
	bandwidth            OpusBandwidth
	silk_bw_switch       int
	first                int
	energy_masking       []int
	width_mem            StereoWidthState
	delay_buffer         [MAX_ENCODER_BUFFER * 2]int16
	detected_bandwidth   OpusBandwidth
	rangeFinal           int
	SilkEncoder          SilkEncoder
	Celt_Encoder         CeltEncoder
}

func (st *OpusEncoder) reset() {
	st.silk_mode.Reset()
	st.application = OPUS_APPLICATION_UNIMPLEMENTED
	st.channels = 0
	st.delay_compensation = 0
	st.force_channels = 0
	st.signal_type = OPUS_SIGNAL_UNKNOWN
	st.user_bandwidth = OPUS_BANDWIDTH_UNKNOWN
	st.max_bandwidth = OPUS_BANDWIDTH_UNKNOWN
	st.user_forced_mode = MODE_UNKNOWN
	st.voice_ratio = 0
	st.Fs = 0
	st.use_vbr = 0
	st.vbr_constraint = 0
	st.variable_duration = OPUS_FRAMESIZE_UNKNOWN
	st.bitrate_bps = 0
	st.user_bitrate_bps = 0
	st.lsb_depth = 0
	st.encoder_buffer = 0
	st.lfe = 0
	st.analysis.Reset()
	st.PartialReset()
}

func (st *OpusEncoder) PartialReset() {
	st.stream_channels = 0
	st.hybrid_stereo_width_Q14 = 0
	st.variable_HP_smth2_Q15 = 0
	st.prev_HB_gain = Q15ONE
	for i := range st.hp_mem {
		st.hp_mem[i] = 0
	}
	st.mode = MODE_UNKNOWN
	st.prev_mode = MODE_UNKNOWN
	st.prev_channels = 0
	st.prev_framesize = 0
	st.bandwidth = OPUS_BANDWIDTH_UNKNOWN
	st.silk_bw_switch = 0
	st.first = 0
	st.energy_masking = nil
	st.width_mem.Reset()
	for i := range st.delay_buffer {
		st.delay_buffer[i] = 0
	}
	st.detected_bandwidth = OPUS_BANDWIDTH_UNKNOWN
	st.rangeFinal = 0
}

func (st *OpusEncoder) resetState() {
	dummy := EncControlState{}
	st.analysis.Reset()
	st.PartialReset()
	st.Celt_Encoder.ResetState()
	silk_InitEncoder(&st.SilkEncoder, &dummy)
	st.stream_channels = st.channels
	st.hybrid_stereo_width_Q14 = 1 << 14
	st.prev_HB_gain = Q15ONE
	st.first = 1
	st.mode = MODE_HYBRID
	st.bandwidth = OPUS_BANDWIDTH_FULLBAND
	st.variable_HP_smth2_Q15 = silk_LSHIFT(silk_lin2log(VARIABLE_HP_MIN_CUTOFF_HZ), 8)
}

func NewOpusEncoder(Fs, channels int, application OpusApplication) (*OpusEncoder, error) {
	if Fs != 48000 && Fs != 24000 && Fs != 16000 && Fs != 12000 && Fs != 8000 {
		return nil, errors.New("Sample rate is invalid (must be 8/12/16/24/48 Khz)")
	}
	if channels != 1 && channels != 2 {
		return nil, errors.New("Number of channels must be 1 or 2")
	}
	st := &OpusEncoder{}
	ret := st.opus_init_encoder(Fs, channels, application)
	if ret != OPUS_OK {
		if ret == OPUS_BAD_ARG {
			return nil, errors.New("OPUS_BAD_ARG when creating encoder")
		}
		return nil, errors.New("Error while initializing encoder")
	}
	return st, nil
}

func (st *OpusEncoder) opus_init_encoder(Fs, channels int, application OpusApplication) int {
	if (Fs != 48000 && Fs != 24000 && Fs != 16000 && Fs != 12000 && Fs != 8000) || (channels != 1 && channels != 2) || application == OPUS_APPLICATION_UNIMPLEMENTED {
		return OPUS_BAD_ARG
	}
	st.reset()
	st.stream_channels = channels
	st.channels = channels
	st.Fs = Fs
	ret := silk_InitEncoder(&st.SilkEncoder, &st.silk_mode)
	if ret != 0 {
		return OPUS_INTERNAL_ERROR
	}
	st.silk_mode.nChannelsAPI = channels
	st.silk_mode.nChannelsInternal = channels
	st.silk_mode.API_sampleRate = Fs
	st.silk_mode.maxInternalSampleRate = 16000
	st.silk_mode.minInternalSampleRate = 8000
	st.silk_mode.desiredInternalSampleRate = 16000
	st.silk_mode.payloadSize_ms = 20
	st.silk_mode.bitRate = 25000
	st.silk_mode.packetLossPercentage = 0
	st.silk_mode.complexity = 9
	st.silk_mode.useInBandFEC = 0
	st.silk_mode.useDTX = 0
	st.silk_mode.useCBR = 0
	st.silk_mode.reducedDependency = 0
	err := st.Celt_Encoder.celt_encoder_init(Fs, channels)
	if err != OPUS_OK {
		return OPUS_INTERNAL_ERROR
	}
	st.Celt_Encoder.SetSignalling(0)
	st.Celt_Encoder.SetComplexity(st.silk_mode.complexity)
	st.use_vbr = 1
	st.vbr_constraint = 1
	st.user_bitrate_bps = OPUS_AUTO
	st.bitrate_bps = 3000 + Fs*channels
	st.application = application
	st.signal_type = OPUS_SIGNAL_AUTO
	st.user_bandwidth = OPUS_BANDWIDTH_AUTO
	st.max_bandwidth = OPUS_BANDWIDTH_FULLBAND
	st.force_channels = OPUS_AUTO
	st.user_forced_mode = MODE_AUTO
	st.voice_ratio = -1
	st.encoder_buffer = Fs / 100
	st.lsb_depth = 24
	st.variable_duration = OPUS_FRAMESIZE_ARG
	st.delay_compensation = Fs / 250
	st.hybrid_stereo_width_Q14 = 1 << 14
	st.prev_HB_gain = Q15ONE
	st.variable_HP_smth2_Q15 = silk_LSHIFT(silk_lin2log(VARIABLE_HP_MIN_CUTOFF_HZ), 8)
	st.first = 1
	st.mode = MODE_HYBRID
	st.bandwidth = OPUS_BANDWIDTH_FULLBAND
	tonality_analysis_init(&st.analysis)
	return OPUS_OK
}

func (st *OpusEncoder) user_bitrate_to_bitrate(frame_size, max_data_bytes int) int {
	if frame_size == 0 {
		frame_size = st.Fs / 400
	}
	if st.user_bitrate_bps == OPUS_AUTO {
		return 60*st.Fs/frame_size + st.Fs*st.channels
	} else if st.user_bitrate_bps == OPUS_BITRATE_MAX {
		return max_data_bytes * 8 * st.Fs / frame_size
	} else {
		return st.user_bitrate_bps
	}
}

func (st *OpusEncoder) opus_encode_native(pcm []int16, pcm_ptr, frame_size int, data []byte, data_ptr, out_data_bytes, lsb_depth int, analysis_pcm []int16, analysis_pcm_ptr, analysis_size, c1, c2, analysis_channels, float_api int) int {
	silk_enc := &st.SilkEncoder
	celt_enc := &st.Celt_Encoder
	enc := NewEntropyCoder()
	max_data_bytes := imin(1276, out_data_bytes)
	st.rangeFinal = 0
	if (st.variable_duration == OPUS_FRAMESIZE_UNKNOWN && 400*frame_size != st.Fs && 200*frame_size != st.Fs && 100*frame_size != st.Fs && 50*frame_size != st.Fs && 25*frame_size != st.Fs && 50*frame_size != 3*st.Fs) || (400*frame_size < st.Fs) || max_data_bytes <= 0 {
		return OPUS_BAD_ARG
	}
	delay_compensation := 0
	if st.application != OPUS_APPLICATION_RESTRICTED_LOWDELAY {
		delay_compensation = st.delay_compensation
	}
	lsb_depth = imin(lsb_depth, st.lsb_depth)
	celt_mode := celt_enc.GetMode()
	st.voice_ratio = -1
	if st.analysis.enabled {
		analysis_info := AnalysisInfo{valid: 0}
		if st.silk_mode.complexity >= 7 && st.Fs == 48000 {
			analysis_read_pos_bak := st.analysis.read_pos
			analysis_read_subframe_bak := st.analysis.read_subframe
			run_analysis(&st.analysis, celt_mode, analysis_pcm, analysis_pcm_ptr, analysis_size, frame_size, c1, c2, analysis_channels, st.Fs, lsb_depth, &analysis_info)
			st.analysis.read_pos = analysis_read_pos_bak
			st.analysis.read_subframe = analysis_read_subframe_bak
		}
		st.detected_bandwidth = OPUS_BANDWIDTH_UNKNOWN
		if analysis_info.valid != 0 {
			analysis_bandwidth := analysis_info.bandwidth
			if analysis_bandwidth <= 12 {
				st.detected_bandwidth = OPUS_BANDWIDTH_NARROWBAND
			} else if analysis_bandwidth <= 14 {
				st.detected_bandwidth = OPUS_BANDWIDTH_MEDIUMBAND
			} else if analysis_bandwidth <= 16 {
				st.detected_bandwidth = OPUS_BANDWIDTH_WIDEBAND
			} else if analysis_bandwidth <= 18 {
				st.detected_bandwidth = OPUS_BANDWIDTH_SUPERWIDEBAND
			} else {
				st.detected_bandwidth = OPUS_BANDWIDTH_FULLBAND
			}
			if st.signal_type == OPUS_SIGNAL_AUTO {
				st.voice_ratio = int(math.Floor(0.5 + 100*(1-analysis_info.music_prob))
			}
		}
	}
	stereo_width := 0
	if st.channels == 2 && st.force_channels != 1 {
		stereo_width = compute_stereo_width(pcm, pcm_ptr, frame_size, st.Fs, &st.width_mem)
	}
	total_buffer := delay_compensation
	st.bitrate_bps = st.user_bitrate_to_bitrate(frame_size, max_data_bytes)
	frame_rate := st.Fs / frame_size
	if st.use_vbr == 0 {
		frame_rate3 := 3 * st.Fs / frame_size
		cbrBytes := imin((3*st.bitrate_bps/8+frame_rate3/2)/frame_rate3, max_data_bytes)
		st.bitrate_bps = cbrBytes * frame_rate3 * 8 / 3
		max_data_bytes = cbrBytes
	}
	if max_data_bytes < 3 || st.bitrate_bps < 3*frame_rate*8 || (frame_rate < 50 && (max_data_bytes*frame_rate < 300 || st.bitrate_bps < 2400)) {
		tocmode := st.mode
		bw := OPUS_BANDWIDTH_NARROWBAND
		if st.bandwidth != OPUS_BANDWIDTH_UNKNOWN {
			bw = st.bandwidth
		}
		if tocmode == MODE_UNKNOWN {
			tocmode = MODE_SILK_ONLY
		}
		if frame_rate > 100 {
			tocmode = MODE_CELT_ONLY
		}
		if frame_rate < 50 {
			tocmode = MODE_SILK_ONLY
		}
		if tocmode == MODE_SILK_ONLY && GetOrdinal(bw) > GetOrdinal(OPUS_BANDWIDTH_WIDEBAND) {
			bw = OPUS_BANDWIDTH_WIDEBAND
		} else if tocmode == MODE_CELT_ONLY && GetOrdinal(bw) == GetOrdinal(OPUS_BANDWIDTH_MEDIUMBAND) {
			bw = OPUS_BANDWIDTH_NARROWBAND
		} else if tocmode == MODE_HYBRID && GetOrdinal(bw) <= GetOrdinal(OPUS_BANDWIDTH_SUPERWIDEBAND) {
			bw = OPUS_BANDWIDTH_SUPERWIDEBAND
		}
		data[data_ptr] = gen_toc(tocmode, frame_rate, bw, st.stream_channels)
		ret := 1
		if st.use_vbr == 0 {
			ret = padPacket(data, data_ptr, ret, max_data_bytes)
			if ret == OPUS_OK {
				ret = max_data_bytes
			}
		}
		return ret
	}
	max_rate := frame_rate * max_data_bytes * 8
	equiv_rate := st.bitrate_bps - (40*st.channels+20)*(st.Fs/frame_size-50)
	voice_est := 0
	if st.signal_type == OPUS_SIGNAL_VOICE {
		voice_est = 127
	} else if st.signal_type == OPUS_SIGNAL_MUSIC {
		voice_est = 0
	} else if st.voice_ratio >= 0 {
		voice_est = st.voice_ratio * 327 >> 8
		if st.application == OPUS_APPLICATION_AUDIO {
			voice_est = imin(voice_est, 115)
		}
	} else if st.application == OPUS_APPLICATION_VOIP {
		voice_est = 115
	} else {
		voice_est = 48
	}
	if st.force_channels != OPUS_AUTO && st.channels == 2 {
		st.stream_channels = st.force_channels
	} else if st.channels == 2 {
		stereo_threshold := STEREO_MUSIC_THRESHOLD + ((voice_est*voice_est*(STEREO_VOICE_THRESHOLD-STEREO_MUSIC_THRESHOLD))>>14)
		if st.stream_channels == 2 {
			stereo_threshold -= 1000
		} else {
			stereo_threshold += 1000
		}
		if equiv_rate > stereo_threshold {
			st.stream_channels = 2
		} else {
			st.stream_channels = 1
		}
	} else {
		st.stream_channels = st.channels
	}
	equiv_rate = st.bitrate_bps - (40*st.stream_channels+20)*(st.Fs/frame_size-50)
	if st.application == OPUS_APPLICATION_RESTRICTED_LOWDELAY {
		st.mode = MODE_CELT_ONLY
	} else if st.user_forced_mode == MODE_AUTO {
		mode_voice := MULT16_32_Q15(Q15ONE-stereo_width, MODE_THRESHOLDS[0][0]) + MULT16_32_Q15(stereo_width, MODE_THRESHOLDS[1][0])
		mode_music := MULT16_32_Q15(Q15ONE-stereo_width, MODE_THRESHOLDS[1][1]) + MULT16_32_Q15(stereo_width, MODE_THRESHOLDS[1][1])
		threshold := mode_music + ((voice_est * voice_est * (mode_voice - mode_music)) >> 14)
		if st.application == OPUS_APPLICATION_VOIP {
			threshold += 8000
		}
		if st.prev_mode == MODE_CELT_ONLY {
			threshold -= 4000
		} else if st.prev_mode != MODE_AUTO && st.prev_mode != MODE_UNKNOWN {
			threshold += 4000
		}
		if equiv_rate >= threshold {
			st.mode = MODE_CELT_ONLY
		} else {
			st.mode = MODE_SILK_ONLY
		}
		if st.silk_mode.useInBandFEC != 0 && st.silk_mode.packetLossPercentage > (128-voice_est)>>4 {
			st.mode = MODE_SILK_ONLY
		}
		if st.silk_mode.useDTX != 0 && voice_est > 100 {
			st.mode = MODE_SILK_ONLY
		}
	} else {
		st.mode = st.user_forced_mode
	}
	if st.mode != MODE_CELT_ONLY && frame_size < st.Fs/100 {
		st.mode = MODE_CELT_ONLY
	}
	if st.lfe != 0 {
		st.mode = MODE_CELT_ONLY
	}
	if max_data_bytes < (frame_rate*12000)/st.Fs {
		st.mode = MODE_CELT_ONLY
	}
	if st.stream_channels == 1 && st.prev_channels == 2 && st.silk_mode.toMono == 0 && st.mode != MODE_CELT_ONLY && st.prev_mode != MODE_CELT_ONLY {
		st.silk_mode.toMono = 1
		st.stream_channels = 2
	} else {
		st.silk_mode.toMono = 0
	}
	redundancy := 0
	celt_to_silk := 0
	to_celt := 0
	if (st.prev_mode != MODE_AUTO && st.prev_mode != MODE_UNKNOWN) && ((st.mode != MODE_CELT_ONLY && st.prev_mode == MODE_CELT_ONLY) || (st.mode == MODE_CELT_ONLY && st.prev_mode != MODE_CELT_ONLY)) {
		redundancy = 1
		celt_to_silk = 0
		if st.mode != MODE_CELT_ONLY {
			celt_to_silk = 1
		}
		if celt_to_silk == 0 {
			if frame_size >= st.Fs/100 {
				st.mode = st.prev_mode
				to_celt = 1
			} else {
				redundancy = 0
			}
		}
	}
	if st.silk_bw_switch != 0 {
		redundancy = 1
		celt_to_silk = 1
		st.silk_bw_switch = 0
	}
	redundancy_bytes := 0
	if redundancy != 0 {
		redundancy_bytes = imin(257, max_data_bytes*(st.Fs/200)/(frame_size+st.Fs/200))
		if st.use_vbr != 0 {
			redundancy_bytes = imin(redundancy_bytes, st.bitrate_bps/1600)
		}
	}
	if st.mode != MODE_CELT_ONLY && st.prev_mode == MODE_CELT_ONLY {
		dummy := EncControlState{}
		silk_InitEncoder(silk_enc, &dummy)
	}
	curr_bandwidth := st.bandwidth
	if st.mode == MODE_CELT_ONLY || st.first != 0 || st.silk_mode.allowBandwidthSwitch != 0 {
		bandwidth := OPUS_BANDWIDTH_FULLBAND
		equiv_rate2 := equiv_rate
		if st.mode != MODE_CELT_ONLY {
			equiv_rate2 = equiv_rate2 * (45 + st.silk_mode.complexity) / 50
			if st.use_vbr == 0 {
				equiv_rate2 -= 1000
			}
		}
		var voice_bandwidth_thresholds, music_bandwidth_thresholds []int
		if st.channels == 2 && st.force_channels != 1 {
			voice_bandwidth_thresholds = STEREO_VOICE_BANDWIDTH_THRESHOLDS
			music_bandwidth_thresholds = STEREO_MUSIC_BANDWIDTH_THRESHOLDS
		} else {
			voice_bandwidth_thresholds = MONO_VOICE_BANDWIDTH_THRESHOLDS
			music_bandwidth_thresholds = MONO_MUSIC_BANDWIDTH_THRESHOLDS
		}
		bandwidth_thresholds := make([]int, 8)
		for i := 0; i < 8; i++ {
			bandwidth_thresholds[i] = music_bandwidth_thresholds[i] + ((voice_est * voice_est * (voice_bandwidth_thresholds[i] - music_bandwidth_thresholds[i])) >> 14)
		}
		for GetOrdinal(bandwidth) > GetOrdinal(OPUS_BANDWIDTH_NARROWBAND) {
			idx := 2 * (GetOrdinal(bandwidth) - GetOrdinal(OPUS_BANDWIDTH_MEDIUMBAND))
			threshold := bandwidth_thresholds[idx]
			hysteresis := bandwidth_thresholds[idx+1]
			if st.first == 0 {
				if GetOrdinal(st.bandwidth) >= GetOrdinal(bandwidth) {
					threshold -= hysteresis
				} else {
					threshold += hysteresis
				}
			}
			if equiv_rate2 >= threshold {
				break
			}
			bandwidth = Subtract(bandwidth, 1)
		}
		st.bandwidth = bandwidth
		if st.first == 0 && st.mode != MODE_CELT_ONLY && st.silk_mode.inWBmodeWithoutVariableLP == 0 && GetOrdinal(st.bandwidth) > GetOrdinal(OPUS_BANDWIDTH_WIDEBAND) {
			st.bandwidth = OPUS_BANDWIDTH_WIDEBAND
		}
	}
	if GetOrdinal(st.bandwidth) > GetOrdinal(st.max_bandwidth) {
		st.bandwidth = st.max_bandwidth
	}
	if st.user_bandwidth != OPUS_BANDWIDTH_AUTO {
		st.bandwidth = st.user_bandwidth
	}
	if st.mode != MODE_CELT_ONLY && max_rate < 15000 {
		st.bandwidth = MinBandwidth(st.bandwidth, OPUS_BANDWIDTH_WIDEBAND)
	}
	if st.Fs <= 24000 && GetOrdinal(st.bandwidth) > GetOrdinal(OPUS_BANDWIDTH_SUPERWIDEBAND) {
		st.bandwidth = OPUS_BANDWIDTH_SUPERWIDEBAND
	}
	if st.Fs <= 16000 && GetOrdinal(st.bandwidth) > GetOrdinal(OPUS_BANDWIDTH_WIDEBAND) {
		st.bandwidth = OPUS_BANDWIDTH_WIDEBAND
	}
	if st.Fs <= 12000 && GetOrdinal(st.bandwidth) > GetOrdinal(OPUS_BANDWIDTH_MEDIUMBAND) {
		st.bandwidth = OPUS_BANDWIDTH_MEDIUMBAND
	}
	if st.Fs <= 8000 && GetOrdinal(st.bandwidth) > GetOrdinal(OPUS_BANDWIDTH_NARROWBAND) {
		st.bandwidth = OPUS_BANDWIDTH_NARROWBAND
	}
	if st.detected_bandwidth != OPUS_BANDWIDTH_UNKNOWN && st.user_bandwidth == OPUS_BANDWIDTH_AUTO {
		var min_detected_bandwidth OpusBandwidth
		if equiv_rate <= 18000*st.stream_channels && st.mode == MODE_CELT_ONLY {
			min_detected_bandwidth = OPUS_BANDWIDTH_NARROWBAND
		} else if equiv_rate <= 24000*st.stream_channels && st.mode == MODE_CELT_ONLY {
			min_detected_bandwidth = OPUS_BANDWIDTH_MEDIUMBAND
		} else if equiv_rate <= 30000*st.stream_channels {
			min_detected_bandwidth = OPUS_BANDWIDTH_WIDEBAND
		} else if equiv_rate <= 44000*st.stream_channels {
			min_detected_bandwidth = OPUS_BANDWIDTH_SUPERWIDEBAND
		} else {
			min_detected_bandwidth = OPUS_BANDWIDTH_FULLBAND
		}
		if GetOrdinal(st.detected_bandwidth) < GetOrdinal(min_detected_bandwidth) {
			st.detected_bandwidth = min_detected_bandwidth
		}
		if GetOrdinal(st.bandwidth) > GetOrdinal(st.detected_bandwidth) {
			st.bandwidth = st.detected_bandwidth
		}
	}
	celt_enc.SetLSBDepth(lsb_depth)
	if st.mode == MODE_CELT_ONLY && st.bandwidth == OPUS_BANDWIDTH_MEDIUMBAND {
		st.bandwidth = OPUS_BANDWIDTH_WIDEBAND
	}
	if st.lfe != 0 {
		st.bandwidth = OPUS_BANDWIDTH_NARROWBAND
	}
	if frame_size > st.Fs/50 && (st.mode == MODE_CELT_ONLY || GetOrdinal(st.bandwidth) > GetOrdinal(OPUS_BANDWIDTH_WIDEBAND)) {
		nb_frames := 3
		if frame_size <= st.Fs/25 {
			nb_frames = 2
		}
		bytes_per_frame := imin(1276, (out_data_bytes-3)/nb_frames)
		tmp_data := make([]byte, nb_frames*bytes_per_frame)
		rp := NewOpusRepacketizer()
		bak_mode := st.user_forced_mode
		bak_bandwidth := st.user_bandwidth
		bak_channels := st.force_channels
		bak_to_mono := st.silk_mode.toMono
		st.user_forced_mode = st.mode
		st.user_bandwidth = st.bandwidth
		st.force_channels = st.stream_channels
		if bak_to_mono != 0 {
			st.force_channels = 1
		} else {
			st.prev_channels = st.stream_channels
		}
		for i := 0; i < nb_frames; i++ {
			st.silk_mode.toMono = 0
			if to_celt != 0 && i == nb_frames-1 {
				st.user_forced_mode = MODE_CELT_ONLY
			}
			tmp_len := st.opus_encode_native(pcm, pcm_ptr+i*(st.channels*st.Fs/50), st.Fs/50, tmp_data, i*bytes_per_frame, bytes_per_frame, lsb_depth, nil, 0, 0, c1, c2, analysis_channels, float_api)
			if tmp_len < 0 {
				return OPUS_INTERNAL_ERROR
			}
			if rp.AddPacket(tmp_data, i*bytes_per_frame, tmp_len) < 0 {
				return OPUS_INTERNAL_ERROR
			}
		}
		repacketize_len := out_data_bytes
		if st.use_vbr == 0 {
			repacketize_len = imin(3*st.bitrate_bps/(3*8*50/nb_frames), out_data_bytes)
		}
		ret := rp.OutRange(0, nb_frames, data, data_ptr, repacketize_len, false, st.use_vbr == 0)
		if ret < 0 {
			return OPUS_INTERNAL_ERROR
		}
		st.user_forced_mode = bak_mode
		st.user_bandwidth = bak_bandwidth
		st.force_channels = bak_channels
		st.silk_mode.toMono = bak_to_mono
		return ret
	}
	bytes_target := imin(max_data_bytes-redundancy_bytes, st.bitrate_bps*frame_size/(st.Fs*8)) - 1
	data_ptr++
	enc.Init(data, data_ptr, max_data_bytes-1)
	pcm_buf := make([]int16, (total_buffer+frame_size)*st.channels)
	copy(pcm_buf[:total_buffer*st.channels], st.delay_buffer[(st.encoder_buffer-total_buffer)*st.channels:(st.encoder_buffer-total_buffer)*st.channels+total_buffer*st.channels])
	hp_freq_smth1 := 0
	if st.mode == MODE_CELT_ONLY {
		hp_freq_smth1 = silk_LSHIFT(silk_lin2log(VARIABLE_HP_MIN_CUTOFF_HZ), 8)
	} else {
		hp_freq_smth1 = silk_enc.State_Fxx[0].Variable_HP_smth1_Q15
	}
	st.variable_HP_smth2_Q15 = silk_SMLAWB(st.variable_HP_smth2_Q15, hp_freq_smth1-st.variable_HP_smth2_Q15, int(VARIABLE_HP_SMTH_COEF2*(1<<16)+0.5)
	cutoff_Hz := silk_log2lin(st.variable_HP_smth2_Q15 >> 8)
	if st.application == OPUS_APPLICATION_VOIP {
		hp_cutoff(pcm, pcm_ptr, cutoff_Hz, pcm_buf, total_buffer*st.channels, st.hp_mem[:], frame_size, st.channels, st.Fs)
	} else {
		dc_reject(pcm, pcm_ptr, 3, pcm_buf, total_buffer*st.channels, st.hp_mem[:], frame_size, st.channels, st.Fs)
	}
	HB_gain := Q15ONE
	if st.mode != MODE_CELT_ONLY {
		pcm_silk := make([]int16, st.channels*frame_size)
		total_bitRate := 8 * bytes_target * frame_rate
		if st.mode == MODE_HYBRID {
			st.silk_mode.bitRate = st.stream_channels * (5000)
			if st.Fs == 100*frame_size {
				st.silk_mode.bitRate += 1000
			}
			if st.bandwidth == OPUS_BANDWIDTH_SUPERWIDEBAND {
				st.silk_mode.bitRate += (total_bitRate - st.silk_mode.bitRate) * 2 / 3
			} else {
				st.silk_mode.bitRate += (total_bitRate - st.silk_mode.bitRate) * 3 / 5
			}
			if st.silk_mode.bitRate > total_bitRate*4/5 {
				st.silk_mode.bitRate = total_bitRate * 4 / 5
			}
			if st.energy_masking == nil {
				celt_rate := total_bitRate - st.silk_mode.bitRate
				HB_gain_ref := 3000
				if st.bandwidth == OPUS_BANDWIDTH_FULLBAND {
					HB_gain_ref = 3600
				}
				HB_gain = SHL32(celt_rate, 9) / SHR32(celt_rate+st.stream_channels*HB_gain_ref, 6)
				if HB_gain < Q15ONE*6/7 {
					HB_gain += Q15ONE / 7
				} else {
					HB_gain = Q15ONE
				}
			}
		} else {
			st.silk_mode.bitRate = total_bitRate
		}
		st.silk_mode.payloadSize_ms = 1000 * frame_size / st.Fs
		st.silk_mode.nChannelsAPI = st.channels
		st.silk_mode.nChannelsInternal = st.stream_channels
		if st.bandwidth == OPUS_BANDWIDTH_NARROWBAND {
			st.silk_mode.desiredInternalSampleRate = 8000
		} else if st.bandwidth == OPUS_BANDWIDTH_MEDIUMBAND {
			st.silk_mode.desiredInternalSampleRate = 12000
		} else {
			st.silk_mode.desiredInternalSampleRate = 16000
		}
		if st.mode == MODE_HYBRID {
			st.silk_mode.minInternalSampleRate = 16000
		} else {
			st.silk_mode.minInternalSampleRate = 8000
		}
		if st.mode == MODE_SILK_ONLY {
			effective_max_rate := max_rate
			st.silk_mode.maxInternalSampleRate = 16000
			if frame_rate > 50 {
				effective_max_rate = effective_max_rate * 2 / 3
			}
			if effective_max_rate < 13000 {
				st.silk_mode.maxInternalSampleRate = 12000
				if st.silk_mode.desiredInternalSampleRate > 12000 {
					st.silk_mode.desiredInternalSampleRate = 12000
				}
			}
			if effective_max_rate < 9600 {
				st.silk_mode.maxInternalSampleRate = 8000
				if st.silk_mode.desiredInternalSampleRate > 8000 {
					st.silk_mode.desiredInternalSampleRate = 8000
				}
			}
		} else {
			st.silk_mode.maxInternalSampleRate = 16000
		}
		st.silk_mode.useCBR = 0
		if st.use_vbr == 0 {
			st.silk_mode.useCBR = 1
		}
		nBytes := imin(1275, max_data_bytes-1-redundancy_bytes)
		st.silk_mode.maxBits = nBytes * 8
		if st.mode == MODE_HYBRID {
			st.silk_mode.maxBits = st.silk_mode.maxBits * 9 / 10
		}
		if st.silk_mode.useCBR != 0 {
			st.silk_mode.maxBits = (st.silk_mode.bitRate * frame_size / (st.Fs * 8)) * 8
			st.silk_mode.bitRate = imax(1, st.silk_mode.bitRate-2000)
		}
		copy(pcm_silk, pcm_buf[total_buffer*st.channels:(total_buffer+frame_size)*st.channels])
		silkBytes := nBytes
		ret := silk_Encode(silk_enc, &st.silk_mode, pcm_silk, frame_size, enc, &silkBytes, 0)
		nBytes = silkBytes
		if ret != 0 {
			return OPUS_INTERNAL_ERROR
		}
		if nBytes == 0 {
			st.rangeFinal = 0
			data[data_ptr-1] = gen_toc(st.mode, st.Fs/frame_size, curr_bandwidth, st.stream_channels)
			return 1
		}
		if st.mode == MODE_SILK_ONLY {
			if st.silk_mode.internalSampleRate == 8000 {
				curr_bandwidth = OPUS_BANDWIDTH_NARROWBAND
			} else if st.silk_mode.internalSampleRate == 12000 {
				curr_bandwidth = OPUS_BANDWIDTH_MEDIUMBAND
			} else if st.silk_mode.internalSampleRate == 16000 {
				curr_bandwidth = OPUS_BANDWIDTH_WIDEBAND
			}
		}
		st.silk_mode.opusCanSwitch = st.silk_mode.switchReady
		if st.silk_mode.opusCanSwitch != 0 {
			redundancy = 1
			celt_to_silk = 0
			st.silk_bw_switch = 1
		}
	}
	endband := 21
	switch curr_bandwidth {
	case OPUS_BANDWIDTH_NARROWBAND:
		endband = 13
	case OPUS_BANDWIDTH_MEDIUMBAND, OPUS_BANDWIDTH_WIDEBAND:
		endband = 17
	case OPUS_BANDWIDTH_SUPERWIDEBAND:
		endband = 19
	case OPUS_BANDWIDTH_FULLBAND:
		endband = 21
	}
	celt_enc.SetEndBand(endband)
	celt_enc.SetChannels(st.stream_channels)
	start_band := 0
	if st.mode == MODE_SILK_ONLY {
		start_band = 17
	}
	nb_compr_bytes := 0
	if st.mode != MODE_SILK_ONLY {
		if st.mode == MODE_HYBRID {
			len := (enc.Tell() + 7) >> 3
			if redundancy != 0 {
				if st.mode == MODE_HYBRID {
					len += 3
				} else {
					len += 1
				}
			}
			if st.use_vbr != 0 {
				nb_compr_bytes = len + bytes_target - (st.silk_mode.bitRate * frame_size) / (8 * st.Fs)
			} else {
				if len > bytes_target {
					nb_compr_bytes = len
				} else {
					nb_compr_bytes = bytes_target
				}
			}
		} else if st.use_vbr != 0 {
			bonus := 0
			if st.analysis.enabled && st.variable_duration == OPUS_FRAMESIZE_VARIABLE && frame_size != st.Fs/50 {
				bonus = (60*st.stream_channels + 40) * (st.Fs/frame_size - 50)
			}
			celt_enc.SetVBR(true)
			celt_enc.SetVBRConstraint(st.vbr_constraint != 0)
			celt_enc.SetBitrate(st.bitrate_bps + bonus)
			nb_compr_bytes = max_data_bytes - 1 - redundancy_bytes
		} else {
			nb_compr_bytes = bytes_target
		}
	}
	tmp_prefill := make([]int16, st.channels*st.Fs/400)
	if st.mode != MODE_SILK_ONLY && st.mode != st.prev_mode && (st.prev_mode != MODE_AUTO && st.prev_mode != MODE_UNKNOWN) {
		copy(tmp_prefill, st.delay_buffer[(st.encoder_buffer-total_buffer-st.Fs/400)*st.channels:(st.encoder_buffer-total_buffer-st.Fs/400)*st.channels+st.channels*st.Fs/400])
	}
	if st.channels*(st.encoder_buffer-(frame_size+total_buffer)) > 0 {
		copy(st.delay_buffer[:st.channels*(st.encoder_buffer-frame_size-total_buffer)], st.delay_buffer[st.channels*frame_size:st.channels*frame_size+st.channels*(st.encoder_buffer-frame_size-total_buffer)])
		copy(st.delay_buffer[st.channels*(st.encoder_buffer-frame_size-total_buffer):], pcm_buf[:frame_size+total_buffer])
	} else {
		copy(st.delay_buffer[:], pcm_buf[(frame_size+total_buffer-st.encoder_buffer)*st.channels:(frame_size+total_buffer-st.encoder_buffer)*st.channels+st.encoder_buffer*st.channels])
	}
	if st.prev_HB_gain < Q15ONE || HB_gain < Q15ONE {
		gain_fade(pcm_buf, 0, st.prev_HB_gain, HB_gain, celt_mode.overlap, frame_size, st.channels, celt_mode.window, st.Fs)
	}
	st.prev_HB_gain = HB_gain
	if st.mode != MODE_HYBRID || st.stream_channels == 1 {
		st.silk_mode.stereoWidth_Q14 = imin(1<<14, 2*imax(0, equiv_rate-30000))
	}
	if st.energy_masking == nil && st.channels == 2 {
		g1 := st.hybrid_stereo_width_Q14
		g2 := st.silk_mode.stereoWidth_Q14
		if g1 == 16384 {
			g1 = Q15ONE
		} else {
			g1 = SHL16(g1, 1)
		}
		if g2 == 16384 {
			g2 = Q15ONE
		} else {
			g2 = SHL16(g2, 1)
		}
		stereo_fade(pcm_buf, g1, g2, celt_mode.overlap, frame_size, st.channels, celt_mode.window, st.Fs)
		st.hybrid_stereo_width_Q14 = int16(st.silk_mode.stereoWidth_Q14)
	}
	if st.mode != MODE_CELT_ONLY && enc.Tell()+17+20*btol(st.mode == MODE_HYBRID) <= 8*(max_data_bytes-1) {
		if st.mode == MODE_HYBRID && (redundancy != 0 || enc.Tell()+37 <= 8*nb_compr_bytes) {
			enc.EncBitLogp(redundancy, 12)
		}
		if redundancy != 0 {
			max_redundancy := max_data_bytes - 1 - nb_compr_bytes
			if st.mode == MODE_HYBRID {
				redundancy_bytes = imin(max_redundancy, st.bitrate_bps/1600)
				redundancy_bytes = imin(257, imax(2, redundancy_bytes))
				enc.EncUint(redundancy_bytes-2, 256)
			} else {
				redundancy_bytes = imin(max_redundancy, 257)
				redundancy_bytes = imax(2, redundancy_bytes)
			}
		}
	} else {
		redundancy = 0
		redundancy_bytes = 0
		st.silk_bw_switch = 0
	}
	if st.mode == MODE_SILK_ONLY {
		ret := (enc.Tell() + 7) >> 3
		enc.EncDone()
		nb_compr_bytes = ret
	} else {
		nb_compr_bytes = imin(max_data_bytes-1-redundancy_bytes, nb_compr_bytes)
		enc.Shrink(nb_compr_bytes)
	}
	redundant_rng := 0
	if redundancy != 0 && celt_to_silk != 0 {
		celt_enc.SetStartBand(0)
		celt_enc.SetVBR(false)
		err := celt_enc.celt_encode_with_ec(pcm_buf, 0, st.Fs/200, data, data_ptr+nb_compr_bytes, redundancy_bytes, nil)
		if err < 0 {
			return OPUS_INTERNAL_ERROR
		}
		redundant_rng = celt_enc.GetFinalRange()
		celt_enc.ResetState()
	}
	celt_enc.SetStartBand(start_band)
	if st.mode != MODE_SILK_ONLY {
		if st.mode != st.prev_mode && (st.prev_mode != MODE_AUTO && st.prev_mode != MODE_UNKNOWN) {
			dummy := make([]byte, 2)
			celt_enc.ResetState()
			celt_enc.celt_encode_with_ec(tmp_prefill, 0, st.Fs/400, dummy, 0, 2, nil)
			celt_enc.SetPrediction(0)
		}
		if enc.Tell() <= 8*nb_compr_bytes {
			ret := celt_enc.celt_encode_with_ec(pcm_buf, 0, frame_size, nil, 0, nb_compr_bytes, enc)
			if ret < 0 {
				return OPUS_INTERNAL_ERROR
			}
		}
	}
	if redundancy != 0 && celt_to_silk == 0 {
		N2 := st.Fs / 200
		N4 := st.Fs / 400
		celt_enc.ResetState()
		celt_enc.SetStartBand(0)
		celt_enc.SetPrediction(0)
		dummy := make([]byte, 2)
		celt_enc.celt_encode_with_ec(pcm_buf[st.channels*(frame_size-N2-N4):], 0, N4, dummy, 0, 2, nil)
		err := celt_enc.celt_encode_with_ec(pcm_buf[st.channels*(frame_size-N2):], 0, N2, data, data_ptr+nb_compr_bytes, redundancy_bytes, nil)
		if err < 0 {
			return OPUS_INTERNAL_ERROR
		}
		redundant_rng = celt_enc.GetFinalRange()
	}
	data_ptr--
	data[data_ptr] = gen_toc(st.mode, st.Fs/frame_size, curr_bandwidth, st.stream_channels)
	st.rangeFinal = int(enc.Rng) ^ redundant_rng
	if to_celt != 0 {
		st.prev_mode = MODE_CELT_ONLY
	} else {
		st.prev_mode = st.mode
	}
	st.prev_channels = st.stream_channels
	st.prev_framesize = frame_size
	st.first = 0
	ret := (enc.Tell() + 7) >> 3
	if enc.Tell() > (max_data_bytes-1)*8 {
		if max_data_bytes < 2 {
			return OPUS_BUFFER_TOO_SMALL
		}
		data[data_ptr+1] = 0
		ret = 1
		st.rangeFinal = 0
	} else if st.mode == MODE_SILK_ONLY && redundancy == 0 {
		for ret > 2 && data[data_ptr+ret] == 0 {
			ret--
		}
	}
	ret += 1 + redundancy_bytes
	if st.use_vbr == 0 {
		if padPacket(data, data_ptr, ret, max_data_bytes) != OPUS_OK {
			return OPUS_INTERNAL_ERROR
		}
		ret = max_data_bytes
	}
	return ret
}

func (st *OpusEncoder) Encode(in_pcm []int16, pcm_offset, frame_size int, out_data []byte, out_data_offset, max_data_bytes int) (int, error) {
	if out_data_offset+max_data_bytes > len(out_data) {
		return 0, errors.New("Output buffer is too small")
	}
	delay_compensation := st.delay_compensation
	if st.application == OPUS_APPLICATION_RESTRICTED_LOWDELAY {
		delay_compensation = 0
	}
	internal_frame_size := compute_frame_size(in_pcm, pcm_offset, frame_size, st.variable_duration, st.channels, st.Fs, st.bitrate_bps, delay_compensation, st.analysis.subframe_mem, st.analysis.enabled)
	if pcm_offset+internal_frame_size > len(in_pcm) {
		return 0, errors.New("Not enough samples provided in input signal")
	}
	ret := st.opus_encode_native(in_pcm, pcm_offset, internal_frame_size, out_data, out_data_offset, max_data_bytes, 16, in_pcm, pcm_offset, frame_size, 0, -2, st.channels, 0)
	if ret < 0 {
		if ret == OPUS_BAD_ARG {
			return 0, errors.New("OPUS_BAD_ARG while encoding")
		}
		return 0, errors.New("An error occurred during encoding")
	}
	return ret, nil
}

func (st *OpusEncoder) GetApplication() OpusApplication {
	return st.application
}

func (st *OpusEncoder) SetApplication(value OpusApplication) {
	if st.first == 0 && st.application != value {
		panic("Application cannot be changed after encoding has started")
	}
	st.application = value
}

func (st *OpusEncoder) GetBitrate() int {
	return st.user_bitrate_to_bitrate(st.prev_framesize, 1276)
}

func (st *OpusEncoder) SetBitrate(value int) {
	if value != OPUS_AUTO && value != OPUS_BITRATE_MAX {
		if value <= 0 {
			panic("Bitrate must be positive")
		} else if value <= 500 {
			value = 500
		} else if value > 300000*st.channels {
			value = 300000 * st.channels
		}
	}
	st.user_bitrate_bps = value
}

func (st *OpusEncoder) GetForceChannels() int {
	return st.force_channels
}

func (st *OpusEncoder) SetForceChannels(value int) {
	if (value < 1 || value > st.channels) && value != OPUS_AUTO {
		panic("Force channels must be <= num. of channels")
	}
	st.force_channels = value
}

func (st *OpusEncoder) GetMaxBandwidth() OpusBandwidth {
	return st.max_bandwidth
}

func (st *OpusEncoder) SetMaxBandwidth(value OpusBandwidth) {
	st.max_bandwidth = value
	if value == OPUS_BANDWIDTH_NARROWBAND {
		st.silk_mode.maxInternalSampleRate = 8000
	} else if value == OPUS_BANDWIDTH_MEDIUMBAND {
		st.silk_mode.maxInternalSampleRate = 12000
	} else {
		st.silk_mode.maxInternalSampleRate = 16000
	}
}

func (st *OpusEncoder) GetBandwidth() OpusBandwidth {
	return st.bandwidth
}

func (st *OpusEncoder) SetBandwidth(value OpusBandwidth) {
	st.user_bandwidth = value
	if value == OPUS_BANDWIDTH_NARROWBAND {
		st.silk_mode.maxInternalSampleRate = 8000
	} else if value == OPUS_BANDWIDTH_MEDIUMBAND {
		st.silk_mode.maxInternalSampleRate = 12000
	} else {
		st.silk_mode.maxInternalSampleRate = 16000
	}
}

func (st *OpusEncoder) GetUseDTX() bool {
	return st.silk_mode.useDTX != 0
}

func (st *OpusEncoder) SetUseDTX(value bool) {
	if value {
		st.silk_mode.useDTX = 1
	} else {
		st.silk_mode.useDTX = 0
	}
}

func (st *OpusEncoder) GetComplexity() int {
	return st.silk_mode.complexity
}

func (st *OpusEncoder) SetComplexity(value int) {
	if value < 0 || value > 10 {
		panic("Complexity must be between 0 and 10")
	}
	st.silk_mode.complexity = value
	st.Celt_Encoder.SetComplexity(value)
}

func (st *OpusEncoder) GetUseInbandFEC() bool {
	return st.silk_mode.useInBandFEC != 0
}

func (st *OpusEncoder) SetUseInbandFEC(value bool) {
	if value {
		st.silk_mode.useInBandFEC = 1
	} else {
		st.silk_mode.useInBandFEC = 0
	}
}

func (st *OpusEncoder) GetPacketLossPercent() int {
	return st.silk_mode.packetLossPercentage
}

func (st *OpusEncoder) SetPacketLossPercent(value int) {
	if value < 0 || value > 100 {
		panic("Packet loss must be between 0 and 100")
	}
	st.silk_mode.packetLossPercentage = value
	st.Celt_Encoder.SetPacketLossPercent(value)
}

func (st *OpusEncoder) GetUseVBR() bool {
	return st.use_vbr != 0
}

func (st *OpusEncoder) SetUseVBR(value bool) {
	if value {
		st.use_vbr = 1
		st.silk_mode.useCBR = 0
	} else {
		st.use_vbr = 0
		st.silk_mode.useCBR = 1
	}
}

func (st *OpusEncoder) GetUseConstrainedVBR() bool {
	return st.vbr_constraint != 0
}

func (st *OpusEncoder) SetUseConstrainedVBR(value bool) {
	if value {
		st.vbr_constraint = 1
	} else {
		st.vbr_constraint = 0
	}
}

func (st *OpusEncoder) GetSignalType() OpusSignal {
	return st.signal_type
}

func (st *OpusEncoder) SetSignalType(value OpusSignal) {
	st.signal_type = value
}

func (st *OpusEncoder) GetLookahead() int {
	returnVal := st.Fs / 400
	if st.application != OPUS_APPLICATION_RESTRICTED_LOWDELAY {
		returnVal += st.delay_compensation
	}
	return returnVal
}

func (st *OpusEncoder) GetSampleRate() int {
	return st.Fs
}

func (st *OpusEncoder) GetFinalRange() int {
	return st.rangeFinal
}

func (st *OpusEncoder) GetLSBDepth() int {
	return st.lsb_depth
}

func (st *OpusEncoder) SetLSBDepth(value int) {
	if value < 8 || value > 24 {
		panic("LSB depth must be between 8 and 24")
	}
	st.lsb_depth = value
}

func (st *OpusEncoder) GetExpertFrameDuration() OpusFramesize {
	return st.variable_duration
}

func (st *OpusEncoder) SetExpertFrameDuration(value OpusFramesize) {
	st.variable_duration = value
	st.Celt_Encoder.SetExpertFrameDuration(value)
}

func (st *OpusEncoder) GetForceMode() OpusMode {
	return st.user_forced_mode
}

func (st *OpusEncoder) SetForceMode(value OpusMode) {
	st.user_forced_mode = value
}

func (st *OpusEncoder) GetIsLFE() bool {
	return st.lfe != 0
}

func (st *OpusEncoder) SetIsLFE(value bool) {
	if value {
		st.lfe = 1
	} else {
		st.lfe = 0
	}
	st.Celt_Encoder.SetLFE(btol(value))
}

func (st *OpusEncoder) GetPredictionDisabled() bool {
	return st.silk_mode.reducedDependency != 0
}

func (st *OpusEncoder) SetPredictionDisabled(value bool) {
	if value {
		st.silk_mode.reducedDependency = 1
	} else {
		st.silk_mode.reducedDependency = 0
	}
}

func (st *OpusEncoder) GetEnableAnalysis() bool {
	return st.analysis.enabled
}

func (st *OpusEncoder) SetEnableAnalysis(value bool) {
	st.analysis.enabled = value
}

func (st *OpusEncoder) SetEnergyMask(value []int) {
	st.energy_masking = value
	st.Celt_Encoder.SetEnergyMask(value)
}

func (st *OpusEncoder) GetCeltMode() *CeltMode {
	return st.Celt_Encoder.GetMode()
}

func btol(b bool) int {
	if b {
		return 1
	}
	return 0
}

func imin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func imax(a, b int) int {
	if a > b {
		return a
	}
	return b
}