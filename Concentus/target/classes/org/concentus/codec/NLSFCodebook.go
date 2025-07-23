// Package codec implements NLSF (Normalized Line Spectral Frequencies) codebook structures
// Original Java code copyright (c) 2006-2011 Skype Limited, ported to Go with idiomatic changes
package codec

// NLSFCodebook represents a structure containing NLSF codebook information
// for quantization and entropy coding of line spectral frequencies.
//
// The Go version makes several idiomatic improvements:
// 1. Unexported fields (lowercase names) since they should be accessed via methods
// 2. Proper Go documentation format
// 3. nil slices instead of null arrays
// 4. Reset method converted to a pointer receiver
type NLSFCodebook struct {
	// Number of vectors in the codebook
	nVectors int16

	// Order of the codebook
	order int16

	// Quantization step size in Q16 format
	quantStepSizeQ16 int16

	// Inverse quantization step size in Q6 format
	invQuantStepSizeQ6 int16

	// Codebook vectors in Q8 format
	cb1NLSFQ8 []int16

	// Cumulative distribution function for codebook selection
	cb1ICDF []int16

	// Backward predictor coefficients [order]
	predQ8 []int16

	// Indices to entropy coding tables [order]
	ecSel []int16

	// Entropy coding CDF tables
	ecICDF []int16

	// Entropy coding rates in Q5 format
	ecRatesQ5 []int16

	// Minimum delta values in Q15 format
	deltaMinQ15 []int16
}

// Reset clears all codebook fields to their zero values
// In Go, we use a pointer receiver since we're modifying the struct
func (c *NLSFCodebook) Reset() {
	c.nVectors = 0
	c.order = 0
	c.quantStepSizeQ16 = 0
	c.invQuantStepSizeQ6 = 0
	c.cb1NLSFQ8 = nil
	c.cb1ICDF = nil
	c.predQ8 = nil
	c.ecSel = nil
	c.ecICDF = nil
	c.ecRatesQ5 = nil
	c.deltaMinQ15 = nil
}
