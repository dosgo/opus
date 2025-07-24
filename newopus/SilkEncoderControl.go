package opus

type SilkEncoderControl struct {
    Gains_Q16 [MAX_NB_SUBFR]int32
    PredCoef_Q12 [2][MAX_LPC_ORDER]int16
    LTPCoef_Q14 [LTP_ORDER * MAX_NB_SUBFR]int16
    LTP_scale_Q14 int32
    pitchL [MAX_NB_SUBFR]int32
    AR1_Q13 [MAX_NB_SUBFR * MAX_SHAPE_LPC_ORDER]int16
    AR2_Q13 [MAX_NB_SUBFR * MAX_SHAPE_LPC_ORDER]int16
    LF_shp_Q14 [MAX_NB_SUBFR]int32
    GainsPre_Q14 [MAX_NB_SUBFR]int32
    HarmBoost_Q14 [MAX_NB_SUBFR]int32
    Tilt_Q14 [MAX_NB_SUBFR]int32
    HarmShapeGain_Q14 [MAX_NB_SUBFR]int32
    Lambda_Q10 int32
    input_quality_Q14 int32
    coding_quality_Q14 int32
    sparseness_Q8 int32
    predGain_Q16 int32
    LTPredCodGain_Q7 int32
    ResNrg [MAX_NB_SUBFR]int32
    ResNrgQ [MAX_NB_SUBFR]int32
    GainsUnq_Q16 [MAX_NB_SUBFR]int32
    lastGainIndexPrev byte
}

func (s *SilkEncoderControl) Reset() {
    *s = SilkEncoderControl{}
}