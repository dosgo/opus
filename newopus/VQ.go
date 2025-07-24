package opus

var SPREAD_FACTOR = [3]int{15, 10, 5}

func exp_rotation1(X []int, X_ptr int, len int, stride int, c int, s int) {
	ms := NEG16(s)
	Xptr := X_ptr
	for i := 0; i < len-stride; i++ {
		x1 := X[Xptr]
		x2 := X[Xptr+stride]
		X[Xptr+stride] = EXTRACT16(PSHR32(MAC16_16(MULT16_16(c, x2), s, x1), 15))
		X[Xptr] = EXTRACT16(PSHR32(MAC16_16(MULT16_16(c, x1), ms, x2), 15))
		Xptr++
	}
	Xptr = X_ptr + (len - 2*stride - 1)
	for i := len - 2*stride - 1; i >= 0; i-- {
		x1 := X[Xptr]
		x2 := X[Xptr+stride]
		X[Xptr+stride] = EXTRACT16(PSHR32(MAC16_16(MULT16_16(c, x2), s, x1), 15))
		X[Xptr] = EXTRACT16(PSHR32(MAC16_16(MULT16_16(c, x1), ms, x2), 15))
		Xptr--
	}
}

func exp_rotation(X []int, X_ptr int, len int, dir int, stride int, K int, spread int) {
	if 2*K >= len || spread == Spread.SPREAD_NONE {
		return
	}

	factor := SPREAD_FACTOR[spread-1]
	gain := Celt_div(int(MULT16_16(CeltConstants.Q15_ONE, len)), (len + factor*K))
	theta := HALF16(MULT16_16_Q15(gain, gain))
	c := Celt_cos_norm(EXTEND32(theta))
	s := Celt_cos_norm(EXTEND32(SUB16(CeltConstants.Q15ONE, theta)))
	stride2 := 0
	if len >= 8*stride {
		stride2 = 1
		for (stride2*stride2+stride2)*stride+(stride>>2) < len {
			stride2++
		}
	}

	len = Celt_udiv(len, stride)
	for i := 0; i < stride; i++ {
		if dir < 0 {
			if stride2 != 0 {
				exp_rotation1(X, X_ptr+i*len, len, stride2, s, c)
			}
			exp_rotation1(X, X_ptr+i*len, len, 1, c, s)
		} else {
			exp_rotation1(X, X_ptr+i*len, len, 1, c, NEG16(s).(int))
			if stride2 != 0 {
				exp_rotation1(X, X_ptr+i*len, len, stride2, s, NEG16(c).(int))
			}
		}
	}
}

func normalise_residual(iy []int, X []int, X_ptr int, N int, Ryy int, gain int) {
	k := Celt_ilog2(Ryy) >> 1
	t := VSHR32(Ryy, 2*(k-7))
	g := MULT16_16_P15(Celt_rsqrt_norm(t), gain)
	for i := 0; i < N; i++ {
		X[X_ptr+i] = EXTRACT16(PSHR32(MULT16_16(g, iy[i]), k+1))
	}
}

func extract_collapse_mask(iy []int, N int, B int) int {
	if B <= 1 {
		return 1
	}
	N0 := Celt_udiv(N, B)
	collapse_mask := 0
	for i := 0; i < B; i++ {
		tmp := 0
		for j := 0; j < N0; j++ {
			tmp |= iy[i*N0+j]
		}
		if tmp != 0 {
			collapse_mask |= 1 << i
		}
	}
	return collapse_mask
}

func alg_quant(X []int, X_ptr int, N int, K int, spread int, B int, enc EntropyCoder) int {
	y := make([]int, N)
	iy := make([]int, N)
	signx := make([]int, N)

	OpusAssert(K > 0)
	OpusAssert(N > 1)

	exp_rotation(X, X_ptr, N, 1, B, K, spread)

	sum := 0
	for j := 0; j < N; j++ {
		if X[X_ptr+j] > 0 {
			signx[j] = 1
		} else {
			signx[j] = -1
			X[X_ptr+j] = -X[X_ptr+j]
		}
		iy[j] = 0
		y[j] = 0
	}

	xy := 0
	yy := 0
	pulsesLeft := K

	if K > (N >> 1) {
		for j := 0; j < N; j++ {
			sum += X[X_ptr+j]
		}

		if sum <= K {
			X[X_ptr] = int(0.5 + (1.0)*(1<<14))
			for j := X_ptr + 1; j < X_ptr+N; j++ {
				X[j] = 0
			}
			sum = int(0.5 + (1.0)*(1<<14))
		}

		rcp := EXTRACT16(MULT16_32_Q16(K-1, Celt_rcp(sum)))
		for j := 0; j < N; j++ {
			iy[j] = MULT16_16_Q15(X[X_ptr+j], rcp)
			y[j] = iy[j]
			yy = MAC16_16(yy, y[j], y[j])
			xy = MAC16_16(xy, X[X_ptr+j], y[j])
			y[j] *= 2
			pulsesLeft -= iy[j]
		}
	}

	OpusAssert(pulsesLeft >= 1)

	if pulsesLeft > N+3 {
		tmp := pulsesLeft
		yy = MAC16_16(yy, tmp, tmp)
		yy = MAC16_16(yy, tmp, y[0])
		iy[0] += pulsesLeft
		pulsesLeft = 0
	}

	s := 1
	for i := 0; i < pulsesLeft; i++ {
		best_id := 0
		best_num := -CeltConstants.VERY_LARGE16
		best_den := 0
		rshift := 1 + Celt_ilog2(K-pulsesLeft+i+1)
		yy = ADD16(yy, 1)
		for j := 0; j < N; j++ {
			Rxy := EXTRACT16(SHR32(ADD32(xy, EXTEND32(X[X_ptr+j])), rshift))
			Ryy := ADD16(yy, y[j])
			Rxy = MULT16_16_Q15(Rxy, Rxy)
			if MULT16_16(best_den, Rxy) > MULT16_16(Ryy, best_num) {
				best_den = Ryy
				best_num = Rxy
				best_id = j
			}
		}
		xy = ADD32(xy, EXTEND32(X[X_ptr+best_id]))
		yy = ADD16(yy, y[best_id])
		y[best_id] += 2 * s
		iy[best_id]++
	}

	for j := 0; j < N; j++ {
		X[X_ptr+j] = MULT16_16(signx[j], X[X_ptr+j])
		if signx[j] < 0 {
			iy[j] = -iy[j]
		}
	}

	CWRS.Encode_pulses(iy, N, K, enc)
	collapse_mask := extract_collapse_mask(iy, N, B)
	return collapse_mask
}

func alg_unquant(X []int, X_ptr int, N int, K int, spread int, B int, dec EntropyCoder, gain int) int {
	OpusAssert(K > 0, "alg_unquant() needs at least one pulse")
	OpusAssert(N > 1, "alg_unquant() needs at least two dimensions")
	iy := make([]int, N)
	Ryy := CWRS.Decode_pulses(iy, N, K, dec)
	normalise_residual(iy, X, X_ptr, N, Ryy, gain)
	exp_rotation(X, X_ptr, N, -1, B, K, spread)
	collapse_mask := extract_collapse_mask(iy, N, B)
	return collapse_mask
}

func renormalise_vector(X []int, X_ptr int, N int, gain int) {
	xptr := X_ptr
	E := CeltConstants.EPSILON + Kernels.Celt_inner_prod(X, X_ptr, X, X_ptr, N)
	k := celt_ilog2(E) >> 1
	t := VSHR32(E, 2*(k-7))
	g := MULT16_16_P15(celt_rsqrt_norm(t), gain)
	for i := 0; i < N; i++ {
		X[xptr] = EXTRACT16(PSHR32(MULT16_16(g, X[xptr]), k+1))
		xptr++
	}
}

func stereo_itheta(X []int, X_ptr int, Y []int, Y_ptr int, stereo int, N int) int {
	Emid := CeltConstants.EPSILON
	Eside := CeltConstants.EPSILON
	if stereo != 0 {
		for i := 0; i < N; i++ {
			m := ADD16(SHR16(X[X_ptr+i], 1), SHR16(Y[Y_ptr+i], 1))
			s := SUB16(SHR16(X[X_ptr+i], 1), SHR16(Y[Y_ptr+i], 1))
			Emid = MAC16_16(Emid, m, m)
			Eside = MAC16_16(Eside, s, s)
		}
	} else {
		Emid += celt_inner_prod(X, X_ptr, X, X_ptr, N)
		Eside += celt_inner_prod(Y, Y_ptr, Y, Y_ptr, N)
	}
	mid := celt_sqrt(Emid)
	side := celt_sqrt(Eside)
	itheta := MULT16_16_Q15(int(0.5+(0.63662)*(1<<15)), Celt_atan2p(side, mid))
	return itheta
}
