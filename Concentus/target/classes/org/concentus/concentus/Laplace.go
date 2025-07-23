// Package concentus implements audio codec utilities
package concentus

// Laplace implements Laplace distribution probability functions for entropy coding.
// This is a direct translation from the original Java implementation with Go idioms applied.
type Laplace struct{}

const (
	// The minimum probability of an energy delta (out of 32768)
	laplaceLogMinP = 0
	laplaceMinP    = 1 << laplaceLogMinP

	// The minimum number of guaranteed representable energy deltas (in one direction)
	laplaceNMin = 16
)

// ecLaplaceGetFreq1 calculates the frequency for Laplace distribution encoding.
// When called, decay is positive and at most 11456.
func (l *Laplace) ecLaplaceGetFreq1(fs0 uint32, decay int) uint32 {
	// Cap to 32-bit unsigned integer range (Go doesn't need explicit capping like Java)
	ft := 32768 - laplaceMinP*(2*laplaceNMin) - fs0
	return uint32((ft * (16384 - uint32(decay))) >> 15)
}

// Encode encodes a value using Laplace distribution
// enc: Entropy coder interface
// value: Pointer to integer value to encode (will be modified during encoding)
// fs: Frequency state
// decay: Decay parameter (positive and at most 11456)
func (l *Laplace) Encode(enc entropy.EntropyCoder, value *int, fs uint32, decay int) {
	var fl uint32
	val := *value

	if val != 0 {
		var s int
		// Calculate sign and absolute value
		s = 0
		if val < 0 {
			s = -1
		}
		val = (val + s) ^ s

		fl = fs
		fs = l.ecLaplaceGetFreq1(fs, decay)

		// Search the decaying part of the PDF
		for i := 1; fs > 0 && i < val; i++ {
			fs *= 2
			fl = fl + fs + 2*laplaceMinP
			fs = uint32((int64(fs) * int64(decay)) >> 15)
		}

		// Everything beyond has probability laplaceMinP
		if fs == 0 {
			var di int
			ndiMax := (32768 - fl + laplaceMinP - 1) >> laplaceLogMinP
			ndiMax = (ndiMax - uint32(s)) >> 1
			di = min(val-i, int(ndiMax)-1)
			fl = fl + uint32(2*di+1+s)*laplaceMinP
			fs = min(laplaceMinP, 32768-fl)
			*value = (i + di + s) ^ s
		} else {
			fs += laplaceMinP
			if s == 0 {
				fl += fs
			}
		}

		// Assertions converted to panics in Go (only active in debug builds)
		debugAssert(fl+fs <= 32768, "fl+fs overflow")
		debugAssert(fs > 0, "fs must be positive")
	}

	enc.EncodeBin(fl, fl+fs, 15)
}

// Decode decodes a value using Laplace distribution
// dec: Entropy decoder interface
// fs: Frequency state
// decay: Decay parameter (positive and at most 11456)
// Returns the decoded value
func (l *Laplace) Decode(dec EntropyDecoder, fs uint32, decay int) int {
	val := 0
	var fl uint32

	fm := dec.DecodeBin(15)
	fl = 0

	if fm >= fs {
		val++
		fl = fs
		fs = l.ecLaplaceGetFreq1(fs, decay) + laplaceMinP

		// Search the decaying part of the PDF
		for fs > laplaceMinP && fm >= fl+2*fs {
			fs *= 2
			fl += fs
			fs = uint32((int64(fs-2*laplaceMinP) * int64(decay)) >> 15)
			fs += laplaceMinP
			val++
		}

		// Everything beyond has probability laplaceMinP
		if fs <= laplaceMinP {
			di := (fm - fl) >> (laplaceLogMinP + 1)
			val += int(di)
			fl += 2 * uint32(di) * laplaceMinP
		}

		if fm < fl+fs {
			val = -val
		} else {
			fl += fs
		}
	}

	// Assertions converted to panics in Go (only active in debug builds)
	debugAssert(fl < 32768, "fl overflow")
	debugAssert(fs > 0, "fs must be positive")
	debugAssert(fl <= fm, "fl <= fm violation")
	debugAssert(fm < min(fl+fs, 32768), "fm range violation")

	dec.DecUpdate(fl, min(fl+fs, 32768), 32768)
	return val
}

// Helper functions

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minU32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// debugAssert is a helper for debugging assertions
func debugAssert(condition bool, message string) {
	if !condition {
		panic("Assertion failed: " + message)
	}
}
