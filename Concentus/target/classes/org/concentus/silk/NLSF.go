package silk

import (
	"math"
)

// Constants
const (
	MAX_STABILIZE_LOOPS   = 20
	QA                    = 16
	BIN_DIV_STEPS_A2NLSF  = 3
	MAX_ITERATIONS_A2NLSF = 30
)

// NLSF implements normalized line spectrum frequency processing
type NLSF struct{}

// NewNLSF creates a new NLSF processor
func NewNLSF() *NLSF {
	return &NLSF{}
}

// NLSF_VQ computes quantization errors for an LPC_order element input vector for a VQ codebook
func (n *NLSF) NLSF_VQ(err_Q26 []int32, in_Q15 []int16, pCB_Q8 []int16, K, LPC_order int) {
	// Input validation
	if len(err_Q26) < K || len(in_Q15) < LPC_order || len(pCB_Q8) < K*LPC_order {
		return
	}
	if LPC_order > 16 || (LPC_order&1) != 0 {
		return
	}

	pCB_idx := 0
	for i := 0; i < K; i++ {
		sum_error_Q26 := int32(0)

		// Process pairs of elements for better SIMD-like optimization
		for m := 0; m < LPC_order; m += 2 {
			// Compute weighted squared quantization error for index m
			diff_Q15 := int32(in_Q15[m]) - int32(pCB_Q8[pCB_idx])<<7
			sum_error_Q30 := diff_Q15 * diff_Q15
			pCB_idx++

			// Compute for index m+1
			diff_Q15 = int32(in_Q15[m+1]) - int32(pCB_Q8[pCB_idx])<<7
			sum_error_Q30 += diff_Q15 * diff_Q15
			pCB_idx++

			sum_error_Q26 += sum_error_Q30 >> 4
		}

		err_Q26[i] = sum_error_Q26
	}
}

// NLSF_VQ_weights_laroia computes Laroia low complexity NLSF weights
func (n *NLSF) NLSF_VQ_weights_laroia(pNLSFW_Q_OUT []int16, pNLSF_Q15 []int16, D int) {
	if len(pNLSFW_Q_OUT) < D || len(pNLSF_Q15) < D || D <= 0 || (D&1) != 0 {
		return
	}

	// First value
	tmp1_int := maxInt(int(pNLSF_Q15[0]), 1)
	tmp1_int = (1 << (15 + NLSF_W_Q)) / tmp1_int
	tmp2_int := maxInt(int(pNLSF_Q15[1])-int(pNLSF_Q15[0]), 1)
	tmp2_int = (1 << (15 + NLSF_W_Q)) / tmp2_int
	pNLSFW_Q_OUT[0] = int16(minInt(tmp1_int+tmp2_int, math.MaxInt16))

	// Main loop
	for k := 1; k < D-1; k += 2 {
		tmp1_int = maxInt(int(pNLSF_Q15[k+1])-int(pNLSF_Q15[k]), 1)
		tmp1_int = (1 << (15 + NLSF_W_Q)) / tmp1_int
		pNLSFW_Q_OUT[k] = int16(minInt(tmp1_int+tmp2_int, math.MaxInt16))

		tmp2_int = maxInt(int(pNLSF_Q15[k+2])-int(pNLSF_Q15[k+1]), 1)
		tmp2_int = (1 << (15 + NLSF_W_Q)) / tmp2_int
		pNLSFW_Q_OUT[k+1] = int16(minInt(tmp1_int+tmp2_int, math.MaxInt16))
	}

	// Last value
	tmp1_int = maxInt((1<<15)-int(pNLSF_Q15[D-1]), 1)
	tmp1_int = (1 << (15 + NLSF_W_Q)) / tmp1_int
	pNLSFW_Q_OUT[D-1] = int16(minInt(tmp1_int+tmp2_int, math.MaxInt16))
}

// NLSF_residual_dequant returns RD value in Q30
func (n *NLSF) NLSF_residual_dequant(
	x_Q10 []int16,
	indices []byte,
	indices_ptr int,
	pred_coef_Q8 []int16,
	quant_step_size_Q16 int32,
	order int16) {

	out_Q10 := int32(0)
	for i := int(order) - 1; i >= 0; i-- {
		pred_Q10 := (out_Q10 * int32(pred_coef_Q8[i])) >> 8
		out_Q10 = int32(indices[indices_ptr+i]) << 10

		if out_Q10 > 0 {
			out_Q10 -= int32(NLSF_QUANT_LEVEL_ADJ * (1 << 10))
		} else if out_Q10 < 0 {
			out_Q10 += int32(NLSF_QUANT_LEVEL_ADJ * (1 << 10))
		}

		out_Q10 = pred_Q10 + ((out_Q10 * quant_step_size_Q16) >> 16)
		x_Q10[i] = int16(out_Q10)
	}
}

// NLSF_unpack unpacks predictor values and indices for entropy coding tables
func (n *NLSF) NLSF_unpack(ec_ix, pred_Q8 []int16, psNLSF_CB *NLSFCodebook, CB1_index int) {
	ec_sel_ptr := CB1_index * psNLSF_CB.order / 2

	for i := 0; i < psNLSF_CB.order; i += 2 {
		entry := psNLSF_CB.ec_sel[ec_sel_ptr]
		ec_sel_ptr++

		ec_ix[i] = int16(((entry >> 1) & 7) * (2*NLSF_QUANT_MAX_AMPLITUDE + 1))
		pred_Q8[i] = psNLSF_CB.pred_Q8[i+(entry&1)*(psNLSF_CB.order-1)]

		ec_ix[i+1] = int16(((entry >> 5) & 7) * (2*NLSF_QUANT_MAX_AMPLITUDE + 1))
		pred_Q8[i+1] = psNLSF_CB.pred_Q8[i+((entry>>4)&1)*(psNLSF_CB.order-1)+1]
	}
}

// NLSF_stabilize stabilizes a single input data vector
func (n *NLSF) NLSF_stabilize(NLSF_Q15, NDeltaMin_Q15 []int16, L int) {
	// Input validation
	if len(NLSF_Q15) < L || len(NDeltaMin_Q15) < L+1 {
		return
	}

	for loops := 0; loops < MAX_STABILIZE_LOOPS; loops++ {
		// Find smallest distance
		min_diff_Q15 := int32(NLSF_Q15[0]) - int32(NDeltaMin_Q15[0])
		I := 0

		// Middle elements
		for i := 1; i <= L-1; i++ {
			diff_Q15 := int32(NLSF_Q15[i]) - (int32(NLSF_Q15[i-1]) + int32(NDeltaMin_Q15[i]))
			if diff_Q15 < min_diff_Q15 {
				min_diff_Q15 = diff_Q15
				I = i
			}
		}

		// Last element
		diff_Q15 := (1 << 15) - (int32(NLSF_Q15[L-1]) + int32(NDeltaMin_Q15[L]))
		if diff_Q15 < min_diff_Q15 {
			min_diff_Q15 = diff_Q15
			I = L
		}

		// Check if smallest distance is non-negative
		if min_diff_Q15 >= 0 {
			return
		}

		if I == 0 {
			// Move away from lower limit
			NLSF_Q15[0] = NDeltaMin_Q15[0]
		} else if I == L {
			// Move away from higher limit
			NLSF_Q15[L-1] = int16((1 << 15) - int32(NDeltaMin_Q15[L]))
		} else {
			// Find lower extreme
			min_center_Q15 := 0
			for k := 0; k < I; k++ {
				min_center_Q15 += int(NDeltaMin_Q15[k])
			}
			min_center_Q15 += int(NDeltaMin_Q15[I]) >> 1

			// Find upper extreme
			max_center_Q15 := 1 << 15
			for k := L; k > I; k-- {
				max_center_Q15 -= int(NDeltaMin_Q15[k])
			}
			max_center_Q15 -= int(NDeltaMin_Q15[I]) >> 1

			// Move apart while keeping same center frequency
			center_freq_Q15 := limitInt32(
				(int32(NLSF_Q15[I-1])+int32(NLSF_Q15[I])+1)>>1,
				int32(min_center_Q15),
				int32(max_center_Q15),
			)
			NLSF_Q15[I-1] = int16(center_freq_Q15 - int32(NDeltaMin_Q15[I]>>1))
			NLSF_Q15[I] = int16(int32(NLSF_Q15[I-1]) + int32(NDeltaMin_Q15[I]))
		}
	}

	// Fallback method if loops exhausted
	if loops == MAX_STABILIZE_LOOPS {
		insertionSortIncreasingInt16(NLSF_Q15[:L])

		// First NLSF should be no less than NDeltaMin[0]
		NLSF_Q15[0] = int16(maxInt(int(NLSF_Q15[0]), int(NDeltaMin_Q15[0])))

		// Keep delta_min distance between NLSFs
		for i := 1; i < L; i++ {
			NLSF_Q15[i] = int16(maxInt(int(NLSF_Q15[i]), int(NLSF_Q15[i-1])+int(NDeltaMin_Q15[i])))
		}

		// Last NLSF should be no higher than 1 - NDeltaMin[L]
		NLSF_Q15[L-1] = int16(minInt(int(NLSF_Q15[L-1]), (1<<15)-int(NDeltaMin_Q15[L])))

		// Keep NDeltaMin distance between NLSFs
		for i := L - 2; i >= 0; i-- {
			NLSF_Q15[i] = int16(minInt(int(NLSF_Q15[i]), int(NLSF_Q15[i+1])-int(NDeltaMin_Q15[i+1])))
		}
	}
}

// Helper functions used in the translation
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func limitInt32(val, min, max int32) int32 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func insertionSortIncreasingInt16(arr []int16) {
	for i := 1; i < len(arr); i++ {
		key := arr[i]
		j := i - 1
		for j >= 0 && arr[j] > key {
			arr[j+1] = arr[j]
			j--
		}
		arr[j+1] = key
	}
}
