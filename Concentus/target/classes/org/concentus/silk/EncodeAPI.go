
package silk

import (
	"errors"
	"math"
)



// Constants

// Tuning parameters
const (
	BITRESERVOIR_DECAY_TIME_MS         = 500
	SPEECH_ACTIVITY_DTX_THRES          = 0.1
	MAX_BANDWIDTH_SWITCH_DELAY_MS      = 20000
)

// Encoder represents the Silk encoder state
type Encoder struct {
	stateFxx        [ENCODER_NUM_CHANNELS]*ChannelEncoder
	sStereo         StereoState
	nChannelsAPI    int
	nChannelsInternal int
	allowBandwidthSwitch bool
	nBitsExceeded   int
	nBitsUsedLBRR   int
	nPrevChannelsInternal int
	timeSinceSwitchAllowedMs int
	prevDecodeOnlyMiddle bool
}



// NewEncoder creates a new Silk encoder instance
func NewEncoder() *Encoder {
	return &Encoder{
		stateFxx: [ENCODER_NUM_CHANNELS]*ChannelEncoder{
			NewChannelEncoder(),
			NewChannelEncoder(),
		},
		nChannelsAPI:    1,
		nChannelsInternal: 1,
	}
}

// InitEncoder initializes or resets the encoder
func InitEncoder(encState *Encoder, encStatus *EncControlState) error {
	// Reset encoder
	encState.Reset()

	// Initialize channel encoders
	for n := 0; n < ENCODER_NUM_CHANNELS; n++ {
		if err := encState.stateFxx[n].Init(); err != nil {
			return err
		}
	}

	encState.nChannelsAPI = 1
	encState.nChannelsInternal = 1

	// Read control structure
	return QueryEncoder(encState, encStatus)
}

// QueryEncoder reads control structure from encoder
func QueryEncoder(encState *Encoder, encStatus *EncControlState) error {
	stateFxx := encState.stateFxx[0]

	*encStatus = EncControlState{
		nChannelsAPI:              encState.nChannelsAPI,
		nChannelsInternal:         encState.nChannelsInternal,
		API_sampleRate:            stateFxx.API_fs_Hz,
		maxInternalSampleRate:     stateFxx.maxInternal_fs_Hz,
		minInternalSampleRate:     stateFxx.minInternal_fs_Hz,
		desiredInternalSampleRate: stateFxx.desiredInternal_fs_Hz,
		payloadSizeMs:            stateFxx.PacketSize_ms,
		bitRate:                  stateFxx.TargetRate_bps,
		packetLossPercentage:     stateFxx.PacketLoss_perc,
		complexity:               stateFxx.Complexity,
		useInBandFEC:             stateFxx.useInBandFEC,
		useDTX:                   stateFxx.useDTX,
		useCBR:                   stateFxx.useCBR,
		internalSampleRate:       stateFxx.fs_kHz * 1000,
		allowBandwidthSwitch:     stateFxx.allow_bandwidth_switch,
		inWBmodeWithoutVariableLP: stateFxx.fs_kHz == 16 && stateFxx.sLP.mode == 0,
	}

	return nil
}

// Encode encodes a frame with Silk
func Encode(
	psEnc *Encoder,
	encControl *EncControlState,
	samplesIn []int16,
	nSamplesIn int,
	psRangeEnc *EntropyCoder,
	prefillFlag bool,
) (nBytesOut int, err error) {
	// Key translation decisions:
	// 1. Used Go error handling instead of return codes
	// 2. Replaced BoxedValueInt with direct return of nBytesOut
	// 3. Used slices instead of arrays where appropriate
	// 4. Simplified memory management with Go's GC
	// 5. Used idiomatic Go naming conventions
	// 6. Removed redundant assertions and added proper error returns
	// 7. Simplified control flow where possible

	if encControl.reducedDependency {
		psEnc.stateFxx[0].first_frame_after_reset = 1
		psEnc.stateFxx[1].first_frame_after_reset = 1
	}
	psEnc.stateFxx[0].nFramesEncoded = 0
	psEnc.stateFxx[1].nFramesEncoded = 0

	// Check values in encoder control structure
	if err := encControl.checkControlInput(); err != nil {
		return 0, err
	}

	encControl.switchReady = false

	// Handle mono to stereo transition
	if encControl.nChannelsInternal > psEnc.nChannelsInternal {
		if err := psEnc.stateFxx[1].Init(); err != nil {
			return 0, err
		}

		// Initialize stereo state
		psEnc.sStereo = StereoState{
			pred_prev_Q13:    [2]int16{0, 0},
			sSide:            [2]int16{0, 0},
			mid_side_amp_Q0:  [4]int16{0, 1, 0, 1},
			width_prev_Q14:   0,
			smth_width_Q14:   int16(float32(1.0) * (1 << 14)),
		}

		if psEnc.nChannelsAPI == 2 {
			psEnc.stateFxx[1].resampler_state = psEnc.stateFxx[0].resampler_state
			copy(psEnc.stateFxx[1].In_HP_State[:], psEnc.stateFxx[0].In_HP_State[:])
		}
	}

	transition := encControl.payloadSizeMs != psEnc.stateFxx[0].PacketSize_ms || 
		psEnc.nChannelsInternal != encControl.nChannelsInternal

	psEnc.nChannelsAPI = encControl.nChannelsAPI
	psEnc.nChannelsInternal = encControl.nChannelsInternal

	nBlocksOf10ms := (100 * nSamplesIn) / encControl.API_sampleRate
	tot_blocks := 1
	if nBlocksOf10ms > 1 {
		tot_blocks = nBlocksOf10ms >> 1
	}
	curr_block := 0

	var tmp_payloadSizeMs, tmp_complexity int
	if prefillFlag {
		// Only accept input length of 10 ms
		if nBlocksOf10ms != 1 {
			return 0, errors.New("invalid number of samples for prefill")
		}

		// Reset encoder
		for n := 0; n < encControl.nChannelsInternal; n++ {
			if err := psEnc.stateFxx[n].Init(); err != nil {
				return 0, err
			}
		}

		tmp_payloadSizeMs = encControl.payloadSizeMs
		encControl.payloadSizeMs = 10
		tmp_complexity = encControl.complexity
		encControl.complexity = 0

		for n := 0; n < encControl.nChannelsInternal; n++ {
			psEnc.stateFxx[n].controlled_since_last_payload = 0
			psEnc.stateFxx[n].prefillFlag = true
		}
	} else {
		// Only accept input lengths that are a multiple of 10 ms
		if nBlocksOf10ms*encControl.API_sampleRate != 100*nSamplesIn || nSamplesIn < 0 {
			return 0, errors.New("invalid number of samples")
		}

		// Make sure no more than one packet can be produced
		if 1000*nSamplesIn > encControl.payloadSizeMs*encControl.API_sampleRate {
			return 0, errors.New("too many samples for payload size")
		}
	}

	TargetRate_bps := encControl.bitRate >> (encControl.nChannelsInternal - 1)

	// Process each channel
	for n := 0; n < encControl.nChannelsInternal; n++ {
		force_fs_kHz := 0
		if n == 1 {
			force_fs_kHz = psEnc.stateFxx[0].fs_kHz
		}

		if err := psEnc.stateFxx[n].ControlEncoder(
			encControl, 
			TargetRate_bps, 
			psEnc.allowBandwidthSwitch, 
			n, 
			force_fs_kHz,
		); err != nil {
			return 0, err
		}

		if psEnc.stateFxx[n].first_frame_after_reset || transition {
			for i := 0; i < psEnc.stateFxx[0].nFramesPerPacket; i++ {
				psEnc.stateFxx[n].LBRR_flags[i] = false
			}
		}

		psEnc.stateFxx[n].inDTX = psEnc.stateFxx[n].useDTX
	}

	if encControl.nChannelsInternal > 1 && psEnc.stateFxx[0].fs_kHz != psEnc.stateFxx[1].fs_kHz {
		return 0, errors.New("channel sample rate mismatch")
	}

	// Input buffering/resampling and encoding
	nSamplesToBufferMax := 10 * nBlocksOf10ms * psEnc.stateFxx[0].fs_kHz
	nSamplesFromInputMax := (nSamplesToBufferMax * psEnc.stateFxx[0].API_fs_Hz) / (psEnc.stateFxx[0].fs_kHz * 1000)

	buf := make([]int16, nSamplesFromInputMax)
	samplesIn_ptr := 0

	for {
		nSamplesToBuffer := psEnc.stateFxx[0].frame_length - psEnc.stateFxx[0].inputBufIx
		if nSamplesToBuffer > nSamplesToBufferMax {
			nSamplesToBuffer = nSamplesToBufferMax
		}
		nSamplesFromInput := (nSamplesToBuffer * psEnc.stateFxx[0].API_fs_Hz) / (psEnc.stateFxx[0].fs_kHz * 1000)

		// Resample and write to buffer
		switch {
		case encControl.nChannelsAPI == 2 && encControl.nChannelsInternal == 2:
			// Stereo input to stereo output
			for n := 0; n < nSamplesFromInput; n++ {
				buf[n] = samplesIn[samplesIn_ptr+2*n]
			}

			// Sync resampler states when switching from mono to stereo
			if psEnc.nPrevChannelsInternal == 1 && psEnc.stateFxx[0].nFramesEncoded == 0 {
				psEnc.stateFxx[1].resampler_state = psEnc.stateFxx[0].resampler_state
			}

			if err := Resample(
				psEnc.stateFxx[0].resampler_state,
				psEnc.stateFxx[0].inputBuf[psEnc.stateFxx[0].inputBufIx+2:],
				buf[:nSamplesFromInput],
			); err != nil {
				return 0, err
			}
			psEnc.stateFxx[0].inputBufIx += nSamplesToBuffer

			// Process right channel
			nSamplesToBuffer = psEnc.stateFxx[1].frame_length - psEnc.stateFxx[1].inputBufIx
			if nSamplesToBuffer > 10*nBlocksOf10ms*psEnc.stateFxx[1].fs_kHz {
				nSamplesToBuffer = 10 * nBlocksOf10ms * psEnc.stateFxx[1].fs_kHz
			}
			for n := 0; n < nSamplesFromInput; n++ {
				buf[n] = samplesIn[samplesIn_ptr+2*n+1]
			}

			if err := Resample(
				psEnc.stateFxx[1].resampler_state,
				psEnc.stateFxx[1].inputBuf[psEnc.stateFxx[1].inputBufIx+2:],
				buf[:nSamplesFromInput],
			); err != nil {
				return 0, err
			}
			psEnc.stateFxx[1].inputBufIx += nSamplesToBuffer

		case encControl.nChannelsAPI == 2 && encControl.nChannelsInternal == 1:
			// Stereo input to mono output
			for n := 0; n < nSamplesFromInput; n++ {
				sum := int32(samplesIn[samplesIn_ptr+2*n]) + int32(samplesIn[samplesIn_ptr+2*n+1])
				buf[n] = int16(sum >> 1)
			}

			if err := Resample(
				psEnc.stateFxx[0].resampler_state,
				psEnc.stateFxx[0].inputBuf[psEnc.stateFxx[0].inputBufIx+2:],
				buf[:nSamplesFromInput],
			); err != nil {
				return 0, err
			}

			// Average resampler states when switching from stereo to mono
			if psEnc.nPrevChannelsInternal == 2 && psEnc.stateFxx[0].nFramesEncoded == 0 {
				if err := Resample(
					psEnc.stateFxx[1].resampler_state,
					psEnc.stateFxx[1].inputBuf[psEnc.stateFxx[1].inputBufIx+2:],
					buf[:nSamplesFromInput],
				); err != nil {
					return 0, err
				}

				for n := 0; n < psEnc.stateFxx[0].frame_length; n++ {
					psEnc.stateFxx[0].inputBuf[psEnc.stateFxx[0].inputBufIx+n+2] = 
						(psEnc.stateFxx[0].inputBuf[psEnc.stateFxx[0].inputBufIx+n+2] + 
							psEnc.stateFxx[1].inputBuf[psEnc.stateFxx[1].inputBufIx+n+2]) >> 1
				}
			}
			psEnc.stateFxx[0].inputBufIx += nSamplesToBuffer

		default:
			// Mono input to mono output
			copy(buf, samplesIn[samplesIn_ptr:samplesIn_ptr+nSamplesFromInput])
			if err := Resample(
				psEnc.stateFxx[0].resampler_state,
				psEnc.stateFxx[0].inputBuf[psEnc.stateFxx[0].inputBufIx+2:],
				buf[:nSamplesFromInput],
			); err != nil {
				return 0, err
			}
			psEnc.stateFxx[0].inputBufIx += nSamplesToBuffer
		}

		samplesIn_ptr += nSamplesFromInput * encControl.nChannelsAPI
		nSamplesIn -= nSamplesFromInput

		psEnc.allowBandwidthSwitch = false

		// Encode when we have enough samples
		if psEnc.stateFxx[0].inputBufIx >= psEnc.stateFxx[0].frame_length {
			if encControl.nChannelsInternal > 1 && psEnc.stateFxx[1].inputBufIx != psEnc.stateFxx[1].frame_length {
				return 0, errors.New("channel buffer size mismatch")
			}

			// Handle LBRR data
			if psEnc.stateFxx[0].nFramesEncoded == 0 && !prefillFlag {
				// Create space for VAD and FEC flags
				iCDF := []int16{0, int16(256 - (256 >> (psEnc.stateFxx[0].nFramesPerPacket + 1) * encControl.nChannelsInternal))}
				psRangeEnc.EncodeICDF(0, iCDF, 8)

				// Encode LBRR flags
				for n := 0; n < encControl.nChannelsInternal; n++ {
					LBRR_symbol := 0
					for i := 0; i < psEnc.stateFxx[n].nFramesPerPacket; i++ {
						if psEnc.stateFxx[n].LBRR_flags[i] {
							LBRR_symbol |= 1 << i
						}
					}

					psEnc.stateFxx[n].LBRR_flag = LBRR_symbol > 0
					if LBRR_symbol > 0 && psEnc.stateFxx[n].nFramesPerPacket > 1 {
						psRangeEnc.EncodeICDF(LBRR_symbol-1, silk_LBRR_flags_iCDF_ptr[psEnc.stateFxx[n].nFramesPerPacket-2], 8)
				