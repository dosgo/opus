package opus
import (
	inlines "concentus/inlines"
	CeltConstants "concentus/celtconstants"
	Spread "concentus/spread"
	CWRS "concentus/cwrs"
	Kernels "concentus/kernels"
)

var SPREAD_FACTOR = [3]int{15, 10, 5}

func exp_rotation1(X []int, X_ptr int, len int, stride int, c int, s int) {
	ms := inlines.NEG16(s)
	Xptr := X_ptr
	for i := 0; i < len-stride; i++ {
		x1 := X[Xptr]
		x2 := X[Xptr+stride]
		X[Xptr+stride] = inlines.EXTRACT16(inlines.PSHR32(inlines.MAC16_16(inlines.MULT16_16(c, x2), s, x1), 15)
		X[Xptr] = inlines.EXTRACT16(inlines.PSHR32(inlines.MAC16_16(inlines.MULT16_16(c, x1), ms, x2), 15)
		Xptr++
	}
	Xptr = X_ptr + (len - 2*stride - 1)
	for i := len - 2*stride - 1; i >= 0; i-- {
		x1 := X[Xptr]
		x2 := X[Xptr+stride]
		X[Xptr+stride] = inlines.EXTRACT16(inlines.PSHR32(inlines.MAC16_16(inlines.MULT16_16(c, x2), s, x1), 15)
		X[Xptr] = inlines.EXTRACT16(inlines.PSHR32(inlines.MAC16_16(inlines.MULT16_16(c, x1), ms, x2), 15)
		Xptr--
	}
}

func exp_rotation(X []int, X_ptr int, len int, dir int, stride int, K int, spread int) {
	if 2*K >= len || spread == Spread.SPREAD_NONE {
		return
	}

	factor := SPREAD_FACTOR[spread-1]
	gain := inlines.Celt_div(int(inlines.MULT16_16(CeltConstants.Q15_ONE, len)), (len + factor*K))
	theta := inlines.HALF16(inlines.MULT16_16_Q15(gain, gain))
	c := inlines.Celt_cos_norm(inlines.EXTEND32(theta))
	s := inlines.Celt_cos_norm(inlines.EXTEND32(inlines.SUB16(CeltConstants.Q15ONE, theta)))
	stride2 := 0
	if len >= 8*stride {
		stride2 = 1
		for (stride2*stride2+stride2)*stride+(stride>>2) < len {
			stride2++
		}
	}

	len = inlines.Celt_udiv(len, stride)
	for i := 0; i < stride; i++ {
		if dir < 0 {
			if stride2 != 0 {
				exp_rotation1(X, X_ptr+i*len, len, stride2, s, c)
			}
			exp_rotation1(X, X_ptr+i*len, len, 1, c, s)
		} else {
			exp_rotation1(X, X_ptr+i*len, len, 1, c, inlines.NEG16(s).(int))
			if stride2 != 0 {
				exp_rotation1(X, X_ptr+i*len, len, stride2, s, inlines.NEG16(c).(int))
			}
		}
	}
}

func normalise_residual(iy []int, X []int, X_ptr int, N int, Ryy int, gain int) {
	k := inlines.Celt_ilog2(Ryy) >> 1
	t := inlines.VSHR32(Ryy, 2*(k-7))
	g := inlines.MULT16_16_P15(inlines.Celt_rsqrt_norm(t), gain)
	for i := 0; i < N; i++ {
		X[X_ptr+i] = inlines.EXTRACT16(inlines.PSHR32(inlines.MULT16_16(g, iy[i]), k+1))
	}
}

func extract_collapse_mask(iy []int, N int, B int) int {
	if B <= 1 {
		return 1
	}
	N0 := inlines.Celt_udiv(N, B)
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

	inlines.OpusAssert(K > 0, "alg_quant() needs at least one pulse")
	inlines.OpusAssert(N > 1, "alg_quant() needs at least two dimensions")

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
			X[X_ptr] = int(0.5 + (1.0) * (1 << 14))
			for j := X_ptr + 1; j < X_ptr+N; j++ {
				X[j] = 0
			}
			sum = int(0.5 + (1.0) * (1 << 14))
		}

		rcp := inlines.EXTRACT16(inlines.MULT16_32_Q16(K-1, inlines.Celt_rcp(sum)))
		for j := 0; j < N; j++ {
			iy[j] = inlines.MULT16_16_Q15(X[X_ptr+j], rcp)
			y[j] = iy[j]
			yy = inlines.MAC16_16(yy, y[j], y[j])
			xy = inlines.MAC16_16(xy, X[X_ptr+j], y[j])
			y[j] *= 2
			pulsesLeft -= iy[j]
		}
	}

	inlines.OpusAssert(pulsesLeft >= 1, "Allocated too many pulses in the quick pass")

	if pulsesLeft > N+3 {
		tmp := pulsesLeft
		yy = inlines.MAC16_16(yy, tmp, tmp)
		yy = inlines.MAC16_16(yy, tmp, y[0])
		iy[0] += pulsesLeft
		pulsesLeft = 0
	}

	s := 1
	for i := 0; i < pulsesLeft; i++ {
		best_id := 0
		best_num := -CeltConstants.VERY_LARGE16
		best_den := 0
		rshift := 1 + inlines.Celt_ilog2(K-pulsesLeft+i+1)
		yy = inlines.ADD16(yy, 1)
		for j := 0; j < N; j++ {
			Rxy := inlines.EXTRACT16(inlines.SHR32(inlines.ADD32(xy, inlines.EXTEND32(X[X_ptr+j])), rshift))
			Ryy := inlines.ADD16(yy, y[j])
			Rxy = inlines.MULT16_16_Q15(Rxy, Rxy)
			if inlines.MULT16_16(best_den, Rxy) > inlines.MULT16_16(Ryy, best_num) {
				best_den = Ryy
				best_num = Rxy
				best_id = j
			}
		}
		xy = inlines.ADD32(xy, inlines.EXTEND32(X[X_ptr+best_id]))
		yy = inlines.ADD16(yy, y[best_id])
		y[best_id] += 2 * s
		iy[best_id]++
	}

	for j := 0; j < N; j++ {
		X[X_ptr+j] = inlines.MULT16_16(signx[j], X[X_ptr+j])
		if signx[j] < 0 {
			iy[j] = -iy[j]
		}
	}

	CWRS.Encode_pulses(iy, N, K, enc)
	collapse_mask := extract_collapse_mask(iy, N, B)
	return collapse_mask
}

func alg_unquant(X []int, X_ptr int, N int, K int, spread int, B int, dec EntropyCoder, gain int) int {
	inlines.OpusAssert(K > 0, "alg_unquant() needs at least one pulse")
	inlines.OpusAssert(N > 1, "alg_unquant() needs at least two dimensions")
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
	k := inlines.Celt_ilog2(E) >> 1
	t := inlines.VSHR32(E, 2*(k-7))
	g := inlines.MULT16_16_P15(inlines.Celt_rsqrt_norm(t), gain)
	for i := 0; i < N; i++ {
		X[xptr] = inlines.EXTRACT16(inlines.PSHR32(inlines.MULT16_16(g, X[xptr]), k+1))
		xptr++
	}
}

func stereo_itheta(X []int, X_ptr int, Y []int, Y_ptr int, stereo int, N int) int {
	Emid := CeltConstants.EPSILON
	Eside := CeltConstants.EPSILON
	if stereo != 0 {
		for i := 0; i < N; i++ {
			m := inlines.ADD16(inlines.SHR16(X[X_ptr+i], 1), inlines.SHR16(Y[Y_ptr+i], 1))
			s := inlines.SUB16(inlines.SHR16(X[X_ptr+i], 1), inlines.SHR16(Y[Y_ptr+i], 1))
			Emid = inlines.MAC16_16(Emid, m, m)
			Eside = inlines.MAC16_16(Eside, s, s)
		}
	} else {
		Emid += Kernels.Celt_inner_prod(X, X_ptr, X, X_ptr, N)
		Eside += Kernels.Celt_inner_prod(Y, Y_ptr, Y, Y_ptr, N)
	}
	mid := inlines.Celt_sqrt(Emid)
	side := inlines.Celt_sqrt(Eside)
	itheta := inlines.MULT16_16_Q15(int(0.5+(0.63662)*(1<<15)), inlines.Celt_atan2p(side, mid))
	return itheta
}