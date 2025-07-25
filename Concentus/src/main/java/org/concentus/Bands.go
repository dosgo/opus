package opus
type band_ctx struct {
	encode         int
	m              *CeltMode
	i              int
	intensity      int
	spread         int
	tf_change      int
	ec             *EntropyCoder
	remaining_bits int
	bandE          [][]int
	seed           int
}

type split_ctx struct {
	inv    int
	imid   int
	iside  int
	delta  int
	itheta int
	qalloc int
}

func hysteresis_decision(val int, thresholds []int, hysteresis []int, N int, prev int) int {
	i := 0
	for i < N {
		if val < thresholds[i] {
			break
		}
		i++
	}
	if i > prev && val < thresholds[prev]+hysteresis[prev] {
		i = prev
	}
	if i < prev && val > thresholds[prev-1]-hysteresis[prev-1] {
		i = prev
	}
	return i
}

func celt_lcg_rand(seed int) int {
	return 1664525*seed + 1013904223
}

func bitexact_cos(x int) int {
	tmp := (4096 + x*x) >> 13
	OpusAssert(tmp <= 32767)
	x2 := tmp
	x2 = (32767 - x2) + FRAC_MUL16(x2, (-7651+FRAC_MUL16(x2, (8277+FRAC_MUL16(-626, x2))))
	OpusAssert(x2 <= 32766)
	return 1 + x2
}

func bitexact_log2tan(isin int, icos int) int {
	lc := EC_ILOG(int64(icos))
	ls := EC_ILOG(int64(isin))
	icos <<= 15 - lc
	isin <<= 15 - ls
	return (ls-lc)*(1<<11) +
		FRAC_MUL16(isin, FRAC_MUL16(isin, -2597)+7932) -
		FRAC_MUL16(icos, FRAC_MUL16(icos, -2597)+7932)
}

func compute_band_energies(m *CeltMode, X [][]int, bandE [][]int, end int, C int, LM int) {
	eBands := m.eBands
	N := m.shortMdctSize << LM
	for c := 0; c < C; c++ {
		for i := 0; i < end; i++ {
			maxval := celt_maxabs32(X[c], eBands[i]<<LM, (eBands[i+1]-eBands[i])<<LM)
			if maxval > 0 {
				shift := celt_ilog2(maxval) - 14 + ((m.logN[i]>>BITRES + LM + 1) >> 1)
				j := eBands[i] << LM
				sum := 0
				if shift > 0 {
					for j < eBands[i+1]<<LM {
						x := EXTRACT16(SHR32(X[c][j], shift)
						sum = MAC16_16(sum, x, x)
						j++
					}
				} else {
					for j < eBands[i+1]<<LM {
						x := EXTRACT16(SHL32(X[c][j], -shift)
						sum = MAC16_16(sum, x, x)
						j++
					}
				}
				bandE[c][i] = EPSILON + VSHR32(celt_sqrt(sum), -shift)
			} else {
				bandE[c][i] = EPSILON
			}
		}
	}
}

func normalise_bands(m *CeltMode, freq [][]int, X [][]int, bandE [][]int, end int, C int, M int) {
	eBands := m.eBands
	for c := 0; c < C; c++ {
		for i := 0; i < end; i++ {
			shift := celt_zlog2(bandE[c][i]) - 13
			E := VSHR32(bandE[c][i], shift)
			g := EXTRACT16(celt_rcp(SHL32(E, 3)))
			j := M * eBands[i]
			endBand := M * eBands[i+1]
			for j < endBand {
				X[c][j] = MULT16_16_Q15(VSHR32(freq[c][j], shift-1), g)
				j++
			}
		}
	}
}

func denormalise_bands(m *CeltMode, X []int, freq []int, freq_ptr int, bandLogE []int, bandLogE_ptr int, start int, end int, M int, downsample int, silence int) {
	eBands := m.eBands
	N := M * m.shortMdctSize
	bound := M * eBands[end]
	if downsample != 1 {
		bound = IMIN(bound, N/downsample)
	}
	if silence != 0 {
		bound = 0
		start = 0
		end = 0
	}
	f := freq_ptr
	x := M * eBands[start]

	for i := 0; i < M*eBands[start]; i++ {
		freq[f] = 0
		f++
	}

	for i := start; i < end; i++ {
		j := M * eBands[i]
		band_end := M * eBands[i+1]
		lg := ADD16(bandLogE[bandLogE_ptr+i], SHL16(eMeans[i], 6))
		shift := 16 - (lg >> DB_SHIFT)
		g := 0
		if shift <= 31 {
			g = celt_exp2_frac(lg & ((1 << DB_SHIFT) - 1))
		}
		if shift < 0 {
			if shift < -2 {
				g = 32767
				shift = -2
			}
			for j < band_end {
				freq[f] = SHR32(MULT16_16(X[x], g), -shift)
				x++
				j++
				f++
			}
		} else {
			for j < band_end {
				freq[f] = SHR32(MULT16_16(X[x], g), shift)
				x++
				j++
				f++
			}
		}
	}

	OpusAssert(start <= end)
	for i := freq_ptr + bound; i < freq_ptr+N; i++ {
		freq[i] = 0
	}
}

func anti_collapse(m *CeltMode, X_ [][]int, collapse_masks []int16, LM int, C int, size int, start int, end int, logE []int, prev1logE []int, prev2logE []int, pulses []int, seed int) {
	for i := start; i < end; i++ {
		N0 := m.eBands[i+1] - m.eBands[i]
		OpusAssert(pulses[i] >= 0)
		depth := celt_udiv(1+pulses[i], N0) >> LM
		thresh32 := SHR32(celt_exp2(-SHL16(depth, 10-BITRES)), 1)
		thresh := MULT16_32_Q15(QCONST16(0.5, 15), IMIN32(32767, thresh32))
		t := N0 << LM
		shift := celt_ilog2(t) >> 1
		t = SHL32(t, (7-shift)<<1)
		sqrt_1 := celt_rsqrt_norm(t)

		for c := 0; c < C; c++ {
			prev1 := prev1logE[c*m.nbEBands+i]
			prev2 := prev2logE[c*m.nbEBands+i]
			if C == 1 {
				prev1 = MAX16(prev1, prev1logE[m.nbEBands+i])
				prev2 = MAX16(prev2, prev2logE[m.nbEBands+i])
			}
			Ediff := EXTEND32(logE[c*m.nbEBands+i]) - EXTEND32(MIN16(prev1, prev2))
			Ediff = MAX32(0, Ediff)
			r := 0
			if Ediff < 16384 {
				r32 := SHR32(celt_exp2(-EXTRACT16(Ediff)), 1)
				r = 2 * IMIN16(16383, r32)
			}
			if LM == 3 {
				r = MULT16_16_Q14(23170, IMIN32(23169, r))
			}
			r = SHR16(IMIN16(thresh, r), 1)
			r = SHR32(MULT16_16_Q15(sqrt_1, r), shift)

			X := m.eBands[i] << LM
			renormalize := 0
			for k := 0; k < 1<<LM; k++ {
				if (collapse_masks[i*C+c] & (1 << uint(k))) == 0 {
					Xk := X + k
					for j := 0; j < N0; j++ {
						seed = celt_lcg_rand(seed)
						X_[c][Xk+(j<<LM)] = 0
						if seed&0x8000 != 0 {
							X_[c][Xk+(j<<LM)] = r
						} else {
							X_[c][Xk+(j<<LM)] = -r
						}
					}
					renormalize = 1
				}
			}
			if renormalize != 0 {
				renormalise_vector(X_[c], X, N0<<LM, Q15ONE)
			}
		}
	}
}

func intensity_stereo(m *CeltMode, X []int, X_ptr int, Y []int, Y_ptr int, bandE [][]int, bandID int, N int) {
	i := bandID
	shift := celt_zlog2(MAX32(bandE[0][i], bandE[1][i])) - 13
	left := VSHR32(bandE[0][i], shift)
	right := VSHR32(bandE[1][i], shift)
	norm := EPSILON + celt_sqrt(EPSILON+MULT16_16(left, left)+MULT16_16(right, right))
	a1 := DIV32_16(SHL32(left, 14), norm)
	a2 := DIV32_16(SHL32(right, 14), norm)
	for j := 0; j < N; j++ {
		l := X[X_ptr+j]
		r := Y[Y_ptr+j]
		X[X_ptr+j] = EXTRACT16(SHR32(MAC16_16(MULT16_16(a1, l), a2, r), 14))
	}
}

func stereo_split(X []int, X_ptr int, Y []int, Y_ptr int, N int) {
	for j := 0; j < N; j++ {
		l := MULT16_16(QCONST16(.70710678, 15), X[X_ptr+j])
		r := MULT16_16(QCONST16(.70710678, 15), Y[Y_ptr+j])
		X[X_ptr+j] = EXTRACT16(SHR32(ADD32(l, r), 15))
		Y[Y_ptr+j] = EXTRACT16(SHR32(SUB32(r, l), 15))
	}
}

func stereo_merge(X []int, X_ptr int, Y []int, Y_ptr int, mid int, N int) {
	xp := &BoxedValueInt{Val: 0}
	side := &BoxedValueInt{Val: 0}
	dual_inner_prod(Y, Y_ptr, X, X_ptr, Y, Y_ptr, N, xp, side)
	xp.Val = MULT16_32_Q15(mid, xp.Val)
	mid2 := SHR16(mid, 1)
	El := MULT16_16(mid2, mid2) + side.Val - (2 * xp.Val)
	Er := MULT16_16(mid2, mid2) + side.Val + (2 * xp.Val)
	if Er < QCONST32(6e-4, 28) || El < QCONST32(6e-4, 28) {
		copy(Y[Y_ptr:Y_ptr+N], X[X_ptr:X_ptr+N])
		return
	}

	kl := celt_ilog2(El) >> 1
	kr := celt_ilog2(Er) >> 1
	t := VSHR32(El, (kl-7)<<1)
	lgain := celt_rsqrt_norm(t)
	t = VSHR32(Er, (kr-7)<<1)
	rgain := celt_rsqrt_norm(t)

	if kl < 7 {
		kl = 7
	}
	if kr < 7 {
		kr = 7
	}

	for j := 0; j < N; j++ {
		l := MULT16_16_P15(mid, X[X_ptr+j])
		r := Y[Y_ptr+j]
		X[X_ptr+j] = EXTRACT16(PSHR32(MULT16_16(lgain, SUB16(l, r)), kl+1))
		Y[Y_ptr+j] = EXTRACT16(PSHR32(MULT16_16(rgain, ADD16(l, r)), kr+1))
	}
}

func spreading_decision(m *CeltMode, X [][]int, average *BoxedValueInt, last_decision int, hf_average *BoxedValueInt, tapset_decision *BoxedValueInt, update_hf int, end int, C int, M int) int {
	eBands := m.eBands
	OpusAssert(end > 0)

	if M*(eBands[end]-eBands[end-1]) <= 8 {
		return SPREAD_NONE
	}

	sum := 0
	nbBands := 0
	hf_sum := 0

	for c := 0; c < C; c++ {
		for i := 0; i < end; i++ {
			N := M * (eBands[i+1] - eBands[i])
			if N <= 8 {
				continue
			}

			tcount := [3]int{0, 0, 0}
			x_ptr := M * eBands[i]
			for j := x_ptr; j < x_ptr+N; j++ {
				x2N := MULT16_16(MULT16_16_Q15(X[c][j], X[c][j]), N)
				if x2N < QCONST16(0.25, 13) {
					tcount[0]++
				}
				if x2N < QCONST16(0.0625, 13) {
					tcount[1]++
				}
				if x2N < QCONST16(0.015625, 13) {
					tcount[2]++
				}
			}

			if i > m.nbEBands-4 {
				hf_sum += celt_udiv(32*(tcount[1]+tcount[0]), N)
			}

			tmp := 0
			if 2*tcount[2] >= N {
				tmp = 1
			}
			if 2*tcount[1] >= N {
				tmp++
			}
			if 2*tcount[0] >= N {
				tmp++
			}
			sum += tmp * 256
			nbBands++
		}
	}

	if update_hf != 0 {
		if hf_sum > 0 {
			hf_sum = celt_udiv(hf_sum, C*(4-m.nbEBands+end))
		}

		hf_average.Val = (hf_average.Val + hf_sum) >> 1
		hf_sum = hf_average.Val

		if tapset_decision.Val == 2 {
			hf_sum += 4
		} else if tapset_decision.Val == 0 {
			hf_sum -= 4
		}
		if hf_sum > 22 {
			tapset_decision.Val = 2
		} else if hf_sum > 18 {
			tapset_decision.Val = 1
		} else {
			tapset_decision.Val = 0
		}
	}

	OpusAssert(nbBands > 0)
	sum = celt_udiv(sum, nbBands)
	sum = (sum + average.Val) >> 1
	average.Val = sum
	sum = (3*sum + (((3-last_decision)<<7)+64) + 2) >> 2

	decision := SPREAD_NONE
	if sum < 80 {
		decision = SPREAD_AGGRESSIVE
	} else if sum < 256 {
		decision = SPREAD_NORMAL
	} else if sum < 384 {
		decision = SPREAD_LIGHT
	} else {
		decision = SPREAD_NONE
	}
	return decision
}

func deinterleave_hadamard(X []int, X_ptr int, N0 int, stride int, hadamard int) {
	N := N0 * stride
	tmp := make([]int, N)
	OpusAssert(stride > 0)

	if hadamard != 0 {
		ordery := stride - 2
		for i := 0; i < stride; i++ {
			for j := 0; j < N0; j++ {
				idx := ordery_table[ordery+i]*N0 + j
				tmp[idx] = X[X_ptr+j*stride+i]
			}
		}
	} else {
		for i := 0; i < stride; i++ {
			for j := 0; j < N0; j++ {
				tmp[i*N0+j] = X[X_ptr+j*stride+i]
			}
		}
	}
	copy(X[X_ptr:X_ptr+N], tmp)
}

func interleave_hadamard(X []int, X_ptr int, N0 int, stride int, hadamard int) {
	N := N0 * stride
	tmp := make([]int, N)

	if hadamard != 0 {
		ordery := stride - 2
		for i := 0; i < stride; i++ {
			for j := 0; j < N0; j++ {
				tmp[j*stride+i] = X[X_ptr+ordery_table[ordery+i]*N0+j]
			}
		}
	} else {
		for i := 0; i < stride; i++ {
			for j := 0; j < N0; j++ {
				tmp[j*stride+i] = X[X_ptr+i*N0+j]
			}
		}
	}
	copy(X[X_ptr:X_ptr+N], tmp)
}

func haar1(X []int, X_ptr int, N0 int, stride int) {
	N0 >>= 1
	for i := 0; i < stride; i++ {
		for j := 0; j < N0; j++ {
			tmpidx := X_ptr + i + stride*2*j
			tmp1 := MULT16_16(QCONST16(.70710678, 15), X[tmpidx])
			tmp2 := MULT16_16(QCONST16(.70710678, 15), X[tmpidx+stride])
			X[tmpidx] = EXTRACT16(PSHR32(ADD32(tmp1, tmp2), 15))
			X[tmpidx+stride] = EXTRACT16(PSHR32(SUB32(tmp1, tmp2), 15))
		}
	}
}

func compute_qn(N int, b int, offset int, pulse_cap int, stereo int) int {
	exp2_table8 := []int16{16384, 17866, 19483, 21247, 23170, 25267, 27554, 30048}
	N2 := 2*N - 1
	if stereo != 0 && N == 2 {
		N2--
	}
	qb := celt_sudiv(b+N2*offset, N2)
	qb = IMIN(b-pulse_cap-(4<<BITRES), qb)
	qb = IMIN(8<<BITRES, qb)

	qn := 1
	if qb >= (1<<BITRES)>>1 {
		qn = int(exp2_table8[qb&0x7]) >> (14 - (qb >> BITRES))
		qn = ((qn + 1) >> 1) << 1
	}
	OpusAssert(qn <= 256)
	return qn
}

func compute_theta(ctx *band_ctx, sctx *split_ctx, X []int, X_ptr int, Y []int, Y_ptr int, N int, b *BoxedValueInt, B int, B0 int, LM int, stereo int, fill *BoxedValueInt) {
	encode := ctx.encode
	m := ctx.m
	i := ctx.i
	intensity := ctx.intensity
	ec := ctx.ec
	bandE := ctx.bandE

	pulse_cap := m.logN[i] + LM*(1<<BITRES)
	offset := (pulse_cap >> 1)
	if stereo != 0 && N == 2 {
		offset -= QTHETA_OFFSET_TWOPHASE
	} else {
		offset -= QTHETA_OFFSET
	}
	qn := compute_qn(N, b.Val, offset, pulse_cap, stereo)
	if stereo != 0 && i >= intensity {
		qn = 1
	}

	itheta := 0
	if encode != 0 {
		itheta = stereo_itheta(X, X_ptr, Y, Y_ptr, stereo, N)
	}

	tell := int(ec.tell_frac())

	if qn != 1 {
		if encode != 0 {
			itheta = (itheta*qn + 8192) >> 14
		}

		if stereo != 0 && N > 2 {
			p0 := 3
			x := itheta
			x0 := qn / 2
			ft := CapToUint32(p0*(x0+1) + x0)
			if encode != 0 {
				if x <= x0 {
					ec.encode(p0*x, p0*(x+1), ft)
				} else {
					ec.encode((x-1-x0)+(x0+1)*p0, (x-x0)+(x0+1)*p0, ft)
				}
			} else {
				fs := int(ec.decode(ft))
				if fs < (x0+1)*p0 {
					x = fs / p0
				} else {
					x = x0 + 1 + (fs - (x0+1)*p0)
				}
				if x <= x0 {
					ec.dec_update(p0*x, p0*(x+1), ft)
				} else {
					ec.dec_update((x-1-x0)+(x0+1)*p0, (x-x0)+(x0+1)*p0, ft)
				}
				itheta = x
			}
		} else if B0 > 1 || stereo != 0 {
			if encode != 0 {
				ec.enc_uint(itheta, qn+1)
			} else {
				itheta = int(ec.dec_uint(qn + 1))
			}
		} else {
			ft := ((qn >> 1) + 1) * ((qn >> 1) + 1)
			if encode != 0 {
				fs := 0
				fl := 0
				if itheta <= qn>>1 {
					fs = itheta + 1
					fl = itheta * (itheta + 1) >> 1
				} else {
					fs = qn + 1 - itheta
					fl = ft - ((qn+1-itheta)*(qn+2-itheta) >> 1)
				}
				ec.encode(fl, fl+fs, ft)
			} else {
				fm := int(ec.decode(ft))
				fl := 0
				if fm < (qn>>1)*((qn>>1)+1)>>1 {
					itheta = (isqrt32(8*fm+1) - 1) >> 1
					fs = itheta + 1
					fl = itheta * (itheta + 1) >> 1
				} else {
					itheta = (2*(qn+1) - isqrt32(8*(ft-fm-1)+1)) >> 1
					fs = qn + 1 - itheta
					fl = ft - ((qn+1-itheta)*(qn+2-itheta) >> 1)
				}
				ec.dec_update(fl, fl+fs, ft)
			}
		}
		OpusAssert(itheta >= 0)
		itheta = celt_udiv(itheta*16384, qn)
		if encode != 0 && stereo != 0 {
			if itheta == 0 {
				intensity_stereo(m, X, X_ptr, Y, Y_ptr, bandE, i, N)
			} else {
				stereo_split(X, X_ptr, Y, Y_ptr, N)
			}
		}
	} else if stereo != 0 {
		inv := 0
		if encode != 0 {
			if itheta > 8192 {
				inv = 1
				for j := 0; j < N; j++ {
					Y[Y_ptr+j] = -Y[Y_ptr+j]
				}
			}
			intensity_stereo(m, X, X_ptr, Y, Y_ptr, bandE, i, N)
		}
		if b.Val > 2<<BITRES && ctx.remaining_bits > 2<<BITRES {
			if encode != 0 {
				ec.enc_bit_logp(inv, 2)
			} else {
				inv = ec.dec_bit_logp(2)
			}
		} else {
			inv = 0
		}
		itheta = 0
	}
	qalloc := int(ec.tell_frac()) - tell
	b.Val -= qalloc

	imid := 32767
	iside := 0
	delta := -16384
	fill_mask := (1 << B) - 1
	if itheta == 16384 {
		imid = 0
		iside = 32767
		fill.Val &= fill_mask << B
		delta = 16384
	} else if itheta != 0 {
		imid = bitexact_cos(itheta)
		iside = bitexact_cos(16384 - itheta)
		delta = FRAC_MUL16((N-1)<<7, bitexact_log2tan(iside, imid))
		fill_mask = (1<<B - 1) | (1<<B - 1) << B
		fill.Val &= fill_mask
	} else {
		fill.Val &= fill_mask
	}

	sctx.inv = inv
	sctx.imid = imid
	sctx.iside = iside
	sctx.delta = delta
	sctx.itheta = itheta
	sctx.qalloc = qalloc
}

func quant_band_n1(ctx *band_ctx, X []int, X_ptr int, Y []int, Y_ptr int, b int, lowband_out []int, lowband_out_ptr int) int {
	resynth := 0
	if ctx.encode == 0 {
		resynth = 1
	}
	stereo := 0
	if Y != nil {
		stereo = 1
	}
	encode := ctx.encode
	ec := ctx.ec

	x := X
	x_ptr := X_ptr
	c := 0
	for c < 1+stereo {
		sign := 0
		if ctx.remaining_bits >= 1<<BITRES {
			if encode != 0 {
				if x[x_ptr] < 0 {
					sign = 1
				}
				ec.enc_bits(sign, 1)
			} else {
				sign = ec.dec_bits(1)
			}
			ctx.remaining_bits -= 1 << BITRES
			b -= 1 << BITRES
		}
		if resynth != 0 {
			if sign != 0 {
				x[x_ptr] = -NORM_SCALING
			} else {
				x[x_ptr] = NORM_SCALING
			}
		}
		x = Y
		x_ptr = Y_ptr
		c++
	}
	if lowband_out != nil {
		lowband_out[lowband_out_ptr] = SHR16(X[X_ptr], 4)
	}
	return 1
}

func quant_partition(ctx *band_ctx, X []int, X_ptr int, N int, b int, B int, lowband []int, lowband_ptr int, LM int, gain int, fill int) int {
	resynth := 0
	if ctx.encode == 0 {
		resynth = 1
	}
	encode := ctx.encode
	m := ctx.m
	i := ctx.i
	spread := ctx.spread
	ec := ctx.ec
	cache := m.cache.bits
	cache_ptr := m.cache.index[(LM+1)*m.nbEBands+i]

	cm := 0
	if LM != -1 && b > cache[cache_ptr]+12 && N > 2 {
		B0 := B
		N0 := N
		Y := X_ptr + N
		LM -= 1
		if B == 1 {
			fill = (fill & 1) | (fill << 1)
		}
		B = (B + 1) >> 1

		boxed_b := &BoxedValueInt{Val: b}
		boxed_fill := &BoxedValueInt{Val: fill}
		sctx := &split_ctx{}
		compute_theta(ctx, sctx, X, X_ptr, X, Y, N, boxed_b, B, B0, LM, 0, boxed_fill)
		b = boxed_b.Val
		fill = boxed_fill.Val

		imid := sctx.imid
		iside := sctx.iside
		delta := sctx.delta
		itheta := sctx.itheta
		qalloc := sctx.qalloc
		mid := imid
		side := iside

		if B0 > 1 && itheta != 0 {
			if itheta > 8192 {
				delta -= delta >> (4 - LM)
			} else {
				delta = IMIN(0, delta+(N<<BITRES>>(5-LM)))
			}
		}
		mbits := IMAX(0, IMIN(b, (b-delta)/2))
		sbits := b - mbits
		ctx.remaining_bits -= qalloc

		next_lowband2 := 0
		if lowband != nil {
			next_lowband2 = lowband_ptr + N
		}

		rebalance := ctx.remaining_bits
		if mbits >= sbits {
			cm = quant_partition(ctx, X, X_ptr, N, mbits, B, lowband, lowband_ptr, LM, MULT16_16_P15(gain, mid), fill)
			rebalance = mbits - (rebalance - ctx.remaining_bits)
			if rebalance > 3<<BITRES && itheta != 0 {
				sbits += rebalance - (3 << BITRES)
			}
			cm |= quant_partition(ctx, X, Y, N, sbits, B, lowband, next_lowband2, LM, MULT16_16_P15(gain, side), fill>>B) << (B0 >> 1)
		} else {
			cm = quant_partition(ctx, X, Y, N, sbits, B, lowband, next_lowband2, LM, MULT16_16_P15(gain, side), fill>>B) << (B0 >> 1)
			rebalance = sbits - (rebalance - ctx.remaining_bits)
			if rebalance > 3<<BITRES && itheta != 16384 {
				mbits += rebalance - (3 << BITRES)
			}
			cm |= quant_partition(ctx, X, X_ptr, N, mbits, B, lowband, lowband_ptr, LM, MULT16_16_P15(gain, mid), fill)
		}
	} else {
		q := bits2pulses(m, i, LM, b)
		curr_bits := pulses2bits(m, i, LM, q)
		ctx.remaining_bits -= curr_bits

		for ctx.remaining_bits < 0 && q > 0 {
			ctx.remaining_bits += curr_bits
			q--
			curr_bits = pulses2bits(m, i, LM, q)
			ctx.remaining_bits -= curr_bits
		}

		if q != 0 {
			K := get_pulses(q)
			if encode != 0 {
				cm = alg_quant(X, X_ptr, N, K, spread, B, ec)
			} else {
				cm = alg_unquant(X, X_ptr, N, K, spread, B, ec, gain)
			}
		} else if resynth != 0 {
			cm_mask := (1 << B) - 1
			fill &= cm_mask
			if fill == 0 {
				for j := 0; j < N; j++ {
					X[X_ptr+j] = 0
				}
			} else {
				if lowband == nil {
					for j := 0; j < N; j++ {
						ctx.seed = celt_lcg_rand(ctx.seed)
						X[X_ptr+j] = int(ctx.seed) >> 20
					}
					cm = cm_mask
				} else {
					for j := 0; j < N; j++ {
						ctx.seed = celt_lcg_rand(ctx.seed)
						tmp := 0
						if ctx.seed&0x8000 != 0 {
							tmp = int(QCONST16(1.0/256, 10))
						} else {
							tmp = -int(QCONST16(1.0/256, 10))
						}
						X[X_ptr+j] = lowband[lowband_ptr+j] + tmp
					}
					cm = fill
				}
				renormalise_vector(X, X_ptr, N, gain)
			}
		}
	}
	return cm
}

var bit_interleave_table = []int{0, 1, 1, 1, 2, 3, 3, 3, 2, 3, 3, 3, 2, 3, 3, 3}
var bit_deinterleave_table = []int{0, 3, 12, 15, 48, 51, 60, 63, 192, 195, 204, 207, 240, 243, 252, 255}

func quant_band(ctx *band_ctx, X []int, X_ptr int, N int, b int, B int, lowband []int, lowband_ptr int, LM int, lowband_out []int, lowband_out_ptr int, gain int, lowband_scratch []int, lowband_scratch_ptr int, fill int) int {
	N0 := N
	N_B := N
	B0 := B
	time_divide := 0
	recombine := 0
	longBlocks := 0
	if B0 == 1 {
		longBlocks = 1
	} else {
		longBlocks = 0
	}
	cm := 0
	resynth := 0
	if ctx.encode == 0 {
		resynth = 1
	}
	encode := ctx.encode
	tf_change := ctx.tf_change

	N_B = celt_udiv(N_B, B)

	if tf_change > 0 {
		recombine = tf_change
	}

	if lowband_scratch != nil && lowband != nil && (recombine != 0 || (N_B%2 == 0 && tf_change < 0) || B0 > 1) {
		copy(lowband_scratch[lowband_scratch_ptr:lowband_scratch_ptr+N], lowband[lowband_ptr:lowband_ptr+N])
		lowband = lowband_scratch
		lowband_ptr = lowband_scratch_ptr
	}

	for k := 0; k < recombine; k++ {
		if encode != 0 {
			haar1(X, X_ptr, N>>k, 1<<k)
		}
		if lowband != nil {
			haar1(lowband, lowband_ptr, N>>k, 1<<k)
		}
		fill = bit_interleave_table[fill&0xF] | bit_interleave_table[fill>>4]<<2
	}
	B >>= recombine
	N_B <<= recombine

	for (N_B&1) == 0 && tf_change < 0 {
		if encode != 0 {
			haar1(X, X_ptr, N_B, B)
		}
		if lowband != nil {
			haar1(lowband, lowband_ptr, N_B, B)
		}
		fill |= fill << B
		B <<= 1
		N_B >>= 1
		time_divide++
		tf_change++
	}
	B0 = B
	N_B0 := N_B

	if B0 > 1 {
		if encode != 0 {
			deinterleave_hadamard(X, X_ptr, N_B>>recombine, B0<<recombine, longBlocks)
		}
		if lowband != nil {
			deinterleave_hadamard(lowband, lowband_ptr, N_B>>recombine, B0<<recombine, longBlocks)
		}
	}

	cm = quant_partition(ctx, X, X_ptr, N, b, B, lowband, lowband_ptr, LM, gain, fill)

	if resynth != 0 {
		if B0 > 1 {
			interleave_hadamard(X, X_ptr, N_B>>recombine, B0<<recombine, longBlocks)
		}

		N_B = N_B0
		B = B0
		for k := 0; k < time_divide; k++ {
			B >>= 1
			N_B <<= 1
			cm |= cm >> B
			haar1(X, X_ptr, N_B, B)
		}

		for k := 0; k < recombine; k++ {
			cm = bit_deinterleave_table[cm]
			haar1(X, X_ptr, N0>>k, 1<<k)
		}
		B <<= recombine

		if lowband_out != nil {
			n := celt_sqrt(SHL32(int64(N0), 22))
			for j := 0; j < N0; j++ {
				lowband_out[lowband_out_ptr+j] = MULT16_16_Q15(int(n), X[X_ptr+j])
			}
		}
		cm &= (1 << B) - 1
	}
	return cm
}

func quant_band_stereo(ctx *band_ctx, X []int, X_ptr int, Y []int, Y_ptr int, N int, b int, B int, lowband []int, lowband_ptr int, LM int, lowband_out []int, lowband_out_ptr int, lowband_scratch []int, lowband_scratch_ptr int, fill int) int {
	imid := 0
	iside := 0
	inv := 0
	cm := 0
	resynth := 0
	if ctx.encode == 0 {
		resynth = 1
	}
	encode := ctx.encode
	ec := ctx.ec
	orig_fill := fill

	if N == 1 {
		return quant_band_n1(ctx, X, X_ptr, Y, Y_ptr, b, lowband_out, lowband_out_ptr)
	}

	boxed_b := &BoxedValueInt{Val: b}
	boxed_fill := &BoxedValueInt{Val: fill}
	sctx := &split_ctx{}
	compute_theta(ctx, sctx, X, X_ptr, Y, Y_ptr, N, boxed_b, B, B, LM, 1, boxed_fill)
	b = boxed_b.Val
	fill = boxed_fill.Val

	inv = sctx.inv
	imid = sctx.imid
	iside = sctx.iside
	delta := sctx.delta
	itheta := sctx.itheta
	qalloc := sctx.qalloc
	mid := imid
	side := iside

	if N == 2 {
		mbits := b
		sbits := 0
		if itheta != 0 && itheta != 16384 {
			sbits = 1 << BITRES
		}
		mbits -= sbits
		c := 0
		if itheta > 8192 {
			c = 1
		}
		ctx.remaining_bits -= qalloc + sbits

		x2 := X
		x2_ptr := X_ptr
		y2 := Y
		y2_ptr := Y_ptr
		if c != 0 {
			x2 = Y
			x2_ptr = Y_ptr
			y2 = X
			y2_ptr = X_ptr
		}

		sign := 0
		if sbits != 0 {
			if encode != 0 {
				if x2[x2_ptr]*y2[y2_ptr+1]-x2[x2_ptr+1]*y2[y2_ptr] < 0 {
					sign = 1
				}
				ec.enc_bits(sign, 1)
			} else {
				sign = ec.dec_bits(1)
			}
		}
		sign = 1 - 2*sign
		cm = quant_band(ctx, x2, x2_ptr, N, mbits, B, lowband, lowband_ptr, LM, lowband_out, lowband_out_ptr, Q15ONE, lowband_scratch, lowband_scratch_ptr, orig_fill)

		y2[y2_ptr] = -sign * x2[x2_ptr+1]
		y2[y2_ptr+1] = sign * x2[x2_ptr]

		if resynth != 0 {
			X[X_ptr] = MULT16_16_Q15(mid, X[X_ptr])
			X[X_ptr+1] = MULT16_16_Q15(mid, X[X_ptr+1])
			Y[Y_ptr] = MULT16_16_Q15(side, Y[Y_ptr])
			Y[Y_ptr+1] = MULT16_16_Q15(side, Y[Y_ptr+1])
			tmp := X[X_ptr]
			X[X_ptr] = tmp - Y[Y_ptr]
			Y[Y_ptr] = tmp + Y[Y_ptr]
			tmp = X[X_ptr+1]
			X[X_ptr+1] = tmp - Y[Y_ptr+1]
			Y[Y_ptr+1] = tmp + Y[Y_ptr+1]
		}
	} else {
		mbits := IMAX(0, IMIN(b, (b-delta)/2))
		sbits := b - mbits
		ctx.remaining_bits -= qalloc

		rebalance := ctx.remaining_bits
		if mbits >= sbits {
			cm = quant_band(ctx, X, X_ptr, N, mbits, B, lowband, lowband_ptr, LM, lowband_out, lowband_out_ptr, Q15ONE, lowband_scratch, lowband_scratch_ptr, fill)
			rebalance = mbits - (rebalance - ctx.remaining_bits)
			if rebalance > 3<<BITRES && itheta != 0 {
				sbits += rebalance - (3 << BITRES)
			}
			cm |= quant_band(ctx, Y, Y_ptr, N, sbits, B, nil, 0, LM, nil, 0, side, nil, 0, fill>>B)
		} else {
			cm = quant_band(ctx, Y, Y_ptr, N, sbits, B, nil, 0, LM, nil, 0, side, nil, 0, fill>>B)
			rebalance = sbits - (rebalance - ctx.remaining_bits)
			if rebalance > 3<<BITRES && itheta != 16384 {
				mbits += rebalance - (3 << BITRES)
			}
			cm |= quant_band(ctx, X, X_ptr, N, mbits, B, lowband, lowband_ptr, LM, lowband_out, lowband_out_ptr, Q15ONE, lowband_scratch, lowband_scratch_ptr, fill)
		}
	}

	if resynth != 0 && N != 2 {
		stereo_merge(X, X_ptr, Y, Y_ptr, mid, N)
	}
	if inv != 0 {
		for j := Y_ptr; j < Y_ptr+N; j++ {
			Y[j] = -Y[j]
		}
	}
	return cm
}

func quant_all_bands(encode int, m *CeltMode, start int, end int, X_ []int, Y_ []int, collapse_masks []int16, bandE [][]int, pulses []int, shortBlocks int, spread int, dual_stereo int, intensity int, tf_res []int, total_bits int, balance int, ec *EntropyCoder, LM int, codedBands int, seed *BoxedValueInt) {
	eBands := m.eBands
	M := 1 << LM
	B := 1
	if shortBlocks != 0 {
		B = M
	}
	norm_offset := M * eBands[start]
	norm := make([]int, M*eBands[m.nbEBands-1]-norm_offset)
	norm2 := M*eBands[m.nbEBands-1] - norm_offset
	lowband_scratch := X_
	lowband_scratch_ptr := M * eBands[m.nbEBands-1]
	lowband_offset := 0
	C := 1
	if Y_ != nil {
		C = 2
	}

	ctx := &band_ctx{
		encode:         encode,
		m:              m,
		intensity:      intensity,
		spread:         spread,
		ec:             ec,
		bandE:          bandE,
		seed:           seed.Val,
	}

	for i := start; i < end; i++ {
		ctx.i = i
		last := 0
		if i == end-1 {
			last = 1
		}

		X_ptr := M * eBands[i]
		X := X_
		Y := Y_
		Y_ptr := M * eBands[i]
		N := M*eBands[i+1] - X_ptr
		tell := int(ec.tell_frac())

		if i != start {
			balance -= tell
		}
		remaining_bits := total_bits - tell - 1
		ctx.remaining_bits = remaining_bits

		b := 0
		if i <= codedBands-1 {
			curr_balance := celt_sudiv(balance, IMIN(3, codedBands-i))
			b = IMAX(0, IMIN(16383, IMIN(remaining_bits+1, pulses[i]+curr_balance)))
		}

		update_lowband := 1
		effective_lowband := -1
		x_cm := int64(0)
		y_cm := int64(0)

		if resynth := 0; encode == 0 {
			resynth = 1
		}; resynth != 0 && M*eBands[i]-N >= M*eBands[start] && (update_lowband != 0 || lowband_offset == 0) {
			lowband_offset = i
		}

		tf_change := tf_res[i]
		ctx.tf_change = tf_change

		if i >= m.effEBands {
			X = norm
			X_ptr = 0
			if Y_ != nil {
				Y = norm
				Y_ptr = 0
			}
			lowband_scratch = nil
		}
		if i == end-1 {
			lowband_scratch = nil
		}

		if lowband_offset != 0 && (spread != SPREAD_AGGRESSIVE || B > 1 || tf_change < 0) {
			fold_start := lowband_offset
			for fold_start > 0 && M*eBands[fold_start] > effective_lowband+norm_offset {
				fold_start--
			}
			fold_end := lowband_offset
			for fold_end < m.nbEBands-1 && M*eBands[fold_end] < effective_lowband+norm_offset+N {
				fold_end++
			}
			x_cm = 0
			y_cm = 0
			for fold_i := fold_start; fold_i < fold_end; fold_i++ {
				x_cm |= int64(collapse_masks[fold_i*C+0])
				y_cm |= int64(collapse_masks[fold_i*C+C-1])
			}
		} else {
			x_cm = (1 << B) - 1
			y_cm = (1 << B) - 1
		}

		if dual_stereo != 0 && i == intensity {
			dual_stereo = 0
			if resynth := 0; encode == 0 {
				resynth = 1
			}; resynth != 0 {
				for j := 0; j < M*eBands[i]-norm_offset; j++ {
					norm[j] = HALF32(norm[j] + norm[norm2+j])
				}
			}
		}

		if dual_stereo != 0 {
			x_cm = int64(quant_band(ctx, X, X_ptr, N, b/2, B, effective_lowband, lowband_ptr, LM, last, lowband_out_ptr, Q15ONE, lowband_scratch, lowband_scratch_ptr, int(x_cm)))
			y_cm = int64(quant_band(ctx, Y, Y_ptr, N, b/2, B, effective_lowband, norm2+lowband_ptr, LM, last, norm2+lowband_out_ptr, Q15ONE, lowband_scratch, lowband_scratch_ptr, int(y_cm)))
		} else {
			if Y != nil {
				x_cm = int64(quant_band_stereo(ctx, X, X_ptr, Y, Y_ptr, N, b, B, effective_lowband, lowband_ptr, LM, last, lowband_out_ptr, lowband_scratch, lowband_scratch_ptr, int(x_cm|y_cm)))
			} else {
				x_cm = int64(quant_band(ctx, X, X_ptr, N, b, B, effective_lowband, lowband_ptr, LM, last, lowband_out_ptr, gain, lowband_scratch, lowband_scratch_ptr, int(x_cm|y_cm)))
			}
			y_cm = x_cm
		}
		collapse_masks[i*C+0] = int16(x_cm)
		collapse_masks[i*C+C-1] = int16(y_cm)
		balance += pulses[i] + tell
		if b > N<<BITRES {
			update_lowband = 1
		} else {
			update_lowband = 0
		}
	}
	seed.Val = ctx.seed
}