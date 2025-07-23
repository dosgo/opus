package opus
func warped_gain(coefs_Q24 []int32, lambda_Q16 int32, order int) int32 {
	lambda_Q16 = -lambda_Q16
	gain_Q24 := coefs_Q24[order-1]
	for i := order - 2; i >= 0; i-- {
		gain_Q24 = silk_SMLAWB(coefs_Q24[i], gain_Q24, lambda_Q16)
	}
	gain_Q24 = silk_SMLAWB(int32((1.0)*float64(1<<24)+0.5, gain_Q24, -lambda_Q16)
	return silk_INVERSE32_varQ(gain_Q24, 40)
}

func limit_warped_coefs(coefs_syn_Q24 []int32, coefs_ana_Q24 []int32, lambda_Q16 int32, limit_Q24 int32, order int) {
	var i, iter, ind int
	var tmp, maxabs_Q24, chirp_Q16, gain_syn_Q16, gain_ana_Q16 int32
	var nom_Q16, den_Q24 int32

	lambda_Q16 = -lambda_Q16
	for i = order - 1; i > 0; i-- {
		coefs_syn_Q24[i-1] = silk_SMLAWB(coefs_syn_Q24[i-1], coefs_syn_Q24[i], lambda_Q16)
		coefs_ana_Q24[i-1] = silk_SMLAWB(coefs_ana_Q24[i-1], coefs_ana_Q24[i], lambda_Q16)
	}
	lambda_Q16 = -lambda_Q16
	nom_Q16 = silk_SMLAWB(int32((1.0)*float64(1<<16)+0.5, -lambda_Q16, lambda_Q16)
	den_Q24 = silk_SMLAWB(int32((1.0)*float64(1<<24)+0.5, coefs_syn_Q24[0], lambda_Q16)
	gain_syn_Q16 = silk_DIV32_varQ(nom_Q16, den_Q24, 24)
	den_Q24 = silk_SMLAWB(int32((1.0)*float64(1<<24)+0.5, coefs_ana_Q24[0], lambda_Q16)
	gain_ana_Q16 = silk_DIV32_varQ(nom_Q16, den_Q24, 24)
	for i = 0; i < order; i++ {
		coefs_syn_Q24[i] = silk_SMULWW(gain_syn_Q16, coefs_syn_Q24[i])
		coefs_ana_Q24[i] = silk_SMULWW(gain_ana_Q16, coefs_ana_Q24[i])
	}

	for iter = 0; iter < 10; iter++ {
		maxabs_Q24 = -1
		for i = 0; i < order; i++ {
			tmp = silk_max(silk_abs_int32(coefs_syn_Q24[i]), silk_abs_int32(coefs_ana_Q24[i]))
			if tmp > maxabs_Q24 {
				maxabs_Q24 = tmp
				ind = i
			}
		}
		if maxabs_Q24 <= limit_Q24 {
			return
		}

		for i = 1; i < order; i++ {
			coefs_syn_Q24[i-1] = silk_SMLAWB(coefs_syn_Q24[i-1], coefs_syn_Q24[i], lambda_Q16)
			coefs_ana_Q24[i-1] = silk_SMLAWB(coefs_ana_Q24[i-1], coefs_ana_Q24[i], lambda_Q16)
		}
		gain_syn_Q16 = silk_INVERSE32_varQ(gain_syn_Q16, 32)
		gain_ana_Q16 = silk_INVERSE32_varQ(gain_ana_Q16, 32)
		for i = 0; i < order; i++ {
			coefs_syn_Q24[i] = silk_SMULWW(gain_syn_Q16, coefs_syn_Q24[i])
			coefs_ana_Q24[i] = silk_SMULWW(gain_ana_Q16, coefs_ana_Q24[i])
		}

		chirp_Q16 = int32((0.99)*float64(1<<16)+0.5) - silk_DIV32_varQ(
			silk_SMULWB(maxabs_Q24-limit_Q24, silk_SMLABB(int32((0.8)*float64(1<<10)+0.5, int32((0.1)*float64(1<<10)+0.5, iter)),
			silk_MUL(maxabs_Q24, int32(ind+1)), 22)
		silk_bwexpander_32(coefs_syn_Q24, order, chirp_Q16)
		silk_bwexpander_32(coefs_ana_Q24, order, chirp_Q16)

		lambda_Q16 = -lambda_Q16
		for i = order - 1; i > 0; i-- {
			coefs_syn_Q24[i-1] = silk_SMLAWB(coefs_syn_Q24[i-1], coefs_syn_Q24[i], lambda_Q16)
			coefs_ana_Q24[i-1] = silk_SMLAWB(coefs_ana_Q24[i-1], coefs_ana_Q24[i], lambda_Q16)
		}
		lambda_Q16 = -lambda_Q16
		nom_Q16 = silk_SMLAWB(int32((1.0)*float64(1<<16)+0.5, -lambda_Q16, lambda_Q16)
		den_Q24 = silk_SMLAWB(int32((1.0)*float64(1<<24)+0.5, coefs_syn_Q24[0], lambda_Q16)
		gain_s极