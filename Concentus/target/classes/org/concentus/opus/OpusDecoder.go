package opus

import (
	"errors"
	"fmt"
	"opustest/concentus"
)

// OpusDecoder represents the Opus decoder structure.
// Opus is a stateful codec with overlapping blocks, meaning packets must be
// decoded serially and in order. Lost packets can be replaced with loss concealment.
type OpusDecoder struct {
	channels   int
	Fs         int // Sampling rate (at the API level)
	DecControl concentus.DecControlState
	decodeGain int

	// Everything beyond this point gets cleared on a reset
	streamChannels     int
	bandwidth          OpusBandwidth
	mode               OpusMode
	prevMode           OpusMode
	frameSize          int
	prevRedundancy     int
	lastPacketDuration int
	rangeFinal         int
	silkDecoder        *SilkDecoder
	celtDecoder        *CeltDecoder
}

// NewOpusDecoder creates and initializes a new Opus decoder.
// Fs must be one of: 8000, 12000, 16000, 24000, or 48000 Hz.
// channels must be 1 (mono) or 2 (stereo).
func NewOpusDecoder(Fs, channels int) (*OpusDecoder, error) {
	if !isValidSampleRate(Fs) {
		return nil, errors.New("sample rate is invalid (must be 8/12/16/24/48 kHz)")
	}
	if channels != 1 && channels != 2 {
		return nil, errors.New("number of channels must be 1 or 2")
	}

	dec := &OpusDecoder{
		silkDecoder: newSilkDecoder(),
		celtDecoder: newCeltDecoder(),
	}

	if err := dec.init(Fs, channels); err != nil {
		return nil, fmt.Errorf("error initializing decoder: %w", err)
	}

	return dec, nil
}

func isValidSampleRate(Fs int) bool {
	switch Fs {
	case 8000, 12000, 16000, 24000, 48000:
		return true
	default:
		return false
	}
}

// init initializes the decoder state
func (d *OpusDecoder) init(Fs, channels int) error {
	d.reset()

	// Initialize SILK decoder
	d.streamChannels = channels
	d.channels = channels
	d.Fs = Fs
	d.DecControl.APISampleRate = Fs
	d.DecControl.NChannelsAPI = channels

	// Reset decoder
	if err := d.silkDecoder.init(); err != nil {
		return errors.New("internal error initializing SILK decoder")
	}

	// Initialize CELT decoder
	if err := d.celtDecoder.init(Fs, channels); err != nil {
		return errors.New("internal error initializing CELT decoder")
	}

	d.celtDecoder.setSignalling(0)
	d.prevMode = ModeUnknown
	d.frameSize = Fs / 400
	return nil
}

// reset fully resets the decoder state
func (d *OpusDecoder) reset() {
	d.channels = 0
	d.Fs = 0
	d.DecControl.reset()
	d.decodeGain = 0
	d.partialReset()
}

// partialReset resets part of the decoder state (OPUS_DECODER_RESET_START)
func (d *OpusDecoder) partialReset() {
	d.streamChannels = 0
	d.bandwidth = BandwidthUnknown
	d.mode = ModeUnknown
	d.prevMode = ModeUnknown
	d.frameSize = 0
	d.prevRedundancy = 0
	d.lastPacketDuration = 0
	d.rangeFinal = 0
}

// Decode decodes an Opus packet.
// inData can be nil for packet loss concealment.
// frameSize must be > 0 and should be large enough for the packet duration.
// decodeFEC indicates if we should decode forward error correction data.
func (d *OpusDecoder) Decode(inData []byte, outPcm []int16, frameSize int, decodeFEC bool) (int, error) {
	if frameSize <= 0 {
		return 0, errors.New("frame size must be > 0")
	}

	ret, err := d.decodeNative(inData, outPcm, frameSize, decodeFEC, false, nil)
	if err != nil {
		if errors.Is(err, ErrBadArg) {
			return 0, fmt.Errorf("bad argument while decoding: %w", err)
		}
		return 0, fmt.Errorf("decoding error: %w", err)
	}

	return ret, nil
}

// decodeNative is the internal decode implementation
func (d *OpusDecoder) decodeNative(
	inData []byte,
	outPcm []int16,
	frameSize int,
	decodeFEC bool,
	selfDelimited bool,
	packetOffset *int,
) (int, error) {
	if decodeFEC && !(decodeFEC || len(inData) == 0 || inData == nil) && frameSize%(d.Fs/400) != 0 {
		return 0, ErrBadArg
	}

	// Handle packet loss (PLC)
	if len(inData) == 0 || inData == nil {
		pcmCount := 0
		for pcmCount < frameSize {
			ret, err := d.decodeFrame(nil, outPcm[pcmCount*d.channels:], frameSize-pcmCount, 0)
			if err != nil {
				return 0, err
			}
			pcmCount += ret
		}
		d.lastPacketDuration = pcmCount
		return pcmCount, nil
	}

	// Parse packet info
	packetMode := getEncoderMode(inData)
	packetBandwidth := getBandwidth(inData)
	packetFrameSize := getNumSamplesPerFrame(inData, d.Fs)
	packetStreamChannels := getNumEncodedChannels(inData)

	// Parse packet
	sizes := make([]int, 48) // 48 x 2.5ms = 120ms max
	count, offset, err := opusPacketParseImpl(inData, selfDelimited, nil, sizes, packetOffset)
	if err != nil {
		return 0, err
	}

	// Handle FEC decoding
	if decodeFEC {
		// If no FEC can be present, run PLC
		if frameSize < packetFrameSize || packetMode == ModeCeltOnly || d.mode == ModeCeltOnly {
			return d.decodeNative(nil, outPcm, frameSize, false, false, nil)
		}

		// Run PLC on everything except the size we might have FEC for
		durationCopy := d.lastPacketDuration
		if frameSize-packetFrameSize != 0 {
			ret, err := d.decodeNative(nil, outPcm, frameSize-packetFrameSize, false, false, nil)
			if err != nil {
				d.lastPacketDuration = durationCopy
				return 0, err
			}
		}

		// Complete with FEC
		d.mode = packetMode
		d.bandwidth = packetBandwidth
		d.frameSize = packetFrameSize
		d.streamChannels = packetStreamChannels

		ret, err := d.decodeFrame(inData[offset:], outPcm[(frameSize-packetFrameSize)*d.channels:], packetFrameSize, 1)
		if err != nil {
			return 0, err
		}

		d.lastPacketDuration = frameSize
		return frameSize, nil
	}

	// Check output buffer size
	if count*packetFrameSize > frameSize {
		return 0, ErrBufferTooSmall
	}

	// Update decoder state
	d.mode = packetMode
	d.bandwidth = packetBandwidth
	d.frameSize = packetFrameSize
	d.streamChannels = packetStreamChannels

	// Decode all frames in the packet
	nbSamples := 0
	for i := 0; i < count; i++ {
		ret, err := d.decodeFrame(inData[offset:], outPcm[nbSamples*d.channels:], frameSize-nbSamples, 0)
		if err != nil {
			return 0, err
		}
		offset += sizes[i]
		nbSamples += ret
	}
	d.lastPacketDuration = nbSamples

	return nbSamples, nil
}

// decodeFrame decodes a single Opus frame
func (d *OpusDecoder) decodeFrame(data []byte, pcm []int16, frameSize, decodeFEC int) (int, error) {
	// Calculate frame durations
	F20 := d.Fs / 50
	F10 := F20 >> 1
	F5 := F10 >> 1
	F2_5 := F5 >> 1

	if frameSize < F2_5 {
		return 0, ErrBufferTooSmall
	}

	// Limit frame size to avoid excessive allocations
	frameSize = min(frameSize, d.Fs/25*3)

	// Handle PLC/DTX for small payloads
	var dec *entropyCoder
	var mode OpusMode
	var audiosize int

	if len(data) <= 1 {
		data = nil
		frameSize = min(frameSize, d.frameSize)
	}

	if data != nil {
		audiosize = d.frameSize
		mode = d.mode
		dec = newEntropyCoder(data)
	} else {
		audiosize = frameSize
		mode = d.prevMode

		if mode == ModeUnknown {
			// Return zeros if we haven't received any packets yet
			for i := range pcm[:audiosize*d.channels] {
				pcm[i] = 0
			}
			return audiosize, nil
		}

		// Handle PLC for non-standard sizes
		if audiosize > F20 {
			total := 0
			for total < audiosize {
				ret, err := d.decodeFrame(nil, pcm[total*d.channels:], min(audiosize, F20), 0)
				if err != nil {
					return 0, err
				}
				total += ret
			}
			return frameSize, nil
		} else if audiosize < F20 {
			if audiosize > F10 {
				audiosize = F10
			} else if mode != ModeSilkOnly && audiosize > F5 && audiosize < F10 {
				audiosize = F5
			}
		}
	}

	// Main decoding logic would continue here...
	// (Implementation of the full decodeFrame would continue with SILK/CELT processing,
	// transitions, redundancy handling, etc. Similar to the Java version but with
	// Go idioms and memory management.)

	// Placeholder return - actual implementation would return the decoded samples
	return audiosize, nil
}

// ResetState resets the decoder state without clearing the parameters
func (d *OpusDecoder) ResetState() {
	d.partialReset()
	d.celtDecoder.resetState()
	d.silkDecoder.init()
	d.streamChannels = d.channels
	d.frameSize = d.Fs / 400
}

// Getters for decoder properties
func (d *OpusDecoder) Bandwidth() OpusBandwidth { return d.bandwidth }
func (d *OpusDecoder) FinalRange() int          { return d.rangeFinal }
func (d *OpusDecoder) SampleRate() int          { return d.Fs }
func (d *OpusDecoder) Gain() int                { return d.decodeGain }
func (d *OpusDecoder) LastPacketDuration() int  { return d.lastPacketDuration }

// SetGain sets the decoder gain adjustment
func (d *OpusDecoder) SetGain(value int) error {
	if value < -32768 || value > 32767 {
		return errors.New("gain must be within the range of a signed int16")
	}
	d.decodeGain = value
	return nil
}

// GetPitch returns the pitch of the last decoded frame
func (d *OpusDecoder) GetPitch() int {
	if d.prevMode == ModeCeltOnly {
		return d.celtDecoder.getPitch()
	}
	return d.DecControl.prevPitchLag
}
