package concentus

// TonalityAnalysisState represents the state for tonality analysis in Opus audio codec.
// This is a direct translation from Java to Go with appropriate idiomatic changes.
type TonalityAnalysisState struct {
	enabled               bool
	angle                 [240]float32
	dAngle                [240]float32
	d2Angle               [240]float32
	inmem                 [ANALYSIS_BUF_SIZE]int32
	memFill               int // number of usable samples in the buffer
	prevBandTonality      [NB_TBANDS]float32
	prevTonality          float32
	E                     [NB_FRAMES][NB_TBANDS]float32
	lowE                  [NB_TBANDS]float32
	highE                 [NB_TBANDS]float32
	meanE                 [NB_TOT_BANDS]float32
	mem                   [32]float32
	cmean                 [8]float32
	std                   [9]float32
	musicProb             float32
	etracker              float32
	lowECount             float32
	eCount                int
	lastMusic             int
	lastTransition        int
	count                 int
	subframeMem           [3]float32
	analysisOffset        int
	pspeech               [DETECT_SIZE]float32 // Probability of speech in window
	pmusic                [DETECT_SIZE]float32 // Probability of music in window
	speechConfidence      float32
	musicConfidence       float32
	speechConfidenceCount int
	musicConfidenceCount  int
	writePos              int
	readPos               int
	readSubframe          int
	info                  [DETECT_SIZE]AnalysisInfo
}

// NewTonalityAnalysisState creates and initializes a new TonalityAnalysisState.
func NewTonalityAnalysisState() *TonalityAnalysisState {
	state := &TonalityAnalysisState{}
	for i := range state.info {
		state.info[i] = *NewAnalysisInfo()
	}
	return state
}

// Reset clears all state variables to their zero values.
func (s *TonalityAnalysisState) Reset() {
	// Clear all arrays by assigning zero values
	s.angle = [240]float32{}
	s.dAngle = [240]float32{}
	s.d2Angle = [240]float32{}
	s.inmem = [ANALYSIS_BUF_SIZE]int32{}
	s.memFill = 0
	s.prevBandTonality = [NB_TBANDS]float32{}
	s.prevTonality = 0
	s.E = [NB_FRAMES][NB_TBANDS]float32{}
	s.lowE = [NB_TBANDS]float32{}
	s.highE = [NB_TBANDS]float32{}
	s.meanE = [NB_TOT_BANDS]float32{}
	s.mem = [32]float32{}
	s.cmean = [8]float32{}
	s.std = [9]float32{}
	s.musicProb = 0
	s.etracker = 0
	s.lowECount = 0
	s.eCount = 0
	s.lastMusic = 0
	s.lastTransition = 0
	s.count = 0
	s.subframeMem = [3]float32{}
	s.analysisOffset = 0
	s.pspeech = [DETECT_SIZE]float32{}
	s.pmusic = [DETECT_SIZE]float32{}
	s.speechConfidence = 0
	s.musicConfidence = 0
	s.speechConfidenceCount = 0
	s.musicConfidenceCount = 0
	s.writePos = 0
	s.readPos = 0
	s.readSubframe = 0

	// Reset each AnalysisInfo in the array
	for i := range s.info {
		s.info[i].Reset()
	}
}
