
package silk

import (
	"errors"
	"math"
)



// Error codes
var (
	ErrInvalidFrameSize            = errors.New("invalid frame size")
	ErrInvalidSamplingFrequency    = errors.New("invalid sampling frequency")
	ErrNoError                     = errors.New("no error")
)

// DecoderAPIFlag represents decoder API flags
type DecoderAPIFlag int

const (
	FLAG_DECODE_NORMAL    DecoderAPIFlag = 0
	FLAG_PACKET_LOST      DecoderAPIFlag = 1
	FLAG_DECODE_LBRR      DecoderAPIFlag = 2
)





// DecControlState contains decoder control parameters
type DecControlState struct {
	nChannelsAPI       int
	nChannelsInternal  int
	API_sampleRate     int
	internalSampleRate int
	payloadSize_ms     int
	prevPitchLag       int
}

// SilkDecoder represents the main decoder state
type SilkDecoder struct {
	channel_state           [DECODER_NUM_CHANNELS]*ChannelDecoder
	sStereo                 StereoState
	nChannelsAPI            int
	nChannelsInternal       int
	prev_decode_only_middle int
}

// Reset resets the decoder state
func (d *SilkDecoder) Reset() {
	// Implementation would reset all internal state
}

// StereoState represents stereo decoder state
type StereoState struct {
	pred_prev_Q13 [2]int
	sMid          [2]int16
	sSide         [2]int16
}

// Reset resets the stereo state
func (s *StereoState) Reset() {
	s.pred_prev_Q13 = [2]int{0, 0}
	s.sMid = [2]int16{0, 0}
	s.sSide = [2]int16{0, 0}
}

// ChannelDecoder represents per-channel decoder state
type ChannelDecoder struct {
	nFramesDecoded         int
	nFramesPerPacket       int
	nb_subfr               int
	fs_kHz                 int
	VAD_flags              [MAX_FRAMES_PER_PACKET]int
	LBRR_flag              int
	LBRR_flags             [MAX_FRAMES_PER_PACKET]int
	frame_length           int
	outBuf                 [MAX_FRAME_LENGTH + 2*MAX_SUB_FRAME_LENGTH]int16
	sLPC_Q14_buf           [MAX_LPC_ORDER]int32
	lagPrev                int
	LastGainIndex          int
	prevSignalType         int
	first_frame_after_reset int
	resampler_state        ResamplerState
	indices                FrameIndices
}

// FrameIndices contains frame indices
type FrameIndices struct {
	signalType       int
	quantOffsetType  int
	frame_length     int
}

// ResamplerState represents resampler state
type ResamplerState struct {
	// Implementation would contain resampler state
}

// Assign copies resampler state
func (r *ResamplerState) Assign(src ResamplerState) {
	// Implementation would copy state
}

// EntropyCoder represents the range decoder
type EntropyCoder struct {
	// Implementation would contain range decoder state
}

// InitDecoder initializes/resets the decoder state
func InitDecoder(decState *SilkDecoder) error {
	// Reset decoder
	decState.Reset()

	var ret error = ErrNoError

	// Initialize channel states
	for n := 0; n < DECODER_NUM_CHANNELS; n++ {
		if err := decState.channel_state[n].initDecoder(); err != nil {
			ret = err
		}
	}

	// Reset stereo state
	decState.sStereo.Reset()

	// Clean initialization
	decState.prev_decode_only_middle = 0

	return ret
}

// Decode decodes a frame of SILK audio
func Decode(
	psDec *SilkDecoder,
	decControl *DecControlState,
	lostFlag DecoderAPIFlag,
	newPacketFlag int,
	psRangeDec *EntropyCoder,
	samplesOut []int16,
	nSamplesOut *int,
) error {
	var (
		decode_only_middle int
		ret                error = ErrNoError
		nSamplesOutDec     int
		MS_pred_Q13        = [2]int{0, 0}
		channel_state      = psDec.channel_state
	)

	// Validate number of channels
	if decControl.nChannelsInternal != 1 && decControl.nChannelsInternal != 2 {
		return ErrInvalidFrameSize
	}

	*nSamplesOut = 0

	// Test if first frame in payload
	if newPacketFlag != 0 {
		for n := 0; n < decControl.nChannelsInternal; n++ {
			channel_state[n].nFramesDecoded = 0
		}
	}

	// If Mono -> Stereo transition in bitstream: init state of second channel
	if decControl.nChannelsInternal > psDec.nChannelsInternal {
		if err := channel_state[1].initDecoder(); err != nil {
			ret = err
		}
	}

	// Check for stereo to mono transition
	stereo_to_mono := 0
	if decControl.nChannelsInternal == 1 && psDec.nChannelsInternal == 2 &&
		decControl.internalSampleRate == 1000*channel_state[0].fs_kHz {
		stereo_to_mono = 1
	}

	// Initialize frame parameters if first frame
	if channel_state[0].nFramesDecoded == 0 {
		for n := 0; n < decControl.nChannelsInternal; n++ {
			// Set frames per packet based on payload size
			switch decControl.payloadSize_ms {
			case 0, 10:
				channel_state[n].nFramesPerPacket = 1
				channel_state[n].nb_subfr = 2
			case 20:
				channel_state[n].nFramesPerPacket = 1
				channel_state[n].nb_subfr = 4
			case 40:
				channel_state[n].nFramesPerPacket = 2
				channel_state[n].nb_subfr = 4
			case 60:
				channel_state[n].nFramesPerPacket = 3
				channel_state[n].nb_subfr = 4
			default:
				return ErrInvalidFrameSize
			}

			// Validate sample rate
			fs_kHz_dec := (decControl.internalSampleRate >> 10) + 1
			if fs_kHz_dec != 8 && fs_kHz_dec != 12 && fs_kHz_dec != 16 {
				return ErrInvalidSamplingFrequency
			}

			// Set sample rate
			if err := channel_state[n].setSampleRate(fs_kHz_dec, decControl.API_sampleRate); err != nil {
				ret = err
			}
		}
	}

	// Handle channel configuration changes
	if decControl.nChannelsAPI == 2 && decControl.nChannelsInternal == 2 &&
		(psDec.nChannelsAPI == 1 || psDec.nChannelsInternal == 1) {
		psDec.sStereo.pred_prev_Q13 = [2]int{0, 0}
		psDec.sStereo.sSide = [2]int16{0, 0}
		channel_state[1].resampler_state.Assign(channel_state[0].resampler_state)
	}
	psDec.nChannelsAPI = decControl.nChannelsAPI
	psDec.nChannelsInternal = decControl.nChannelsInternal

	// Validate API sample rate
	if decControl.API_sampleRate > MAX_API_FS_KHZ*1000 || decControl.API_sampleRate < 8000 {
		return ErrInvalidSamplingFrequency
	}

	// Decode VAD and LBRR flags for new packets
	if lostFlag != FLAG_PACKET_LOST && channel_state[0].nFramesDecoded == 0 {
		for n := 0; n < decControl.nChannelsInternal; n++ {
			// Decode VAD flags
			for i := 0; i < channel_state[n].nFramesPerPacket; i++ {
				channel_state[n].VAD_flags[i] = psRangeDec.decodeBitLogp(1)
			}

			// Decode LBRR flag
			channel_state[n].LBRR_flag = psRangeDec.decodeBitLogp(1)

			// Decode LBRR flags
			channel_state[n].LBRR_flags = [MAX_FRAMES_PER_PACKET]int{}
			if channel_state[n].LBRR_flag != 0 {
				if channel_state[n].nFramesPerPacket == 1 {
					channel_state[n].LBRR_flags[0] = 1
				} else {
					LBRR_symbol := psRangeDec.decodeICDF(SilkTables.LBRR_flags_iCDF[channel_state[n].nFramesPerPacket-2], 8) + 1
					for i := 0; i < channel_state[n].nFramesPerPacket; i++ {
						channel_state[n].LBRR_flags[i] = (LBRR_symbol >> i) & 1
					}
				}
			}
		}

		// For normal decoding, skip LBRR data
		if lostFlag == FLAG_DECODE_NORMAL {
			for i := 0; i < channel_state[0].nFramesPerPacket; i++ {
				for n := 0; n < decControl.nChannelsInternal; n++ {
					if channel_state[n].LBRR_flags[i] != 0 {
						pulses := make([]int16, MAX_FRAME_LENGTH)
						var condCoding int

						// Stereo handling
						if decControl.nChannelsInternal == 2 && n == 0 {
							StereoDecodePred(psRangeDec, &MS_pred_Q13)
							if channel_state[1].LBRR_flags[i] == 0 {
								StereoDecodeMidOnly(psRangeDec, &decode_only_middle)
							}
						}

						// Determine coding mode
						if i > 0 && channel_state[n].LBRR_flags[i-1] != 0 {
							condCoding = CODE_CONDITIONALLY
						} else {
							condCoding = CODE_INDEPENDENTLY
						}

						// Decode indices and pulses
						DecodeIndices(channel_state[n], psRangeDec, i, 1, condCoding)
						DecodePulses(psRangeDec, pulses, channel_state[n].indices.signalType,
							channel_state[n].indices.quantOffsetType, channel_state[n].frame_length)
					}
				}
			}
		}
	}

	// Get MS predictor index for stereo
	if decControl.nChannelsInternal == 2 {
		if lostFlag == FLAG_DECODE_NORMAL ||
			(lostFlag == FLAG_DECODE_LBRR && channel_state[0].LBRR_flags[channel_state[0].nFramesDecoded] == 1) {
			StereoDecodePred(psRangeDec, &MS_pred_Q13)

			// Decode mid-only flag for LBRR data if side-channel's LBRR flag is false
			if (lostFlag == FLAG_DECODE_NORMAL && channel_state[1].VAD_flags[channel_state[0].nFramesDecoded] == 0) ||
				(lostFlag == FLAG_DECODE_LBRR && channel_state[1].LBRR_flags[channel_state[0].nFramesDecoded] == 0) {
				StereoDecodeMidOnly(psRangeDec, &decode_only_middle)
			} else {
				decode_only_middle = 0
			}
		} else {
			MS_pred_Q13 = psDec.sStereo.pred_prev_Q13
		}
	}

	// Reset side channel decoder prediction memory when transitioning from mid-only to full stereo
	if decControl.nChannelsInternal == 2 && decode_only_middle == 0 && psDec.prev_decode_only_middle == 1 {
		for i := range channel_state[1].outBuf {
			channel_state[1].outBuf[i] = 0
		}
		for i := range channel_state[1].sLPC_Q14_buf {
			channel_state[1].sLPC_Q14_buf[i] = 0
		}
		channel_state[1].lagPrev = 100
		channel_state[1].LastGainIndex = 10
		channel_state[1].prevSignalType = TYPE_NO_VOICE_ACTIVITY
		channel_state[1].first_frame_after_reset = 1
	}

	// Allocate temporary buffers
	var (
		samplesOut_tmp      []int16
		samplesOut_tmp_ptrs [2]int
	)

	// Determine if we can delay buffer allocation
	delay_stack_alloc := decControl.internalSampleRate*decControl.nChannelsInternal <
		decControl.API_sampleRate*decControl.nChannelsAPI

	if delay_stack_alloc {
		samplesOut_tmp = samplesOut
		samplesOut_tmp_ptrs[0] = 0
		samplesOut_tmp_ptrs[1] = channel_state[0].frame_length + 2
	} else {
		samplesOut_tmp = make([]int16, decControl.nChannelsInternal*(channel_state[0].frame_length+2))
		samplesOut_tmp_ptrs[0] = 0
		samplesOut_tmp_ptrs[1] = channel_state[0].frame_length + 2
	}

	// Determine if we have side channel data
	has_side := 0
	if lostFlag == FLAG_DECODE_NORMAL {
		if decode_only_middle == 0 {
			has_side = 1
		}
	} else {
		if psDec.prev_decode_only_middle == 0 ||
			(decControl.nChannelsInternal == 2 &&
				lostFlag == FLAG_DECODE_LBRR &&
				channel_state[1].LBRR_flags[channel_state[1].nFramesDecoded] == 1) {
			has_side = 1
		}
	}

	// Decode each channel
	for n := 0; n < decControl.nChannelsInternal; n++ {
		if n == 0 || has_side != 0 {
			frameIndex := channel_state[0].nFramesDecoded - n
			var condCoding int

			// Determine coding mode
			if frameIndex <= 0 {
				condCoding = CODE_INDEPENDENTLY
			} else if lostFlag == FLAG_DECODE_LBRR {
				if channel_state[n].LBRR_flags[frameIndex-1] != 0 {
					condCoding = CODE_CONDITIONALLY
				} else {
					condCoding = CODE_INDEPENDENTLY
				}
			} else if n > 0 && psDec.prev_decode_only_middle != 0 {
				condCoding = CODE_INDEPENDENTLY_NO_LTP_SCALING
			} else {
				condCoding = CODE_CONDITIONALLY
			}

			// Decode frame
			if err := channel_state[n].decodeFrame(
				psRangeDec,
				samplesOut_tmp,
				samplesOut_tmp_ptrs[n]+2,
				&nSamplesOutDec,
				lostFlag,
				condCoding,
			); err != nil {
				ret = err
			}
		} else {
			// Zero out samples if no side channel
			for i := samplesOut_tmp_ptrs[n] + 2; i < samplesOut_tmp_ptrs[n]+2+nSamplesOutDec; i++ {
				samplesOut_tmp[i] = 0
			}
		}
		channel_state[n].nFramesDecoded++
	}

	// Handle stereo processing
	if decControl.nChannelsAPI == 2 && decControl.nChannelsInternal == 2 {
		// Convert Mid/Side to Left/Right
		StereoMSToLR(
			&psDec.sStereo,
			samplesOut_tmp,
			samplesOut_tmp_ptrs[0],
			samplesOut_tmp,
			samplesOut_tmp_ptrs[1],
			MS_pred_Q13,
			channel_state[0].fs_kHz,
			nSamplesOutDec,
		)
	} else {
		// Buffering for mono
		copy(psDec.sStereo.sMid[:], samplesOut_tmp[samplesOut_tmp_ptrs[0]:samplesOut_tmp_ptrs[0]+2])
		copy(samplesOut_tmp[samplesOut_tmp_ptrs[0]:samplesOut_tmp_ptrs[0]+2],
			samplesOut_tmp[samplesOut_tmp_ptrs[0]+nSamplesOutDec:samplesOut_tmp_ptrs[0]+nSamplesOutDec+2])
	}

	// Calculate number of output samples
	*nSamplesOut = nSamplesOutDec * decControl.API_sampleRate / (channel_state[0].fs_kHz