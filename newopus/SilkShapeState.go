package opus
type SilkShapeState struct {
    LastGainIndex byte
    HarmBoost_smth_Q16 int32
    HarmShapeGain_smth_Q16 int32
    Tilt_smth_Q16 int32
}

func (s *SilkShapeState) Reset() {
    s.LastGainIndex = 0
    s.HarmBoost_smth_Q16 = 0
    s.HarmShapeGain_smth_Q16 = 0
    s.Tilt_smth_Q16 = 0
}