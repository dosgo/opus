package opus

type SilkVADState struct {
	AnaState        []int
	AnaState1       []int
	AnaState2       []int
	XnrgSubfr       [VAD_N_BANDS]int
	NrgRatioSmth_Q8 [VAD_N_BANDS]int
	HPstate         int16
	NL              [VAD_N_BANDS]int
	inv_NL          [VAD_N_BANDS]int
	NoiseLevelBias  [VAD_N_BANDS]int
	counter         int
}

func (s *SilkVADState) Reset() {
	*s = SilkVADState{}
}
