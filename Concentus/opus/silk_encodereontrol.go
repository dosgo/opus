package opus

type SilkEncoderControl struct {
	Gains_Q16          []int
	PredCoef_Q12       [][]int16
	LTPCoef_Q14        []int16
	LTP_scale_Q14      int
	pitchL             []int
	AR1_Q13            [MAX_NB_SUBFR * MAX_SHAPE_LPC_ORDER]int16
	AR2_Q13            [MAX_NB_SUBFR * MAX_SHAPE_LPC_ORDER]int16
	LF_shp_Q14         []int
	GainsPre_Q14       []int
	HarmBoost_Q14      []int
	Tilt_Q14           []int
	HarmShapeGain_Q14  []int
	Lambda_Q10         int
	input_quality_Q14  int
	coding_quality_Q14 int
	sparseness_Q8      int
	predGain_Q16       int
	LTPredCodGain_Q7   int
	ResNrg             [MAX_NB_SUBFR]int
	ResNrgQ            [MAX_NB_SUBFR]int
	GainsUnq_Q16       [MAX_NB_SUBFR]int
	lastGainIndexPrev  byte
}

func (s *SilkEncoderControl) Reset() {
	  Arrays.MemSet(Gains_Q16, 0, SilkConstants.MAX_NB_SUBFR);
        Arrays.MemSet(PredCoef_Q12[0], (short) 0, SilkConstants.MAX_LPC_ORDER);
        Arrays.MemSet(PredCoef_Q12[1], (short) 0, SilkConstants.MAX_LPC_ORDER);
        Arrays.MemSet(LTPCoef_Q14, (short) 0, SilkConstants.LTP_ORDER * SilkConstants.MAX_NB_SUBFR);
        LTP_scale_Q14 = 0;
        Arrays.MemSet(pitchL, 0, SilkConstants.MAX_NB_SUBFR);
        Arrays.MemSet(AR1_Q13, (short) 0, SilkConstants.MAX_NB_SUBFR * SilkConstants.MAX_SHAPE_LPC_ORDER);
        Arrays.MemSet(AR2_Q13, (short) 0, SilkConstants.MAX_NB_SUBFR * SilkConstants.MAX_SHAPE_LPC_ORDER);
        Arrays.MemSet(LF_shp_Q14, 0, SilkConstants.MAX_NB_SUBFR);
        Arrays.MemSet(GainsPre_Q14, 0, SilkConstants.MAX_NB_SUBFR);
        Arrays.MemSet(HarmBoost_Q14, 0, SilkConstants.MAX_NB_SUBFR);
        Arrays.MemSet(Tilt_Q14, 0, SilkConstants.MAX_NB_SUBFR);
        Arrays.MemSet(HarmShapeGain_Q14, 0, SilkConstants.MAX_NB_SUBFR);
        Lambda_Q10 = 0;
        input_quality_Q14 = 0;
        coding_quality_Q14 = 0;
        sparseness_Q8 = 0;
        predGain_Q16 = 0;
        LTPredCodGain_Q7 = 0;
        Arrays.MemSet(ResNrg, 0, SilkConstants.MAX_NB_SUBFR);
        Arrays.MemSet(ResNrgQ, 0, SilkConstants.MAX_NB_SUBFR);
        Arrays.MemSet(GainsUnq_Q16, 0, SilkConstants.MAX_NB_SUBFR);
        lastGainIndexPrev = 0;
}
