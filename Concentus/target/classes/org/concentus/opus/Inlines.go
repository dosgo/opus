package opus

import (
	"errors"
	"math"
)

// Inlines contains various utility functions for audio processing
type Inlines struct{}

// OpusAssert checks a condition and panics if it's false
func (i *Inlines) OpusAssert(condition bool) {
	if !condition {
		panic(errors.New("assertion failed"))
	}
}

// OpusAssertWithMessage checks a condition and panics with a message if false
func (i *Inlines) OpusAssertWithMessage(condition bool, message string) {
	if !condition {
		panic(errors.New(message))
	}
}

// CapToUInt32 caps a value to 32-bit unsigned integer range
func (i *Inlines) CapToUInt32(val int64) uint32 {
	return uint32(val & 0xFFFFFFFF)
}

// CELT-SPECIFIC INLINES

// MULT16_16SU multiplies a 16-bit signed value by a 16-bit unsigned value
func (i *Inlines) MULT16_16SU(a, b int) int {
	return int(int16(a)) * int(uint16(b))
}

// MULT16_32_Q16 multiplies 16x32 with 16-bit shift right
func (i *Inlines) MULT16_32_Q16(a int16, b int) int {
	return i.ADD32(
		i.MULT16_16(int(a), i.SHR(b, 16)),
		i.SHR(i.MULT16_16SU(int(a), b&0x0000FFFF), 16),
	)
}

// MULT16_32_P16 multiplies 16x32 with rounding-to-nearest 16-bit shift right
func (i *Inlines) MULT16_32_P16(a int16, b int) int {
	return i.ADD32(
		i.MULT16_16(int(a), i.SHR(b, 16)),
		i.PSHR(i.MULT16_16SU(int(a), b&0x0000FFFF), 16),
	)
}

// MULT16_32_Q15 multiplies 16x32 with 15-bit shift right
func (i *Inlines) MULT16_32_Q15(a int16, b int) int {
	return ((a * (b >> 16)) << 1) + ((a * (b & 0xFFFF)) >> 15)
}

// MULT32_32_Q31 multiplies 32x32 with 31-bit shift right
func (i *Inlines) MULT32_32_Q31(a, b int) int {
	return i.ADD32(
		i.ADD32(
			i.SHL(i.MULT16_16(i.SHR(a, 16), i.SHR(b, 16)), 1),
			i.SHR(i.MULT16_16SU(i.SHR(a, 16), b&0x0000FFFF), 15),
		),
		i.SHR(i.MULT16_16SU(i.SHR(b, 16), a&0x0000FFFF), 15),
	)
}

// QCONST16 converts float constant to 16-bit Q value
func (i *Inlines) QCONST16(x float32, bits int) int16 {
	return int16(math.Round(float64(x) * float64(1<<bits)))
}

// QCONST32 converts float constant to 32-bit Q value
func (i *Inlines) QCONST32(x float32, bits int) int {
	return int(math.Round(float64(x) * float64(1<<bits)))
}

// NEG16 negates a 16-bit value
func (i *Inlines) NEG16(x int16) int16 {
	return -x
}

// NEG32 negates a 32-bit value
func (i *Inlines) NEG32(x int) int {
	return -x
}

// EXTRACT16 extracts 16 bits from 32-bit value
func (i *Inlines) EXTRACT16(x int) int16 {
	return int16(x)
}

// EXTEND32 extends 16-bit to 32-bit
func (i *Inlines) EXTEND32(x int16) int {
	return int(x)
}

// SHR16 arithmetic shift-right of 16-bit value
func (i *Inlines) SHR16(a int16, shift int) int16 {
	return a >> shift
}

// SHL16 arithmetic shift-left of 16-bit value
func (i *Inlines) SHL16(a int16, shift int) int16 {
	return int16(uint16(a) << shift)
}

// SHR32 arithmetic shift-right of 32-bit value
func (i *Inlines) SHR32(a, shift int) int {
	return a >> shift
}

// SHL32 arithmetic shift-left of 32-bit value
func (i *Inlines) SHL32(a, shift int) int {
	return int(uint32(a) << shift)
}

// PSHR32 arithmetic shift right with rounding-to-nearest
func (i *Inlines) PSHR32(a, shift int) int {
	return i.SHR32(a+((1<<shift)>>1), shift)
}

// VSHR32 variable shift right (can be negative)
func (i *Inlines) VSHR32(a, shift int) int {
	if shift > 0 {
		return i.SHR32(a, shift)
	}
	return i.SHL32(a, -shift)
}

// private helper functions
func (i *Inlines) SHR(a, shift int) int {
	return a >> shift
}

func (i *Inlines) SHL(a, shift int) int {
	return i.SHL32(a, shift)
}

func (i *Inlines) PSHR(a, shift int) int {
	return i.SHR(a+((1<<shift)>>1), shift)
}

// SATURATE clamps value between -a and a
func (i *Inlines) SATURATE(x, a int) int {
	if x > a {
		return a
	}
	if x < -a {
		return -a
	}
	return x
}

// SATURATE16 clamps 16-bit value
func (i *Inlines) SATURATE16(x int) int16 {
	if x > 32767 {
		return 32767
	}
	if x < -32768 {
		return -32768
	}
	return int16(x)
}

// ROUND16 rounds with shift
func (i *Inlines) ROUND16(x int16, a int16) int16 {
	return i.EXTRACT16(i.PSHR32(int(x), int(a)))
}

// HALF16 divides by two
func (i *Inlines) HALF16(x int16) int16 {
	return i.SHR16(x, 1)
}

// ADD16 adds two 16-bit values
func (i *Inlines) ADD16(a, b int16) int16 {
	return a + b
}

// SUB16 subtracts two 16-bit values
func (i *Inlines) SUB16(a, b int16) int16 {
	return a - b
}

// ADD32 adds two 32-bit values
func (i *Inlines) ADD32(a, b int) int {
	return a + b
}

// SUB32 subtracts two 32-bit values
func (i *Inlines) SUB32(a, b int) int {
	return a - b
}

// MULT16_16_16 multiplies two 16-bit values with 16-bit result
func (i *Inlines) MULT16_16_16(a, b int16) int16 {
	return a * b
}

// MULT16_16 multiplies two 16-bit values with 32-bit result
func (i *Inlines) MULT16_16(a, b int) int {
	return a * b
}

// MAC16_16 multiply-add operation
func (i *Inlines) MAC16_16(c int, a, b int16) int {
	return c + int(a)*int(b)
}

// MAC16_32_Q15 multiply-accumulate with Q15 shift
func (i *Inlines) MAC16_32_Q15(c int, a int16, b int16) int {
	return i.ADD32(c, i.ADD32(
		i.MULT16_16(int(a), i.SHR(int(b), 15)),
		i.SHR(i.MULT16_16(int(a), int(b&0x00007FFF)), 15),
	))
}

// DIV32_16 divides 32-bit by 16-bit
func (i *Inlines) DIV32_16(a int, b int16) int16 {
	return int16(a / int(b))
}

// DIV32 divides two 32-bit values
func (i *Inlines) DIV32(a, b int) int {
	return a / b
}

// MIN/MAX functions
func (i *Inlines) MIN16(a, b int16) int16 {
	if a < b {
		return a
	}
	return b
}

func (i *Inlines) MAX16(a, b int16) int16 {
	if a > b {
		return a
	}
	return b
}

func (i *Inlines) MIN(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (i *Inlines) MAX(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (i *Inlines) MIN32(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (i *Inlines) MAX32(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ABS functions
func (i *Inlines) ABS16(x int16) int16 {
	if x < 0 {
		return -x
	}
	return x
}

func (i *Inlines) ABS32(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// celt_udiv unsigned division with assert
func (i *Inlines) celt_udiv(n, d int) int {
	i.OpusAssert(d > 0)
	return n / d
}

// celt_ilog2 integer log2
func (i *Inlines) celt_ilog2(x int) int {
	i.OpusAssert(x > 0, "celt_ilog2() only defined for strictly positive numbers")
	return i.EC_ILOG(int64(x)) - 1
}

// celt_zlog2 integer log2 with zero handling
func (i *Inlines) celt_zlog2(x int) int {
	if x <= 0 {
		return 0
	}
	return i.celt_ilog2(x)
}

// celt_maxabs16 finds maximum absolute value in 16-bit array
func (i *Inlines) celt_maxabs16(x []int, x_ptr, len int) int {
	maxval := 0
	minval := 0
	for i := x_ptr; i < len+x_ptr; i++ {
		maxval = i.MAX32(maxval, x[i])
		minval = i.MIN32(minval, x[i])
	}
	return i.MAX32(i.EXTEND32(maxval), -i.EXTEND32(minval))
}

// EC_ILOG computes integer log2
func (i *Inlines) EC_ILOG(x int64) int {
	if x == 0 {
		return 1
	}
	x |= x >> 1
	x |= x >> 2
	x |= x >> 4
	x |= x >> 8
	x |= x >> 16
	y := x - ((x >> 1) & 0x55555555)
	y = ((y >> 2) & 0x33333333) + (y & 0x33333333)
	y = ((y >> 4) + y) & 0x0F0F0F0F
	y += y >> 8
	y += y >> 16
	return int(y & 0x3F)
}
