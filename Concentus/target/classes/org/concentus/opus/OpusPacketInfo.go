package opus

import (
	"errors"
)

// OpusPacketInfo represents the parsed information from an Opus packet
type OpusPacketInfo struct {
	TOCByte       byte     // Table of Contents byte containing mode and frame info
	Frames        [][]byte // List of subframes in this packet
	PayloadOffset int      // Index of the start of the payload within the packet
}

// OpusBandwidth represents the bandwidth setting of an Opus stream
type OpusBandwidth int

const (
	OPUS_BANDWIDTH_NARROWBAND OpusBandwidth = iota
	OPUS_BANDWIDTH_MEDIUMBAND
	OPUS_BANDWIDTH_WIDEBAND
	OPUS_BANDWIDTH_SUPERWIDEBAND
	OPUS_BANDWIDTH_FULLBAND
)

// OpusMode represents the encoding mode used
type OpusMode int

const (
	MODE_SILK_ONLY OpusMode = iota
	MODE_CELT_ONLY
	MODE_HYBRID
)

// OpusError represents error codes from Opus operations
type OpusError int

const (
	OPUS_OK             OpusError = 0
	OPUS_BAD_ARG        OpusError = -1
	OPUS_INVALID_PACKET OpusError = -4
)

// ParseOpusPacket parses an Opus packet into a PacketInfo object containing one or more frames.
// Most applications don't need this as Opus_decode performs this internally.
func ParseOpusPacket(packet []byte, packetOffset, length int) (*OpusPacketInfo, error) {
	if len(packet) < packetOffset+length {
		return nil, errors.New("packet buffer too small")
	}

	// Get number of frames
	numFrames, err := getNumFrames(packet, packetOffset, length)
	if err != nil {
		return nil, err
	}

	// Parse the packet
	var toc byte
	frames := make([][]byte, numFrames)
	sizes := make([]int, numFrames)
	payloadOffset := 0
	parsedLength := 0

	count, err := opusPacketParseImpl(packet[packetOffset:packetOffset+length],
		0, false, &toc, frames, sizes, &payloadOffset, &parsedLength)
	if err != nil {
		return nil, err
	}

	if count != numFrames {
		return nil, errors.New("frame count mismatch")
	}

	// In Go, we can directly use byte slices without deep copying since slices are
	// already references to underlying arrays. The parse function creates new slices.
	return &OpusPacketInfo{
		TOCByte:       toc,
		Frames:        frames,
		PayloadOffset: payloadOffset,
	}, nil
}

// GetNumSamplesPerFrame calculates the number of samples per frame
func GetNumSamplesPerFrame(packet []byte, packetOffset, fs int) int {
	audiosize := 0
	switch {
	case (packet[packetOffset] & 0x80) != 0:
		// CELT-only mode
		audiosize = ((packet[packetOffset] >> 3) & 0x3)
		audiosize = (fs << audiosize) / 400
	case (packet[packetOffset] & 0x60) == 0x60:
		// Hybrid mode
		if (packet[packetOffset] & 0x08) != 0 {
			audiosize = fs / 50
		} else {
			audiosize = fs / 100
		}
	default:
		// SILK-only mode
		audiosize = ((packet[packetOffset] >> 3) & 0x3)
		if audiosize == 3 {
			audiosize = fs * 60 / 1000
		} else {
			audiosize = (fs << audiosize) / 100
		}
	}
	return audiosize
}

// GetBandwidth returns the encoded bandwidth of an Opus packet
func GetBandwidth(packet []byte, packetOffset int) OpusBandwidth {
	var bandwidth OpusBandwidth

	switch {
	case (packet[packetOffset] & 0x80) != 0:
		// CELT-only
		bw := ((packet[packetOffset] >> 5) & 0x3)
		bandwidth = OPUS_BANDWIDTH_NARROWBAND + OpusBandwidth(bw)
		if bandwidth == OPUS_BANDWIDTH_MEDIUMBAND {
			bandwidth = OPUS_BANDWIDTH_NARROWBAND
		}
	case (packet[packetOffset] & 0x60) == 0x60:
		// Hybrid
		if (packet[packetOffset] & 0x10) != 0 {
			bandwidth = OPUS_BANDWIDTH_FULLBAND
		} else {
			bandwidth = OPUS_BANDWIDTH_SUPERWIDEBAND
		}
	default:
		// SILK-only
		bw := ((packet[packetOffset] >> 5) & 0x3)
		bandwidth = OPUS_BANDWIDTH_NARROWBAND + OpusBandwidth(bw)
	}
	return bandwidth
}

// GetNumEncodedChannels returns the number of encoded channels (1 or 2)
func GetNumEncodedChannels(packet []byte, packetOffset int) int {
	if (packet[packetOffset] & 0x4) != 0 {
		return 2
	}
	return 1
}

// GetNumFrames returns the number of frames in an Opus packet
func GetNumFrames(packet []byte, packetOffset, length int) (int, error) {
	if length < 1 {
		return 0, errors.New("invalid packet length")
	}

	count := packet[packetOffset] & 0x3
	switch count {
	case 0:
		return 1, nil
	case 1, 2:
		return 2, nil
	case 3:
		if length < 2 {
			return 0, errors.New("invalid packet for count=3")
		}
		return int(packet[packetOffset+1] & 0x3F), nil
	default:
		return 0, errors.New("invalid frame count")
	}
}

// GetNumSamples calculates the total number of samples in the packet
func GetNumSamples(packet []byte, packetOffset, length, fs int) (int, error) {
	count, err := GetNumFrames(packet, packetOffset, length)
	if err != nil {
		return 0, err
	}

	samples := count * GetNumSamplesPerFrame(packet, packetOffset, fs)
	// Can't have more than 120 ms
	if samples*25 > fs*3 {
		return 0, errors.New("invalid packet duration")
	}
	return samples, nil
}

// GetEncoderMode returns the encoding mode used in the packet
func GetEncoderMode(packet []byte, packetOffset int) OpusMode {
	switch {
	case (packet[packetOffset] & 0x80) != 0:
		return MODE_CELT_ONLY
	case (packet[packetOffset] & 0x60) == 0x60:
		return MODE_HYBRID
	default:
		return MODE_SILK_ONLY
	}
}

// encodeSize encodes a size value into the data buffer
func encodeSize(size int, data []byte) int {
	if size < 252 {
		data[0] = byte(size)
		return 1
	}
	dp1 := 252 + (size & 0x3)
	data[0] = byte(dp1)
	data[1] = byte((size - dp1) >> 2)
	return 2
}

// parseSize decodes a size value from the data buffer
func parseSize(data []byte) (size int, bytesRead int, err error) {
	if len(data) < 1 {
		return 0, 0, errors.New("insufficient data")
	}

	val := int(data[0])
	if val < 252 {
		return val, 1, nil
	}

	if len(data) < 2 {
		return 0, 0, errors.New("insufficient data for size")
	}

	return 4*int(data[1]) + int(data[0]), 2, nil
}

// opusPacketParseImpl is the core packet parsing implementation
func opusPacketParseImpl(data []byte, selfDelimited bool, outToc *byte,
	frames [][]byte, sizes []int, payloadOffset, packetOffset *int) (int, error) {

	var toc byte
	count := 0
	cbr := false
	pad := 0
	dataPtr := 0
	*outToc = 0
	*payloadOffset = 0
	*packetOffset = 0

	if len(data) == 0 {
		return 0, errors.New("empty packet")
	}

	framesize := GetNumSamplesPerFrame(data, 0, 48000)

	toc = data[dataPtr]
	dataPtr++
	lastSize := len(data) - dataPtr

	switch toc & 0x3 {
	case 0: // One frame
		count = 1
	case 1: // Two CBR frames
		count = 2
		cbr = true
		if !selfDelimited {
			if (len(data)-dataPtr)&0x1 != 0 {
				return 0, errors.New("invalid CBR packet length")
			}
			lastSize = (len(data) - dataPtr) / 2
			sizes[0] = lastSize
		}
	case 2: // Two VBR frames
		count = 2
		size, bytesRead, err := parseSize(data[dataPtr:])
		if err != nil {
			return 0, err
		}
		sizes[0] = size
		dataPtr += bytesRead
		lastSize = len(data) - dataPtr - sizes[0]
		if sizes[0] < 0 || sizes[0] > len(data)-dataPtr {
			return 0, errors.New("invalid VBR size")
		}
	default: // Multiple frames (case 3)
		if len(data)-dataPtr < 1 {
			return 0, errors.New("invalid multi-frame packet")
		}
		ch := int(data[dataPtr])
		dataPtr++
		count = ch & 0x3F
		if count <= 0 || framesize*count > 5760 {
			return 0, errors.New("invalid frame count")
		}

		// Padding flag
		if (ch & 0x40) != 0 {
			for {
				if len(data)-dataPtr <= 0 {
					return 0, errors.New("invalid padding")
				}
				p := int(data[dataPtr])
				dataPtr++
				tmp := p
				if p == 255 {
					tmp = 254
				}
				if len(data)-dataPtr < tmp {
					return 0, errors.New("invalid padding length")
				}
				pad += tmp
				dataPtr += tmp
				if p != 255 {
					break
				}
			}
		}

		// VBR flag
		cbr = (ch & 0x80) == 0
		if !cbr {
			// VBR case
			lastSize = len(data) - dataPtr
			for i := 0; i < count-1; i++ {
				size, bytesRead, err := parseSize(data[dataPtr:])
				if err != nil {
					return 0, err
				}
				sizes[i] = size
				dataPtr += bytesRead
				lastSize -= bytesRead + sizes[i]
				if sizes[i] < 0 || sizes[i] > len(data)-dataPtr {
					return 0, errors.New("invalid VBR size")
				}
			}
			if lastSize < 0 {
				return 0, errors.New("invalid VBR sizes")
			}
		} else if !selfDelimited {
			// CBR case
			lastSize = (len(data) - dataPtr) / count
			if lastSize*count != len(data)-dataPtr {
				return 0, errors.New("invalid CBR sizes")
			}
			for i := 0; i < count-1; i++ {
				sizes[i] = lastSize
			}
		}
	}

	// Handle self-delimited framing
	if selfDelimited {
		size, bytesRead, err := parseSize(data[dataPtr:])
		if err != nil {
			return 0, err
		}
		sizes[count-1] = size
		dataPtr += bytesRead

		if cbr {
			// Apply size to all frames for CBR
			if sizes[count-1]*count > len(data)-dataPtr {
				return 0, errors.New("invalid CBR self-delimited sizes")
			}
			for i := 0; i < count-1; i++ {
				sizes[i] = sizes[count-1]
			}
		} else if bytesRead+sizes[count-1] > lastSize {
			return 0, errors.New("invalid VBR self-delimited sizes")
		}
	} else {
		// Reject packets that would be too large
		if lastSize > 1275 {
			return 0, errors.New("packet too large")
		}
		sizes[count-1] = lastSize
	}

	*payloadOffset = dataPtr

	// Extract frames
	for i := 0; i < count; i++ {
		if frames != nil {
			end := dataPtr + sizes[i]
			if end > len(data) {
				return 0, errors.New("frame extends beyond packet")
			}
			frames[i] = make([]byte, sizes[i])
			copy(frames[i], data[dataPtr:end])
		}
		dataPtr += sizes[i]
	}

	*packetOffset = pad + dataPtr
	*outToc = toc

	return count, nil
}
