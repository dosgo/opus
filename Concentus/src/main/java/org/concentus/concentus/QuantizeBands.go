package concentus

var (
	pred_coef        = [4]int{29440, 26112, 21248, 16384}
	beta_coef        = [4]int{30147, 22282, 12124, 6554}
	beta_intra       = 4915
	small_energy_icdf = [3]int16{2, 1, 0}
)

func loss_distortion(eBands, oldEBands [][]int16, start, end, len, C int) int {
	var dist int
	for c := 0; c < C; c++ {
		for i := start; i < end; i++ {
			d := SUB16(SHR16(eBands[c][i], 3) - SUB16(SHR16(oldEBands[c][i], 3)
			dist = MAC16_16(dist, d, d)
		}
	}
	return MIN32(200, SHR32(dist, 2*DB_SHIFT-6))
}

func quant_coarse_energy_impl(m *CeltMode, start, end int, eBands, oldEBands [][]int16, 
	budget, tell int, prob_model []int16, error [][]int16, enc *EntropyCoder, 
	C, LM, intra, max_decay, lfe int) int {
	var badness int
	prev := [2]int{0, 0}
	var coef, beta int

	if tell+3 <= budget {
		enc.enc_bit_logp(intra, 3)
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
			var bits_left, qi, qi0, q, x, f, tmp, oldE, decay_bound int
			x = int(eBands[c][i])
			oldE = MAX16(-QCONST16(9.0, DB_SHIFT), oldEBands[c][i])
			f = SHL32(EXTEND32(x), 7) - PSHR32(MULT16_16(coef, oldE), 8) - prev[c]
			qi = (f + QCONST32(.5, DB_SHIFT+7)) >> (DB_SHIFT + 7)
			decay_bound = EXTRACT16(MAX32(-QCONST16(28.0, DB_SHIFT),
				SUB32(int(oldEBands[c][i]), max_decay)))

			if qi < 0 && x < decay_bound {
				qi += int(SHR16(SUB16(decay_bound, x), DB_SHIFT))
				if qi > 0 {
					qi = 0
				}
			}
			qi0 = qi

			tell = enc.tell()
			bits_left = budget - tell - 3*C*(end-i)
			if i != start && bits_left < 30 {
				if bits_left < 24 {
					qi = IMIN(1, qi)
				}
				if bits_left < 16 {
					qi = IMAX(-1, qi)
				}
			}
			if lfe != 0 && i >= 2 {
				qi = IMIN(qi, 0)
			}

			if budget-tell >= 15 {
				pi := 2 * IMIN(i, 20)
				qi = ec_laplace_encode(enc, qi, prob_model[pi]<<7, prob_model[pi+1]<<6)
			} else if budget-tell >= 2 {
				qi = IMAX(-1, IMIN(qi, 1))
				enc.enc_icdf(2*qi^-(qi>>31), small_energy_icdf[:], 2)
			} else if budget-tell >= 1 {
				qi = IMIN(0, qi)
				enc.enc_bit_logp(-qi, 1)
			} else {
				qi = -1
			}

			error[c][i] = PSHR32(f, 7) - SHL16(qi, DB_SHIFT)
			badness += abs(qi0 - qi)
			q = SHL32(qi, DB_SHIFT)

			tmp = PSHR32(MULT16_16(coef, oldE), 8) + prev[c] + SHL32(q, 7)
			tmp = MAX32(-QCONST32(28.0, DB_SHIFT+7), tmp)
			oldEBands[c][i] = PSHR32(tmp, 7)
			prev[c] += SHL32(q, 7) - MULT16_16(beta, PSHR32(q, 8))
		}
	}

	if lfe != 0 {
		return 0
	}
	return badness
}

func quant_coarse_energy(m *CeltMode, start, end, effEnd int, eBands, oldEBands [][]int16, 
	budget int, error [][]int16, enc *EntropyCoder, C, LM, nbAvailableBytes, 
	force_intra int, delayedIntra *int, two_pass, loss_rate, lfe int) {
	var intra, max_decay, badness1, intra_bias, new_distortion int
	var oldEBands_intra, error_intra [][]int16
	var enc_start_state EntropyCoder

	intra = 0
	if force_intra != 0 || (two_pass == 0 && *delayedIntra > 2*C*(end-start) && nbAvailableBytes > (end-start)*C) {
		intra = 1
	}
	intra_bias = (budget * *delayedIntra * loss_rate) / (C * 512)
	new_distortion = loss_distortion(eBands, oldEBands, start, effEnd, m.nbEBands, C)

	tell = enc.tell()
	if tell+3 > budget {
		two_pass = 0
		intra = 0
	}

	max_decay = QCONST16(16.0, DB_SHIFT)
	if end-start > 10 {
		max_decay = MIN32(max_decay, SHL32(nbAvailableBytes, DB_SHIFT-3))
	}
	if lfe != 0 {
		max_decay = QCONST16(3.0, DB_SHIFT)
	}
	enc_start_state = *enc

	oldEBands_intra = make([][]int16, C)
	error_intra = make([][]int16, C)
	for c := 0; c < C; c++ {
		oldEBands_intra[c] = make([]int16, m.nbEBands)
		error_intra[c] = make([]int16, m.nbEBands)
		copy(oldEBands_intra[c], oldEBands[c])
	}

	if two_pass != 0 || intra != 0 {
		badness1 = quant_coarse_energy_impl(m, start, end, eBands, oldEBands_intra, budget,
			tell, CeltTables.e_prob_model[LM][1], error_intra, enc, C, LM, 1, max_decay, lfe)
	}

	if intra == 0 {
		var enc_intra_state EntropyCoder
		var tell_intra, nstart_bytes, nintra_bytes, save_bytes, badness2 int
		var intra_bits []byte

		tell_intra = enc.tell_frac()
		enc_intra_state = *enc

		nstart_bytes = enc_start_state.range_bytes()
		nintra_bytes = enc_intra_state.range_bytes()
		save_bytes = nintra_bytes - nstart_bytes

		if save_bytes != 0 {
			intra_bits = make([]byte, save_bytes)
			copy(intra_bits, enc_intra_state.get_buffer()[nstart_bytes:nintra_bytes])
		}

		*enc = enc_start_state

		badness2 = quant_coarse_energy_impl(m, start, end, eBands, oldEBands, budget,
			tell, CeltTables.e_prob_model[LM][intra], error, enc, C, LM, 0, max_decay, lfe)

		if two_pass != 0 && (badness1 < badness2 || (badness1 == badness2 && enc.tell_frac()+intra_bias > tell_intra)) {
			*enc = enc_intra_state
			if intra_bits != nil {
				enc.write_buffer(intra_bits, 0, nstart_bytes, save_bytes)
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
		*delayedIntra = new_distortion
	} else {
		*delayedIntra = ADD32(MULT16_32_Q15(MULT16_16_Q15(pred_coef[LM], pred_coef[LM]), *delayedIntra),
			new_distortion)
	}
}

func quant_fine_energy(m *CeltMode, start, end int, oldEBands, error [][]int16, 
	fine_quant []int, enc *EntropyCoder, C int) {
	for i := start; i < end; i++ {
		frac := 1 << fine_quant[i]
		if fine_quant[i] <= 0 {
			continue
		}
		for c := 0; c < C; c++ {
			var q2, offset int
			q2 = (error[c][i] + QCONST16(.5, DB_SHIFT)) >> (DB_SHIFT - fine_quant[i])
			if q2 > frac-1 {
				q2 = frac - 1
			}
			if q2 < 0 {
				q2 = 0
			}
			enc.enc_bits(q2, fine_quant[i])
			offset = SUB16(SHR32(SHL32(q2, DB_SHIFT)+QCONST16(.5, DB_SHIFT), fine_quant[i]),
				QCONST16(.5, DB_SHIFT))
			oldEBands[c][i] += offset
			error[c][i] -= offset
		}
	}
}

func quant_energy_finalise(m *CeltMode, start, end int, oldEBands, error [][]int16, 
	fine_quant, fine_priority []int, bits_left int, enc *EntropyCoder, C int) {
	for prio := 0; prio < 2; prio++ {
		for i := start; i < end && bits_left >= C; i++ {
			if fine_quant[i] >= MAX_FINE_BITS || fine_priority[i] != prio {
				continue
			}
			for c := 0; c < C; c++ {
				var q2, offset int
				if error[c][i] < 0 {
					q2 = 0
				} else {
					q2 = 1
				}
				enc.enc_bits(q2, 1)
				offset = SHR16(SHL16(q2, DB_SHIFT)-QCONST16(.5, DB_SHIFT), fine_quant[i]+1)
				oldEBands[c][i] += offset
				bits_left--
			}
		}
	}
}

func unquant_coarse_energy(m *CeltMode, start, end int, oldEBands []int16, 
	intra int, dec *EntropyCoder, C, LM int) {
	prob_model := CeltTables.e_prob_model[LM][intra]
	prev := [2]int{0, 0}
	var coef, beta, budget, tell int

	if intra != 0 {
		coef = 0
		beta = beta_intra
	} else {
		beta = beta_coef[LM]
		coef = pred_coef[LM]
	}

	budget = dec.storage * 8

	for i := start; i < end; i++ {
		for c := 0; c < C; c++ {
			var qi, q, tmp int
			tell = dec.tell()
			if budget-tell >= 15 {
				pi := 2 * IMIN(i, 20)
				qi = ec_laplace_decode(dec, prob_model[pi]<<7, prob_model[pi+1]<<6)
			} else if budget-tell >= 2 {
				qi = dec.dec_icdf(small_energy_icdf[:], 2)
				qi = (qi >> 1) ^ -(qi & 1)
			} else if budget-tell >= 1 {
				qi = -dec.dec_bit_logp(1)
			} else {
				qi = -1
			}
			q = SHL32(qi, DB_SHIFT)

			oldEBands[i+c*m.nbEBands] = MAX16(-QCONST16(9.0, DB_SHIFT), oldEBands[i+c*m.nbEBands])
			tmp = PSHR32(MULT16_16(coef, oldEBands[i+c*m.nbEBands]), 8) + prev[c] + SHL32(q, 7)
			tmp = MAX32(-QCONST32(28.0, DB_SHIFT+7), tmp)
			oldEBands[i+c*m.nbEBands] = PSHR32(tmp, 7)
			prev[c] += SHL32(q, 7) - MULT16_16(beta, PSHR32(q, 8))
		}
	}
}

func unquant_fine_energy(m *CeltMode, start, end int, oldEBands []int16, 
	fine_quant []int, dec *EntropyCoder, C int) {
	for i := start; i < end; i++ {
		if fine_quant[i] <= 0 {
			continue
		}
		for c := 0; c < C; c++ {
			var q2, offset int
			q2 = dec.dec_bits(fine_quant[i])
			offset = SUB16(SHR32(SHL32(q2, DB_SHIFT)+QCONST16(.5, DB_SHIFT), fine_quant[i]),
				QCONST16(.5, DB_SHIFT))
			oldEBands[i+c*m.nbEBands] += offset
		}
	}
}

func unquant_energy_finalise(m *CeltMode, start, end int, oldEBands []int16, 
	fine_quant, fine_priority []int, bits_left int, dec *EntropyCoder, C int) {
	for prio := 0; prio < 2; prio++ {
		for i := start; i < end && bits_left >= C; i++ {
			if fine_quant[i] >= MAX_FINE_BITS || fine_priority[i] != prio {
				continue
			}
			for c := 0; c < C; c++ {
				var q2, offset int
				q2 = dec.dec_bits(1)
				offset = SHR16(SHL16(q2, DB_SHIFT)-QCONST16(.5, DB_SHIFT), fine_quant[i]+1)
				oldEBands[i+c*m.nbEBands] += offset
				bits_left--
			}
		}
	}
}

func amp2Log2(m *CeltMode, effEnd, end int, bandE, bandLogE [][]int16, C int) {
	for c := 0; c < C; c++ {
		for i := 0; i < effEnd; i++ {
			bandLogE[c][i] = celt_log2(SHL32(bandE[c][i], 2)) - SHL16(CeltTables.eMeans[i], 6)
		}
		for i := effEnd; i < end; i++ {
			bandLogE[c][i] = -QCONST16(14.0, DB_SHIFT)
		}
	}
}

func amp2Log2_ptr(m *CeltMode, effEnd, end int, bandE, bandLogE []int16, bandLogE_ptr, C int) {
	for c := 0; c < C; c++ {
		for i := 0; i < effEnd; i++ {
			bandLogE[bandLogE_ptr+c*m.nbEBands+i] = 
				celt_log2(SHL32(bandE[i+c*m.nbEBands], 2)) - SHL16(CeltTables.eMeans[i], 6)
		}
		for i := effEnd; i < end; i++ {
			bandLogE[bandLogE_ptr+c*m.nbEBands+i] = -QCONST16(14.0, DB_SHIFT)
		}
	}
}
