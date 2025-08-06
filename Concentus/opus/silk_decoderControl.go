package opus

type SilkDecoderControl struct {
	pitchL        []int
	Gains_Q16     []int
	PredCoef_Q12  [2][]int16
	LTPCoef_Q14   []int16
	LTP_scale_Q14 int
}

func (p *SilkDecoderControl) Reset() {
	 Arrays.MemSet(pitchL, 0, SilkConstants.MAX_NB_SUBFR);
        Arrays.MemSet(Gains_Q16, 0, SilkConstants.MAX_NB_SUBFR);
        Arrays.MemSet(PredCoef_Q12[0], (short) 0, SilkConstants.MAX_LPC_ORDER);
        Arrays.MemSet(PredCoef_Q12[1], (short) 0, SilkConstants.MAX_LPC_ORDER);
        Arrays.MemSet(LTPCoef_Q14, (short) 0, SilkConstants.LTP_ORDER * SilkConstants.MAX_NB_SUBFR);
        LTP_scale_Q14 = 0;
}
