package opus
type SilkDecoderControl struct {
    pitchL        [MAX_NB_SUBFR]int32
    Gains_Q16     [MAX_NB_SUBFR]int32
    PredCoef_Q12  [2][MAX_LPC_ORDER]int16
    LTPCoef_Q14   [LTP_ORDER * MAX_NB_SUBFR]int16
    LTP_scale_Q14 int32
}

func (p *SilkDecoderControl) Reset() {
    *p = SilkDecoderControl{}
}