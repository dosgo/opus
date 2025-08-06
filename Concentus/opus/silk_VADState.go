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
	Arrays.MemSet(AnaState, 0, 2)
	Arrays.MemSet(AnaState1, 0, 2)
	Arrays.MemSet(AnaState2, 0, 2)
	Arrays.MemSet(XnrgSubfr, 0, SilkConstants.VAD_N_BANDS)
	Arrays.MemSet(NrgRatioSmth_Q8, 0, SilkConstants.VAD_N_BANDS)
	HPstate = 0
	Arrays.MemSet(NL, 0, SilkConstants.VAD_N_BANDS)
	Arrays.MemSet(inv_NL, 0, SilkConstants.VAD_N_BANDS)
	Arrays.MemSet(NoiseLevelBias, 0, SilkConstants.VAD_N_BANDS)
	counter = 0
}
