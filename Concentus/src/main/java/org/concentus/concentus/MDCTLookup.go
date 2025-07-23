// Package concentus implements audio processing components.
//
// This file contains MDCT (Modified Discrete Cosine Transform) lookup tables.
// Original copyright notices are preserved from the Java version.
package concentus

// MDCTLookup contains precomputed data for MDCT operations.
// This is equivalent to the Java MDCTLookup class but follows Go conventions.
type MDCTLookup struct {
	n        int         // Size of the MDCT
	maxshift int         // Maximum shift value
	kfft     []*FFTState // Slice of FFT states (replaces Java array)
	trig     []int16     // Trigonometric lookup table (int16 replaces Java short)
}

// NewMDCTLookup creates and returns a new initialized MDCTLookup instance.
// This follows Go's convention of using constructor functions rather than
// direct struct initialization when initialization logic might be needed.
func NewMDCTLookup() *MDCTLookup {
	return &MDCTLookup{
		// In Go, slices are nil by default (equivalent to Java null)
		// and numeric types are zero-initialized, so no explicit
		// initialization is needed beyond creating the struct.
	}
}
