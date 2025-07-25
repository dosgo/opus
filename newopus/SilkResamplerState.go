package opus

type SilkResamplerState struct {
	sIIR               [SILK_RESAMPLER_MAX_IIR_ORDER]int
	sFIR_i32           [SILK_RESAMPLER_MAX_FIR_ORDER]int
	sFIR_i16           [SILK_RESAMPLER_MAX_FIR_ORDER]int16
	delayBuf           [48]int16
	resampler_function int
	batchSize          int
	invRatio_Q16       int
	FIR_Order          int
	FIR_Fracs          int
	Fs_in_kHz          int
	Fs_out_kHz         int
	inputDelay         int
	Coefs              []int16
}

func (s *SilkResamplerState) Reset() {
	*s = SilkResamplerState{}
}

func (s *SilkResamplerState) Assign(other *SilkResamplerState) {
	*s = *other
}
