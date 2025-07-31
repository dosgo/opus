package opus

import "math"

func silk_quant_LTP_gains(
	B_Q14 []int16,
	cbk_index []byte,
	periodicity_index BoxedValueByte,
	sum_log_gain_Q7 *BoxedValueInt,
	W_Q18 []int,
	mu_Q9 int,
	lowComplexity int,
	nb_subfr int) {

	var j, k, cbk_size int
	var temp_idx [MAX_NB_SUBFR]byte
	var cl_ptr_Q5 []int16
	var cbk_ptr_Q7 [][]int8
	var cbk_gain_ptr_Q7 []int16
	var b_Q14_ptr int
	var W_Q18_ptr int
	var rate_dist_Q14, min_rate_dist_Q14 int
	var sum_log_gain_tmp_Q7, best_sum_log_gain_Q7, max_gain_Q7 int

	min_rate_dist_Q14 = int(^uint(0) >> 1)
	best_sum_log_gain_Q7 = 0
	for k = 0; k < 3; k++ {
		gain_safety := int(SILK_CONST(0.4, 7))

		cl_ptr_Q5 = silk_LTP_gain_BITS_Q5_ptrs[k]
		cbk_ptr_Q7 = silk_LTP_vq_ptrs_Q7[k]
		cbk_gain_ptr_Q7 = silk_LTP_vq_gain_ptrs_Q7[k]
		cbk_size = int(silk_LTP_vq_sizes[k])

		W_Q18_ptr = 0
		b_Q14_ptr = 0

		rate_dist_Q14 = 0
		sum_log_gain_tmp_Q7 = sum_log_gain_Q7.Val
		for j = 0; j < nb_subfr; j++ {
			max_gain_Q7 = silk_log2lin(((SILK_CONST(TuningParameters.MAX_SUM_LOG_GAIN_DB/6.0, 7) - sum_log_gain_tmp_Q7) + SILK_CONST(7, 7)) - gain_safety)

			var tempIdxVal = BoxedValueByte{0}
			//var rate_dist_Q14_subfr int
			var gain_Q7 = BoxedValueInt{0}
			var rate_dist_Q14_subfr = BoxedValueInt{0}

			silk_VQ_WMat_EC(
				&tempIdxVal,
				&rate_dist_Q14_subfr,
				&gain_Q7,
				B_Q14,
				b_Q14_ptr,
				W_Q18,
				W_Q18_ptr,
				cbk_ptr_Q7,
				cbk_gain_ptr_Q7,
				cl_ptr_Q5,
				mu_Q9,
				max_gain_Q7,
				cbk_size,
			)

			rate_dist_Q14 = silk_ADD_POS_SAT32(rate_dist_Q14, rate_dist_Q14_subfr.Val)
			sum_log_gain_tmp_Q7 = silk_max(0, sum_log_gain_tmp_Q7+silk_lin2log(gain_safety+gain_Q7.Val)-int(math.Round(7*(1<<(7))+0.5)))

			temp_idx[j] = byte(tempIdxVal.Val)
			b_Q14_ptr += LTP_ORDER
			W_Q18_ptr += LTP_ORDER * LTP_ORDER
		}

		if rate_dist_Q14 < min_rate_dist_Q14 {
			min_rate_dist_Q14 = rate_dist_Q14
			periodicity_index.Val = int8(k)
			copy(cbk_index, temp_idx[:nb_subfr])
			best_sum_log_gain_Q7 = sum_log_gain_tmp_Q7
		}

		if lowComplexity != 0 && rate_dist_Q14 < int(SilkTables.Silk_LTP_gain_middle_avg_RD_Q14) {
			break
		}
	}

	cbk_ptr_Q7 = silk_LTP_vq_ptrs_Q7[periodicity_index.Val]
	for j = 0; j < nb_subfr; j++ {
		for k = 0; k < LTP_ORDER; k++ {
			B_Q14[j*LTP_ORDER+k] = int16(cbk_ptr_Q7[cbk_index[j]][k] << 7)
		}
	}

	sum_log_gain_Q7.Val = best_sum_log_gain_Q7
}
