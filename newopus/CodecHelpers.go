package opus

import "math"

func gen_toc(mode OpusMode, framerate int, bandwidth OpusBandwidth, channels int) byte {
	var period int
	var toc int16
	period = 0
	for framerate < 400 {
		framerate <<= 1
		period++
	}
	if mode == MODE_SILK_ONLY {
		toc = int16((OpusBandwidthHelpers_GetOrdinal(bandwidth) - OpusBandwidthHelpers_GetOrdinal(OPUS_BANDWIDTH_NARROWBAND)) << 5)
		toc |= int16((period - 2) << 3)
	} else if mode == MODE_CELT_ONLY {
		tmp := OpusBandwidthHelpers_GetOrdinal(bandwidth) - OpusBandwidthHelpers_GetOrdinal(OPUS_BANDWIDTH_MEDIUMBAND)
		if tmp < 0 {
			tmp = 0
		}
		toc = 0x80
		toc |= int16(tmp << 5)
		toc |= int16(period << 3)
	} else {
		toc = 0x60
		toc |= int16((OpusBandwidthHelpers_GetOrdinal(bandwidth) - OpusBandwidthHelpers_GetOrdinal(OPUS_BANDWIDTH_SUPERWIDEBAND)) << 4)
		toc |= int16((period - 2) << 3)
	}
	toc |= int16((map[bool]int{true: 1, false: 0}[channels == 2]) << 2)
	return byte(0xFF & toc)
}

func hp_cutoff(input []int16, input_ptr int, cutoff_Hz int, output []int16, output_ptr int, hp_mem []int32, len int, channels int, Fs int) {
	var B_Q28 [3]int32
	var A_Q28 [2]int32
	var Fc_Q19, r_Q28, r_Q22 int32

	Inlines_OpusAssert(cutoff_Hz <= 2147483647/((int32)((1.5*3.14159/1000)*(1<<19)+0.5)))
	Fc_Q19 = Inlines_silk_DIV32_16(Inlines_silk_SMULBB((int32)((1.5*3.14159/1000)*(1<<19)+0.5, int32(cutoff_Hz)), int32(Fs/1000)))
	Inlines_OpusAssert(Fc_Q19 > 0 && Fc_Q19 < 32768)

	r_Q28 = (int32)((1.0)*(1<<28)+0.5) - Inlines_silk_MUL((int32)((0.92)*(1<<9)+0.5), Fc_Q19)

	B_Q28[0] = r_Q28
	B_Q28[1] = Inlines_silk_LSHIFT(-r_Q28, 1)
	B_Q28[2] = r_Q28

	r_Q22 = Inlines_silk_RSHIFT(r_Q28, 6)
	A_Q28[0] = Inlines_silk_SMULWW(r_Q22, Inlines_silk_SMULWW(Fc_Q19, Fc_Q19)-(int32)((2.0)*(1<<22)+0.5))
	A_Q28[1] = Inlines_silk_SMULWW(r_Q22, r_Q22)

	Filters_silk_biquad_alt(input, input_ptr, B_Q28[:], A_Q28[:], hp_mem, 0, output, output_ptr, len, channels)
	if channels == 2 {
		Filters_silk_biquad_alt(input, input_ptr+1, B_Q28[:], A_Q28[:], hp_mem, 2, output, output_ptr+1, len, channels)
	}
}

func dc_reject(input []int16, input_ptr int, cutoff_Hz int, output []int16, output_ptr int, hp_mem []int32, len int, channels int, Fs int) {
	var shift int

	shift = Inlines_celt_ilog2(Fs / (cutoff_Hz * 3))
	for c := 0; c < channels; c++ {
		for i := 0; i < len; i++ {
			var x, tmp, y int32
			x = Inlines_SHL32(Inlines_EXTEND32(int32(input[channels*i+c+input_ptr])), 15)
			tmp = x - hp_mem[2*c]
			hp_mem[2*c] = hp_mem[2*c] + Inlines_PSHR32(x-hp_mem[2*c], shift)
			y = tmp - hp_mem[2*c+1]
			hp_mem[2*c+1] = hp_mem[2*c+1] + Inlines_PSHR32(tmp-hp_mem[2*c+1], shift)
			output[channels*i+c+output_ptr] = int16(Inlines_EXTRACT16(Inlines_SATURATE(Inlines_PSHR32(y, 15), 32767)))
		}
	}
}

func stereo_fade(pcm_buf []int16, g1 int, g2 int, overlap48 int, frame_size int, channels int, window []int32, Fs int) {
	var overlap, inc int
	inc = 48000 / Fs
	overlap = overlap48 / inc
	g1 = CeltConstants_Q15ONE - g1
	g2 = CeltConstants_Q15ONE - g2
	for i := 0; i < overlap; i++ {
		var diff int32
		var g, w int32
		w = Inlines_MULT16_16_Q15(window[i*inc], window[i*inc])
		g = Inlines_SHR32(Inlines_MAC16_16(Inlines_MULT16_16(w, int32(g2)), CeltConstants_Q15ONE-w, int32(g1)), 15)
		diff = Inlines_EXTRACT16(Inlines_HALF32(int32(pcm_buf[i*channels]) - int32(pcm_buf[i*channels+1])))
		diff = Inlines_MULT16_16_Q15(int32(g), diff)
		pcm_buf[i*channels] = int16(int32(pcm_buf[i*channels]) - diff)
		pcm_buf[i*channels+1] = int16(int32(pcm_buf[i*channels+1]) + diff)
	}
	for i := overlap; i < frame_size; i++ {
		var diff int32
		diff = Inlines_EXTRACT16(Inlines_HALF32(int32(pcm_buf[i*channels]) - int32(pcm_buf[i*channels+1])))
		diff = Inlines_MULT16_16_Q15(int32(g2), diff)
		pcm_buf[i*channels] = int16(int32(pcm_buf[i*channels]) - diff)
		pcm_buf[i*channels+1] = int16(int32(pcm_buf[i*channels+1]) + diff)
	}
}

func gain_fade(buffer []int16, buf_ptr int, g1 int, g2 int, overlap48 int, frame_size int, channels int, window []int32, Fs int) {
	var inc, overlap int
	inc = 48000 / Fs
	overlap = overlap48 / inc
	if channels == 1 {
		for i := 0; i < overlap; i++ {
			var g, w int32
			w = Inlines_MULT16_16_Q15(window[i*inc], window[i*inc])
			g = Inlines_SHR32(Inlines_MAC16_16(Inlines_MULT16_16(w, int32(g2)), CeltConstants_Q15ONE-w, int32(g1)), 15)
			buffer[buf_ptr+i] = int16(Inlines_MULT16_16_Q15(int32(g), int32(buffer[buf_ptr+i])))
		}
	} else {
		for i := 0; i < overlap; i++ {
			var g, w int32
			w = Inlines_MULT16_16_Q15(window[i*inc], window[i*inc])
			g = Inlines_SHR32(Inlines_MAC16_16(Inlines_MULT16_16(w, int32(g2)), CeltConstants_Q15ONE-w, int32(g1)), 15)
			buffer[buf_ptr+i*2] = int16(Inlines_MULT16_16_Q15(int32(g), int32(buffer[buf_ptr+i*2])))
			buffer[buf_ptr+i*2+1] = int16(Inlines_MULT16_16_Q15(int32(g), int32(buffer[buf_ptr+i*2+1])))
		}
	}
	for c := 0; c < channels; c++ {
		for i := overlap; i < frame_size; i++ {
			buffer[buf_ptr+i*channels+c] = int16(Inlines_MULT16_16_Q15(int32(g2), int32(buffer[buf_ptr+i*channels+c])))
		}
	}
}

const MAX_DYNAMIC_FRAMESIZE = 24

func transient_boost(E []float32, E_ptr int, E_1 []float32, LM int, maxM int) float32 {
	var M int
	var sumE, sumE_1 float32

	M = Inlines_IMIN(maxM, (1<<LM)+1)
	for i := E_ptr; i < M+E_ptr; i++ {
		sumE += E[i]
		sumE_1 += E_1[i]
	}
	metric := sumE * sumE_1 / float32(M*M)
	return Inlines_MIN16(1, float32(math.Sqrt(float64(Inlines_MAX16(0, 0.05*(metric-2))))))
}

func transient_viterbi(E []float32, E_1 []float32, N int, frame_cost int, rate int) int {
	cost := make([][]float32, MAX_DYNAMIC_FRAMESIZE)
	states := make([][]int, MAX_DYNAMIC_FRAMESIZE)
	for i := range cost {
		cost[i] = make([]float32, 16)
		states[i] = make([]int, 16)
	}
	var factor float32

	if rate < 80 {
		factor = 0
	} else if rate > 160 {
		factor = 1
	} else {
		factor = float32(rate-80) / 80.0
	}
	for i := 0; i < 16; i++ {
		states[0][i] = -1
		cost[0][i] = 1e10
	}
	for i := 0; i < 4; i++ {
		shift := 1 << i
		cost[0][shift] = float32(frame_cost+rate*shift) * (1 + factor*transient_boost(E, 0, E_1, i, N+1))
		states[0][shift] = i
	}
	for i := 1; i < N; i++ {
		for j := 2; j < 16; j++ {
			cost[i][j] = cost[i-1][j-1]
			states[i][j] = j - 1
		}
		for j := 0; j < 4; j++ {
			shift := 1 << j
			min_cost := cost[i-1][1]
			states[i][shift] = 1
			for k := 1; k < 4; k++ {
				state_val := (1 << (k + 1)) - 1
				tmp := cost[i-1][state_val]
				if tmp < min_cost {
					states[i][shift] = state_val
					min_cost = tmp
				}
			}
			curr_cost := float32(frame_cost+rate*shift) * (1 + factor*transient_boost(E, i, E_1, j, N-i+1))
			cost[i][shift] = min_cost
			if N-i < shift {
				cost[i][shift] += curr_cost * float32(N-i) / float32(shift)
			} else {
				cost[i][shift] += curr_cost
			}
		}
	}
	best_state := 1
	best_cost := cost[N-1][1]
	for i := 2; i < 16; i++ {
		if cost[N-1][i] < best_cost {
			best_cost = cost[N-1][i]
			best_state = i
		}
	}
	for i := N - 1; i >= 0; i-- {
		best_state = states[i][best_state]
	}
	return best_state
}

func optimize_framesize(x []int16, x_ptr int, len int, C int, Fs int, bitrate int, tonality int, mem []float32, buffering int) int {
	var N, pos, offset int
	e := make([]float32, MAX_DYNAMIC_FRAMESIZE+4)
	e_1 := make([]float32, MAX_DYNAMIC_FRAMESIZE+3)
	var memx int

	subframe := Fs / 400
	sub := make([]int, subframe)
	e[0] = mem[0]
	e_1[0] = 1.0 / (CeltConstants_EPSILON + mem[0])
	if buffering != 0 {
		offset = 2*subframe - buffering
		if offset < 0 || offset > subframe {
			panic("offset out of range")
		}
		len -= offset
		e[1] = mem[1]
		e_1[1] = 1.0 / (CeltConstants_EPSILON + mem[1])
		e[2] = mem[2]
		e_1[2] = 1.0 / (CeltConstants_EPSILON + mem[2])
		pos = 3
	} else {
		pos = 1
		offset = 0
	}
	N = Inlines_IMIN(len/subframe, MAX_DYNAMIC_FRAMESIZE)
	for i := 0; i < N; i++ {
		tmp := CeltConstants_EPSILON
		var tmpx int
		Downmix_downmix_int(x, x_ptr, sub, 0, subframe, i*subframe+offset, 0, -2, C)
		if i == 0 {
			memx = sub[0]
		}
		for j := 0; j < subframe; j++ {
			tmpx = sub[j]
			diff := float32(tmpx - memx)
			tmp += diff * diff
			memx = tmpx
		}
		e[i+pos] = tmp
		e_1[i+pos] = 1.0 / tmp
	}
	e[N+pos] = e[N+pos-1]
	if buffering != 0 {
		N = Inlines_IMIN(MAX_DYNAMIC_FRAMESIZE, N+2)
	}
	bestLM := transient_viterbi(e, e_1, N, int(1.0+0.5*float32(tonality))*(60*C+40), bitrate/400)
	mem[0] = e[1<<bestLM]
	if buffering != 0 {
		mem[1] = e[(1<<bestLM)+1]
		mem[2] = e[(1<<bestLM)+2]
	}
	return bestLM
}

func frame_size_select(frame_size int, variable_duration OpusFramesize, Fs int) int {
	var new_size int
	if frame_size < Fs/400 {
		return -1
	}
	if variable_duration == OPUS_FRAMESIZE_ARG {
		new_size = frame_size
	} else if variable_duration == OPUS_FRAMESIZE_VARIABLE {
		new_size = Fs / 50
	} else if OpusFramesizeHelpers_GetOrdinal(variable_duration) >= OpusFramesizeHelpers_GetOrdinal(OPUS_FRAMESIZE_2_5_MS) &&
		OpusFramesizeHelpers_GetOrdinal(variable_duration) <= OpusFramesizeHelpers_GetOrdinal(OPUS_FRAMESIZE_60_MS) {
		new_size = Inlines_IMIN(3*Fs/50, (Fs/400)<<(OpusFramesizeHelpers_GetOrdinal(variable_duration)-OpusFramesizeHelpers_GetOrdinal(OPUS_FRAMESIZE_2_5_MS)))
	} else {
		return -1
	}
	if new_size > frame_size {
		return -1
	}
	if 400*new_size != Fs && 200*new_size != Fs && 100*new_size != Fs &&
		50*new_size != Fs && 25*new_size != Fs && 50*new_size != 3*Fs {
		return -1
	}
	return new_size
}

func compute_frame_size(analysis_pcm []int16, analysis_pcm_ptr int, frame_size int, variable_duration OpusFramesize, C int, Fs int, bitrate_bps int, delay_compensation int, subframe_mem []float32, analysis_enabled bool) int {
	if analysis_enabled && variable_duration == OPUS_FRAMESIZE_VARIABLE && frame_size >= Fs/200 {
		LM := 3
		LM = optimize_framesize(analysis_pcm, analysis_pcm_ptr, frame_size, C, Fs, bitrate_bps, 0, subframe_mem, delay_compensation)
		for (Fs/400)<<LM > frame_size {
			LM--
		}
		frame_size = (Fs / 400) << LM
	} else {
		frame_size = frame_size_select(frame_size, variable_duration, Fs)
	}
	if frame_size < 0 {
		return -1
	}
	return frame_size
}

func compute_stereo_width(pcm []int16, pcm_ptr int, frame_size int, Fs int, mem *StereoWidthState) int {
	var xx, xy, yy int32
	var frame_rate, short_alpha int

	frame_rate = Fs / frame_size
	short_alpha = CeltConstants_Q15ONE - (25*CeltConstants_Q15ONE)/Inlines_IMAX(50, frame_rate)
	for i := 0; i < frame_size-3; i += 4 {
		var pxx, pxy, pyy int32
		p2i := pcm_ptr + (2 * i)
		x := int32(pcm[p2i])
		y := int32(pcm[p2i+1])
		pxx = Inlines_SHR32(Inlines_MULT16_16(x, x), 2)
		pxy = Inlines_SHR32(Inlines_MULT16_16(x, y), 2)
		pyy = Inlines_SHR32(Inlines_MULT16_16(y, y), 2)

		x = int32(pcm[p2i+2])
		y = int32(pcm[p2i+3])
		pxx += Inlines_SHR32(Inlines_MULT16_16(x, x), 2)
		pxy += Inlines_SHR32(Inlines_MULT16_16(x, y), 2)
		pyy += Inlines_SHR32(Inlines_MULT16_16(y, y), 2)

		x = int32(pcm[p2i+4])
		y = int32(pcm[p2i+5])
		pxx += Inlines_SHR32(Inlines_MULT16_16(x, x), 2)
		pxy += Inlines_SHR32(Inlines_MULT16_16(x, y), 2)
		pyy += Inlines_SHR32(Inlines_MULT16_16(y, y), 2)

		x = int32(pcm[p2i+6])
		y = int32(pcm[p2i+7])
		pxx += Inlines_SHR32(Inlines_MULT16_16(x, x), 2)
		pxy += Inlines_SHR32(Inlines_MULT16_16(x, y), 2)
		pyy += Inlines_SHR32(Inlines_MULT16_16(y, y), 2)

		xx += Inlines_SHR32(pxx, 10)
		xy += Inlines_SHR32(pxy, 10)
		yy += Inlines_SHR32(pyy, 10)
	}

	mem.XX += Inlines_MULT16_32_Q15(int32(short_alpha), xx-mem.XX)
	mem.XY += Inlines_MULT16_32_Q15(int32(short_alpha), xy-mem.XY)
	mem.YY += Inlines_MULT16_32_Q15(int32(short_alpha), yy-mem.YY)
	mem.XX = Inlines_MAX32(0, mem.XX)
	mem.XY = Inlines_MAX32(0, mem.XY)
	mem.YY = Inlines_MAX32(0, mem.YY)
	if Inlines_MAX32(mem.XX, mem.YY) > int32(0.5+(8e-4)*(1<<18)) {
		sqrt_xx := Inlines_celt_sqrt(mem.XX)
		sqrt_yy := Inlines_celt_sqrt(mem.YY)
		qrrt_xx := Inlines_celt_sqrt(sqrt_xx)
		qrrt_yy := Inlines_celt_sqrt(sqrt_yy)
		mem.XY = Inlines_MIN32(mem.XY, sqrt_xx*sqrt_yy)
		corr := Inlines_SHR32(Inlines_frac_div32(mem.XY, CeltConstants_EPSILON+Inlines_MULT16_16(sqrt_xx, sqrt_yy)), 16)
		ldiff := CeltConstants_Q15ONE * Inlines_ABS16(qrrt_xx-qrrt_yy) / (CeltConstants_EPSILON + qrrt_xx + qrrt_yy)
		width := Inlines_MULT16_16_Q15(Inlines_celt_sqrt(1<<30-Inlines_MULT16_16(corr, corr)), ldiff)
		mem.smoothed_width += (width - mem.smoothed_width) / int32(frame_rate)
		mem.max_follower = Inlines_MAX16(mem.max_follower-int32(0.5+(0.02)*(1<<15))/int32(frame_rate), mem.smoothed_width)
	} else {
		mem.smoothed_width = 0
		mem.max_follower = 0
	}
	return Inlines_EXTRACT16(Inlines_MIN32(CeltConstants_Q15ONE, 20*mem.max_follower))
}

func smooth_fade(in1 []int16, in1_ptr int, in2 []int16, in2_ptr int, output []int16, output_ptr int, overlap int, channels int, window []int32, Fs int) {
	inc := 48000 / Fs
	for c := 0; c < channels; c++ {
		for i := 0; i < overlap; i++ {
			w := Inlines_MULT16_16_Q15(window[i*inc], window[i*inc])
			output[output_ptr+(i*channels)+c] = int16(Inlines_SHR32(Inlines_MAC16_16(Inlines_MULT16_16(w, int32(in2[in2_ptr+(i*channels)+c])), CeltConstants_Q15ONE-w, int32(in1[in1_ptr+(i*channels)+c])), 15))
		}
	}
}

func opus_strerror(error int) string {
	error_strings := []string{
		"success",
		"invalid argument",
		"buffer too small",
		"error",
		"corrupted stream",
		"request not implemented",
		"invalid state",
		"memory allocation failed",
	}
	if error > 0 || error < -7 {
		return "unknown error"
	} else {
		return error_strings[-error]
	}
}

func GetVersionString() string {
	return "concentus 1.0a-java-fixed"
}
