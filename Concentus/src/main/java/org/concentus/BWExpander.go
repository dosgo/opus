package opus
func silk_bwexpander_32(ar []int32, d int, chirp_Q16 int32) {
    chirp_minus_one_Q16 := chirp_Q16 - 65536
    for i := 0; i < d-1; i++ {
        product := int64(chirp_Q16) * int64(ar[i])
        ar[i] = int32((product + 32768) >> 16)
        product2 := int64(chirp_Q16) * int64(chirp_minus_one_Q16)
        rounded := (product2 + 32768) >> 16
        chirp_Q16 += int32(rounded)
    }
    product := int64(chirp_Q16) * int64(ar[d-1])
    ar[d-1] = int32((product + 32768) >> 16)
}

func silk_bwexpander(ar []int16, d int, chirp_Q16 int32) {
    chirp_minus_one_Q16 := chirp_Q16 - 65536
    for i := 0; i < d-1; i++ {
        product := int64(chirp_Q16) * int64(ar[i])
        ar[i] = int16((product + 32768) >> 16)
        product2 := int64(chirp_Q16) * int64(chirp_minus_one_Q16)
        rounded := (product2 + 32768) >> 16
        chirp_Q16 += int32(rounded)
    }
    product := int64(chirp_Q16) * int64(ar[d-1])
    ar[d-1] = int16((product + 32768) >> 16)
}