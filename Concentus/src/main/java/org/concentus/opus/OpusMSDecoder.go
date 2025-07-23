package opus

import (
	"errors"
	"fmt"
)

// Error constants matching Opus error codes
const (
	OPUS_OK               = 0
	OPUS_BAD_ARG          = -1
	OPUS_BUFFER_TOO_SMALL = -2
	OPUS_INTERNAL_ERROR   = -3
	OPUS_INVALID_PACKET   = -4
	OPUS_INVALID_STATE    = -5
)

// ChannelLayout represents the audio channel configuration
type ChannelLayout struct {
	NbChannels       int
	NbStreams        int
	NbCoupledStreams int
	Mapping          [256]byte // Using fixed size array since max channels is 255
}

// OpusMSDecoder represents a multistream Opus decoder
type OpusMSDecoder struct {
	layout   ChannelLayout
	decoders []*OpusDecoder // Slice is more idiomatic than array in Go
}

// New creates a new multistream Opus decoder
func New(fs, channels, streams, coupledStreams int, mapping []byte) (*OpusMSDecoder, error) {
	// Validate input parameters
	if channels > 255 || channels < 1 || coupledStreams > streams ||
		streams < 1 || coupledStreams < 0 || streams > 255-coupledStreams {
		return nil, errors.New("invalid channel/stream configuration")
	}

	// Create decoder instance
	st := &OpusMSDecoder{
		decoders: make([]*OpusDecoder, streams),
		layout: ChannelLayout{
			NbChannels:       channels,
			NbStreams:        streams,
			NbCoupledStreams: coupledStreams,
		},
	}

	// Copy mapping (using byte instead of short since values are 0-255)
	for i := 0; i < channels; i++ {
		if i < len(mapping) {
			st.layout.Mapping[i] = mapping[i]
		} else {
			st.layout.Mapping[i] = 255 // Default to muted channel
		}
	}

	// Validate layout
	if !validateLayout(&st.layout) {
		return nil, errors.New("invalid surround channel layout")
	}

	// Initialize decoders
	decoderPtr := 0
	for i := 0; i < st.layout.NbCoupledStreams; i++ {
		dec, err := NewDecoder(fs, 2) // Stereo decoders for coupled streams
		if err != nil {
			return nil, fmt.Errorf("decoder init failed: %w", err)
		}
		st.decoders[decoderPtr] = dec
		decoderPtr++
	}

	for ; decoderPtr < st.layout.NbStreams; decoderPtr++ {
		dec, err := NewDecoder(fs, 1) // Mono decoders for remaining streams
		if err != nil {
			return nil, fmt.Errorf("decoder init failed: %w", err)
		}
		st.decoders[decoderPtr] = dec
	}

	return st, nil
}

// validateLayout checks if the channel layout is valid
func validateLayout(layout *ChannelLayout) bool {
	// Implementation would mirror Java's OpusMultistream.validate_layout
	// This is a placeholder - actual implementation would validate the mapping
	return true
}

// Decode decodes a multistream Opus packet
func (d *OpusMSDecoder) Decode(data []byte, outPcm []int16, frameSize int, decodeFec bool) (int, error) {
	return d.decodeNative(data, outPcm, frameSize, decodeFec, false)
}

// decodeNative implements the core decoding logic
func (d *OpusMSDecoder) decodeNative(data []byte, pcm []int16, frameSize int, decodeFec, softClip bool) (int, error) {
	// Limit frame size to avoid excessive allocations
	fs := d.SampleRate()
	if fs == 0 {
		return 0, errors.New("decoder not initialized")
	}
	frameSize = min(frameSize, fs/25*3)

	// Temporary buffer for decoded data
	buf := make([]int16, 2*frameSize)
	decoderPtr := 0
	doPlc := len(data) == 0 // Packet loss concealment if no data

	// Validate packet
	if len(data) < 0 {
		return 0, errors.New("bad argument")
	}
	if !doPlc && len(data) < 2*d.layout.NbStreams-1 {
		return 0, errors.New("invalid packet")
	}
	if !doPlc {
		samples, err := d.packetValidate(data, fs)
		if err != nil {
			return 0, err
		}
		if samples > frameSize {
			return 0, errors.New("buffer too small")
		}
	}

	// Process each stream
	var err error
	for s := 0; s < d.layout.NbStreams; s++ {
		dec := d.decoders[decoderPtr]
		decoderPtr++

		if !doPlc && len(data) <= 0 {
			return 0, errors.New("internal error")
		}

		var packetOffset int
		var ret int
		ret, packetOffset, err = dec.decodeNative(data, buf, frameSize, decodeFec, s != d.layout.NbStreams-1, softClip)
		if err != nil {
			return 0, err
		}
		if ret <= 0 {
			return ret, nil
		}

		data = data[packetOffset:]
		frameSize = ret

		// Distribute decoded audio to appropriate channels
		if s < d.layout.NbCoupledStreams {
			// Stereo stream processing
			prev := -1
			for {
				chanIdx := getLeftChannel(&d.layout, s, prev)
				if chanIdx == -1 {
					break
				}
				copyChannelOutShort(pcm, d.layout.NbChannels, chanIdx, buf, 2, frameSize)
				prev = chanIdx
			}

			prev = -1
			for {
				chanIdx := getRightChannel(&d.layout, s, prev)
				if chanIdx == -1 {
					break
				}
				copyChannelOutShort(pcm, d.layout.NbChannels, chanIdx, buf, 2, frameSize, 1)
				prev = chanIdx
			}
		} else {
			// Mono stream processing
			prev := -1
			for {
				chanIdx := getMonoChannel(&d.layout, s, prev)
				if chanIdx == -1 {
					break
				}
				copyChannelOutShort(pcm, d.layout.NbChannels, chanIdx, buf, 1, frameSize)
				prev = chanIdx
			}
		}
	}

	// Handle muted channels
	for c := 0; c < d.layout.NbChannels; c++ {
		if d.layout.Mapping[c] == 255 {
			copyChannelOutShort(pcm, d.layout.NbChannels, c, nil, 0, frameSize)
		}
	}

	return frameSize, nil
}

// packetValidate validates a multistream packet
func (d *OpusMSDecoder) packetValidate(data []byte, fs int) (int, error) {
	var samples int
	for s := 0; s < d.layout.NbStreams; s++ {
		if len(data) <= 0 {
			return 0, errors.New("invalid packet")
		}

		toc, size, packetOffset, err := parsePacket(data, s != d.layout.NbStreams-1)
		if err != nil {
			return 0, err
		}

		tmpSamples := getNumSamples(data, packetOffset, fs)
		if s != 0 && samples != tmpSamples {
			return 0, errors.New("invalid packet")
		}
		samples = tmpSamples
		data = data[packetOffset:]
	}
	return samples, nil
}

// copyChannelOutShort copies a channel from source to destination buffer
func copyChannelOutShort(dst []int16, dstStride, dstChannel int, src []int16, srcStride, frameSize int, srcOffset ...int) {
	offset := 0
	if len(srcOffset) > 0 {
		offset = srcOffset[0]
	}

	if src != nil {
		for i := 0; i < frameSize; i++ {
			dst[i*dstStride+dstChannel] = src[i*srcStride+offset]
		}
	} else {
		for i := 0; i < frameSize; i++ {
			dst[i*dstStride+dstChannel] = 0
		}
	}
}

// SampleRate returns the decoder's sample rate
func (d *OpusMSDecoder) SampleRate() int {
	if len(d.decoders) == 0 {
		return 0
	}
	return d.decoders[0].SampleRate()
}

// Bandwidth returns the decoder's bandwidth
func (d *OpusMSDecoder) Bandwidth() (Bandwidth, error) {
	if len(d.decoders) == 0 {
		return 0, errors.New("decoder not initialized")
	}
	return d.decoders[0].Bandwidth()
}

// Gain returns the decoder's gain
func (d *OpusMSDecoder) Gain() (int, error) {
	if len(d.decoders) == 0 {
		return 0, errors.New("decoder not initialized")
	}
	return d.decoders[0].Gain()
}

// SetGain sets the decoder's gain
func (d *OpusMSDecoder) SetGain(value int) error {
	for _, dec := range d.decoders {
		if err := dec.SetGain(value); err != nil {
			return err
		}
	}
	return nil
}

// LastPacketDuration returns the duration of the last decoded packet
func (d *OpusMSDecoder) LastPacketDuration() (int, error) {
	if len(d.decoders) == 0 {
		return 0, errors.New("invalid state")
	}
	return d.decoders[0].LastPacketDuration()
}

// FinalRange returns the final range value
func (d *OpusMSDecoder) FinalRange() (int, error) {
	if len(d.decoders) == 0 {
		return 0, errors.New("invalid state")
	}
	value := 0
	for _, dec := range d.decoders {
		r, err := dec.FinalRange()
		if err != nil {
			return 0, err
		}
		value ^= r
	}
	return value, nil
}

// Reset resets the decoder state
func (d *OpusMSDecoder) Reset() error {
	for _, dec := range d.decoders {
		if err := dec.Reset(); err != nil {
			return err
		}
	}
	return nil
}

// GetDecoder returns a specific stream decoder
func (d *OpusMSDecoder) GetDecoder(streamID int) (*OpusDecoder, error) {
	if streamID < 0 || streamID >= len(d.decoders) {
		return nil, errors.New("invalid stream ID")
	}
	return d.decoders[streamID], nil
}

// Helper functions (would be implemented elsewhere in the package)
func getLeftChannel(layout *ChannelLayout, stream, prev int) int {
	// Implementation would mirror Java's OpusMultistream.get_left_channel
	return -1
}

func getRightChannel(layout *ChannelLayout, stream, prev int) int {
	// Implementation would mirror Java's OpusMultistream.get_right_channel
	return -1
}

func getMonoChannel(layout *ChannelLayout, stream, prev int) int {
	// Implementation would mirror Java's OpusMultistream.get_mono_channel
	return -1
}

func parsePacket(data []byte, lastPacket bool) (byte, []int16, int, error) {
	// Implementation would mirror Java's OpusPacketInfo.opus_packet_parse_impl
	return 0, nil, 0, nil
}

func getNumSamples(data []byte, offset, fs int) int {
	// Implementation would mirror Java's OpusPacketInfo.getNumSamples
	return 0
}
