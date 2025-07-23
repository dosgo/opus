package silk

// SideInfoIndices contains various indices used in SILK audio encoding
type SideInfoIndices struct {
	GainsIndices     [MAX_NB_SUBFR]byte
	LTPIndex         [MAX_NB_SUBFR]byte
	NLSFIndices      [MAX_LPC_ORDER + 1]byte
	LagIndex         int16
	ContourIndex     byte
	SignalType       byte
	QuantOffsetType  byte
	NLSFInterpCoefQ2 byte
	PERIndex         byte
	LTPScaleIndex    byte
	Seed             byte
}

// Reset clears all fields in the SideInfoIndices struct
func (s *SideInfoIndices) Reset() {
	// In Go, we can use array literals to zero out arrays efficiently
	s.GainsIndices = [MAX_NB_SUBFR]byte{}
	s.LTPIndex = [MAX_NB_SUBFR]byte{}
	s.NLSFIndices = [MAX_LPC_ORDER + 1]byte{}
	s.LagIndex = 0
	s.ContourIndex = 0
	s.SignalType = 0
	s.QuantOffsetType = 0
	s.NLSFInterpCoefQ2 = 0
	s.PERIndex = 0
	s.LTPScaleIndex = 0
	s.Seed = 0
}

// Assign copies all values from another SideInfoIndices struct
func (s *SideInfoIndices) Assign(other *SideInfoIndices) {
	// In Go, arrays are values so assignment performs a deep copy
	s.GainsIndices = other.GainsIndices
	s.LTPIndex = other.LTPIndex
	s.NLSFIndices = other.NLSFIndices
	s.LagIndex = other.LagIndex
	s.ContourIndex = other.ContourIndex
	s.SignalType = other.SignalType
	s.QuantOffsetType = other.QuantOffsetType
	s.NLSFInterpCoefQ2 = other.NLSFInterpCoefQ2
	s.PERIndex = other.PERIndex
	s.LTPScaleIndex = other.LTPScaleIndex
	s.Seed = other.Seed
}
