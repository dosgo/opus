package opus
import "math/bits"

const SILK_MAX_ORDER_LPC = 16

func silk_CLZ32(x int) int {
	if x == 0 {
		return 32
	}
	return bits.LeadingZeros32(uint32(x))
}

func silk_RSHIFT(x, n int) int {
	return x >> n
}

func silk_LSHIFT(x, n int) int {
	return x << n
}

func silk_abs_int32(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func silk_DIV32_16(a, b int) int {
	return a / b
}

func silk_max_32(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func silk_SMLAWB(a, b, c int) int {
	return a + int((int64(b)<<16+0x8000)*int64(c)>>16)
}

func silk_SAT16(x int) int {
	if x > 32767 {
		return 32767
	}
	if x < -32768 {
		return -32768
	}
	return x
}

func silk_DIV32_varQ(a, b, Q int) int {
	if Q == 31 {
		if b == 0 {
			if a >= 0 {
				return 0x7FFFFFFF
			} else {
				return -0x7FFFFFFF
			}
		}
		return int((int64(a) << 31) / int64(b))
	}
	panic("silk_DIV32_varQ: Q != 31 not implemented")
}

func silk_RSHIFT_ROUND(a, shift int) int {
	if shift == 1 {
		return (a >> 1) + (a & 1)
	}
	return ((a >> (shift - 1)) + 1) >> 1
}

func silk_SMMUL(a, b int) int {
	return int(int64(a)*int64(b) >> 32)
}

func InitTwoDimensionalArrayInt(rows, cols int) [][]int {
	array := make([][]int, rows)
	for i := range array {
		array[i] = make([]int, cols)
	}
	return array
}

func MemSetInt(array []int, value, length int) {
	for i := 0; i < length; i++ {
		array[i] = value
	}
}

const SILK_CONST_0_99_Q15 = 32440
const SILK_CONST_0_99_Q16 = 64881

func silk_schur(rc_Q15 []int16, c []int, order int) int {
	k, n, lz := 0, 0, 0
	C := InitTwoDimensionalArrayInt(SILK_MAX_ORDER_LPC+1, 2)
	Ctmp1, Ctmp2, rc_tmp_Q15 := 0, 0, 0

	if !(order == 6 || order == 8 || order == 10 || order == 12 || order == 14 || order == 16) {
		panic("OpusAssert failed")
	}

	lz = silk_CLZ32(c[0])

	if lz < 2 {
		for k = 0; k < order+1; k++ {
			C[k][0] = silk_RSHIFT(c[k], 1)
			C[k][1] = silk_RSHIFT(c[k], 1)
		}
	} else if lz > 2 {
		lz -= 2
		for k = 0; k < order+1; k++ {
			C[k][0] = silk_LSHIFT(c[k], lz)
			C[k][1] = silk_LSHIFT(c[k], lz)
		}
	} else {
		for k = 0; k < order+1; k++ {
			C[k][0] = c[k]
			C[k][1] = c[k]
		}
	}

	for k = 0; k < order; k++ {
		if silk_abs_int32(C[k+1][0]) >= C[0][1] {
			if C[k+1][0] > 0 {
				rc_Q15[k] = -SILK_CONST_0_99_Q15
			} else {
				rc_Q15[k] = SILK_CONST_0_99_Q15
			}
			k++
			break
		}

		rc_tmp_Q15 = -silk_DIV32_16(C[k+1][0], silk_max_32(silk_RSHIFT(C[0][1], 15), 1)
		rc_tmp_Q15 = silk_SAT16(rc_tmp_Q15)
		rc_Q15[k] = int16(rc_tmp_Q15)

		for n = 0; n < order-k; n++ {
			Ctmp1 = C[n+k+1][0]
			Ctmp2 = C[n][1]
			C[n+k+1][0] = silk_SMLAWB(Ctmp1, silk_LSHIFT(Ctmp2, 1), rc_tmp_Q15)
			C[n][1] = silk_SMLAWB(Ctmp2, silk_LSHIFT(Ctmp1, 1), rc_tmp_Q15)
		}
	}

	for ; k < order; k++ {
		rc_Q15[k] = 0
	}

	return silk_max_32(1, C[0][1])
}

func silk_schur64(rc_Q16 []int, c []int, order int) int {
	k, n := 0, 0
	C := InitTwoDimensionalArrayInt(SILK_MAX_ORDER_LPC+1, 2)
	Ctmp1_Q30, Ctmp2_Q30, rc_tmp_Q31 := 0, 0, 0

	if !(order == 6 || order == 8 || order == 10 || order == 12 || order == 14 || order == 16) {
		panic("OpusAssert failed")
	}

	if c[0] <= 0 {
		MemSetInt(rc_Q16, 0, order)
		return 0
	}

	for k = 0; k < order+1; k++ {
		C[k][0] = c[k]
		C[k][1] = c[k]
	}

	for k = 0; k < order; k++ {
		if silk_abs_int32(C[k+1][0]) >= C[0][1] {
			if C[k+1][0] > 0 {
				rc_Q16[k] = -SILK_CONST_0_99_Q16
			} else {
				rc_Q16[k] = SILK_CONST_0_99_Q16
			}
			k++
			break
		}

		rc_tmp_Q31 = silk_DIV32_varQ(-C[k+1][0], C[0][1], 31)
		rc_Q16[k] = silk_RSHIFT_ROUND(rc_tmp_Q31, 15)

		for n = 0; n < order-k; n++ {
			Ctmp1_Q30 = C[n+k+1][0]
			Ctmp2_Q30 = C[n][1]
			C[n+k+1][0] = Ctmp1_Q30 + silk_SMMUL(silk_LSHIFT(Ctmp2_Q30, 1), rc_tmp_Q31)
			C[n][1] = Ctmp2_Q30 + silk_SMMUL(silk_LSHIFT(Ctmp1_Q30, 1), rc_tmp_Q31)
		}
	}

	for ; k < order; k++ {
		rc_Q16[k] = 0
	}

	return silk_max_32(1, C[0][1])
}