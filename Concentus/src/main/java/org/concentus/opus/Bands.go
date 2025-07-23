package opus

import (
	"math"
)

// Bands contains functions for band processing in the Opus codec
type Bands struct{}

// hysteresisDecision implements the hysteresis decision logic
func (b *Bands) hysteresisDecision(val int, thresholds, hysteresis []int, N, prev int) int {
	i := 0
	for ; i < N; i++ {
		if val < thresholds[i] {
			break
		}
	}

	if i > prev && val < thresholds[prev]+hysteresis[prev] {
		i = prev
	}

	if i < prev && val > thresholds[prev-1]-hysteresis[prev-1] {
		i = prev
	}

	return i
}

// celtLCGRand implements a simple linear congruential generator
func (b *Bands) celtLCGRand(seed int) int {
	return 1664525*seed + 1013904223
}

// bitexactCos is a bit-exact cos approximation
func (b *Bands) bitexactCos(x int) int {
	tmp := (4096 + x*x) >> 13
	// Assert tmp <= 32767
	x2 := tmp
	x2 = (32767 - x2) + fracMul16(x2, (-7651 + fracMul16(x2, (8277+fracMul16(-626, x2)))))
	// Assert x2 <= 32766
	return 1 + x2
}

// bitexactLog2tan computes a bit-exact log2(tan()) approximation
func (b *Bands) bitexactLog2tan(isin, icos int) int {
	lc := ecILog(uint32(icos))
	ls := ecILog(uint32(isin))
	icos <<= 15 - lc
	isin <<= 15 - ls
	return (ls-lc)*(1<<11) +
		fracMul16(isin, fracMul16(isin, -2597)+7932) -
		fracMul16(icos, fracMul16(icos, -2597)+7932)
}

// computeBandEnergies computes the amplitude (sqrt energy) in each band
func (b *Bands) computeBandEnergies(m *CeltMode, X [][]int, bandE [][]int, end, C, LM int) {
	eBands := m.eBands
	N := m.shortMdctSize << LM

	for c := 0; c < C; c++ {
		for i := 0; i < end; i++ {
			maxval := celtMaxabs32(X[c], eBands[i]<<LM, (eBands[i+1]-eBands[i])<<LM)
			if maxval > 0 {
				shift := celtILog2(maxval) - 14 + ((m.logN[i]>>BITRES + LM + 1) >> 1)
				sum := 0
				j := eBands[i] << LM
				if shift > 0 {
					for ; j < eBands[i+1]<<LM; j++ {
						val := X[c][j] >> shift
						sum += mac16_16(sum, extract16(val), extract16(val))
					}
				} else {
					shift = -shift
					for ; j < eBands[i+1]<<LM; j++ {
						val := X[c][j] << shift
						sum += mac16_16(sum, extract16(val), extract16(val))
					}
				}
				bandE[c][i] = EPSILON + vshr32(celtSqrt(sum), -shift)
			} else {
				bandE[c][i] = EPSILON
			}
		}
	}
}

// normaliseBands normalizes each band such that the energy is one
func (b *Bands) normaliseBands(m *CeltMode, freq, X, bandE [][]int, end, C, M int) {
	eBands := m.eBands

	for c := 0; c < C; c++ {
		for i := 0; i < end; i++ {
			shift := celtZLog2(bandE[c][i]) - 13
			E := vshr32(bandE[c][i], shift)
			g := extract16(celtRcp(shl32(E, 3)))

			for j := M * eBands[i]; j < M*eBands[i+1]; j++ {
				X[c][j] = mult16_16_Q15(vshr32(freq[c][j], shift-1), g)
			}
		}
	}
}

// denormaliseBands de-normalizes the energy to produce synthesis
func (b *Bands) denormaliseBands(m *CeltMode, X, freq []int, freqPtr int, bandLogE []int, bandLogEPtr, start, end, M, downsample, silence int) {
	eBands := m.eBands
	N := M * m.shortMdctSize
	bound := M * eBands[end]

	if downsample != 1 {
		bound = imin(bound, N/downsample)
	}
	if silence != 0 {
		bound = 0
		start = 0
		end = 0
	}

	f := freqPtr
	x := M * eBands[start]

	// Zero out frequencies before start
	for i := 0; i < M*eBands[start]; i++ {
		freq[f] = 0
		f++
	}

	for i := start; i < end; i++ {
		j := M * eBands[i]
		bandEnd := M * eBands[i+1]
		lg := add16(bandLogE[bandLogEPtr+i], shl16(eMeans[i], 6))

		// Handle integer part of log energy
		shift := 16 - (lg >> DB_SHIFT)
		var g int
		if shift > 31 {
			shift = 0
			g = 0
		} else {
			// Handle fractional part
			g = celtExp2Frac(lg & ((1 << DB_SHIFT) - 1))
		}

		// Handle extreme gains with negative shift
		if shift < 0 {
			if shift < -2 {
				g = 32767
				shift = -2
			}
			for ; j < bandEnd; j++ {
				freq[f] = shr32(mult16_16(X[x], g), -shift)
				f++
				x++
			}
		} else {
			for ; j < bandEnd; j++ {
				freq[f] = shr32(mult16_16(X[x], g), shift)
				f++
				x++
			}
		}
	}

	// Zero out remaining frequencies
	for i := bound; i < N; i++ {
		freq[freqPtr+i] = 0
	}
}

// BandCtx contains context for band processing
type BandCtx struct {
	Encode        int
	M             *CeltMode
	I             int
	Intensity     int
	Spread        int
	TfChange      int
	Ec            *EntropyCoder
	RemainingBits int
	BandE         [][]int
	Seed          int
}

// SplitCtx contains context for split processing
type SplitCtx struct {
	Inv     int
	Imid    int
	Iside   int
	Delta   int
	Itheta  int
	Qalloc  int
}

// computeTheta computes theta for stereo processing
func (b *Bands) computeTheta(ctx *BandCtx, sctx *SplitCtx, X, Y []int, N int, bval *int, B, B0, LM int, stereo int, fill *int) {
	var qn int
	var itheta int
	encode := ctx.Encode
	m := ctx.M
	i := ctx.I
	intensity := ctx.Intensity
	ec := ctx.Ec
	bandE := ctx.BandE

	// Decide on resolution for split parameter theta
	pulseCap := m.logN[i] + LM*(1<<BITRES)
	offset := (pulseCap >> 1)
	if stereo != 0 && N == 2 {
		offset -= QTHETA_OFFSET_TWOPHASE
	} else {
		offset -= QTHETA_OFFSET
	}
	qn = b.computeQn(N, *bval, offset, pulseCap, stereo)
	if stereo != 0 && i >= intensity {
		qn = 1
	}

	if encode != 0 {
		itheta = stereoItheta(X, Y, stereo, N)
	}

	tell := int(ec.TellFrac())

	if qn != 1 {
		if encode != 0 {
			itheta = (itheta*qn + 8192) >> 14
		}

		// Entropy coding of the angle
		if stereo != 0 && N > 2 {
			p0 := 3
			x0 := qn / 2
			ft := uint32(p0*(x0+1) + x0)
			if encode != 0 {
				var fl int
				if itheta <= x0 {
					fl = p0 * itheta
					ec.Encode(uint32(fl), uint32(fl+p0), ft)
				} else {
					fl = (itheta - 1 - x0) + (x0+1)*p0
					ec.Encode(uint32(fl), uint32(fl+1), ft)
				}
			} else {
				fs := int(ec.Decode(ft))
				var x int
				if fs < (x0+1)*p0 {
					x = fs / p0
				} else {
					x = x0 + 1 + (fs - (x0+1)*p0)
				}
				ec.DecUpdate(uint32(p0*x), uint32(p0*(x+1)), ft)
				itheta = x
			}
		} else if B0 > 1 || stereo != 0 {
			// Uniform pdf
			if encode != 0 {
				ec.EncUint(uint32(itheta), uint32(qn+1))
			} else {
				itheta = int(ec.DecUint(uint32(qn + 1)))
			}
		} else {
			ft := ((qn >> 1) + 1) * ((qn >> 1) + 1)
			if encode != 0 {
				var fl int
				if itheta <= qn>>1 {
					fl = itheta * (itheta + 1) >> 1
					fs := itheta + 1
					ec.Encode(uint32(fl), uint32(fl+fs), uint32(ft))
				} else {
					fs := qn + 1 - itheta
					fl := ft - (qn+1-itheta)*(qn+2-itheta)>>1
					ec.Encode(uint32(fl), uint32(fl+fs), uint32(ft))
				}
			} else {
				// Triangular pdf
				fm := int(ec.Decode(uint32(ft)))
				var fl, fs int
				if fm < ((qn>>1)*((qn>>1)+1)>>1) {
					itheta = (isqrt32(8*fm+1) - 1) >> 1
					fs = itheta + 1
					fl = itheta * (itheta + 1) >> 1
				} else {
					itheta = (2*(qn+1) - isqrt32(8*(ft-fm-1)+1)) >> 1
					fs = qn + 1 - itheta
					fl = ft - (qn+1-itheta)*(qn+2-itheta)>>1
				}
				ec.DecUpdate(uint32(fl), uint32(fl+fs), uint32(ft))
			}
		}
		itheta = celtUDiv(itheta*16384, qn)
		if encode != 0 && stereo != 0 {
			if itheta == 0 {
				b.intensityStereo(m, X, Y, bandE, i, N)
			} else {
				b.stereoSplit(X, Y, N)
			}
		}
	} else if stereo != 0 {
		inv := 0
		if encode != 0 {
			if itheta > 8192 {
				inv = 1
				for j := 0; j < N; j++ {
					Y[j] = -Y[j]
				}
			}
			b.intensityStereo(m, X, Y, bandE, i, N)
		}
		if *bval > 2<<BITRES && ctx.RemainingBits > 2<<BITRES {
			if encode != 0 {
				ec.EncBitLogp(inv, 2)
			} else {
				inv = ec.DecBitLogp(2)
			}
		} else {
			inv = 0
		}
		itheta = 0
	}

	qalloc := int(ec.TellFrac()) - tell
	*bval -= qalloc

	var imid, iside int
	var delta int
	if itheta == 0 {
		imid = 32767
		iside = 0
		*fill &= (1 << B) - 1
		delta = -16384
	} else if itheta == 16384 {
		imid = 0
		iside = 32767
		*fill &= ((1 << B) - 1) << B
		delta = 16384
	} else {
		imid = b.bitexactCos(itheta)
		iside = b.bitexactCos(16384 - itheta)
		delta = fracMul16((N-1)<<7, b.bitexactLog2tan(iside, imid))
	}

	sctx.Inv = inv
	sctx.Imid = imid
	sctx.Iside = iside
	sctx.Delta = delta
	sctx.Itheta = itheta
	sctx.Qalloc = qalloc
}

// computeQn computes the quantization parameter qn
func (b *Bands) computeQn(N, bval, offset, pulseCap, stereo int) int {
	exp2Table8 := []int{16384, 17866, 19483, 21247, 23170, 25267, 27554, 30048}
	N2 := 2*N - 1
	if stereo != 0 && N == 2 {
		N2--
	}

	// Calculate qb
	qb := celtSudiv(bval+N2*offset, N2)
	qb = imin(bval-pulseCap-(4<<BITRES), qb)
	qb = imin(8<<BITRES, qb)

	if qb < (1<<BITRES>>1) {
		return 1
	}

	qn := exp2Table8[qb&0x7] >> (14 - (qb >> BITRES))
	return ((qn + 1) >> 1) << 1
}

// quantBandN1 quantizes a band with N=1
func (b *Bands) quantBandN1(ctx *BandCtx, X []int, Y []int, bval int, lowbandOut []int) int {
	resynth := 0
	if ctx.Encode == 0 {
		resynth = 1
	}
	stereo := 0
	if Y != nil {
		stereo = 1
	}

	c := 0
	for c < 1+stereo {
		sign := 0
		if ctx.RemainingBits >= 1<<BITRES {
			if ctx.Encode != 0 {
				if X[0] < 0 {
					sign = 1
				}
				ctx.Ec.EncBits(uint32(sign), 1)
			} else {
				sign = int(ctx.Ec.DecBits(1))
			}
			ctx.RemainingBits -= 1 << BITRES
			bval -= 1 << BITRES
		}
		if resynth != 0 {
			if sign != 0 {
				X[0] = -NORM_SCALING
			} else {
				X[0] = NORM_SCALING
			}
		}
		X = Y
		c++
	}

	if lowbandOut != nil {
		lowbandOut[0] = shr16(X[0], 4)
	}

	return 1
}

// quantBand quantizes a single band (mono case)
func (b *Bands) quantBand(ctx *BandCtx, X []int, N, bval, B int, lowband []int, LM int, lowbandOut []int, gain int, lowbandScratch []int, fill int) int {
	N0 := N
	N_B := N
	B0 := B
	timeDivide := 0
	recombine := 0
	longBlocks := 0
	if B0 == 1 {
		longBlocks = 1
	}
	cm := 0
	resynth := 0
	if ctx.Encode == 0 {
		resynth = 1
	}

	N_B = celtUDiv(N_B, B)

	// Special case for one sample
	if N == 1 {
		return b.quantBandN1(ctx, X, nil, bval, lowbandOut)
	}

	// Band recombining to increase frequency resolution
	if lowbandScratch != nil && lowband != nil && (recombine != 0 || (N_B&1 == 0 && ctx.TfChange < 0) || B0 > 1) {
		copy(lowbandScratch, lowband)
		lowband = lowbandScratch
	}

	for k := 0; k < recombine; k++ {
		if ctx.Encode != 0 {
			b.haar1(X, N>>k, 1<<k)
		}
		if lowband != nil {
