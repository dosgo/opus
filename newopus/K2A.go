package opus
func silk_k2a(A_Q24 []int32, rc_Q15 []int16, order int) {
    if order == 0 {
        return
    }
    Atmp := make([]int32, order)
    for k := 0; k < order; k++ {
        for n := 0; n < k; n++ {
            Atmp[n] = A_Q24[n]
        }
        for n := 0; n < k; n++ {
            A_Q24[n] += ((Atmp[k-n-1] << 1) * int32(rc_Q15[k])) >> 16
        }
        A_Q24[k] = -(int32(rc_Q15[k]) << 9)
    }
}

func silk_k2a_Q16(A_Q24 []int32, rc_Q16 []int32, order int) {
    if order == 0 {
        return
    }
    Atmp := make([]int32, order)
    for k := 0; k < order; k++ {
        for n := 0; n < k; n++ {
            Atmp[n] = A_Q24[n]
        }
        for n := 0; n < k; n++ {
            A_Q24[n] += (Atmp[k-n-1] * rc_Q16[k]) >> 16
        }
        A_Q24[k] = -(rc_Q16[k] << 8)
    }
}