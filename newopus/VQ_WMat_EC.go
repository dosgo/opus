package opus

import "math"

func silk_VQ_WMat_EC(ind BoxedValueByte, rate_dist_Q14 BoxedValueInt, gain_Q7 BoxedValueInt, in_Q14 []int16, in_Q14_ptr int, W_Q18 []int, W_Q18_ptr int, cb_Q7 [][]int8, cb_gain_Q7 []int16, cl_Q5 []int16, mu_Q9 int, max_gain_Q7 int, L int) {
	var k, gain_tmp_Q7 int
	var cb_row_Q7 []int8
	diff_Q14 := make([]int16, 5)
	var sum1_Q14, sum2_Q16 int

	rate_dist_Q14.Val = math.MaxInt
	for k = 0; k < L; k++ {
		cb_row_Q7 = cb_Q7[k]
		gain_tmp_Q7 = int(cb_gain_Q7[k])

		diff_Q14[0] = in_Q14[in_Q14_ptr] - int16(int8(cb_row_Q7[0]))<<7
		diff_Q14[1] = in_Q14[in_Q14_ptr+1] - int16(int8(cb_row_Q7[1]))<<7
		diff_Q14[2] = in_Q14[in_Q14_ptr+2] - int16(int8(cb_row_Q7[2]))<<7
		diff_Q14[3] = in_Q14[in_Q14_ptr+3] - int16(int8(cb_row_Q7[3]))<<7
		diff_Q14[4] = in_Q14[in_Q14_ptr+4] - int16(int8(cb_row_Q7[4]))<<7

		sum1_Q14 = mu_Q9 * int(cl_Q5[k])

		penalty := gain_tmp_Q7 - max_gain_Q7
		if penalty < 0 {
			penalty = 0
		}
		sum1_Q14 = sum1_Q14 + (penalty << 10)

		if sum1_Q14 < 0 {
			panic("OpusAssert failed: sum1_Q14 >= 0")
		}

		sum2_Q16 = int((int(W_Q18[W_Q18_ptr+1]) * int(diff_Q14[1]) >> 16))
		sum2_Q16 += int((int(W_Q18[W_Q18_ptr+2]) * int(diff_Q14[2]) >> 16))
		sum2_Q16 += int((int(W_Q18[W_Q18_ptr+3]) * int(diff_Q14[3]) >> 16))
		sum2_Q16 += int((int(W_Q18[W_Q18_ptr+4]) * int(diff_Q14[4]) >> 16))
		sum2_Q16 = sum2_Q16 << 1
		sum2_Q16 += int((int(W_Q18[W_Q18_ptr]) * int(diff_Q14[0]) >> 16))
		sum1_Q14 += int((int(sum2_Q16) * int(diff_Q14[0]) >> 16))

		sum2_Q16 = int((int(W_Q18[W_Q18_ptr+7]) * int(diff_Q14[2]) >> 16))
		sum2_Q16 += int((int(W_Q18[W_Q18_ptr+8]) * int(diff_Q14[3]) >> 16))
		sum2_Q16 += int((int(W_Q18[W_Q18_ptr+9]) * int(diff_Q14[4]) >> 16))
		sum2_Q16 = sum2_Q16 << 1
		sum2_Q16 += int((int(W_Q18[W_Q18_ptr+6]) * int(diff_Q14[1]) >> 16))
		sum1_Q14 += int((int(sum2_Q16) * int(diff_Q14[1]) >> 16))

		sum2_Q16 = int((int(W_Q18[W_Q18_ptr+13]) * int(diff_Q14[3]) >> 16))
		sum2_Q16 += int((int(W_Q18[W_Q18_ptr+14]) * int(diff_Q14[4]) >> 16))
		sum2_Q16 = sum2_Q16 << 1
		sum2_Q16 += int((int(W_Q18[W_Q18_ptr+12]) * int(diff_Q14[2]) >> 16))
		sum1_Q14 += int((int(sum2_Q16) * int(diff_Q14[2]) >> 16))

		sum2_Q16 = int((int(W_Q18[W_Q18_ptr+19]) * int(diff_Q14[4]) >> 16))
		sum2_Q16 = sum2_Q16 << 1
		sum2_Q16 += int((int(W_Q18[W_Q18_ptr+18]) * int(diff_Q14[3]) >> 16))
		sum1_Q14 += int((int(sum2_Q16) * int(diff_Q14[3]) >> 16))

		sum2_Q16 = int((int(W_Q18[W_Q18_ptr+24]) * int(diff_Q14[4]) >> 16))
		sum1_Q14 += int((int(sum2_Q16) * int(diff_Q14[4]) >> 16))

		if sum1_Q14 < 0 {
			panic("OpusAssert failed: sum1_Q14 >= 0")
		}

		if sum1_Q14 < rate_dist_Q14.Val {
			rate_dist_Q14.Val = sum1_Q14
			ind.Val = int8(k)
			gain_Q7.Val = gain_tmp_Q7
		}
	}
}
