package entropy

import (
	"errors"
	"math/bits"
)

// Constants for the entropy coder configuration
const (
	ecWindowSize = 32
	ecUintBits   = 8
	bitRes       = 3
	ecSymBits    = 8
	ecCodeBits   = 32
	ecSymMax     = 0x000000FF
	ecCodeShift  = 0x00000017
	ecCodeTop    = 0x80000000
	ecCodeBot    = 0x00800000
	ecCodeExtra  = 0x00000007
)

// EntropyCoder represents a range decoder/encoder for entropy coding
type EntropyCoder struct {
	buf        []byte
	bufPtr     int
	storage    int
	endOffs    int
	endWindow  uint32
	nendBits   int
	nbitsTotal int
	offs       int
	rng        uint32
	val        uint32
	ext        uint32
	rem        int
	error      int
}

// NewEntropyCoder creates a new initialized EntropyCoder
func NewEntropyCoder() *EntropyCoder {
	return &EntropyCoder{}
}

// Reset resets the coder to initial state
func (ec *EntropyCoder) Reset() {
	ec.buf = nil
	ec.bufPtr = 0
	ec.storage = 0
	ec.endOffs = 0
	ec.endWindow = 0
	ec.nendBits = 0
	ec.offs = 0
	ec.rng = 0
	ec.val = 0
	ec.ext = 0
	ec.rem = -1
	ec.error = 0
}

// Assign copies state from another coder
func (ec *EntropyCoder) Assign(other *EntropyCoder) {
	ec.buf = other.buf
	ec.bufPtr = other.bufPtr
	ec.storage = other.storage
	ec.endOffs = other.endOffs
	ec.endWindow = other.endWindow
	ec.nendBits = other.nendBits
	ec.nbitsTotal = other.nbitsTotal
	ec.offs = other.offs
	ec.rng = other.rng
	ec.val = other.val
	ec.ext = other.ext
	ec.rem = other.rem
	ec.error = other.error
}

// GetBuffer returns a copy of the internal buffer
func (ec *EntropyCoder) GetBuffer() []byte {
	buf := make([]byte, ec.storage)
	copy(buf, ec.buf[ec.bufPtr:ec.bufPtr+ec.storage])
	return buf
}

// WriteBuffer writes data to the internal buffer
func (ec *EntropyCoder) WriteBuffer(data []byte, dataPtr, targetOffset, size int) {
	copy(ec.buf[ec.bufPtr+targetOffset:], data[dataPtr:dataPtr+size])
}

// readByte reads a byte from the current position
func (ec *EntropyCoder) readByte() uint32 {
	if ec.offs < ec.storage {
		b := ec.buf[ec.bufPtr+ec.offs]
		ec.offs++
		return uint32(b)
	}
	return 0
}

// readByteFromEnd reads a byte from the end of the buffer
func (ec *EntropyCoder) readByteFromEnd() uint32 {
	if ec.endOffs < ec.storage {
		ec.endOffs++
		return uint32(ec.buf[ec.bufPtr+(ec.storage-ec.endOffs)])
	}
	return 0
}

// writeByte writes a byte to the current position
func (ec *EntropyCoder) writeByte(value uint32) error {
	if ec.offs+ec.endOffs >= ec.storage {
		return errors.New("buffer overflow")
	}
	ec.buf[ec.bufPtr+ec.offs] = byte(value & 0xFF)
	ec.offs++
	return nil
}

// writeByteAtEnd writes a byte to the end of the buffer
func (ec *EntropyCoder) writeByteAtEnd(value uint32) error {
	if ec.offs+ec.endOffs >= ec.storage {
		return errors.New("buffer overflow")
	}
	ec.endOffs++
	ec.buf[ec.bufPtr+(ec.storage-ec.endOffs)] = byte(value & 0xFF)
	return nil
}

// decNormalize normalizes the decoder state
func (ec *EntropyCoder) decNormalize() {
	for ec.rng <= ecCodeBot {
		ec.nbitsTotal += ecSymBits
		ec.rng <<= ecSymBits

		// Use remaining bits from last symbol
		sym := ec.rem
		// Read next value
		ec.rem = int(ec.readByte())
		// Take needed bits from new symbol
		sym = (sym<<ecSymBits | ec.rem) >> (ecSymBits - ecCodeExtra)
		// Subtract from val, capped to be less than EC_CODE_TOP
		ec.val = ((ec.val << ecSymBits) + (ecSymMax & ^uint32(sym))) & (ecCodeTop - 1)
	}
}

// DecInit initializes the decoder
func (ec *EntropyCoder) DecInit(buf []byte, bufPtr, storage int) {
	ec.buf = buf
	ec.bufPtr = bufPtr
	ec.storage = storage
	ec.endOffs = 0
	ec.endWindow = 0
	ec.nendBits = 0
	ec.nbitsTotal = ecCodeBits + 1 - ((ecCodeBits-ecCodeExtra)/ecSymBits)*ecSymBits
	ec.offs = 0
	ec.rng = 1 << ecCodeExtra
	ec.rem = int(ec.readByte())
	ec.val = ec.rng - 1 - uint32(ec.rem>>(ecSymBits-ecCodeExtra))
	ec.error = 0
	ec.decNormalize()
}

// Decode decodes a symbol with given frequency total
func (ec *EntropyCoder) Decode(ft uint32) uint32 {
	ec.ext = ec.rng / ft
	s := ec.val / ec.ext
	if s+1 < ft {
		return ft - (s + 1)
	}
	return 0
}

// DecodeBin decodes a binary symbol with given bits
func (ec *EntropyCoder) DecodeBin(bits int) uint32 {
	ec.ext = ec.rng >> bits
	s := ec.val / ec.ext
	return (1 << bits) - 1 - s
}

// DecUpdate updates the decoder state after decoding
func (ec *EntropyCoder) DecUpdate(fl, fh, ft uint32) {
	s := ec.ext * (ft - fh)
	ec.val -= s
	if fl > 0 {
		ec.rng = ec.ext * (fh - fl)
	} else {
		ec.rng -= s
	}
	ec.decNormalize()
}

// DecBitLogp decodes a bit with given log probability
func (ec *EntropyCoder) DecBitLogp(logp uint32) int {
	r := ec.rng
	d := ec.val
	s := r >> logp
	if d < s {
		ec.rng = s
		return 1
	}
	ec.val = d - s
	ec.rng = r - s
	ec.decNormalize()
	return 0
}

// DecICDF decodes using an inverse cumulative distribution function
func (ec *EntropyCoder) DecICDF(icdf []uint16, ftb int) int {
	s := ec.rng
	d := ec.val
	r := s >> ftb
	ret := -1
	t := s
	for {
		ret++
		s = uint32(r) * uint32(icdf[ret])
		if d >= s {
			break
		}
	}
	ec.val = d - s
	ec.rng = t - s
	ec.decNormalize()
	return ret
}

// DecUint decodes an unsigned integer
func (ec *EntropyCoder) DecUint(ft uint32) uint32 {
	if ft <= 1 {
		panic("ft must be > 1")
	}
	ft--
	ftb := ecILog(ft)
	if ftb > ecUintBits {
		ftb -= ecUintBits
		ft1 := (ft >> ftb) + 1
		s := ec.Decode(ft1)
		ec.DecUpdate(s, s+1, ft1)
		t := (s << ftb) | uint32(ec.DecBits(ftb))
		if t <= ft {
			return t
		}
		ec.error = 1
		return ft
	}
	ft++
	s := ec.Decode(ft)
	ec.DecUpdate(s, s+1, ft)
	return s
}

// DecBits decodes raw bits
func (ec *EntropyCoder) DecBits(bits int) uint32 {
	window := ec.endWindow
	available := ec.nendBits
	if available < bits {
		for available <= ecWindowSize-ecSymBits {
			window |= ec.readByteFromEnd() << available
			available += ecSymBits
		}
	}
	ret := window & ((1 << bits) - 1)
	window >>= bits
	available -= bits
	ec.endWindow = window
	ec.nendBits = available
	ec.nbitsTotal += bits
	return ret
}

// encCarryOut handles carry propagation in the encoder
func (ec *EntropyCoder) encCarryOut(c int) {
	if c != int(ecSymMax) {
		carry := c >> ecSymBits
		if ec.rem >= 0 {
			if err := ec.writeByte(uint32(ec.rem + carry)); err != nil {
				ec.error = -1
			}
		}
		if ec.ext > 0 {
			sym := uint32((ecSymMax + carry) & ecSymMax)
			for ec.ext > 0 {
				if err := ec.writeByte(sym); err != nil {
					ec.error = -1
				}
				ec.ext--
			}
		}
		ec.rem = c & int(ecSymMax)
	} else {
		ec.ext++
	}
}

// encNormalize normalizes the encoder state
func (ec *EntropyCoder) encNormalize() {
	for ec.rng <= ecCodeBot {
		ec.encCarryOut(int(ec.val >> ecCodeShift))
		ec.val = (ec.val << ecSymBits) & (ecCodeTop - 1)
		ec.rng <<= ecSymBits
		ec.nbitsTotal += ecSymBits
	}
}

// EncInit initializes the encoder
func (ec *EntropyCoder) EncInit(buf []byte, bufPtr, size int) {
	ec.buf = buf
	ec.bufPtr = bufPtr
	ec.endOffs = 0
	ec.endWindow = 0
	ec.nendBits = 0
	ec.nbitsTotal = ecCodeBits + 1
	ec.offs = 0
	ec.rng = ecCodeTop
	ec.rem = -1
	ec.val = 0
	ec.ext = 0
	ec.storage = size
	ec.error = 0
}

// Encode encodes a symbol with given frequency range
func (ec *EntropyCoder) Encode(fl, fh, ft uint32) {
	r := ec.rng / ft
	if fl > 0 {
		ec.val += ec.rng - r*(ft-fl)
		ec.rng = r * (fh - fl)
	} else {
		ec.rng -= r * (ft - fh)
	}
	ec.encNormalize()
}

// EncodeBin encodes a binary symbol with given bits
func (ec *EntropyCoder) EncodeBin(fl, fh uint32, bits int) {
	r := ec.rng >> bits
	if fl > 0 {
		ec.val += ec.rng - r*((1<<bits)-fl)
		ec.rng = r * (fh - fl)
	} else {
		ec.rng -= r * ((1 << bits) - fh)
	}
	ec.encNormalize()
}

// EncBitLogp encodes a bit with given log probability
func (ec *EntropyCoder) EncBitLogp(val int, logp int) {
	r := ec.rng
	l := ec.val
	s := r >> logp
	r -= s
	if val != 0 {
		ec.val = l + r
		ec.rng = s
	} else {
		ec.rng = r
	}
	ec.encNormalize()
}

// EncICDF encodes using an inverse cumulative distribution function
func (ec *EntropyCoder) EncICDF(s int, icdf []uint16, ftb int) {
	r := ec.rng >> ftb
	if s > 0 {
		ec.val += ec.rng - uint32(r)*uint32(icdf[s-1])
		ec.rng = uint32(r) * uint32(icdf[s-1]-icdf[s])
	} else {
		ec.rng -= uint32(r) * uint32(icdf[s])
	}
	ec.encNormalize()
}

// EncUint encodes an unsigned integer
func (ec *EntropyCoder) EncUint(fl, ft uint32) {
	if ft <= 1 {
		panic("ft must be > 1")
	}
	ft--
	ftb := ecILog(ft)
	if ftb > ecUintBits {
		ftb -= ecUintBits
		ft1 := (ft >> ftb) + 1
		fl1 := fl >> ftb
		ec.Encode(fl1, fl1+1, ft1)
		ec.EncBits(fl&((1<<ftb)-1), ftb)
	} else {
		ft++
		ec.Encode(fl, fl+1, ft)
	}
}

// EncBits encodes raw bits
func (ec *EntropyCoder) EncBits(fl uint32, bits int) {
	window := ec.endWindow
	used := ec.nendBits
	for used+bits > ecWindowSize {
		if err := ec.writeByteAtEnd(window & ecSymMax); err != nil {
			ec.error = -1
		}
		window >>= ecSymBits
		used -= ecSymBits
	}
	window |= fl << used
	used += bits
	ec.endWindow = window
	ec.nendBits = used
	ec.nbitsTotal += bits
}

// RangeBytes returns the number of bytes used in the range coder
func (ec *EntropyCoder) RangeBytes() int {
	return ec.offs
}

// GetError returns the error state
func (ec *EntropyCoder) GetError() int {
	return ec.error
}

// Tell returns the number of bits used so far
func (ec *EntropyCoder) Tell() int {
	return ec.nbitsTotal - ecILog(ec.rng)
}

var correction = [8]uint32{35733, 38967, 42495, 46340, 50535, 55109, 60097, 65535}

// TellFrac returns the fractional number of bits used so far
func (ec *EntropyCoder) TellFrac() int {
	nbits := ec.nbitsTotal << bitRes
	l := ecILog(ec.rng)
	r := ec.rng >> (l - 16)
	b := (r >> 12) - 8
	if r > correction[b] {
		b++
	}
	l = (l << 3) + int(b)
	return nbits - l
}

// EncDone finalizes the encoding
func (ec *EntropyCoder) EncDone() {
	// Calculate minimum bits needed for correct decoding
	l := ecCodeBits - ecILog(ec.rng)
	msk := (ecCodeTop - 1) >> l
	end := (ec.val + uint32(msk)) & ^uint32(msk)

	if (end | uint32(msk)) >= ec.val+ec.rng {
		l++
		msk >>= 1
		end = (ec.val + uint32(msk)) & ^uint32(msk)
	}

	for l > 0 {
		ec.encCarryOut(int(end >> ecCodeShift))
		end = (end << ecSymBits) & (ecCodeTop - 1)
		l -= ecSymBits
	}

	// Flush any buffered bytes
	if ec.rem >= 0 || ec.ext > 0 {
		ec.encCarryOut(0)
	}

	// Flush any extra bits
	window := ec.endWindow
	used := ec.nendBits

	for used >= ecSymBits {
		if err := ec.writeByteAtEnd(window & ecSymMax); err != nil {
			ec.error = -1
		}
		window >>= ecSymBits
		used -= ecSymBits
	}

	// Handle remaining bits
	if ec.error == 0 && used > 0 {
		if ec.endOffs >= ec.storage {
			ec.error = -1
		} else {
			// Don't corrupt range coder data if we've busted
			if ec.offs+ec.endOffs >= ec.storage && -l < used {
				window &= (1 << uint(-l)) - 1
				ec.error = -1
			}
			z := ec.bufPtr + ec.storage - ec.endOffs - 1
			ec.buf[z] |= byte(window)
		}
	}
}

// ecILog computes integer logarithm (floor(log2(x)))
func ecILog(x uint32) int {
	return 31 - bits.LeadingZeros32(x)
}
