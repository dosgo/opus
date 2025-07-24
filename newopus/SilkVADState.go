package opus
type SilkVADState struct {
    AnaState         [2]int
    AnaState1        [2]int
    AnaState2        [2]int
    XnrgSubfr        [VAD_N_BANDS]int
    NrgRatioSmth_Q8  [VAD_N_BANDS]int
    HPstate          int16
    NL               [VAD_N_BANDS]int
    inv_NL           [VAD_N_BANDS]int
    NoiseLevelBias   [VAD_N_BANDS]int
    counter          int
}

func (s *SilkVADState) Reset() {
    *s = SilkVADState{}
}