// Package silk implements SILK audio codec functionality.
// This file contains decoder control structures and methods.
package silk

// DecoderControl holds prediction and coding parameters for SILK decoder.
// This is a direct translation from Java to Go with idiomatic Go improvements.
type DecoderControl struct {
	// Prediction and coding parameters
	PitchL   [MAX_NB_SUBFR]int // Pitch lag for each subframe
	GainsQ16 [MAX_NB_SUBFR]int // Quantized gains for each subframe in Q16 format

	// Holds interpolated and final coefficients
	PredCoefQ12 [2][MAX_LPC_ORDER]int16         // Prediction coefficients in Q12 format (2 sets)
	LTPCoefQ14  [LTP_ORDER * MAX_NB_SUBFR]int16 // LTP coefficients in Q14 format
	LTPScaleQ14 int                             // LTP scaling factor in Q14 format
}

// Reset initializes all decoder control parameters to their default values.
// This is the Go equivalent of the Java Reset() method.
func (dc *DecoderControl) Reset() {
	// In Go, we can simply use the zero value initialization by creating a new struct
	// However, for demonstration, we'll explicitly zero the fields to match the Java behavior

	// Zero out pitch lags
	for i := range dc.PitchL {
		dc.PitchL[i] = 0
	}

	// Zero out gains
	for i := range dc.GainsQ16 {
		dc.GainsQ16[i] = 0
	}

	// Zero out prediction coefficients
	for i := range dc.PredCoefQ12 {
		for j := range dc.PredCoefQ12[i] {
			dc.PredCoefQ12[i][j] = 0
		}
	}

	// Zero out LTP coefficients
	for i := range dc.LTPCoefQ14 {
		dc.LTPCoefQ14[i] = 0
	}

	// Reset LTP scale
	dc.LTPScaleQ14 = 0
}
