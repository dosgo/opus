package opus
const (
	EC_WINDOW_SIZE  = 32
	EC_UINT_BITS    = 8
	BITRES          = 3
	EC_SYM_BITS     = 8
	EC_CODE_BITS    = 32
	EC_SYM_MAX      = 0x000000FF
	EC_CODE_SHIFT   = 23
	EC_CODE_TOP     = 0x80000000
	EC_CODE_BOT     = 0x00800000
	EC_CODE_EXTRA   = 7
)

var correction = []uint32{35733, 38967, 42495, 46340, 50535, 55109, 60097, 65535}

type EntropyCoder struct {
	buf        []byte
	buf_ptr    int
	storage    int
	end_offs   int
	end_window uint64
	nend_bits  int
	nbits_total int
	offs       int
	rng        uint32
	val        uint32
	ext        uint32
	rem        int
	error      int
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

func (ec *EntropyCoder) write_byte(_value uint32) int {
	if ec.offs+ec.end_offs >= ec.storage {
		return -1
	}
	ec.buf[ec.buf_ptr+ec.offs] = byte(_value)
	ec.offs++
	return 0
}

func (ec *EntropyCoder) write_byte_at_end(_value uint32) int {
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
		ec.val = ((ec.val << EC_SYM_BITS) + (EC_SYM_MAX & ^uint32(sym))) & (EC_CODE_TOP - 1)
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
	ec.val = ec.rng - 1 - uint32(ec.rem>>(EC_SYM_BITS-EC_CODE_EXTRA))
	ec.error = 0
	ec.dec_normalize()
}

func (ec *EntropyCoder) decode(_ft uint32) uint32 {
	ec.ext = ec.rng / _ft
	s := ec.val / ec.ext
	if s+1 < _ft {
		return _ft - (s + 1)
	}
	return 0
}

func (ec *EntropyCoder) decode_bin(_bits int) uint32 {
	ec.ext = ec.rng >> _bits
	s := ec.val / ec.ext
	max := uint32(1 << _bits)
	if s+1 < max {
		return max - (s + 1)
	}
	return 0
}

func (ec *EntropyCoder) dec_update(_fl uint32, _fh uint32, _ft uint32) {
	s := ec.ext * (_ft - _fh)
	ec.val -= s
	if _fl > 0 {
		ec.rng = ec.ext * (_fh - _fl)
	} else {
		ec.rng -= s
	}
	ec.dec_normalize()
}

func (ec *EntropyCoder) dec_bit_logp(_logp uint32) int {
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
	var t uint32
	for {
		ret++
		t = s
		s = r * uint32(_icdf[ret])
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

func (ec *EntropyCoder) dec_uint(_ft uint32) uint32 {
	ft := _ft
	if ft <= 1 {
		panic("ft must be >1")
	}
	ft--
	ftb := EC_ILOG(ft)
	if ftb > EC_UINT_BITS {
		ftb -= EC_UINT_BITS
		ft1 := (ft >> ftb) + 1
		s := ec.decode(ft1)
		ec.dec_update(s, s+1, ft1)
		t := (s << ftb) | uint32(ec.dec_bits(ftb))
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
		window |= uint64(ec.read_byte_from_end()) << available
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
	if _c != EC_SYM_MAX {
		carry := _c >> EC_SYM_BITS
		if ec.rem >= 0 {
			ec.error |= ec.write_byte(uint32(ec.rem + carry))
		}
		if ec.ext > 0 {
			sym := (EC_SYM_MAX + carry) & EC_SYM_MAX
			for ec.ext > 0 {
				ec.error |= ec.write_byte(uint32(sym))
				ec.ext--
			}
		}
		ec.rem = _c & EC_SYM_MAX
	} else {
		ec.ext++
	}
}

func (ec *EntropyCoder) enc_normalize() {
	for ec.rng <= EC_CODE_BOT {
		ec.enc_carry_out(int(ec.val >> EC_CODE_SHIFT))
		ec.val = (ec.val << EC_SYM_BITS) & (EC_CODE_TOP - 1)
		ec.rng <<= EC_SYM_BITS
		ec.nbits_total += EC_SYM_BITS
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

func (ec *EntropyCoder) encode(_fl uint32, _fh uint32, _ft uint32) {
	r := ec.rng / _ft
	if _fl > 0 {
		ec.val += ec.rng - r*(_ft-_fl)
		ec.rng = r * (_fh - _fl)
	} else {
		ec.rng -= r * (_ft - _fh)
	}
	ec.enc_normalize()
}

func (ec *EntropyCoder) encode_bin(_fl uint32, _fh uint32, _bits int) {
	r := ec.rng >> _bits
	if _fl > 0 {
		ec.val += ec.rng - r*((1<<_bits)-_fl)
		ec.rng = r * (_fh - _fl)
	} else {
		ec.rng -= r * ((1 << _bits) - _fh)
	}
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
	ec.enc_normalize()
}

func (ec *EntropyCoder) enc_icdf(_s int, _icdf []int16, _ftb int) {
	r := ec.rng >> _ftb
	if _s > 0 {
		ec.val += ec.rng - r*uint32(_icdf[_s-1])
		ec.rng = r*uint32(_icdf[_s-1]) - uint32(_icdf[_s])
	} else {
		ec.rng -= r * uint32(_icdf[_s])
	}
	ec.enc_normalize()
}

func (ec *EntropyCoder) enc_icdf_offset(_s int, _icdf []int16, icdf_ptr int, _ftb int) {
	sub := _icdf[icdf_ptr:]
	if _s > 0 {
		ec.enc_icdf(_s, sub, _ftb)
	} else {
		r := ec.rng >> _ftb
		ec.rng -= r * uint32(sub[_s])
		ec.enc_normalize()
	}
}

func (ec *EntropyCoder) enc_uint(_fl uint32, _ft uint32) {
	if _ft <= 1 {
		panic("ft must be >1")
	}
	ft := _ft - 1
	ftb := EC_ILOG(ft)
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

func (ec *EntropyCoder) enc_bits(_fl uint32, _bits int) {
	window := ec.end_window
	used := ec.nend_bits
	if used+_bits > EC_WINDOW_SIZE {
		for used >= EC_SYM_BITS {
			ec.error |= ec.write_byte_at_end(uint32(window & EC_SYM_MAX))
			window >>= EC_SYM_BITS
			used -= EC_SYM_BITS
		}
	}
	window |= uint64(_fl) << used
	used += _bits
	ec.end_window = window
	ec.nend_bits = used
	ec.nbits_total += _bits
}

func (ec *EntropyCoder) enc_patch_initial_bits(_val uint32, _nbits int) {
	shift := EC_SYM_BITS - _nbits
	mask := uint32(((1 << _nbits) - 1) << shift)
	if ec.offs > 0 {
		ec.buf[ec.buf_ptr] = (ec.buf[ec.buf_ptr] & ^byte(mask)) | byte(_val<<shift)
	} else if ec.rem >= 0 {
		ec.rem = int((uint32(ec.rem) & ^mask) | (_val << shift)
	} else if ec.rng <= EC_CODE_TOP>>_nbits {
		ec.val = (ec.val & ^(mask << EC_CODE_SHIFT)) | (_val << (EC_CODE_SHIFT + shift))
	} else {
		ec.error = -1
	}
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
	return ec.nbits_total - EC_ILOG(ec.rng)
}

func (ec *EntropyCoder) tell_frac() int {
	nbits := ec.nbits_total << BITRES
	l := EC_ILOG(ec.rng)
	var r uint32
	if l < 16 {
		r = ec.rng << uint(16-l)
	} else {
		r = ec.rng >> uint(l-16)
	}
	b := (r >> 12) - 8
	if b < 0 {
		b = 0
	} else if b > 7 {
		b = 7
	}
	if r > correction[b] {
		b++
	}
	l = (l << 3) + int(b)
	return nbits - l
}

func (ec *EntropyCoder) enc_done() {
	l := EC_CODE_BITS - EC_ILOG(ec.rng)
	msk := (EC_CODE_TOP - 1) >> l
	end := (ec.val + uint32(msk)) & ^uint32(msk)
	if (end | uint32(msk)) >= ec.val+ec.rng {
		l++
		msk >>= 1
		end = (ec.val + uint32(msk)) & ^uint32(msk)
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
		ec.error |= ec.write_byte_at_end(uint32(window & EC_SYM_MAX))
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

func EC_ILOG(x uint32) int {
	ret := 0
	for x > 0 {
		ret++
		x >>= 1
	}
	return ret
}

func EC_MINI(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

func OpusAssert(cond bool) {
	if !cond {
		panic("assertion failed")
	}
}