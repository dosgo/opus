package silk

import (
	"math"
)

// ChannelDecoder represents the decoder state for SILK audio decoding.
type ChannelDecoder struct {
	prevGainQ16          int32
	excQ14               [MAX_FRAME_LENGTH]int32
	sLPCQ14Buf           [MAX_LPC_ORDER]int32
	outBuf               [MAX_FRAME_LENGTH + 2*MAX_SUB_FRAME_LENGTH]int16
	lagPrev              int32
	lastGainIndex        byte
	fsKHz                int32
	fsAPIHz              int32
	nbSubfr              int32
	frameLength          int32
	subfrLength          int32
	ltpMemLength         int32
	LPCOrder             int32
	prevNLSFQ15          [MAX_LPC_ORDER]int16
	firstFrameAfterReset int32
	pitchLagLowBitsICDF  []int16
	pitchContourICDF     []int16

	// For buffering payload in case of more frames per packet
	nFramesDecoded   int32
	nFramesPerPacket int32

	// Specifically for entropy coding
	ecPrevSignalType int32
	ecPrevLagIndex   int16

	VADFlags  [MAX_FRAMES_PER_PACKET]int32
	LBRRFlag  int32
	LBRRFlags [MAX_FRAMES_PER_PACKET]int32

	resamplerState ResamplerState
	psNLSFCB       *NLSFCodebook
	indices        SideInfoIndices
	sCNG           CNGState
	lossCnt        int32
	prevSignalType int32
	sPLC           PLCStruct
}

// Reset initializes the decoder state to default values.
func (d *ChannelDecoder) Reset() {
	d.prevGainQ16 = 0
	d.excQ14 = [MAX_FRAME_LENGTH]int32{}
	d.sLPCQ14Buf = [MAX_LPC_ORDER]int32{}
	d.outBuf = [MAX_FRAME_LENGTH + 2*MAX_SUB_FRAME_LENGTH]int16{}
	d.lagPrev = 0
	d.lastGainIndex = 0
	d.fsKHz = 0
	d.fsAPIHz = 0
	d.nbSubfr = 0
	d.frameLength = 0
	d.subfrLength = 0
	d.ltpMemLength = 0
	d.LPCOrder = 0
	d.prevNLSFQ15 = [MAX_LPC_ORDER]int16{}
	d.firstFrameAfterReset = 0
	d.pitchLagLowBitsICDF = nil
	d.pitchContourICDF = nil
	d.nFramesDecoded = 0
	d.nFramesPerPacket = 0
	d.ecPrevSignalType = 0
	d.ecPrevLagIndex = 0
	d.VADFlags = [MAX_FRAMES_PER_PACKET]int32{}
	d.LBRRFlag = 0
	d.LBRRFlags = [MAX_FRAMES_PER_PACKET]int32{}
	d.resamplerState.Reset()
	d.psNLSFCB = nil
	d.indices.Reset()
	d.sCNG.Reset()
	d.lossCnt = 0
	d.prevSignalType = 0
	d.sPLC.Reset()
}

// InitDecoder initializes the decoder state.
func (d *ChannelDecoder) InitDecoder() int32 {
	// Clear the entire encoder state, except anything copied
	d.Reset()

	// Used to deactivate LSF interpolation
	d.firstFrameAfterReset = 1
	d.prevGainQ16 = 65536

	// Reset CNG state
	d.cngReset()

	// Reset PLC state
	d.plcReset()

	return 0
}

// cngReset resets the CNG (Comfort Noise Generation) state.
func (d *ChannelDecoder) cngReset() {
	var NLSFStepQ15, NLSFAccQ15 int32

	NLSFStepQ15 = Div32_16(math.MaxInt16, d.LPCOrder+1)
	NLSFAccQ15 = 0
	for i := int32(0); i < d.LPCOrder; i++ {
		NLSFAccQ15 += NLSFStepQ15
		d.sCNG.CNGSmthNLSFQ15[i] = int16(NLSFAccQ15)
	}
	d.sCNG.CNGSmthGainQ16 = 0
	d.sCNG.RandSeed = 3176576
}

// plcReset resets the PLC (Packet Loss Concealment) state.
func (d *ChannelDecoder) plcReset() {
	d.sPLC.PitchLQ8 = LSHIFT(d.frameLength, 8-1)
	d.sPLC.PrevGainQ16[0] = int32((1)*(1<<16) + 0.5) // SILK_CONST(1, 16)
	d.sPLC.PrevGainQ16[1] = int32((1)*(1<<16) + 0.5) // SILK_CONST(1, 16)
	d.sPLC.SubfrLength = 20
	d.sPLC.NbSubfr = 2
}

// SetSampleRate sets the decoder sampling rate.
func (d *ChannelDecoder) SetSampleRate(fsKHz, fsAPIHz int32) int32 {
	var frameLength, ret int32 = 0, 0

	// Input validation
	if fsKHz != 8 && fsKHz != 12 && fsKHz != 16 {
		panic("invalid fsKHz")
	}
	if d.nbSubfr != MAX_NB_SUBFR && d.nbSubfr != MAX_NB_SUBFR/2 {
		panic("invalid nbSubfr")
	}

	// New (sub)frame length
	d.subfrLength = SMULBB(SUB_FRAME_LENGTH_MS, fsKHz)
	frameLength = SMULBB(d.nbSubfr, d.subfrLength)

	// Initialize resampler when switching or external sampling frequency
	if d.fsKHz != fsKHz || d.fsAPIHz != fsAPIHz {
		// Initialize the resampler for dec_API.c preparing resampling from fsKHz to API_fs_Hz
		ret += d.resamplerState.Init(SMULBB(fsKHz, 1000), fsAPIHz, 0)
		d.fsAPIHz = fsAPIHz
	}

	if d.fsKHz != fsKHz || frameLength != d.frameLength {
		if fsKHz == 8 {
			if d.nbSubfr == MAX_NB_SUBFR {
				d.pitchContourICDF = PitchContourNBICDF[:]
			} else {
				d.pitchContourICDF = PitchContour10MsNBICDF[:]
			}
		} else if d.nbSubfr == MAX_NB_SUBFR {
			d.pitchContourICDF = PitchContourICDF[:]
		} else {
			d.pitchContourICDF = PitchContour10MsICDF[:]
		}

		if d.fsKHz != fsKHz {
			d.ltpMemLength = SMULBB(LTP_MEM_LENGTH_MS, fsKHz)
			if fsKHz == 8 || fsKHz == 12 {
				d.LPCOrder = MIN_LPC_ORDER
				d.psNLSFCB = &NLSFCBNBMB
			} else {
				d.LPCOrder = MAX_LPC_ORDER
				d.psNLSFCB = &NLSFCBWB
			}

			switch fsKHz {
			case 16:
				d.pitchLagLowBitsICDF = Uniform8ICDF[:]
			case 12:
				d.pitchLagLowBitsICDF = Uniform6ICDF[:]
			case 8:
				d.pitchLagLowBitsICDF = Uniform4ICDF[:]
			default:
				panic("unsupported sampling rate")
			}

			d.firstFrameAfterReset = 1
			d.lagPrev = 100
			d.lastGainIndex = 10
			d.prevSignalType = TYPE_NO_VOICE_ACTIVITY
			d.outBuf = [MAX_FRAME_LENGTH + 2*MAX_SUB_FRAME_LENGTH]int16{}
			d.sLPCQ14Buf = [MAX_LPC_ORDER]int32{}
		}

		d.fsKHz = fsKHz
		d.frameLength = frameLength
	}

	// Check that settings are valid
	if d.frameLength <= 0 || d.frameLength > MAX_FRAME_LENGTH {
		panic("invalid frame length")
	}

	return ret
}

// DecodeFrame decodes a single audio frame.
func (d *ChannelDecoder) DecodeFrame(
	psRangeDec *EntropyCoder,
	pOut []int16,
	pOutPtr int,
	pN *int32,
	lostFlag int32,
	condCoding int32,
) int32 {
	var thisCtrl DecoderControl
	var L, mvLen, ret int32 = d.frameLength, 0, 0

	thisCtrl.LTPScaleQ14 = 0

	// Safety checks
	if L <= 0 || L > MAX_FRAME_LENGTH {
		panic("invalid frame length")
	}

	if lostFlag == FLAG_DECODE_NORMAL ||
		(lostFlag == FLAG_DECODE_LBRR && d.LBRRFlags[d.nFramesDecoded] == 1) {
		// Allocate pulses buffer with proper alignment
		pulseLength := (L + SHELL_CODEC_FRAME_LENGTH - 1) & ^(SHELL_CODEC_FRAME_LENGTH - 1)
		pulses := make([]int16, pulseLength)

		// Decode quantization indices of side info
		d.indices.Decode(d, psRangeDec, d.nFramesDecoded, lostFlag, condCoding)

		// Decode quantization indices of excitation
		DecodePulses(psRangeDec, pulses, d.indices.SignalType,
			d.indices.QuantOffsetType, d.frameLength)

		// Decode parameters and pulse signal
		d.DecodeParameters(&thisCtrl, condCoding)

		// Run inverse NSQ
		d.DecodeCore(&thisCtrl, pOut, pOutPtr, pulses)

		// Update PLC state
		d.PLC(&thisCtrl, pOut, pOutPtr, 0)

		d.lossCnt = 0
		d.prevSignalType = d.indices.SignalType
		if d.prevSignalType < 0 || d.prevSignalType > 2 {
			panic("invalid signal type")
		}

		// A frame has been decoded without errors
		d.firstFrameAfterReset = 0
	} else {
		// Handle packet loss by extrapolation
		d.PLC(&thisCtrl, pOut, pOutPtr, 1)
	}

	// Update output buffer
	if d.ltpMemLength < d.frameLength {
		panic("invalid ltp memory length")
	}
	mvLen = d.ltpMemLength - d.frameLength
	copy(d.outBuf[:mvLen], d.outBuf[d.frameLength:d.frameLength+mvLen])
	copy(d.outBuf[mvLen:], pOut[pOutPtr:pOutPtr+int(d.frameLength)])

	// Comfort noise generation / estimation
	d.CNG(&thisCtrl, pOut, pOutPtr, L)

	// Ensure smooth connection of extrapolated and good frames
	d.PLCGlueFrames(pOut, pOutPtr, L)

	// Update some decoder state variables
	d.lagPrev = thisCtrl.PitchL[d.nbSubfr-1]

	// Set output frame length
	*pN = L

	return ret
}
