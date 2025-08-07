/*
Copyright (c) 2006-2011 Skype Limited. All Rights Reserved
Ported to Java by Logan Stromberg

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions
are met:

- Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.

- Redistributions in binary form must reproduce the above copyright
notice, this list of conditions and the following disclaimer in the
documentation and/or other materials provided with the distribution.

- Neither the name of Internet Society, IETF or IETF Trust, nor the
names of specific contributors, may be used to endorse or promote
products derived from this software without specific prior written
permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
“AS IS” AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER
OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL,
EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF
LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/
package opus

var OFFSET = ((MIN_QGAIN_DB*128)/6 + 16*128)
var SCALE_Q16 = (65536 * (N_LEVELS_QGAIN - 1)) / (((MAX_QGAIN_DB - MIN_QGAIN_DB) * 128) / 6)
var INV_SCALE_Q16 = (65536 * (((MAX_QGAIN_DB - MIN_QGAIN_DB) * 128) / 6)) / (N_LEVELS_QGAIN - 1)

func silk_gains_quant(ind []byte, gain_Q16 []int, prev_ind *BoxedValueByte, conditional int, nb_subfr int) {
	for k := 0; k < nb_subfr; k++ {
		ind[k] = byte(silk_SMULWB(SCALE_Q16, silk_lin2log(gain_Q16[k])-OFFSET))

		if ind[k] < byte(prev_ind.Val) {
			ind[k]++
		}

		ind[k] = byte(silk_LIMIT_int(int(ind[k]), 0, N_LEVELS_QGAIN-1))

		if k == 0 && conditional == 0 {
			ind[k] = byte(silk_LIMIT_int(int(ind[k]), int(prev_ind.Val)+MIN_DELTA_GAIN_QUANT, N_LEVELS_QGAIN-1))
			prev_ind.Val = int8(ind[k])
		} else {
			ind[k] -= byte(prev_ind.Val)

			double_step_size_threshold := 2*MAX_DELTA_GAIN_QUANT - N_LEVELS_QGAIN + int(prev_ind.Val)
			if int(ind[k]) > double_step_size_threshold {
				ind[k] = byte(double_step_size_threshold + silk_RSHIFT(int(ind[k])-double_step_size_threshold+1, 1))
			}

			ind[k] = byte(silk_LIMIT_int(int(ind[k]), MIN_DELTA_GAIN_QUANT, MAX_DELTA_GAIN_QUANT))

			if int(ind[k]) > double_step_size_threshold {
				prev_ind.Val = int8(int(prev_ind.Val) + (int(silk_LSHIFT(int(ind[k]), 1)) - double_step_size_threshold))
			} else {
				prev_ind.Val = int8(int(prev_ind.Val) + int(ind[k]))
			}

			ind[k] -= byte(SilkConstants.MIN_DELTA_GAIN_QUANT)
		}

		gain_Q16[k] = silk_log2lin(silk_min_32(silk_SMULWB(INV_SCALE_Q16, int(prev_ind.Val))+OFFSET, 3967))
	}
}

func silk_gains_dequant(gain_Q16 []int, ind []byte, prev_ind *BoxedValueByte, conditional int, nb_subfr int) {
	for k := 0; k < nb_subfr; k++ {
		if k == 0 && conditional == 0 {
			prev_ind.Val = int8(silk_max_int(int(ind[k]), int(prev_ind.Val)-16))
		} else {
			ind_tmp := int(ind[k]) + MIN_DELTA_GAIN_QUANT

			double_step_size_threshold := 2*MAX_DELTA_GAIN_QUANT - N_LEVELS_QGAIN + int(prev_ind.Val)
			if ind_tmp > double_step_size_threshold {
				prev_ind.Val = int8(int(prev_ind.Val) + (silk_LSHIFT(int(ind_tmp), 1) - double_step_size_threshold))
			} else {
				prev_ind.Val = int8(int(prev_ind.Val) + ind_tmp)
			}
		}

		prev_ind.Val = int8(silk_LIMIT_int(int(prev_ind.Val), 0, N_LEVELS_QGAIN-1))

		gain_Q16[k] = silk_log2lin(silk_min_32(silk_SMULWB(INV_SCALE_Q16, int(prev_ind.Val))+OFFSET, 3967))
	}
}

func silk_gains_ID(ind []byte, nb_subfr int) int {
	gainsID := int(0)
	for k := 0; k < nb_subfr; k++ {
		gainsID = silk_ADD_LSHIFT32(int(ind[k]), gainsID, 8)
	}
	return gainsID
}
