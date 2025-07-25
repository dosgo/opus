package opus
var CELT_PVQ_U_ROW = [...]int{
    0, 176, 351, 525, 698, 870, 1041, 1131, 1178, 1207, 1226, 1240, 1248, 1254, 1257,
}

func CELT_PVQ_U(_n, _k int) uint64 {
    return CELT_PVQ_U_DATA[CELT_PVQ_U_ROW[IMIN(_n, _k)] + IMAX(_n, _k)]
}

func CELT_PVQ_V(_n, _k int) uint64 {
    return CELT_PVQ_U(_n, _k) + CELT_PVQ_U(_n, _k+1)
}

func icwrs(_n int, _y []int) uint64 {
    var i uint64
    OpusAssert(_n >= 2)
    j := _n - 1
    if _y[j] < 0 {
        i = 1
    } else {
        i = 0
    }
    k := abs(_y[j])
    for {
        j--
        i += CELT_PVQ_U(_n-j, k)
        k += abs(_y[j])
        if _y[j] < 0 {
            i += CELT_PVQ_U(_n-j, k+1)
        }
        if j <= 0 {
            break
        }
    }
    return i
}

func encode_pulses(_y []int, _n, _k int, _enc EntropyCoder) {
    OpusAssert(_k > 0)
    _enc.enc_uint(icwrs(_n, _y), CELT_PVQ_V(_n, _k))
}

func cwrsi(_n, _k int, _i uint64, _y []int) int {
    var p uint64
    var s int
    var k0 int
    var val int16
    yy := 0
    y_ptr := 0
    OpusAssert(_k > 0)
    OpusAssert(_n > 1)

    for _n > 2 {
        if _k >= _n {
            row := CELT_PVQ_U_ROW[_n]
            if _i >= CELT_PVQ_U_DATA[row+_k+1] {
                s = -1
            } else {
                s = 0
            }
            if s == -1 {
                _i -= (CELT_PVQ_U_DATA[row+_k+1] & 0xFFFFFFFF)
            }
            k0 = _k
            q := CELT_PVQ_U_DATA[row+_n]
            if q > _i {
                OpusAssert(CELT_PVQ_U_DATA[row+_k] > q)
                _k = _n
                for {
                    _k--
                    p = CELT_PVQ_U_DATA[CELT_PVQ_U_ROW[_k]+_n]
                    if p <= _i {
                        break
                    }
                }
            } else {
                p = CELT_PVQ_U_DATA[row+_k]
                for p > _i {
                    _k--
                    p = CELT_PVQ_U_DATA[row+_k]
                }
            }
            _i -= p
            val = int16((k0 - _k + s) ^ s)
            _y[y_ptr] = int(val)
            y_ptr++
            yy = MAC16_16(yy, val, val)
        } else {
            p = CELT_PVQ_U_DATA[CELT_PVQ_U_ROW[_k]+_n]
            q := CELT_PVQ_U_DATA[CELT_PVQ_U_ROW[_k+1]+_n]
            if p <= _i && _i < q {
                _i -= p
                _y[y_ptr] = 0
                y_ptr++
            } else {
                if _i >= q {
                    s = -1
                } else {
                    s = 0
                }
                if s == -1 {
                    _i -= (q & 0xFFFFFFFF)
                }
                k0 = _k
                for {
                    _k--
                    p = CELT_PVQ_U_DATA[CELT_PVQ_U_ROW[_k]+_n]
                    if p <= _i {
                        break
                    }
                }
                _i -= p
                val = int16((k0 - _k + s) ^ s)
                _y[y_ptr] = int(val)
                y_ptr++
                yy = MAC16_16(yy, val, val)
            }
        }
        _n--
    }

    p = uint64(2*_k + 1)
    if _i >= p {
        s = -1
    } else {
        s = 0
    }
    if s == -1 {
        _i -= (p & 0xFFFFFFFF)
    }
    k0 = _k
    _k = int((_i + 1) >> 1)
    if _k != 0 {
        _i -= uint64(2*_k - 1)
    }
    val = int16((k0 - _k + s) ^ s)
    _y[y_ptr] = int(val)
    y_ptr++
    yy = MAC16_16(yy, val, val)

    s = -int(_i)
    val = int16((_k + s) ^ s)
    _y[y_ptr] = int(val)
    yy = MAC16_16(yy, val, val)
    return yy
}

func decode_pulses(_y []int, _n, _k int, _dec EntropyCoder) int {
    return cwrsi(_n, _k, _dec.dec_uint(CELT_PVQ_V(_n, _k)), _y)
}