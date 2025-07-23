package opus

// VQ implements vector quantization operations for Opus audio codec.
type VQ struct{}

// SPREAD_FACTOR defines the spreading factors used in rotation calculations
var SPREAD_FACTOR = [3]int{15, 10, 5}

// expRotation1 performs exponential rotation on a vector segment
func (vq *VQ) expRotation1(X []int16, X_ptr, len, stride int, c, s int16) {
	ms := neg16(s)
	Xptr := X_ptr

	// Forward pass
	for i := 0; i < len-stride; i++ {
		x1 := X[Xptr]
		x2 := X[Xptr+stride]
		X[Xptr+stride] = extract16(pshr32(mac16_16(mult16_16(c, x2), s, x1), 15))
		X[Xptr] = extract16(pshr32(mac16_16(mult16_16(c, x1), ms, x2), 15))
		Xptr++
	}

	// Backward pass
	Xptr = X_ptr + (len - 2*stride - 1)
	for i := len - 2*stride - 1; i >= 0; i-- {
		x1 := X[Xptr]
		x2 := X[Xptr+stride]
		X[Xptr+stride] = extract16(pshr32(mac16_16(mult16_16(c, x2), s, x1), 15))
		X[Xptr] = extract16(pshr32(mac16_16(mult16_16(c, x1), ms, x2), 15))
		Xptr--
	}
}

// expRotation applies exponential rotation to a vector based on spreading factor
func (vq *VQ) expRotation(X []int16, X_ptr, len, dir, stride, K int, spread int) {
	if 2*K >= len || spread == SPREAD_NONE {
		return
	}

	factor := SPREAD_FACTOR[spread-1]
	gain := celtDiv(int32(Q15_ONE*len), int32(len+factor*K))
	theta := half16(mult16_16_Q15(int16(gain), int16(gain)))

	c := celtCosNorm(extend32(theta))
	s := celtCosNorm(extend32(sub16(Q15ONE, theta)))

	stride2 := 0
	if len >= 8*stride {
		stride2 = 1
		// Compute sqrt(len/stride) with rounding
		for (stride2*stride2+stride2)*stride+(stride>>2) < len {
			stride2++
		}
	}

	len = celtUdiv(len, stride)
	for i := 0; i < stride; i++ {
		if dir < 0 {
			if stride2 != 0 {
				vq.expRotation1(X, X_ptr+(i*len), len, stride2, int16(s), int16(c))
			}
			vq.expRotation1(X, X_ptr+(i*len), len, 1, int16(c), int16(s))
		} else {
			vq.expRotation1(X, X_ptr+(i*len), len, 1, int16(c), int16(-s))
			if stride2 != 0 {
				vq.expRotation1(X, X_ptr+(i*len), len, stride2, int16(s), int16(-c))
			}
		}
	}
}

// normaliseResidual normalizes the residual vector with pitch vector
func (vq *VQ) normaliseResidual(iy []int16, X []int16, X_ptr, N int, Ryy int32, gain int16) {
	k := celtIlog2(Ryy) >> 1
	t := vsHR32(Ryy, 2*(k-7))
	g := mult16_16_P15(celtRsqrtNorm(t), gain)

	for i := 0; i < N; i++ {
		X[X_ptr+i] = extract16(pshr32(mult16_16(g, iy[i]), k+1), 15)
	}
}

// extractCollapseMask creates a collapse mask from the quantization indices
func (vq *VQ) extractCollapseMask(iy []int16, N, B int) int {
	if B <= 1 {
		return 1
	}

	N0 := celtUdiv(N, B)
	collapseMask := 0

	for i := 0; i < B; i++ {
		tmp := int16(0)
		for j := 0; j < N0; j++ {
			tmp |= iy[i*N0+j]
		}

		if tmp != 0 {
			collapseMask |= 1 << i
		}
	}

	return collapseMask
}

// algQuant performs algebraic vector quantization
func (vq *VQ) algQuant(X []int16, X_ptr, N, K, spread, B int, enc *EntropyCoder) int {
	y := make([]int16, N)
	iy := make([]int16, N)
	signx := make([]int16, N)

	// Input validation
	if K <= 0 {
		panic("alg_quant() needs at least one pulse")
	}
	if N <= 1 {
		panic("alg_quant() needs at least two dimensions")
	}

	vq.expRotation(X, X_ptr, N, 1, B, K, spread)

	// Remove sign and initialize arrays
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

	xy := int32(0)
	yy := int32(0)
	pulsesLeft := K

	// Pre-search by projecting on the pyramid
	if K > (N >> 1) {
		sum := 0
		for j := 0; j < N; j++ {
			sum += int(X[X_ptr+j])
		}

		// Handle very small X values
		if sum <= K {
			X[X_ptr] = QCONST16(1.0, 14)
			for j := X_ptr + 1; j < N+X_ptr; j++ {
				X[j] = 0
			}
			sum = QCONST16(1.0, 14)
		}

		rcp := extract16(mult16_32_Q16(int16(K-1), celtRcp(int32(sum))), 16)
		for j := 0; j < N; j++ {
			iy[j] = mult16_16_Q15(X[X_ptr+j], rcp)
			y[j] = iy[j]
			yy = mac16_16(yy, y[j], y[j])
			xy = mac16_16(xy, X[X_ptr+j], y[j])
			y[j] *= 2
			pulsesLeft -= int(iy[j])
		}
	}

	if pulsesLeft < 1 {
		panic("Allocated too many pulses in the quick pass")
	}

	// Handle remaining pulses
	if pulsesLeft > N+3 {
		tmp := int16(pulsesLeft)
		yy = mac16_16(yy, tmp, tmp)
		yy = mac16_16(yy, tmp, y[0])
		iy[0] += tmp
		pulsesLeft = 0
	}

	s := int16(1)
	for i := 0; i < pulsesLeft; i++ {
		best_id := 0
		best_num := -VERY_LARGE16
		best_den := 0
		rshift := 1 + celtIlog2(int32(K-pulsesLeft+i+1))
		yy = add16(yy, 1) // Opus bug fix - was add32

		for j := 0; j < N; j++ {
			Rxy := extract16(shr32(add32(xy, extend32(X[X_ptr+j])), rshift), 16)
			Ryy := add16(yy, y[j])

			Rxy = mult16_16_Q15(Rxy, Rxy)
			if mult16_16(best_den, Rxy) > mult16_16(Ryy, best_num) {
				best_den = Ryy
				best_num = Rxy
				best_id = j
			}
		}

		xy = add32(xy, extend32(X[X_ptr+best_id]))
		yy = add16(yy, y[best_id])
		y[best_id] += 2 * s
		iy[best_id]++
	}

	// Restore original sign
	for j := 0; j < N; j++ {
		X[X_ptr+j] = mult16_16(signx[j], X[X_ptr+j])
		if signx[j] < 0 {
			iy[j] = -iy[j]
		}
	}

	enc.EncodePulses(iy, N, K)
	return vq.extractCollapseMask(iy, N, B)
}

// algUnquant decodes pulse vector and combines with pitch vector
func (vq *VQ) algUnquant(X []int16, X_ptr, N, K, spread, B int, dec *EntropyCoder, gain int16) int {
	if K <= 0 {
		panic("alg_unquant() needs at least one pulse")
	}
	if N <= 1 {
		panic("alg_unquant() needs at least two dimensions")
	}

	iy := make([]int16, N)
	Ryy := dec.DecodePulses(iy, N, K)
	vq.normaliseResidual(iy, X, X_ptr, N, Ryy, gain)
	vq.expRotation(X, X_ptr, N, -1, B, K, spread)
	return vq.extractCollapseMask(iy, N, B)
}

// renormaliseVector normalizes a vector to unit norm
func (vq *VQ) renormaliseVector(X []int16, X_ptr, N int, gain int16) {
	E := EPSILON + Kernels{}.celtInnerProd(X, X_ptr, X, X_ptr, N)
	k := celtIlog2(E) >> 1
	t := vsHR32(E, 2*(k-7))
	g := mult16_16_P15(celtRsqrtNorm(t), gain)

	for i := 0; i < N; i++ {
		X[X_ptr+i] = extract16(pshr32(mult16_16(g, X[X_ptr+i]), k+1), 15)
	}
}

// stereoItheta computes the intensity stereo parameter
func (vq *VQ) stereoItheta(X []int16, X_ptr int, Y []int16, Y_ptr int, stereo, N int) int16 {
	Emid := EPSILON
	Eside := EPSILON

	if stereo != 0 {
		for i := 0; i < N; i++ {
			m := add16(shr16(X[X_ptr+i], 1), shr16(Y[Y_ptr+i], 1))
			s := sub16(shr16(X[X_ptr+i], 1), shr16(Y[Y_ptr+i], 1))
			Emid = mac16_16(Emid, m, m)
			Eside = mac16_16(Eside, s, s)
		}
	} else {
		Emid += Kernels{}.celtInnerProd(X, X_ptr, X, X_ptr, N)
		Eside += Kernels{}.celtInnerProd(Y, Y_ptr, Y, Y_ptr, N)
	}

	mid := celtSqrt(Emid)
	side := celtSqrt(Eside)
	// 0.63662 = 2/pi
	return mult16_16_Q15(QCONST16(0.63662, 15), celtAtan2p(side, mid))
}
