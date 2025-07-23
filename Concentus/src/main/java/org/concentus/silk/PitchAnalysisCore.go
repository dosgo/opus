package silk

import (
	"math"
)

const (
	SCRATCH_SIZE   = 22
	SF_LENGTH_4KHZ = PE_SUBFR_LENGTH_MS * 4
	SF_LENGTH_8KHZ = PE_SUBFR_LENGTH_MS * 8
	MIN_LAG_4KHZ   = PE_MIN_LAG_MS * 4
	MIN_LAG_8KHZ   = PE_MIN_LAG_MS * 8
	MAX_LAG_4KHZ   = PE_MAX_LAG_MS * 4
	MAX_LAG_8KHZ   = PE_MAX_LAG_MS*8 - 1
	CSTRIDE_4KHZ   = MAX_LAG_4KHZ + 1 - MIN_LAG_4KHZ
	CSTRIDE_8KHZ   = MAX_LAG_8KHZ + 3 - (MIN_LAG_8KHZ - 2)
	D_COMP_MIN     = MIN_LAG_8KHZ - 3
	D_COMP_MAX     = MAX_LAG_8KHZ + 4
	D_COMP_STRIDE  = D_COMP_MAX - D_COMP_MIN
)

type SilkPEStage3Vals struct {
	Values [PE_NB_STAGE3_LAGS]int32
}

type PitchAnalysisCore struct{}

func (p *PitchAnalysisCore) PitchAnalysisCore(
	frame []int16,
	pitchOut []int32,
	lagIndex *int16,
	contourIndex *int8,
	LTPCorrQ15 *int32,
	prevLag int32,
	searchThres1Q16 int32,
	searchThres2Q13 int32,
	FsKHz int32,
	complexity int32,
	nbSubfr int32,
) int32 {
	var frame8kHz, frame4kHz []int16
	var filtState [6]int32
	var inputFramePtr []int16
	var C []int16
	var xcorr32 []int32
	var dSrch [PE_D_SRCH_LENGTH]int32
	var dComp []int16
	var CC [PE_NB_CBKS_STAGE2_EXT]int32
	var energiesSt3, crossCorrSt3 []SilkPEStage3Vals
	var LagCBptr [][]int8

	// Check for valid sampling frequency
	if FsKHz != 8 && FsKHz != 12 && FsKHz != 16 {
		panic("invalid sampling frequency")
	}

	// Check for valid complexity setting
	if complexity < SILK_PE_MIN_COMPLEX || complexity > SILK_PE_MAX_COMPLEX {
		panic("invalid complexity setting")
	}

	if searchThres1Q16 < 0 || searchThres1Q16 > (1<<16) ||
		searchThres2Q13 < 0 || searchThres2Q13 > (1<<13) {
		panic("invalid threshold values")
	}

	// Set up frame lengths max / min lag for the sampling frequency
	frameLength := (PE_LTP_MEM_LENGTH_MS + nbSubfr*PE_SUBFR_LENGTH_MS) * FsKHz
	frameLength4kHz := (PE_LTP_MEM_LENGTH_MS + nbSubfr*PE_SUBFR_LENGTH_MS) * 4
	frameLength8kHz := (PE_LTP_MEM_LENGTH_MS + nbSubfr*PE_SUBFR_LENGTH_MS) * 8
	sfLength := PE_SUBFR_LENGTH_MS * FsKHz
	minLag := PE_MIN_LAG_MS * FsKHz
	maxLag := PE_MAX_LAG_MS*FsKHz - 1

	// Resample from input sampled at Fs_kHz to 8 kHz
	frame8kHz = make([]int16, frameLength8kHz)
	switch FsKHz {
	case 16:
		var filtState [2]int32
		resampler := &Resampler{}
		resampler.Down2(filtState[:], frame8kHz, frame, frameLength)
	case 12:
		var filtState [6]int32
		resampler := &Resampler{}
		resampler.Down23(filtState[:], frame8kHz, frame, frameLength)
	case 8:
		copy(frame8kHz, frame[:frameLength8kHz])
	default:
		panic("unsupported sampling rate")
	}

	// Decimate again to 4 kHz
	var filtState2 [2]int32
	frame4kHz = make([]int16, frameLength4kHz)
	resampler := &Resampler{}
	resampler.Down2(filtState2[:], frame4kHz, frame8kHz, frameLength8kHz)

	// Low-pass filter
	for i := frameLength4kHz - 1; i > 0; i-- {
		frame4kHz[i] = int16(AddSat32(int32(frame4kHz[i]), int32(frame4kHz[i-1])))
	}

	// Scale 4 kHz signal down to prevent correlations measures from overflowing
	var energy, shift int32
	energy, shift = SumSqrShift(frame4kHz, frameLength4kHz)

	if shift > 0 {
		shift = shift >> 1
		for i := range frame4kHz {
			frame4kHz[i] = int16(int32(frame4kHz[i]) >> shift)
		}
	}

	// FIRST STAGE, operating in 4 khz
	C = make([]int16, nbSubfr*CSTRIDE_8KHZ)
	xcorr32 = make([]int32, MAX_LAG_4KHZ-MIN_LAG_4KHZ+1)
	for i := range C[:(nbSubfr>>1)*CSTRIDE_4KHZ] {
		C[i] = 0
	}

	target := frame4kHz
	targetPtr := SF_LENGTH_4KHZ * 2
	for k := int32(0); k < nbSubfr>>1; k++ {
		basis := target
		basisPtr := targetPtr - MIN_LAG_4KHZ

		pitchXCorr := &CeltPitchXCorr{}
		pitchXCorr.PitchXCorr(target, targetPtr, target, targetPtr-MAX_LAG_4KHZ, xcorr32, SF_LENGTH_8KHZ, MAX_LAG_4KHZ-MIN_LAG_4KHZ+1)

		// Calculate first vector products before loop
		crossCorr := xcorr32[MAX_LAG_4KHZ-MIN_LAG_4KHZ]
		normalizer := InnerProdSelf(target, targetPtr, SF_LENGTH_8KHZ)
		normalizer = Add32(normalizer, InnerProdSelf(basis, basisPtr, SF_LENGTH_8KHZ))
		normalizer = Add32(normalizer, SMulBB(SF_LENGTH_8KHZ, 4000))

		MatrixSet(C, k, 0, CSTRIDE_4KHZ, int16(Div32VarQ(crossCorr, normalizer, 13+1)))

		// From now on normalizer is computed recursively
		for d := MIN_LAG_4KHZ + 1; d <= MAX_LAG_4KHZ; d++ {
			basisPtr--

			crossCorr = xcorr32[MAX_LAG_4KHZ-d]

			// Add contribution of new sample and remove contribution from oldest sample
			normalizer = Add32(normalizer,
				SMulBB(int32(basis[basisPtr]), int32(basis[basisPtr]))-
					SMulBB(int32(basis[basisPtr+SF_LENGTH_8KHZ]), int32(basis[basisPtr+SF_LENGTH_8KHZ])))

			MatrixSet(C, k, d-MIN_LAG_4KHZ, CSTRIDE_4KHZ, int16(Div32VarQ(crossCorr, normalizer, 13+1)))
		}
		// Update target pointer
		targetPtr += SF_LENGTH_8KHZ
	}

	// Combine two subframes into single correlation measure and apply short-lag bias
	if nbSubfr == PE_MAX_NB_SUBFR {
		for i := MAX_LAG_4KHZ; i >= MIN_LAG_4KHZ; i-- {
			sum := int32(MatrixGet(C, 0, i-MIN_LAG_4KHZ, CSTRIDE_4KHZ)) +
				int32(MatrixGet(C, 1, i-MIN_LAG_4KHZ, CSTRIDE_4KHZ))
			sum = SMLAWB(sum, sum, int32(-i)<<4)
			C[i-MIN_LAG_4KHZ] = int16(sum)
		}
	} else {
		// Only short-lag bias
		for i := MAX_LAG_4KHZ; i >= MIN_LAG_4KHZ; i-- {
			sum := int32(C[i-MIN_LAG_4KHZ]) << 1
			sum = SMLAWB(sum, sum, int32(-i)<<4)
			C[i-MIN_LAG_4KHZ] = int16(sum)
		}
	}

	// Sort
	lengthDSrch := 4 + (complexity << 1)
	if 3*lengthDSrch > PE_D_SRCH_LENGTH {
		panic("invalid length_d_srch")
	}

	sort := &Sort{}
	sort.InsertionSortDecreasingInt16(C, dSrch[:], CSTRIDE_4KHZ, lengthDSrch)

	// Escape if correlation is very low already here
	Cmax := int32(C[0])
	if Cmax < int32(0.2*float64(1<<14)+0.5) {
		for i := range pitchOut {
			pitchOut[i] = 0
		}
		*LTPCorrQ15 = 0
		*lagIndex = 0
		*contourIndex = 0
		return 1
	}

	threshold := SMulWB(searchThres1Q16, Cmax)
	for i := int32(0); i < lengthDSrch; i++ {
		if C[i] > threshold {
			dSrch[i] = (dSrch[i] + MIN_LAG_4KHZ) << 1
		} else {
			lengthDSrch = i
			break
		}
	}
	if lengthDSrch <= 0 {
		panic("length_d_srch must be positive")
	}

	dComp = make([]int16, D_COMP_STRIDE)
	for i := D_COMP_MIN; i < D_COMP_MAX; i++ {
		dComp[i-D_COMP_MIN] = 0
	}
	for i := int32(0); i < lengthDSrch; i++ {
		dComp[dSrch[i]-D_COMP_MIN] = 1
	}

	// Convolution
	for i := D_COMP_MAX - 1; i >= MIN_LAG_8KHZ; i-- {
		dComp[i-D_COMP_MIN] += int16(int32(dComp[i-1-D_COMP_MIN]) + int32(dComp[i-2-D_COMP_MIN]))
	}

	lengthDSrch = 0
	for i := MIN_LAG_8KHZ; i < MAX_LAG_8KHZ+1; i++ {
		if dComp[i+1-D_COMP_MIN] > 0 {
			dSrch[lengthDSrch] = i
			lengthDSrch++
		}
	}

	// Convolution
	for i := D_COMP_MAX - 1; i >= MIN_LAG_8KHZ; i-- {
		dComp[i-D_COMP_MIN] += int16(int32(dComp[i-1-D_COMP_MIN]) + int32(dComp[i-2-D_COMP_MIN]) + int32(dComp[i-3-D_COMP_MIN]))
	}

	lengthDComp := int32(0)
	for i := MIN_LAG_8KHZ; i < D_COMP_MAX; i++ {
		if dComp[i-D_COMP_MIN] > 0 {
			dComp[lengthDComp] = int16(i - 2)
			lengthDComp++
		}
	}

	// SECOND STAGE, operating at 8 kHz, on lag sections with high correlation
	// Scale signal down to avoid correlations measures from overflowing
	energy, shift = SumSqrShift(frame8kHz, frameLength8kHz)
	if shift > 0 {
		shift = shift >> 1
		for i := range frame8kHz {
			frame8kHz[i] = int16(int32(frame8kHz[i]) >> shift)
		}
	}

	// Find energy of each subframe projected onto its history, for a range of delays
	for i := range C[:nbSubfr*CSTRIDE_8KHZ] {
		C[i] = 0
	}

	target = frame8kHz
	targetPtr = PE_LTP_MEM_LENGTH_MS * 8
	for k := int32(0); k < nbSubfr; k++ {
		energyTarget := Add32(InnerProdSelf(target, targetPtr, SF_LENGTH_8KHZ), 1)
		for j := int32(0); j < lengthDComp; j++ {
			d := dComp[j]
			basis := target
			basisPtr := targetPtr - d

			crossCorr := InnerProd(target, targetPtr, basis, basisPtr, SF_LENGTH_8KHZ)
			if crossCorr > 0 {
				energyBasis := InnerProdSelf(basis, basisPtr, SF_LENGTH_8KHZ)
				MatrixSet(C, k, d-(MIN_LAG_8KHZ-2), CSTRIDE_8KHZ,
					int16(Div32VarQ(crossCorr, Add32(energyTarget, energyBasis), 13+1)))
			} else {
				MatrixSet(C, k, d-(MIN_LAG_8KHZ-2), CSTRIDE_8KHZ, 0)
			}
		}
		targetPtr += SF_LENGTH_8KHZ
	}

	// search over lag range and lags codebook
	CCmax := math.MinInt32
	CCmaxB := math.MinInt32
	CBimax := 0
	lag := -1

	var prevLagLog2Q7 int32
	if prevLag > 0 {
		if FsKHz == 12 {
			prevLag = Div32_16(Lsh(prevLag, 1), 3)
		} else if FsKHz == 16 {
			prevLag = Rsh(prevLag, 1)
		}
		prevLagLog2Q7 = Lin2Log(int32(prevLag))
	} else {
		prevLagLog2Q7 = 0
	}

	if searchThres2Q13 != Sat16(searchThres2Q13) {
		panic("search_thres2_Q13 must be saturated")
	}

	// Set up stage 2 codebook based on number of subframes
	if nbSubfr == PE_MAX_NB_SUBFR {
		LagCBptr = CB_LAGS_STAGE2
		if FsKHz == 8 && complexity > SILK_PE_MIN_COMPLEX {
			nbCbkSearch = PE_NB_CBKS_STAGE2_EXT
		} else {
			nbCbkSearch = PE_NB_CBKS_STAGE2
		}
	} else {
		LagCBptr = CB_LAGS_STAGE2_10_MS
		nbCbkSearch = PE_NB_CBKS_STAGE2_10MS
	}

	for k := int32(0); k < lengthDSrch; k++ {
		d := dSrch[k]
		for j := 0; j < nbCbkSearch; j++ {
			CC[j] = 0
			for i := int32(0); i < nbSubfr; i++ {
				dSubfr := d + int32(LagCBptr[i][j])
				CC[j] += int32(MatrixGet(C, i, dSubfr-(MIN_LAG_8KHZ-2), CSTRIDE_8KHZ))
			}
		}

		// Find best codebook
		CCmaxNew := math.MinInt32
		CBimaxNew := 0
		for i := 0; i < nbCbkSearch; i++ {
			if CC[i] > CCmaxNew {
				CCmaxNew = CC[i]
				CBimaxNew = i
			}
		}

		// Bias towards shorter lags
		lagLog2Q7 := Lin2Log(d)
		if lagLog2Q7 != Sat16(lagLog2Q7) {
			panic("lag_log2_Q7 must be saturated")
		}

		shortLagBias := PE_SHORTLAG_BIAS * (1 << 13)
		if nbSubfr*shortLagBias != Sat16(nbSubfr*shortLagBias) {
			panic("short lag bias must be saturated")
		}

		CCmaxNewB := CCmaxNew - Rsh(SMulBB(nbSubfr*shortLagBias, lagLog2Q7), 7)

		// Bias towards previous lag
		prevLagBias := PE_PREVLAG_BIAS * (1 << 13)
		if nbSubfr*prevLagBias != Sat16(nbSubfr*prevLagBias) {
			panic("prev lag bias must be saturated")
		}

		var prevLagBiasQ13 int32
		if prevLag > 0 {
			deltaLagLog2SqrQ7 := lagLog2Q7 - prevLagLog2Q7
			if deltaLagLog2SqrQ7 != Sat16(deltaLagLog2SqrQ7) {
				panic("delta_lag_log2_sqr_Q7 must be saturated")
			}
			deltaLagLog2SqrQ7 = Rsh(SMulBB(deltaLagLog2SqrQ7, deltaLagLog2SqrQ7), 7)
			prevLagBiasQ13 = Rsh(SMulBB(nbSubfr*prevLagBias, *LTPCorrQ15), 15)
			prevLagBiasQ13 = Div32(Mul(prevLagBiasQ13, deltaLagLog2SqrQ7), deltaLagLog2SqrQ7+(0.5*(1<<7)+0.5))
			CCmaxNewB -= prevLagBiasQ13
		}

		if CCmaxNewB > CCmaxB &&
			CCmaxNew > SMulBB(nbSubfr, searchThres2Q13) &&
			CB_LAGS_STAGE2[0][CBimaxNew] <= MIN_LAG_8KHZ {
			CCmaxB = CCmaxNewB
			CCmax = CCmaxNew
			lag = d
			CBimax = CBimaxNew
		}
	}

	if lag == -1 {
		// No suitable candidate found
		for i := range pitchOut {
			pitchOut[i] = 0
		}
		*LTPCorrQ15 = 0
		*lagIndex = 0
		*contourIndex = 0
		return 1
	}

	// Output normalized correlation
	*LTPCorrQ15 = Lsh(Div32_16(CCmax, nbSubfr), 2)
	if *LTPCorrQ15 < 0 {
		panic("LTPCorr_Q15 must be non-negative")
	}

	if FsKHz > 8 {
		var scratchMem []int16
		// Scale input signal down to avoid correlations measures from overflowing
		energy, shift = SumSqrShift(frame, frameLength)
		if shift > 0 {
			scratchMem = make([]int16, frameLength)
			shift = shift >> 1
			for i := range frame {
				scratchMem[i] = int16(int32(frame[i]) >> shift)
			}
			inputFramePtr = scratchMem
		} else {
			inputFramePtr = frame
		}

		// Search in original signal
		CBimaxOld := CBimax
		// Compensate for decimation
		if lag != Sat16(lag) {
			panic("lag must be saturated")
		}

		switch FsKHz {
		case 12:
			lag = Rsh(SMulBB(lag, 3), 1)
		case 16:
			lag = Lsh(lag, 1)
		default:
			lag = SMulBB(lag, 3)
		}

		lag = LimitInt(lag, minLag, maxLag)
		startLag := MaxInt(lag-2, minLag)
		endLag := MinInt(lag+2, maxLag)
		lagNew := lag // to avoid undefined lag
		CBimax = 0    // to avoid undefined lag

		CCmax = math.MinInt32
		// pitch lags according to second stage
		for k := int32(0); k < nbSubfr; k++ {
			pitchOut[k] = lag + 2*int32(CB_LAGS_STAGE2[k][CBimaxOld])
		}

		// Set up codebook parameters according to complexity setting and frame length
		if nbSubfr == PE_MAX_NB_SUBFR {
			nbCbkSearch = NB_CBK_SEARCHS_STAGE3[complexity]
			LagCBptr = CB_LAGS_STAGE3
		} else {
			nbCbkSearch = PE_NB_CBKS_STAGE3_10MS
			LagCBptr = CB_LAGS_STAGE3_10_MS
		}

		// Calculate the correlations and energies needed in stage 3
		energiesSt3 = make([]SilkPEStage3Vals, nbSubfr*nbCbkSearch)
		crossCorrSt3 = make([]SilkPEStage3Vals, nbSubfr*nbCbkSearch)
		for i := range energiesSt3 {
			energiesSt3[i] = SilkPEStage3Vals{}
			crossCorrSt3[i] = SilkPEStage3Vals{}
		}

		p := &PitchAnalysisCore{}
		p.CalcCorrSt3(crossCorrSt3, inputFramePtr, startLag, sfLength, nbSubfr, complexity)
		p.CalcEnergySt3(energiesSt3, inputFramePtr, startLag, sfLength, nbSubfr, complexity)

		lagCounter := 0
		if lag != Sat16(lag) {
			panic("lag must be saturated")
		}
		contourBiasQ15 := Div32_16(PE_FLATCONTOUR_BIAS*(1<<15)+0.5, lag)

		target = inputFramePtr
		targetPtr = PE_LTP_MEM_LENGTH_MS * FsKHz
		energyTarget := Add32(InnerProdSelf(target, targetPtr, nbSubfr*sfLength), 1)
		for d := startLag; d <= endLag; d++ {
			for j := 0; j < nbCbkSearch; j++ {
				crossCorr := int32(0)
				energy := energyTarget
				for k := int32(0); k < nbSubfr; k++ {
					crossCorr = Add32(crossCorr, MatrixGet(crossCorrSt3, k, j, nbCbkSearch).Values[lagCounter])
					energy = Add32(energy, MatrixGet(energiesSt3, k, j, nbCbkSearch).Values[lagCounter])
					if energy < 0 {
						panic("energy must be non-negative")
					}
				}
				if crossCorr > 0 {
					CCmaxNew := Div32VarQ(crossCorr, energy, 13+1)
					// Reduce depending on flatness of contour
					diff := math.MaxInt16 - Mul(contourBiasQ15, int32(j))
					if diff != Sat16(diff) {
						panic("diff must be saturated")
					}
					CCmaxNew = SMulWB(CCmaxNew, diff)
				} else {
					CCmaxNew = 0
				}

				if CCmaxNew > CCmax && (d+int32(CB_LAGS_STAGE3[0][j])) <= maxLag {
					CCmax = CCmaxNew
					lagNew = d
					CBimax = j
				}
			}
			lagCounter++
		}

		for k := int32(0); k < nbSubfr; k++ {
			pitchOut[k] = lagNew + int32(LagCBptr[k][CBimax])
			pitchOut[k] = LimitInt(pitchOut[k], minLag, PE_MAX_LAG_MS*FsKHz)
		}
		*lagIndex = int16(lagNew - minLag)
		*contourIndex = int8(CBimax)
	} else {
		// Fs_kHz == 8
		// Save Lags
		for k := int32(0); k < nbSubfr; k++ {
			pitchOut[k] = lag + int32(LagCBptr[k][CBimax])
			pitchOut[k] = LimitInt(pitchOut[k], MIN_LAG_8KHZ, PE_MAX_LAG_MS*8)
		}
		*lagIndex = int16(lag - MIN_LAG_8KHZ)
		*contourIndex = int8(CBimax)
	}

	if *lagIndex < 0 {
		panic("lag_index must be non-negative")
	}

	// return as voiced
	return 0
}

func (p *PitchAnalysisCore) CalcCorrSt3(
	crossCorrSt3 []SilkPEStage3Vals,
	frame []int16,
	startLag int32,
	sfLength int32,
	nbSubfr int32,
	complexity int32,
) {
	if complexity < SILK_PE_MIN_COMPLEX || complexity > SILK_PE_MAX_COMPLEX {
		panic("invalid complexity setting")
	}

	var LagRangePtr [][]int8
	var LagCBptr [][]int8
	var nbCbkSearch int32

	if nbSubfr == PE_MAX_NB_SUBFR {
		LagRangePtr = LAG_RANGE_STAGE3[complexity]
		LagCBptr = CB_LAGS_STAGE3
		nbCbkSearch = NB_CBK_SEARCHS_STAGE3[complexity]
	} else {
		if nbSubfr != PE_MAX_NB_SUBFR>>1 {
			panic("invalid number of subframes")
		}
		LagRangePtr = LAG_RANGE_STAGE3_10_MS
		LagCBptr = CB_LAGS_STAGE3_10_MS
		nbCbkSearch = PE_NB_CBKS_STAGE3_10MS
	}

	scratchMem := make([]int32, SCRATCH_SIZE)
	xcorr32 := make([]int32, SCRATCH_SIZE)

	targetPtr := Lsh(sfLength, 2)
	for k := int32(0); k < nbSubfr; k++ {
		lagCounter := 0

		// Calculate the correlations for each subframe
		lagLow := LagRangePtr[k][0]
		lagHigh := LagRangePtr[k][1]
		if lagHigh-lagLow+1 > SCRATCH_SIZE {
			panic("scratch size too small")
		}

		pitchXCorr := &CeltPitchXCorr{}
		pitchXCorr.PitchXCorr(frame, targetPtr, frame, targetPtr-startLag-int32(lagHigh), xcorr32, sfLength, int32(lagHigh-lagLow+1))

		for j := lagLow; j <= lagHigh; j++ {
			if lagCounter >= SCRATCH_SIZE {
				panic("lag counter overflow")
			}
			scratchMem[lagCounter] = xcorr32[int32(lagHigh-j)]
			lagCounter++
		}

		delta := LagRangePtr[k][0]
		for i := int32(0); i < nbCbkSearch; i++ {
			idx := int32(LagCBptr[k][i]) - int32(delta)
			for j := 0; j < PE_NB_STAGE3_LAGS; j++ {
				if idx+int32(j) >= SCRATCH_SIZE || idx+int32(j) >= int32(lagCounter) {
					panic("index out of bounds")
				}
				MatrixGet(crossCorrSt3, k, i, nbCbkSearch).Values[j] = scratchMem[idx+int32(j)]
			}
		}
		targetPtr += sfLength
	}
}

func (p *PitchAnalysisCore) CalcEnergySt3(
	energiesSt3 []SilkPEStage3Vals,
	frame []int16,
	startLag int32,
	sfLength int32,
	nbSubfr int32,
	complexity int32,
) {
	if complexity < SILK_PE_MIN_COMPLEX || complexity > SILK_PE_MAX_COMPLEX {
		panic("invalid complexity setting")
	}

	var LagRangePtr [][]int8
	var LagCBptr [][]int8
	var nbCbkSearch int32

	if nbSubfr == PE_MAX_NB_SUBFR {
		LagRangePtr = LAG_RANGE_STAGE3[complexity]
		LagCBptr = CB_LAGS_STAGE3
		nbCbkSearch = NB_CBK_SEARCHS_STAGE3[complexity]
	} else {
		if nbSubfr != PE_MAX_NB_SUBFR>>1 {
			panic("invalid number of subframes")
		}
		LagRangePtr = LAG_RANGE_STAGE3_10_MS
		LagCBptr = CB_LAGS_STAGE3_10_MS
		nbCbkSearch = PE_NB_CBKS_STAGE3_10MS
	}

	scratchMem := make([]int32, SCRATCH_SIZE)

	targetPtr := Lsh(sfLength, 2)
	for k := int32(0); k < nbSubfr; k++ {
		lagCounter := 0

		// Calculate the energy for first lag
		basisPtr := targetPtr - (startLag + int32(LagRangePtr[k][0]))
		energy := InnerProdSelf(frame, basisPtr, sfLength)
		if energy < 0 {
			panic("energy must be non-negative")
		}
		scratchMem[lagCounter] = energy
		lagCounter++

		lagDiff := int32(LagRangePtr[k][1] - LagRangePtr[k][0] + 1)
		for i := int32(1); i < lagDiff; i++ {
			// remove part outside new window
			energy -= SMulBB(int32(frame[basisPtr+sfLength-i]), int32(frame[basisPtr+sfLength-i]))
			if energy < 0 {
				panic("energy must be non-negative")
			}

			// add part that comes into window
			energy = AddSat32(energy, SMulBB(int32(frame[basisPtr-i]), int32(frame[basisPtr-i])))
			if energy < 0 {
				panic("energy must be non-negative")
			}
			if lagCounter >= SCRATCH_SIZE {
				panic("lag counter overflow")
			}
			scratchMem[lagCounter] = energy
			lagCounter++
		}

		delta := LagRangePtr[k][0]
		for i := int32(0); i < nbCbkSearch; i++ {
			idx := int32(LagCBptr[k][i]) - int32(delta)
			for j := 0; j < PE_NB_STAGE3_LAGS; j++ {
				if idx+int32(j) >= SCRATCH_SIZE || idx+int32(j) >= int32(lagCounter) {
					panic("index out of bounds")
				}
				MatrixGet(energiesSt3, k, i, nbCbkSearch).Values[j] = scratchMem[idx+int32(j)]
				if MatrixGet(energiesSt3, k, i, nbCbkSearch).Values[j] < 0 {
					panic("energy must be non-negative")
				}
			}
		}
		targetPtr += sfLength
	}
}
