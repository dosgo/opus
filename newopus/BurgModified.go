package opus

const (
	MAX_FRAME_SIZE   = 384
	QA               = 25
	N_BITS_HEAD_ROOM = 2
	MIN_RSHIFTS      = -16
	MAX_RSHIFTS      = 32 - QA
)

var SILK_CONST_FIND_LPC_COND_FAC_32 int = 42950

func BurgModified_silk_burg_modified(res_nrg BoxedValueInt, res_nrg_Q BoxedValueInt, A_Q16 []int, x []int16, x_ptr int, minInvGain_Q30 int, subfr_length int, nb_subfr int, D int) {
	var k, n, s, lz, rshifts, reached_max_gain int
	var C0, num, nrg, rc_Q31, invGain_Q30, Atmp_QA, Atmp1, tmp1, tmp2, x1, x2 int
	var x_offset int
	C_first_row := make([]int, SILK_MAX_ORDER_LPC)
	C_last_row := make([]int, SILK_MAX_ORDER_LPC)
	Af_QA := make([]int, SILK_MAX_ORDER_LPC)
	CAf := make([]int, SILK_MAX_ORDER_LPC+1)
	CAb := make([]int, SILK_MAX_ORDER_LPC+1)
	xcorr := make([]int, SILK_MAX_ORDER_LPC)
	var C0_64 int64

	for i := range C_first_row {
		C_first_row[i] = 0
	}

	C0_64 = silk_inner_prod16_aligned_64(x, x_ptr, x, x_ptr, int(subfr_length)*int(nb_subfr))
	lz = silk_CLZ64(C0_64)
	rshifts = 32 + 1 + N_BITS_HEAD_ROOM - lz
	if rshifts > MAX_RSHIFTS {
		rshifts = MAX_RSHIFTS
	}
	if rshifts < MIN_RSHIFTS {
		rshifts = MIN_RSHIFTS
	}

	if rshifts > 0 {
		C0 = int(C0_64 >> uint(rshifts))
	} else {
		C0 = int(C0_64) << uint(-rshifts)
	}

	//CAf[0] = C0 + silk_SMMUL(SILK_CONST_FIND_LPC_COND_FAC_32, C0) + 1
	CAf[0] = C0 + silk_SMMUL(int(TuningParameters.FIND_LPC_COND_FAC*float32(1<<(32))+0.5), C0) + 1

	CAb[0] = CAf[0]

	if rshifts > 0 {
		for s = 0; s < int(nb_subfr); s++ {
			x_offset = x_ptr + s*int(subfr_length)
			for n = 1; n < int(D)+1; n++ {
				C_first_row[n-1] += int(silk_inner_prod16_aligned_64(x, x_offset, x, x_offset+n, int(subfr_length)-n) >> uint(rshifts))
			}
		}
	} else {
		for s = 0; s < int(nb_subfr); s++ {
			var i, d int
			x_offset = x_ptr + s*int(subfr_length)
			pitch_xcorr(x, x_offset, x, x_offset+1, xcorr, int(subfr_length)-int(D), int(D))
			for n = 1; n < int(D)+1; n++ {
				d = 0
				for i = n + int(subfr_length) - int(D); i < int(subfr_length); i++ {
					d = int(int(d) + int(int(x[x_offset+i])*int(x[x_offset+i-n])))
				}
				xcorr[n-1] += int(d)
			}
			for n = 1; n < int(D)+1; n++ {
				C_first_row[n-1] += int(xcorr[n-1]) << uint(-rshifts)
			}
		}
	}
	copy(C_last_row, C_first_row)

	//CAf[0] = C0 + silk_SMMUL(SILK_CONST_FIND_LPC_COND_FAC_32, C0) + 1
	CAf[0] = C0 + silk_SMMUL(int(TuningParameters.FIND_LPC_COND_FAC*float32(1<<(32))+0.5), C0) + 1

	CAb[0] = CAf[0]

	invGain_Q30 = 1 << 30
	reached_max_gain = 0
	for n = 0; n < int(D); n++ {
		if rshifts > -2 {
			for s = 0; s < int(nb_subfr); s++ {
				x_offset = x_ptr + s*int(subfr_length)
				x1 = -silk_LSHIFT32(int(x[x_offset+n]), 16-rshifts)
				x2 = -silk_LSHIFT32(int(x[x_offset+int(subfr_length)-n-1]), 16-rshifts)
				tmp1 = silk_LSHIFT32(int(x[x_offset+n]), QA-16)
				tmp2 = silk_LSHIFT32(int(x[x_offset+int(subfr_length)-n-1]), QA-16)
				for k = 0; k < n; k++ {
					C_first_row[k] = silk_SMLAWB(C_first_row[k], x1, int(x[x_offset+n-k-1]))
					C_last_row[k] = silk_SMLAWB(C_last_row[k], x2, int(x[x_offset+int(subfr_length)-n+k]))
					Atmp_QA = Af_QA[k]
					tmp1 = silk_SMLAWB(tmp1, Atmp_QA, int(x[x_offset+n-k-1]))
					tmp2 = silk_SMLAWB(tmp2, Atmp_QA, int(x[x_offset+int(subfr_length)-n+k]))
				}
				tmp1 = silk_LSHIFT32(-tmp1, 32-QA-rshifts)
				tmp2 = silk_LSHIFT32(-tmp2, 32-QA-rshifts)
				for k = 0; k <= n; k++ {
					CAf[k] = silk_SMLAWB(CAf[k], tmp1, int(x[x_offset+n-k]))
					CAb[k] = silk_SMLAWB(CAb[k], tmp2, int(x[x_offset+int(subfr_length)-n+k-1]))
				}
			}
		} else {
			for s = 0; s < int(nb_subfr); s++ {
				x_offset = x_ptr + s*int(subfr_length)
				x1 = -silk_LSHIFT32(int(x[x_offset+n]), -rshifts)
				x2 = -silk_LSHIFT32(int(x[x_offset+int(subfr_length)-n-1]), -rshifts)
				tmp1 = silk_LSHIFT32(int(x[x_offset+n]), 17)
				tmp2 = silk_LSHIFT32(int(x[x_offset+int(subfr_length)-n-1]), 17)
				for k = 0; k < n; k++ {
					C_first_row[k] = silk_MLA(C_first_row[k], x1, int(x[x_offset+n-k-1]))
					C_last_row[k] = silk_MLA(C_last_row[k], x2, int(x[x_offset+int(subfr_length)-n+k]))
					Atmp1 = silk_RSHIFT_ROUND(Af_QA[k], QA-17)
					tmp1 = silk_MLA(tmp1, int(x[x_offset+n-k-1]), Atmp1)
					tmp2 = silk_MLA(tmp2, int(x[x_offset+int(subfr_length)-n+k]), Atmp1)
				}
				tmp1 = -tmp1
				tmp2 = -tmp2
				for k = 0; k <= n; k++ {
					CAf[k] = silk_SMLAWW(CAf[k], tmp1, silk_LSHIFT32(int(x[x_offset+n-k]), -rshifts-1))
					CAb[k] = silk_SMLAWW(CAb[k], tmp2, silk_LSHIFT32(int(x[x_offset+int(subfr_length)-n+k-1]), -rshifts-1))
				}
			}
		}

		tmp1 = C_first_row[n]
		tmp2 = C_last_row[n]
		num = 0
		nrg = silk_ADD32(CAb[0], CAf[0])
		for k = 0; k < n; k++ {
			Atmp_QA = Af_QA[k]
			lz = silk_CLZ32(silk_abs(Atmp_QA)) - 1
			if 32-QA < lz {
				lz = 32 - QA
			}
			Atmp1 = Atmp_QA << uint(lz)
			tmp1 = silk_ADD_LSHIFT32(tmp1, silk_SMMUL(C_last_row[n-k-1], Atmp1), 32-QA-lz)
			tmp2 = silk_ADD_LSHIFT32(tmp2, silk_SMMUL(C_first_row[n-k-1], Atmp1), 32-QA-lz)
			num = silk_ADD_LSHIFT32(num, silk_SMMUL(CAb[n-k], Atmp1), 32-QA-lz)
			nrg = silk_ADD_LSHIFT32(nrg, silk_SMMUL(silk_ADD32(CAb[k+1], CAf[k+1]), Atmp1), 32-QA-lz)
		}
		CAf[n+1] = tmp1
		CAb[n+1] = tmp2
		num = silk_ADD32(num, tmp2)
		num = silk_LSHIFT32(-num, 1)

		if silk_abs(num) < nrg {
			rc_Q31 = silk_DIV32_varQ(num, nrg, 31)
		} else {
			if num > 0 {
				rc_Q31 = 1<<31 - 1
			} else {
				rc_Q31 = -1 << 31
			}
		}

		tmp1 = (1 << 30) - silk_SMMUL(rc_Q31, rc_Q31)
		tmp1 = silk_LSHIFT(silk_SMMUL(invGain_Q30, tmp1), 2)
		if tmp1 <= minInvGain_Q30 {
			tmp2 = (1 << 30) - silk_DIV32_varQ(minInvGain_Q30, invGain_Q30, 30)
			rc_Q31 = silk_SQRT_APPROX(tmp2)
			rc_Q31 = (rc_Q31 + silk_DIV32(tmp2, rc_Q31)) >> 1
			rc_Q31 = rc_Q31 << 16
			if num < 0 {
				rc_Q31 = -rc_Q31
			}
			invGain_Q30 = minInvGain_Q30
			reached_max_gain = 1
		} else {
			invGain_Q30 = tmp1
		}

		for k = 0; k < (n+1)>>1; k++ {
			tmp1 = Af_QA[k]
			tmp2 = Af_QA[n-k-1]
			Af_QA[k] = silk_ADD_LSHIFT32(tmp1, silk_SMMUL(tmp2, rc_Q31), 1)
			Af_QA[n-k-1] = silk_ADD_LSHIFT32(tmp2, silk_SMMUL(tmp1, rc_Q31), 1)
		}
		Af_QA[n] = silk_RSHIFT32(rc_Q31, 31-QA)

		if reached_max_gain != 0 {
			for k = n + 1; k < int(D); k++ {
				Af_QA[k] = 0
			}
			break
		}

		for k = 0; k <= n+1; k++ {
			tmp1 = CAf[k]
			tmp2 = CAb[n-k+1]
			CAf[k] = silk_ADD_LSHIFT32(tmp1, silk_SMMUL(tmp2, rc_Q31), 1)
			CAb[n-k+1] = silk_ADD_LSHIFT32(tmp2, silk_SMMUL(tmp1, rc_Q31), 1)
		}
	}

	if reached_max_gain != 0 {
		for k = 0; k < int(D); k++ {
			A_Q16[k] = -silk_RSHIFT_ROUND(Af_QA[k], QA-16)
		}
		if rshifts > 0 {
			for s = 0; s < int(nb_subfr); s++ {
				x_offset = x_ptr + s*int(subfr_length)
				C0 -= int(silk_inner_prod16_aligned_64(x, x_offset, x, x_offset, int(D)) >> uint(rshifts))
			}
		} else {
			for s = 0; s < int(nb_subfr); s++ {
				x_offset = x_ptr + s*int(subfr_length)
				C0 -= silk_LSHIFT32(silk_inner_prod_self(x, x_offset, int(D)), -rshifts)
			}
		}
		res_nrg.Val = silk_LSHIFT(silk_SMMUL(invGain_Q30, C0), 2)
		res_nrg_Q.Val = int(0 - rshifts)
	} else {
		nrg = CAf[0]
		tmp1 = 1 << 16
		for k = 0; k < int(D); k++ {
			Atmp1 = silk_RSHIFT_ROUND(Af_QA[k], QA-16)
			nrg = silk_SMLAWW(nrg, CAf[k+1], Atmp1)
			tmp1 = silk_SMLAWW(tmp1, Atmp1, Atmp1)
			A_Q16[k] = -Atmp1
		}
		//res_nrg.Val = silk_SMLAWW(nrg, silk_SMMUL(SILK_CONST_FIND_LPC_COND_FAC_32, C0), -tmp1)
		res_nrg.Val = silk_SMLAWW(nrg, silk_SMMUL(int((TuningParameters.FIND_LPC_COND_FAC)*float32(1<<(32))+0.5), C0), -tmp1)

		res_nrg_Q.Val = int(-rshifts)
	}
}
