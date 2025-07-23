// Package silk contains structures and constants for SILK audio codec processing
package silk

// PLCStruct represents Packet Loss Concealment state
//
// This struct is used to maintain state between frames when handling packet loss
// in SILK audio codec processing. It stores various parameters needed for generating
// replacement audio data when packets are lost.
type PLCStruct struct {
	pitchL_Q8         int       // Pitch lag to use for voiced concealment (Q8 fixed point)
	LTPCoef_Q14       [5]int16  // LTP coefficients to use for voiced concealment (Q14 fixed point)
	prevLPC_Q12       [16]int16 // Previous LPC coefficients (Q12 fixed point)
	last_frame_lost   int       // Was previous frame lost (boolean as int)
	rand_seed         int       // Seed for unvoiced signal generation
	randScale_Q14     int16     // Scaling of unvoiced random signal (Q14 fixed point)
	conc_energy       int       // Concealment energy
	conc_energy_shift int       // Energy shift value
	prevLTP_scale_Q14 int16     // Previous LTP scaling (Q14 fixed point)
	prevGain_Q16      [2]int32  // Previous gain values (Q16 fixed point)
	fs_kHz            int       // Sampling frequency in kHz
	nb_subfr          int       // Number of subframes
	subfr_length      int       // Subframe length
}

// Reset initializes all PLCStruct fields to their zero values
//
// This method is equivalent to the Java version's Reset() method, but takes advantage
// of Go's zero value initialization. In Go, we can simply create a new struct value
// to reset, but this method provides the same behavior as the Java version for API
// compatibility.
func (plc *PLCStruct) Reset() {
	*plc = PLCStruct{}
}
