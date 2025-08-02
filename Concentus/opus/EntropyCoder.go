package opus

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	EC_WINDOW_SIZE = 32
	EC_UINT_BITS   = 8
	BITRES         = 3
	EC_SYM_BITS    = 8
	EC_CODE_BITS   = 32
	EC_SYM_MAX     = 0x000000FF
	EC_CODE_SHIFT  = 23
	EC_CODE_TOP    = 0x80000000
	EC_CODE_BOT    = 0x00800000
	EC_CODE_EXTRA  = 7
)

var correction = []int{35733, 38967, 42495, 46340, 50535, 55109, 60097, 65535}

type EntropyCoder struct {
	buf         []byte
	buf_ptr     int
	storage     int
	end_offs    int
	end_window  int64
	nend_bits   int
	nbits_total int
	offs        int
	rng         int64
	val         int64
	ext         int64
	rem         int
	error       int
}

func NewEntropyCoder() *EntropyCoder {
	obj := &EntropyCoder{}
	obj.Reset()
	return obj
}
func (ec *EntropyCoder) Reset() {
	ec.buf = nil
	ec.buf_ptr = 0
	ec.storage = 0
	ec.end_offs = 0
	ec.end_window = 0
	ec.nend_bits = 0
	ec.offs = 0
	ec.rng = 0
	ec.val = 0
	ec.ext = 0
	ec.rem = 0
	ec.error = 0
}

func (ec *EntropyCoder) Assign(other *EntropyCoder) {
	ec.buf = other.buf
	ec.buf_ptr = other.buf_ptr
	ec.storage = other.storage
	ec.end_offs = other.end_offs
	ec.end_window = other.end_window
	ec.nend_bits = other.nend_bits
	ec.nbits_total = other.nbits_total
	ec.offs = other.offs
	ec.rng = other.rng
	ec.val = other.val
	ec.ext = other.ext
	ec.rem = other.rem
	ec.error = other.error
}

func (ec *EntropyCoder) get_buffer() []byte {
	bufCopy := make([]byte, ec.storage)
	copy(bufCopy, ec.buf[ec.buf_ptr:ec.buf_ptr+ec.storage])
	return bufCopy
}

func (ec *EntropyCoder) write_buffer(data []byte, data_ptr int, target_offset int, size int) {

	json1, _ := json.Marshal(data)
	fmt.Printf("data:%+v\r\n", json1)
	copy(ec.buf[ec.buf_ptr+target_offset:], data[data_ptr:data_ptr+size])
}

func (ec *EntropyCoder) read_byte() int {
	if ec.offs < ec.storage {
		val := ec.buf[ec.buf_ptr+ec.offs]
		ec.offs++
		return int(val)
	}
	return 0
}

func (ec *EntropyCoder) read_byte_from_end() int {
	if ec.end_offs < ec.storage {
		ec.end_offs++
		return int(ec.buf[ec.buf_ptr+(ec.storage-ec.end_offs)])
	}
	return 0
}

func (ec *EntropyCoder) write_byte(_value uint) int {
	if ec.offs+ec.end_offs >= ec.storage {
		return -1
	}
	ec.buf[ec.buf_ptr+ec.offs] = byte(_value)
	//fmt.Printf("write_byte _value:%d\r\n", _value)
	ec.offs++
	return 0
}

func (ec *EntropyCoder) write_byte_at_end(_value uint) int {
	if ec.offs+ec.end_offs >= ec.storage {
		return -1
	}
	ec.end_offs++
	ec.buf[ec.buf_ptr+(ec.storage-ec.end_offs)] = byte(_value)
	return 0
}

func (ec *EntropyCoder) dec_normalize() {
	for ec.rng <= EC_CODE_BOT {
		ec.nbits_total += EC_SYM_BITS
		ec.rng <<= EC_SYM_BITS
		sym := ec.rem
		ec.rem = ec.read_byte()
		sym = (sym << EC_SYM_BITS) | ec.rem
		sym >>= (EC_SYM_BITS - EC_CODE_EXTRA)
		ec.val = ((ec.val << EC_SYM_BITS) + (EC_SYM_MAX & ^int64(sym))) & (EC_CODE_TOP - 1)
	}
}

func (ec *EntropyCoder) dec_init(_buf []byte, _buf_ptr int, _storage int) {
	ec.buf = _buf
	ec.buf_ptr = _buf_ptr
	ec.storage = _storage
	ec.end_offs = 0
	ec.end_window = 0
	ec.nend_bits = 0
	ec.nbits_total = EC_CODE_BITS + 1 - ((EC_CODE_BITS-EC_CODE_EXTRA)/EC_SYM_BITS)*EC_SYM_BITS
	ec.offs = 0
	ec.rng = 1 << EC_CODE_EXTRA
	ec.rem = ec.read_byte()
	ec.val = ec.rng - 1 - int64(ec.rem>>(EC_SYM_BITS-EC_CODE_EXTRA))
	ec.error = 0
	ec.dec_normalize()
}

func (ec *EntropyCoder) decode(_ft int64) int64 {
	ec.ext = ec.rng / _ft
	s := ec.val / ec.ext
	if s+1 < _ft {
		return _ft - (s + 1)
	}
	return 0
}

func (ec *EntropyCoder) decode_bin(_bits int) int64 {
	ec.ext = ec.rng >> _bits
	s := ec.val / ec.ext
	max := int64(1 << _bits)
	if s+1 < max {
		return max - (s + 1)
	}
	return 0
}

func (ec *EntropyCoder) dec_update(_fl int64, _fh int64, _ft int64) {
	s := ec.ext * (_ft - _fh)
	ec.val -= s
	if _fl > 0 {
		ec.rng = ec.ext * (_fh - _fl)
	} else {
		ec.rng -= s
	}
	ec.dec_normalize()
}

func (ec *EntropyCoder) dec_bit_logp(_logp int64) int {
	r := ec.rng
	d := ec.val
	s := r >> _logp
	ret := 0
	if d < s {
		ret = 1
	} else {
		ec.val = d - s
	}
	if ret != 0 {
		ec.rng = s
	} else {
		ec.rng = r - s
	}
	ec.dec_normalize()
	return ret
}

func (ec *EntropyCoder) dec_icdf(_icdf []int16, _ftb int) int {
	s := ec.rng
	d := ec.val
	r := s >> _ftb
	ret := -1
	var t int64
	for {
		ret++
		t = s
		s = r * int64(_icdf[ret])
		if d < s {
			continue
		}
		break
	}
	ec.val = d - s
	ec.rng = t - s
	ec.dec_normalize()
	return ret
}

func (ec *EntropyCoder) dec_icdf_offset(_icdf []int16, _icdf_offset int, _ftb int) int {
	sub := _icdf[_icdf_offset:]
	ret := ec.dec_icdf(sub, _ftb)
	return ret
}

func (ec *EntropyCoder) dec_uint(_ft int64) int64 {
	ft := _ft
	if ft <= 1 {
		panic("ft must be >1")
	}
	ft--
	ftb := EC_ILOG(int64(ft))
	if ftb > EC_UINT_BITS {
		ftb -= EC_UINT_BITS
		ft1 := (ft >> ftb) + 1
		s := ec.decode(ft1)
		ec.dec_update(s, s+1, ft1)
		t := (s << ftb) | int64(ec.dec_bits(ftb))
		if t <= ft {
			return t
		}
		ec.error = 1
		return ft
	} else {
		ft++
		s := ec.decode(ft)
		ec.dec_update(s, s+1, ft)
		return s
	}
}

func (ec *EntropyCoder) dec_bits(_bits int) int {
	window := ec.end_window
	available := ec.nend_bits
	for available < _bits {
		window |= int64(ec.read_byte_from_end()) << available
		available += EC_SYM_BITS
		if available > EC_WINDOW_SIZE-EC_SYM_BITS {
			break
		}
	}
	ret := int(window & ((1 << _bits) - 1))
	window >>= _bits
	available -= _bits
	ec.end_window = window
	ec.nend_bits = available
	ec.nbits_total += _bits
	return ret
}

func (ec *EntropyCoder) enc_carry_out(_c int) {
	fmt.Printf("\r\nenc_carry_out c:%d\r\n", _c)
	if _c != EC_SYM_MAX {
		carry := _c >> EC_SYM_BITS
		if ec.rem >= 0 {
			ec.error |= ec.write_byte(uint(ec.rem + carry))
		}
		if ec.ext > 0 {
			sym := (EC_SYM_MAX + carry) & EC_SYM_MAX
			for ec.ext > 0 {
				ec.error |= ec.write_byte(uint(sym))
				ec.ext--
			}
		}
		ec.rem = _c & EC_SYM_MAX
	} else {
		ec.ext++
	}
}

var i = 0

func (ec *EntropyCoder) enc_normalize() {
	/*If the range is too small, output some bits and rescale it.*/
	fmt.Printf(" enc_normalize ec.rng:%d ec.val:%d\r\n", ec.rng, ec.val)
	i++
	if i > 100 {
		os.Exit(0)
	}
	for ec.rng <= EC_CODE_BOT {
		ec.enc_carry_out(int(ec.val >> EC_CODE_SHIFT))
		/*Move the next-to-high-order symbol into the high-order position.*/
		ec.val = int64((ec.val << EC_SYM_BITS) & (EC_CODE_TOP - 1))
		ec.rng = int64(ec.rng << EC_SYM_BITS)
		ec.nbits_total += EC_SYM_BITS
		fmt.Printf("enc_normalize end ec.rng:%d\r\n", ec.rng)
	}

}

func (ec *EntropyCoder) enc_init(_buf []byte, buf_ptr int, _size int) {
	ec.buf = _buf
	ec.buf_ptr = buf_ptr
	ec.end_offs = 0
	ec.end_window = 0
	ec.nend_bits = 0
	ec.nbits_total = EC_CODE_BITS + 1
	ec.offs = 0
	ec.rng = EC_CODE_TOP
	ec.rem = -1
	ec.val = 0
	ec.ext = 0
	ec.storage = _size
	ec.error = 0
}

func (ec *EntropyCoder) encodeOld(_fl int64, _fh int64, _ft int64) {
	r := ec.rng / _ft
	if _fl > 0 {
		ec.val += ec.rng - r*(_ft-_fl)
		ec.rng = r * (_fh - _fl)
	} else {
		ec.rng -= r * (_ft - _fh)
	}
	fmt.Printf("encode\r\n")
	ec.enc_normalize()
}
func (ec *EntropyCoder) encode(_fl int64, _fh int64, _ft int64) {
	r := ec.rng / _ft
	if _fl > 0 {
		ec.val += (ec.rng - (r * (_ft - _fl)))
		ec.rng = (r * (_fh - _fl))
	} else {
		ec.rng = (ec.rng - (r * (_ft - _fh)))
	}
	//panic("eeee")

	fmt.Printf("encode ec.rng:%d _fl:%d _fh:%d _ft:%d\r\n", ec.rng, _fl, _fh, _ft)
	if ec.rng == 143798 {
		panic("eeee")
	}
	ec.enc_normalize()
}

func (ec *EntropyCoder) encode_bin(_fl int64, _fh int64, _bits int) {
	r := ec.rng >> _bits
	if _fl > 0 {
		ec.val += ec.rng - r*((1<<_bits)-_fl)
		ec.rng = r * (_fh - _fl)
	} else {
		ec.rng -= r * ((1 << _bits) - _fh)
	}
	fmt.Printf("encode_bin\r\n")
	ec.enc_normalize()
}

func (ec *EntropyCoder) enc_bit_logp(_val int, _logp int) {
	r := ec.rng
	l := ec.val
	s := r >> _logp
	r -= s
	if _val != 0 {
		ec.val = l + r
	}
	if _val != 0 {
		ec.rng = s
	} else {
		ec.rng = r
	}
	fmt.Printf("enc_bit_logp\r\n")
	ec.enc_normalize()
}

func (ec *EntropyCoder) enc_icdf(_s int, _icdf []int16, _ftb int) {
	r := ec.rng >> _ftb
	if _s > 0 {
		ec.val += ec.rng - r*int64(_icdf[_s-1])
		ec.rng = r * (int64(_icdf[_s-1]) - int64(_icdf[_s]))
	} else {
		ec.rng -= r * int64(_icdf[_s])
	}
	fmt.Printf("enc_icdf\r\n")
	ec.enc_normalize()
}

func (ec *EntropyCoder) enc_icdf_offset(_s int, _icdf []int16, icdf_ptr int, _ftb int) {
	r := (ec.rng >> _ftb)
	if _s > 0 {
		ec.val = ec.val + (ec.rng - (r * int64(_icdf[icdf_ptr+_s-1])))
		ec.rng = (r * int64(_icdf[icdf_ptr+_s-1]-_icdf[icdf_ptr+_s]))
	} else {
		ec.rng = int64(ec.rng - (r * int64(_icdf[icdf_ptr+_s])))
	}
	fmt.Printf("enc_icdf_offset\r\n")
	ec.enc_normalize()
}

func (ec *EntropyCoder) enc_uintOld(_fl int64, _ft int64) {
	if _ft <= 1 {
		panic("ft must be >1")
	}
	ft := _ft - 1
	ftb := EC_ILOG(int64(ft))
	if ftb > EC_UINT_BITS {
		ftb -= EC_UINT_BITS
		ft1 := (ft >> ftb) + 1
		fl := _fl >> ftb
		ec.encode(fl, fl+1, ft1)
		ec.enc_bits(_fl&((1<<ftb)-1), ftb)
	} else {
		ec.encode(_fl, _fl+1, ft+1)
	}
}
func (ec *EntropyCoder) enc_uint(_fl int64, _ft int64) {

	var ft int64
	var fl int64
	var ftb int
	/*In order to optimize EC_ILOG(), it is undefined for the value 0.*/
	OpusAssert(_ft > 1)
	_ft--
	ftb = EC_ILOG(_ft)
	if ftb > EC_UINT_BITS {
		ftb -= EC_UINT_BITS
		ft = ((_ft >> ftb) + 1)
		fl = (_fl >> ftb)
		ec.encode(fl, fl+1, ft)
		ec.enc_bits(_fl&int64((1<<ftb)-1), ftb)
	} else {
		ec.encode(_fl, _fl+1, _ft+1)
	}
}

func (ec *EntropyCoder) enc_bits(_fl int64, _bits int) {
	window := ec.end_window
	used := ec.nend_bits
	if used+_bits > EC_WINDOW_SIZE {
		for used >= EC_SYM_BITS {
			ec.error |= ec.write_byte_at_end(uint(window & EC_SYM_MAX))
			window >>= EC_SYM_BITS
			used -= EC_SYM_BITS
		}
	}
	window |= int64(_fl) << used
	used += _bits
	ec.end_window = window
	ec.nend_bits = used
	ec.nbits_total += _bits
}

func (ec *EntropyCoder) enc_patch_initial_bits(_val int64, _nbits int) {
	shift := EC_SYM_BITS - _nbits
	mask := int64(((1 << _nbits) - 1) << shift)
	if ec.offs > 0 {
		ec.buf[ec.buf_ptr] = (ec.buf[ec.buf_ptr] & ^byte(mask)) | byte(_val<<shift)
	} else if ec.rem >= 0 {
		ec.rem = int((int64(ec.rem) & ^mask) | (_val << shift))
	} else if ec.rng <= EC_CODE_TOP>>_nbits {
		ec.val = (ec.val & ^(mask << EC_CODE_SHIFT)) | (_val << (EC_CODE_SHIFT + shift))
	} else {
		ec.error = -1
	}
	fmt.Printf("enc_patch_initial_bits\r\n")
}

func (ec *EntropyCoder) enc_shrink(_size int) {
	if ec.offs+ec.end_offs > _size {
		panic("offs + end_offs > size")
	}
	copy(ec.buf[ec.buf_ptr+_size-ec.end_offs:], ec.buf[ec.buf_ptr+ec.storage-ec.end_offs:ec.buf_ptr+ec.storage])
	ec.storage = _size
}

func (ec *EntropyCoder) range_bytes() int {
	return ec.offs
}

func (ec *EntropyCoder) get_error() int {
	return ec.error
}

func (ec *EntropyCoder) tell() int {
	return ec.nbits_total - EC_ILOG(int64(ec.rng))
}

func (ec *EntropyCoder) tell_frac() int {
	// int nbits;
	//  int r;
	// int l;
	// long b;
	nbits := ec.nbits_total << BITRES
	l := EC_ILOG(ec.rng)
	r := int(ec.rng >> (l - 16))
	b := int((r >> 12) - 8)
	b1 := 0
	if r > correction[b] {
		b1 = 1
	}
	b = int(b + b1)
	l = ((l << 3) + b)
	return nbits - l
}

func (ec *EntropyCoder) enc_done() {
	fmt.Printf("\r\n\r\n\r\n\r\n\r\nenc_done\r\n\r\n\r\n")
	l := EC_CODE_BITS - EC_ILOG(int64(ec.rng))
	msk := (EC_CODE_TOP - 1) >> l
	end := (ec.val + int64(msk)) & ^int64(msk)
	if (end | int64(msk)) >= ec.val+ec.rng {
		l++
		msk >>= 1
		end = (ec.val + int64(msk)) & ^int64(msk)
	}
	for l > 0 {
		ec.enc_carry_out(int(end >> EC_CODE_SHIFT))
		end = (end << EC_SYM_BITS) & (EC_CODE_TOP - 1)
		l -= EC_SYM_BITS
	}
	if ec.rem >= 0 || ec.ext > 0 {
		ec.enc_carry_out(0)
	}
	window := ec.end_window
	used := ec.nend_bits
	for used >= EC_SYM_BITS {
		ec.error |= ec.write_byte_at_end(uint(window & EC_SYM_MAX))
		window >>= EC_SYM_BITS
		used -= EC_SYM_BITS
	}
	if ec.error == 0 {
		for i := ec.offs; i < ec.storage-ec.end_offs; i++ {
			ec.buf[ec.buf_ptr+i] = 0
		}
		if used > 0 {
			if ec.end_offs >= ec.storage {
				ec.error = -1
			} else {
				remaining := ec.storage - ec.offs - ec.end_offs
				if remaining <= 0 && used > -l {
					window &= (1 << uint(-l)) - 1
					ec.error = -1
				}
				idx := ec.buf_ptr + ec.storage - ec.end_offs - 1
				ec.buf[idx] |= byte(window)
			}
		}
	}
}
