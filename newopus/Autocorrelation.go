package opus

const QC = 10
const QS = 14

func silk_autocorr(results []int, scale *int, inputData []int16, inputDataSize int, correlationCount int) {
	corrCount := silk_min_int(inputDataSize, correlationCount)
	*scale = _celt_autocorr(inputData, results, corrCount-1, inputDataSize)
}

func _celt_autocorr(x []int16, ac []int, lag int, n int) int {
	d := int(0)
	fastN := n - lag
	shift := 0
	var xptr []int16
	xx := make([]int16, n)
	OpusAssert(n > 0)
	xptr = x

	ac0 := int(1 + (n << 7))
	if (n & 1) != 0 {
		ac0 += SHR32(MULT16_16(int(xptr[0]), int(xptr[0]), 9))
	}
	for i := (n & 1); i < n; i += 2 {
		ac0 += SHR32(MULT16_16(int(xptr[i]), int(xptr[i]), 9))
		ac0 += SHR32(MULT16_16(int(xptr[i+1]), int(xptr[i+1]), 9))
	}
	shift = celt_ilog2(ac0) - 30 + 10
	shift = (shift) / 2
	if shift > 0 {
		for i := 0; i < n; i++ {
			xx[i] = int16(PSHR32(int(xptr[i]), shift))
		}
		xptr = xx
	} else {
		shift = 0
	}

	pitch_xcorr(xptr, xptr, ac, fastN, lag+1)
	for k := 0; k <= lag; k++ {
		d = 0
		for i := k + fastN; i < n; i++ {
			d = int(MAC16_16(int16(d), int16(xptr[i]), int16(xptr[i-k])))
		}
		ac[k] += d
	}

	shift = 2 * shift
	if shift <= 0 {
		ac[0] += int(SHL32(1, -shift))
	}
	if ac[0] < 268435456 {
		shift2 := 29 - EC_ILOG(ac[0])
		for i := 0; i <= lag; i++ {
			ac[i] = SHL32(ac[i], shift2)
		}
		shift -= shift2
	} else if ac[0] >= 536870912 {
		shift2 := 1
		if ac[0] >= 1073741824 {
			shift2++
		}
		for i := 0; i <= lag; i++ {
			ac[i] = SHR32(ac[i], shift2)
		}
		shift += shift2
	}

	return shift
}

func _celt_autocorr_with_window(x []int, ac []int, window []int, overlap int, lag int, n int) int {
	d := int(0)
	fastN := n - lag
	shift := 0
	var xptr []int
	xx := make([]int, n)

	OpusAssert(n > 0)
	OpusAssert(overlap >= 0)

	if overlap == 0 {
		xptr = x
	} else {
		for i := 0; i < n; i++ {
			xx[i] = x[i]
		}
		for i := 0; i < overlap; i++ {
			xx[i] = MULT16_16_Q15(x[i], window[i])
			xx[n-i-1] = MULT16_16_Q15(x[n-i-1], window[i])
		}
		xptr = xx
	}

	ac0 := int(1 + (n << 7))
	if (n & 1) != 0 {
		ac0 += SHR32(MULT16_16(xptr[0], xptr[0]), 9)
	}
	for i := (n & 1); i < n; i += 2 {
		ac0 += SHR32(MULT16_16(xptr[i], xptr[i]), 9)
		ac0 += SHR32(MULT16_16(xptr[i+1], xptr[i+1]), 9)
	}

	shift = celt_ilog2(ac0) - 30 + 10
	shift = (shift) / 2
	if shift > 0 {
		for i := 0; i < n; i++ {
			xx[i] = PSHR32(xptr[i], shift)
		}
		xptr = xx
	} else {
		shift = 0
	}

	pitch_xcorr(xptr, xptr, ac, fastN, lag+1)
	for k := 0; k <= lag; k++ {
		d = 0
		for i := k + fastN; i < n; i++ {
			d = MAC16_16(d, xptr[i], xptr[i-k])
		}
		ac[k] += d
	}

	shift = 2 * shift
	if shift <= 0 {
		ac[0] += SHL32(1, -shift)
	}
	if ac[0] < 268435456 {
		shift2 := 29 - EC_ILOG(ac[0])
		for i := 0; i <= lag; i++ {
			ac[i] = SHL32(ac[i], shift2)
		}
		shift -= shift2
	} else if ac[0] >= 536870912 {
		shift2 := 1
		if ac[0] >= 1073741824 {
			shift2++
		}
		for i := 0; i <= lag; i++ {
			ac[i] = SHR32(ac[i], shift2)
		}
		shift += shift2
	}

	return shift
}

func silk_warped_autocorr(corr []int, scale *int, input []int16, warping_Q16 int, length int, order int) {
	var n, i, lsh int
	var tmp1_QS, tmp2_QS int
	state_QS := make([]int, SilkConstants.MAX_SHAPE_LPC_ORDER+1)
	corr_QC := make([]int64, SilkConstants.MAX_SHAPE_LPC_ORDER+1)

	OpusAssert((order & 1) == 0)
	OpusAssert(2*QS-QC >= 0)

	for n = 0; n < length; n++ {
		tmp1_QS = int(SHL32(int(input[n]), QS))
		for i = 0; i < order; i += 2 {
			tmp2_QS = silk_SMLAWB(state_QS[i], state_QS[i+1]-tmp1_QS, int(warping_Q16))
			state_QS[i] = tmp1_QS
			corr_QC[i] += silk_RSHIFT64(silk_SMULL(int(tmp1_QS), int(state_QS[0])), 2*QS-QC)
			tmp1_QS = silk_SMLAWB(state_QS[i+1], state_QS[i+2]-tmp2_QS, int(warping_Q16))
			state_QS[i+1] = tmp2_QS
			corr_QC[i+1] += silk_RSHIFT64(silk_SMULL(int(tmp2_QS), int(state_QS[0])), 2*QS-QC)
		}
		state_QS[order] = tmp1_QS
		corr_QC[order] += silk_RSHIFT64(silk_SMULL(int(tmp1_QS), int(state_QS[0])), 2*QS-QC)
	}

	lsh = silk_CLZ64(corr_QC[0]) - 35
	lsh = silk_LIMIT(lsh, -12-QC, 30-QC)
	*scale = -(QC + lsh)
	OpusAssert(*scale >= -30 && *scale <= 12)
	if lsh >= 0 {
		for i = 0; i < order+1; i++ {
			corr[i] = int(silk_LSHIFT64(corr_QC[i], lsh))
		}
	} else {
		for i = 0; i < order+1; i++ {
			corr[i] = int(silk_RSHIFT64(corr_QC[i], -lsh))
		}
	}
}
