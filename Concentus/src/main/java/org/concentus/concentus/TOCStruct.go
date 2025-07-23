package concentus

import "opustest/silk"

// TOCStruct represents a Table of Contents structure for audio packets
type TOCStruct struct {
	// VADFlag indicates voice activity for the entire packet
	VADFlag int

	// VADFlags contains voice activity indicators for each frame in the packet
	VADFlags [silk.SILK_MAX_FRAMES_PER_PACKET]int

	// InbandFECFlag indicates if the packet contains in-band Forward Error Correction
	InbandFECFlag int
}

// Reset clears all fields in the TOCStruct to their zero values
func (t *TOCStruct) Reset() {
	// In Go, we can simply reassign a new zero-value struct to reset all fields,
	// but since we want to maintain the same behavior as the Java version,
	// we'll explicitly zero out each field.

	t.VADFlag = 0
	t.InbandFECFlag = 0

	// For arrays in Go, we can't use memset equivalent directly.
	// The most efficient way is to loop through the array.
	// The compiler will optimize this to efficient memory operations.
	for i := range t.VADFlags {
		t.VADFlags[i] = 0
	}
}
