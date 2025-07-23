// Copyright (c) 2007-2008 CSIRO
// Copyright (c) 2007-2011 Xiph.Org Foundation
// Originally written by Jean-Marc Valin, Gregory Maxwell, Koen Vos,
// Timothy B. Terriberry, and the Opus open-source contributors
// Ported to Java by Logan Stromberg
// Translated to Go by [Your Name]

// Package concentus provides Opus codec functionality.
package concentus

// OpusFramesize represents the possible frame sizes for Opus encoding.
type OpusFramesize int

// Constants representing different Opus frame sizes.
// Note: In Go, we use camelCase for exported constants following Go conventions.
const (
	// FramesizeUnknown represents an error state
	FramesizeUnknown OpusFramesize = iota // iota starts at 0

	// FramesizeArg selects frame size from the argument (default)
	FramesizeArg

	// Framesize2_5ms uses 2.5 ms frames
	Framesize2_5ms

	// Framesize5ms uses 5 ms frames
	Framesize5ms

	// Framesize10ms uses 10 ms frames
	Framesize10ms

	// Framesize20ms uses 20 ms frames
	Framesize20ms

	// Framesize40ms uses 40 ms frames
	Framesize40ms

	// Framesize60ms uses 60 ms frames
	Framesize60ms

	// FramesizeVariable should not be used - not fully implemented. Optimizes frame size dynamically.
	FramesizeVariable
)

// GetOrdinal returns the ordinal value of the frame size.
// This is a method on the type rather than a separate helper class,
// which is more idiomatic in Go.
func (of OpusFramesize) GetOrdinal() int {
	// In Go, the enum values are already sequential integers starting from 0,
	// but since the Java version starts with 1 for FramesizeArg, we adjust accordingly.
	if of >= FramesizeArg && of <= FramesizeVariable {
		return int(of) // returns 1-8 for valid values
	}
	return -1 // for unknown/invalid values (including FramesizeUnknown)
}
