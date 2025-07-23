package concentus

// StereoEncodeState represents the state for stereo encoding
type StereoEncodeState struct {
	predPrevQ13   [2]int16     // Prediction from previous frame (Q13)
	sMid          [2]int16     // Mid signal
	sSide         [2]int16     // Side signal
	midSideAmpQ0  [4]int32     // Mid/side amplitude (Q0)
	smthWidthQ14  int16        // Smoothed width (Q14)
	widthPrevQ14  int16        // Previous width (Q14)
	silentSideLen int16        // Length of silent side
	predIx        [][2][3]int8 // Prediction indices [frame][channel][index]
	midOnlyFlags  []int8       // Flags indicating mid-only frames
}

// NewStereoEncodeState creates a new StereoEncodeState with initialized values
func NewStereoEncodeState() *StereoEncodeState {
	s := &StereoEncodeState{
		predIx:       make([][2][3]int8, MAX_FRAMES_PER_PACKET),
		midOnlyFlags: make([]int8, MAX_FRAMES_PER_PACKET),
	}
	s.Reset()
	return s
}

// Reset initializes or resets all state variables to their default values
func (s *StereoEncodeState) Reset() {
	// Clear fixed-size arrays
	s.predPrevQ13 = [2]int16{}
	s.sMid = [2]int16{}
	s.sSide = [2]int16{}
	s.midSideAmpQ0 = [4]int32{}

	// Reset scalar values
	s.smthWidthQ14 = 0
	s.widthPrevQ14 = 0
	s.silentSideLen = 0

	// Clear variable-length arrays
	for i := range s.predIx {
		s.predIx[i] = [2][3]int8{}
	}

	for i := range s.midOnlyFlags {
		s.midOnlyFlags[i] = 0
	}
}
