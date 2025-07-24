package opus

const (
	SCRATCH_SIZE   = 22
	SF_LENGTH_4KHZ = PE_SUBFR_LENGTH_MS * 4
	SF_LENGTH_8KHZ = PE_SUBFR_LENGTH_MS * 8
	MIN_LAG_4KHZ   = PE_MIN_LAG_MS * 4
	MIN_LAG_8KHZ   = PE_MIN_LAG_MS * 8
	MAX_LAG_4KHZ   = PE_MAX_LAG_MS * 4
	MAX_LAG_8KHZ   = PE_MAX_LAG_MS*8 - 1
	CSTRIDE_4KHZ   = MAX_LAG_4KHZ + 1 - MIN_LAG_4KHZ
	CSTRIDE_8KHZ   = MAX_LAG_8KHZ + 3 - (MIN_LAG_8KHZ - 2)
	D_COMP_MIN     = MIN_LAG_8KHZ - 3
	D_COMP_MAX     = MAX_LAG_8KHZ + 4
	D_COMP_STRIDE  = D_COMP_MAX - D_COMP_MIN
)

type silk_pe_stage3_vals struct {
	Values [PE_NB_STAGE3_LAGS]int
}

func silk_pitch_analysis_core(frame []int16, pitch_out []int, lagIndex *BoxedValueShort, contourIndex *BoxedValueByte, LTPCorr_Q15 *BoxedValueInt, prevLag int, search_thres1_Q16 int, search_thres2_Q13 int, Fs_kHz int, complexity int, nb_subfr int) int {
	frame_8kHz := make([]int16, (PE_LTP_MEM_LENGTH_MS+nb_subfr*PE_SUBFR_LENGTH_MS)*8)
	frame_4kHz := make([]int16, (PE_LTP_MEM_LENGTH_MS+nb_subfr*PE_SUBFR_LENGTH_MS)*4)
	filt_state := make([]int32, 6)
	var input_frame_ptr []int16
	var i, k, d, j int
	var C []int16
	xcorr32 := make([]int32, MAX_LAG_4KHZ-MIN_LAG_4KHZ+1)
	var basis []int16
	var basis_ptr int
	var target []int16
	var target_ptr int
	var cross_corr, normalizer, energy, shift, energy_basis, energy_target int32
	var Cmax, length_d_srch, length_d_comp int
	d_srch := make([]int, PE_D_SRCH_LENGTH)
	d_comp := make([]int16, D_COMP_STRIDE)
	var sum, threshold, lag_counter int
	var CBimax, CBimax_new, CBimax_old, lag, start_lag, end_lag, lag_new int
	var CCmax, CCmax_b, CCmax_new_b, CCmax_new int
	CC := make([]int, PE_NB_CBKS_STAGE2_EXT)
	var energies_st3, cross_corr_st3 []*silk_pe_stage3_vals
	frame_length := (PE_LTP_MEM_LENGTH_MS + nb_subfr*PE_SUBFR_LENGTH_MS) * Fs_kHz
	frame_length_8kHz := (PE_LTP_MEM_LENGTH_MS + nb_subfr*PE_SUBFR_LENGTH_MS) * 8
	frame_length_4kHz := (PE_LTP_MEM_LENGTH_MS + nb_subfr*PE_SUBFR_LENGTH_MS) * 4
	sf_length := PE_SUBFR_LENGTH_MS * Fs_kHz
	min_lag := PE_MIN_LAG_MS * Fs_kHz
	max_lag := PE_MAX_LAG_MS*Fs_kHz - 1

	if Fs_kHz == 16 {
		for i := range filt_state {
			filt_state[i] = 0
		}
		silk_resampler_down2(filt_state, frame_8kHz, frame, frame_length)
	} else if Fs_kHz == 12 {
		for i := range filt_state {
			filt_state[i] = 0
		}
		silk_resampler_down2_3(filt_state, frame_8kHz, frame, frame_length)
	} else {
		copy(frame_8kHz, frame[:frame_length_8kHz])
	}

	for i := range filt_state {
		filt_state[i] = 0
	}
	silk_resampler_down2(filt_state, frame_4kHz, frame_8kHz, frame_length_8kHz)

	for i := frame_length_4kHz - 1; i > 0; i-- {
		frame_4kHz[i] = int16(silk_ADD_SAT32(int32(frame_4kHz[i]), int32(frame_4kHz[i-1])))
	}

	energy_val, shift_val := silk_sum_sqr_shift(frame_4kHz, frame_length_4kHz)
	energy, shift = energy_val, shift_val
	if shift > 0 {
		shift = silk_RSHIFT(shift, 1)
		for i := 0; i < frame_length_4kHz; i++ {
			frame_4kHz[i] = int16(silk_RSHIFT_uint(uint32(frame_4kHz[i]), uint(shift)))
		}
	}

	C = make([]int16, nb_subfr*CSTRIDE_8KHZ)
	for i := range xcorr32 {
		xcorr32[i] = 0
	}
	for i := 0; i < (nb_subfr>>1)*CSTRIDE_4KHZ; i++ {
		C[i] = 0
	}
	target = frame_4kHz
	target_ptr = silk_LSHIFT(SF_LENGTH_4KHZ, 2)
	for k = 0; k < nb_subfr>>1; k++ {
		basis = target
		basis_ptr = target_ptr - MIN_LAG_4KHZ

		pitch_xcorr(target, target_ptr, target, target_ptr-MAX_LAG_4KHZ, xcorr32, SF_LENGTH_8KHZ, MAX_LAG_4KHZ-MIN_LAG_4KHZ+1)

		cross_corr = xcorr32[MAX_LAG_4KHZ-MIN_LAG_4KHZ]
		normalizer = silk_inner_prod_self(target, target_ptr, SF_LENGTH_8KHZ)
		normalizer = silk_ADD32(normalizer, silk_inner_prod_self(basis, basis_ptr, SF_LENGTH_8KHZ))
		normalizer = silk_ADD32(normalizer, int32(int16(SF_LENGTH_8KHZ*4000)))

		C[k*CSTRIDE_4KHZ+0] = int16(silk_DIV32_varQ(cross_corr, normalizer, 14))

		for d = MIN_LAG_4KHZ + 1; d <= MAX_LAG_4KHZ; d++ {
			basis_ptr--
			cross_corr = xcorr32[MAX_LAG_4KHZ-d]
			normalizer = silk_ADD32(normalizer,
				int32(int16(int32(basis[basis_ptr])*int32(basis[basis_ptr])-
					int32(basis[basis_ptr+SF_LENGTH_8KHZ])*int32(basis[basis_ptr+SF_LENGTH_8KHZ]))))
			C[k*CSTRIDE_4KHZ+(d-MIN_LAG_4KHZ)] = int16(silk_DIV32_varQ(cross_corr, normalizer, 14))
		}
		target_ptr += SF_LENGTH_8KHZ
	}

	if nb_subfr == PE_MAX_NB_SUBFR {
		for i := MAX_LAG_4KHZ; i >= MIN_LAG_4KHZ; i-- {
			sum = int(C[0*CSTRIDE_4KHZ+(i-MIN_LAG_4KHZ)] + C[1*CSTRIDE_4KHZ+(i-MIN_LAG_4KHZ)])
			sum = silk_SMLAWB(sum, sum, silk_LSHIFT(-i, 4))
			C[i-MIN_LAG_4KHZ] = int16(sum)
		}
	} else {
		for i := MAX_LAG_4KHZ; i >= MIN_LAG_4KHZ; i-- {
			sum = int(silk_LSHIFT(int32(C[i-MIN_LAG_4KHZ]), 1))
			sum = silk_SMLAWB(sum, sum, silk_LSHIFT(-i, 4))
			C[i-MIN_LAG_4KHZ] = int16(sum)
		}
	}

	length_d_srch = 4 + (complexity << 1)
	for i := 0; i < length_d_srch; i++ {
		d_srch[i] = i
	}
	silk_insertion_sort_decreasing_int16(C, d_srch, CSTRIDE_4KHZ, length_d_srch)

	Cmax = int(C[0])
	if Cmax < int(0.2*float64(1<<14)+0.5) {
		for i := range pitch_out {
			pitch_out[i] = 0
		}
		LTPCorr_Q15.Val = 0
		lagIndex.Val = 0
		contourIndex.Val = 0
		return 1
	}

	threshold = silk_SMULWB(search_thres1_Q16, Cmax)
	for i = 0; i < length_d_srch; i++ {
		if int(C[i]) > threshold {
			d_srch[i] = silk_LSHIFT(d_srch[i]+MIN_LAG_4KHZ, 1)
		} else {
			length_d_srch = i
			break
		}
	}

	for i := range d_comp {
		d_comp[i] = 0
	}
	for i := 0; i < length_d_srch; i++ {
		if d_srch[i]-D_COMP_MIN >= 0 && d_srch[i]-D_COMP_MIN < len(d_comp) {
			d_comp[d_srch[i]-D_COMP_MIN] = 1
		}
	}

	for i := D_COMP_MAX - 1; i >= MIN_LAG_8KHZ; i-- {
		if i-1-D_COMP_MIN >= 0 && i-2-D_COMP_MIN >= 0 {
			d_comp[i-D_COMP_MIN] += int16(d_comp[i-1-D_COMP_MIN] + d_comp[i-2-D_COMP_MIN])
		}
	}

	length_d_srch = 0
	for i := MIN_LAG_8KHZ; i < MAX_LAG_8KHZ+1; i++ {
		if i+1-D_COMP_MIN < len(d_comp) && d_comp[i+1-D_COMP_MIN] > 0 {
			d_srch[length_d_srch] = i
			length_d_srch++
		}
	}

	for i := D_COMP_MAX - 1; i >= MIN_LAG_8KHZ; i-- {
		if i-1-D_COMP_MIN >= 0 && i-2-D_COMP_MIN >= 0 && i-3-D_COMP_MIN >= 0 {
			d_comp[i-D_COMP_MIN] += int16(d_comp[i-1-D_COMP_MIN] + d_comp[i-2-D_COMP_MIN] + d_comp[i-3-D_COMP_MIN])
		}
	}

	length_d_comp = 0
	for i := MIN_LAG_8KHZ; i < D_COMP_MAX; i++ {
		if d_comp[i-D_COMP_MIN] > 0 {
			d_comp[length_d_comp] = int16(i - 2)
			length_d_comp++
		}
	}

	energy_val, shift_val = silk_sum_sqr_shift(frame_8kHz, frame_length_8kHz)
	energy, shift = energy_val, shift_val
	if shift > 0 {
		shift = silk_RSHIFT(shift, 1)
		for i := 0; i < frame_length_8kHz; i++ {
			frame_8kHz[i] = int16(silk_RSHIFT_uint(uint32(frame_8kHz[i]), uint(shift)))
		}
	}

	for i := range C {
		C[i] = 0
	}

	target = frame_8kHz
	target_ptr = PE_LTP_MEM_LENGTH_MS * 8
	for k = 0; k < nb_subfr; k++ {
		energy_target = silk_ADD32(silk_inner_prod_self(target, target_ptr, SF_LENGTH_8KHZ), 1)
		for j = 0; j < length_d_comp; j++ {
			d = int(d_comp[j])
			basis = target
			basis_ptr = target_ptr - d
			cross_corr = silk_inner_prod(target, target_ptr, basis, basis_ptr, SF_LENGTH_8KHZ)
			if cross_corr > 0 {
				energy_basis = silk_inner_prod_self(basis, basis_ptr, SF_LENGTH_8KHZ)
				idx := k*CSTRIDE_8KHZ + (d - (MIN_LAG_8KHZ - 2))
				C[idx] = int16(silk_DIV32_varQ(cross_corr, silk_ADD32(energy_target, energy_basis), 14))
			} else {
				idx := k*CSTRIDE_8KHZ + (d - (MIN_LAG_8KHZ - 2))
				C[idx] = 0
			}
		}
		target_ptr += SF_LENGTH_8KHZ
	}

	CCmax = -1 << 31
	CCmax_b = -1 << 31
	CBimax = 0
	lag = -1

	if prevLag > 0 {
		if Fs_kHz == 12 {
			prevLag = silk_DIV32_16(silk_LSHIFT(int32(prevLag), 1), 3)
		} else if Fs_kHz == 16 {
			prevLag = silk_RSHIFT(prevLag, 1)
		}
		prevLag_log2_Q7 = silk_lin2log(prevLag)
	} else {
		prevLag_log2_Q7 = 0
	}

	var Lag_CB_ptr [][]byte
	nb_cbk_search := 0
	if nb_subfr == PE_MAX_NB_SUBFR {
		Lag_CB_ptr = silk_CB_lags_stage2
		if Fs_kHz == 8 && complexity > SILK_PE_MIN_COMPLEX {
			nb_cbk_search = PE_NB_CBKS_STAGE2_EXT
		} else {
			nb_cbk_search = PE_NB_CBKS_STAGE2
		}
	} else {
		Lag_CB_ptr = silk_CB_lags_stage2_10_ms
		nb_cbk_search = PE_NB_CBKS_STAGE2_10MS
	}

	for k = 0; k < length_d_srch; k++ {
		d = d_srch[k]
		for j = 0; j < nb_cbk_search; j++ {
			CC[j] = 0
			for i = 0; i < nb_subfr; i++ {
				d_subfr := d + int(Lag_CB_ptr[i][j])
				idx := i*CSTRIDE_8KHZ + (d_subfr - (MIN_LAG_8KHZ - 2))
				CC[j] += int(C[idx])
			}
		}

		CCmax_new = -1 << 31
		CBimax_new = 0
		for i = 0; i < nb_cbk_search; i++ {
			if CC[i] > CCmax_new {
				CCmax_new = CC[i]
				CBimax_new = i
			}
		}

		lag_log2_Q7 = silk_lin2log(d)
		CCmax_new_b = CCmax_new - silk_RSHIFT(silk_SMULBB(nb_subfr*int(0.2*float64(1<<13)+0.5, lag_log2_Q7), 7))

		if prevLag > 0 {
			delta_lag_log2_sqr_Q7 = lag_log2_Q7 - prevLag_log2_Q7
			delta_lag_log2_sqr_Q7 = silk_RSHIFT(silk_SMULBB(delta_lag_log2_sqr_Q7, delta_lag_log2_sqr_Q7), 7)
			prev_lag_bias_Q13 = silk_RSHIFT(silk_SMULBB(nb_subfr*int(0.2*float64(1<<13)+0.5, LTPCorr_Q15.Val), 15))
			prev_lag_bias_Q13 = silk_DIV32(silk_MUL(prev_lag_bias_Q13, delta_lag_log2_sqr_Q7), delta_lag_log2_sqr_Q7+int(0.5*float64(1<<7)+0.5))
			CCmax_new_b -= prev_lag_bias_Q13
		}

		if CCmax_new_b > CCmax_b && CCmax_new > silk_SMULBB(nb_subfr, search_thres2_Q13) && int(Lag_CB_ptr[0][CBimax_new]) <= MIN_LAG_8KHZ {
			CCmax_b = CCmax_new_b
			CCmax = CCmax_new
			lag = d
			CBimax = CBimax_new
		}
	}

	if lag == -1 {
		for i := range pitch_out {
			pitch_out[i] = 0
		}
		LTPCorr_Q15.Val = 0
		lagIndex.Val = 0
		contourIndex.Val = 0
		return 1
	}

	LTPCorr_Q15.Val = silk_LSHIFT(silk_DIV32_16(int32(CCmax), int16(nb_subfr)), 2)

	if Fs_kHz > 8 {
		var scratch_mem []int16
		energy_val, shift_val = silk_sum_sqr_shift(frame, frame_length)
		energy, shift = energy_val, shift_val
		if shift > 0 {
			scratch_mem = make([]int16, frame_length)
			shift = silk_RSHIFT(shift, 1)
			for i = 0; i < frame_length; i++ {
				scratch_mem[i] = int16(silk_RSHIFT_uint(uint32(frame[i]), uint(shift)))
			}
			input_frame_ptr = scratch_mem
		} else {
			input_frame_ptr = frame
		}

		CBimax_old = CBimax
		if Fs_kHz == 12 {
			lag = silk_RSHIFT(silk_SMULBB(int32(lag), 3), 1)
		} else if Fs_kHz == 16 {
			lag = silk_LSHIFT(lag, 1)
		} else {
			lag = silk_SMULBB(lag, 3)
		}
		lag = silk_LIMIT_int(lag, min_lag, max_lag)
		start_lag = silk_max_int(lag-2, min_lag)
		end_lag = silk_min_int(lag+2, max_lag)
		lag_new = lag
		CBimax = 0
		for k = 0; k < nb_subfr; k++ {
			pitch_out[k] = lag + 2*int(Lag_CB_ptr[k][CBimax_old])
		}

		if nb_subfr == PE_MAX_NB_SUBFR {
			nb_cbk_search = int(silk_nb_cbk_searchs_stage3[complexity])
			Lag_CB_ptr = silk_CB_lags_stage3
		} else {
			nb_cbk_search = PE_NB_CBKS_STAGE3_10MS
			Lag_CB_ptr = silk_CB_lags_stage3_10_ms
		}

		energies_st3 = make([]*silk_pe_stage3_vals, nb_subfr*nb_cbk_search)
		cross_corr_st3 = make([]*silk_pe_stage3_vals, nb_subfr*nb_cbk_search)
		for i := range energies_st3 {
			energies_st3[i] = &silk_pe_stage3_vals{}
			cross_corr_st3[i] = &silk_pe_stage3_vals{}
		}
		silk_P_Ana_calc_corr_st3(cross_corr_st3, input_frame_ptr, start_lag, sf_length, nb_subfr, complexity)
		silk_P_Ana_calc_energy_st3(energies_st3, input_frame_ptr, start_lag, sf_length, nb_subfr, complexity)

		lag_counter = 0
		contour_bias_Q15 = silk_DIV32_16(int(0.05*float64(1<<15)+0.5, lag))

		target = input_frame_ptr
		target_ptr = PE_LTP_MEM_LENGTH_MS * Fs_kHz
		energy_target = silk_ADD32(silk_inner_prod_self(target, target_ptr, nb_subfr*sf_length), 1)
		for d = start_lag; d <= end_lag; d++ {
			for j = 0; j < nb_cbk_search; j++ {
				cross_corr = 0
				energy = energy_target
				for k = 0; k < nb_subfr; k++ {
					idx := k*nb_cbk_search + j
					cross_corr += cross_corr_st3[idx].Values[lag_counter]
					energy += energies_st3[idx].Values[lag_counter]
				}
				if cross_corr > 0 {
					CCmax_new = silk_DIV32_varQ(cross_corr, energy, 14)
					diff := 32767 - silk_MUL(contour_bias_Q15, j)
					CCmax_new = silk_SMULWB(CCmax_new, diff)
				} else {
					CCmax_new = 0
				}

				if CCmax_new > CCmax && (d+int(Lag_CB_ptr[0][j])) <= max_lag {
					CCmax = CCmax_new
					lag_new = d
					CBimax = j
				}
			}
			lag_counter++
		}

		for k = 0; k < nb_subfr; k++ {
			pitch_out[k] = lag_new + int(Lag_CB_ptr[k][CBimax])
			pitch_out[k] = silk_LIMIT(pitch_out[k], min_lag, PE_MAX_LAG_MS*Fs_kHz)
		}
		lagIndex.Val = int16(lag_new - min_lag)
		contourIndex.Val = byte(CBimax)
	} else {
		for k = 0; k < nb_subfr; k++ {
			pitch_out[k] = lag + int(Lag_CB_ptr[k][CBimax])
			pitch_out[k] = silk_LIMIT(pitch_out[k], MIN_LAG_8KHZ, PE_MAX_LAG_MS*8)
		}
		lagIndex.Val = int16(lag - MIN_LAG_8KHZ)
		contourIndex.Val = byte(CBimax)
	}

	return 0
}

func silk_P_Ana_calc_corr_st3(cross_corr_st3 []*silk_pe_stage3_vals, frame []int16, start_lag int, sf_length int, nb_subfr int, complexity int) {
	var target_ptr int
	var i, j, k, lag_counter, lag_low, lag_high int
	var nb_cbk_search, delta, idx int
	scratch_mem := make([]int32, SCRATCH_SIZE)
	xcorr32 := make([]int32, SCRATCH_SIZE)
	var Lag_range_ptr, Lag_CB_ptr [][]byte

	if nb_subfr == PE_MAX_NB_SUBFR {
		Lag_range_ptr = silk_Lag_range_stage3[complexity]
		Lag_CB_ptr = silk_CB_lags_stage3
		nb_cbk_search = int(silk_nb_cbk_searchs_stage3[complexity])
	} else {
		Lag_range_ptr = silk_Lag_range_stage3_10_ms
		Lag_CB_ptr = silk_CB_lags_stage3_10_ms
		nb_cbk_search = PE_NB_CBKS_STAGE3_10MS
	}

	target_ptr = silk_LSHIFT(sf_length, 2)
	for k = 0; k < nb_subfr; k++ {
		lag_counter = 0
		lag_low = int(Lag_range_ptr[k][0])
		lag_high = int(Lag_range_ptr[k][1])
		pitch_xcorr(frame, target_ptr, frame, target_ptr-start_lag-lag_high, xcorr32, sf_length, lag_high-lag_low+1)
		for j = lag_low; j <= lag_high; j++ {
			scratch_mem[lag_counter] = xcorr32[lag_high-j]
			lag_counter++
		}

		delta = int(Lag_range_ptr[k][0])
		for i = 0; i < nb_cbk_search; i++ {
			idx = int(Lag_CB_ptr[k][i]) - delta
			for j = 0; j < PE_NB_STAGE3_LAGS; j++ {
				cross_corr_st3[k*nb_cbk_search+i].Values[j] = scratch_mem[idx+j]
			}
		}
		target_ptr += sf_length
	}
}

func silk_P_Ana_calc_energy_st3(energies_st3 []*silk_pe_stage3_vals, frame []int16, start_lag int, sf_length int, nb_subfr int, complexity int) {
	var target_ptr, basis_ptr int
	var energy int32
	var k, i, j, lag_counter int
	var nb_cbk_search, delta, idx, lag_diff int
	scratch_mem := make([]int32, SCRATCH_SIZE)
	var Lag_range_ptr, Lag_CB_ptr [][]byte

	if nb_subfr == PE_MAX_NB_SUBFR {
		Lag_range_ptr = silk_Lag_range_stage3[complexity]
		Lag_CB_ptr = silk_CB_lags_stage3
		nb_cbk_search = int(silk_nb_cbk_searchs_stage3[complexity])
	} else {
		Lag_range_ptr = silk_Lag_range_stage3_10_ms
		Lag_CB_ptr = silk_CB_lags_stage3_10_ms
		nb_cbk_search = PE_NB_CBKS_STAGE3_10MS
	}

	target_ptr = silk_LSHIFT(sf_length, 2)
	for k = 0; k < nb_subfr; k++ {
		lag_counter = 0
		basis_ptr = target_ptr - (start_lag + int(Lag_range_ptr[k][0]))
		energy = silk_inner_prod_self(frame, basis_ptr, sf_length)
		scratch_mem[lag_counter] = energy
		lag_counter++

		lag_diff = int(Lag_range_ptr[k][1]) - int(Lag_range_ptr[k][0]) + 1
		for i = 1; i < lag_diff; i++ {
			energy -= int32(frame[basis_ptr+sf_length-i]) * int32(frame[basis_ptr+sf_length-i])
			energy = silk_ADD_SAT32(energy, int32(frame[basis_ptr-i])*int32(frame[basis_ptr-i]))
			scratch_mem[lag_counter] = energy
			lag_counter++
		}

		delta = int(Lag_range_ptr[k][0])
		for i = 0; i < nb_cbk_search; i++ {
			idx = int(Lag_CB_ptr[k][i]) - delta
			for j = 0; j < PE_NB_STAGE3_LAGS; j++ {
				energies_st3[k*nb_cbk_search+i].Values[j] = scratch_mem[idx+j]
			}
		}
		target_ptr += sf_length
	}
}

func pitch_xcorr(target []int16, t_start int, basis []int16, b_start int, xcorr []int32, len int, max_lag int) {
	for lag := 0; lag < max_lag; lag++ {
		var sum int64
		for i := 0; i < len; i++ {
			sum += int64(target[t_start+i]) * int64(basis[b_start+i-lag])
		}
		xcorr[lag] = int32(silk_SAT32(sum))
	}
}

func silk_resampler_down2(state []int32, out, in []int16, len int)   {}
func silk_resampler_down2_3(state []int32, out, in []int16, len int) {}

var silk_CB_lags_stage2 = [][]byte{}
var silk_CB_lags_stage2_10_ms = [][]byte{}
var silk_nb_cbk_searchs_stage3 = []byte{}
var silk_CB_lags_stage3 = [][]byte{}
var silk_CB_lags_stage3_10_ms = [][]byte{}
var silk_Lag_range_stage3 = [][][]byte{}
var silk_Lag_range_stage3_10_ms = [][][]byte{}
