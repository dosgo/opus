package opus

type CeltDecoder struct {
	mode                  *CeltMode
	overlap               int
	channels              int
	stream_channels       int
	downsample            int
	start                 int
	end                   int
	signalling            int
	rng                   int
	error                 int
	last_pitch_index      int
	loss_count            int
	postfilter_period     int
	postfilter_period_old int
	postfilter_gain       int
	postfilter_gain_old   int
	postfilter_tapset     int
	postfilter_tapset_old int
	preemph_memD          [2]int
	decode_mem            [][]int
	lpc                   [][]int
	oldEBands             []int
	oldLogE               []int
	oldLogE2              []int
	backgroundLogE        []int
}

func (this *CeltDecoder) Reset() {
	this.mode = nil
	this.overlap = 0
	this.channels = 0
	this.stream_channels = 0
	this.downsample = 0
	this.start = 0
	this.end = 0
	this.signalling = 0
	this.PartialReset()
}

func (this *CeltDecoder) PartialReset() {
	this.rng = 0
	this.error = 0
	this.last_pitch_index = 0
	this.loss_count = 0
	this.postfilter_period = 0
	this.postfilter_period_old = 0
	this.postfilter_gain = 0
	this.postfilter_gain_old = 0
	this.postfilter_tapset = 0
	this.postfilter_tapset_old = 0
	this.preemph_memD = [2]int{}
	this.decode_mem = nil
	this.lpc = nil
	this.oldEBands = nil
	this.oldLogE = nil
	this.oldLogE2 = nil
	this.backgroundLogE = nil
}

func (this *CeltDecoder) ResetState() {
	this.PartialReset()

	if this.channels > 0 && this.mode != nil {
		this.decode_mem = make([][]int, this.channels)
		this.lpc = make([][]int, this.channels)
		for c := 0; c < this.channels; c++ {
			this.decode_mem[c] = make([]int, CeltConstants.DECODE_BUFFER_SIZE+this.mode.overlap)
			this.lpc[c] = make([]int, CeltConstants.LPC_ORDER)
		}
		nbEBands := this.mode.nbEBands
		this.oldEBands = make([]int, 2*nbEBands)
		this.oldLogE = make([]int, 2*nbEBands)
		this.oldLogE2 = make([]int, 2*nbEBands)
		this.backgroundLogE = make([]int, 2*nbEBands)

		q28 := QCONST16(28.0, DB_SHIFT)
		for i := 0; i < 2*nbEBands; i++ {
			this.oldLogE[i] = -q28
			this.oldLogE2[i] = -q28
		}
	}
}

func (this *CeltDecoder) celt_decoder_init(sampling_rate int, channels int) int {
	ret := this.opus_custom_decoder_init(mode48000_960_120, channels)
	if ret != OpusError.OPUS_OK {
		return ret
	}
	this.downsample = resampling_factor(sampling_rate)
	if this.downsample == 0 {
		return OpusError.OPUS_BAD_ARG
	}
	return OpusError.OPUS_OK
}

func (this *CeltDecoder) opus_custom_decoder_init(mode *CeltMode, channels int) int {
	if channels < 0 || channels > 2 {
		return OpusError.OPUS_BAD_ARG
	}
	if this == nil {
		return OpusError.OPUS_ALLOC_FAIL
	}
	this.Reset()
	this.mode = mode
	this.overlap = mode.overlap
	this.stream_channels = channels
	this.channels = channels
	this.downsample = 1
	this.start = 0
	this.end = this.mode.effEBands
	this.signalling = 1
	this.loss_count = 0
	this.ResetState()
	return OpusError.OPUS_OK
}

func (this *CeltDecoder) celt_decode_lost(N int, LM int) {
	C := this.channels
	out_syn := make([][]int, 2)
	out_syn_ptrs := make([]int, 2)
	mode := this.mode
	nbEBands := mode.nbEBands
	overlap := mode.overlap
	eBands := mode.eBands

	for c := 0; c < C; c++ {
		out_syn[c] = this.decode_mem[c]
		out_syn_ptrs[c] = CeltConstants.DECODE_BUFFER_SIZE - N
	}

	noise_based := 0
	if this.loss_count >= 5 || this.start != 0 {
		noise_based = 1
	}
	if noise_based != 0 {
		end := this.end
		effEnd := IMAX(this.start, IMIN(end, mode.effEBands))

		X := make([][]int, C)
		for c := range X {
			X[c] = make([]int, N)
		}

		decay := QCONST16(0.5, DB_SHIFT)
		if this.loss_count == 0 {
			decay = QCONST16(1.5, DB_SHIFT)
		}
		for c := 0; c < C; c++ {
			for i := this.start; i < end; i++ {
				idx := c*nbEBands + i
				this.oldEBands[idx] = MAX16(this.backgroundLogE[idx], this.oldEBands[idx]-decay)
			}
		}
		seed := this.rng
		for c := 0; c < C; c++ {
			for i := this.start; i < effEnd; i++ {
				boffs := eBands[i] << LM
				blen := (eBands[i+1] - eBands[i]) << LM
				for j := 0; j < blen; j++ {
					seed = celt_lcg_rand(seed)
					X[c][boffs+j] = int(seed) >> 20
				}
				renormalise_vector(X[c], boffs, blen, Q15ONE)
			}
		}
		this.rng = seed

		for c := 0; c < C; c++ {
			copy(this.decode_mem[c][:CeltConstants.DECODE_BUFFER_SIZE-N+(overlap>>1)], this.decode_mem[c][N:])
		}

		celt_synthesis(mode, X, out_syn, out_syn_ptrs, this.oldEBands, this.start, effEnd, C, C, 0, LM, this.downsample, 0)
	} else {
		fade := Q15ONE
		pitch_index := 0
		if this.loss_count == 0 {
			this.last_pitch_index = celt_plc_pitch_search(this.decode_mem, C)
			pitch_index = this.last_pitch_index
		} else {
			pitch_index = this.last_pitch_index
			fade = QCONST16(0.8, 15)
		}

		etmp := make([]int, overlap)
		exc := make([]int, CeltConstants.MAX_PERIOD)
		window := mode.window
		for c := 0; c < C; c++ {
			buf := this.decode_mem[c]
			for i := 0; i < CeltConstants.MAX_PERIOD; i++ {
				exc[i] = ROUND16(buf[CeltConstants.DECODE_BUFFER_SIZE-CeltConstants.MAX_PERIOD+i], CeltConstants.SIG_SHIFT)
			}

			if this.loss_count == 0 {
				ac := make([]int, CeltConstants.LPC_ORDER+1)
				_celt_autocorr(exc, ac, window, overlap, CeltConstants.LPC_ORDER, CeltConstants.MAX_PERIOD)
				ac[0] += SHR32(ac[0], 13)
				for i := 1; i <= CeltConstants.LPC_ORDER; i++ {
					ac[i] -= MULT16_32_Q15(2*i*i, ac[i])
				}
				celt_lpc(this.lpc[c], ac, LPC_ORDER)
			}

			exc_length := IMIN(2*pitch_index, CeltConstants.MAX_PERIOD)
			lpc_mem := make([]int, LPC_ORDER)
			for i := 0; i < LPC_ORDER; i++ {
				lpc_mem[i] = ROUND16(buf[CeltConstants.DECODE_BUFFER_SIZE-exc_length-1-i], SIG_SHIFT)
			}
			celt_fir(exc, MAX_PERIOD-exc_length, this.lpc[c], 0, exc, MAX_PERIOD-exc_length, exc_length, LPC_ORDER, lpc_mem)

			shift := IMAX(0, 2*celt_zlog2(celt_maxabs16(exc, MAX_PERIOD-exc_length, exc_length))-20)
			decay_length := exc_length >> 1
			E1 := 1
			E2 := 1
			for i := 0; i < decay_length; i++ {
				e := exc[CeltConstants.MAX_PERIOD-decay_length+i]
				E1 += SHR32(MULT16_16(e, e), shift)
				e = exc[CeltConstants.MAX_PERIOD-2*decay_length+i]
				E2 += SHR32(MULT16_16(e, e), shift)
			}
			E1 = MIN32(E1, E2)
			decay := celt_sqrt(frac_div32(SHR32(E1, 1), E2))

			copy(buf[:CeltConstants.DECODE_BUFFER_SIZE-N], buf[N:])

			extrapolation_offset := CeltConstants.MAX_PERIOD - pitch_index
			extrapolation_len := N + overlap
			attenuation := MULT16_16_Q15(fade, decay)
			S1 := 0
			j := 0
			for i := 0; i < extrapolation_len; i++ {
				if j >= pitch_index {
					j -= pitch_index
					attenuation = MULT16_16_Q15(attenuation, decay)
				}
				val := MULT16_16_Q15(attenuation, exc[extrapolation_offset+j])
				buf[CeltConstants.DECODE_BUFFER_SIZE-N+i] = SHL32(val, SIG_SHIFT)
				tmp := ROUND16(buf[CeltConstants.DECODE_BUFFER_SIZE-CeltConstants.MAX_PERIOD-N+extrapolation_offset+j], SIG_SHIFT)
				S1 += SHR32(MULT16_16(tmp, tmp), 8)
				j++
			}

			lpc_mem = make([]int, LPC_ORDER)
			for i := 0; i < LPC_ORDER; i++ {
				lpc_mem[i] = ROUND16(buf[CeltConstants.DECODE_BUFFER_SIZE-N-1-i], SIG_SHIFT)
			}
			celt_iir(buf, CeltConstants.DECODE_BUFFER_SIZE-N, this.lpc[c], buf, CeltConstants.DECODE_BUFFER_SIZE-N, extrapolation_len, LPC_ORDER, lpc_mem)

			S2 := 0
			for i := 0; i < extrapolation_len; i++ {
				tmp := ROUND16(buf[CeltConstants.DECODE_BUFFER_SIZE-N+i], SIG_SHIFT)
				S2 += SHR32(MULT16_16(tmp, tmp), 8)
			}
			if !(S1 > SHR32(S2, 2)) {
				for i := 0; i < extrapolation_len; i++ {
					buf[CeltConstants.DECODE_BUFFER_SIZE-N+i] = 0
				}
			} else if S1 < S2 {
				ratio := celt_sqrt(frac_div32(SHR32(S1, 1)+1, S2+1))
				for i := 0; i < overlap; i++ {
					tmp_g := Q15ONE - MULT16_16_Q15(window[i], Q15ONE-ratio)
					buf[CeltConstants.DECODE_BUFFER_SIZE-N+i] = MULT16_32_Q15(tmp_g, buf[CeltConstants.DECODE_BUFFER_SIZE-N+i])
				}
				for i := overlap; i < extrapolation_len; i++ {
					buf[CeltConstants.DECODE_BUFFER_SIZE-N+i] = MULT16_32_Q15(ratio, buf[CeltConstants.DECODE_BUFFER_SIZE-N+i])
				}
			}

			comb_filter(etmp, 0, buf, CeltConstants.DECODE_BUFFER_SIZE, this.postfilter_period_old, this.postfilter_period, overlap, -this.postfilter_gain_old, -this.postfilter_gain, this.postfilter_tapset_old, this.postfilter_tapset, nil, 0)

			for i := 0; i < overlap/2; i++ {
				buf[CeltConstants.DECODE_BUFFER_SIZE+i] = MULT16_32_Q15(window[i], etmp[overlap-1-i]) + MULT16_32_Q15(window[overlap-i-1], etmp[i])
			}
		}
	}
	this.loss_count++
}

func (this *CeltDecoder) celt_decode_with_ec(data []byte, data_ptr int, len int, pcm []int16, pcm_ptr int, frame_size int, dec *EntropyCoder, accum int) int {
	C := this.channels
	CC := this.stream_channels
	mode := this.mode
	nbEBands := mode.nbEBands
	overlap := mode.overlap
	eBands := mode.eBands
	start := this.start
	end := this.end
	frame_size *= this.downsample

	oldBandE := this.oldEBands
	oldLogE := this.oldLogE
	oldLogE2 := this.oldLogE2
	backgroundLogE := this.backgroundLogE

	LM := 0
	for LM = 0; LM <= mode.maxLM; LM++ {
		if mode.shortMdctSize<<LM == frame_size {
			break
		}
	}
	if LM > mode.maxLM {
		return OpusError.OPUS_BAD_ARG
	}
	M := 1 << LM
	N := M * mode.shortMdctSize

	if len < 0 || len > 1275 || pcm == nil {
		return OpusError.OPUS_BAD_ARG
	}

	out_syn := make([][]int, 2)
	out_syn_ptrs := make([]int, 2)
	for c := 0; c < CC; c++ {
		out_syn[c] = this.decode_mem[c]
		out_syn_ptrs[c] = CeltConstants.DECODE_BUFFER_SIZE - N
	}

	effEnd := end
	if effEnd > mode.effEBands {
		effEnd = mode.effEBands
	}

	if data == nil || len <= 1 {
		this.celt_decode_lost(N, LM)
		deemphasis(out_syn, out_syn_ptrs, pcm, pcm_ptr, N, CC, this.downsample, mode.preemph, this.preemph_memD[:], accum)
		return frame_size / this.downsample
	}

	localDec := dec
	if dec == nil {
		localDec = new(EntropyCoder)
		localDec.dec_init(data, data_ptr, len)
	}

	if CC == 1 {
		for i := 0; i < nbEBands; i++ {
			oldBandE[i] = MAX16(oldBandE[i], oldBandE[nbEBands+i])
		}
	}

	total_bits := len * 8
	tell := localDec.tell()
	silence := 0
	if tell >= total_bits {
		silence = 1
	} else if tell == 1 {
		silence = localDec.dec_bit_logp(15)
	}
	if silence != 0 {
		tell = total_bits
		localDec.nbits_total += tell - localDec.tell()
	}

	postfilter_gain := 0
	postfilter_pitch := 0
	postfilter_tapset := 0
	if start == 0 && tell+16 <= total_bits {
		if localDec.dec_bit_logp(1) != 0 {
			octave := int(localDec.dec_uint(6))
			postfilter_pitch = (16 << octave) + int(localDec.dec_bits(4+octave)) - 1
			qg := int(localDec.dec_bits(3))
			if localDec.tell()+2 <= total_bits {
				postfilter_tapset = int(localDec.dec_icdf(tapset_icdf[:], 2))
			}
			postfilter_gain = QCONST16(0.09375, 15) * (qg + 1)
		}
		tell = localDec.tell()
	}

	isTransient := 0
	if LM > 0 && tell+3 <= total_bits {
		isTransient = localDec.dec_bit_logp(3)
		tell = localDec.tell()
	}

	shortBlocks := 0
	if isTransient != 0 {
		shortBlocks = M
	}

	intra_ener := 0
	if tell+3 <= total_bits {
		intra_ener = localDec.dec_bit_logp(3)
	}
	unquant_coarse_energy(mode, start, end, oldBandE, intra_ener, localDec, CC, LM)

	tf_res := make([]int, nbEBands)
	tf_decode(start, end, isTransient, tf_res, LM, localDec)

	tell = localDec.tell()
	spread_decision := SPREAD_NORMAL
	if tell+4 <= total_bits {
		spread_decision = int(localDec.dec_icdf(spread_icdf[:], 5))
	}

	cap := make([]int, nbEBands)
	init_caps(mode, cap, LM, CC)

	offsets := make([]int, nbEBands)
	dynalloc_logp := 6
	total_bits <<= BITRES
	tell = localDec.tell_frac()
	for i := start; i < end; i++ {
		width := CC * (eBands[i+1] - eBands[i]) << LM
		quanta := IMIN(width<<BITRES, IMAX(6<<BITRES, width))
		dynalloc_loop_logp := dynalloc_logp
		boost := 0
		for tell+(dynalloc_loop_logp<<BITRES) < total_bits && boost < cap[i] {
			flag := localDec.dec_bit_logp(dynalloc_loop_logp)
			tell = localDec.tell_frac()
			if flag == 0 {
				break
			}
			boost += quanta
			total_bits -= quanta
			dynalloc_loop_logp = 1
		}
		offsets[i] = boost
		if boost > 0 {
			dynalloc_logp = IMAX(2, dynalloc_logp-1)
		}
	}

	fine_quant := make([]int, nbEBands)
	alloc_trim := 5
	if tell+(6<<BITRES) <= total_bits {
		alloc_trim = int(localDec.dec_icdf(trim_icdf[:], 7))
	}

	bits := (len*8)<<BITRES - localDec.tell_frac() - 1
	anti_collapse_rsv := 0
	if isTransient != 0 && LM >= 2 && bits >= (LM+2)<<BITRES {
		anti_collapse_rsv = 1 << BITRES
	}
	bits -= anti_collapse_rsv

	pulses := make([]int, nbEBands)
	fine_priority := make([]int, nbEBands)
	intensity := 0
	dual_stereo := 0
	balance := 0
	codedBands := compute_allocation(mode, start, end, offsets, cap, alloc_trim, &intensity, &dual_stereo, bits, &balance, pulses, fine_quant, fine_priority, CC, LM, localDec, 0, 0, 0)

	unquant_fine_energy(mode, start, end, oldBandE, fine_quant, localDec, CC)

	for c := 0; c < CC; c++ {
		copy(this.decode_mem[c][:CeltConstants.DECODE_BUFFER_SIZE-N+overlap/2], this.decode_mem[c][N:])
	}

	collapse_masks := make([]int16, CC*nbEBands)
	X := make([][]int, CC)
	for c := range X {
		X[c] = make([]int, N)
	}

	boxed_rng := this.rng
	quant_all_bands(0, mode, start, end, X[0], func() []int {
		if CC == 2 {
			return X[1]
		}
		return nil
	}(), collapse_masks, nil, pulses, shortBlocks, spread_decision, dual_stereo, intensity, tf_res, len*(8<<BITRES)-anti_collapse_rsv, balance, localDec, LM, codedBands, &boxed_rng)
	this.rng = boxed_rng

	anti_collapse_on := 0
	if anti_collapse_rsv > 0 {
		anti_collapse_on = localDec.dec_bits(1)
	}

	unquant_energy_finalise(mode, start, end, oldBandE, fine_quant, fine_priority, len*8-localDec.tell(), localDec, CC)

	if anti_collapse_on != 0 {
		anti_collapse(mode, X, collapse_masks, LM, CC, N, start, end, oldBandE, oldLogE, oldLogE2, pulses, this.rng)
	}

	if silence != 0 {
		q28 := QCONST16(28.0, DB_SHIFT)
		for i := 0; i < CC*nbEBands; i++ {
			oldBandE[i] = -q28
		}
	}

	celt_synthesis(mode, X, out_syn, out_syn_ptrs, oldBandE, start, effEnd, CC, CC, isTransient, LM, this.downsample, silence)

	for c := 0; c < CC; c++ {
		this.postfilter_period = IMAX(this.postfilter_period, CeltConstants.COMBFILTER_MINPERIOD)
		this.postfilter_period_old = IMAX(this.postfilter_period_old, CeltConstants.COMBFILTER_MINPERIOD)
		comb_filter(out_syn[c], out_syn_ptrs[c], out_syn[c], out_syn_ptrs[c], this.postfilter_period_old, this.postfilter_period, mode.shortMdctSize, this.postfilter_gain_old, this.postfilter_gain, this.postfilter_tapset_old, this.postfilter_tapset, mode.window, overlap)
		if LM != 0 {
			comb_filter(out_syn[c], out_syn_ptrs[c]+mode.shortMdctSize, out_syn[c], out_syn_ptrs[c]+mode.shortMdctSize, this.postfilter_period, postfilter_pitch, N-mode.shortMdctSize, this.postfilter_gain, postfilter_gain, this.postfilter_tapset, postfilter_tapset, mode.window, overlap)
		}
	}
	this.postfilter_period_old = this.postfilter_period
	this.postfilter_gain_old = this.postfilter_gain
	this.postfilter_tapset_old = this.postfilter_tapset
	this.postfilter_period = postfilter_pitch
	this.postfilter_gain = postfilter_gain
	this.postfilter_tapset = postfilter_tapset
	if LM != 0 {
		this.postfilter_period_old = this.postfilter_period
		this.postfilter_gain_old = this.postfilter_gain
		this.postfilter_tapset_old = this.postfilter_tapset
	}

	if CC == 1 {
		copy(oldBandE[nbEBands:], oldBandE[:nbEBands])
	}

	if isTransient == 0 {
		max_background_increase := 0
		if this.loss_count < 10 {
			max_background_increase = M * QCONST16(0.001, DB_SHIFT)
		} else {
			max_background_increase = QCONST16(1.0, DB_SHIFT)
		}
		copy(oldLogE2, oldLogE)
		copy(oldLogE, oldBandE)
		for i := 0; i < 2*nbEBands; i++ {
			backgroundLogE[i] = MIN16(backgroundLogE[i]+max_background_increase, oldBandE[i])
		}
	} else {
		for i := 0; i < 2*nbEBands; i++ {
			oldLogE[i] = MIN16(oldLogE[i], oldBandE[i])
		}
	}

	q28 := QCONST16(28.0, DB_SHIFT)
	for c := 0; c < 2; c++ {
		for i := 0; i < start; i++ {
			idx := c*nbEBands + i
			oldBandE[idx] = 0
			oldLogE[idx] = -q28
			oldLogE2[idx] = -q28
		}
		for i := end; i < nbEBands; i++ {
			idx := c*nbEBands + i
			oldBandE[idx] = 0
			oldLogE[idx] = -q28
			oldLogE2[idx] = -q28
		}
	}
	this.rng = int(localDec.rng)

	deemphasis(out_syn, out_syn_ptrs, pcm, pcm_ptr, N, CC, this.downsample, mode.preemph, this.preemph_memD[:], accum)
	this.loss_count = 0

	if localDec.tell() > 8*len {
		return OpusError.OPUS_INTERNAL_ERROR
	}
	if localDec.get_error() != 0 {
		this.error = 1
	}
	return frame_size / this.downsample
}

func (this *CeltDecoder) SetStartBand(value int) {
	if value < 0 || value >= this.mode.nbEBands {
		panic("Start band above max number of ebands (or negative)")
	}
	this.start = value
}

func (this *CeltDecoder) SetEndBand(value int) {
	if value < 1 || value > this.mode.nbEBands {
		panic("End band above max number of ebands (or less than 1)")
	}
	this.end = value
}

func (this *CeltDecoder) SetChannels(value int) {
	if value < 1 || value > 2 {
		panic("Channel count must be 1 or 2")
	}
	this.stream_channels = value
}

func (this *CeltDecoder) GetAndClearError() int {
	returnVal := this.error
	this.error = 0
	return returnVal
}

func (this *CeltDecoder) GetLookahead() int {
	return this.overlap / this.downsample
}

func (this *CeltDecoder) GetPitch() int {
	return this.postfilter_period
}

func (this *CeltDecoder) GetMode() *CeltMode {
	return this.mode
}

func (this *CeltDecoder) SetSignalling(value int) {
	this.signalling = value
}

func (this *CeltDecoder) GetFinalRange() int {
	return this.rng
}
