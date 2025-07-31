package opus

type SilkDecoderControl struct {
	pitchL        []int
	Gains_Q16     []int
	PredCoef_Q12  [2][]int16
	LTPCoef_Q14   []int16
	LTP_scale_Q14 int
}

func (p *SilkDecoderControl) Reset() {
	*p = SilkDecoderControl{}
}
