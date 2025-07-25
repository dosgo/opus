package silk

import (
	"math"
)

type StereoEncodeState struct {
	sMid            [2]int16
	sSide           [2]int16
	mid_side_amp_Q0 [4]int
	pred_prev_Q13   [2]int16
	width_prev_Q14  int16
	smth_width_Q14  int16
	silent_side_len int16
}

type StereoDecodeState struct {
	sMid          [2]int16
	sSide         [2]int16
	pred_prev_Q13 [2]int16
}

type BoxedValueInt struct {
	Val int
}

type BoxedValueByte struct {
	Val byte
}

func silk_stereo_decode_pred(psRangeDec *EntropyCoder, pred_Q13 []int) {
	var n int
	ix := make([][]int, 2)
	for i := range ix {
		ix[i] = make([]int, 3)
	}
	var low_Q13, step_Q13 int

	// Entropy decoding
	n = psRangeDec.dec_icdf(silk_stereo_pred_joint_iCDF[:], 8)
	ix[0][2] = int(DIV32_16(n, 5))
	ix[1][2] = n - 5*ix[0][2]
	for n = 0; n < 2; n++ {
		ix[n][0] = psRangeDec.dec_icdf(silk_uniform3_iCDF[:], 8)
		ix[n][1] = psRangeDec.dec_icdf(silk_uniform5_iCDF[:], 8)
	}

	// Dequantize
	for n = 0; n < 2; n++ {
		ix[n][0] += 3 * ix[n][2]
		low_Q13 = int(silk_stereo_pred_quant_Q13[ix[n][0]])
		step_Q13 = SMULWB(int(silk_stereo_pred_quant_Q13[ix[n][0]+1]-low_Q13),
			int(0.5/STEREO_QUANT_SUB_STEPS*(1<<16)+0.5))
		pred_Q13[n] = SMLABB(low_Q13, step_Q13, 2*ix[n][1]+1)
	}

	// Subtract second from first predictor
	pred_Q13[0] -= pred_Q13[1]
}

func silk_stereo_decode_mid_only(psRangeDec *EntropyCoder, decode_only_mid *BoxedValueByte) {
	decode_only_mid.Val = byte(psRangeDec.dec_icdf(silk_stereo_only_code_mid_iCDF[:], 8))
}

func silk_stereo_encode_pred(psRangeEnc *EntropyCoder, ix [][]byte) {
	var n int

	// Entropy coding
	n = 5*int(ix[0][2]) + int(ix[1][2])
	OpusAssert(n < 25, "n should be less than 25")
	psRangeEnc.enc_icdf(n, silk_stereo_pred_joint_iCDF[:], 8)
	for n = 0; n < 2; n++ {
		OpusAssert(int(ix[n][0]) < 3, "ix[n][0] should be less than 3")
		OpusAssert(int(ix[n][1]) < STEREO_QUANT_SUB_STEPS, "ix[n][1] should be less than STEREO_QUANT_SUB_STEPS")
		psRangeEnc.enc_icdf(int(ix[n][0]), silk_uniform3_iCDF[:], 8)
		psRangeEnc.enc_icdf(int(ix[n][1]), silk_uniform5_iCDF[:], 8)
	}
}

func silk_stereo_encode_mid_only(psRangeEnc *EntropyCoder, mid_only_flag byte) {
	psRangeEnc.enc_icdf(int(mid_only_flag), silk_stereo_only_code_mid_iCDF[:], 8)
}

func silk_stereo_find_predictor(
	ratio_Q14 *BoxedValueInt,
	x []int16,
	y []int16,
	mid_res_amp_Q0 []int,
	mid_res_amp_Q0_ptr int,
	length int,
	smooth_coef_Q16 int) int {

	var scale int
	nrgx := &BoxedValueInt{0}
	nrgy := &BoxedValueInt{0}
	scale1 := &BoxedValueInt{0}
	scale2 := &BoxedValueInt{0}
	var corr, pred_Q13, pred2_Q10 int

	// Find predictor
	silk_sum_sqr_shift(nrgx, scale1, x, length)
	silk_sum_sqr_shift(nrgy, scale2, y, length)
	scale = max_int(scale1.Val, scale2.Val)
	scale += scale & 1 // make even
	nrgy.Val = RSHIFT32(nrgy.Val, scale-scale2.Val)
	nrgx.Val = RSHIFT32(nrgx.Val, scale-scale1.Val)
	nrgx.Val = max_int(nrgx.Val, 1)
	corr = silk_inner_prod_aligned_scale(x, y, scale, length)
	pred_Q13 = DIV32_varQ(corr, nrgx.Val, 13)
	pred_Q13 = LIMIT(pred_Q13, -(1 << 14), 1<<14)
	pred2_Q10 = SMULWB(pred_Q13, pred_Q13)

	// Faster update for signals with large prediction parameters
	smooth_coef_Q16 = max_int(smooth_coef_Q16, abs(pred2_Q10))

	// Smoothed mid and residual norms
	OpusAssert(smooth_coef_Q16 < 32768, "smooth_coef_Q16 should be less than 32768")
	scale = RSHIFT(scale, 1)
	mid_res_amp_Q0[mid_res_amp_Q0_ptr] = SMLAWB(mid_res_amp_Q0[mid_res_amp_Q0_ptr],
		LSHIFT(SQRT_APPROX(nrgx.Val), scale)-mid_res_amp_Q0[mid_res_amp_Q0_ptr], smooth_coef_Q16)

	// Residual energy = nrgy - 2 * pred * corr + pred^2 * nrgx
	nrgy.Val = SUB_LSHIFT32(nrgy.Val, SMULWB(corr, pred_Q13), 3+1)
	nrgy.Val = ADD_LSHIFT32(nrgy.Val, SMULWB(nrgx.Val, pred2_Q10), 6)
	mid_res_amp_Q0[mid_res_amp_Q0_ptr+1] = SMLAWB(mid_res_amp_Q0[mid_res_amp_Q0_ptr+1],
		LSHIFT(SQRT_APPROX(nrgy.Val), scale)-mid_res_amp_Q0[mid_res_amp_Q0_ptr+1], smooth_coef_Q16)

	// Ratio of smoothed residual and mid norms
	ratio_Q14.Val = DIV32_varQ(mid_res_amp_Q0[mid_res_amp_Q0_ptr+1], max_int(mid_res_amp_Q0[mid_res_amp_Q0_ptr], 1), 14)
	ratio_Q14.Val = LIMIT(ratio_Q14.Val, 0, 32767)

	return pred_Q13
}

func silk_stereo_LR_to_MS(
	state *StereoEncodeState,
	x1 []int16,
	x1_ptr int,
	x2 []int16,
	x2_ptr int,
	ix [][]byte,
	mid_only_flag *BoxedValueByte,
	mid_side_rates_bps []int,
	total_rate_bps int,
	prev_speech_act_Q8 int,
	toMono int,
	fs_kHz int,
	frame_length int) {

	var n, is10msFrame, denom_Q16, delta0_Q13, delta1_Q13 int
	var sum, diff, smooth_coef_Q16, pred0_Q13, pred1_Q13 int
	pred_Q13 := make([]int, 2)
	var frac_Q16, frac_3_Q16, min_mid_rate_bps, width_Q14, w_Q24, deltaw_Q24 int
	LP_ratio_Q14 := &BoxedValueInt{0}
	HP_ratio_Q14 := &BoxedValueInt{0}
	var side []int16
	var LP_mid, HP_mid, LP_side, HP_side []int16
	mid := x1_ptr - 2

	side = make([]int16, frame_length+2)

	// Convert to basic mid/side signals
	for n = 0; n < frame_length+2; n++ {
		sum = int(x1[x1_ptr+n-2]) + int(x2[x2_ptr+n-2])
		diff = int(x1[x1_ptr+n-2]) - int(x2[x2_ptr+n-2])
		x1[mid+n] = int16(RSHIFT_ROUND(sum, 1))
		side[n] = int16(SAT16(RSHIFT_ROUND(diff, 1)))
	}

	// Buffering
	copy(x1[mid:], state.sMid[:])
	copy(side, state.sSide[:])
	copy(state.sMid[:], x1[mid+frame_length:])
	copy(state.sSide[:], side[frame_length:])

	// LP and HP filter mid signal
	LP_mid = make([]int16, frame_length)
	HP_mid = make([]int16, frame_length)
	for n = 0; n < frame_length; n++ {
		sum = RSHIFT_ROUND(ADD_LSHIFT32(int(x1[mid+n])+int(x1[mid+n+2]), int(x1[mid+n+1]), 1), 2)
		LP_mid[n] = int16(sum)
		HP_mid[n] = int16(int(x1[mid+n+1]) - sum)
	}

	// LP and HP filter side signal
	LP_side = make([]int16, frame_length)
	HP_side = make([]int16, frame_length)
	for n = 0; n < frame_length; n++ {
		sum = RSHIFT_ROUND(ADD_LSHIFT32(int(side[n])+int(side[n+2]), int(side[n+1]), 1), 2)
		LP_side[n] = int16(sum)
		HP_side[n] = int16(int(side[n+1]) - sum)
	}

	// Find energies and predictors
	is10msFrame = 0
	if frame_length == 10*fs_kHz {
		is10msFrame = 1
	}
	if is10msFrame != 0 {
		smooth_coef_Q16 = int(STEREO_RATIO_SMOOTH_COEF/2*(1<<16) + 0.5)
	} else {
		smooth_coef_Q16 = int(STEREO_RATIO_SMOOTH_COEF*(1<<16) + 0.5)
	}
	smooth_coef_Q16 = SMULWB(SMULBB(prev_speech_act_Q8, prev_speech_act_Q8), smooth_coef_Q16)

	pred_Q13[0] = silk_stereo_find_predictor(LP_ratio_Q14, LP_mid, LP_side, state.mid_side_amp_Q0[:], 0, frame_length, smooth_coef_Q16)
	pred_Q13[1] = silk_stereo_find_predictor(HP_ratio_Q14, HP_mid, HP_side, state.mid_side_amp_Q0[:], 2, frame_length, smooth_coef_Q16)

	// Ratio of the norms of residual and mid signals
	frac_Q16 = SMLABB(HP_ratio_Q14.Val, LP_ratio_Q14.Val, 3)
	frac_Q16 = min(frac_Q16, int(1*(1<<16)+0.5))

	// Determine bitrate distribution between mid and side
	total_rate_bps -= is10msFrame*1200 + (1-is10msFrame)*600
	if total_rate_bps < 1 {
		total_rate_bps = 1
	}
	min_mid_rate_bps = SMLABB(2000, fs_kHz, 900)
	OpusAssert(min_mid_rate_bps < 32767, "min_mid_rate_bps should be less than 32767")

	// Default bitrate distribution
	frac_3_Q16 = MUL(3, frac_Q16)
	mid_side_rates_bps[0] = DIV32_varQ(total_rate_bps, int(8+5*(1<<16)+0.5)+frac_3_Q16, 16+3)

	// If Mid bitrate below minimum, reduce stereo width
	if mid_side_rates_bps[0] < min_mid_rate_bps {
		mid_side_rates_bps[0] = min_mid_rate_bps
		mid_side_rates_bps[1] = total_rate_bps - mid_side_rates_bps[0]
		width_Q14 = DIV32_varQ(LSHIFT(mid_side_rates_bps[1], 1)-min_mid_rate_bps,
			SMULWB(int(1*(1<<16)+0.5+frac_3_Q16, min_mid_rate_bps), 14+2))
		width_Q14 = LIMIT(width_Q14, 0, int(1*(1<<14)+0.5))
	} else {
		mid_side_rates_bps[1] = total_rate_bps - mid_side_rates_bps[0]
		width_Q14 = int(1*(1<<14) + 0.5)
	}

	// Smoother
	state.smth_width_Q14 = int16(SMLAWB(int(state.smth_width_Q14), width_Q14-int(state.smth_width_Q14), smooth_coef_Q16))

	// At very low bitrates or for nearly amplitude panned inputs, switch to panned-mono
	mid_only_flag.Val = 0
	if toMono != 0 {
		// Last frame before stereo->mono transition
		width_Q14 = 0
		pred_Q13[0] = 0
		pred_Q13[1] = 0
		silk_stereo_quant_pred(pred_Q13, ix)
	} else if state.width_prev_Q14 == 0 &&
		(8*total_rate_bps < 13*min_mid_rate_bps || SMULWB(frac_Q16, int(state.smth_width_Q14)) < int(0.05*(1<<14)+0.5)) {
		// Code as panned-mono
		pred_Q13[0] = RSHIFT(SMULBB(int(state.smth_width_Q14), pred_Q13[0]), 14)
		pred_Q13[1] = RSHIFT(SMULBB(int(state.smth_width_Q14), pred_Q13[1]), 14)
		silk_stereo_quant_pred(pred_Q13, ix)
		width_Q14 = 0
		pred_Q13[0] = 0
		pred_Q13[1] = 0
		mid_side_rates_bps[0] = total_rate_bps
		mid_side_rates_bps[1] = 0
		mid_only_flag.Val = 1
	} else if state.width_prev_Q14 != 0 &&
		(8*total_rate_bps < 11*min_mid_rate_bps || SMULWB(frac_Q16, int(state.smth_width_Q14)) < int(0.02*(1<<14)+0.5)) {
		// Transition to zero-width stereo
		pred_Q13[0] = RSHIFT(SMULBB(int(state.smth_width_Q14), pred_Q13[0]), 14)
		pred_Q13[1] = RSHIFT(SMULBB(int(state.smth_width_Q14), pred_Q13[1]), 14)
		silk_stereo_quant_pred(pred_Q13, ix)
		width_Q14 = 0
		pred_Q13[0] = 0
		pred_Q13[1] = 0
	} else if int(state.smth_width_Q14) > int(0.95*(1<<14)+0.5) {
		// Full-width stereo
		silk_stereo_quant_pred(pred_Q13, ix)
		width_Q14 = int(1*(1<<14) + 0.5)
	} else {
		// Reduced-width stereo
		pred_Q13[0] = RSHIFT(SMULBB(int(state.smth_width_Q14), pred_Q13[0]), 14)
		pred_Q13[1] = RSHIFT(SMULBB(int(state.smth_width_Q14), pred_Q13[1]), 14)
		silk_stereo_quant_pred(pred_Q13, ix)
		width_Q14 = int(state.smth_width_Q14)
	}

	// Make sure to keep encoding until tapered output transmitted
	if mid_only_flag.Val == 1 {
		state.silent_side_len += int16(frame_length - STEREO_INTERP_LEN_MS*fs_kHz)
		if state.silent_side_len < LA_SHAPE_MS*fs_kHz {
			mid_only_flag.Val = 0
		} else {
			state.silent_side_len = 10000
		}
	} else {
		state.silent_side_len = 0
	}

	if mid_only_flag.Val == 0 && mid_side_rates_bps[1] < 1 {
		mid_side_rates_bps[1] = 1
		mid_side_rates_bps[0] = max_int(1, total_rate_bps-mid_side_rates_bps[1])
	}

	// Interpolate predictors and subtract prediction from side channel
	pred0_Q13 = -int(state.pred_prev_Q13[0])
	pred1_Q13 = -int(state.pred_prev_Q13[1])
	w_Q24 = LSHIFT(int(state.width_prev_Q14), 10)
	denom_Q16 = DIV32_16(1<<16, STEREO_INTERP_LEN_MS*fs_kHz)
	delta0_Q13 = -RSHIFT_ROUND(SMULBB(pred_Q13[0]-int(state.pred_prev_Q13[0]), denom_Q16), 16)
	delta1_Q13 = -RSHIFT_ROUND(SMULBB(pred_Q13[1]-int(state.pred_prev_Q13[1]), denom_Q16), 16)
	deltaw_Q24 = LSHIFT(SMULWB(width_Q14-int(state.width_prev_Q14), denom_Q16), 10)
	for n = 0; n < STEREO_INTERP_LEN_MS*fs_kHz; n++ {
		pred0_Q13 += delta0_Q13
		pred1_Q13 += delta1_Q13
		w_Q24 += deltaw_Q24
		sum = LSHIFT(ADD_LSHIFT32(int(x1[mid+n])+int(x1[mid+n+2]), int(x1[mid+n+1]), 1), 9) // Q11
		sum = SMLAWB(SMULWB(w_Q24, int(side[n+1])), sum, pred0_Q13)                         // Q8
		sum = SMLAWB(sum, LSHIFT(int(x1[mid+n+1]), 11), pred1_Q13)                          // Q8
		x2[x2_ptr+n-1] = int16(SAT16(RSHIFT_ROUND(sum, 8)))
	}

	pred0_Q13 = -pred_Q13[0]
	pred1_Q13 = -pred_Q13[1]
	w_Q24 = LSHIFT(width_Q14, 10)
	for n = STEREO_INTERP_LEN_MS * fs_kHz; n < frame_length; n++ {
		sum = LSHIFT(ADD_LSHIFT32(int(x1[mid+n])+int(x1[mid+n+2]), int(x1[mid+n+1]), 1), 9) // Q11
		sum = SMLAWB(SMULWB(w_Q24, int(side[n+1])), sum, pred0_Q13)                         // Q8
		sum = SMLAWB(sum, LSHIFT(int(x1[mid+n+1]), 11), pred1_Q13)                          // Q8
		x2[x2_ptr+n-1] = int16(SAT16(RSHIFT_ROUND(sum, 8)))
	}
	state.pred_prev_Q13[0] = int16(pred_Q13[0])
	state.pred_prev_Q13[1] = int16(pred_Q13[1])
	state.width_prev_Q14 = int16(width_Q14)
}

func silk_stereo_MS_to_LR(
	state *StereoDecodeState,
	x1 []int16,
	x1_ptr int,
	x2 []int16,
	x2_ptr int,
	pred_Q13 []int,
	fs_kHz int,
	frame_length int) {

	var n, denom_Q16, delta0_Q13, delta1_Q13 int
	var sum, diff, pred0_Q13, pred1_Q13 int

	// Buffering
	copy(x1[x1_ptr:], state.sMid[:])
	copy(x2[x2_ptr:], state.sSide[:])
	copy(state.sMid[:], x1[x1_ptr+frame_length:])
	copy(state.sSide[:], x2[x2_ptr+frame_length:])

	// Interpolate predictors and add prediction to side channel
	pred0_Q13 = int(state.pred_prev_Q13[0])
	pred1_Q13 = int(state.pred_prev_Q13[1])
	denom_Q16 = DIV32_16(1<<16, STEREO_INTERP_LEN_MS*fs_kHz)
	delta0_Q13 = RSHIFT_ROUND(SMULBB(pred_Q13[0]-int(state.pred_prev_Q13[0]), denom_Q16), 16)
	delta1_Q13 = RSHIFT_ROUND(SMULBB(pred_Q13[1]-int(state.pred_prev_Q13[1]), denom_Q16), 16)
	for n = 0; n < STEREO_INTERP_LEN_MS*fs_kHz; n++ {
		pred0_Q13 += delta0_Q13
		pred1_Q13 += delta1_Q13
		sum = LSHIFT(ADD_LSHIFT32(int(x1[x1_ptr+n])+int(x1[x1_ptr+n+2]), int(x1[x1_ptr+n+1]), 1), 9) // Q11
		sum = SMLAWB(LSHIFT(int(x2[x2_ptr+n+1]), 8), sum, pred0_Q13)                                 // Q8
		sum = SMLAWB(sum, LSHIFT(int(x1[x1_ptr+n+1]), 11), pred1_Q13)                                // Q8
		x2[x2_ptr+n+1] = int16(SAT16(RSHIFT_ROUND(sum, 8)))
	}
	pred0_Q13 = pred_Q13[0]
	pred1_Q13 = pred_Q13[1]
	for n = STEREO_INTERP_LEN_MS * fs_kHz; n < frame_length; n++ {
		sum = LSHIFT(ADD_LSHIFT32(int(x1[x1_ptr+n])+int(x1[x1_ptr+n+2]), int(x1[x1_ptr+n+1]), 1), 9) // Q11
		sum = SMLAWB(LSHIFT(int(x2[x2_ptr+n+1]), 8), sum, pred0_Q13)                                 // Q8
		sum = SMLAWB(sum, LSHIFT(int(x1[x1_ptr+n+1]), 11), pred1_Q13)                                // Q8
		x2[x2_ptr+n+1] = int16(SAT16(RSHIFT_ROUND(sum, 8)))
	}
	state.pred_prev_Q13[0] = int16(pred_Q13[0])
	state.pred_prev_Q13[1] = int16(pred_Q13[1])

	// Convert to left/right signals
	for n = 0; n < frame_length; n++ {
		sum = int(x1[x1_ptr+n+1]) + int(x2[x2_ptr+n+1])
		diff = int(x1[x1_ptr+n+1]) - int(x2[x2_ptr+n+1])
		x1[x1_ptr+n+1] = int16(SAT16(sum))
		x2[x2_ptr+n+1] = int16(SAT16(diff))
	}
}

func silk_stereo_quant_pred(pred_Q13 []int, ix [][]byte) {
	var i, j, n int
	var low_Q13, step_Q13, lvl_Q13, err_min_Q13, err_Q13, quant_pred_Q13 int

	// Clear ix
	for i := range ix {
		for j := range ix[i] {
			ix[i][j] = 0
		}
	}

	// Quantize
	for n = 0; n < 2; n++ {
		done := false
		err_min_Q13 = math.MaxInt32

		// Brute-force search over quantization levels
		for i = 0; !done && i < STEREO_QUANT_TAB_SIZE-1; i++ {
			low_Q13 = int(silk_stereo_pred_quant_Q13[i])
			step_Q13 = SMULWB(int(silk_stereo_pred_quant_Q13[i+1]-low_Q13),
				int(0.5/STEREO_QUANT_SUB_STEPS*(1<<16)+0.5))

			for j = 0; !done && j < STEREO_QUANT_SUB_STEPS; j++ {
				lvl_Q13 = SMLABB(low_Q13, step_Q13, 2*j+1)
				err_Q13 = abs(pred_Q13[n] - lvl_Q13)
				if err_Q13 < err_min_Q13 {
					err_min_Q13 = err_Q13
					quant_pred_Q13 = lvl_Q13
					ix[n][0] = byte(i)
					ix[n][1] = byte(j)
				} else {
					// Error increasing, past the optimum
					done = true
				}
			}
		}

		ix[n][2] = byte(DIV32_16(int(ix[n][0]), 3))
		ix[n][0] -= ix[n][2] * 3
		pred_Q13[n] = quant_pred_Q13
	}

	// Subtract second from first predictor
	pred_Q13[0] -= pred_Q13[1]
}

// Helper functions
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max_int(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Note: The following functions need to be implemented elsewhere:
// DIV32_16, SMULWB, SMLABB, RSHIFT32, RSHIFT, LSHIFT,
// ADD_LSHIFT32, SUB_LSHIFT32, SQRT_APPROX, SAT16,
// RSHIFT_ROUND, DIV32_varQ, LIMIT, MUL, SMULBB, SMLAWB
// along with the silk_stereo_pred_quant_Q13 table and iCDF tables
