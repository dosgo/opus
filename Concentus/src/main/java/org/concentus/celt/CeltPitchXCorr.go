package celt

// MAX32 returns the maximum of two int32 values.
func MAX32(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// pitchXCorr calculates the cross-correlation between two signals with unrolled loop optimization.
// Returns the maximum correlation value found.
func pitchXCorr(x, y []int, xcorr []int, len, maxPitch int) int {
	maxcorr := 1
	if maxPitch <= 0 {
		panic("maxPitch must be positive")
	}

	// Process 4 elements at a time for better performance
	var sum0, sum1, sum2, sum3 int
	i := 0
	for ; i < maxPitch-3; i += 4 {
		sum0, sum1, sum2, sum3 = 0, 0, 0, 0
		xcorrKernel(x, y, i, &sum0, &sum1, &sum2, &sum3, len)

		xcorr[i] = sum0
		xcorr[i+1] = sum1
		xcorr[i+2] = sum2
		xcorr[i+3] = sum3

		sum0 = MAX32(sum0, sum1)
		sum2 = MAX32(sum2, sum3)
		sum0 = MAX32(sum0, sum2)
		maxcorr = MAX32(maxcorr, sum0)
	}

	// Process remaining elements
	for ; i < maxPitch; i++ {
		innerSum := celtInnerProd(x, 0, y, i, len)
		xcorr[i] = innerSum
		maxcorr = MAX32(maxcorr, innerSum)
	}

	return maxcorr
}

// pitchXCorrShort calculates cross-correlation for short (int16) arrays with offset pointers.
func pitchXCorrShort(x []int16, xPtr int, y []int16, yPtr int, xcorr []int, len, maxPitch int) int {
	maxcorr := 1
	if maxPitch <= 0 {
		panic("maxPitch must be positive")
	}

	var sum0, sum1, sum2, sum3 int
	i := 0
	for ; i < maxPitch-3; i += 4 {
		sum0, sum1, sum2, sum3 = 0, 0, 0, 0
		xcorrKernelShort(x, xPtr, y, yPtr+i, &sum0, &sum1, &sum2, &sum3, len)

		xcorr[i] = sum0
		xcorr[i+1] = sum1
		xcorr[i+2] = sum2
		xcorr[i+3] = sum3

		sum0 = MAX32(sum0, sum1)
		sum2 = MAX32(sum2, sum3)
		sum0 = MAX32(sum0, sum2)
		maxcorr = MAX32(maxcorr, sum0)
	}

	for ; i < maxPitch; i++ {
		innerSum := celtInnerProdShort(x, xPtr, y, yPtr+i, len)
		xcorr[i] = innerSum
		maxcorr = MAX32(maxcorr, innerSum)
	}

	return maxcorr
}

// pitchXCorrShortNoPtr calculates cross-correlation for short arrays without offset pointers.
func pitchXCorrShortNoPtr(x, y []int16, xcorr []int, len, maxPitch int) int {
	maxcorr := 1
	if maxPitch <= 0 {
		panic("maxPitch must be positive")
	}

	var sum0, sum1, sum2, sum3 int
	i := 0
	for ; i < maxPitch-3; i += 4 {
		sum0, sum1, sum2, sum3 = 0, 0, 0, 0
		xcorrKernelShort(x, 0, y, i, &sum0, &sum1, &sum2, &sum3, len)

		xcorr[i] = sum0
		xcorr[i+1] = sum1
		xcorr[i+2] = sum2
		xcorr[i+3] = sum3

		sum0 = MAX32(sum0, sum1)
		sum2 = MAX32(sum2, sum3)
		sum0 = MAX32(sum0, sum2)
		maxcorr = MAX32(maxcorr, sum0)
	}

	for ; i < maxPitch; i++ {
		innerSum := celtInnerProdShort(x, y, i, len)
		xcorr[i] = innerSum
		maxcorr = MAX32(maxcorr, innerSum)
	}

	return maxcorr
}

// Note: The following helper functions (xcorrKernel, xcorrKernelShort, celtInnerProd,
// celtInnerProdShort) would need to be implemented separately based on the original
// Kernels class implementation.
