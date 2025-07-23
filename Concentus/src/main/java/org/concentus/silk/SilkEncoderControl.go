package silk

// EncoderControl represents the encoder control structure for SILK codec
type EncoderControl struct {
	// Prediction and coding parameters
	Gains_Q16     [MAX_NB_SUBFR]int32
	PredCoef_Q12  [2][MAX_LPC_ORDER]int16 // holds interpolated and final coefficients
	LTPCoef_Q14   [LTP_ORDER * MAX_NB_SUBFR]int16
	LTP_scale_Q14 int32
	pitchL        [MAX_NB_SUBFR]int32

	// Noise shaping parameters
	AR1_Q13            [MAX_NB_SUBFR * MAX_SHAPE_LPC_ORDER]int16
	AR2_Q13            [MAX_NB_SUBFR * MAX_SHAPE_LPC_ORDER]int16
	LF_shp_Q14         [MAX_NB_SUBFR]int32
	GainsPre_Q14       [MAX_NB_SUBFR]int32 // Packs two int16 coefficients per int32 value
	HarmBoost_Q14      [MAX_NB_SUBFR]int32
	Tilt_Q14           [MAX_NB_SUBFR]int32
	HarmShapeGain_Q14  [MAX_NB_SUBFR]int32
	Lambda_Q10         int32
	input_quality_Q14  int32
	coding_quality_Q14 int32

	// Measures
	sparseness_Q8    int32
	predGain_Q16     int32
	LTPredCodGain_Q7 int32

	// Residual energy per subframe
	ResNrg [MAX_NB_SUBFR]int32

	// Q domain for the residual energy > 0
	ResNrgQ [MAX_NB_SUBFR]int32

	// Parameters for CBR mode
	GainsUnq_Q16      [MAX_NB_SUBFR]int32
	lastGainIndexPrev uint8
}

// Reset initializes all encoder control fields to zero values
func (ec *EncoderControl) Reset() {
	// Clear all arrays and reset all fields to zero
	*ec = EncoderControl{}

	// Note: In Go, the above single assignment is more idiomatic and efficient than
	// individually zeroing each field. The compiler will optimize this to efficient
	// memory operations. This is cleaner than the Java version which required
	// explicit array zeroing for each field.

	// Alternative implementation (if needed for specific optimization):
	// We could use explicit loops or the copy() function, but the zero value
	// assignment is generally preferred in Go for simplicity and performance.
}
