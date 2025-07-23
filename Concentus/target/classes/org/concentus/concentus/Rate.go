package concentus

// Rate contains functions for bit allocation and rate control in Opus audio codec
type Rate struct{}

// LOG2_FRAC_TABLE is a lookup table for log2 fractional parts
var LOG2_FRAC_TABLE = []byte{
	0,
	8, 13,
	16, 19, 21, 23,
	24, 26, 27, 28, 29, 30, 31, 32,
	32, 33, 34, 34, 35, 36, 36, 37, 37,
}

const ALLOC_STEPS = 6

// GetPulses calculates the number of pulses from an index
func (r *Rate) GetPulses(i int) int {
	if i < 8 {
		return i
	}
	return (8 + (i & 7)) << ((i >> 3) - 1)
}

// Bits2Pulses converts bits to pulses for a given band
func (r *Rate) Bits2Pulses(m *CeltMode, band, LM, bits int) int {
	LM++
	cache := m.cache.bits
	cachePtr := m.cache.index[LM*m.nbEBands+band]

	lo := 0
	hi := int(cache[cachePtr])
	bits--

	for i := 0; i < CeltConstants.LOG_MAX_PSEUDO; i++ {
		mid := (lo + hi + 1) >> 1
		if int(cache[cachePtr+mid]) >= bits {
			hi = mid
		} else {
			lo = mid
		}
	}

	if bits-int(cache[cachePtr+lo]) <= int(cache[cachePtr+hi])-bits {
		return lo
	}
	return hi
}

// Pulses2Bits converts pulses back to bits for a given band
func (r *Rate) Pulses2Bits(m *CeltMode, band, LM, pulses int) int {
	LM++
	if pulses == 0 {
		return 0
	}
	return int(m.cache.bits[m.cache.index[LM*m.nbEBands+band]+pulses]) + 1
}

// InterpBits2Pulses performs interpolation between bit allocations
func (r *Rate) InterpBits2Pulses(m *CeltMode, start, end, skipStart int,
	bits1, bits2, thresh, cap []int, total int, balance *IntBox, skipRsv int,
	intensity *IntBox, intensityRsv int, dualStereo *IntBox, dualStereoRsv int,
	bits, ebits, finePriority []int, C, LM int, ec *EntropyCoder, encode, prev, signalBandwidth int) int {

	allocFloor := C << EntropyCoder.BITRES
	stereo := 0
	if C > 1 {
		stereo = 1
	}

	logM := LM << EntropyCoder.BITRES
	lo := 0
	hi := 1 << ALLOC_STEPS

	// Binary search to find optimal allocation
	for i := 0; i < ALLOC_STEPS; i++ {
		mid := (lo + hi) >> 1
		psum := 0
		done := 0
		for j := end - 1; j >= start; j-- {
			tmp := bits1[j] + (mid * bits2[j] >> ALLOC_STEPS)
			if tmp >= thresh[j] || done != 0 {
				done = 1
				// Don't allocate more than we can actually use
				psum += min(tmp, cap[j])
			} else if tmp >= allocFloor {
				psum += allocFloor
			}
		}
		if psum > total {
			hi = mid
		} else {
			lo = mid
		}
	}

	psum := 0
	done := 0
	for j := end - 1; j >= start; j-- {
		tmp := bits1[j] + (lo * bits2[j] >> ALLOC_STEPS)
		if tmp < thresh[j] && done == 0 {
			if tmp >= allocFloor {
				tmp = allocFloor
			} else {
				tmp = 0
			}
		} else {
			done = 1
		}

		// Don't allocate more than we can actually use
		tmp = min(tmp, cap[j])
		bits[j] = tmp
		psum += tmp
	}

	// Decide which bands to skip, working backwards from the end
	codedBands := end
	for {
		j := codedBands - 1
		// Never skip the first band or a boosted band
		if j <= skipStart {
			// Give the bit we reserved to end skipping back
			total += skipRsv
			break
		}

		// Figure out leftover bits
		left := total - psum
		perCoeff := udiv(left, m.eBands[codedBands]-m.eBands[start])
		left -= (m.eBands[codedBands] - m.eBands[start]) * perCoeff
		rem := max(left-(m.eBands[j]-m.eBands[start]), 0)
		bandWidth := m.eBands[codedBands] - m.eBands[j]
		bandBits := bits[j] + perCoeff*bandWidth + rem

		// Only code a skip decision if above threshold
		if bandBits >= max(thresh[j], allocFloor+(1<<EntropyCoder.BITRES)) {
			if encode != 0 {
				// Choose threshold with hysteresis
				if codedBands <= start+2 || (bandBits > ((map[bool]int{true: 7, false: 9}[j < prev])*bandWidth<<LM<<EntropyCoder.BITRES)>>4 && j <= signalBandwidth) {
					ec.EncBitLogp(1, 1)
					break
				}
				ec.EncBitLogp(0, 1)
			} else if ec.DecBitLogp(1) != 0 {
				break
			}
			// We used a bit to skip this band
			psum += 1 << EntropyCoder.BITRES
			bandBits -= 1 << EntropyCoder.BITRES
		}

		// Reclaim bits originally allocated to this band
		psum -= bits[j] + intensityRsv
		if intensityRsv > 0 {
			intensityRsv = int(LOG2_FRAC_TABLE[j-start])
		}
		psum += intensityRsv

		if bandBits >= allocFloor {
			// Use fine energy bit per channel
			psum += allocFloor
			bits[j] = allocFloor
		} else {
			// Band gets nothing
			bits[j] = 0
		}
		codedBands--
	}

	// Code intensity and dual stereo parameters
	if intensityRsv > 0 {
		if encode != 0 {
			intensity.Val = min(intensity.Val, codedBands)
			ec.EncUint(uint32(intensity.Val-start), uint32(codedBands+1-start))
		} else {
			intensity.Val = start + int(ec.DecUint(uint32(codedBands+1-start)))
		}
	} else {
		intensity.Val = 0
	}

	if intensity.Val <= start {
		total += dualStereoRsv
		dualStereoRsv = 0
	}
	if dualStereoRsv > 0 {
		if encode != 0 {
			ec.EncBitLogp(dualStereo.Val, 1)
		} else {
			dualStereo.Val = ec.DecBitLogp(1)
		}
	} else {
		dualStereo.Val = 0
	}

	// Allocate remaining bits
	left := total - psum
	perCoeff := udiv(left, m.eBands[codedBands]-m.eBands[start])
	left -= (m.eBands[codedBands] - m.eBands[start]) * perCoeff
	for j := start; j < codedBands; j++ {
		bits[j] += perCoeff * (m.eBands[j+1] - m.eBands[j])
	}
	for j := start; j < codedBands; j++ {
		tmp := min(left, m.eBands[j+1]-m.eBands[j])
		bits[j] += tmp
		left -= tmp
	}

	balance := 0
	for j := start; j < codedBands; j++ {
		N0 := m.eBands[j+1] - m.eBands[j]
		N := N0 << LM
		bit := bits[j] + balance

		if N > 1 {
			excess := max(bit-cap[j], 0)
			bits[j] = bit - excess

			// Compensate for extra DoF in stereo
			den := C * N
			if C == 2 && N > 2 && dualStereo.Val == 0 && j < intensity.Val {
				den++
			}

			NClogN := den * (m.logN[j] + logM)

			// Offset for fine bits
			offset := (NClogN >> 1) - den*CeltConstants.FINE_OFFSET

			if N == 2 {
				offset += den << EntropyCoder.BITRES >> 2
			}

			// Adjust offset for allocating additional fine bits
			if bits[j]+offset < den*2<<EntropyCoder.BITRES {
				offset += NClogN >> 2
			} else if bits[j]+offset < den*3<<EntropyCoder.BITRES {
				offset += NClogN >> 3
			}

			// Divide with rounding
			ebits[j] = max(0, bits[j]+offset+(den<<(EntropyCoder.BITRES-1)))
			ebits[j] = udiv(ebits[j], den) >> EntropyCoder.BITRES

			// Make sure not to bust
			if C*ebits[j] > (bits[j] >> EntropyCoder.BITRES) {
				ebits[j] = bits[j] >> stereo >> EntropyCoder.BITRES
			}

			// Cap at maximum fine bits
			ebits[j] = min(ebits[j], CeltConstants.MAX_FINE_BITS)

			// Set fine priority
			finePriority[j] = 0
			if ebits[j]*(den<<EntropyCoder.BITRES) >= bits[j]+offset {
				finePriority[j] = 1
			}

			// Remove allocated fine bits
			bits[j] -= C * ebits[j] << EntropyCoder.BITRES
		} else {
			// For N=1, all bits go to fine energy except sign bit
			excess := max(0, bit-(C<<EntropyCoder.BITRES))
			bits[j] = bit - excess
			ebits[j] = 0
			finePriority[j] = 1
		}

		// Rebalance fine energy
		if excess > 0 {
			extraFine := min(excess>>(stereo+EntropyCoder.BITRES), CeltConstants.MAX_FINE_BITS-ebits[j])
			ebits[j] += extraFine
			extraBits := extraFine * C << EntropyCoder.BITRES
			if extraBits >= excess-balance {
				finePriority[j] = 1
			} else {
				finePriority[j] = 0
			}
			excess -= extraBits
		}
		balance = excess
	}

	// Save remaining bits for rebalancing
	balance.Val = balance

	// Skipped bands use all bits for fine energy
	for ; j < end; j++ {
		ebits[j] = bits[j] >> stereo >> EntropyCoder.BITRES
		bits[j] = 0
		if ebits[j] < 1 {
			finePriority[j] = 1
		} else {
			finePriority[j] = 0
		}
	}

	return codedBands
}

// ComputeAllocation calculates the optimal bit allocation
func (r *Rate) ComputeAllocation(m *CeltMode, start, end int, offsets, cap []int, allocTrim int,
	intensity, dualStereo *IntBox, total int, balance *IntBox, pulses, ebits, finePriority []int,
	C, LM int, ec *EntropyCoder, encode, prev, signalBandwidth int) int {

	total = max(total, 0)
	len := m.nbEBands
	skipStart := start

	// Reserve bit for signaling end of skipped bands
	skipRsv := 0
	if total >= 1<<EntropyCoder.BITRES {
		skipRsv = 1 << EntropyCoder.BITRES
	}
	total -= skipRsv

	// Reserve bits for intensity and dual stereo
	intensityRsv := 0
	dualStereoRsv := 0
	if C == 2 {
		intensityRsv = int(LOG2_FRAC_TABLE[end-start])
		if intensityRsv > total {
			intensityRsv = 0
		} else {
			total -= intensityRsv
			if total >= 1<<EntropyCoder.BITRES {
				dualStereoRsv = 1 << EntropyCoder.BITRES
			}
			total -= dualStereoRsv
		}
	}

	bits1 := make([]int, len)
	bits2 := make([]int, len)
	thresh := make([]int, len)
	trimOffset := make([]int, len)

	for j := start; j < end; j++ {
		// Threshold below which no PVQ bits are allocated
		thresh[j] = max((C)<<EntropyCoder.BITRES, (3*(m.eBands[j+1]-m.eBands[j])<<LM<<EntropyCoder.BITRES)>>4)
		// Tilt of allocation curve
		trimOffset[j] = C * (m.eBands[j+1] - m.eBands[j]) * (allocTrim - 5 - LM) * (end - j - 1) *
			(1 << (LM + EntropyCoder.BITRES)) >> 6
		// Less resolution for single-coefficient bands
		if (m.eBands[j+1]-m.eBands[j])<<LM == 1 {
			trimOffset[j] -= C << EntropyCoder.BITRES
		}
	}

	lo := 1
	hi := m.nbAllocVectors - 1
	for lo <= hi {
		done := 0
		psum := 0
		mid := (lo + hi) >> 1
		for j := end - 1; j >= start; j-- {
			N := m.eBands[j+1] - m.eBands[j]
			bitsj := C * N * m.allocVectors[mid*len+j] << LM >> 2

			if bitsj > 0 {
				bitsj = max(0, bitsj+trimOffset[j])
			}
			bitsj += offsets[j]

			if bitsj >= thresh[j] || done != 0 {
				done = 1
				// Don't allocate more than we can use
				psum += min(bitsj, cap[j])
			} else if bitsj >= C<<EntropyCoder.BITRES {
				psum += C << EntropyCoder.BITRES
			}
		}
		if psum > total {
			hi = mid - 1
		} else {
			lo = mid + 1
		}
	}

	hi = lo
	lo--

	for j := start; j < end; j++ {
		N := m.eBands[j+1] - m.eBands[j]
		bits1j := C * N * m.allocVectors[lo*len+j] << LM >> 2
		bits2j := 0
		if hi < m.nbAllocVectors {
			bits2j = C * N * m.allocVectors[hi*len+j] << LM >> 2
		} else {
			bits2j = cap[j]
		}

		if bits1j > 0 {
			bits1j = max(0, bits1j+trimOffset[j])
		}
		if bits2j > 0 {
			bits2j = max(0, bits2j+trimOffset[j])
		}
		if lo > 0 {
			bits1j += offsets[j]
		}
		bits2j += offsets[j]
		if offsets[j] > 0 {
			skipStart = j
		}
		bits2j = max(0, bits2j-bits1j)
		bits1[j] = bits1j
		bits2[j] = bits2j
	}

	codedBands := r.InterpBits2Pulses(m, start, end, skipStart, bits1, bits2, thresh, cap,
		total, balance, skipRsv, intensity, intensityRsv, dualStereo, dualStereoRsv,
		pulses, ebits, finePriority, C, LM, ec, encode, prev, signalBandwidth)

	return codedBands
}
