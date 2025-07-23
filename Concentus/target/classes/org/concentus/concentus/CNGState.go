// Package concentus provides functionality for audio codecs
package concentus

// CNGState represents the state for Comfort Noise Generation (CNG)
// This is used to generate background noise during silence periods in audio streams
type CNGState struct {
	// CNG_exc_buf_Q14 stores the excitation buffer in Q14 fixed-point format
	CNGExcBufQ14 [320]int32

	// CNG_smth_NLSF_Q15 stores the smoothed Normalized Line Spectral Frequencies in Q15 format
	CNGSmthNLSFQ15 [16]int16

	// CNG_synth_state maintains the synthesis filter state
	CNGSynthState [16]int32

	// CNG_smth_Gain_Q16 stores the smoothed gain in Q16 fixed-point format
	CNGSmthGainQ16 int32

	// rand_seed is the random number generator seed for noise generation
	RandSeed int32

	// fs_kHz indicates the sampling frequency in kHz
	FSkHz int32
}

// Reset initializes all CNG state variables to their default values
// This is equivalent to the Java Reset() method but follows Go naming conventions
func (c *CNGState) Reset() {
	// In Go, we can simply create a new zero-value array since arrays are value types
	// This is more efficient than explicitly zeroing each element
	c.CNGExcBufQ14 = [320]int32{}
	c.CNGSmthNLSFQ15 = [16]int16{}
	c.CNGSynthState = [16]int32{}
	c.CNGSmthGainQ16 = 0
	c.RandSeed = 0
	c.FSkHz = 0
}
