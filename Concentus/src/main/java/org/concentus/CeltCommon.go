package opus
import (
	"math"
)

var inv_table = []int16{
	255, 255, 156, 110, 86, 70, 59, 51, 45, 40, 37, 33, 31, 28, 26, 25,
	23, 22, 21, 20, 19, 18, 17, 16, 16, 15, 15, 14, 13, 13, 12, 12,
	12, 12, 11, 11, 11, 10, 10, 10, 9, 9, 9, 9, 9, 9, 8, 8,
	8, 8, 8, 7, 7, 7, 7, 7, 7, 6, 6, 6, 6, 6, 6, 6,
	6, 6, 6, 6, 6, 6, 6, 6, 6, 5, 5, 5, 5, 5, 5, 5,
	5, 5, 5, 5, 5, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4,
	4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 2,
}

func compute_vbr(mode *CeltMode, analysis *AnalysisInfo, base_target int, LM int, bitrate int, lastCodedBands int, C int, intensity int, constrained_vbr int, stereo_saving int, tot_boost int, tf_estimate int, pitch_change int, maxDepth int, variable_duration OpusFramesize, lfe int, has_surround_mask int, surround_masking int, temporal_vbr int) int {
	target := base_target
	coded_bands := lastCodedBands
	if coded_bands == 0 {
		coded_bands = mode.nbEBands
	}
	coded_bins := mode.eBands[coded_bands] << LM
	if C == 2 {
		intensityBand := intensity
		if intensity > coded_bands {
			intensityBand = coded_bands
		}
		coded_bins += mode.eBands[intensityBand] << LM
	}

	if analysis.enabled && analysis.valid != 0 && analysis.activity < 0.4 {
		target -= int(float32(coded_bins<<EntropyCoder_BITRES) * (0.4 - analysis.activity))
	}

	if C == 2 {
		coded_stereo_bands := intensity
		if intensity > coded_bands {
			coded_stereo_bands = coded_bands
		}
		coded_stereo_dof := (mode.eBands[coded_stereo_bands] << LM) - coded_stereo_bands
		max_frac := Inlines_DIV32_16(Inlines_MULT16_16(int16(0.8*32767.5), int16(coded_stereo_dof)), int16(coded_bins))
		stereo_saving_val := stereo_saving
		if stereo_saving_val > 256 {
			stereo_saving_val = 256
		}
		saving := int(Inlines_MIN32(
			Inlines_MULT16_32_Q15(int16(max_frac), int32(target)),
			Inlines_SHR32(Inlines_MULT16_16(int16(stereo_saving_val-25), int16(coded_stereo_dof<<EntropyCoder_BITRES)), 8),
		))
		target -= saving
	}

	target += tot_boost - (16 << LM)
	tf_calibration := int16(0)
	if variable_duration == OPUS_FRAMESIZE_VARIABLE {
		tf_calibration = int16(0.02 * 16384.5)
	} else {
		tf_calibration = int16(0.04 * 16384.5)
	}
	target += int(Inlines_SHL32(Inlines_MULT16_32_Q15(int16(tf_estimate)-tf_calibration, int32(target)), 1)

	if analysis.enabled && analysis.valid != 0 && lfe == 0 {
		tonal := analysis.tonality - 0.15
		if tonal < 0 {
			tonal = 0
		}
		tonal -= 0.09
		tonal_target := target + int(float32(coded_bins<<EntropyCoder_BITRES)*1.2*tonal)
		if pitch_change != 0 {
			tonal_target += int(float32(coded_bins<<EntropyCoder_BITRES) * 0.8)
		}
		target = tonal_target
	}

	if has_surround_mask != 0 && lfe == 0 {
		surround_target := target + int(Inlines_SHR32(Inlines_MULT16_16(int16(surround_masking), int16(coded_bins<<EntropyCoder_BITRES)), CeltConstants_DB_SHIFT))
		if surround_target > target/4 {
			target = surround_target
		} else {
			target = target / 4
		}
	}

	bins := mode.eBands[mode.nbEBands-2] << LM
	floor_depth := int(Inlines_SHR32(Inlines_MULT16_16(int16(C*bins<<EntropyCoder_BITRES), int16(maxDepth)), CeltConstants_DB_SHIFT))
	if floor_depth < target>>2 {
		floor_depth = target >> 2
	}
	if target > floor_depth {
		target = floor_depth
	}

	if (has_surround_mask == 0 || lfe != 0) && (constrained_vbr != 0 || bitrate < 64000) {
		rate_factor := bitrate - 32000
		if rate_factor < 0 {
			rate_factor = 0
		}
		if constrained_vbr != 0 {
			if rate_factor > int(0.67*32767.5) {
				rate_factor = int(0.67 * 32767.5)
			}
		}
		target = base_target + int(Inlines_MULT16_32_Q15(int16(rate_factor), int32(target-base_target)))
	}

	if has_surround_mask == 0 && tf_estimate < int(0.2*16384.5) {
		amount := Inlines_MULT16_16_Q15(int16(0.0000031*1073741823.5), int16(Inlines_IMAX(0, Inlines_IMIN(32000, 96000-bitrate))))
		tvbr_factor := Inlines_SHR32(amount, CeltConstants_DB_SHIFT)
		target += int(Inlines_MULT16_32_Q15(int16(tvbr_factor), int32(target)))
	}

	if target > 2*base_target {
		target = 2 * base_target
	}
	return target
}

func transient_analysis(input [][]int, len int, C int, tf_estimate *BoxedValueInt, tf_chan *BoxedValueInt) int {
	tmp := make([]int, len)
	is_transient := 0
	mask_metric := 0
	tf_chan.Val = 0
	len2 := len / 2

	for c := 0; c < C; c++ {
		unmask := 0
		mem0 := 0
		mem1 := 0
		for i := 0; i < len; i++ {
			x := Inlines_SHR32(input[c][i], CeltConstants_SIG_SHIFT)
			y := Inlines_ADD32(mem0, x)
			mem0 = mem1 + y - Inlines_SHL32(x, 1)
			mem1 = x - Inlines_SHR32(y, 1)
			tmp[i] = Inlines_EXTRACT16(Inlines_SHR32(y, 2))
		}
		for i := 0; i < 12; i++ {
			tmp[i] = 0
		}

		shift := 14 - Inlines_celt_ilog2(1+Inlines_celt_maxabs32(tmp, 0, len))
		if shift != 0 {
			for i := 0; i < len; i++ {
				tmp[i] = Inlines_SHL16(tmp[i], shift)
			}
		}

		mean := 0
		mem0 = 0
		for i := 0; i < len2; i++ {
			x2 := Inlines_PSHR32(Inlines_ADD32(Inlines_MULT16_16(tmp[2*i], tmp[2*i]), Inlines_MULT16_16(tmp[2*i+1], tmp[2*i+1])), 16)
			mean += x2
			tmp[i] = mem0 + Inlines_PSHR32(x2-mem0, 4)
			mem0 = tmp[i]
		}

		mem0 = 0
		maxE := 0
		for i := len2 - 1; i >= 0; i-- {
			tmp[i] = mem0 + Inlines_PSHR32(tmp[i]-mem0, 3)
			mem0 = tmp[i]
			if mem0 > maxE {
				maxE = mem0
			}
		}

		mean = Inlines_MULT16_16(Inlines_celt_sqrt(mean), Inlines_celt_sqrt(Inlines_MULT16_16(maxE, len2>>1)))
		norm := (len2 << (6 + 14)) / (CeltConstants_EPSILON + Inlines_SHR32(mean, 1))

		for i := 12; i < len2-5; i += 4 {
			id := Inlines_MAX32(0, Inlines_MIN32(127, Inlines_MULT16_32_Q15(int16(tmp[i]+CeltConstants_EPSILON), int32(norm))))
			unmask += int(inv_table[id])
		}
		unmask = 64 * unmask * 4 / (6 * (len2 - 17))
		if unmask > mask_metric {
			tf_chan.Val = c
			mask_metric = unmask
		}
	}

	if mask_metric > 200 {
		is_transient = 1
	}

	tf_max := Inlines_MAX16(0, int16(Inlines_celt_sqrt(27*float32(mask_metric))-42))
	tf_estimate_val := int16(0.0069 * 16384.5)
	tf_estimate.Val = int(Inlines_celt_sqrt(Inlines_MAX32(0, Inlines_SHL32(Inlines_MULT16_16(tf_estimate_val, int16(Inlines_MIN16(163, tf_max))), 14)-int32(0.139*268435455.5))))
	return is_transient
}

func patch_transient_decision(newE [][]int, oldE [][]int, nbEBands int, start int, end int, C int) int {
	mean_diff := 0
	spread_old := make([]int, 26)

	if C == 1 {
		spread_old[start] = oldE[0][start]
		for i := start + 1; i < end; i++ {
			spread_old[i] = Inlines_MAX16(spread_old[i-1]-int16(1.0*float32(1<<CeltConstants_DB_SHIFT)), oldE[0][i])
		}
	} else {
		spread_old[start] = Inlines_MAX16(oldE[0][start], oldE[1][start])
		for i := start + 1; i < end; i++ {
			spread_old[i] = Inlines_MAX16(spread_old[i-1]-int16(1.0*float32(1<<CeltConstants_DB_SHIFT)), Inlines_MAX16(oldE[0][i], oldE[1][i]))
		}
	}

	for i := end - 2; i >= start; i-- {
		spread_old[i] = Inlines_MAX16(spread_old[i], spread_old[i+1]-int16(1.0*float32(1<<CeltConstants_DB_SHIFT)))
	}

	for c := 0; c < C; c++ {
		for i := Inlines_IMAX(2, start); i < end-1; i++ {
			x1 := Inlines_MAX16(0, newE[c][i])
			x2 := Inlines_MAX16(0, spread_old[i])
			diff := Inlines_MAX16(0, x1-x2)
			mean_diff += int(diff)
		}
	}

	mean_diff /= C * (end - 1 - Inlines_IMAX(2, start))
	if mean_diff > int(1.0*float32(1<<CeltConstants_DB_SHIFT)) {
		return 1
	}
	return 0
}

func compute_mdcts(mode *CeltMode, shortBlocks int, input [][]int, output [][]int, C int, CC int, LM int, upsample int) {
	overlap := mode.overlap
	N := 0
	B := 0
	shift := 0
	if shortBlocks != 0 {
		B = shortBlocks
		N = mode.shortMdctSize
		shift = mode.maxLM
	} else {
		B = 1
		N = mode.shortMdctSize << LM
		shift = mode.maxLM - LM
	}

	for c := 0; c < CC; c++ {
		for b := 0; b < B; b++ {
			MDCT_clt_mdct_forward(mode.mdct, input[c], b*N, output[c], b, mode.window, overlap, shift, B)
		}
	}

	if CC == 2 && C == 1 {
		for i := 0; i < B*N; i++ {
			output[0][i] = Inlines_ADD32(Inlines_HALF32(output[0][i]), Inlines_HALF32(output[1][i]))
		}
	}

	if upsample != 1 {
		for c := 0; c < C; c++ {
			bound := B * N / upsample
			for i := 0; i < bound; i++ {
				output[c][i] *= upsample
			}
			for i := bound; i < B*N; i++ {
				output[c][i] = 0
			}
		}
	}
}

func celt_preemphasis(pcmp []int16, pcmp_ptr int, inp []int, inp_ptr int, N int, CC int, upsample int, coef []int, mem *BoxedValueInt, clip int) {
	coef0 := coef[0]
	m := mem.Val

	if coef[1] == 0 && upsample == 1 && clip == 0 {
		for i := 0; i < N; i++ {
			x := int(pcmp[pcmp_ptr+CC*i])
			inp[inp_ptr+i] = Inlines_SHL32(x, CeltConstants_SIG_SHIFT) - m
			m = Inlines_SHR32(Inlines_MULT16_16(int16(coef0), int16(x)), 15-CeltConstants_SIG_SHIFT)
		}
		mem.Val = m
		return
	}

	Nu := N / upsample
	if upsample != 1 {
		for i := inp_ptr; i < inp_ptr+N; i++ {
			inp[i] = 0
		}
	}
	for i := 0; i < Nu; i++ {
		inp[inp_ptr+i*upsample] = int(pcmp[pcmp_ptr+CC*i])
	}

	for i := 0; i < N; i++ {
		x := inp[inp_ptr+i]
		inp[inp_ptr+i] = Inlines_SHL32(x, CeltConstants_SIG_SHIFT) - m
		m = Inlines_SHR32(Inlines_MULT16_16(int16(coef0), int16(x)), 15-CeltConstants_SIG_SHIFT)
	}
	mem.Val = m
}

func l1_metric(tmp []int, N int, LM int, bias int) int {
	L1 := 0
	for i := 0; i < N; i++ {
		L1 += Inlines_EXTEND32(Inlines_ABS32(tmp[i]))
	}
	L1 += Inlines_MAC16_32_Q15(L1, int16(LM*bias), int32(L1))
	return L1
}

func tf_encode(start int, end int, isTransient int, tf_res []int, LM int, tf_select int, enc *EntropyCoder) {
	curr := 0
	tf_select_rsv := 0
	tf_changed := 0
	logp := 0
	if isTransient != 0 {
		logp = 2
	} else {
		logp = 4
	}
	budget := enc.storage * 8
	tell := enc.Tell()
	if LM > 0 && tell+logp+1 <= budget {
		tf_select_rsv = 1
	}
	budget -= tf_select_rsv

	for i := start; i < end; i++ {
		if tell+logp <= budget {
			enc.EncBitLogp(tf_res[i]^curr, logp)
			tell = enc.Tell()
			curr = tf_res[i]
			if curr != 0 {
				tf_changed = 1
			}
		} else {
			tf_res[i] = curr
		}
		if isTransient != 0 {
			logp = 4
		} else {
			logp = 5
		}
	}

	if tf_select_rsv != 0 && CeltTables_tf_select_table[LM][4*isTransient+0+tf_changed] != CeltTables_tf_select_table[LM][4*isTransient+2+tf_changed] {
		enc.EncBitLogp(tf_select, 1)
	} else {
		tf_select = 0
	}

	for i := start; i < end; i++ {
		tf_res[i] = CeltTables_tf_select_table[LM][4*isTransient+2*tf_select+tf_res[i]]
	}
}

func alloc_trim_analysis(m *CeltMode, X [][]int, bandLogE [][]int, end int, LM int, C int, analysis *AnalysisInfo, stereo_saving *BoxedValueInt, tf_estimate int, intensity int, surround_trim int) int {
	diff := 0
	trim := int(5.0 * 256.5)
	logXC := 0
	logXC2 := 0

	if C == 2 {
		sum := 0
		minXC := 0
		for i := 0; i < 8; i++ {
			partial := Kernels_celt_inner_prod(X[0], m.eBands[i]<<LM, X[1], m.eBands[i]<<LM, (m.eBands[i+1]-m.eBands[i])<<LM)
			sum = Inlines_ADD16(sum, Inlines_EXTRACT16(Inlines_SHR32(partial, 18)))
		}
		sum = Inlines_MULT16_16_Q15(int16(1.0/8*32767.5), int16(sum))
		if sum > 1024 {
			sum = 1024
		} else if sum < -1024 {
			sum = -1024
		}
		minXC = sum
		for i := 8; i < intensity; i++ {
			partial := Kernels_celt_inner_prod(X[0], m.eBands[i]<<LM, X[1], m.eBands[i]<<LM, (m.eBands[i+1]-m.eBands[i])<<LM)
			absPartial := Inlines_ABS16(Inlines_EXTRACT16(Inlines_SHR32(partial, 18)))
			if absPartial < minXC {
				minXC = absPartial
			}
		}
		logXC = Inlines_celt_log2(1074791424 - Inlines_MULT16_16(int16(sum), int16(sum)))
		logXC2 = Inlines_MAX16(logXC/2, Inlines_celt_log2(1074791424-Inlines_MULT16_16(int16(minXC), int16(minXC))))
		logXC = (logXC - int(6.0*float32(1<<CeltConstants_DB_SHIFT))) >> (CeltConstants_DB_SHIFT - 8)
		logXC2 = (logXC2 - int(6.0*float32(1<<CeltConstants_DB_SHIFT))) >> (CeltConstants_DB_SHIFT - 8)
		trim += Inlines_MAX16(-1024, Inlines_MULT16_16_Q15(int16(0.75*32767.5), int16(logXC)))
		if stereo_saving.Val+64 < -logXC2/2 {
			stereo_saving.Val = -logXC2/2 - 64
		} else {
			stereo_saving.Val += 64
		}
	}

	for c := 0; c < C; c++ {
		for i := 0; i < end-1; i++ {
			diff += bandLogE[c][i] * (2 + 2*i - end)
		}
	}
	diff /= C * (end - 1)
	trim -= Inlines_MAX16(-512, Inlines_MIN16(512, (diff+int(1.0*float32(1<<CeltConstants_DB_SHIFT)))/(6*(1<<(CeltConstants_DB_SHIFT-8)))))
	trim -= surround_trim >> (CeltConstants_DB_SHIFT - 8)
	trim -= 2 * (tf_estimate >> (14 - 8))

	if analysis.enabled && analysis.valid != 0 {
		adjust := int(2.0*256.5 * (analysis.tonality_slope + 0.05))
		if adjust < -512 {
			adjust = -512
		} else if adjust > 512 {
			adjust = 512
		}
		trim -= adjust
	}

	trim_index := trim >> 8
	if trim_index < 0 {
		trim_index = 0
	} else if trim_index > 10 {
		trim_index = 10
	}
	return trim_index
}

func stereo_analysis(m *CeltMode, X [][]int, LM int) int {
	thetas := 0
	sumLR := CeltConstants_EPSILON
	sumMS := CeltConstants_EPSILON

	for i := 0; i < 13; i++ {
		for j := m.eBands[i] << LM; j < m.eBands[i+1]<<LM; j++ {
			L := Inlines_EXTEND32(X[0][j])
			R := Inlines_EXTEND32(X[1][j])
			M := Inlines_ADD32(L, R)
			S := Inlines_SUB32(L, R)
			sumLR = Inlines_ADD32(sumLR, Inlines_ADD32(Inlines_ABS32(L), Inlines_ABS32(R)))
			sumMS = Inlines_ADD32(sumMS, Inlines_ADD32(Inlines_ABS32(M), Inlines_ABS32(S)))
		}
	}
	sumMS = Inlines_MULT16_32_Q15(int16(0.707107*32767.5), sumMS)
	thetas = 13
	if LM <= 1 {
		thetas -= 8
	}

	left := Inlines_MULT16_32_Q15(int16((m.eBands[13]<<(LM+1))+thetas), sumMS)
	right := Inlines_MULT16_32_Q15(int16(m.eBands[13]<<(LM+1)), sumLR)
	if left > right {
		return 1
	}
	return 0
}

func median_of_5(x []int, x_ptr int) int {
	t2 := x[x_ptr+2]
	t0, t1 := x[x_ptr], x[x_ptr+1]
	if t0 > t1 {
		t0, t1 = t1, t0
	}
	t3, t4 := x[x_ptr+3], x[x_ptr+4]
	if t3 > t4 {
		t3, t4 = t4, t3
	}
	if t0 > t3 {
		t0, t3 = t3, t0
		t1, t4 = t4, t1
	}
	if t2 > t1 {
		if t1 < t3 {
			if t2 < t3 {
				return t2
			}
			return t3
		}
		if t4 < t1 {
			return t1
		}
		return t4
	} else if t2 < t3 {
		if t1 < t3 {
			return t1
		}
		return t3
	}
	if t2 < t4 {
		return t2
	}
	return t4
}

func median_of_3(x []int, x_ptr int) int {
	t0, t1 := x[x_ptr], x[x_ptr+1]
	if t0 > t1 {
		t0, t1 = t1, t0
	}
	t2 := x[x_ptr+2]
	if t1 < t2 {
		return t1
	} else if t0 < t2 {
		return t2
	}
	return t0
}

func dynalloc_analysis(bandLogE [][]int, bandLogE2 [][]int, nbEBands int, start int, end int, C int, offsets []int, lsb_depth int, logN []int16, isTransient int, vbr int, constrained_vbr int, eBands []int16, LM int, effectiveBytes int, tot_boost_ *BoxedValueInt, lfe int, surround_dynalloc []int) int {
	tot_boost := 0
	maxDepth := int(-31.9 * float32(1<<CeltConstants_DB_SHIFT))
	noise_floor := make([]int, C*nbEBands)
	follower := make([][]int, 2)
	for i := range follower {
		follower[i] = make([]int, nbEBands)
	}

	for i := 0; i < end; i++ {
		noise_floor[i] = int(Inlines_MULT16_16(int16(0.0625*float32(1<<CeltConstants_DB_SHIFT)), logN[i])) +
			int(0.5*float32(1<<CeltConstants_DB_SHIFT)) +
			(9-lsb_depth)<<CeltConstants_DB_SHIFT -
			int(CeltTables_eMeans[i])<<6 +
			int(Inlines_MULT16_16(int16(0.0062*float32(1<<CeltConstants_DB_SHIFT)), int16((i+5)*(i+5))))
	}

	for c := 0; c < C; c++ {
		for i := 0; i < end; i++ {
			depth := bandLogE[c][i] - noise_floor[i]
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	}

	if effectiveBytes > 50 && LM >= 1 && lfe == 0 {
		last := 0
		for c := 0; c < C; c++ {
			f := follower[c]
			f[0] = bandLogE2[c][0]
			for i := 1; i < end; i++ {
				if bandLogE2[c][i] > bandLogE2[c][i-1]+int(0.5*float32(1<<CeltConstants_DB_SHIFT)) {
					last = i
				}
				f[i] = Inlines_MIN16(f[i-1]+int(1.5*float32(1<<CeltConstants_DB_SHIFT)), bandLogE2[c][i])
			}
			for i := last - 1; i >= 0; i-- {
				f[i] = Inlines_MIN16(f[i], Inlines_MIN16(f[i+1]+int(2.0*float32(1<<CeltConstants_DB_SHIFT)), bandLogE2[c][i]))
			}
			offset := int(1.0 * float32(1<<CeltConstants_DB_SHIFT))
			for i := 2; i < end-2; i++ {
				med := median_of_5(bandLogE2[c], i-2) - offset
				if f[i] < med {
					f[i] = med
				}
			}
			tmp := median_of_3(bandLogE2[c], 0) - offset
			if f[0] < tmp {
				f[0] = tmp
			}
			if f[1] < tmp {
				f[1] = tmp
			}
			tmp = median_of_3(bandLogE2[c], end-3) - offset
			if f[end-2] < tmp {
				f[end-2] = tmp
			}
			if f[end-1] < tmp {
				f[end-1] = tmp
			}
			for i := 0; i < end; i++ {
				if f[i] < noise_floor[i] {
					f[i] = noise_floor[i]
				}
			}
		}

		if C == 2 {
			for i := start; i < end; i++ {
				f0 := follower[0][i]
				f1 := follower[1][i]
				if f1 < f0-int(4.0*float32(1<<CeltConstants_DB_SHIFT)) {
					f1 = f0 - int(4.0*float32(1<<CeltConstants_DB_SHIFT))
				}
				if f0 < f1-int(4.0*float32(1<<CeltConstants_DB_SHIFT)) {
					f0 = f1 - int(4.0*float32(1<<CeltConstants_DB_SHIFT))
				}
				follower[0][i] = (Inlines_MAX16(0, bandLogE[0][i]-f0) + Inlines_MAX16(0, bandLogE[1][i]-f1)) / 2
			}
		} else {
			for i := start; i < end; i++ {
				follower[0][i] = Inlines_MAX16(0, bandLogE[0][i]-follower[0][i])
			}
		}

		for i := start; i < end; i++ {
			if follower[0][i] < surround_dynalloc[i] {
				follower[0][i] = surround_dynalloc[i]
			}
		}

		if (vbr == 0 || (constrained_vbr != 0 && isTransient == 0)) {
			for i := start; i < end; i++ {
				follower[0][i] /= 2
			}
		}

		for i := start; i < end; i++ {
			width := C * (int(eBands[i+1]) - int(eBands[i])) << LM
			boost := 0
			boost_bits := 0
			if i < 8 {
				follower[0][i] *= 2
			}
			if i >= 12 {
				follower[0][i] /= 2
			}
			if follower[0][i] > int(4.0*float32(1<<CeltConstants_DB_SHIFT)) {
				follower[0][i] = int(4.0 * float32(1<<CeltConstants_DB_SHIFT))
			}

			if width < 6 {
				boost = follower[0][i] >> CeltConstants_DB_SHIFT
				boost_bits = boost * width << EntropyCoder_BITRES
			} else if width > 48 {
				boost = (follower[0][i] * 8) >> CeltConstants_DB_SHIFT
				boost_bits = (boost * width << EntropyCoder_BITRES) / 8
			} else {
				boost = (follower[0][i] * width) / (6 << CeltConstants_DB_SHIFT)
				boost_bits = boost * 6 << EntropyCoder_BITRES
			}

			if (vbr == 0 || (constrained_vbr != 0 && isTransient == 0)) && (tot_boost+boost_bits)>>(EntropyCoder_BITRES+3) > effectiveBytes/4 {
				cap := (effectiveBytes / 4) << (EntropyCoder_BITRES + 3)
				offsets[i] = (cap - tot_boost) >> EntropyCoder_BITRES
				tot_boost = cap
				break
			} else {
				offsets[i] = boost
				tot_boost += boost_bits
			}
		}
	}

	tot_boost_.Val = tot_boost
	return maxDepth
}

func deemphasis(input [][]int, input_ptrs []int, pcm []int16, pcm_ptr int, N int, C int, downsample int, coef []int, mem []int, accum int) {
	Nd := N / downsample
	scratch := make([]int, N)

	for c := 0; c < C; c++ {
		m_val := mem[c]
		x := input[c]
		x_ptr := input_ptrs[c]
		y_ptr := pcm_ptr + c

		if downsample > 1 {
			for j := 0; j < N; j++ {
				tmp := x[x_ptr+j] + m_val + CeltConstants_VERY_SMALL
				m_val = Inlines_MULT16_32_Q15(int16(coef[0]), tmp)
				scratch[j] = tmp
			}
			for j := 0; j < Nd; j++ {
				idx := y_ptr + j*C
				pcm[idx] = Inlines_SIG2WORD16(scratch[j*downsample])
			}
		} else if accum != 0 {
			for j := 0; j < N; j++ {
				tmp := x[x_ptr+j] + m_val + CeltConstants_VERY_SMALL
				m_val = Inlines_MULT16_32_Q15(int16(coef[0]), tmp)
				idx := y_ptr + j*C
				pcm[idx] = Inlines_SAT16(Inlines_ADD32(int32(pcm[idx]), Inlines_SIG2WORD16(tmp)))
			}
		} else {
			for j := 0; j < N; j++ {
				tmp := x[x_ptr+j] + m_val + CeltConstants_VERY_SMALL
				if x[x_ptr+j] > 0 && m_val > 0 && tmp < 0 {
					tmp = math.MaxInt32
					m_val = math.MaxInt32
				} else {
					m_val = Inlines_MULT16_32_Q15(int16(coef[0]), tmp)
				}
				pcm[y_ptr+j*C] = Inlines_SIG2WORD16(tmp)
			}
		}
		mem[c] = m_val
	}
}

func celt_synthesis(mode *CeltMode, X [][]int, out_syn [][]int, out_syn_ptrs []int, oldBandE []int, start int, effEnd int, C int, CC int, isTransient int, LM int, downsample int, silence int) {
	M := 1 << LM
	overlap := mode.overlap
	nbEBands := mode.nbEBands
	N := mode.shortMdctSize << LM
	freq := make([]int, N)

	B := 0
	NB := 0
	shift := 0
	if isTransient != 0 {
		B = M
		NB = mode.shortMdctSize
		shift = mode.maxLM
	} else {
		B = 1
		NB = mode.shortMdctSize << LM
		shift = mode.maxLM - LM
	}

	if CC == 2 && C == 1 {
		Bands_denormalise_bands(mode, X[0], freq, 0, oldBandE, 0, start, effEnd, M, downsample, silence)
		freq2 := out_syn_ptrs[1] + overlap/2
		copy(out_syn[1][freq2:freq2+N], freq)
		for b := 0; b < B; b++ {
			MDCT_clt_mdct_backward(mode.mdct, out_syn[1], freq2+b, out_syn[0], out_syn_ptrs[0]+NB*b, mode.window, overlap, shift, B)
		}
		for b := 0; b < B; b++ {
			MDCT_clt_mdct_backward(mode.mdct, freq, b, out_syn[1], out_syn_ptrs[1]+NB*b, mode.window, overlap, shift, B)
		}
	} else if CC == 1 && C == 2 {
		freq2 := out_syn_ptrs[0] + overlap/2
		Bands_denormalise_bands(mode, X[0], freq, 0, oldBandE, 0, start, effEnd, M, downsample, silence)
		Bands_denormalise_bands(mode, X[1], out_syn[0], freq2, oldBandE, nbEBands, start, effEnd, M, downsample, silence)
		for i := 0; i < N; i++ {
			freq[i] = (freq[i] + out_syn[0][freq2+i]) / 2
		}
		for b := 0; b < B; b++ {
			MDCT_clt_mdct_backward(mode.mdct, freq, b, out_syn[0], out_syn_ptrs[0]+NB*b, mode.window, overlap, shift, B)
		}
	} else {
		for c := 0; c < CC; c++ {
			Bands_denormalise_bands(mode, X[c], freq, 0, oldBandE, c*nbEBands, start, effEnd, M, downsample, silence)
			for b := 0; b < B; b++ {
				MDCT_clt_mdct_backward(mode.mdct, freq, b, out_syn[c], out_syn_ptrs[c]+NB*b, mode.window, overlap, shift, B)
			}
		}
	}
}

func tf_decode(start int, end int, isTransient int, tf_res []int, LM int, dec *EntropyCoder) {
	curr := 0
	tf_select := 0
	tf_select_rsv := 0
	tf_changed := 0
	logp := 0
	if isTransient != 0 {
		logp = 2
	} else {
		logp = 4
	}
	budget := dec.storage * 8
	tell := dec.Tell()
	if LM > 0 && tell+logp+1 <= budget {
		tf_select_rsv = 1
	}
	budget -= tf_select_rsv

	for i := start; i < end; i++ {
		if tell+logp <= budget {
			bit := dec.DecBitLogp(logp)
			curr ^= bit
			if bit != 0 {
				tf_changed = 1
			}
			tell = dec.Tell()
		}
		tf_res[i] = curr
		if isTransient != 0 {
			logp = 4
		} else {
			logp = 5
		}
	}

	if tf_select_rsv != 0 && CeltTables_tf_select_table[LM][4*isTransient+0+tf_changed] != CeltTables_tf_select_table[LM][4*isTransient+2+tf_changed] {
		tf_select = dec.DecBitLogp(1)
	}

	for i := start; i < end; i++ {
		tf_res[i] = CeltTables_tf_select_table[LM][4*isTransient+2*tf_select+tf_res[i]]
	}
}

func celt_plc_pitch_search(decode_mem [][]int, C int) int {
	pitch_index := &BoxedValueInt{Val: 0}
	lp_pitch_buf := make([]int, CeltConstants_DECODE_BUFFER_SIZE>>1)
	Pitch_pitch_downsample(decode_mem, lp_pitch_buf, CeltConstants_DECODE_BUFFER_SIZE, C)
	Pitch_pitch_search(lp_pitch_buf, CeltConstants_PLC_PITCH_LAG_MAX>>1, lp_pitch_buf, CeltConstants_DECODE_BUFFER_SIZE-CeltConstants_PLC_PITCH_LAG_MAX, CeltConstants_PLC_PITCH_LAG_MAX-CeltConstants_PLC_PITCH_LAG_MIN, pitch_index)
	return CeltConstants_PLC_PITCH_LAG_MAX - pitch_index.Val
}

func resampling_factor(rate int) int {
	switch rate {
	case 48000:
		return 1
	case 24000:
		return 2
	case 16000:
		return 3
	case 12000:
		return 4
	case 8000:
		return 6
	default:
		panic("resampling_factor: unsupported rate")
	}
}

func comb_filter_const(y []int, y_ptr int, x []int, x_ptr int, T int, N int, g10 int, g11 int, g12 int) {
	xpt := x_ptr - T
	x4 := x[xpt-2]
	x3 := x[xpt-1]
	x2 := x[xpt]
	x1 := x[xpt+1]
	for i := 0; i < N; i++ {
		x0 := x[xpt+i+2]
		y[y_ptr+i] = x[x_ptr+i] +
			Inlines_MULT16_32_Q15(int16(g10), x2) +
			Inlines_MULT16_32_Q15(int16(g11), Inlines_ADD32(x1, x3)) +
			Inlines_MULT16_32_Q15(int16(g12), Inlines_ADD32(x0, x4))
		x4 = x3
		x3 = x2
		x2 = x1
		x1 = x0
	}
}

var gains = [][]int16{
	{int16(0.3066406250 * 32767.5), int16(0.2170410156 * 32767.5), int16(0.1296386719 * 32767.5)},
	{int16(0.4638671875 * 32767.5), int16(0.2680664062 * 32767.5), 0},
	{int16(0.7998046875 * 32767.5), int16(0.1000976562 * 32767.5), 0},
}

func comb_filter(y []int, y_ptr int, x []int, x_ptr int, T0 int, T1 int, N int, g0 int, g1 int, tapset0 int, tapset1 int, window []int, overlap int) {
	if g0 == 0 && g1 == 0 {
		copy(y[y_ptr:y_ptr+N], x[x_ptr:x_ptr+N])
		return
	}

	g00 := Inlines_MULT16_16_P15(int16(g0), gains[tapset0][0])
	g01 := Inlines_MULT16_16_P15(int16(g0), gains[tapset0][1])
	g02 := Inlines_MULT16_16_P15(int16(g0), gains[tapset0][2])
	g10 := Inlines_MULT16_16_P15(int16(g1), gains[tapset1][0])
	g11 := Inlines_MULT16_16_P15(int16(g1), gains[tapset1][1])
	g12 := Inlines_MULT16_16_P15(int16(g1), gains[tapset1][2])

	x1 := x[x_ptr-T1+1]
	x2 := x[x_ptr-T1]
	x3 := x[x_ptr-T1-1]
	x4 := x[x_ptr-T1-2]

	if g0 == g1 && T0 == T1 && tapset0 == tapset1 {
		overlap = 0
	}

	for i := 0; i < overlap; i++ {
		x0 := x[x_ptr+i-T1+2]
		f := Inlines_MULT16_16_Q15(int16(window[i]), int16(window[i]))
		inv_f := 32768 - int(f)
		term1 := Inlines_MULT16_32_Q15(int16(inv_f), g00*int32(x[x_ptr+i-T0]))
		term2 := Inlines_MULT16_32_Q15(int16(inv_f), g01*int32(Inlines_ADD32(x[x_ptr+i-T0+1], x[x_ptr+i-T0-1])))
		term3 := Inlines_MULT16_32_Q15(int16(inv_f), g02*int32(Inlines_ADD32(x[x_ptr+i-T0+2], x[x_ptr+i-T0-2])))
		term4 := Inlines_MULT16_32_Q15(int16(f), g10*int32(x2))
		term5 := Inlines_MULT16_32_Q15(int16(f), g11*int32(Inlines_ADD32(x1, x3)))
		term6 := Inlines_MULT16_32_Q15(int16(f), g12*int32(Inlines_ADD32(x0, x4)))
		y[y_ptr+i] = x[x_ptr+i] + int(term1+term2+term3+term4+term5+term6)
		x4 = x3
		x3 = x2
		x2 = x1
		x1 = x0
	}

	if g1 == 0 {
		copy(y[y_ptr+overlap:y_ptr+N], x[x_ptr+overlap:x_ptr+N])
	} else {
		comb_filter_const(y, y_ptr+overlap, x, x_ptr+overlap, T1, N-overlap, g10, g11, g12)
	}
}

var tf_select_table = [][]int8{
	{0, -1, 0, -1, 0, -1, 0, -1},
	{0, -1, 0, -2, 1, 0, 1, -1},
	{0, -2, 0, -3, 2, 0, 1, -1},
	{0, -2, 0, -3, 3, 0, 1, -1},
}

func init_caps(m *CeltMode, cap []int, LM int, C int) {
	for i := 0; i < m.nbEBands; i++ {
		N := (m.eBands[i+1] - m.eBands[i]) << LM
		cap[i] = (m.cache.caps[m.nbEBands*(2*LM+C-1)+i] + 64) * C * N >> 2
	}
}