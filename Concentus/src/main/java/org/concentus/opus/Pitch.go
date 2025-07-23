package opus

import "opustest/kernels"

// Pitch contains pitch estimation functions
type Pitch struct{}

// FindBestPitch finds the best pitch period given the correlation data
func (p *Pitch) FindBestPitch(xcorr []int32, y []int32, len int, maxPitch int, bestPitch []int, yshift int, maxcorr int32) {
	var Syy int32 = 1
	var bestNum0 int32 = -1
	var bestNum1 int32 = -1
	var bestDen0 int32 = 0
	var bestDen1 int32 = 0
	xshift := CeltInlines.CeltILog2(maxcorr) - 14

	bestPitch[0] = 0
	bestPitch[1] = 1

	// Calculate Syy (sum of squares of y)
	for j := 0; j < len; j++ {
		Syy = CeltInlines.ADD32(Syy, CeltInlines.SHR32(CeltInlines.MULT16_16(y[j], y[j]), yshift))
	}

	for i := 0; i < maxPitch; i++ {
		if xcorr[i] > 0 {
			xcorr16 := CeltInlines.EXTRACT16(CeltInlines.VSHR32(xcorr[i], xshift))
			num := CeltInlines.MULT16_16_Q15(xcorr16, xcorr16)

			if CeltInlines.MULT16_32_Q15(num, bestDen1) > CeltInlines.MULT16_32_Q15(bestNum1, Syy) {
				if CeltInlines.MULT16_32_Q15(num, bestDen0) > CeltInlines.MULT16_32_Q15(bestNum0, Syy) {
					bestNum1 = bestNum0
					bestDen1 = bestDen0
					bestPitch[1] = bestPitch[0]
					bestNum0 = num
					bestDen0 = Syy
					bestPitch[0] = i
				} else {
					bestNum1 = num
					bestDen1 = Syy
					bestPitch[1] = i
				}
			}
		}

		// Update Syy
		Syy += CeltInlines.SHR32(CeltInlines.MULT16_16(y[i+len], y[i+len]), yshift) -
			CeltInlines.SHR32(CeltInlines.MULT16_16(y[i], y[i]), yshift)
		Syy = CeltInlines.MAX32(1, Syy)
	}
}

// CeltFir5 performs a 5-tap FIR filter
func (p *Pitch) CeltFir5(x []int32, num []int32, y []int32, N int, mem []int32) {
	num0 := num[0]
	num1 := num[1]
	num2 := num[2]
	num3 := num[3]
	num4 := num[4]
	mem0 := mem[0]
	mem1 := mem[1]
	mem2 := mem[2]
	mem3 := mem[3]
	mem4 := mem[4]

	for i := 0; i < N; i++ {
		sum := CeltInlines.SHL32(CeltInlines.EXTEND32(x[i]), CeltConstants.SIG_SHIFT)
		sum = CeltInlines.MAC16_16(sum, num0, mem0)
		sum = CeltInlines.MAC16_16(sum, num1, mem1)
		sum = CeltInlines.MAC16_16(sum, num2, mem2)
		sum = CeltInlines.MAC16_16(sum, num3, mem3)
		sum = CeltInlines.MAC16_16(sum, num4, mem4)

		mem4 = mem3
		mem3 = mem2
		mem2 = mem1
		mem1 = mem0
		mem0 = x[i]

		y[i] = CeltInlines.ROUND16(sum, CeltConstants.SIG_SHIFT)
	}

	// Update memory
	mem[0] = mem0
	mem[1] = mem1
	mem[2] = mem2
	mem[3] = mem3
	mem[4] = mem4
}

// PitchDownsample performs pitch downsampling
func (p *Pitch) PitchDownsample(x [][]int32, xLp []int32, len int, C int) {
	ac := make([]int32, 5)
	tmp := CeltConstants.Q15ONE
	lpc := make([]int32, 4)
	mem := make([]int32, 5)
	lpc2 := make([]int32, 5)
	c1 := int32(0.8 * float32(1<<15)) // QCONST16(0.8f, 15)

	// Find maxabs value
	maxabs := CeltInlines.CeltMaxAbs32(x[0], 0, len)
	if C == 2 {
		maxabs1 := CeltInlines.CeltMaxAbs32(x[1], 0, len)
		maxabs = CeltInlines.MAX32(maxabs, maxabs1)
	}
	if maxabs < 1 {
		maxabs = 1
	}

	shift := CeltInlines.CeltILog2(maxabs) - 10
	if shift < 0 {
		shift = 0
	}
	if C == 2 {
		shift++
	}

	halflen := len >> 1
	for i := 1; i < halflen; i++ {
		xLp[i] = CeltInlines.SHR32(
			CeltInlines.HALF32(
				CeltInlines.HALF32(x[0][2*i-1]+x[0][2*i+1])+x[0][2*i]),
			shift)
	}
	xLp[0] = CeltInlines.SHR32(
		CeltInlines.HALF32(CeltInlines.HALF32(x[0][1])+x[0][0]),
		shift)

	if C == 2 {
		for i := 1; i < halflen; i++ {
			xLp[i] += CeltInlines.SHR32(
				CeltInlines.HALF32(
					CeltInlines.HALF32(x[1][2*i-1]+x[1][2*i+1])+x[1][2*i]),
				shift)
		}
		xLp[0] += CeltInlines.SHR32(
			CeltInlines.HALF32(CeltInlines.HALF32(x[1][1])+x[1][0]),
			shift)
	}

	// Autocorrelation
	acorr := &Autocorrelation{}
	acorr.CeltAutocorr(xLp, ac, nil, 0, 4, halflen)

	// Noise floor -40 dB
	ac[0] += CeltInlines.SHR32(ac[0], 13)

	// Lag windowing
	for i := 1; i <= 4; i++ {
		ac[i] -= CeltInlines.MULT16_32_Q15(2*int32(i*i), ac[i])
	}

	// LPC analysis
	lpcObj := &CeltLPC{}
	lpcObj.CeltLPC(lpc, ac, 4)

	// Apply bandwidth expansion
	for i := 0; i < 4; i++ {
		tmp = CeltInlines.MULT16_16_Q15(int32(0.9*float32(1<<15)), tmp)
		lpc[i] = CeltInlines.MULT16_16_Q15(lpc[i], tmp)
	}

	// Add a zero
	lpc2[0] = lpc[0] + int32(0.8*float32(1<<CeltConstants.SIG_SHIFT))
	lpc2[1] = lpc[1] + CeltInlines.MULT16_16_Q15(c1, lpc[0])
	lpc2[2] = lpc[2] + CeltInlines.MULT16_16_Q15(c1, lpc[1])
	lpc2[3] = lpc[3] + CeltInlines.MULT16_16_Q15(c1, lpc[2])
	lpc2[4] = CeltInlines.MULT16_16_Q15(c1, lpc[3])

	// Apply FIR filter
	p.CeltFir5(xLp, lpc2, xLp, halflen, mem)
}

// PitchSearch performs pitch search
func (p *Pitch) PitchSearch(xLp []int32, xLpPtr int, y []int32, len int, maxPitch int) int {
	var lag int
	bestPitch := make([]int, 2)
	var maxcorr int32
	var shift, offset int

	// Downsample by 2 again
	len2 := len >> 2
	lag = len + maxPitch
	lag2 := lag >> 2
	maxPitch2 := maxPitch >> 1

	xLp4 := make([]int32, len2)
	yLp4 := make([]int32, lag2)
	xcorr := make([]int32, maxPitch2)

	for j := 0; j < len2; j++ {
		xLp4[j] = xLp[xLpPtr+2*j]
	}
	for j := 0; j < lag2; j++ {
		yLp4[j] = y[2*j]
	}

	// Coarse search with 4x decimation
	xmax := CeltInlines.CeltMaxAbs32(xLp4, 0, len2)
	ymax := CeltInlines.CeltMaxAbs32(yLp4, 0, lag2)
	shift = CeltInlines.CeltILog2(CeltInlines.MAX32(1, CeltInlines.MAX32(xmax, ymax))) - 11
	if shift > 0 {
		for j := 0; j < len2; j++ {
			xLp4[j] = CeltInlines.SHR16(xLp4[j], shift)
		}
		for j := 0; j < lag2; j++ {
			yLp4[j] = CeltInlines.SHR16(yLp4[j], shift)
		}
		shift *= 2
	} else {
		shift = 0
	}

	// Coarse search
	pitchXCorr := &CeltPitchXCorr{}
	maxcorr = pitchXCorr.PitchXCorr(xLp4, yLp4, xcorr, len2, maxPitch2)
	p.FindBestPitch(xcorr, yLp4, len2, maxPitch2, bestPitch, 0, maxcorr)

	// Finer search with 2x decimation
	maxcorr = 1
	len1 := len >> 1
	maxPitch1 := maxPitch >> 1
	for i := 0; i < maxPitch1; i++ {
		if CeltInlines.Abs(i-2*bestPitch[0]) > 2 && CeltInlines.Abs(i-2*bestPitch[1]) > 2 {
			continue
		}

		var sum int32
		for j := 0; j < len1; j++ {
			sum += CeltInlines.SHR32(CeltInlines.MULT16_16(xLp[xLpPtr+j], y[i+j]), shift)
		}
		xcorr[i] = CeltInlines.MAX32(-1, sum)
		maxcorr = CeltInlines.MAX32(maxcorr, sum)
	}
	p.FindBestPitch(xcorr, y, len1, maxPitch1, bestPitch, shift+1, maxcorr)

	// Refine by pseudo-interpolation
	if bestPitch[0] > 0 && bestPitch[0] < (maxPitch1-1) {
		a := xcorr[bestPitch[0]-1]
		b := xcorr[bestPitch[0]]
		c := xcorr[bestPitch[0]+1]

		if (c - a) > CeltInlines.MULT16_32_Q15(int32(0.7*float32(1<<15)), b-a) {
			offset = 1
		} else if (a - c) > CeltInlines.MULT16_32_Q15(int32(0.7*float32(1<<15)), b-c) {
			offset = -1
		} else {
			offset = 0
		}
	} else {
		offset = 0
	}

	return 2*bestPitch[0] - offset
}

// RemoveDoubling removes pitch doubling
func (p *Pitch) RemoveDoubling(x []int32, maxperiod int, minperiod int, N int, T0 *int, prevPeriod int, prevGain int32) int32 {
	var k, i, T int
	var g, g0, pg int32
	var bestXy, bestYy int32
	var offset int
	minperiod0 := minperiod

	secondCheck := []int{0, 0, 3, 2, 3, 2, 5, 2, 3, 2, 3, 2, 5, 2, 3, 2}

	maxperiod /= 2
	minperiod /= 2
	*T0 /= 2
	prevPeriod /= 2
	N /= 2
	xPtr := maxperiod

	if *T0 >= maxperiod {
		*T0 = maxperiod - 1
	}

	T = *T0

	// Compute dual inner products
	xx, xy := Kernels.DualInnerProd(x, xPtr, x, xPtr, x, xPtr-T, N)
	yyLookup := make([]int32, maxperiod+1)
	yyLookup[0] = xx
	yy := xx

	for i := 1; i <= maxperiod; i++ {
		xi := xPtr - i
		yy += CeltInlines.MULT16_16(x[xi], x[xi]) - CeltInlines.MULT16_16(x[xi+N], x[xi+N])
		yyLookup[i] = CeltInlines.MAX32(0, yy)
	}
	yy = yyLookup[T]
	bestXy = xy
	bestYy = yy

	// Compute initial gain
	x2y2 := 1 + CeltInlines.HALF32(CeltInlines.MULT32_32_Q31(xx, yy))
	sh := CeltInlines.CeltILog2(x2y2) >> 1
	t := CeltInlines.VSHR32(x2y2, 2*(sh-7))
	g = CeltInlines.VSHR32(CeltInlines.MULT16_32_Q15(CeltInlines.CeltRsqrtNorm(t), xy), sh+1)
	g0 = g

	// Look for any pitch at T/k
	for k = 2; k <= 15; k++ {
		var T1, T1b int
		var g1 int32
		var cont int32
		var thresh int32

		T1 = (2*T + k) / (2 * k)
		if T1 < minperiod {
			break
		}

		// Look for another strong correlation at T1b
		if k == 2 {
			if T1+T > maxperiod {
				T1b = T
			} else {
				T1b = T + T1
			}
		} else {
			T1b = (2*secondCheck[k]*T + k) / (2 * k)
		}

		xy, xy2 := Kernels.DualInnerProd(x, xPtr, x, xPtr-T1, x, xPtr-T1b, N)
		xy += xy2
		yy = yyLookup[T1] + yyLookup[T1b]

		// Compute gain for this k
		x2y2 = 1 + CeltInlines.MULT32_32_Q31(xx, yy)
		sh = CeltInlines.CeltILog2(x2y2) >> 1
		t = CeltInlines.VSHR32(x2y2, 2*(sh-7))
		g1 = CeltInlines.VSHR32(CeltInlines.MULT16_32_Q15(CeltInlines.CeltRsqrtNorm(t), xy), sh+1)

		// Apply continuity constraint
		if CeltInlines.Abs(T1-prevPeriod) <= 1 {
			cont = prevGain
		} else if CeltInlines.Abs(T1-prevPeriod) <= 2 && 5*k*k < T {
			cont = CeltInlines.HALF16(prevGain)
		} else {
			cont = 0
		}

		// Compute threshold
		thresh = CeltInlines.MAX32(int32(0.3*float32(1<<15)),
			CeltInlines.MULT16_16_Q15(int32(0.7*float32(1<<15)), g0)-cont)

		// Bias against very high pitch
		if T1 < 3*minperiod {
			thresh = CeltInlines.MAX32(int32(0.4*float32(1<<15)),
				CeltInlines.MULT16_16_Q15(int32(0.85*float32(1<<15)), g0)-cont)
		} else if T1 < 2*minperiod {
			thresh = CeltInlines.MAX32(int32(0.5*float32(1<<15)),
				CeltInlines.MULT16_16_Q15(int32(0.9*float32(1<<15)), g0)-cont)
		}

		if g1 > thresh {
			bestXy = xy
			bestYy = yy
			T = T1
			g = g1
		}
	}

	bestXy = CeltInlines.MAX32(0, bestXy)
	if bestYy <= bestXy {
		pg = CeltConstants.Q15ONE
	} else {
		pg = CeltInlines.SHR32(CeltInlines.FracDiv32(bestXy, bestYy+1), 16)
	}

	// Compute xcorr around the best pitch
	xcorr := make([]int32, 3)
	for k := 0; k < 3; k++ {
		xcorr[k] = kernels.CeltInnerProd(x, xPtr, x, xPtr-(T+k-1), N)
	}

	// Determine offset
	if (xcorr[2] - xcorr[0]) > CeltInlines.MULT16_32_Q15(int32(0.7*float32(1<<15)), xcorr[1]-xcorr[0]) {
		offset = 1
	} else if (xcorr[0] - xcorr[2]) > CeltInlines.MULT16_32_Q15(int32(0.7*float32(1<<15)), xcorr[1]-xcorr[2]) {
		offset = -1
	} else {
		offset = 0
	}

	if pg > g {
		pg = g
	}

	*T0 = 2*T + offset
	if *T0 < minperiod0 {
		*T0 = minperiod0
	}

	return pg
}

// Helper functions and constants
type CeltInlines struct{}

func (c *CeltInlines) MAX32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func (c *CeltInlines) MULT16_16(a, b int32) int32 {
	return a * b
}

func (c *CeltInlines) SHR32(a int32, shift int) int32 {
	return a >> uint(shift)
}

func (c *CeltInlines) VSHR32(a int32, shift int) int32 {
	if shift < -31 {
		return 0
	} else if shift < 0 {
		return a << uint(-shift)
	}
	return a >> uint(shift)
}

// ... (other inline functions implementations)

type CeltConstants struct{}

const (
	SIG_SHIFT = 15
	Q15ONE    = 1 << 15
)

// ... (other constants)

type Kernels struct{}

func (k *Kernels) DualInnerProd(x []int32, xPtr1 int, y1 []int32, yPtr1 int, y2 []int32, yPtr2 int, len int) (int32, int32) {
	var xy1, xy2 int32
	for i := 0; i < len; i++ {
		xy1 += CeltInlines.MULT16_16(x[xPtr1+i], y1[yPtr1+i])
		xy2 += CeltInlines.MULT16_16(x[xPtr1+i], y2[yPtr2+i])
	}
	return xy1, xy2
}

func (k *Kernels) CeltInnerProd(x []int32, xPtr int, y []int32, yPtr int, len int) int32 {
	var sum int32
	for i := 0; i < len; i++ {
		sum += CeltInlines.MULT16_16(x[xPtr+i], y[yPtr+i])
	}
	return sum
}

type Autocorrelation struct{}

func (a *Autocorrelation) CeltAutocorr(x []int32, ac []int32, lag int, shift int, len int) {
	// Implementation of autocorrelation
}

type CeltLPC struct{}

func (c *CeltLPC) CeltLPC(lpc []int32, ac []int32, p int) {
	// Implementation of LPC analysis
}

type CeltPitchXCorr struct{}

func (c *CeltPitchXCorr) PitchXCorr(x []int32, y []int32, xcorr []int32, len int, maxPitch int) int32 {
	var maxcorr int32
	// Implementation of pitch cross-correlation
	return maxcorr
}
