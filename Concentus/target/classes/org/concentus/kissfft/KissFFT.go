// Package kissfft implements a Fast Fourier Transform (FFT) based on the KISS-FFT algorithm.
// Original work by Mark Borgerding, modified by Jean-Marc Valin for Opus, ported to Java by Logan Stromberg,
// and now translated to Go.
package kissfft

import (
	"opustest/opus"
)

const (
	MAXFACTORS = 8 // Maximum number of factors in decomposition
)

// FFTState represents the state of the FFT computation
type FFTState struct {
	Nfft       int   // Size of the FFT
	Scale      int16 // Scaling factor
	ScaleShift int   // Scaling shift
	Shift      int   // General shift parameter
	Factors    []int // Factorization of Nfft
	Twiddles   []int // Twiddle factors
	Bitrev     []int // Bit-reversal permutation
}

// S_MUL performs a signed multiplication with Q15 scaling
func S_MUL(a int, b int) int {
	return opus.MULT16_32_Q15(b, a)
}

// S_MUL16 performs a signed multiplication with Q15 scaling (16-bit version)
func S_MUL16(a int, b int16) int {
	return opus.MULT16_32_Q15(int(b), a)
}

// HALF_OF returns half of the input value (arithmetic shift right by 1)
func HALF_OF(x int) int {
	return x >> 1
}

// kf_bfly2 performs a butterfly operation for radix-2 FFT
func kf_bfly2(Fout []int, foutPtr, m, N int) {
	// We know that m==4 here because the radix-2 is just after a radix-4
	// Q15 value for 0.7071067812
	_tw := 0.5 + 0.7071067812*float32(1<<15)
	tw := int(_tw)

	for i := 0; i < N; i++ {
		Fout2 := foutPtr + 8

		// First pair
		t_r := Fout[Fout2]
		t_i := Fout[Fout2+1]
		Fout[Fout2] = Fout[foutPtr] - t_r
		Fout[Fout2+1] = Fout[foutPtr+1] - t_i
		Fout[foutPtr] += t_r
		Fout[foutPtr+1] += t_i

		// Second pair with twiddle
		t_r = S_MUL(Fout[Fout2+2]+Fout[Fout2+3], tw)
		t_i = S_MUL(Fout[Fout2+3]-Fout[Fout2+2], tw)
		Fout[Fout2+2] = Fout[foutPtr+2] - t_r
		Fout[Fout2+3] = Fout[foutPtr+3] - t_i
		Fout[foutPtr+2] += t_r
		Fout[foutPtr+3] += t_i

		// Third pair
		t_r = Fout[Fout2+5]
		t_i = -Fout[Fout2+4]
		Fout[Fout2+4] = Fout[foutPtr+4] - t_r
		Fout[Fout2+5] = Fout[foutPtr+5] - t_i
		Fout[foutPtr+4] += t_r
		Fout[foutPtr+5] += t_i

		// Fourth pair with twiddle
		t_r = S_MUL(Fout[Fout2+7]-Fout[Fout2+6], tw)
		t_i = S_MUL(-Fout[Fout2+7]-Fout[Fout2+6], tw)
		Fout[Fout2+6] = Fout[foutPtr+6] - t_r
		Fout[Fout2+7] = Fout[foutPtr+7] - t_i
		Fout[foutPtr+6] += t_r
		Fout[foutPtr+7] += t_i

		foutPtr += 16
	}
}

// kf_bfly4 performs a butterfly operation for radix-4 FFT
func kf_bfly4(Fout []int, foutPtr, fstride int, st *FFTState, m, N, mm int) {
	if m == 1 {
		// Degenerate case where all twiddles are 1
		for i := 0; i < N; i++ {
			scratch0 := Fout[foutPtr] - Fout[foutPtr+4]
			scratch1 := Fout[foutPtr+1] - Fout[foutPtr+5]
			Fout[foutPtr] += Fout[foutPtr+4]
			Fout[foutPtr+1] += Fout[foutPtr+5]

			scratch2 := Fout[foutPtr+2] + Fout[foutPtr+6]
			scratch3 := Fout[foutPtr+3] + Fout[foutPtr+7]
			Fout[foutPtr+4] = Fout[foutPtr] - scratch2
			Fout[foutPtr+5] = Fout[foutPtr+1] - scratch3
			Fout[foutPtr] += scratch2
			Fout[foutPtr+1] += scratch3

			scratch2 = Fout[foutPtr+2] - Fout[foutPtr+6]
			scratch3 = Fout[foutPtr+3] - Fout[foutPtr+7]
			Fout[foutPtr+2] = scratch0 + scratch3
			Fout[foutPtr+3] = scratch1 - scratch2
			Fout[foutPtr+6] = scratch0 - scratch3
			Fout[foutPtr+7] = scratch1 + scratch2

			foutPtr += 8
		}
	} else {
		FoutBeg := foutPtr
		for i := 0; i < N; i++ {
			foutPtr = FoutBeg + 2*i*mm
			m1 := foutPtr + 2*m
			m2 := foutPtr + 4*m
			m3 := foutPtr + 6*m
			tw1, tw2, tw3 := 0, 0, 0

			// m is guaranteed to be a multiple of 4
			for j := 0; j < m; j++ {
				// Complex multiplications with twiddle factors
				scratch0 := S_MUL(Fout[m1], st.Twiddles[tw1]) - S_MUL(Fout[m1+1], st.Twiddles[tw1+1])
				scratch1 := S_MUL(Fout[m1], st.Twiddles[tw1+1]) + S_MUL(Fout[m1+1], st.Twiddles[tw1])
				scratch2 := S_MUL(Fout[m2], st.Twiddles[tw2]) - S_MUL(Fout[m2+1], st.Twiddles[tw2+1])
				scratch3 := S_MUL(Fout[m2], st.Twiddles[tw2+1]) + S_MUL(Fout[m2+1], st.Twiddles[tw2])
				scratch4 := S_MUL(Fout[m3], st.Twiddles[tw3]) - S_MUL(Fout[m3+1], st.Twiddles[tw3+1])
				scratch5 := S_MUL(Fout[m3], st.Twiddles[tw3+1]) + S_MUL(Fout[m3+1], st.Twiddles[tw3])

				scratch10 := Fout[foutPtr] - scratch2
				scratch11 := Fout[foutPtr+1] - scratch3
				Fout[foutPtr] += scratch2
				Fout[foutPtr+1] += scratch3

				scratch6 := scratch0 + scratch4
				scratch7 := scratch1 + scratch5
				scratch8 := scratch0 - scratch4
				scratch9 := scratch1 - scratch5

				Fout[m2] = Fout[foutPtr] - scratch6
				Fout[m2+1] = Fout[foutPtr+1] - scratch7

				tw1 += fstride * 2
				tw2 += fstride * 4
				tw3 += fstride * 6

				Fout[foutPtr] += scratch6
				Fout[foutPtr+1] += scratch7
				Fout[m1] = scratch10 + scratch9
				Fout[m1+1] = scratch11 - scratch8
				Fout[m3] = scratch10 - scratch9
				Fout[m3+1] = scratch11 + scratch8

				foutPtr += 2
				m1 += 2
				m2 += 2
				m3 += 2
			}
		}
	}
}

// kf_bfly3 performs a butterfly operation for radix-3 FFT
func kf_bfly3(Fout []int, foutPtr, fstride int, st *FFTState, m, N, mm int) {
	m1 := 2 * m
	m2 := 4 * m
	FoutBeg := foutPtr

	for i := 0; i < N; i++ {
		foutPtr = FoutBeg + 2*i*mm
		tw1, tw2 := 0, 0

		// For non-custom modes, m is guaranteed to be a multiple of 4
		k := m
		for k > 0 {
			scratch2 := S_MUL(Fout[foutPtr+m1], st.Twiddles[tw1]) - S_MUL(Fout[foutPtr+m1+1], st.Twiddles[tw1+1])
			scratch3 := S_MUL(Fout[foutPtr+m1], st.Twiddles[tw1+1]) + S_MUL(Fout[foutPtr+m1+1], st.Twiddles[tw1])
			scratch4 := S_MUL(Fout[foutPtr+m2], st.Twiddles[tw2]) - S_MUL(Fout[foutPtr+m2+1], st.Twiddles[tw2+1])
			scratch5 := S_MUL(Fout[foutPtr+m2], st.Twiddles[tw2+1]) + S_MUL(Fout[foutPtr+m2+1], st.Twiddles[tw2])

			scratch6 := scratch2 + scratch4
			scratch7 := scratch3 + scratch5
			scratch0 := scratch2 - scratch4
			scratch1 := scratch3 - scratch5

			tw1 += fstride * 2
			tw2 += fstride * 4

			Fout[foutPtr+m1] = Fout[foutPtr] - HALF_OF(scratch6)
			Fout[foutPtr+m1+1] = Fout[foutPtr+1] - HALF_OF(scratch7)

			scratch0 = S_MUL(scratch0, -28378)
			scratch1 = S_MUL(scratch1, -28378)

			Fout[foutPtr] += scratch6
			Fout[foutPtr+1] += scratch7

			Fout[foutPtr+m2] = Fout[foutPtr+m1] + scratch1
			Fout[foutPtr+m2+1] = Fout[foutPtr+m1+1] - scratch0

			Fout[foutPtr+m1] -= scratch1
			Fout[foutPtr+m1+1] += scratch0

			foutPtr += 2
			k--
		}
	}
}

// kf_bfly5 performs a butterfly operation for radix-5 FFT
func kf_bfly5(Fout []int, fout_ptr int, fstride int, st *FFTState, m int, N int, mm int) {
	var Fout0, Fout1, Fout2, Fout3, Fout4 int
	var i, u int
	var scratch0, scratch1, scratch2, scratch3, scratch4, scratch5,
		scratch6, scratch7, scratch8, scratch9, scratch10, scratch11,
		scratch12, scratch13, scratch14, scratch15, scratch16, scratch17,
		scratch18, scratch19, scratch20, scratch21, scratch22, scratch23,
		scratch24, scratch25 int

	Fout_beg := fout_ptr

	ya_r := int16(10126)
	ya_i := int16(-31164)
	yb_r := int16(-26510)
	yb_i := int16(-19261)
	var tw1, tw2, tw3, tw4 int

	for i = 0; i < N; i++ {
		tw1 = 0
		tw2 = 0
		tw3 = 0
		tw4 = 0
		fout_ptr = Fout_beg + 2*i*mm
		Fout0 = fout_ptr
		Fout1 = fout_ptr + (2 * m)
		Fout2 = fout_ptr + (4 * m)
		Fout3 = fout_ptr + (6 * m)
		Fout4 = fout_ptr + (8 * m)

		// For non-custom modes, m is guaranteed to be a multiple of 4.
		for u = 0; u < m; u++ {
			scratch0 = Fout[Fout0+0]
			scratch1 = Fout[Fout0+1]

			scratch2 = (S_MUL(Fout[Fout1+0], st.Twiddles[tw1]) - S_MUL(Fout[Fout1+1], st.Twiddles[tw1+1]))
			scratch3 = (S_MUL(Fout[Fout1+0], st.Twiddles[tw1+1]) + S_MUL(Fout[Fout1+1], st.Twiddles[tw1]))
			scratch4 = (S_MUL(Fout[Fout2+0], st.Twiddles[tw2]) - S_MUL(Fout[Fout2+1], st.Twiddles[tw2+1]))
			scratch5 = (S_MUL(Fout[Fout2+0], st.Twiddles[tw2+1]) + S_MUL(Fout[Fout2+1], st.Twiddles[tw2]))
			scratch6 = (S_MUL(Fout[Fout3+0], st.Twiddles[tw3]) - S_MUL(Fout[Fout3+1], st.Twiddles[tw3+1]))
			scratch7 = (S_MUL(Fout[Fout3+0], st.Twiddles[tw3+1]) + S_MUL(Fout[Fout3+1], st.Twiddles[tw3]))
			scratch8 = (S_MUL(Fout[Fout4+0], st.Twiddles[tw4]) - S_MUL(Fout[Fout4+1], st.Twiddles[tw4+1]))
			scratch9 = (S_MUL(Fout[Fout4+0], st.Twiddles[tw4+1]) + S_MUL(Fout[Fout4+1], st.Twiddles[tw4]))

			tw1 += (2 * fstride)
			tw2 += (4 * fstride)
			tw3 += (6 * fstride)
			tw4 += (8 * fstride)

			scratch14 = scratch2 + scratch8
			scratch15 = scratch3 + scratch9
			scratch20 = scratch2 - scratch8
			scratch21 = scratch3 - scratch9
			scratch16 = scratch4 + scratch6
			scratch17 = scratch5 + scratch7
			scratch18 = scratch4 - scratch6
			scratch19 = scratch5 - scratch7

			Fout[Fout0+0] += scratch14 + scratch16
			Fout[Fout0+1] += scratch15 + scratch17

			scratch10 = scratch0 + S_MUL(scratch14, int(ya_r)) + S_MUL(scratch16, int(yb_r))
			scratch11 = scratch1 + S_MUL(scratch15, int(ya_r)) + S_MUL(scratch17, int(yb_r))

			scratch12 = S_MUL(scratch21, int(ya_i)) + S_MUL(scratch19, int(yb_i))
			scratch13 = 0 - S_MUL(scratch20, int(ya_i)) - S_MUL(scratch18, int(yb_i))

			Fout[Fout1+0] = scratch10 - scratch12
			Fout[Fout1+1] = scratch11 - scratch13
			Fout[Fout4+0] = scratch10 + scratch12
			Fout[Fout4+1] = scratch11 + scratch13

			scratch22 = scratch0 + S_MUL(scratch14, int(yb_r)) + S_MUL(scratch16, int(ya_r))
			scratch23 = scratch1 + S_MUL(scratch15, int(yb_r)) + S_MUL(scratch17, int(ya_r))
			scratch24 = 0 - S_MUL(scratch21, int(yb_i)) + S_MUL(scratch19, int(ya_i))
			scratch25 = S_MUL(scratch20, int(yb_i)) - S_MUL(scratch18, int(ya_i))

			Fout[Fout2+0] = scratch22 + scratch24
			Fout[Fout2+1] = scratch23 + scratch25
			Fout[Fout3+0] = scratch22 - scratch24
			Fout[Fout3+1] = scratch23 - scratch25

			Fout0 += 2
			Fout1 += 2
			Fout2 += 2
			Fout3 += 2
			Fout4 += 2
		}
	}
}
func opus_fft_impl(st *FFTState, fout []int, fout_ptr int) {
	var m2, m int
	var p int
	var L int
	fstride := make([]int, MAXFACTORS)
	var i int
	var shift int

	// st.shift can be -1
	if st.Shift > 0 {
		shift = st.Shift
	} else {
		shift = 0
	}

	fstride[0] = 1
	L = 0
	for {
		p = st.Factors[2*L]
		m = st.Factors[2*L+1]
		fstride[L+1] = fstride[L] * p
		L++
		if m == 1 {
			break
		}
	}

	m = st.Factors[2*L-1]
	for i = L - 1; i >= 0; i-- {
		if i != 0 {
			m2 = st.Factors[2*i-1]
		} else {
			m2 = 1
		}
		switch st.Factors[2*i] {
		case 2:
			kf_bfly2(fout, fout_ptr, m, fstride[i])
		case 4:
			kf_bfly4(fout, fout_ptr, fstride[i]<<shift, st, m, fstride[i], m2)
		case 3:
			kf_bfly3(fout, fout_ptr, fstride[i]<<shift, st, m, fstride[i], m2)
		case 5:
			kf_bfly5(fout, fout_ptr, fstride[i]<<shift, st, m, fstride[i], m2)
		}
		m = m2
	}
}

func opus_fft(st *FFTState, fin []int, fout []int) {
	var i int
	// Allows us to scale with MULT16_32_Q16()
	scale_shift := st.ScaleShift - 1
	scale := st.Scale

	OpusAssert(!(fin != nil && fout != nil && &fin[0] == &fout[0]), "In-place FFT not supported")

	// Bit-reverse the input
	for i = 0; i < st.Nfft; i++ {
		fout[2*st.Bitrev[i]] = SHR32(MULT16_32_Q16(scale, fin[2*i]), scale_shift)
		fout[2*st.Bitrev[i]+1] = SHR32(MULT16_32_Q16(scale, fin[2*i+1]), scale_shift)
	}

	opus_fft_impl(st, fout, 0)
}

// Helper functions that need to be implemented:
func MULT16_32_Q16(a int16, b int) int {
	// Implement this according to your specific fixed-point arithmetic requirements
	return int((int64(a) * int64(b)) >> 16)
}

func SHR32(a int, shift int) int {
	if shift > 0 {
		return a >> uint(shift)
	}
	return a
}

func OpusAssert(cond bool, message string) {
	if !cond {
		panic(message)
	}
}
