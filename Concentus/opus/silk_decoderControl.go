package opus

type SilkDecoderControl struct {
	pitchL        []int
	Gains_Q16     []int
	PredCoef_Q12  [2][]int16
	LTPCoef_Q14   []int16
	LTP_scale_Q14 int
}

func (p *SilkDecoderControl) Reset() {
	MemSetLen(p.pitchL, 0, SilkConstants.MAX_NB_SUBFR)
	MemSetLen(p.Gains_Q16, 0, SilkConstants.MAX_NB_SUBFR)
	MemSetLen(p.PredCoef_Q12[0], 0, SilkConstants.MAX_LPC_ORDER)
	MemSetLen(p.PredCoef_Q12[1], 0, SilkConstants.MAX_LPC_ORDER)
	MemSetLen(p.LTPCoef_Q14, 0, SilkConstants.LTP_ORDER*SilkConstants.MAX_NB_SUBFR)
	p.LTP_scale_Q14 = 0
}
