package silk

import "math"

// CNG implements Comfort Noise Generation and estimation functionality.
// This is a direct translation from the Java version, adapted to Go idioms.
type CNG struct {
	CNG_smth_NLSF_Q15 []int16 // Smoothed NLSF coefficients
	CNG_smth_Gain_Q16 int32   // Smoothed gain
	CNG_exc_buf_Q14   []int16 // Excitation buffer
	CNG_synth_state   []int16 // Synthesis filter state
	rand_seed         int32   // Random seed for excitation generation
	fs_kHz            int     // Sampling rate in kHz
}

// NewCNG creates a new CNG state with the given LPC order.
func NewCNG(LPC_order int) *CNG {
	return &CNG{
		CNG_smth_NLSF_Q15: make([]int16, LPC_order),
		CNG_exc_buf_Q14:   make([]int16, CNG_BUF_MASK_MAX+1),
		CNG_synth_state:   make([]int16, MAX_LPC_ORDER),
		rand_seed:         3176576, // Default seed from Java version
	}
}

// CNG_exc generates excitation for CNG LPC synthesis.
//
// Parameters:
//
//	exc_Q10: Output CNG excitation signal (Q10 format)
//	exc_buf_Q14: Random samples buffer (Q14 format)
//	Gain_Q16: Gain to apply (Q16 format)
//	length: Number of samples to generate
//	rand_seed: Pointer to random seed (updated during generation)
//
// Note: Go uses pass-by-value for primitives, so we use a pointer for rand_seed to match Java's BoxedValueInt behavior.
func CNG_exc(exc_Q10 []int16, exc_buf_Q14 []int16, Gain_Q16 int32, length int, rand_seed *int32) {
	// Calculate mask for buffer indexing
	exc_mask := CNG_BUF_MASK_MAX
	for exc_mask > length {
		exc_mask >>= 1
	}

	seed := *rand_seed
	for i := 0; i < length; i++ {
		seed = RAND(seed)
		idx := int((seed >> 24) & int32(exc_mask))
		// Assertions converted to debug checks
		if debug {
			if idx < 0 || idx > CNG_BUF_MASK_MAX {
				panic("CNG_exc: index out of bounds")
			}
		}
		// Perform the same arithmetic as Java version
		exc_Q10[i] = int16(SAT16(SMULWW(int32(exc_buf_Q14[idx]), Gain_Q16>>4)))
	}
	*rand_seed = seed
}

// CNG_Reset resets the CNG state to default values.
func (c *CNG) Reset(LPC_order int) {
	NLSF_step_Q15 := DIV32_16(math.MaxInt16, int16(LPC_order+1))
	NLSF_acc_Q15 := int16(0)
	for i := 0; i < LPC_order; i++ {
		NLSF_acc_Q15 += NLSF_step_Q15
		c.CNG_smth_NLSF_Q15[i] = NLSF_acc_Q15
	}
	c.CNG_smth_Gain_Q16 = 0
	c.rand_seed = 3176576 // Reset to default seed
}

// CNG updates the CNG estimate and applies CNG when packet is lost.
//
// Parameters:
//
//	psDec: Decoder state
//	psDecCtrl: Decoder control
//	frame: Signal to modify
//	length: Length of residual
func (c *CNG) CNG(psDec *ChannelDecoder, psDecCtrl *DecoderControl, frame []int16, length int) {
	A_Q12 := make([]int16, psDec.LPC_order)

	// Reset if sample rate changed
	if psDec.fs_kHz != c.fs_kHz {
		c.Reset(psDec.LPC_order)
		c.fs_kHz = psDec.fs_kHz
	}

	if psDec.lossCnt == 0 && psDec.prevSignalType == TYPE_NO_VOICE_ACTIVITY {
		// Update CNG parameters when no packet loss and previous frame was silence

		// Smoothing of LSF's
		for i := 0; i < psDec.LPC_order; i++ {
			delta := int32(psDec.prevNLSF_Q15[i]) - int32(c.CNG_smth_NLSF_Q15[i])
			c.CNG_smth_NLSF_Q15[i] += int16(SMULWB(delta, CNG_NLSF_SMTH_Q16))
		}

		// Find subframe with highest gain
		max_Gain_Q16 := int32(0)
		subfr := 0
		for i := 0; i < psDec.nb_subfr; i++ {
			if psDecCtrl.Gains_Q16[i] > max_Gain_Q16 {
				max_Gain_Q16 = psDecCtrl.Gains_Q16[i]
				subfr = i
			}
		}

		// Update CNG excitation buffer (shift left by subfr_length)
		copy(c.CNG_exc_buf_Q14, c.CNG_exc_buf_Q14[psDec.subfr_length:])

		// Smooth gains
		for i := 0; i < psDec.nb_subfr; i++ {
			delta := psDecCtrl.Gains_Q16[i] - c.CNG_smth_Gain_Q16
			c.CNG_smth_Gain_Q16 += SMULWB(delta, CNG_GAIN_SMTH_Q16)
		}
	}

	// Add CNG when packet is lost or during DTX
	if psDec.lossCnt != 0 {
		CNG_sig_Q10 := make([]int32, length+MAX_LPC_ORDER)

		// Generate CNG excitation
		gain_Q16 := SMULWW(psDec.sPLC.randScale_Q14, psDec.sPLC.prevGain_Q16[1])
		if gain_Q16 >= (1<<21) || c.CNG_smth_Gain_Q16 > (1<<23) {
			gain_Q16 = SMULTT(gain_Q16, gain_Q16)
			gain_Q16 = SUB_LSHIFT32(SMULTT(c.CNG_smth_Gain_Q16, c.CNG_smth_Gain_Q16), gain_Q16, 5)
			gain_Q16 = LSHIFT32(SQRT_APPROX(gain_Q16), 16)
		} else {
			gain_Q16 = SMULWW(gain_Q16, gain_Q16)
			gain_Q16 = SUB_LSHIFT32(SMULWW(c.CNG_smth_Gain_Q16, c.CNG_smth_Gain_Q16), gain_Q16, 5)
			gain_Q16 = LSHIFT32(SQRT_APPROX(gain_Q16), 8)
		}

		seed := c.rand_seed
		CNG_exc(CNG_sig_Q10[MAX_LPC_ORDER:], c.CNG_exc_buf_Q14, gain_Q16, length, &seed)
		c.rand_seed = seed

		// Convert CNG NLSF to filter representation
		NLSF2A(A_Q12, c.CNG_smth_NLSF_Q15, psDec.LPC_order)

		// Initialize with synthesis state
		copy(CNG_sig_Q10[:MAX_LPC_ORDER], int16ToInt32Slice(c.CNG_synth_state))

		// Generate CNG signal by synthesis filtering
		for i := 0; i < length; i++ {
			lpci := MAX_LPC_ORDER + i
			if debug {
				if psDec.LPC_order != 10 && psDec.LPC_order != 16 {
					panic("CNG: LPC_order must be 10 or 16")
				}
			}

			// Avoid bias by starting with half the LPC order
			sum_Q6 := int32(psDec.LPC_order >> 1)
			sum_Q6 = SMLAWB(sum_Q6, CNG_sig_Q10[lpci-1], A_Q12[0])
			sum_Q6 = SMLAWB(sum_Q6, CNG_sig_Q10[lpci-2], A_Q12[1])
			sum_Q6 = SMLAWB(sum_Q6, CNG_sig_Q10[lpci-3], A_Q12[2])
			sum_Q6 = SMLAWB(sum_Q6, CNG_sig_Q10[lpci-4], A_Q12[3])
			sum_Q6 = SMLAWB(sum_Q6, CNG_sig_Q10[lpci-5], A_Q12[4])
			sum_Q6 = SMLAWB(sum_Q6, CNG_sig_Q10[lpci-6], A_Q12[5])
			sum_Q6 = SMLAWB(sum_Q6, CNG_sig_Q10[lpci-7], A_Q12[6])
			sum_Q6 = SMLAWB(sum_Q6, CNG_sig_Q10[lpci-8], A_Q12[7])
			sum_Q6 = SMLAWB(sum_Q6, CNG_sig_Q10[lpci-9], A_Q12[8])
			sum_Q6 = SMLAWB(sum_Q6, CNG_sig_Q10[lpci-10], A_Q12[9])

			if psDec.LPC_order == 16 {
				sum_Q6 = SMLAWB(sum_Q6, CNG_sig_Q10[lpci-11], A_Q12[10])
				sum_Q6 = SMLAWB(sum_Q6, CNG_sig_Q10[lpci-12], A_Q12[11])
				sum_Q6 = SMLAWB(sum_Q6, CNG_sig_Q10[lpci-13], A_Q12[12])
				sum_Q6 = SMLAWB(sum_Q6, CNG_sig_Q10[lpci-14], A_Q12[13])
				sum_Q6 = SMLAWB(sum_Q6, CNG_sig_Q10[lpci-15], A_Q12[14])
				sum_Q6 = SMLAWB(sum_Q6, CNG_sig_Q10[lpci-16], A_Q12[15])
			}

			// Update states
			CNG_sig_Q10[lpci] = ADD_LSHIFT(CNG_sig_Q10[lpci], sum_Q6, 4)

			// Add to output frame with saturation
			frame[i] = ADD_SAT16(frame[i], int16(RSHIFT_ROUND(CNG_sig_Q10[lpci], 10)))
		}

		// Save final state
		copy(c.CNG_synth_state, int32ToInt16Slice(CNG_sig_Q10[length:length+MAX_LPC_ORDER]))
	} else {
		// Clear state when no packet loss
		for i := 0; i < psDec.LPC_order; i++ {
			c.CNG_synth_state[i] = 0
		}
	}
}

// Helper functions for type conversion
func int16ToInt32Slice(s []int16) []int32 {
	r := make([]int32, len(s))
	for i, v := range s {
		r[i] = int32(v)
	}
	return r
}

func int32ToInt16Slice(s []int32) []int16 {
	r := make([]int16, len(s))
	for i, v := range s {
		r[i] = int16(v)
	}
	return r
}
