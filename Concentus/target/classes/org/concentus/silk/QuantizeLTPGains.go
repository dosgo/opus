
package silk

// QuantizeLTPGains quantizes the LTP (Long-Term Prediction) gains using vector quantization.
// This is a direct translation from Java to Go with idiomatic Go practices applied.
func QuantizeLTPGains(
	B_Q14 []int16,          // I/O: (un)quantized LTP gains [MAX_NB_SUBFR * LTP_ORDER]
	cbkIndex []uint8,       // O: Codebook Index [MAX_NB_SUBFR]
	periodicityIndex *uint8, // O: Periodicity Index
	sumLogGainQ7 *int32,    // I/O: Cumulative max prediction gain
	W_Q18 []int32,          // I: Error Weights in Q18 [MAX_NB_SUBFR * LTP_ORDER * LTP_ORDER]
	muQ9 int32,             // I: Mu value (R/D tradeoff)
	lowComplexity bool,     // I: Flag for low complexity
	nbSubfr int,            // I: number of subframes
) {
	// Translation notes:
	// 1. Go doesn't have BoxedValue types like Java, so we use pointers for output parameters
	// 2. Go slices are used instead of Java arrays for more flexible memory management
	// 3. Constants are defined in the silk package
	// 4. Error handling is implicit in Go (no exceptions)
	// 5. Go's stricter type system requires explicit type conversions

	var (
		j, k, cbkSize int
		tempIdx       [MAX_NB_SUBFR]uint8
		clPtrQ5       []int16
		cbkPtrQ7      [][]int16
		cbkGainPtrQ7  []int16
		bQ14Ptr       int
		WQ18Ptr       int
		rateDistQ14Subfr, rateDistQ14, minRateDistQ14 int32
		sumLogGainTmpQ7, bestSumLogGainQ7, maxGainQ7, gainQ7 int32
	)

	// Initialize minimum rate distortion to maximum possible value
	minRateDistQ14 = 1<<31 - 1 // Equivalent to Java's Integer.MAX_VALUE
	bestSumLogGainQ7 = 0

	// Iterate over different codebooks with different rates/distortions, and choose best
	for k = 0; k < 3; k++ {
		// Safety margin for pitch gain control
		gainSafety := int32(0.4 * float64(1<<7) + 0.5) // SILK_CONST(0.4f, 7)

		clPtrQ5 = LTPGainBITSQ5Ptrs[k]
		cbkPtrQ7 = LTPVQPtrsQ7[k]
		cbkGainPtrQ7 = LTPVQGainPtrsQ7[k]
		cbkSize = LTPVQSizes[k]

		// Set up pointer to first subframe
		WQ18Ptr = 0
		bQ14Ptr = 0

		rateDistQ14 = 0
		sumLogGainTmpQ7 = *sumLogGainQ7

		for j = 0; j < nbSubfr; j++ {
			// Calculate maximum gain for this subframe
			maxGainQ7 = Log2lin((int32(MAX_SUM_LOG_GAIN_DB/6.0*(1<<7)+0.5) - sumLogGainTmpQ7) +
				int32(7*(1<<7)+0.5) - gainSafety

			// Vector quantization with error weighting
			tempIdx[j], rateDistQ14Subfr, gainQ7 = VQWMatEC(
				B_Q14[bQ14Ptr:],            // Input vector to be quantized
				W_Q18[WQ18Ptr:],             // Weighting matrix
				cbkPtrQ7,                    // Codebook
				cbkGainPtrQ7,                // Codebook effective gains
				clPtrQ5,                     // Code length for each codebook vector
				muQ9,                        // Tradeoff between weighted error and rate
				maxGainQ7,                   // Maximum sum of absolute LTP coefficients
				cbkSize,                     // Number of vectors in codebook
			)

			rateDistQ14 = AddPOSSat32(rateDistQ14, rateDistQ14Subfr)
			sumLogGainTmpQ7 = max32(0, sumLogGainTmpQ7+
				Lin2log(gainSafety+gainQ7)-int32(7*(1<<7)+0.5))

			bQ14Ptr += LTP_ORDER
			WQ18Ptr += LTP_ORDER * LTP_ORDER
		}

		// Avoid never finding a codebook
		rateDistQ14 = min32(1<<31-2, rateDistQ14) // Equivalent to Integer.MAX_VALUE-1

		if rateDistQ14 < minRateDistQ14 {
			minRateDistQ14 = rateDistQ14
			*periodicityIndex = uint8(k)
			copy(cbkIndex, tempIdx[:nbSubfr])
			bestSumLogGainQ7 = sumLogGainTmpQ7
		}

		// Break early in low-complexity mode if rate distortion is below threshold
		if lowComplexity && (rateDistQ14 < LTPGainMiddleAvgRDQ14) {
			break
		}
	}

	// Apply the selected quantization
	cbkPtrQ7 = LTPVQPtrsQ7[*periodicityIndex]
	for j = 0; j < nbSubfr; j++ {
		for k = 0; k < LTP_ORDER; k++ {
			B_Q14[j*LTP_ORDER+k] = int16(cbkPtrQ7[cbkIndex[j]][k] << 7)
		}
	}

	*sumLogGainQ7 = bestSumLogGainQ7
}