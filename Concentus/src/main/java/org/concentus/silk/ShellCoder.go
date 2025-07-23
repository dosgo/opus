// Package silk implements SILK audio codec functionality.
// This file contains the shell coder implementation with pulse-subframe length hardcoded.
package silk

// ShellCoder provides methods for encoding and decoding pulse amplitudes using shell coding.
type ShellCoder struct{}

// CombinePulses combines pairs of pulses from input into output.
// This is the version that takes an input pointer offset.
//
// Parameters:
//
//	output - combined pulses vector [len]
//	input - input vector [2 * len]
//	inputPtr - offset into input vector
//	len - number of output samples
func (sc *ShellCoder) CombinePulses(output, input []int, inputPtr, len int) {
	for k := 0; k < len; k++ {
		output[k] = input[inputPtr+2*k] + input[inputPtr+2*k+1]
	}
}

// CombinePulsesNoOffset combines pairs of pulses from input into output.
// This is the version without input pointer offset.
//
// Parameters:
//
//	output - combined pulses vector [len]
//	input - input vector [2 * len]
//	len - number of output samples
func (sc *ShellCoder) CombinePulsesNoOffset(output, input []int, len int) {
	for k := 0; k < len; k++ {
		output[k] = input[2*k] + input[2*k+1]
	}
}

// EncodeSplit encodes a split between parent and child pulse amplitudes.
//
// Parameters:
//
//	psRangeEnc - compressor data structure
//	pChild1 - pulse amplitude of first child subframe
//	p - pulse amplitude of current subframe
//	shellTable - table of shell CDFs
func (sc *ShellCoder) EncodeSplit(psRangeEnc *EntropyCoder, pChild1, p int, shellTable []int16) {
	if p > 0 {
		offset := SilkTables.ShellCodeTableOffsets[p]
		psRangeEnc.EncIcdf(pChild1, shellTable, offset, 8)
	}
}

// DecodeSplit decodes a split between parent and child pulse amplitudes.
//
// Parameters:
//
//	pChild1 - output pulse amplitude of first child subframe
//	pChild2 - output pulse amplitude of second child subframe
//	psRangeDec - compressor data structure
//	p - pulse amplitude of current subframe
//	shellTable - table of shell CDFs
func (sc *ShellCoder) DecodeSplit(pChild1, pChild2 *int16, psRangeDec *EntropyCoder, p int, shellTable []int16) {
	if p > 0 {
		*pChild1 = int16(psRangeDec.DecIcdf(shellTable, SilkTables.ShellCodeTableOffsets[p], 8))
		*pChild2 = int16(p) - *pChild1
	} else {
		*pChild1 = 0
		*pChild2 = 0
	}
}

// ShellEncoder operates on one shell code frame of 16 pulses.
//
// Parameters:
//
//	psRangeEnc - compressor data structure
//	pulses0 - data: nonnegative pulse amplitudes
//	pulses0Ptr - offset into pulses0 array
func (sc *ShellCoder) ShellEncoder(psRangeEnc *EntropyCoder, pulses0 []int, pulses0Ptr int) {
	// Allocate arrays for intermediate pulse combinations
	pulses1 := make([]int, 8)
	pulses2 := make([]int, 4)
	pulses3 := make([]int, 2)
	pulses4 := make([]int, 1)

	// This function operates on one shell code frame of 16 pulses
	// Note: In Go, we'd typically use a constant or configuration value
	// rather than embedding the assertion. The original Java had:
	// Inlines.OpusAssert(SilkConstants.SHELL_CODEC_FRAME_LENGTH == 16)

	// Tree representation per pulse-subframe
	sc.CombinePulses(pulses1, pulses0, pulses0Ptr, 8)
	sc.CombinePulsesNoOffset(pulses2, pulses1, 4)
	sc.CombinePulsesNoOffset(pulses3, pulses2, 2)
	sc.CombinePulsesNoOffset(pulses4, pulses3, 1)

	// Encode splits in the pulse tree
	sc.EncodeSplit(psRangeEnc, pulses3[0], pulses4[0], SilkTables.ShellCodeTable3)
	sc.EncodeSplit(psRangeEnc, pulses2[0], pulses3[0], SilkTables.ShellCodeTable2)
	sc.EncodeSplit(psRangeEnc, pulses1[0], pulses2[0], SilkTables.ShellCodeTable1)
	sc.EncodeSplit(psRangeEnc, pulses0[pulses0Ptr], pulses1[0], SilkTables.ShellCodeTable0)
	sc.EncodeSplit(psRangeEnc, pulses0[pulses0Ptr+2], pulses1[1], SilkTables.ShellCodeTable0)
	sc.EncodeSplit(psRangeEnc, pulses1[2], pulses2[1], SilkTables.ShellCodeTable1)
	sc.EncodeSplit(psRangeEnc, pulses0[pulses0Ptr+4], pulses1[2], SilkTables.ShellCodeTable0)
	sc.EncodeSplit(psRangeEnc, pulses0[pulses0Ptr+6], pulses1[3], SilkTables.ShellCodeTable0)
	sc.EncodeSplit(psRangeEnc, pulses2[2], pulses3[1], SilkTables.ShellCodeTable2)
	sc.EncodeSplit(psRangeEnc, pulses1[4], pulses2[2], SilkTables.ShellCodeTable1)
	sc.EncodeSplit(psRangeEnc, pulses0[pulses0Ptr+8], pulses1[4], SilkTables.ShellCodeTable0)
	sc.EncodeSplit(psRangeEnc, pulses0[pulses0Ptr+10], pulses1[5], SilkTables.ShellCodeTable0)
	sc.EncodeSplit(psRangeEnc, pulses1[6], pulses2[3], SilkTables.ShellCodeTable1)
	sc.EncodeSplit(psRangeEnc, pulses0[pulses0Ptr+12], pulses1[6], SilkTables.ShellCodeTable0)
	sc.EncodeSplit(psRangeEnc, pulses0[pulses0Ptr+14], pulses1[7], SilkTables.ShellCodeTable0)
}

// ShellDecoder operates on one shell code frame of 16 pulses.
//
// Parameters:
//
//	pulses0 - output data: nonnegative pulse amplitudes
//	pulses0Ptr - offset into pulses0 array
//	psRangeDec - compressor data structure
//	pulses4 - number of pulses per pulse-subframe
func (sc *ShellCoder) ShellDecoder(pulses0 []int16, pulses0Ptr int, psRangeDec *EntropyCoder, pulses4 int) {
	// Allocate arrays for intermediate pulse combinations
	pulses1 := make([]int16, 8)
	pulses2 := make([]int16, 4)
	pulses3 := make([]int16, 2)

	// This function operates on one shell code frame of 16 pulses
	// Note: Similar to encoder, we'd use a constant in practice

	// Decode splits in the pulse tree
	sc.DecodeSplit(&pulses3[0], &pulses3[1], psRangeDec, pulses4, SilkTables.ShellCodeTable3)
	sc.DecodeSplit(&pulses2[0], &pulses2[1], psRangeDec, int(pulses3[0]), SilkTables.ShellCodeTable2)
	sc.DecodeSplit(&pulses1[0], &pulses1[1], psRangeDec, int(pulses2[0]), SilkTables.ShellCodeTable1)
	sc.DecodeSplit(&pulses0[pulses0Ptr], &pulses0[pulses0Ptr+1], psRangeDec, int(pulses1[0]), SilkTables.ShellCodeTable0)
	sc.DecodeSplit(&pulses0[pulses0Ptr+2], &pulses0[pulses0Ptr+3], psRangeDec, int(pulses1[1]), SilkTables.ShellCodeTable0)
	sc.DecodeSplit(&pulses1[2], &pulses1[3], psRangeDec, int(pulses2[1]), SilkTables.ShellCodeTable1)
	sc.DecodeSplit(&pulses0[pulses0Ptr+4], &pulses0[pulses0Ptr+5], psRangeDec, int(pulses1[2]), SilkTables.ShellCodeTable0)
	sc.DecodeSplit(&pulses0[pulses0Ptr+6], &pulses0[pulses0Ptr+7], psRangeDec, int(pulses1[3]), SilkTables.ShellCodeTable0)
	sc.DecodeSplit(&pulses2[2], &pulses2[3], psRangeDec, int(pulses3[1]), SilkTables.ShellCodeTable2)
	sc.DecodeSplit(&pulses1[4], &pulses1[5], psRangeDec, int(pulses2[2]), SilkTables.ShellCodeTable1)
	sc.DecodeSplit(&pulses0[pulses0Ptr+8], &pulses0[pulses0Ptr+9], psRangeDec, int(pulses1[4]), SilkTables.ShellCodeTable0)
	sc.DecodeSplit(&pulses0[pulses0Ptr+10], &pulses0[pulses0Ptr+11], psRangeDec, int(pulses1[5]), SilkTables.ShellCodeTable0)
	sc.DecodeSplit(&pulses1[6], &pulses1[7], psRangeDec, int(pulses2[3]), SilkTables.ShellCodeTable1)
	sc.DecodeSplit(&pulses0[pulses0Ptr+12], &pulses0[pulses0Ptr+13], psRangeDec, int(pulses1[6]), SilkTables.ShellCodeTable0)
	sc.DecodeSplit(&pulses0[pulses0Ptr+14], &pulses0[pulses0Ptr+15], psRangeDec, int(pulses1[7]), SilkTables.ShellCodeTable0)
}
