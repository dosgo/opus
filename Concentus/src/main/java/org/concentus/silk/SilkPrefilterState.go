package silk

// PrefilterState represents the state of a SILK prefilter.
// It maintains various buffers and state variables used during audio processing.
type PrefilterState struct {
	sLTP_shp         [LTP_BUF_LENGTH]int16          // LTP shaping history buffer
	sAR_shp          [MAX_SHAPE_LPC_ORDER + 1]int32 // AR shaping filter state
	sLTP_shp_buf_idx int32                          // Index for sLTP_shp circular buffer
	sLF_AR_shp_Q12   int32                          // AR shaping filter state for low frequencies (Q12)
	sLF_MA_shp_Q12   int32                          // MA shaping filter state for low frequencies (Q12)
	sHarmHP_Q2       int32                          // Harmonic noise shaping state (Q2)
	rand_seed        int32                          // Random seed for noise generation
	lagPrev          int32                          // Previous lag value for pitch analysis
}

// NewPrefilterState creates and initializes a new PrefilterState.
func NewPrefilterState() *PrefilterState {
	return &PrefilterState{}
}

// Reset clears the prefilter state, setting all buffers and variables to zero.
// This should be called when starting a new audio stream or after a discontinuity.
func (s *PrefilterState) Reset() {
	// Clear buffers
	for i := range s.sLTP_shp {
		s.sLTP_shp[i] = 0
	}
	for i := range s.sAR_shp {
		s.sAR_shp[i] = 0
	}

	// Reset state variables
	s.sLTP_shp_buf_idx = 0
	s.sLF_AR_shp_Q12 = 0
	s.sLF_MA_shp_Q12 = 0
	s.sHarmHP_Q2 = 0
	s.rand_seed = 0
	s.lagPrev = 0
}
