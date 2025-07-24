package opus

func celt_lpc(_lpc []int32, ac []int32, p int) {
	var i, j int
	var r int32
	error := ac[0]
	lpc := make([]int32, p)

	if ac[0] != 0 {
		for i = 0; i < p; i++ {
			var rr int32
			for j = 0; j < i; j++ {
				rr += MULT32_32_Q31(lpc[j], ac[i-j])
			}
			rr += SHR32(ac[i+1], 3)
			r = 0 - frac_div32(SHL32(rr, 3), error)
			lpc[i] = SHR32(r, 3)

			for j = 0; j < (i+1)>>1; j++ {
				tmp1 := lpc[j]
				tmp2 := lpc[i-1-j]
				lpc[j] = tmp1 + MULT32_32_Q31(r, tmp2)
				lpc[i-1-j] = tmp2 + MULT32_32_Q31(r, tmp1)
			}

			error = error - MULT32_32_Q31(MULT32_32_Q31(r, r), error)
			if error < SHR32(ac[0], 10) {
				break
			}
		}
	}

	for i = 0; i < p; i++ {
		_lpc[i] = ROUND16(lpc[i], 16)
	}
}

func celt_iir(_x []int32, _x_ptr int, den []int32, _y []int32, _y_ptr int, N int, ord int, mem []int32) {
	var i, j int
	rden := make([]int32, ord)
	y := make([]int32, N+ord)
	OpusAssert((ord & 3) == 0)

	var _sum0, _sum1, _sum2, _sum3 BoxedValueInt
	var sum0, sum1, sum2, sum3 int32

	for i = 0; i < ord; i++ {
		rden[i] = den[ord-i-1]
	}
	for i = 0; i < ord; i++ {
		y[i] = -mem[ord-i-1]
	}
	for ; i < N+ord; i++ {
		y[i] = 0
	}
	for i = 0; i < N-3; i += 4 {
		_sum0.Val = _x[_x_ptr+i]
		_sum1.Val = _x[_x_ptr+i+1]
		_sum2.Val = _x[_x_ptr+i+2]
		_sum3.Val = _x[_x_ptr+i+3]
		xcorr_kernel(rden, y, i, &_sum0, &_sum1, &_sum2, &_sum3, ord)
		sum0 = _sum0.Val
		sum1 = _sum1.Val
		sum2 = _sum2.Val
		sum3 = _sum3.Val

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
	for ; i < N; i++ {
		sum := _x[_x_ptr+i]
		for j = 0; j < ord; j++ {
			sum -= MULT16_16(rden[j], y[i+j])
		}
		y[i+ord] = ROUND16(sum, SIG_SHIFT)
		_y[_y_ptr+i] = sum
	}
	for i = 0; i < ord; i++ {
		mem[i] = _y[_y_ptr+N-i-1]
	}
}
