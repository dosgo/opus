package opus
import (
	"math"
)

const (
	M_PI                  = 3.141592653
	cA                    = 0.43157974
	cB                    = 0.67848403
	cC                    = 0.08595542
	cE                    = float32(M_PI / 2)
	NB_TONAL_SKIP_BANDS   = 9
	DETECT_SIZE           = 21
	ANALYSIS_BUF_SIZE     = 720
	NB_TBANDS             = 21
	NB_TOT_BANDS          = 25
	NB_FRAMES             = 8
	SIG_SHIFT             = 0
)

var OpusTables_analysis_window []float32
var OpusTables_tbands []int
var OpusTables_extra_bands []int
var OpusTables_dct_table []float32
var OpusTables_net *MLP

type TonalityAnalysisState struct {
	read_pos                int
	write_pos               int
	read_subframe           int
	pmusic                  []float32
	pspeech                 []float32
	music_confidence        float32
	speech_confidence       float32
	count                   int
	last_transition         int
	angle                   []float32
	d_angle                 []float32
	d2_angle                []float32
	inmem                   []int32
	mem_fill                int
	info                    []AnalysisInfo
	E                       [][]float32
	lowE                    []float32
	highE                   []float32
	prev_band_tonality      []float32
	music_prob              float32
	last_music              int
	music_confidence_count  int
	speech_confidence_count int
	Etracker                float32
	lowECount               float32
	mem                     []float32
	cmean                   []float32
	std                     []float32
	prev_tonality           float32
	analysis_offset         int
}

func (t *TonalityAnalysisState) Reset() {}

type AnalysisInfo struct {
	music_prob    float32
	tonality      float32
	activity      float32
	tonality_slope float32
	bandwidth     int
	noisiness     float32
	valid         int
	enabled       bool
}

func (info *AnalysisInfo) Assign(other AnalysisInfo) {
	info.music_prob = other.music_prob
	info.tonality = other.tonality
	info.activity = other.activity
	info.tonality_slope = other.tonality_slope
	info.bandwidth = other.bandwidth
	info.noisiness = other.noisiness
	info.valid = other.valid
	info.enabled = other.enabled
}

type CeltMode struct {
	mdct struct {
		kfft []*FFTState
	}
}

type FFTState struct{}

type MLP struct{}

func ABS16(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

func IMIN(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func IMAX(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func MIN16(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func MAX16(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func MIN32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func MAX32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func downmix_int(x []int16, x_ptr int, inmem []int32, inmem_offset int, len int, offset int, c1 int, c2 int, C int) {}

func opus_fft(kfft *FFTState, input []int32, output []int32) {}

func mlp_process(net *MLP, features []float32, probs []float32) {}

func fast_atan2f(y float32, x float32) float32 {
	if ABS16(x)+ABS16(y) < 1e-9 {
		x *= 1e12
		y *= 1e12
	}
	x2 := x * x
	y2 := y * y
	if x2 < y2 {
		den := (y2 + cB*x2) * (y2 + cC*x2)
		if den != 0 {
			term := -x * y * (y2 + cA*x2) / den
			if y < 0 {
				return term - cE
			}
			return term + cE
		} else {
			if y < 0 {
				return -cE
			}
			return cE
		}
	} else {
		den := (x2 + cB*y2) * (x2 + cC*y2)
		if den != 0 {
			term := x * y * (x2 + cA*y2) / den
			if y < 0 {
				term -= cE
			} else {
				term += cE
			}
			if x*y < 0 {
				term += cE
			} else {
				term -= cE
			}
			return term
		} else {
			var term float32
			if y < 0 {
				term = -cE
			} else {
				term = cE
			}
			if x*y < 0 {
				term += cE
			} else {
				term -= cE
			}
			return term
		}
	}
}

func tonality_analysis_init(tonal *TonalityAnalysisState) {
	tonal.Reset()
}

func tonality_get_info(tonal *TonalityAnalysisState, info_out *AnalysisInfo, len int) {
	pos := tonal.read_pos
	curr_lookahead := tonal.write_pos - tonal.read_pos
	if curr_lookahead < 0 {
		curr_lookahead += DETECT_SIZE
	}

	if len > 480 && pos != tonal.write_pos {
		pos++
		if pos == DETECT_SIZE {
			pos = 0
		}
	}
	if pos == tonal.write_pos {
		pos--
	}
	if pos < 0 {
		pos = DETECT_SIZE - 1
	}

	info_out.Assign(tonal.info[pos])
	tonal.read_subframe += len / 120
	for tonal.read_subframe >= 4 {
		tonal.read_subframe -= 4
		tonal.read_pos++
	}
	if tonal.read_pos >= DETECT_SIZE {
		tonal.read_pos -= DETECT_SIZE
	}

	curr_lookahead = IMAX(curr_lookahead-10, 0)

	psum := float32(0)
	for i := 0; i < DETECT_SIZE-curr_lookahead; i++ {
		psum += tonal.pmusic[i]
	}
	for i := DETECT_SIZE - curr_lookahead; i < DETECT_SIZE; i++ {
		psum += tonal.pspeech[i]
	}
	psum = psum*tonal.music_confidence + (1-psum)*tonal.speech_confidence
	info_out.music_prob = psum
}

func tonality_analysis(tonal *TonalityAnalysisState, celt_mode *CeltMode, x []int16, x_ptr int, len int, offset int, c1 int, c2 int, C int, lsb_depth int) {
	const N = 480
	const N2 = 240
	pi4 := float32(M_PI * M_PI * M_PI * M_PI)
	var kfft *FFTState
	input := make([]int32, 960)
	output := make([]int32, 960)
	tonality := make([]float32, 240)
	noisiness := make([]float32, 240)
	band_tonality := make([]float32, NB_TBANDS)
	logE := make([]float32, NB_TBANDS)
	BFCC := make([]float32, 8)
	features := make([]float32, 25)
	frame_tonality := float32(0)
	max_frame_tonality := float32(0)
	frame_noisiness := float32(0)
	slope := float32(0)
	frame_stationarity := float32(0)
	relativeE := float32(0)
	frame_probs := make([]float32, 2)
	frame_loudness := float32(0)
	bandwidth_mask := float32(0)
	bandwidth := 0
	maxE := float32(0)
	noise_floor := float32(0)
	var info *AnalysisInfo

	tonal.last_transition++
	alpha := 1.0 / float32(IMIN(20, 1+tonal.count))
	alphaE := 1.0 / float32(IMIN(50, 1+tonal.count))
	alphaE2 := 1.0 / float32(IMIN(1000, 1+tonal.count))

	if tonal.count < 4 {
		tonal.music_prob = 0.5
	}
	kfft = celt_mode.mdct.kfft[0]
	if tonal.count == 0 {
		tonal.mem_fill = 240
	}

	downmix_int(x, x_ptr, tonal.inmem, tonal.mem_fill, IMIN(len, ANALYSIS_BUF_SIZE-tonal.mem_fill), offset, c1, c2, C)

	if tonal.mem_fill+len < ANALYSIS_BUF_SIZE {
		tonal.mem_fill += len
		return
	}

	info = &tonal.info[tonal.write_pos]
	tonal.write_pos++
	if tonal.write_pos >= DETECT_SIZE {
		tonal.write_pos -= DETECT_SIZE
	}

	for i := 0; i < N2; i++ {
		w := OpusTables_analysis_window[i]
		input[2*i] = int32(w * float32(tonal.inmem[i]))
		input[2*i+1] = int32(w * float32(tonal.inmem[N2+i]))
		input[2*(N-i-1)] = int32(w * float32(tonal.inmem[N-i-1]))
		input[2*(N-i-1)+1] = int32(w * float32(tonal.inmem[N+N2-i-1]))
	}
	copy(tonal.inmem, tonal.inmem[ANALYSIS_BUF_SIZE-240:ANALYSIS_BUF_SIZE])

	remaining := len - (ANALYSIS_BUF_SIZE - tonal.mem_fill)
	downmix_int(x, x_ptr, tonal.inmem, 240, remaining, offset+ANALYSIS_BUF_SIZE-tonal.mem_fill, c1, c2, C)
	tonal.mem_fill = 240 + remaining

	opus_fft(kfft, input, output)

	for i := 1; i < N2; i++ {
		X1r := float32(output[2*i] + output[2*(N-i)])
		X1i := float32(output[2*i+1] - output[2*(N-i)+1])
		X2r := float32(output[2*i+1] + output[2*(N-i)+1])
		X2i := float32(output[2*(N-i)] - output[2*i])

		angle := 0.5 / float32(M_PI) * fast_atan2f(X1i, X1r)
		d_angle := angle - tonal.angle[i]
		d2_angle := d_angle - tonal.d_angle[i]

		angle2 := 0.5 / float32(M_PI) * fast_atan2f(X2i, X2r)
		d_angle2 := angle2 - angle
		d2_angle2 := d_angle2 - d_angle

		mod1 := d2_angle - float32(math.Floor(float64(0.5+d2_angle))
		noisiness[i] = ABS16(mod1)
		mod1 *= mod1
		mod1 *= mod1

		mod2 := d2_angle2 - float32(math.Floor(float64(0.5+d2_angle2))
		noisiness[i] += ABS16(mod2)
		mod2 *= mod2
		mod2 *= mod2

		avg_mod := 0.25 * (tonal.d2_angle[i] + 2.0*mod1 + mod2)
		tonality[i] = 1.0/(1.0+40.0*16.0*pi4*avg_mod) - 0.015

		tonal.angle[i] = angle2
		tonal.d_angle[i] = d_angle2
		tonal.d2_angle[i] = mod2
	}

	frame_tonality = 0
	max_frame_tonality = 0
	info.activity = 0
	frame_noisiness = 0
	frame_stationarity = 0
	if tonal.count == 0 {
		for b := 0; b < NB_TBANDS; b++ {
			tonal.lowE[b] = 1e10
			tonal.highE[b] = -1e10
		}
	}
	relativeE = 0
	frame_loudness = 0
	for b := 0; b < NB_TBANDS; b++ {
		E := float32(0)
		tE := float32(0)
		nE := float32(0)
		var L1, L2 float32
		for i := OpusTables_tbands[b]; i < OpusTables_tbands[b+1]; i++ {
			binE := float32(output[2*i])*float32(output[2*i]) + float32(output[2*(N-i)])*float32(output[2*(N-i)]) +
				float32(output[2*i+1])*float32(output[2*i+1]) + float32(output[2*(N-i)+1])*float32(output[2*(N-i)+1])
			binE *= 5.55e-17
			E += binE
			tE += binE * tonality[i]
			nE += binE * 2.0 * (0.5 - noisiness[i])
		}

		tonal.E[tonal.E_count][b] = E
		frame_noisiness += nE / (1e-15 + E)

		frame_loudness += float32(math.Sqrt(float64(E + 1e-10)))
		logE[b] = float32(math.Log(float64(E + 1e-10)))
		tonal.lowE[b] = MIN32(logE[b], tonal.lowE[b]+0.01)
		tonal.highE[b] = MAX32(logE[b], tonal.highE[b]-0.1)
		if tonal.highE[b] < tonal.lowE[b]+1.0 {
			tonal.highE[b] += 0.5
			tonal.lowE[b] -= 0.5
		}
		relativeE += (logE[b] - tonal.lowE[b]) / (1e-15 + tonal.highE[b] - tonal.lowE[b])

		L1 = 0
		L2 = 0
		for i := 0; i < NB_FRAMES; i++ {
			L1 += float32(math.Sqrt(float64(tonal.E[i][b])))
			L2 += tonal.E[i][b]
		}

		stationarity := MIN16(0.99, L1/float32(math.Sqrt(1e-15+float64(NB_FRAMES)*float64(L2))))
		stationarity *= stationarity
		stationarity *= stationarity
		frame_stationarity += stationarity
		band_tonality[b] = MAX16(tE/(1e-15+E), stationarity*tonal.prev_band_tonality[b])
		frame_tonality += band_tonality[b]
		if b >= NB_TBANDS-NB_TONAL_SKIP_BANDS {
			frame_tonality -= band_tonality[b-NB_TBANDS+NB_TONAL_SKIP_BANDS]
		}
		max_frame_tonality = MAX16(max_frame_tonality, (1.0+0.03*float32(b-NB_TBANDS))*frame_tonality)
		slope += band_tonality[b] * float32(b-8)
		tonal.prev_band_tonality[b] = band_tonality[b]
	}

	bandwidth_mask = 0
	bandwidth = 0
	maxE = 0
	noise_floor = 5.7e-4 / float32(1<<uint(IMAX(0, lsb_depth-8)))
	noise_floor *= 1 << (15 + SIG_SHIFT)
	noise_floor *= noise_floor
	for b := 0; b < NB_TOT_BANDS; b++ {
		E := float32(0)
		band_start := OpusTables_extra_bands[b]
		band_end := OpusTables_extra_bands[b+1]
		for i := band_start; i < band_end; i++ {
			binE := float32(output[2*i])*float32(output[2*i]) + float32(output[2*(N-i)])*float32(output[2*(N-i)]) +
				float32(output[2*i+1])*float32(output[2*i+1]) + float32(output[2*(N-i)+1])*float32(output[2*(N-i)+1])
			E += binE
		}
		maxE = MAX32(maxE, E)
		tonal.meanE[b] = MAX32((1-alphaE2)*tonal.meanE[b], E)
		E = MAX32(E, tonal.meanE[b])
		bandwidth_mask = MAX32(0.05*bandwidth_mask, E)
		if E > 0.1*bandwidth_mask && E*1e9 > maxE && E > noise_floor*float32(band_end-band_start) {
			bandwidth = b
		}
	}
	if tonal.count <= 2 {
		bandwidth = 20
	}
	frame_loudness = 20 * float32(math.Log10(float64(frame_loudness)))
	tonal.Etracker = MAX32(tonal.Etracker-0.03, frame_loudness)
	tonal.lowECount *= (1 - alphaE)
	if frame_loudness < tonal.Etracker-30 {
		tonal.lowECount += alphaE
	}

	for i := 0; i < 8; i++ {
		sum := float32(0)
		for b := 0; b < 16; b++ {
			sum += OpusTables_dct_table[i*16+b] * logE[b]
		}
		BFCC[i] = sum
	}

	frame_stationarity /= NB_TBANDS
	relativeE /= NB_TBANDS
	if tonal.count < 10 {
		relativeE = 0.5
	}
	frame_noisiness /= NB_TBANDS
	info.activity = frame_noisiness + (1-frame_noisiness)*relativeE
	frame_tonality = max_frame_tonality / float32(NB_TBANDS-NB_TONAL_SKIP_BANDS)
	frame_tonality = MAX16(frame_tonality, tonal.prev_tonality*0.8)
	tonal.prev_tonality = frame_tonality

	slope /= 8 * 8
	info.tonality_slope = slope

	tonal.E_count = (tonal.E_count + 1) % NB_FRAMES
	tonal.count++
	info.tonality = frame_tonality

	for i := 0; i < 4; i++ {
		features[i] = -0.12299*(BFCC[i]+tonal.mem[i+24]) + 0.49195*(tonal.mem[i]+tonal.mem[i+16]) + 0.69693*tonal.mem[i+8] - 1.4349*tonal.cmean[i]
	}

	for i := 0; i < 4; i++ {
		tonal.cmean[i] = (1-alpha)*tonal.cmean[i] + alpha*BFCC[i]
	}

	for i := 0; i < 4; i++ {
		features[4+i] = 0.63246*(BFCC[i]-tonal.mem[i+24]) + 0.31623*(tonal.mem[i]-tonal.mem[i+16])
	}
	for i := 0; i < 3; i++ {
		features[8+i] = 0.53452*(BFCC[i]+tonal.mem[i+24]) - 0.26726*(tonal.mem[i]+tonal.mem[i+16]) - 0.53452*tonal.mem[i+8]
	}

	if tonal.count > 5 {
		for i := 0; i < 9; i++ {
			tonal.std[i] = (1-alpha)*tonal.std[i] + alpha*features[i]*features[i]
		}
	}

	for i := 0; i < 8; i++ {
		tonal.mem[i+24] = tonal.mem[i+16]
		tonal.mem[i+16] = tonal.mem[i+8]
		tonal.mem[i+8] = tonal.mem[i]
		tonal.mem[i] = BFCC[i]
	}
	for i := 0; i < 9; i++ {
		features[11+i] = float32(math.Sqrt(float64(tonal.std[i])))
	}
	features[20] = info.tonality
	features[21] = info.activity
	features[22] = frame_stationarity
	features[23] = info.tonality_slope
	features[24] = tonal.lowECount

	if info.enabled {
		mlp_process(OpusTables_net, features, frame_probs)
		frame_probs[0] = 0.5 * (frame_probs[0] + 1)
		frame_probs[0] = 0.01 + 1.21*frame_probs[0]*frame_probs[0] - 0.23*float32(math.Pow(float64(frame_probs[0]), 10))
		frame_probs[1] = 0.5*frame_probs[1] + 0.5
		frame_probs[0] = frame_probs[1]*frame_probs[0] + (1-frame_probs[1])*0.5

		{
			tau := 0.00005 * frame_probs[1]
			beta := float32(0.05)
			{
				p := MAX16(0.05, MIN16(0.95, frame_probs[0]))
				q := MAX16(0.05, MIN16(0.95, tonal.music_prob))
				beta = 0.01 + 0.05*ABS16(p-q)/(p*(1-q)+q*(1-p))
			}
			p0 := (1-tonal.music_prob)*(1-tau) + tonal.music_prob*tau
			p1 := tonal.music_prob*(1-tau) + (1-tonal.music_prob)*tau
			p0 *= float32(math.Pow(float64(1-frame_probs[0]), float64(beta)))
			p1 *= float32(math.Pow(float64(frame_probs[0]), float64(beta)))
			tonal.music_prob = p1 / (p0 + p1)
			info.music_prob = tonal.music_prob

			psum := float32(1e-20)
			speech0 := float32(math.Pow(float64(1-frame_probs[0]), float64(beta)))
			music0 := float32(math.Pow(float64(frame_probs[0]), float64(beta)))
			if tonal.count == 1 {
				tonal.pspeech[0] = 0.5
				tonal.pmusic[0] = 0.5
			}
			s0 := tonal.pspeech[0] + tonal.pspeech[1]
			m0 := tonal.pmusic[0] + tonal.pmusic[1]
			tonal.pspeech[0] = s0 * (1 - tau) * speech0
			tonal.pmusic[0] = m0 * (1 - tau) * music0
			for i := 1; i < DETECT_SIZE-1; i++ {
				tonal.pspeech[i] = tonal.pspeech[i+1] * speech0
				tonal.pmusic[i] = tonal.pmusic[i+1] * music0
			}
			tonal.pspeech[DETECT_SIZE-1] = m0 * tau * speech0
			tonal.pmusic[DETECT_SIZE-1] = s0 * tau * music0

			for i := 0; i < DETECT_SIZE; i++ {
				psum += tonal.pspeech[i] + tonal.pmusic[i]
			}
			psum = 1.0 / psum
			for i := 0; i < DETECT_SIZE; i++ {
				tonal.pspeech[i] *= psum
				tonal.pmusic[i] *= psum
			}
			psum = tonal.pmusic[0]
			for i := 1; i < DETECT_SIZE; i++ {
				psum += tonal.pspeech[i]
			}

			if frame_probs[1] > 0.75 {
				if tonal.music_prob > 0.9 {
					adapt := 1.0 / float32(tonal.music_confidence_count+1)
					tonal.music_confidence_count = IMIN(tonal.music_confidence_count, 500)
					tonal.music_confidence += adapt * MAX16(-0.2, frame_probs[0]-tonal.music_confidence)
				}
				if tonal.music_prob < 0.1 {
					adapt := 1.0 / float32(tonal.speech_confidence_count+1)
					tonal.speech_confidence_count = IMIN(tonal.speech_confidence_count, 500)
					tonal.speech_confidence += adapt * MIN16(0.2, frame_probs[0]-tonal.speech_confidence)
				}
			} else {
				if tonal.music_confidence_count == 0 {
					tonal.music_confidence = 0.9
				}
				if tonal.speech_confidence_count == 0 {
					tonal.speech_confidence = 0.1
				}
			}
		}
		if tonal.last_music != 0 {
			if tonal.music_prob > 0.5 {
				tonal.last_music = 1
			} else {
				tonal.last_music = 0
			}
		} else {
			tonal.last_transition = 0
		}
		if tonal.music_prob > 0.5 {
			tonal.last_music = 1
		} else {
			tonal.last_music = 0
		}
	} else {
		info.music_prob = 0
	}

	info.bandwidth = bandwidth
	info.noisiness = frame_noisiness
	info.valid = 1
}

func run_analysis(analysis *TonalityAnalysisState, celt_mode *CeltMode, analysis_pcm []int16, analysis_pcm_ptr int, analysis_frame_size int, frame_size int, c1 int, c2 int, C int, Fs int, lsb_depth int, analysis_info *AnalysisInfo) {
	offset := 0
	pcm_len := 0

	if analysis_pcm != nil {
		analysis_frame_size = IMIN((DETECT_SIZE-5)*Fs/100, analysis_frame_size)

		pcm_len = analysis_frame_size - analysis.analysis_offset
		offset = analysis.analysis_offset
		for pcm_len > 0 {
			chunk := IMIN(480, pcm_len)
			tonality_analysis(analysis, celt_mode, analysis_pcm, analysis_pcm_ptr, chunk, offset, c1, c2, C, lsb_depth)
			offset += 480
			pcm_len -= 480
		}
		analysis.analysis_offset = analysis_frame_size
		analysis.analysis_offset -= frame_size
	}

	analysis_info.valid = 0
	tonality_get_info(analysis, analysis_info, frame_size)
}