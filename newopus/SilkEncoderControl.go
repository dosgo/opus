package opus

type SilkEncoderControl struct {
	Gains_Q16          [MAX_NB_SUBFR]int
	PredCoef_Q12       [][]int16
	LTPCoef_Q14        [LTP_ORDER * MAX_NB_SUBFR]int16
	LTP_scale_Q14      int
	pitchL             [MAX_NB_SUBFR]int
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
	*s = SilkEncoderControl{}
}
