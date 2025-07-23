package silk

import (
	"math"
)

const (
	maxFrameSize   = 384 // subfr_length * nb_subfr = (0.005*16000 + 16)*4 = 384
	qa             = 25
	nBitsHeadRoom  = 2
	minRshifts     = -16
	maxRshifts     = 32 - qa
	findLpcCondFac = 1e-5 // TuningParameters.FIND_LPC_COND_FAC
)

// BurgModified computes reflection coefficients from input signal using the modified Burg algorithm
func BurgModified(
	resNrg *int32, // O: Residual energy
	resNrgQ *int32, // O: Residual energy Q value
	aQ16 []int32, // O: Prediction coefficients (length order)
	x []int16, // I: Input signal, length: nb_subfr * (D + subfr_length)
	xPtr int, // I: Offset into x
	minInvGainQ30 int32, // I: Inverse of max prediction gain
	subfrLength int, // I: Input signal subframe length (incl. D preceding samples)
	nbSubfr int, // I: Number of subframes stacked in x
	D int, // I: Order
) {
	// Validate input
	if subfrLength*nbSubfr > maxFrameSize {
		panic("subfrLength * nbSubfr exceeds maxFrameSize")
	}

	var (
		k, n, s, lz, rshifts, reachedMaxGain                               int
		c0, num, nrg, rcQ31, invGainQ30, atmpQA, atmp1, tmp1, tmp2, x1, x2 int32
		xOffset                                                            int
		cFirstRow                                                          [maxOrderLPC]int32
		cLastRow                                                           [maxOrderLPC]int32
		afQA                                                               [maxOrderLPC]int32
		cAf                                                                [maxOrderLPC + 1]int32
		cAb                                                                [maxOrderLPC + 1]int32
		xcorr                                                              [maxOrderLPC]int32
	)

	// Compute autocorrelations, added over subframes
	c0_64 := innerProd16Aligned64(x, xPtr, x, xPtr, subfrLength*nbSubfr)
	lz = clz64(c0_64)
	rshifts = 32 + 1 + nBitsHeadRoom - lz

	// Clamp rshifts to valid range
	if rshifts > maxRshifts {
		rshifts = maxRshifts
	}
	if rshifts < minRshifts {
		rshifts = minRshifts
	}

	// Apply right shifts
	if rshifts > 0 {
		c0 = int32(c0_64 >> uint(rshifts))
	} else {
		c0 = int32(c0_64) << uint(-rshifts)
	}

	// Initialize correlation matrices
	condFac := int32(findLpcCondFac * (1 << 32))
	cAb[0] = c0 + smmul(condFac, c0) + 1 // Q(-rshifts)
	cAf[0] = c0 + smmul(condFac, c0) + 1
	// Compute first and last rows of correlation matrix
	if rshifts > 0 {
		for s = 0; s < nbSubfr; s++ {
			xOffset = xPtr + s*subfrLength
			for n = 1; n < D+1; n++ {
				cFirstRow[n-1] += int32(innerProd16Aligned64(x, xOffset, x, xOffset+n, subfrLength-n) >> uint(rshifts))
			}
		}
	} else {
		for s = 0; s < nbSubfr; s++ {
			xOffset = xPtr + s*subfrLength
			pitchXCorr(x, xOffset, x, xOffset+1, xcorr[:], subfrLength-D, D)
			for n = 1; n < D+1; n++ {
				var d int32
				for i := n + subfrLength - D; i < subfrLength; i++ {
					d = mac16_16(d, x[xOffset+i], x[xOffset+i-n])
				}
				xcorr[n-1] += d
			}
			for n = 1; n < D+1; n++ {
				cFirstRow[n-1] += xcorr[n-1] << uint(-rshifts)
			}
		}
	}
	copy(cLastRow[:], cFirstRow[:])

	// Re-initialize
	cAb[0] = c0 + smmul(condFac, c0) + 1 // Q(-rshifts)
	cAf[0] = c0 + smmul(condFac, c0) + 1
	invGainQ30 = 1 << 30
	reachedMaxGain = 0

	// Main processing loop for each order
	for n = 0; n < D; n++ {
		// Update correlation matrices
		if rshifts > -2 {
			for s = 0; s < nbSubfr; s++ {
				xOffset = xPtr + s*subfrLength
				x1 = -int32(x[xOffset+n]) << uint(16-rshifts)               // Q(16-rshifts)
				x2 = -int32(x[xOffset+subfrLength-n-1]) << uint(16-rshifts) // Q(16-rshifts)
				tmp1 = int32(x[xOffset+n]) << uint(qa-16)                   // Q(QA-16)
				tmp2 = int32(x[xOffset+subfrLength-n-1]) << uint(qa-16)     // Q(QA-16)

				for k = 0; k < n; k++ {
					cFirstRow[k] = smlawb(cFirstRow[k], x1, x[xOffset+n-k-1])         // Q(-rshifts)
					cLastRow[k] = smlawb(cLastRow[k], x2, x[xOffset+subfrLength-n+k]) // Q(-rshifts)
					atmpQA = afQA[k]
					tmp1 = smlawb(tmp1, atmpQA, x[xOffset+n-k-1])           // Q(QA-16)
					tmp2 = smlawb(tmp2, atmpQA, x[xOffset+subfrLength-n+k]) // Q(QA-16)
				}

				tmp1 = -tmp1 << uint(32-qa-rshifts) // Q(16-rshifts)
				tmp2 = -tmp2 << uint(32-qa-rshifts) // Q(16-rshifts)

				for k = 0; k <= n; k++ {
					cAf[k] = smlawb(cAf[k], tmp1, x[xOffset+n-k])               // Q(-rshift)
					cAb[k] = smlawb(cAb[k], tmp2, x[xOffset+subfrLength-n+k-1]) // Q(-rshift)
				}
			}
		} else {
			for s = 0; s < nbSubfr; s++ {
				xOffset = xPtr + s*subfrLength
				x1 = -int32(x[xOffset+n]) << uint(-rshifts)               // Q(-rshifts)
				x2 = -int32(x[xOffset+subfrLength-n-1]) << uint(-rshifts) // Q(-rshifts)
				tmp1 = int32(x[xOffset+n]) << 17                          // Q17
				tmp2 = int32(x[xOffset+subfrLength-n-1]) << 17            // Q17

				for k = 0; k < n; k++ {
					cFirstRow[k] = mla(cFirstRow[k], x1, x[xOffset+n-k-1])         // Q(-rshifts)
					cLastRow[k] = mla(cLastRow[k], x2, x[xOffset+subfrLength-n+k]) // Q(-rshifts)
					atmp1 = rshiftRound(afQA[k], qa-17)                            // Q17
					tmp1 = mla(tmp1, x[xOffset+n-k-1], atmp1)                      // Q17
					tmp2 = mla(tmp2, x[xOffset+subfrLength-n+k], atmp1)            // Q17
				}

				tmp1, tmp2 = -tmp1, -tmp2 // Q17

				for k = 0; k <= n; k++ {
					cAf[k] = smlaww(cAf[k], tmp1, int32(x[xOffset+n-k])<<uint(-rshifts-1))               // Q(-rshift)
					cAb[k] = smlaww(cAb[k], tmp2, int32(x[xOffset+subfrLength-n+k-1])<<uint(-rshifts-1)) // Q(-rshift)
				}
			}
		}

		// Calculate nominator and denominator for next reflection coefficient
		tmp1 = cFirstRow[n]         // Q(-rshifts)
		tmp2 = cLastRow[n]          // Q(-rshifts)
		num = 0                     // Q(-rshifts)
		nrg = add32(cAb[0], cAf[0]) // Q(1-rshifts)

		for k = 0; k < n; k++ {
			atmpQA = afQA[k]
			lz = clz32(abs32(atmpQA)) - 1
			lz = min(32-qa, lz)
			atmp1 = atmpQA << uint(lz) // Q(QA + lz)

			tmp1 = addLshift32(tmp1, smmul(cLastRow[n-k-1], atmp1), 32-qa-lz)         // Q(-rshifts)
			tmp2 = addLshift32(tmp2, smmul(cFirstRow[n-k-1], atmp1), 32-qa-lz)        // Q(-rshifts)
			num = addLshift32(num, smmul(cAb[n-k], atmp1), 32-qa-lz)                  // Q(-rshifts)
			nrg = addLshift32(nrg, smmul(add32(cAb[k+1], cAf[k+1]), atmp1), 32-qa-lz) // Q(1-rshifts)
		}

		cAf[n+1] = tmp1        // Q(-rshifts)
		cAb[n+1] = tmp2        // Q(-rshifts)
		num = add32(num, tmp2) // Q(-rshifts)
		num = -num << 1        // Q(1-rshifts)

		// Calculate next reflection coefficient
		if abs32(num) < nrg {
			rcQ31 = div32VarQ(num, nrg, 31)
		} else {
			if num > 0 {
				rcQ31 = math.MaxInt32
			} else {
				rcQ31 = math.MinInt32
			}
		}

		// Update inverse prediction gain
		tmp1 = (1 << 30) - smmul(rcQ31, rcQ31)
		tmp1 = smmul(invGainQ30, tmp1) << 2
		if tmp1 <= minInvGainQ30 {
			// Max prediction gain exceeded
			tmp2 = (1 << 30) - div32VarQ(minInvGainQ30, invGainQ30, 30) // Q30
			rcQ31 = sqrtApprox(tmp2)                                    // Q15
			// Newton-Raphson iteration
			rcQ31 = (rcQ31 + div32(tmp2, rcQ31)) >> 1 // Q15
			rcQ31 <<= 16                              // Q31
			if num < 0 {
				rcQ31 = -rcQ31
			}
			invGainQ30 = minInvGainQ30
			reachedMaxGain = 1
		} else {
			invGainQ30 = tmp1
		}

		// Update AR coefficients
		for k = 0; k < (n+1)>>1; k++ {
			tmp1 = afQA[k]                                         // QA
			tmp2 = afQA[n-k-1]                                     // QA
			afQA[k] = addLshift32(tmp1, smmul(tmp2, rcQ31), 1)     // QA
			afQA[n-k-1] = addLshift32(tmp2, smmul(tmp1, rcQ31), 1) // QA
		}
		afQA[n] = rcQ31 >> (31 - qa) // QA

		if reachedMaxGain != 0 {
			// Reached max prediction gain - set remaining coefficients to zero
			for k = n + 1; k < D; k++ {
				afQA[k] = 0
			}
			break
		}

		// Update C*Af and C*Ab
		for k = 0; k <= n+1; k++ {
			tmp1 = cAf[k]                                         // Q(-rshifts)
			tmp2 = cAb[n-k+1]                                     // Q(-rshifts)
			cAf[k] = addLshift32(tmp1, smmul(tmp2, rcQ31), 1)     // Q(-rshifts)
			cAb[n-k+1] = addLshift32(tmp2, smmul(tmp1, rcQ31), 1) // Q(-rshifts)
		}
	}

	if reachedMaxGain != 0 {
		// Scale coefficients
		for k = 0; k < D; k++ {
			aQ16[k] = -rshiftRound(afQA[k], qa-16)
		}

		// Subtract energy of preceding samples from C0
		if rshifts > 0 {
			for s = 0; s < nbSubfr; s++ {
				xOffset = xPtr + s*subfrLength
				c0 -= int32(innerProd16Aligned64(x, xOffset, x, xOffset, D) >> uint(rshifts))
			}
		} else {
			for s = 0; s < nbSubfr; s++ {
				xOffset = xPtr + s*subfrLength
				c0 -= innerProdSelf(x, xOffset, D) << uint(-rshifts)
			}
		}

		// Approximate residual energy
		*resNrg = smmul(invGainQ30, c0) << 2
		*resNrgQ = int32(-rshifts)
	} else {
		// Return residual energy
		nrg = cAf[0]   // Q(-rshifts)
		tmp1 = 1 << 16 // Q16

		for k = 0; k < D; k++ {
			atmp1 = rshiftRound(afQA[k], qa-16) // Q16
			nrg = smlaww(nrg, cAf[k+1], atmp1)  // Q(-rshifts)
			tmp1 = smlaww(tmp1, atmp1, atmp1)   // Q16
			aQ16[k] = -atmp1
		}

		*resNrg = smlaww(nrg, smmul(condFac, c0), -tmp1) // Q(-rshifts)
		*resNrgQ = int32(-rshifts)
	}
}

// Helper functions (these would be implemented elsewhere in the package)

func innerProd16Aligned64(x []int16, xPtr int, y []int16, yPtr int, len int) int64 {
	// Implementation would go here
}

func clz64(x int64) int {
	// Implementation would go here
}

func smmul(a, b int32) int32 {
	// Implementation would go here
}

func mac16_16(acc, a, b int32) int32 {
	// Implementation would go here
}

func pitchXCorr(x []int16, xPtr int, y []int16, yPtr int, xcorr []int32, len, maxLag int) {
	// Implementation would go here
}

func abs32(x int32) int32 {
	// Implementation would go here
}

func min(a, b int) int {
	// Implementation would go here
}

func div32VarQ(a, b int32, q int) int32 {
	// Implementation would go here
}

func sqrtApprox(x int32) int32 {
	// Implementation would go here
}

func div32(a, b int32) int32 {
	// Implementation would go here
}

func innerProdSelf(x []int16, xPtr int, len int) int32 {
	// Implementation would go here
}
