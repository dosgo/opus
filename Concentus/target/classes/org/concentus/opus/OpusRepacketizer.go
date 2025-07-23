package opus

import (
	"errors"
)

// Repacketizer represents an Opus repacketizer state
type Repacketizer struct {
	toc       byte     // Table of Contents byte
	nbFrames  int      // Number of frames currently stored
	frames    [][]byte // Stored frames (max 48)
	len       [48]int  // Length of each frame
	frameSize int      // Size of each frame in samples
}

// NewRepacketizer creates and initializes a new Repacketizer
func NewRepacketizer() *Repacketizer {
	rp := &Repacketizer{}
	rp.Reset()
	return rp
}

// Reset reinitializes the repacketizer state
func (rp *Repacketizer) Reset() {
	rp.nbFrames = 0
}

// Error definitions
var (
	ErrInvalidPacket  = errors.New("invalid packet")
	ErrBadArgument    = errors.New("bad argument")
	ErrBufferTooSmall = errors.New("buffer too small")
	ErrMaxPacketSize  = errors.New("packet exceeds maximum size")
)

// AddPacket adds a packet to the current repacketizer state
func (rp *Repacketizer) AddPacket(data []byte, offset int) error {
	return rp.catImpl(data, offset, len(data)-offset, false)
}

// GetNumFrames returns the total number of frames contained in packet data
func (rp *Repacketizer) GetNumFrames() int {
	return rp.nbFrames
}

// catImpl is the internal implementation for adding packets
func (rp *Repacketizer) catImpl(data []byte, offset, length int, selfDelimited bool) error {
	if length < 1 {
		return ErrInvalidPacket
	}

	if rp.nbFrames == 0 {
		rp.toc = data[offset]
		rp.frameSize = getNumSamplesPerFrame(data, offset, 8000)
	} else if (rp.toc & 0xFC) != (data[offset] & 0xFC) {
		return ErrInvalidPacket
	}

	currNbFrames := getNumFrames(data, offset, length)
	if currNbFrames < 1 {
		return ErrInvalidPacket
	}

	// Check the 120 ms maximum packet size
	if (currNbFrames+rp.nbFrames)*rp.frameSize > 960 {
		return ErrMaxPacketSize
	}

	var dummyToc byte
	var dummyOffset int
	ret := opusPacketParseImpl(data, offset, length, selfDelimited, &dummyToc,
		rp.frames[:], rp.nbFrames, rp.len[:], rp.nbFrames, &dummyOffset, &dummyOffset)
	if ret < 1 {
		return ErrInvalidPacket
	}

	rp.nbFrames += currNbFrames
	return nil
}

// CreatePacket constructs a new packet from the specified range of frames
func (rp *Repacketizer) CreatePacket(begin, end int, data []byte, offset, maxlen int) (int, error) {
	return rp.outRangeImpl(begin, end, data, offset, maxlen, false, false)
}

// CreateFullPacket constructs a new packet from all frames
func (rp *Repacketizer) CreateFullPacket(data []byte, offset, maxlen int) (int, error) {
	return rp.outRangeImpl(0, rp.nbFrames, data, offset, maxlen, false, false)
}

// outRangeImpl is the internal implementation for creating packets from frame ranges
func (rp *Repacketizer) outRangeImpl(begin, end int, data []byte, ptr, maxlen int, selfDelimited, pad bool) (int, error) {
	if begin < 0 || begin >= end || end > rp.nbFrames {
		return 0, ErrBadArgument
	}

	count := end - begin
	totSize := 0

	if selfDelimited {
		totSize = 1
		if rp.len[count-1] >= 252 {
			totSize++
		}
	}

	if count == 1 {
		// Code 0
		totSize += rp.len[0] + 1
		if totSize > maxlen {
			return 0, ErrBufferTooSmall
		}
		data[ptr] = rp.toc & 0xFC
		ptr++
	} else if count == 2 {
		if rp.len[1] == rp.len[0] {
			// Code 1
			totSize += 2*rp.len[0] + 1
			if totSize > maxlen {
				return 0, ErrBufferTooSmall
			}
			data[ptr] = (rp.toc & 0xFC) | 0x1
			ptr++
		} else {
			// Code 2
			totSize += rp.len[0] + rp.len[1] + 2
			if rp.len[0] >= 252 {
				totSize++
			}
			if totSize > maxlen {
				return 0, ErrBufferTooSmall
			}
			data[ptr] = (rp.toc & 0xFC) | 0x2
			ptr++
			ptr += encodeSize(rp.len[0], data[ptr:])
		}
	}

	if count > 2 || (pad && totSize < maxlen) {
		// Code 3
		var vbr bool
		padAmount := 0

		// Restart for padding case
		ptr = offset
		if selfDelimited {
			totSize = 1
			if rp.len[count-1] >= 252 {
				totSize++
			}
		} else {
			totSize = 0
		}

		vbr = false
		for i := 1; i < count; i++ {
			if rp.len[i] != rp.len[0] {
				vbr = true
				break
			}
		}

		if vbr {
			totSize += 2
			for i := 0; i < count-1; i++ {
				totSize += 1 + rp.len[i]
				if rp.len[i] >= 252 {
					totSize++
				}
			}
			totSize += rp.len[count-1]

			if totSize > maxlen {
				return 0, ErrBufferTooSmall
			}
			data[ptr] = (rp.toc & 0xFC) | 0x3
			ptr++
			data[ptr] = byte(count) | 0x80
			ptr++
		} else {
			totSize += count*rp.len[0] + 2
			if totSize > maxlen {
				return 0, ErrBufferTooSmall
			}
			data[ptr] = (rp.toc & 0xFC) | 0x3
			ptr++
			data[ptr] = byte(count)
			ptr++
		}

		if pad {
			padAmount = maxlen - totSize
			if padAmount > 0 {
				nb255s := (padAmount - 1) / 255
				data[offset+1] |= 0x40
				for i := 0; i < nb255s; i++ {
					data[ptr] = 255
					ptr++
				}
				data[ptr] = byte(padAmount - 255*nb255s - 1)
				ptr++
				totSize += padAmount
			}
		}

		if vbr {
			for i := 0; i < count-1; i++ {
				ptr += encodeSize(rp.len[i], data[ptr:])
			}
		}
	}

	if selfDelimited {
		sdlen := encodeSize(rp.len[count-1], data[ptr:])
		ptr += sdlen
	}

	// Copy the actual data
	for i := begin; i < count+begin; i++ {
		copy(data[ptr:], rp.frames[i][:rp.len[i]])
		ptr += rp.len[i]
	}

	if pad {
		// Fill padding with zeros
		for i := ptr; i < offset+maxlen; i++ {
			data[i] = 0
		}
	}

	return totSize, nil
}

// PadPacket pads an Opus packet to a larger size
func PadPacket(data []byte, offset, length, newLength int) error {
	if length < 1 {
		return ErrBadArgument
	}
	if length == newLength {
		return nil
	}
	if length > newLength {
		return ErrBadArgument
	}

	rp := NewRepacketizer()

	// Move payload to end for in-place padding
	copy(data[offset+newLength-length:], data[offset:offset+length])

	err := rp.catImpl(data, offset+newLength-length, length, false)
	if err != nil {
		return err
	}

	_, err = rp.outRangeImpl(0, rp.nbFrames, data, offset, newLength, false, true)
	return err
}

// UnpadPacket removes all padding from an Opus packet
func UnpadPacket(data []byte, offset, length int) (int, error) {
	if length < 1 {
		return 0, ErrBadArgument
	}

	rp := NewRepacketizer()
	err := rp.catImpl(data, offset, length, false)
	if err != nil {
		return 0, err
	}

	newLen, err := rp.outRangeImpl(0, rp.nbFrames, data, offset, length, false, false)
	if err != nil {
		return 0, err
	}

	if newLen <= 0 || newLen > length {
		return 0, ErrInvalidPacket
	}

	return newLen, nil
}

// PadMultistreamPacket pads a multistream Opus packet
func PadMultistreamPacket(data []byte, offset, length, newLength, nbStreams int) error {
	if length < 1 {
		return ErrBadArgument
	}
	if length == newLength {
		return nil
	}
	if length > newLength {
		return ErrBadArgument
	}

	amount := newLength - length
	var dummyToc byte
	var packetOffset int
	size := make([]int, 48)

	// Seek to last stream
	for s := 0; s < nbStreams-1; s++ {
		if length <= 0 {
			return ErrInvalidPacket
		}
		ret := opusPacketParseImpl(data, offset, length, true, &dummyToc,
			nil, 0, size, 0, &packetOffset, &packetOffset)
		if ret < 0 {
			return ErrInvalidPacket
		}
		offset += packetOffset
		length -= packetOffset
	}

	return PadPacket(data, offset, length, length+amount)
}

// UnpadMultistreamPacket removes padding from a multistream Opus packet
func UnpadMultistreamPacket(data []byte, offset, length, nbStreams int) (int, error) {
	if length < 1 {
		return 0, ErrBadArgument
	}

	dst := offset
	dstLen := 0
	var dummyToc byte
	var packetOffset int
	size := make([]int, 48)
	rp := NewRepacketizer()

	// Unpad all frames
	for s := 0; s < nbStreams; s++ {
		selfDelimited := 0
		if s != nbStreams-1 {
			selfDelimited = 1
		}

		if length <= 0 {
			return 0, ErrInvalidPacket
		}

		rp.Reset()
		ret := opusPacketParseImpl(data, offset, length, selfDelimited != 0, &dummyToc,
			nil, 0, size, 0, &packetOffset, &packetOffset)
		if ret < 0 {
			return 0, ErrInvalidPacket
		}

		err := rp.catImpl(data, offset, packetOffset, selfDelimited != 0)
		if err != nil {
			return 0, err
		}

		newLen, err := rp.outRangeImpl(0, rp.nbFrames, data, dst, length, selfDelimited != 0, false)
		if err != nil {
			return 0, err
		}
		dstLen += newLen
		dst += newLen
		offset += packetOffset
		length -= packetOffset
	}

	return dstLen, nil
}

// Helper functions (would be implemented elsewhere in the package)
func getNumSamplesPerFrame(data []byte, offset int, fs int) int {
	// Implementation would go here
	return 0
}

func getNumFrames(data []byte, offset, length int) int {
	// Implementation would go here
	return 0
}

func opusPacketParseImpl(data []byte, offset, length int, selfDelimited bool, toc *byte,
	frames [][]byte, framesSize int, len []int, lenSize int, packetOffset, packetOffsetOut *int) int {
	// Implementation would go here
	return 0
}

func encodeSize(size int, data []byte) int {
	// Implementation would go here
	return 0
}
