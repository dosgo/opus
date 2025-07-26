package opus

type SilkDecoderControl struct {
	pitchL        [MAX_NB_SUBFR]int
	Gains_Q16     [MAX_NB_SUBFR]int
	PredCoef_Q12  [2][]int16
	LTPCoef_Q14   [LTP_ORDER * MAX_NB_SUBFR]int16
	LTP_scale_Q14 int
}

func (p *SilkDecoderControl) Reset() {
	*p = SilkDecoderControl{}
}
