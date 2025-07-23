// Package silk implements variable cut-off low-pass filter state for audio processing.
// Original Java code copyright (c) 2006-2011 Skype Limited, ported to Go with optimizations.
package silk

// LPState represents the state of a variable cut-off low-pass filter.
type LPState struct {
	// In_LP_State contains the low pass filter state (2 elements)
	In_LP_State [2]int32

	// transition_frame_no is a counter mapped to a cut-off frequency
	transition_frame_no int32

	// mode indicates operating mode:
	// <0: switch down, >0: switch up; 0: do nothing
	mode int32
}

// Reset clears the filter state and resets all parameters to zero.
func (s *LPState) Reset() {
	s.In_LP_State = [2]int32{0, 0}
	s.transition_frame_no = 0
	s.mode = 0
}

// LP_variable_cutoff applies a low-pass filter with variable cutoff frequency based on
// piece-wise linear interpolation between elliptic filters.
//
// frame:       Input/output signal (modified in place)
// frame_ptr:   Starting index in the frame buffer
// frame_length: Length of the frame to process
func (s *LPState) LP_variable_cutoff(frame []int16, frame_ptr int, frame_length int) {
	// Pre-allocate filter coefficient arrays with known sizes
	var B_Q28 [TRANSITION_NB]int32
	var A_Q28 [TRANSITION_NA]int32

	// Validate transition frame number
	if s.transition_frame_no < 0 || s.transition_frame_no > TRANSITION_FRAMES {
		panic("transition_frame_no out of bounds")
	}

	// Only process if in active mode
	if s.mode != 0 {
		// Calculate index and interpolation factor
		fac_Q16 := (TRANSITION_FRAMES - s.transition_frame_no) << (16 - 6)
		ind := fac_Q16 >> 16
		fac_Q16 -= ind << 16

		// Validate index
		if ind < 0 || ind >= TRANSITION_INT_NUM {
			panic("interpolation index out of bounds")
		}

		// Interpolate filter coefficients
		LP_interpolate_filter_taps(&B_Q28, &A_Q28, ind, fac_Q16)

		// Update transition frame number with bounds checking
		s.transition_frame_no += s.mode
		if s.transition_frame_no < 0 {
			s.transition_frame_no = 0
		} else if s.transition_frame_no > TRANSITION_FRAMES {
			s.transition_frame_no = TRANSITION_FRAMES
		}

		// Perform ARMA low-pass filtering
		// Note: Using compile-time assertions would be better, but Go doesn't have them.
		// We assume TRANSITION_NB == 3 and TRANSITION_NA == 2 as per original code.
		biquad_alt(
			frame, frame_ptr,
			&B_Q28, &A_Q28,
			&s.In_LP_State,
			frame, frame_ptr,
			frame_length, 1,
		)
	}
}
