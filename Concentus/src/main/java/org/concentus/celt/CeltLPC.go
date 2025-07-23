package celt

import (
	"errors"
	"math"
)

// celtLPC calculates LPC coefficients from autocorrelation data.
// _lpc: output array for LPC coefficients (size p)
// ac: input autocorrelation values (size p+1)
// p: order of the LPC analysis
func CeltLPC(_lpc []int32, ac []int32, p int) error {
	// Input validation
	if len(_lpc) < p || len(ac) < p+1 {
		return errors.New("invalid input sizes")
	}

	error := ac[0]
	lpc := make([]int32, p) // Go automatically zero-initializes slices

	if ac[0] != 0 {
		for i := 0; i < p; i++ {
			// Sum up this iteration's reflection coefficient
			var rr int32
			for j := 0; j < i; j++ {
				rr += MULT32_32_Q31(lpc[j], ac[i-j])
			}
			rr += SHR32(ac[i+1], 3)
			r := -frac_div32(SHL32(rr, 3), error)

			// Update LPC coefficients and total error
			lpc[i] = SHR32(r, 3)

			// Update all LPC coefficients
			for j := 0; j < (i+1)>>1; j++ {
				tmp1 := lpc[j]
				tmp2 := lpc[i-1-j]
				lpc[j] = tmp1 + MULT32_32_Q31(r, tmp2)
				lpc[i-1-j] = tmp2 + MULT32_32_Q31(r, tmp1)
			}

			error -= MULT32_32_Q31(MULT32_32_Q31(r, r), error)

			// Bail out once we get 30 dB gain
			if error < SHR32(ac[0], 10) {
				break
			}
		}
	}

	// Copy results to output
	for i := 0; i < p; i++ {
		_lpc[i] = ROUND16(lpc[i], 16)
	}

	return nil
}

// celtIIR performs an IIR filter operation.
// _x: input signal
// _x_ptr: starting index in _x
// den: denominator coefficients
// _y: output signal
// _y_ptr: starting index in _y
// N: number of samples to process
// ord: filter order
// mem: memory (state) of the filter
func CeltIIR(_x []int32, _x_ptr int, den []int32, _y []int32, _y_ptr int, N int, ord int, mem []int32) error {
	// Input validation
	if (ord & 3) != 0 {
		return errors.New("filter order must be a multiple of 4")
	}
	if len(_x) < _x_ptr+N || len(_y) < _y_ptr+N || len(den) < ord || len(mem) < ord {
		return errors.New("invalid input sizes")
	}

	rden := make([]int32, ord)
	y := make([]int32, N+ord)

	// Prepare reversed denominator coefficients
	for i := 0; i < ord; i++ {
		rden[i] = den[ord-i-1]
	}

	// Initialize filter memory
	for i := 0; i < ord; i++ {
		y[i] = -mem[ord-i-1]
	}

	// Process samples in blocks of 4 for efficiency
	for i := 0; i < N-3; i += 4 {
		// Unroll by 4 as if it were an FIR filter
		sum0 := _x[_x_ptr+i]
		sum1 := _x[_x_ptr+i+1]
		sum2 := _x[_x_ptr+i+2]
		sum3 := _x[_x_ptr+i+3]

		// Perform correlation kernel operation
		xcorrKernel(rden, y, i, &sum0, &sum1, &sum2, &sum3, ord)

		// Patch up the result to compensate for the fact that this is an IIR
		y[i+ord] = -ROUND16(sum0, SIG_SHIFT)
		_y[_y_ptr+i] = sum0

		sum1 = MAC16_16(sum1, y[i+ord], den[0])
		y[i+ord+1] = -ROUND16(sum1, SIG_SHIFT)
		_y[_y_ptr+i+1] = sum1

		sum2 = MAC16_16(sum2, y[i+ord+1], den[0])
		sum2 = MAC16_16(sum2, y[i+ord], den[1])
		y[i+ord+2] = -ROUND16(sum2, SIG_SHIFT)
		_y[_y_ptr+i+2] = sum2

		sum3 = MAC16_16(sum3, y[i+ord+2], den[0])
		sum3 = MAC16_16(sum3, y[i+ord+1], den[1])
		sum3 = MAC16_16(sum3, y[i+ord], den[2])
		y[i+ord+3] = -ROUND16(sum3, SIG_SHIFT)
		_y[_y_ptr+i+3] = sum3
	}

	// Process remaining samples (if N not divisible by 4)
	for i := (N - 3) & ^3; i < N; i++ {
		sum := _x[_x_ptr+i]
		for j := 0; j < ord; j++ {
			sum -= MULT16_16(rden[j], y[i+j])
		}
		y[i+ord] = ROUND16(sum, SIG_SHIFT)
		_y[_y_ptr+i] = sum
	}

	// Update filter memory
	for i := 0; i < ord; i++ {
		mem[i] = _y[_y_ptr+N-i-1]
	}

	return nil
}

// Helper functions (equivalent to the Java Inlines class methods)

// MULT32_32_Q31 multiplies two 32-bit values with Q31 fixed-point arithmetic
func MULT32_32_Q31(a, b int32) int32 {
	return int32((int64(a) * int64(b)) >> 31)
}

// SHR32 shifts a 32-bit value right with rounding
func SHR32(a int32, shift int) int32 {
	return int32((int64(a) + (1 << (shift - 1))) >> shift)
}

// SHL32 shifts a 32-bit value left
func SHL32(a int32, shift int) int32 {
	return a << uint(shift)
}

// frac_div32 performs fractional division
func frac_div32(n, d int32) int32 {
	if int64(n) <= (int64(math.MaxInt32)>>1) && int64(d) <= (int64(math.MaxInt32)>>1) {
		return int32((int64(n) << 16) / int64(d))
	}
	return int32((int64(n) / int64(d)) << 16)
}

// ROUND16 rounds a value to 16 bits with specified shift
func ROUND16(a int32, shift int) int32 {
	return SHR32(a, shift)
}

// MULT16_16 multiplies two 16-bit values
func MULT16_16(a, b int32) int32 {
	return a * b
}

// MAC16_16 multiplies and accumulates two 16-bit values
func MAC16_16(sum, a, b int32) int32 {
	return sum + a*b
}

// xcorrKernel performs the correlation kernel operation (unrolled version)
func xcorrKernel(rden, y []int32, i int, sum0, sum1, sum2, sum3 *int32, ord int) {
	for j := 0; j < ord; j += 4 {
		*sum0 -= MULT16_16(rden[j], y[i+j])
		*sum1 -= MULT16_16(rden[j], y[i+j+1])
		*sum2 -= MULT16_16(rden[j], y[i+j+2])
		*sum3 -= MULT16_16(rden[j], y[i+j+3])
	}
}
