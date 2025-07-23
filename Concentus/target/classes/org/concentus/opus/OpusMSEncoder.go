package opus

import (
	"errors"
)

type OpusMSEncoder struct {
	layout            ChannelLayout
	lfe_stream        int
	application       OpusApplication
	variable_duration OpusFramesize
	surround          int
	bitrate_bps       int
	subframe_mem      [3]float32
	encoders          []*OpusEncoder
	window_mem        []int32
	preemph_mem       []int32
}

const (
	MS_FRAME_TMP = 3*1275 + 7
)

func NewOpusMSEncoder(nb_streams, nb_coupled_streams int) (*OpusMSEncoder, error) {
	if nb_streams < 1 || nb_coupled_streams > nb_streams || nb_coupled_streams < 0 {
		return nil, errors.New("invalid channel count in MS encoder")
	}

	encoders := make([]*OpusEncoder, nb_streams)
	for c := 0; c < nb_streams; c++ {
		encoders[c] = NewOpusEncoder()
	}

	nb_channels := nb_coupled_streams*2 + (nb_streams - nb_coupled_streams)
	window_mem := make([]int32, nb_channels*120)
	preemph_mem := make([]int32, nb_channels)

	return &OpusMSEncoder{
		encoders:    encoders,
		window_mem:  window_mem,
		preemph_mem: preemph_mem,
	}, nil
}

func (e *OpusMSEncoder) ResetState() {
	e.subframe_mem = [3]float32{0, 0, 0}
	if e.surround != 0 {
		for i := range e.preemph_mem {
			e.preemph_mem[i] = 0
		}
		for i := range e.window_mem {
			e.window_mem[i] = 0
		}
	}
	for _, enc := range e.encoders {
		enc.ResetState()
	}
}

func validateEncoderLayout(layout *ChannelLayout) bool {
	for s := 0; s < layout.nb_streams; s++ {
		if s < layout.nb_coupled_streams {
			if GetLeftChannel(layout, s, -1) == -1 {
				return false
			}
			if GetRightChannel(layout, s, -1) == -1 {
				return false
			}
		} else if GetMonoChannel(layout, s, -1) == -1 {
			return false
		}
	}
	return true
}

func channelPos(channels int, pos *[8]int) {
	switch channels {
	case 4:
		pos[0], pos[1], pos[2], pos[3] = 1, 3, 1, 3
	case 3, 5, 6:
		pos[0], pos[1], pos[2], pos[3], pos[4], pos[5] = 1, 2, 3, 1, 3, 0
	case 7:
		pos[0], pos[1], pos[2], pos[3], pos[4], pos[5], pos[6] = 1, 2, 3, 1, 3, 2, 0
	case 8:
		pos[0], pos[1], pos[2], pos[3], pos[4], pos[5], pos[6], pos[7] = 1, 2, 3, 1, 3, 1, 3, 0
	}
}

var diffTable = [17]int16{
	QCONST16(0.5000000, DB_SHIFT),
	QCONST16(0.2924813, DB_SHIFT),
	QCONST16(0.1609640, DB_SHIFT),
	QCONST16(0.0849625, DB_SHIFT),
	QCONST16(0.0437314, DB_SHIFT),
	QCONST16(0.0221971, DB_SHIFT),
	QCONST16(0.0111839, DB_SHIFT),
	QCONST16(0.0056136, DB_SHIFT),
	QCONST16(0.0028123, DB_SHIFT),
	0, 0, 0, 0, 0, 0, 0, 0,
}

func logSum(a, b int) int {
	var max, diff int
	if a > b {
		max = a
		diff = a - b
	} else {
		max = b
		diff = b - a
	}

	if diff >= QCONST16(8.0, DB_SHIFT) {
		return max
	}

	low := diff >> (DB_SHIFT - 1)
	frac := (diff - (low << (DB_SHIFT - 1))) << (16 - DB_SHIFT)
	return max + int(diffTable[low]) + MULT16_16_Q15(frac, int(diffTable[low+1]-diffTable[low]))
}

func (e *OpusMSEncoder) surroundAnalysis(celt_mode *CeltMode, pcm []int16, bandLogE, mem, preemph_mem []int32, len, overlap, channels, rate int) {
	var pos [8]int
	channelPos(channels, &pos)

	upsample := ResamplingFactor(rate)
	frame_size := len * upsample

	LM := 0
	for ; LM < celt_mode.maxLM; LM++ {
		if celt_mode.shortMdctSize<<LM == frame_size {
			break
		}
	}

	input := make([]int32, frame_size+overlap)
	x := make([]int16, len)
	freq := make([][]int32, 1)
	freq[0] = make([]int32, frame_size)

	maskLogE := make([][]int32, 3)
	for c := 0; c < 3; c++ {
		maskLogE[c] = make([]int32, 21)
		for i := 0; i < 21; i++ {
			maskLogE[c][i] = -QCONST16(28.0, DB_SHIFT)
		}
	}

	for c := 0; c < channels; c++ {
		copy(input[:overlap], mem[c*overlap:(c+1)*overlap])
		opusCopyChannelInShort(x, 0, 1, pcm, channels, c, len)

		preemph := preemph_mem[c]
		CeltPreemphasis(x, input, overlap, frame_size, 1, upsample, celt_mode.preemph, &preemph)
		preemph_mem[c] = preemph

		MDCTForward(celt_mode.mdct, input, freq[0], celt_mode.window, overlap, celt_mode.maxLM-LM, 1)

		if upsample != 1 {
			bound := len
			for i := 0; i < bound; i++ {
				freq[0][i] *= int32(upsample)
			}
			for i := bound; i < frame_size; i++ {
				freq[0][i] = 0
			}
		}

		bandE := make([][]int32, 1)
		bandE[0] = make([]int32, 21)
		ComputeBandEnergies(celt_mode, freq, bandE, 21, 1, LM)
		Amp2Log2(celt_mode, 21, 21, bandE[0], bandLogE, 21*c, 1)

		for i := 1; i < 21; i++ {
			bandLogE[21*c+i] = MAX16(bandLogE[21*c+i], bandLogE[21*c+i-1]-QCONST16(1.0, DB_SHIFT))
		}
		for i := 19; i >= 0; i-- {
			bandLogE[21*c+i] = MAX16(bandLogE[21*c+i], bandLogE[21*c+i+1]-QCONST16(2.0, DB_SHIFT))
		}

		switch pos[c] {
		case 1:
			for i := 0; i < 21; i++ {
				maskLogE[0][i] = logSum(int(maskLogE[0][i]), int(bandLogE[21*c+i]))
			}
		case 3:
			for i := 0; i < 21; i++ {
				maskLogE[2][i] = logSum(int(maskLogE[2][i]), int(bandLogE[21*c+i]))
			}
		case 2:
			for i := 0; i < 21; i++ {
				val := bandLogE[21*c+i] - QCONST16(0.5, DB_SHIFT)
				maskLogE[0][i] = logSum(int(maskLogE[0][i]), int(val))
				maskLogE[2][i] = logSum(int(maskLogE[2][i]), int(val))
			}
		}
		copy(mem[c*overlap:(c+1)*overlap], input[frame_size:frame_size+overlap])
	}

	for i := 0; i < 21; i++ {
		maskLogE[1][i] = MIN32(maskLogE[0][i], maskLogE[2][i])
	}

	channel_offset := HALF16(CeltLog2(QCONST32(2.0, 14) / (channels - 1)))
	for c := 0; c < 3; c++ {
		for i := 0; i < 21; i++ {
			maskLogE[c][i] += channel_offset
		}
	}

	for c := 0; c < channels; c++ {
		if pos[c] != 0 {
			mask := maskLogE[pos[c]-1]
			for i := 0; i < 21; i++ {
				bandLogE[21*c+i] -= mask[i]
			}
		} else {
			for i := 0; i < 21; i++ {
				bandLogE[21*c+i] = 0
			}
		}
	}
}

func (e *OpusMSEncoder) Init(Fs, channels, streams, coupled_streams int, mapping []byte, application OpusApplication, surround int) error {
	if channels > 255 || channels < 1 || coupled_streams > streams ||
		streams < 1 || coupled_streams < 0 || streams > 255-coupled_streams {
		return errors.New("invalid channel configuration")
	}

	e.layout.nb_channels = channels
	e.layout.nb_streams = streams
	e.layout.nb_coupled_streams = coupled_streams
	e.subframe_mem = [3]float32{0, 0, 0}
	if surround == 0 {
		e.lfe_stream = -1
	}
	e.bitrate_bps = OPUS_AUTO
	e.application = application
	e.variable_duration = OPUS_FRAMESIZE_ARG

	for i := 0; i < e.layout.nb_channels; i++ {
		e.layout.mapping[i] = mapping[i]
	}

	if !ValidateLayout(&e.layout) || !validateEncoderLayout(&e.layout) {
		return errors.New("invalid layout")
	}

	encoder_ptr := 0
	for i := 0; i < e.layout.nb_coupled_streams; i++ {
		if err := e.encoders[encoder_ptr].Init(Fs, 2, application); err != nil {
			return err
		}
		if i == e.lfe_stream {
			e.encoders[encoder_ptr].SetIsLFE(true)
		}
		encoder_ptr++
	}

	for i := e.layout.nb_coupled_streams; i < e.layout.nb_streams; i++ {
		if err := e.encoders[encoder_ptr].Init(Fs, 1, application); err != nil {
			return err
		}
		if i == e.lfe_stream {
			e.encoders[encoder_ptr].SetIsLFE(true)
		}
		encoder_ptr++
	}

	if surround != 0 {
		for i := range e.preemph_mem {
			e.preemph_mem[i] = 0
		}
		for i := range e.window_mem {
			e.window_mem[i] = 0
		}
	}
	e.surround = surround
	return nil
}

func (e *OpusMSEncoder) InitSurround(Fs, channels, mapping_family int, streams, coupled_streams *int, mapping []byte, application OpusApplication) error {
	*streams = 0
	*coupled_streams = 0
	if channels > 255 || channels < 1 {
		return errors.New("invalid channel count")
	}

	e.lfe_stream = -1
	switch mapping_family {
	case 0:
		switch channels {
		case 1:
			*streams = 1
			*coupled_streams = 0
			mapping[0] = 0
		case 2:
			*streams = 1
			*coupled_streams = 1
			mapping[0] = 0
			mapping[1] = 1
		default:
			return errors.New("unsupported channel count for mapping family 0")
		}
	case 1:
		if channels < 1 || channels > 8 {
			return errors.New("invalid channel count for mapping family 1")
		}
		vm := VorbisMappings[channels-1]
		*streams = vm.nb_streams
		*coupled_streams = vm.nb_coupled_streams
		copy(mapping, vm.mapping[:channels])
		if channels >= 6 {
			e.lfe_stream = *streams - 1
		}
	case 255:
		*streams = channels
		*coupled_streams = 0
		for i := 0; i < channels; i++ {
			mapping[i] = byte(i)
		}
	default:
		return errors.New("unsupported mapping family")
	}

	return e.Init(Fs, channels, *streams, *coupled_streams, mapping, application, boolToInt(channels > 2 && mapping_family == 1))
}

func CreateMSEncoder(Fs, channels, streams, coupled_streams int, mapping []byte, application OpusApplication) (*OpusMSEncoder, error) {
	if channels > 255 || channels < 1 || coupled_streams > streams ||
		streams < 1 || coupled_streams < 0 || streams > 255-coupled_streams {
		return nil, errors.New("invalid channel/stream configuration")
	}

	st, err := NewOpusMSEncoder(streams, coupled_streams)
	if err != nil {
		return nil, err
	}

	if err := st.Init(Fs, channels, streams, coupled_streams, mapping, application, 0); err != nil {
		if err.Error() == "invalid layout" {
			return nil, errors.New("OPUS_BAD_ARG when creating MS encoder")
		}
		return nil, err
	}
	return st, nil
}

func GetStreamCount(channels, mapping_family int) (nb_streams, nb_coupled_streams int, err error) {
	if mapping_family == 0 {
		switch channels {
		case 1:
			return 1, 0, nil
		case 2:
			return 1, 1, nil
		default:
			return 0, 0, errors.New("more than 2 channels requires custom mappings")
		}
	} else if mapping_family == 1 && channels >= 1 && channels <= 8 {
		vm := VorbisMappings[channels-1]
		return vm.nb_streams, vm.nb_coupled_streams, nil
	} else if mapping_family == 255 {
		return channels, 0, nil
	}
	return 0, 0, errors.New("invalid mapping family")
}

func CreateSurroundEncoder(Fs, channels, mapping_family int, mapping []byte, application OpusApplication) (*OpusMSEncoder, error) {
	if channels > 255 || channels < 1 || application == OPUS_APPLICATION_UNIMPLEMENTED {
		return nil, errors.New("invalid channel count or application")
	}

	nb_streams, nb_coupled_streams, err := GetStreamCount(channels, mapping_family)
	if err != nil {
		return nil, err
	}

	st, err := NewOpusMSEncoder(nb_streams, nb_coupled_streams)
	if err != nil {
		return nil, err
	}

	var streams, coupled_streams int
	if err := st.InitSurround(Fs, channels, mapping_family, &streams, &coupled_streams, mapping, application); err != nil {
		if err.Error() == "invalid layout" {
			return nil, errors.New("bad argument passed to CreateSurround")
		}
		return nil, err
	}
	return st, nil
}

func (e *OpusMSEncoder) surroundRateAllocation(out_rates []int, frame_size int) int {
	Fs := e.encoders[0].GetSampleRate()
	var stream_offset, lfe_offset int

	if e.bitrate_bps > e.layout.nb_channels*40000 {
		stream_offset = 20000
	} else {
		stream_offset = e.bitrate_bps / e.layout.nb_channels / 2
	}
	stream_offset += 60 * (Fs/frame_size - 50)
	lfe_offset = 3500 + 60*(Fs/frame_size-50)
	coupled_ratio := 512
	lfe_ratio := 32

	var channel_rate int
	if e.bitrate_bps == OPUS_AUTO {
		channel_rate = Fs + 60*Fs/frame_size
	} else if e.bitrate_bps == OPUS_BITRATE_MAX {
		channel_rate = 300000
	} else {
		nb_lfe := 0
		if e.lfe_stream != -1 {
			nb_lfe = 1
		}
		nb_coupled := e.layout.nb_coupled_streams
		nb_uncoupled := e.layout.nb_streams - nb_coupled - nb_lfe
		total := (nb_uncoupled << 8) + coupled_ratio*nb_coupled + nb_lfe*lfe_ratio
		channel_rate = 256 * (e.bitrate_bps - lfe_offset*nb_lfe - stream_offset*(nb_coupled+nb_uncoupled)) / total
	}

	rate_sum := 0
	for i := 0; i < e.layout.nb_streams; i++ {
		if i < e.layout.nb_coupled_streams {
			out_rates[i] = stream_offset + (channel_rate * coupled_ratio >> 8)
		} else if i != e.lfe_stream {
			out_rates[i] = stream_offset + channel_rate
		} else {
			out_rates[i] = lfe_offset + (channel_rate * lfe_ratio >> 8)
		}
		out_rates[i] = max(out_rates[i], 500)
		rate_sum += out_rates[i]
	}
	return rate_sum
}

func (e *OpusMSEncoder) EncodeNative(pcm []int16, frame_size int, data []byte, max_data_bytes int, lsb_depth int, float_api bool) (int, error) {
	Fs := e.encoders[0].GetSampleRate()
	vbr := e.encoders[0].GetUseVBR()
	celt_mode := e.encoders[0].GetCeltMode()

	channels := e.layout.nb_streams + e.layout.nb_coupled_streams
	delay_compensation := e.encoders[0].GetLookahead() - Fs/400
	frame_size = ComputeFrameSize(pcm, frame_size, e.variable_duration, channels, Fs, e.bitrate_bps,
		delay_compensation, e.subframe_mem[:], e.encoders[0].AnalysisEnabled())

	if 400*frame_size < Fs {
		return 0, errors.New("invalid frame size")
	}

	if !(400*frame_size == Fs || 200*frame_size == Fs ||
		100*frame_size == Fs || 50*frame_size == Fs ||
		25*frame_size == Fs || 50*frame_size == 3*Fs) {
		return 0, errors.New("invalid frame size")
	}

	smallest_packet := e.layout.nb_streams*2 - 1
	if max_data_bytes < smallest_packet {
		return 0, errors.New("output buffer too small")
	}

	buf := make([]int16, 2*frame_size)
	bandSMR := make([]int32, 21*e.layout.nb_channels)

	if e.surround != 0 {
		e.surroundAnalysis(celt_mode, pcm, bandSMR, e.window_mem, e.preemph_mem, frame_size, 120, e.layout.nb_channels, Fs)
	}

	bitrates := make([]int, 256)
	rate_sum := e.surroundRateAllocation(bitrates, frame_size)

	if vbr == 0 {
		if e.bitrate_bps == OPUS_AUTO {
			max_data_bytes = min(max_data_bytes, 3*rate_sum/(3*8*Fs/frame_size))
		} else if e.bitrate_bps != OPUS_BITRATE_MAX {
			max_data_bytes = min(max_data_bytes, max(smallest_packet, 3*e.bitrate_bps/(3*8*Fs/frame_size)))
		}
	}

	for s, enc := range e.encoders {
		enc.SetBitrate(bitrates[s])
		if e.surround != 0 {
			equiv_rate := e.bitrate_bps
			if frame_size*50 < Fs {
				equiv_rate -= 60 * (Fs/frame_size - 50) * e.layout.nb_channels
			}
			switch {
			case equiv_rate > 10000*e.layout.nb_channels:
				enc.SetBandwidth(OPUS_BANDWIDTH_FULLBAND)
			case equiv_rate > 7000*e.layout.nb_channels:
				enc.SetBandwidth(OPUS_BANDWIDTH_SUPERWIDEBAND)
			case equiv_rate > 5000*e.layout.nb_channels:
				enc.SetBandwidth(OPUS_BANDWIDTH_WIDEBAND)
			default:
				enc.SetBandwidth(OPUS_BANDWIDTH_NARROWBAND)
			}
			if s < e.layout.nb_coupled_streams {
				enc.SetForceMode(MODE_CELT_ONLY)
				enc.SetForceChannels(2)
			}
		}
	}

	rp := NewRepacketizer()
	tot_size := 0
	tmp_data := make([]byte, MS_FRAME_TMP)

	for s := 0; s < e.layout.nb_streams; s++ {
		enc := e.encoders[s]
		var len int
		var c1, c2 int
		bandLogE := make([]int32, 42)

		rp.Reset()
		if s < e.layout.nb_coupled_streams {
			left := GetLeftChannel(&e.layout, s, -1)
			right := GetRightChannel(&e.layout, s, -1)
			opusCopyChannelInShort(buf, 0, 2, pcm, e.layout.nb_channels, left, frame_size)
			opusCopyChannelInShort(buf, 1, 2, pcm, e.layout.nb_channels, right, frame_size)
			if e.surround != 0 {
				for i := 0; i < 21; i++ {
					bandLogE[i] = bandSMR[21*left+i]
					bandLogE[21+i] = bandSMR[21*right+i]
				}
			}
			c1, c2 = left, right
		} else {
			chan1 := GetMonoChannel(&e.layout, s, -1)
			opusCopyChannelInShort(buf, 0, 1, pcm, e.layout.nb_channels, chan1, frame_size)
			if e.surround != 0 {
				for i := 0; i < 21; i++ {
					bandLogE[i] = bandSMR[21*chan1+i]
				}
			}
			c1, c2 = chan1, -1
		}

		if e.surround != 0 {
			enc.SetEnergyMask(bandLogE)
		}

		curr_max := max_data_bytes - tot_size
		curr_max -= max(0, 2*(e.layout.nb_streams-s-1)-1)
		curr_max = min(curr_max, MS_FRAME_TMP)
		if s != e.layout.nb_streams-1 {
			if curr_max > 253 {
				curr_max -= 2
			} else {
				curr_max -= 1
			}
		}
		if vbr == 0 && s == e.layout.nb_streams-1 {
			enc.SetBitrate(curr_max * (8 * Fs / frame_size))
		}

		var err error
		len, err = enc.EncodeNative(buf, frame_size, tmp_data, curr_max, lsb_depth, pcm, frame_size, c1, c2, e.layout.nb_channels, float_api)
		if err != nil {
			return 0, err
		}

		rp.AddPacket(tmp_data[:len])
		len = rp.OutRange(0, rp.GetNumFrames(), data[tot_size:], max_data_bytes-tot_size, s != e.layout.nb_streams-1, vbr == 0 && s == e.layout.nb_streams-1)
		tot_size += len
	}

	return tot_size, nil
}

func opusCopyChannelInShort(dst []int16, dst_ptr, dst_stride int, src []int16, src_stride, src_channel, frame_size int) {
	for i := 0; i < frame_size; i++ {
		dst[dst_ptr+i*dst_stride] = src[i*src_stride+src_channel]
	}
}

func (e *OpusMSEncoder) Encode(pcm []int16, frame_size int, output []byte, max_data_bytes int) (int, error) {
	return e.EncodeNative(pcm, frame_size, output, max_data_bytes, 16, false)
}

func (e *OpusMSEncoder) GetBitrate() int {
	value := 0
	for _, enc := range e.encoders {
		value += enc.GetBitrate()
	}
	return value
}

func (e *OpusMSEncoder) SetBitrate(value int) error {
	if value < 0 && value != OPUS_AUTO && value != OPUS_BITRATE_MAX {
		return errors.New("invalid bitrate")
	}
	e.bitrate_bps = value
	return nil
}

func (e *OpusMSEncoder) GetApplication() OpusApplication {
	return e.encoders[0].GetApplication()
}

func (e *OpusMSEncoder) SetApplication(value OpusApplication) {
	for _, enc := range e.encoders {
		enc.SetApplication(value)
	}
}

// ... (remaining getter/setter methods follow the same pattern)

func (e *OpusMSEncoder) GetEncoderState(streamId int) (*OpusEncoder, error) {
	if streamId >= e.layout.nb_streams {
		return nil, errors.New("requested stream doesn't exist")
	}
	return e.encoders[streamId], nil
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
