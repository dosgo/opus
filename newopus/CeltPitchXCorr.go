package opus

func pitch_xcorr(_x []int, _y []int, xcorr []int, len int, max_pitch int) int {
	var i int
	maxcorr := int(1)
	if max_pitch <= 0 {
		panic("max_pitch must be greater than 0")
	}
	var sum0, sum1, sum2, sum3 int
	for i = 0; i < max_pitch-3; i += 4 {
		sum0 = 0
		sum1 = 0
		sum2 = 0
		sum3 = 0
		xcorr_kernel_int(_x, _y, i, &sum0, &sum1, &sum2, &sum3, len)
		xcorr[i] = sum0
		xcorr[i+1] = sum1
		xcorr[i+2] = sum2
		xcorr[i+3] = sum3
		sum0 = MAX32(sum0, sum1)
		sum2 = MAX32(sum2, sum3)
		sum0 = MAX32(sum0, sum2)
		maxcorr = MAX32(maxcorr, sum0)
	}
	for ; i < max_pitch; i++ {
		inner_sum := celt_inner_prod_int(_x, 0, _y, i, len)
		xcorr[i] = inner_sum
		maxcorr = MAX32(maxcorr, inner_sum)
	}
	return maxcorr
}

func pitch_xcorr(_x []int16, _x_ptr int, _y []int16, _y_ptr int, xcorr []int, len int, max_pitch int) int {
	var i int
	maxcorr := int(1)
	if max_pitch <= 0 {
		panic("max_pitch must be greater than 0")
	}
	var sum0, sum1, sum2, sum3 int
	for i = 0; i < max_pitch-3; i += 4 {
		sum0 = 0
		sum1 = 0
		sum2 = 0
		sum3 = 0
		xcorr_kernel_short(_x, _x_ptr, _y, _y_ptr+i, &sum0, &sum1, &sum2, &sum3, len)
		xcorr[i] = sum0
		xcorr[i+1] = sum1
		xcorr[i+2] = sum2
		xcorr[i+3] = sum3
		sum0 = MAX32(sum0, sum1)
		sum2 = MAX32(sum2, sum3)
		sum0 = MAX32(sum0, sum2)
		maxcorr = MAX32(maxcorr, sum0)
	}
	for ; i < max_pitch; i++ {
		inner_sum := celt_inner_prod_short_offset(_x, _x_ptr, _y, _y_ptr+i, len)
		xcorr[i] = inner_sum
		maxcorr = MAX32(maxcorr, inner_sum)
	}
	return maxcorr
}

func pitch_xcorr(_x []int16, _y []int16, xcorr []int, len int, max_pitch int) int {
	var i int
	maxcorr := int(1)
	if max_pitch <= 0 {
		panic("max_pitch must be greater than 0")
	}
	var sum0, sum1, sum2, sum3 int
	for i = 0; i < max_pitch-3; i += 4 {
		sum0 = 0
		sum1 = 0
		sum2 = 0
		sum3 = 0
		xcorr_kernel_short(_x, 0, _y, i, &sum0, &sum1, &sum2, &sum3, len)
		xcorr[i] = sum0
		xcorr[i+1] = sum1
		xcorr[i+2] = sum2
		xcorr[i+3] = sum3
		sum0 = MAX32(sum0, sum1)
		sum2 = MAX32(sum2, sum3)
		sum0 = MAX32(sum0, sum2)
		maxcorr = MAX32(maxcorr, sum0)
	}
	for ; i < max_pitch; i++ {
		inner_sum := celt_inner_prod_short(_x, _y, i, len)
		xcorr[i] = inner_sum
		maxcorr = MAX32(maxcorr, inner_sum)
	}
	return maxcorr
}

func xcorr_kernel_int(_x []int, _y []int, i int, sum0 *int, sum1 *int, sum2 *int, sum3 *int, len int) {
	for j := 0; j < len; j++ {
		*sum0 += _x[j] * _y[j+i]
		*sum1 += _x[j] * _y[j+i+1]
		*sum2 += _x[j] * _y[j+i+2]
		*sum3 += _x[j] * _y[j+i+3]
	}
}

func xcorr_kernel_short(_x []int16, _x_ptr int, _y []int16, _y_ptr int, sum0 *int, sum1 *int, sum2 *int, sum3 *int, len int) {
	for j := 0; j < len; j++ {
		x0 := int(_x[_x_ptr+j])
		*sum0 += x0 * int(_y[_y_ptr+j])
		*sum1 += x0 * int(_y[_y_ptr+j+1])
		*sum2 += x0 * int(_y[_y_ptr+j+2])
		*sum3 += x0 * int(_y[_y_ptr+j+3])
	}
}

func celt_inner_prod_int(_x []int, _x_idx int, _y []int, _y_idx int, len int) int {
	sum := int(0)
	for i := 0; i < len; i++ {
		sum += _x[_x_idx+i] * _y[_y_idx+i]
	}
	return sum
}

func celt_inner_prod_short_offset(_x []int16, _x_ptr int, _y []int16, _y_ptr int, len int) int {
	sum := int(0)
	for i := 0; i < len; i++ {
		sum += int(_x[_x_ptr+i]) * int(_y[_y_ptr+i])
	}
	return sum
}

func celt_inner_prod_short(_x []int16, _y []int16, _y_idx int, len int) int {
	sum := int(0)
	for i := 0; i < len; i++ {
		sum += int(_x[i]) * int(_y[_y_idx+i])
	}
	return sum
}
