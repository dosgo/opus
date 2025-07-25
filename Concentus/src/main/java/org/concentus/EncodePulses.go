package opus
func combine_and_check(pulses_comb []int, pulses_comb_ptr int, pulses_in []int, pulses_in_ptr int, max_pulses int, len int) int {
    for k := 0; k < len; k++ {
        k2p := 2*k + pulses_in_ptr
        sum := pulses_in[k2p] + pulses_in[k2p+1]
        if sum > max_pulses {
            return 1
        }
        pulses_comb[pulses_comb_ptr+k] = sum
    }
    return 0
}

func silk_encode_pulses(
    psRangeEnc EntropyCoder,
    signalType int,
    quantOffsetType int,
    pulses []int8,
    frame_length int) {

    var i, k, j, iter, bit, nLS, scale_down, RateLevelIndex int
    var abs_q, minSumBits_Q5, sumBits_Q5 int
    var abs_pulses []int
    var sum_pulses []int
    var nRshifts []int
    pulses_comb := make([]int, 8)
    var abs_pulses_ptr int
    var pulses_ptr int
    var nBits_ptr []int16

    for idx := range pulses_comb {
        pulses_comb[idx] = 0
    }

    Inlines_OpusAssert(1<<SilkConstants_LOG2_SHELL_CODEC_FRAME_LENGTH == SilkConstants_SHELL_CODEC_FRAME_LENGTH)
    iter = int(Inlines_silk_RSHIFT(int32(frame_length), int32(SilkConstants_LOG2_SHELL_CODEC_FRAME_LENGTH))
    if iter*SilkConstants_SHELL_CODEC_FRAME_LENGTH < frame_length {
        Inlines_OpusAssert(frame_length == 12*10)
        iter++
        for idx := frame_length; idx < frame_length+SilkConstants_SHELL_CODEC_FRAME_LENGTH; idx++ {
            if idx < len(pulses) {
                pulses[idx] = 0
            }
        }
    }

    abs_pulses = make([]int, iter*SilkConstants_SHELL_CODEC_FRAME_LENGTH)
    Inlines_OpusAssert((SilkConstants_SHELL_CODEC_FRAME_LENGTH & 3) == 0)

    for i = 0; i < iter*SilkConstants_SHELL_CODEC_FRAME_LENGTH; i += 4 {
        val0 := int(pulses[i+0])
        if val0 < 0 {
            abs_pulses[i+0] = -val0
        } else {
            abs_pulses[i+0] = val0
        }
        val1 := int(pulses[i+1])
        if val1 < 0 {
            abs_pulses[i+1] = -val1
        } else {
            abs_pulses[i+1] = val1
        }
        val2 := int(pulses[i+2])
        if val2 < 0 {
            abs_pulses[i+2] = -val2
        } else {
            abs_pulses[i+2] = val2
        }
        val3 := int(pulses[i+3])
        if val3 < 0 {
            abs_pulses[i+3] = -val3
        } else {
            abs_pulses[i+3] = val3
        }
    }

    sum_pulses = make([]int, iter)
    nRshifts = make([]int, iter)
    abs_pulses_ptr = 0
    for i = 0; i < iter; i++ {
        nRshifts[i] = 0

        for {
            scale_down = combine_and_check(pulses_comb, 0, abs_pulses, abs_pulses_ptr, SilkTables_silk_max_pulses_table[0], 8)
            scale_down += combine_and_check(pulses_comb, 0, pulses_comb, 0, SilkTables_silk_max_pulses_table[1], 4)
            scale_down += combine_and_check(pulses_comb, 0, pulses_comb, 0, SilkTables_silk_max_pulses_table[2], 2)
            scale_down += combine_and_check(sum_pulses, i, pulses_comb, 0, SilkTables_silk_max_pulses_table[3], 1)

            if scale_down != 0 {
                nRshifts[i]++
                for k = abs_pulses_ptr; k < abs_pulses_ptr+SilkConstants_SHELL_CODEC_FRAME_LENGTH; k++ {
                    abs_pulses[k] = int(Inlines_silk_RSHIFT(int32(abs_pulses[k]), 1))
                }
            } else {
                break
            }
        }
        abs_pulses_ptr += SilkConstants_SHELL_CODEC_FRAME_LENGTH
    }

    minSumBits_Q5 = int(^uint(0) >> 1
    for k = 0; k < SilkConstants_N_RATE_LEVELS-1; k++ {
        nBits_ptr = SilkTables_silk_pulses_per_block_BITS_Q5[k]
        sumBits_Q5 = SilkTables_silk_rate_levels_BITS_Q5[signalType>>1][k]
        for i = 0; i < iter; i++ {
            if nRshifts[i] > 0 {
                sumBits_Q5 += int(nBits_ptr[SilkConstants_SILK_MAX_PULSES+1])
            } else {
                sumBits_Q5 += int(nBits_ptr[sum_pulses[i]])
            }
        }
        if sumBits_Q5 < minSumBits_Q5 {
            minSumBits_Q5 = sumBits_Q5
            RateLevelIndex = k
        }
    }

    psRangeEnc.enc_icdf(RateLevelIndex, SilkTables_silk_rate_levels_iCDF[signalType>>1], 8)

    for i = 0; i < iter; i++ {
        if nRshifts[i] == 0 {
            psRangeEnc.enc_icdf(sum_pulses[i], SilkTables_silk_pulses_per_block_iCDF[RateLevelIndex], 8)
        } else {
            psRangeEnc.enc_icdf(SilkConstants_SILK_MAX_PULSES+1, SilkTables_silk_pulses_per_block_iCDF[RateLevelIndex], 8)
            for k = 0; k < nRshifts[i]-1; k++ {
                psRangeEnc.enc_icdf(SilkConstants_SILK_MAX_PULSES+1, SilkTables_silk_pulses_per_block_iCDF[SilkConstants_N_RATE_LEVELS-1], 8)
            }
            psRangeEnc.enc_icdf(sum_pulses[i], SilkTables_silk_pulses_per_block_iCDF[SilkConstants_N_RATE_LEVELS-1], 8)
        }
    }

    for i = 0; i < iter; i++ {
        if sum_pulses[i] > 0 {
            ShellCoder_silk_shell_encoder(psRangeEnc, abs_pulses, i*SilkConstants_SHELL_CODEC_FRAME_LENGTH)
        }
    }

    for i = 0; i < iter; i++ {
        if nRshifts[i] > 0 {
            pulses_ptr = i * SilkConstants_SHELL_CODEC_FRAME_LENGTH
            nLS = nRshifts[i] - 1
            for k = 0; k < SilkConstants_SHELL_CODEC_FRAME_LENGTH; k++ {
                val := int(pulses[pulses_ptr+k])
                if val < 0 {
                    abs_q = -val
                } else {
                    abs_q = val
                }
                for j = nLS; j > 0; j-- {
                    bit = (abs_q >> j) & 1
                    psRangeEnc.enc_icdf(bit, SilkTables_silk_lsb_iCDF, 8)
                }
                bit = abs_q & 1
                psRangeEnc.enc_icdf(bit, SilkTables_silk_lsb_iCDF, 8)
            }
        }
    }

    CodeSigns_silk_encode_signs(psRangeEnc, pulses, frame_length, signalType, quantOffsetType, sum_pulses)
}