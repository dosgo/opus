package kernels

import (
	"math"
)

// Constants
const (
	SIG_SHIFT = 15 // Signal shift constant from original CeltConstants
)

// celtFirShort performs a finite impulse response (FIR) filter on 16-bit audio samples
// This is the short (int16) version of the FIR filter
func celtFir(x []int16, xPtr int, num []int16, y []int16, yPtr int, N, ord int, mem []int16) {
	// Allocate reversed num and local x buffers
	rnum := make([]int16, ord)
	localX := make([]int16, N+ord)

	// Reverse the num coefficients
	for i := 0; i < ord; i++ {
		rnum[i] = num[ord-i-1]
	}

	// Initialize local x with memory (reversed)
	for i := 0; i < ord; i++ {
		localX[i] = mem[ord-i-1]
	}

	// Copy input samples to local x buffer
	for i := 0; i < N; i++ {
		localX[i+ord] = x[xPtr+i]
	}

	// Update memory with last N samples (reversed)
	for i := 0; i < ord; i++ {
		mem[i] = x[xPtr+N-i-1]
	}

	// Process blocks of 4 samples for efficiency
	i := 0
	for ; i < N-3; i += 4 {
		var sum0, sum1, sum2, sum3 int32
		xcorrKernelShort(rnum, 0, localX, i, &sum0, &sum1, &sum2, &sum3, ord)

		y[yPtr+i] = saturate16(int32(x[xPtr+i]) + int32(sum0>>SIG_SHIFT))
		y[yPtr+i+1] = saturate16(int32(x[xPtr+i+1]) + int32(sum1>>SIG_SHIFT))
		y[yPtr+i+2] = saturate16(int32(x[xPtr+i+2]) + int32(sum2>>SIG_SHIFT))
		y[yPtr+i+3] = saturate16(int32(x[xPtr+i+3]) + int32(sum3>>SIG_SHIFT))
	}

	// Process remaining samples
	for ; i < N; i++ {
		var sum int32
		for j := 0; j < ord; j++ {
			sum = mac16_16(sum, int32(rnum[j]), int32(localX[i+j]))
		}
		y[yPtr+i] = saturate16(int32(x[xPtr+i]) + int32(sum>>SIG_SHIFT))
	}
}

// celtFirInt performs a finite impulse response (FIR) filter on 32-bit audio samples
// This is the int32 version of the FIR filter
func CeltFirInt(x []int32, xPtr int, num []int32, numPtr int, y []int32, yPtr int, N, ord int, mem []int32) {
	// Allocate reversed num and local x buffers
	rnum := make([]int32, ord)
	localX := make([]int32, N+ord)

	// Reverse the num coefficients
	for i := 0; i < ord; i++ {
		rnum[i] = num[numPtr+ord-i-1]
	}

	// Initialize local x with memory (reversed)
	for i := 0; i < ord; i++ {
		localX[i] = mem[ord-i-1]
	}

	// Copy input samples to local x buffer
	for i := 0; i < N; i++ {
		localX[i+ord] = x[xPtr+i]
	}

	// Update memory with last N samples (reversed)
	for i := 0; i < ord; i++ {
		mem[i] = x[xPtr+N-i-1]
	}

	// Process blocks of 4 samples for efficiency
	i := 0
	for ; i < N-3; i += 4 {
		var sum0, sum1, sum2, sum3 int32
		xcorrKernelInt(rnum, localX, i, &sum0, &sum1, &sum2, &sum3, ord)

		y[yPtr+i] = saturate32(int64(x[xPtr+i]) + int64(sum0>>SIG_SHIFT))
		y[yPtr+i+1] = saturate32(int64(x[xPtr+i+1]) + int64(sum1>>SIG_SHIFT))
		y[yPtr+i+2] = saturate32(int64(x[xPtr+i+2]) + int64(sum2>>SIG_SHIFT))
		y[yPtr+i+3] = saturate32(int64(x[xPtr+i+3]) + int64(sum3>>SIG_SHIFT))
	}

	// Process remaining samples
	for ; i < N; i++ {
		var sum int32
		for j := 0; j < ord; j++ {
			sum = mac16_16(sum, rnum[j], localX[i+j])
		}
		y[yPtr+i] = saturate32(int64(x[xPtr+i]) + int64(sum>>SIG_SHIFT))
	}
}

// xcorrKernelShort computes the cross-correlation of x and y (16-bit version)
// This is optimized to process 4 output samples at a time
func xcorrKernelShort(x []int16, xPtr int, y []int16, yPtr int, sum0, sum1, sum2, sum3 *int32, len int) {
	// Assert len >= 3 (removed in Go as we don't typically use assertions)
	var y0, y1, y2, y3 int16
	y3 = 0 // Initialize to avoid uninitialized warning

	// Load initial y values
	y0 = y[yPtr]
	yPtr++
	y1 = y[yPtr]
	yPtr++
	y2 = y[yPtr]
	yPtr++

	// Process blocks of 4 coefficients
	j := 0
	for ; j < len-3; j += 4 {
		tmp := x[xPtr]
		xPtr++
		y3 = y[yPtr]
		yPtr++
		*sum0 = mac16_16(*sum0, int32(tmp), int32(y0))
		*sum1 = mac16_16(*sum1, int32(tmp), int32(y1))
		*sum2 = mac16_16(*sum2, int32(tmp), int32(y2))
		*sum3 = mac16_16(*sum3, int32(tmp), int32(y3))

		tmp = x[xPtr]
		xPtr++
		y0 = y[yPtr]
		yPtr++
		*sum0 = mac16_16(*sum0, int32(tmp), int32(y1))
		*sum1 = mac16_16(*sum1, int32(tmp), int32(y2))
		*sum2 = mac16_16(*sum2, int32(tmp), int32(y3))
		*sum3 = mac16_16(*sum3, int32(tmp), int32(y0))

		tmp = x[xPtr]
		xPtr++
		y1 = y[yPtr]
		yPtr++
		*sum0 = mac16_16(*sum0, int32(tmp), int32(y2))
		*sum1 = mac16_16(*sum1, int32(tmp), int32(y3))
		*sum2 = mac16_16(*sum2, int32(tmp), int32(y0))
		*sum3 = mac16_16(*sum3, int32(tmp), int32(y1))

		tmp = x[xPtr]
		xPtr++
		y2 = y[yPtr]
		yPtr++
		*sum0 = mac16_16(*sum0, int32(tmp), int32(y3))
		*sum1 = mac16_16(*sum1, int32(tmp), int32(y0))
		*sum2 = mac16_16(*sum2, int32(tmp), int32(y1))
		*sum3 = mac16_16(*sum3, int32(tmp), int32(y2))
	}

	// Process remaining coefficients
	if j < len {
		tmp := x[xPtr]
		xPtr++
		y3 = y[yPtr]
		yPtr++
		*sum0 = mac16_16(*sum0, int32(tmp), int32(y0))
		*sum1 = mac16_16(*sum1, int32(tmp), int32(y1))
		*sum2 = mac16_16(*sum2, int32(tmp), int32(y2))
		*sum3 = mac16_16(*sum3, int32(tmp), int32(y3))
		j++
	}
	if j < len {
		tmp := x[xPtr]
		xPtr++
		y0 = y[yPtr]
		yPtr++
		*sum0 = mac16_16(*sum0, int32(tmp), int32(y1))
		*sum1 = mac16_16(*sum1, int32(tmp), int32(y2))
		*sum2 = mac16_16(*sum2, int32(tmp), int32(y3))
		*sum3 = mac16_16(*sum3, int32(tmp), int32(y0))
		j++
	}
	if j < len {
		tmp := x[xPtr]
		xPtr++
		y1 = y[yPtr]
		yPtr++
		*sum0 = mac16_16(*sum0, int32(tmp), int32(y2))
		*sum1 = mac16_16(*sum1, int32(tmp), int32(y3))
		*sum2 = mac16_16(*sum2, int32(tmp), int32(y0))
		*sum3 = mac16_16(*sum3, int32(tmp), int32(y1))
	}
}

// xcorrKernelInt computes the cross-correlation of x and y (32-bit version)
func xcorrKernelInt(x, y []int32, yPtr int, sum0, sum1, sum2, sum3 *int32, len int) {
	var y0, y1, y2, y3 int32
	y3 = 0 // Initialize to avoid uninitialized warning

	// Load initial y values
	y0 = y[yPtr]
	yPtr++
	y1 = y[yPtr]
	yPtr++
	y2 = y[yPtr]
	yPtr++

	// Process blocks of 4 coefficients
	xPtr := 0
	j := 0
	for ; j < len-3; j += 4 {
		tmp := x[xPtr]
		xPtr++
		y3 = y[yPtr]
		yPtr++
		*sum0 = mac16_16(*sum0, tmp, y0)
		*sum1 = mac16_16(*sum1, tmp, y1)
		*sum2 = mac16_16(*sum2, tmp, y2)
		*sum3 = mac16_16(*sum3, tmp, y3)

		tmp = x[xPtr]
		xPtr++
		y0 = y[yPtr]
		yPtr++
		*sum0 = mac16_16(*sum0, tmp, y1)
		*sum1 = mac16_16(*sum1, tmp, y2)
		*sum2 = mac16_16(*sum2, tmp, y3)
		*sum3 = mac16_16(*sum3, tmp, y0)

		tmp = x[xPtr]
		xPtr++
		y1 = y[yPtr]
		yPtr++
		*sum0 = mac16_16(*sum0, tmp, y2)
		*sum1 = mac16_16(*sum1, tmp, y3)
		*sum2 = mac16_16(*sum2, tmp, y0)
		*sum3 = mac16_16(*sum3, tmp, y1)

		tmp = x[xPtr]
		xPtr++
		y2 = y[yPtr]
		yPtr++
		*sum0 = mac16_16(*sum0, tmp, y3)
		*sum1 = mac16_16(*sum1, tmp, y0)
		*sum2 = mac16_16(*sum2, tmp, y1)
		*sum3 = mac16_16(*sum3, tmp, y2)
	}

	// Process remaining coefficients
	if j < len {
		tmp := x[xPtr]
		xPtr++
		y3 = y[yPtr]
		yPtr++
		*sum0 = mac16_16(*sum0, tmp, y0)
		*sum1 = mac16_16(*sum1, tmp, y1)
		*sum2 = mac16_16(*sum2, tmp, y2)
		*sum3 = mac16_16(*sum3, tmp, y3)
		j++
	}
	if j < len {
		tmp := x[xPtr]
		xPtr++
		y0 = y[yPtr]
		yPtr++
		*sum0 = mac16_16(*sum0, tmp, y1)
		*sum1 = mac16_16(*sum1, tmp, y2)
		*sum2 = mac16_16(*sum2, tmp, y3)
		*sum3 = mac16_16(*sum3, tmp, y0)
		j++
	}
	if j < len {
		tmp := x[xPtr]
		xPtr++
		y1 = y[yPtr]
		yPtr++
		*sum0 = mac16_16(*sum0, tmp, y2)
		*sum1 = mac16_16(*sum1, tmp, y3)
		*sum2 = mac16_16(*sum2, tmp, y0)
		*sum3 = mac16_16(*sum3, tmp, y1)
	}
}

// Inner product functions

// CeltInnerProdShort computes the inner product of two 16-bit vectors
func CeltInnerProd(x []int16, xPtr int, y []int16, yPtr int, N int) int32 {
	var xy int32
	for i := 0; i < N; i++ {
		xy = mac16_16(xy, int32(x[xPtr+i]), int32(y[yPtr+i]))
	}
	return xy
}

// CeltInnerProdShortNoXPtr computes the inner product of two 16-bit vectors (x starts at 0)
func CeltInnerProdShortNoXPtr(x []int16, y []int16, yPtr int, N int) int32 {
	var xy int32
	for i := 0; i < N; i++ {
		xy = mac16_16(xy, int32(x[i]), int32(y[yPtr+i]))
	}
	return xy
}

// CeltInnerProdInt computes the inner product of two 32-bit vectors
func CeltInnerProdInt(x []int32, xPtr int, y []int32, yPtr int, N int) int32 {
	var xy int32
	for i := 0; i < N; i++ {
		xy = mac16_16(xy, x[xPtr+i], y[yPtr+i])
	}
	return xy
}

// DualInnerProd computes two inner products simultaneously
func DualInnerProd(x []int32, xPtr int, y01, y02 []int32, y01Ptr, y02Ptr, N int) (xy1, xy2 int32) {
	for i := 0; i < N; i++ {
		xy1 = mac16_16(xy1, x[xPtr+i], y01[y01Ptr+i])
		xy2 = mac16_16(xy2, x[xPtr+i], y02[y02Ptr+i])
	}
	return
}

// Helper functions

// mac16_16 (int32 version) multiplies two 32-bit values and accumulates into a 32-bit sum
func mac16_16(sum int32, a, b int32) int32 {
	return sum + a*b
}

// saturate16 clamps a 32-bit value to 16-bit range
func saturate16(x int32) int16 {
	if x < math.MinInt16 {
		return math.MinInt16
	}
	if x > math.MaxInt16 {
		return math.MaxInt16
	}
	return int16(x)
}

// saturate32 clamps a 64-bit value to 32-bit range
func saturate32(x int64) int32 {
	if x < math.MinInt32 {
		return math.MinInt32
	}
	if x > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(x)
}
