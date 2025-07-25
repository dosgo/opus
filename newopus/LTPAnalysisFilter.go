package opus

func silk_LTP_analysis_filter(
	LTP_res []int16,
	x []int16,
	x_ptr int,
	LTPCoef_Q14 []int16,
	pitchL []int,
	invGains_Q16 []int,
	subfr_length int,
	nb_subfr int,
	pre_length int) {

	var x_ptr2, x_lag_ptr int
	Btmp_Q14 := make([]int16, SilkConstants.LTP_ORDER)
	var LTP_res_ptr int
	var k, i int
	var LTP_est int

	x_ptr2 = x_ptr
	LTP_res_ptr = 0
	for k = 0; k < nb_subfr; k++ {
		x_lag_ptr = x_ptr2 - pitchL[k]

		Btmp_Q14[0] = LTPCoef_Q14[k*SilkConstants.LTP_ORDER]
		Btmp_Q14[1] = LTPCoef_Q14[k*SilkConstants.LTP_ORDER+1]
		Btmp_Q14[2] = LTPCoef_Q14[k*SilkConstants.LTP_ORDER+2]
		Btmp_Q14[3] = LTPCoef_Q14[k*SilkConstants.LTP_ORDER+3]
		Btmp_Q14[4] = LTPCoef_Q14[k*SilkConstants.LTP_ORDER+4]

		for i = 0; i < subfr_length+pre_length; i++ {
			LTP_res_ptri := LTP_res_ptr + i
			LTP_res[LTP_res_ptri] = x[x_ptr2+i]

			LTP_est = silk_SMULBB(x[x_lag_ptr+SilkConstants.LTP_ORDER/2], Btmp_Q14[0])
			LTP_est = silk_SMLABB_ovflw(LTP_est, int(x[x_lag_ptr+1]), int(Btmp_Q14[1]))
			LTP_est = silk_SMLABB_ovflw(LTP_est, int(x[x_lag_ptr]), int(Btmp_Q14[2]))
			LTP_est = silk_SMLABB_ovflw(LTP_est, int(x[x_lag_ptr-1]), int(Btmp_Q14[3]))
			LTP_est = silk_SMLABB_ovflw(LTP_est, int(x[x_lag_ptr-2]), int(Btmp_Q14[4]))

			LTP_est = silk_RSHIFT_ROUND(LTP_est, 14)

			tmp := int(x[x_ptr2+i]) - LTP_est
			LTP_res[LTP_res_ptri] = silk_SAT16(tmp)

			gain := int(invGains_Q16[k])
			smulwb_result := silk_SMULWB(gain, LTP_res[LTP_res_ptri])
			LTP_res[LTP_res_ptri] = int16(smulwb_result)

			x_lag_ptr++
		}

		LTP_res_ptr += subfr_length + pre_length
		x_ptr2 += subfr_length
	}
}
