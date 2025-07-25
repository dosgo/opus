package opus
var SilkConstants = struct {
    LTP_ORDER    int
    MAX_NB_SUBFR int
}{
    LTP_ORDER:    5,
    MAX_NB_SUBFR: 4,
}

func silk_SMULBB(a, b int16) int32 {
    return int32(a) * int32(b)
}

func silk_SMLABB_ovflw(accum, a, b int32) int32 {
    a16 := int16(a)
    b16 := int16(b)
    return accum + int32(a16)*int32(b16)
}

func silk_RSHIFT_ROUND(a int32, shift int) int32 {
    if shift <= 0 {
        return a
    }
    rnd := int32(1 << (shift - 1))
    return (a + rnd) >> shift
}

func silk_SAT16(a int32) int16 {
    if a > 32767 {
        return 32767
    } else if a < -32768 {
        return -32768
    }
    return int16(a)
}

func silk_SMULWB(a int32, b int16) int32 {
    return (a * int32(b)) >> 16
}

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
    var LTP_est int32

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
            LTP_est = silk_SMLABB_ovflw(LTP_est, int32(x[x_lag_ptr+1]), int32(Btmp_Q14[1]))
            LTP_est = silk_SMLABB_ovflw(LTP_est, int32(x[x_lag_ptr]), int32(Btmp_Q14[2]))
            LTP_est = silk_SMLABB_ovflw(LTP_est, int32(x[x_lag_ptr-1]), int32(Btmp_Q14[3]))
            LTP_est = silk_SMLABB_ovflw(LTP_est, int32(x[x_lag_ptr-2]), int32(Btmp_Q14[4]))

            LTP_est = silk_RSHIFT_ROUND(LTP_est, 14)

            tmp := int32(x[x_ptr2+i]) - LTP_est
            LTP_res[LTP_res_ptri] = silk_SAT16(tmp)

            gain := int32(invGains_Q16[k])
            smulwb_result := silk_SMULWB(gain, LTP_res[LTP_res_ptri])
            LTP_res[LTP_res_ptri] = int16(smulwb_result)

            x_lag_ptr++
        }

        LTP_res_ptr += subfr_length + pre_length
        x_ptr2 += subfr_length
    }
}