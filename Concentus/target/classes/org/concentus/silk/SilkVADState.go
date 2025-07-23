// Package silk implements SILK codec functionality.
// This file contains the VAD (Voice Activity Detection) state structure.
package silk

// VADState represents the state of the Voice Activity Detector.
// This is a direct translation from the Java version with Go idioms applied.
type VADState struct {
	// Analysis filterbank states for different frequency bands:
	AnaState  [2]int // 0-8 kHz
	AnaState1 [2]int // 0-4 kHz
	AnaState2 [2]int // 0-2 kHz

	// Subframe energies
	XnrgSubfr [VAD_N_BANDS]int

	// Smoothed energy level in each band
	NrgRatioSmth_Q8 [VAD_N_BANDS]int

	// State of differentiator in the lowest band
	HPstate int16 // Using int16 to match Java's short type

	// Noise-related fields
	NL             [VAD_N_BANDS]int // Noise energy level in each band
	inv_NL         [VAD_N_BANDS]int // Inverse noise energy level
	NoiseLevelBias [VAD_N_BANDS]int // Noise level estimator bias/offset

	// Frame counter used in the initial phase
	counter int
}

// Reset initializes all state fields to zero values.
// This is more efficient in Go than the Java version since zero-value initialization
// is automatic for most types, but we implement it explicitly for clarity and consistency.
func (s *VADState) Reset() {
	// In Go, arrays are value types and don't need explicit initialization to zero
	// when declared, but we reset them here to match the Java behavior exactly.
	s.AnaState = [2]int{}
	s.AnaState1 = [2]int{}
	s.AnaState2 = [2]int{}
	s.XnrgSubfr = [VAD_N_BANDS]int{}
	s.NrgRatioSmth_Q8 = [VAD_N_BANDS]int{}
	s.HPstate = 0
	s.NL = [VAD_N_BANDS]int{}
	s.inv_NL = [VAD_N_BANDS]int{}
	s.NoiseLevelBias = [VAD_N_BANDS]int{}
	s.counter = 0
}
