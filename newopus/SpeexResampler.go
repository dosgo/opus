package opus

import (
	"math"
)

type FuncDef struct {
	table      []float64
	oversample int
}

type QualityMapping struct {
	base_length          int
	oversample           int
	downsample_bandwidth float32
	upsample_bandwidth   float32
	window_func          *FuncDef
}

var kaiser12_table = []float64{
	0.99859849, 1.00000000, 0.99859849, 0.99440475, 0.98745105, 0.97779076,
	0.96549770, 0.95066529, 0.93340547, 0.91384741, 0.89213598, 0.86843014,
	0.84290116, 0.81573067, 0.78710866, 0.75723148, 0.72629970, 0.69451601,
	0.66208321, 0.62920216, 0.59606986, 0.56287762, 0.52980938, 0.49704014,
	0.46473455, 0.43304576, 0.40211431, 0.37206735, 0.34301800, 0.31506490,
	0.28829195, 0.26276832, 0.23854851, 0.21567274, 0.19416736, 0.17404546,
	0.15530766, 0.13794294, 0.12192957, 0.10723616, 0.09382272, 0.08164178,
	0.07063950, 0.06075685, 0.05193064, 0.04409466, 0.03718069, 0.03111947,
	0.02584161, 0.02127838, 0.01736250, 0.01402878, 0.01121463, 0.00886058,
	0.00691064, 0.00531256, 0.00401805, 0.00298291, 0.00216702, 0.00153438,
	0.00105297, 0.00069463, 0.00043489, 0.00025272, 0.00013031, 0.0000527734,
	0.00001000, 0.00000000,
}

var kaiser10_table = []float64{
	0.99537781, 1.00000000, 0.99537781, 0.98162644, 0.95908712, 0.92831446,
	0.89005583, 0.84522401, 0.79486424, 0.74011713, 0.68217934, 0.62226347,
	0.56155915, 0.50119680, 0.44221549, 0.38553619, 0.33194107, 0.28205962,
	0.23636152, 0.19515633, 0.15859932, 0.12670280, 0.09935205, 0.07632451,
	0.05731132, 0.04193980, 0.02979584, 0.02044510, 0.01345224, 0.00839739,
	0.00488951, 0.00257636, 0.00115101, 0.00035515, 0.00000000, 0.00000000,
}

var kaiser8_table = []float64{
	0.99635258, 1.00000000, 0.99635258, 0.98548012, 0.96759014, 0.94302200,
	0.91223751, 0.87580811, 0.83439927, 0.78875245, 0.73966538, 0.68797126,
	0.63451750, 0.58014482, 0.52566725, 0.47185369, 0.41941150, 0.36897272,
	0.32108304, 0.27619388, 0.23465776, 0.19672670, 0.16255380, 0.13219758,
	0.10562887, 0.08273982, 0.06335451, 0.04724088, 0.03412321, 0.02369490,
	0.01563093, 0.00959968, 0.00527363, 0.00233883, 0.00050000, 0.00000000,
}

var kaiser6_table = []float64{
	0.99733006, 1.00000000, 0.99733006, 0.98935595, 0.97618418, 0.95799003,
	0.93501423, 0.90755855, 0.87598009, 0.84068475, 0.80211977, 0.76076565,
	0.71712752, 0.67172623, 0.62508937, 0.57774224, 0.53019925, 0.48295561,
	0.43647969, 0.39120616, 0.34752997, 0.30580127, 0.26632152, 0.22934058,
	0.19505503, 0.16360756, 0.13508755, 0.10953262, 0.08693120, 0.06722600,
	0.05031820, 0.03607231, 0.02432151, 0.01487334, 0.00752000, 0.00000000,
}

var quality_map = []*QualityMapping{
	{8, 4, 0.830, 0.860, &FuncDef{kaiser6_table, 32}},
	{16, 4, 0.850, 0.880, &FuncDef{kaiser6_table, 32}},
	{32, 4, 0.882, 0.910, &FuncDef{kaiser6_table, 32}},
	{48, 8, 0.895, 0.917, &FuncDef{kaiser8_table, 32}},
	{64, 8, 0.921, 0.940, &FuncDef{kaiser8_table, 32}},
	{80, 16, 0.922, 0.940, &FuncDef{kaiser10_table, 32}},
	{96, 16, 0.940, 0.945, &FuncDef{kaiser10_table, 32}},
	{128, 16, 0.950, 0.950, &FuncDef{kaiser10_table, 32}},
	{160, 16, 0.960, 0.960, &FuncDef{kaiser10_table, 32}},
	{192, 32, 0.968, 0.968, &FuncDef{kaiser12_table, 64}},
	{256, 32, 0.975, 0.975, &FuncDef{kaiser12_table, 64}},
}

const FIXED_STACK_ALLOC = 8192

type SpeexResampler struct {
	in_rate           int
	out_rate          int
	num_rate          int
	den_rate          int
	quality           int
	nb_channels       int
	filt_len          int
	mem_alloc_size    int
	buffer_size       int
	int_advance       int
	frac_advance      int
	cutoff            float32
	oversample        int
	initialised       int
	started           int
	last_sample       []int
	samp_frac_num     []int
	magic_samples     []int
	mem               []int16
	sinc_table        []int16
	sinc_table_length int
	in_stride         int
	out_stride        int
	resampler_ptr     func(*SpeexResampler, int, []int16, int, *BoxedValueInt, []int16, int, *BoxedValueInt) int
}

func WORD2INT(x float32) int16 {
	if x < -32768 {
		return -32768
	} else if x > 32767 {
		return 32767
	}
	return int16(x)
}

func compute_func(x float32, func_def *FuncDef) float64 {
	y := float64(x) * float64(func_def.oversample)
	ind := int(math.Floor(y))
	frac := y - float64(ind)
	interp3 := -0.1666666667*frac + 0.1666666667*frac*frac*frac
	interp2 := frac + 0.5*frac*frac - 0.5*frac*frac*frac
	interp0 := -0.3333333333*frac + 0.5*frac*frac - 0.1666666667*frac*frac*frac
	interp1 := 1.0 - interp3 - interp2 - interp0
	return interp0*func_def.table[ind] + interp1*func_def.table[ind+1] + interp2*func_def.table[ind+2] + interp3*func_def.table[ind+3]
}

func sinc(cutoff, x float32, N int, window_func *FuncDef) int16 {
	if x < 1e-6 && x > -1e-6 {
		return WORD2INT(32768.0 * cutoff)
	} else if x > 0.5*float32(N) || x < -0.5*float32(N) {
		return 0
	}
	xx := x * cutoff
	sinc_val := float32(math.Sin(math.Pi*float64(xx))) / (math.Pi * float32(xx))
	window_val := float32(compute_func(float32(abs(float64(2*x))/float32(N), window_func)))
	return WORD2INT(32768.0 * cutoff * sinc_val * window_val)
}

func cubic_coef(x int16, interp0, interp1, interp2, interp3 *BoxedValueShort) {
	x2 := MULT16_16_P15(x, x)
	x3 := MULT16_16_P15(x, x2)
	interp0.Val = PSHR32(int(MULT16_16(QCONST16(-0.16667, 15), x)+int(MULT16_16(QCONST16(0.16667, 15), x3)), 15))
	interp1.Val = EXTRACT16(EXTEND32(x) + SHR32(SUB32(EXTEND32(x2), EXTEND32(x3)), 1))
	interp3.Val = PSHR32(int(MULT16_16(QCONST16(-0.33333, 15), x))+int(MULT16_16(QCONST16(0.5, 15), x2))-int(MULT16_16(QCONST16(0.16667, 15), x3)), 15)
	interp2.Val = int16(32767 - int(interp0.Val) - int(interp1.Val) - int(interp3.Val))
	if interp2.Val < 32767 {
		interp2.Val += 1
	}
}

func (st *SpeexResampler) resampler_basic_direct_single(channel_index int, input []int16, input_ptr int, in_len *BoxedValueInt, output []int16, output_ptr int, out_len *BoxedValueInt) int {
	N := st.filt_len
	out_sample := 0
	last_sample := st.last_sample[channel_index]
	samp_frac_num := st.samp_frac_num[channel_index]

	for !(last_sample >= in_len.Val || out_sample >= out_len.Val) {
		sinct := samp_frac_num * N
		iptr := input_ptr + last_sample

		sum := 0
		for j := 0; j < N; j++ {
			sum += int(MULT16_16(st.sinc_table[sinct+j], input[iptr+j]))
		}

		output[output_ptr+st.out_stride*out_sample] = SATURATE16(PSHR32(int(sum), 15))
		out_sample++
		last_sample += st.int_advance
		samp_frac_num += st.frac_advance
		if samp_frac_num >= st.den_rate {
			samp_frac_num -= st.den_rate
			last_sample++
		}
	}

	st.last_sample[channel_index] = last_sample
	st.samp_frac_num[channel_index] = samp_frac_num
	return out_sample
}

func (st *SpeexResampler) resampler_basic_interpolate_single(channel_index int, input []int16, input_ptr int, in_len *BoxedValueInt, output []int16, output_ptr int, out_len *BoxedValueInt) int {
	N := st.filt_len
	out_sample := 0
	last_sample := st.last_sample[channel_index]
	samp_frac_num := st.samp_frac_num[channel_index]
	interp0 := &BoxedValueShort{0}
	interp1 := &BoxedValueShort{0}
	interp2 := &BoxedValueShort{0}
	interp3 := &BoxedValueShort{0}

	for !(last_sample >= in_len.Val || out_sample >= out_len.Val) {
		iptr := input_ptr + last_sample
		offset := samp_frac_num * st.oversample / st.den_rate
		frac := int16(PDIV32(SHL32(int((samp_frac_num*st.oversample)%st.den_rate), 15), int(st.den_rate)))

		accum0 := 0
		accum1 := 0
		accum2 := 0
		accum3 := 0
		for j := 0; j < N; j++ {
			curr_in := input[iptr+j]
			accum0 += int(MULT16_16(curr_in, st.sinc_table[4+(j+1)*st.oversample-offset-2]))
			accum1 += int(MULT16_16(curr_in, st.sinc_table[4+(j+1)*st.oversample-offset-1]))
			accum2 += int(MULT16_16(curr_in, st.sinc_table[4+(j+1)*st.oversample-offset]))
			accum3 += int(MULT16_16(curr_in, st.sinc_table[4+(j+1)*st.oversample-offset+1]))
		}

		cubic_coef(frac, interp0, interp1, interp2, interp3)
		sum := int(MULT16_32_Q15(interp0.Val, int(accum0)>>1)) +
			int(MULT16_32_Q15(interp1.Val, int(accum1)>>1)) +
			int(MULT16_32_Q15(interp2.Val, int(accum2)>>1)) +
			int(MULT16_32_Q15(interp3.Val, int(accum3)>>1))

		output[output_ptr+st.out_stride*out_sample] = SATURATE16(PSHR32(int(sum), 14))
		out_sample++
		last_sample += st.int_advance
		samp_frac_num += st.frac_advance
		if samp_frac_num >= st.den_rate {
			samp_frac_num -= st.den_rate
			last_sample++
		}
	}

	st.last_sample[channel_index] = last_sample
	st.samp_frac_num[channel_index] = samp_frac_num
	return out_sample
}

func (st *SpeexResampler) update_filter() {
	old_length := st.filt_len
	st.oversample = quality_map[st.quality].oversample
	st.filt_len = quality_map[st.quality].base_length

	if st.num_rate > st.den_rate {
		st.cutoff = quality_map[st.quality].downsample_bandwidth * float32(st.den_rate) / float32(st.num_rate)
		st.filt_len = st.filt_len * st.num_rate / st.den_rate
		st.filt_len = ((st.filt_len - 1) &^ 0x7) + 8
		if 2*st.den_rate < st.num_rate {
			st.oversample >>= 1
		}
		if 4*st.den_rate < st.num_rate {
			st.oversample >>= 1
		}
		if 8*st.den_rate < st.num_rate {
			st.oversample >>= 1
		}
		if 16*st.den_rate < st.num_rate {
			st.oversample >>= 1
		}
		if st.oversample < 1 {
			st.oversample = 1
		}
	} else {
		st.cutoff = quality_map[st.quality].upsample_bandwidth
	}

	if st.den_rate <= 16*(st.oversample+8) {
		if st.sinc_table == nil || st.sinc_table_length < st.filt_len*st.den_rate {
			st.sinc_table = make([]int16, st.filt_len*st.den_rate)
			st.sinc_table_length = st.filt_len * st.den_rate
		}
		for i := 0; i < st.den_rate; i++ {
			for j := 0; j < st.filt_len; j++ {
				idx := j - st.filt_len/2 + 1
				st.sinc_table[i*st.filt_len+j] = sinc(st.cutoff, float32(idx)-float32(i)/float32(st.den_rate), st.filt_len, quality_map[st.quality].window_func)
			}
		}
		st.resampler_ptr = (*SpeexResampler).resampler_basic_direct_single
	} else {
		if st.sinc_table == nil || st.sinc_table_length < st.filt_len*st.oversample+8 {
			st.sinc_table = make([]int16, st.filt_len*st.oversample+8)
			st.sinc_table_length = st.filt_len*st.oversample + 8
		}
		for i := -4; i < st.oversample*st.filt_len+4; i++ {
			st.sinc_table[i+4] = sinc(st.cutoff, float32(i)/float32(st.oversample)-float32(st.filt_len)/2, st.filt_len, quality_map[st.quality].window_func)
		}
		st.resampler_ptr = (*SpeexResampler).resampler_basic_interpolate_single
	}

	st.int_advance = st.num_rate / st.den_rate
	st.frac_advance = st.num_rate % st.den_rate

	if st.mem == nil {
		st.mem_alloc_size = st.filt_len - 1 + st.buffer_size
		st.mem = make([]int16, st.nb_channels*st.mem_alloc_size)
	} else if st.started == 0 {
		st.mem_alloc_size = st.filt_len - 1 + st.buffer_size
		st.mem = make([]int16, st.nb_channels*st.mem_alloc_size)
	} else if st.filt_len > old_length {
		old_alloc_size := st.mem_alloc_size
		if st.filt_len-1+st.buffer_size > st.mem_alloc_size {
			st.mem_alloc_size = st.filt_len - 1 + st.buffer_size
			st.mem = make([]int16, st.nb_channels*st.mem_alloc_size)
		}
		for i := st.nb_channels - 1; i >= 0; i-- {
			olen := old_length
			if st.magic_samples[i] != 0 {
				olen = old_length + 2*st.magic_samples[i]
				for j := old_length - 2 + st.magic_samples[i]; j >= 0; j-- {
					st.mem[i*st.mem_alloc_size+j+st.magic_samples[i]] = st.mem[i*old_alloc_size+j]
				}
				for j := 0; j < st.magic_samples[i]; j++ {
					st.mem[i*st.mem_alloc_size+j] = 0
				}
				st.magic_samples[i] = 0
			}
			if st.filt_len > olen {
				for j := 0; j < olen-1; j++ {
					st.mem[i*st.mem_alloc_size+(st.filt_len-2-j)] = st.mem[i*st.mem_alloc_size+(olen-2-j)]
				}
				for j := olen - 1; j < st.filt_len-1; j++ {
					st.mem[i*st.mem_alloc_size+(st.filt_len-2-j)] = 0
				}
				st.last_sample[i] += (st.filt_len - olen) / 2
			} else {
				st.magic_samples[i] = (olen - st.filt_len) / 2
				for j := 0; j < st.filt_len-1+st.magic_samples[i]; j++ {
					st.mem[i*st.mem_alloc_size+j] = st.mem[i*st.mem_alloc_size+j+st.magic_samples[i]]
				}
			}
		}
	} else if st.filt_len < old_length {
		for i := 0; i < st.nb_channels; i++ {
			old_magic := st.magic_samples[i]
			st.magic_samples[i] = (old_length - st.filt_len) / 2
			for j := 0; j < st.filt_len-1+st.magic_samples[i]+old_magic; j++ {
				st.mem[i*st.mem_alloc_size+j] = st.mem[i*st.mem_alloc_size+j+st.magic_samples[i]]
			}
			st.magic_samples[i] += old_magic
		}
	}
}

func (st *SpeexResampler) speex_resampler_process_native(channel_index int, in_len *BoxedValueInt, output []int16, output_ptr int, out_len *BoxedValueInt) {
	mem_ptr := channel_index * st.mem_alloc_size
	st.started = 1

	out_sample := st.resampler_ptr(st, channel_index, st.mem, mem_ptr, in_len, output, output_ptr, out_len)

	if st.last_sample[channel_index] < in_len.Val {
		in_len.Val = st.last_sample[channel_index]
	}
	out_len.Val = out_sample
	st.last_sample[channel_index] -= in_len.Val

	ilen := in_len.Val
	N := st.filt_len
	for j := mem_ptr; j < N-1+mem_ptr; j++ {
		st.mem[j] = st.mem[j+ilen]
	}
}

func (st *SpeexResampler) speex_resampler_magic(channel_index int, output []int16, output_ptr *int, out_len int) int {
	tmp_in_len := st.magic_samples[channel_index]
	olen := out_len
	boxed_tmp_in_len := &BoxedValueInt{Val: tmp_in_len}
	boxed_olen := &BoxedValueInt{Val: olen}

	st.speex_resampler_process_native(channel_index, boxed_tmp_in_len, output, *output_ptr, boxed_olen)

	tmp_in_len = boxed_tmp_in_len.Val
	olen = boxed_olen.Val
	st.magic_samples[channel_index] -= tmp_in_len

	if st.magic_samples[channel_index] != 0 {
		mem_ptr := channel_index * st.mem_alloc_size
		N := st.filt_len
		for i := mem_ptr; i < st.magic_samples[channel_index]+mem_ptr; i++ {
			st.mem[N-1+i] = st.mem[N-1+i+tmp_in_len]
		}
	}
	*output_ptr += olen * st.out_stride
	return olen
}

func Create(nb_channels, in_rate, out_rate, quality int) *SpeexResampler {
	return CreateFractional(nb_channels, in_rate, out_rate, in_rate, out_rate, quality)
}

func CreateFractional(nb_channels, ratio_num, ratio_den, in_rate, out_rate, quality int) *SpeexResampler {
	if quality > 10 || quality < 0 {
		panic("Quality must be between 0 and 10")
	}
	st := &SpeexResampler{}
	st.initialised = 0
	st.started = 0
	st.in_rate = 0
	st.out_rate = 0
	st.num_rate = 0
	st.den_rate = 0
	st.quality = -1
	st.sinc_table_length = 0
	st.mem_alloc_size = 0
	st.filt_len = 0
	st.mem = nil
	st.cutoff = 1.0
	st.nb_channels = nb_channels
	st.in_stride = 1
	st.out_stride = 1
	st.buffer_size = 160

	st.last_sample = make([]int, nb_channels)
	st.magic_samples = make([]int, nb_channels)
	st.samp_frac_num = make([]int, nb_channels)
	for i := 0; i < nb_channels; i++ {
		st.last_sample[i] = 0
		st.magic_samples[i] = 0
		st.samp_frac_num[i] = 0
	}

	st.Quality = quality
	st.SetRateFraction(ratio_num, ratio_den, in_rate, out_rate)

	st.update_filter()
	st.initialised = 1
	return st
}

type ResamplerResult struct {
	inputSamplesProcessed  int
	outputSamplesGenerated int
}

func (st *SpeexResampler) Process(channel_index int, input []int16, input_ptr int, in_len int, output []int16, output_ptr int, out_len int) *ResamplerResult {
	ilen := in_len
	olen := out_len
	mem_ptr := channel_index * st.mem_alloc_size
	filt_offs := st.filt_len - 1
	xlen := st.mem_alloc_size - filt_offs
	istride := st.in_stride

	if st.magic_samples[channel_index] != 0 {
		olen -= st.speex_resampler_magic(channel_index, output, &output_ptr, olen)
	}
	if st.magic_samples[channel_index] == 0 {
		for ilen > 0 && olen > 0 {
			ichunk := ilen
			if ichunk > xlen {
				ichunk = xlen
			}
			ochunk := olen
			boxed_ichunk := &BoxedValueInt{Val: ichunk}
			boxed_ochunk := &BoxedValueInt{Val: ochunk}

			if input != nil {
				for j := 0; j < ichunk; j++ {
					st.mem[mem_ptr+j+filt_offs] = input[input_ptr+j*istride]
				}
			} else {
				for j := 0; j < ichunk; j++ {
					st.mem[mem_ptr+j+filt_offs] = 0
				}
			}
			st.speex_resampler_process_native(channel_index, boxed_ichunk, output, output_ptr, boxed_ochunk)
			ichunk = boxed_ichunk.Val
			ochunk = boxed_ochunk.Val
			ilen -= ichunk
			olen -= ochunk
			output_ptr += ochunk * st.out_stride
			if input != nil {
				input_ptr += ichunk * istride
			}
		}
	}
	return &ResamplerResult{
		inputSamplesProcessed:  in_len - ilen,
		outputSamplesGenerated: out_len - olen,
	}
}

func (st *SpeexResampler) ProcessInterleaved(input []int16, input_ptr int, in_len int, output []int16, output_ptr int, out_len int) *ResamplerResult {
	istride_save := st.in_stride
	ostride_save := st.out_stride
	st.in_stride = st.nb_channels
	st.out_stride = st.nb_channels
	bak_out_len := out_len
	bak_in_len := in_len
	for i := 0; i < st.nb_channels; i++ {
		out_len = bak_out_len
		in_len = bak_in_len
		var r *ResamplerResult
		if input != nil {
			r = st.Process(i, input, input_ptr+i, in_len, output, output_ptr+i, out_len)
		} else {
			r = st.Process(i, nil, 0, in_len, output, output_ptr+i, out_len)
		}
		in_len = r.inputSamplesProcessed
		out_len = r.outputSamplesGenerated
	}
	st.in_stride = istride_save
	st.out_stride = ostride_save
	return &ResamplerResult{
		inputSamplesProcessed:  in_len,
		outputSamplesGenerated: out_len,
	}
}

func (st *SpeexResampler) SkipZeroes() {
	for i := 0; i < st.nb_channels; i++ {
		st.last_sample[i] = st.filt_len / 2
	}
}

func (st *SpeexResampler) ResetMem() {
	for i := 0; i < st.nb_channels; i++ {
		st.last_sample[i] = 0
		st.magic_samples[i] = 0
		st.samp_frac_num[i] = 0
	}
	for i := 0; i < st.nb_channels*(st.filt_len-1); i++ {
		st.mem[i] = 0
	}
}

func (st *SpeexResampler) SetRates(in_rate, out_rate int) {
	st.SetRateFraction(in_rate, out_rate, in_rate, out_rate)
}

func (st *SpeexResampler) GetRates() (int, int) {
	return st.in_rate, st.out_rate
}

func (st *SpeexResampler) SetRateFraction(ratio_num, ratio_den, in_rate, out_rate int) {
	if st.in_rate == in_rate && st.out_rate == out_rate && st.num_rate == ratio_num && st.den_rate == ratio_den {
		return
	}

	old_den := st.den_rate
	st.in_rate = in_rate
	st.out_rate = out_rate
	st.num_rate = ratio_num
	st.den_rate = ratio_den

	for fact := 2; fact <= imin(st.num_rate, st.den_rate); fact++ {
		for st.num_rate%fact == 0 && st.den_rate%fact == 0 {
			st.num_rate /= fact
			st.den_rate /= fact
		}
	}

	if old_den > 0 {
		for i := 0; i < st.nb_channels; i++ {
			st.samp_frac_num[i] = st.samp_frac_num[i] * st.den_rate / old_den
			if st.samp_frac_num[i] >= st.den_rate {
				st.samp_frac_num[i] = st.den_rate - 1
			}
		}
	}

	if st.initialised != 0 {
		st.update_filter()
	}
}

func (st *SpeexResampler) GetRateFraction() (int, int) {
	return st.num_rate, st.den_rate
}

func (st *SpeexResampler) Quality() int {
	return st.quality
}

func (st *SpeexResampler) SetQuality(value int) {
	if value > 10 || value < 0 {
		panic("Quality must be between 0 and 10")
	}
	if st.quality == value {
		return
	}
	st.quality = value
	if st.initialised != 0 {
		st.update_filter()
	}
}

func (st *SpeexResampler) InputStride() int {
	return st.in_stride
}

func (st *SpeexResampler) SetInputStride(value int) {
	st.in_stride = value
}

func (st *SpeexResampler) OutputStride() int {
	return st.out_stride
}

func (st *SpeexResampler) SetOutputStride(value int) {
	st.out_stride = value
}

func (st *SpeexResampler) InputLatency() int {
	return st.filt_len / 2
}

func (st *SpeexResampler) OutputLatency() int {
	return ((st.filt_len/2)*st.den_rate + (st.num_rate >> 1)) / st.num_rate
}
