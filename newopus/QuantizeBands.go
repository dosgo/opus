package opus
import "math"

var pred_coef = []int{29440, 26112, 21248, 16384}
var beta_coef = []int{30147, 22282, 12124, 6554}
var beta_intra = 4915
var small_energy_icdf = []int16{2, 1, 0}

func loss_distortion(eBands [][]int16, oldEBands [][]int16, start int, end int, len int, C int) int {
	dist := 0
	for c := 0; c < C; c++ {
		for i := start; i < end; i++ {
			d := int((eBands[c][i] >> 3) - (oldEBands[c][i] >> 3))
			dist += d * d
		}
	}
	minDist := dist >> (2*CeltConstants.DB_SHIFT - 6)
	if minDist < 200 {
		return minDist
	}
	return 200
}

func quant_coarse_energy_impl(m *CeltMode, start int, end int, eBands [][]int16, oldEBands [][]int16, budget int, tell int, prob_model []int16, error [][]int, enc *EntropyCoder, C int, LM int, intra int, max_decay int, lfe int) int {
	badness := 0
	prev := [2]int{0, 0}
	var coef, beta int

	if tell+3 <= budget {
		enc.EncBitLogp(intra, 3)
	}

	if intra != 0 {
		coef = 0
		beta = beta_intra
	} else {
		beta = beta_coef[LM]
		coef = pred_coef[LM]
	}

	for i := start; i < end; i++ {
		for c := 0; c < C; c++ {
			x := eBands[c][i]
			oldE := oldEBands[c][i]
			if oldE < -((9 << CeltConstants.DB_SHIFT) - 1) {
				oldE = -((9 << CeltConstants.DB_SHIFT) - 1)
			}
			f := (int(x) << 7) - ((coef * int(oldE)) >> 8) - prev[c]
			qi := (f + (1 << (CeltConstants.DB_SHIFT + 6))) >> (CeltConstants.DB_SHIFT + 7)
			decay_bound := oldE - int16(max_decay)
			if decay_bound < -(28 << CeltConstants.DB_SHIFT) {
				decay_bound = -(28 << CeltConstants.DB_SHIFT)
			}
			if qi < 0 && int(x) < int(decay_bound) {
				qi += int((decay_bound - int16(x)) >> CeltConstants.DB_SHIFT)
				if qi > 0 {
					qi = 0
				}
			}
			qi0 := qi
			tell = enc.Tell()
			bits_left := budget - tell - 3*C*(end-i)
			if i != start && bits_left < 30 {
				if bits_left < 24 {
					if qi > 1 {
						qi = 1
					} else if qi < -1 {
						qi = -1
					}
				}
				if bits_left < 16 {
					if qi > 0 {
						qi = 0
					} else if qi < -1 {
						qi = -1
					}
				}
			}
			if lfe != 0 && i >= 2 {
				if qi > 0 {
					qi = 0
				}
			}
			if budget-tell >= 15 {
				pi := 2 * i
				if pi > 40 {
					pi = 40
				}
				qi = Laplace.EcLaplaceEncode(enc, qi, int(prob_model[pi])<<7, int(prob_model[pi+1])<<6)
			} else if budget-tell >= 2 {
				if qi < -1 {
					qi = -1
				} else if qi > 1 {
					qi = 1
				}
				val := 2 * qi
				if qi < 0 {
					val = -val
				}
				enc.EncIcdf(val, small_energy_icdf, 2)
			} else if budget-tell >= 1 {
				if qi > 0 {
					qi = 0
				}
				if qi < 0 {
					enc.EncBitLogp(1, 1)
				} else {
					enc.EncBitLogp(0, 1)
				}
			} else {
				qi = -1
			}
			error[c][i] = (f >> 7) - (qi << CeltConstants.DB_SHIFT)
			badness += int(math.Abs(float64(qi0 - qi))
			q := qi << CeltConstants.DB_SHIFT
			tmp := ((coef * int(oldE)) >> 8) + prev[c] + (q << 7)
			if tmp < -(28 << (CeltConstants.DB_SHIFT + 7)) {
				tmp = -(28 << (CeltConstants.DB_SHIFT + 7))
			}
			oldEBands[c][i] = int16(tmp >> 7)
			prev[c] = prev[c] + (q << 7) - (beta * (q >> 8))
		}
	}
	if lfe != 0 {
		return 0
	}
	return badness
}

func quant_coarse_energy(m *CeltMode, start int, end int, effEnd int, eBands [][]int16, oldEBands [][]int16, budget int, error [][]int, enc *EntropyCoder, C int, LM int, nbAvailableBytes int, force_intra int, delayedIntra *BoxedValueInt, two_pass int, loss_rate int, lfe int) {
	intra := 0
	if force_intra != 0 || (two_pass == 0 && delayedIntra.Val > 2*C*(end-start) && nbAvailableBytes > (end-start)*C) {
		intra = 1
	}
	intra_bias := (budget * delayedIntra.Val * loss_rate) / (C * 512)
	new_distortion := loss_distortion(eBands, oldEBands, start, effEnd, m.nbEBands, C)

	tell := enc.Tell()
	if tell+3 > budget {
		two_pass = 0
		intra = 0
	}

	max_decay := 16 << CeltConstants.DB_SHIFT
	if end-start > 10 {
		if max_decay > (nbAvailableBytes << (CeltConstants.DB_SHIFT - 3)) {
			max_decay = nbAvailableBytes << (CeltConstants.DB_SHIFT - 3)
		}
	}
	if lfe != 0 {
		max_decay = 3 << CeltConstants.DB_SHIFT
	}
	enc_start_state := enc.Copy()

	oldEBands_intra := make([][]int16, C)
	error_intra := make([][]int, C)
	for c := 0; c < C; c++ {
		oldEBands_intra[c] = make([]int16, m.nbEBands)
		error_intra[c] = make([]int, m.nbEBands)
		copy(oldEBands_intra[c], oldEBands[c])
	}

	badness1 := 0
	if two_pass != 0 || intra != 0 {
		badness1 = quant_coarse_energy_impl(m, start, end, eBands, oldEBands_intra, budget, tell, CeltTables.E_prob_model[LM][1], error_intra, enc, C, LM, 1, max_decay, lfe)
	}

	if intra == 0 {
		enc_intra_state := enc.Copy()
		tell_intra := enc.TellFrac()
		nstart_bytes := enc_start_state.RangeBytes()
		nintra_bytes := enc_intra_state.RangeBytes()
		intra_buf := nstart_bytes
		save_bytes := nintra_bytes - nstart_bytes
		var intra_bits []byte
		if save_bytes > 0 {
			intra_bits = make([]byte, save_bytes)
			copy(intra_bits, enc_intra_state.GetBuffer()[intra_buf:intra_buf+save_bytes])
		}

		enc.Assign(enc_start_state)
		badness2 := quant_coarse_energy_impl(m, start, end, eBands, oldEBands, budget, tell, CeltTables.E_prob_model[LM][0], error, enc, C, LM, 0, max_decay, lfe)

		if two_pass != 0 && (badness1 < badness2 || (badness1 == badness2 && enc.TellFrac()+intra_bias > tell_intra)) {
			enc.Assign(enc_intra_state)
			if save_bytes > 0 {
				enc.WriteBuffer(intra_bits, intra_buf, save_bytes)
			}
			for c := 0; c < C; c++ {
				copy(oldEBands[c], oldEBands_intra[c])
				copy(error[c], error_intra[c])
			}
			intra = 1
		}
	} else {
		for c := 0; c < C; c++ {
			copy(oldEBands[c], oldEBands_intra[c])
			copy(error[c], error_intra[c])
		}
	}

	if intra != 0 {
		delayedIntra.Val = new_distortion
	} else {
		pred := pred_coef[LM]
		delayedIntra.Val = (pred*pred*delayedIntra.Val)/32768 + new_distortion
	}
}

func quant_fine_energy(m *CeltMode, start int, end int, oldEBands [][]int16, error [][]int, fine_quant []int, enc *EntropyCoder, C int) {
	for i := start; i < end; i++ {
		frac := 1 << fine_quant[i]
		if fine_quant[i] <= 0 {
			continue
		}
		for c := 0; c < C; c++ {
			q2 := (error[c][i] + (1 << (CeltConstants.DB_SHIFT - 1))) >> (CeltConstants.DB_SHIFT - fine_quant[i])
			if q2 > frac-1 {
				q2 = frac - 1
			}
			if q2 < 0 {
				q2 = 0
			}
			enc.EncBits(uint32(q2), fine_quant[i])
			offset := ((q2 << CeltConstants.DB_SHIFT) + (1 << (CeltConstants.DB_SHIFT - 1))) >> fine_quant[i]
			offset -= 1 << (CeltConstants.DB_SHIFT - 1)
			oldEBands[c][i] += int16(offset)
			error[c][i] -= offset
		}
	}
}

func quant_energy_finalise(m *CeltMode, start int, end int, oldEBands [][]int16, error [][]int, fine_quant []int, fine_priority []int, bits_left int, enc *EntropyCoder, C int) {
	for prio := 0; prio < 2; prio++ {
		for i := start; i < end && bits_left >= C; i++ {
			if fine_quant[i] >= CeltConstants.MAX_FINE_BITS || fine_priority[i] != prio {
				continue
			}
			for c := 0; c < C; c++ {
				q2 := 0
				if error[c][i] >= 0 {
					q2 = 1
				}
				enc.EncBits(uint32(q2), 1)
				offset := (q2<<CeltConstants.DB_SHIFT - (1 << (CeltConstants.DB_SHIFT - 1))) >> (fine_quant[i] + 1)
				oldEBands[c][i] += int16(offset)
				bits_left--
			}
		}
	}
}

func unquant_coarse_energy(m *CeltMode, start int, end int, oldEBands []int16, intra int, dec *EntropyCoder, C int, LM int) {
	prob_model := CeltTables.E_prob_model[LM][intra]
	prev := [2]int{0, 0}
	var coef, beta int
	budget := dec.Storage() * 8

	if intra != 0 {
		coef = 0
		beta = beta_intra
	} else {
		beta = beta_coef[LM]
		coef = pred_coef[LM]
	}

	for i := start; i < end; i++ {
		for c := 0; c < C; c++ {
			tell := dec.Tell()
			var qi int
			if budget-tell >= 15 {
				pi := 2 * i
				if pi > 40 {
					pi = 40
				}
				qi = Laplace.EcLaplaceDecode(dec, int(prob_model[pi])<<7, int(prob_model[pi+1])<<6)
			} else if budget-tell >= 2 {
				val := dec.DecIcdf(small_energy_icdf, 2)
				qi = (val >> 1)
				if (val & 1) != 0 {
					qi = -qi
				}
			} else if budget-tell >= 1 {
				qi = -dec.DecBitLogp(1)
			} else {
				qi = -1
			}
			q := qi << CeltConstants.DB_SHIFT
			index := i + c*m.nbEBands
			if oldEBands[index] < -(9 << CeltConstants.DB_SHIFT) {
				oldEBands[index] = -(9 << CeltConstants.DB_SHIFT)
			}
			tmp := (coef*int(oldEBands[index]))>>8 + prev[c] + (q << 7)
			if tmp < -(28 << (CeltConstants.DB_SHIFT + 7)) {
				tmp = -(28 << (CeltConstants.DB_SHIFT + 7))
			}
			oldEBands[index] = int16(tmp >> 7)
			prev[c] = prev[c] + (q<<7) - (beta * (q >> 8))
		}
	}
}

func unquant_fine_energy(m *CeltMode, start int, end int, oldEBands []int16, fine_quant []int, dec *EntropyCoder, C int) {
	for i := start; i < end; i++ {
		if fine_quant[i] <= 0 {
			continue
		}
		for c := 0; c < C; c++ {
			q2 := dec.DecBits(fine_quant[i])
			offset := ((q2 << CeltConstants.DB_SHIFT) + (1 << (fine_quant[i] - 1))) >> fine_quant[i]
			offset -= 1 << (CeltConstants.DB_SHIFT - 1)
			index := i + c*m.nbEBands
			oldEBands[index] += int16(offset)
		}
	}
}

func unquant_energy_finalise(m *CeltMode, start int, end int, oldEBands []int16, fine_quant []int, fine_priority []int, bits_left int, dec *EntropyCoder, C int) {
	for prio := 0; prio < 2; prio++ {
		for i := start; i < end && bits_left >= C; i++ {
			if fine_quant[i] >= CeltConstants.MAX_FINE_BITS || fine_priority[i] != prio {
				continue
			}
			for c := 0; c < C; c++ {
				q2 := dec.DecBits(1)
				offset := (q2<<CeltConstants.DB_SHIFT - (1 << (CeltConstants.DB_SHIFT - 1))) >> (fine_quant[i] + 1)
				index := i + c*m.nbEBands
				oldEBands[index] += int16(offset)
				bits_left--
			}
		}
	}
}

func amp2Log2(m *CeltMode, effEnd int, end int, bandE [][]int16, bandLogE [][]int16, C int) {
	for c := 0; c < C; c++ {
		for i := 0; i < effEnd; i++ {
			bandLogE[c][i] = int16(Inlines.CeltLog2(int32(bandE[c][i])<<2) - int16(CeltTables.EMeans[i])<<6)
		}
		for i := effEnd; i < end; i++ {
			bandLogE[c][i] = -(14 << CeltConstants.DB_SHIFT)
		}
	}
}

func amp2Log2Ptr(m *CeltMode, effEnd int, end int, bandE []int16, bandLogE []int16, bandLogEPtr int, C int) {
	for c := 0; c < C; c++ {
		for i := 0; i < effEnd; i++ {
			bandLogE[bandLogEPtr+c*m.nbEBands+i] = int16(Inlines.CeltLog2(int32(bandE[i+c*m.nbEBands])<<2) - int16(CeltTables.EMeans[i])<<6)
		}
		for i := effEnd; i < end; i++ {
			bandLogE[bandLogEPtr+c*m.nbEBands+i] = -(14 << CeltConstants.DB_SHIFT)
		}
	}
}