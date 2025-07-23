// Package silk implements SILK audio codec functionality.
package silk

// ResamplerState represents the state of a SILK audio resampler.
// This is a direct translation from the Java version with Go idioms applied.
type ResamplerState struct {
	// Note: In the original Java code, sIIR and sFIR_i32/sFIR_i16 were in a union.
	// In Go, we'll keep them as separate fields since Go doesn't have unions.
	// Only one of these will be used at any time based on the resampler configuration.

	// sIIR stores the IIR filter state (for IIR-based resampling)
	sIIR [MAX_IIR_ORDER]int32

	// sFIR_i32 stores the FIR filter state (32-bit version)
	sFIR_i32 [MAX_FIR_ORDER]int32

	// sFIR_i16 stores the FIR filter state (16-bit version)
	sFIR_i16 [MAX_FIR_ORDER]int16

	// delayBuf is a buffer for storing delayed samples
	delayBuf [48]int16

	// resamplerFunction identifies which resampling function to use
	resamplerFunction int

	// batchSize specifies the number of samples to process at once
	batchSize int

	// invRatio_Q16 represents the inverse resampling ratio in Q16 fixed-point format
	invRatio_Q16 uint32

	// FIR_Order specifies the order of the FIR filter
	FIR_Order int

	// FIR_Fracs specifies the number of fractional positions for FIR interpolation
	FIR_Fracs int

	// Fs_in_kHz is the input sample rate in kHz
	Fs_in_kHz int

	// Fs_out_kHz is the output sample rate in kHz
	Fs_out_kHz int

	// inputDelay specifies the input delay in samples
	inputDelay int

	// Coefs points to the filter coefficients
	Coefs []int16
}

// Reset initializes all resampler state fields to their zero values.
// This is equivalent to the Java version's Reset() method.
func (rs *ResamplerState) Reset() {
	// In Go, we can simply create a new zero-value struct and assign it
	// This is more efficient than manually zeroing each field
	*rs = ResamplerState{}
}

// Assign copies all fields from another ResamplerState.
// This implements similar functionality to the Java Assign() method.
func (rs *ResamplerState) Assign(other *ResamplerState) {
	// Copy simple fields
	rs.resamplerFunction = other.resamplerFunction
	rs.batchSize = other.batchSize
	rs.invRatio_Q16 = other.invRatio_Q16
	rs.FIR_Order = other.FIR_Order
	rs.FIR_Fracs = other.FIR_Fracs
	rs.Fs_in_kHz = other.Fs_in_kHz
	rs.Fs_out_kHz = other.Fs_out_kHz
	rs.inputDelay = other.inputDelay

	// Copy array fields
	rs.sIIR = other.sIIR
	rs.sFIR_i32 = other.sFIR_i32
	rs.sFIR_i16 = other.sFIR_i16
	rs.delayBuf = other.delayBuf

	// For the slice, we need to handle nil case and make a proper copy
	if other.Coefs != nil {
		rs.Coefs = make([]int16, len(other.Coefs))
		copy(rs.Coefs, other.Coefs)
	} else {
		rs.Coefs = nil
	}
}
