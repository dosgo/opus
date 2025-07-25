package opus
func silk_encode_indices(psEncC *SilkChannelEncoder, psRangeEnc *EntropyCoder, FrameIndex int, encode_LBRR int, condCoding int) {
    var i, k, typeOffset int
    var encode_absolute_lagIndex, delta_lagIndex int
    ec_ix := make([]int16, MAX_LPC_ORDER)
    pred_Q8 := make([]int16, MAX_LPC_ORDER)
    var psIndices *SideInfoIndices

    if encode_LBRR != 0 {
        psIndices = psEncC.indices_LBRR[FrameIndex]
    } else {
        psIndices = psEncC.indices
    }

    typeOffset = 2*psIndices.signalType + psIndices.quantOffsetType
    OpusAssert(typeOffset >= 0 && typeOffset < 6)
    OpusAssert(encode_LBRR == 0 || typeOffset >= 2)
    if encode_LBRR != 0 || typeOffset >= 2 {
        psRangeEnc.enc_icdf(typeOffset-2, silk_type_offset_VAD_iCDF[:], 8)
    } else {
        psRangeEnc.enc_icdf(typeOffset, silk_type_offset_no_VAD_iCDF[:], 8)
    }

    if condCoding == CODE_CONDITIONALLY {
        OpusAssert(psIndices.GainsIndices[0] >= 0 && psIndices.GainsIndices[0] < MAX_DELTA_GAIN_QUANT-MIN_DELTA_GAIN_QUANT+1)
        psRangeEnc.enc_icdf(psIndices.GainsIndices[0], silk_delta_gain_iCDF[:], 8)
    } else {
        OpusAssert(psIndices.GainsIndices[0] >= 0 && psIndices.GainsIndices[0] < N_LEVELS_QGAIN)
        psRangeEnc.enc_icdf(psIndices.GainsIndices[0]>>3, silk_gain_iCDF[psIndices.signalType][:], 8)
        psRangeEnc.enc_icdf(psIndices.GainsIndices[0]&7, silk_uniform8_iCDF[:], 8)
    }

    for i = 1; i < psEncC.nb_subfr; i++ {
        OpusAssert(psIndices.GainsIndices[i] >= 0 && psIndices.GainsIndices[i] < MAX_DELTA_GAIN_QUANT-MIN_DELTA_GAIN_QUANT+1)
        psRangeEnc.enc_icdf(psIndices.GainsIndices[i], silk_delta_gain_iCDF[:], 8)
    }

    nVectors := psEncC.psNLSF_CB.nVectors
    if psIndices.signalType>>1 != 0 {
        nVectors *= 2
    }
    psRangeEnc.enc_icdf(psIndices.NLSFIndices[0], psEncC.psNLSF_CB.CB1_iCDF, nVectors, 8)
    silk_NLSF_unpack(ec_ix, pred_Q8, psEncC.psNLSF_CB, psIndices.NLSFIndices[0])
    OpusAssert(psEncC.psNLSF_CB.order == psEncC.predictLPCOrder)

    for i = 0; i < psEncC.psNLSF_CB.order; i++ {
        if psIndices.NLSFIndices[i+1] >= NLSF_QUANT_MAX_AMPLITUDE {
            psRangeEnc.enc_icdf(2*NLSF_QUANT_MAX_AMPLITUDE, psEncC.psNLSF_CB.ec_iCDF, int(ec_ix[i]), 8)
            psRangeEnc.enc_icdf(psIndices.NLSFIndices[i+1]-NLSF_QUANT_MAX_AMPLITUDE, silk_NLSF_EXT_iCDF[:], 8)
        } else if psIndices.NLSFIndices[i+1] <= -NLSF_QUANT_MAX_AMPLITUDE {
            psRangeEnc.enc_icdf(0, psEncC.psNLSF_CB.ec_iCDF, int(ec_ix[i]), 8)
            psRangeEnc.enc_icdf(-psIndices.NLSFIndices[i+1]-NLSF_QUANT_MAX_AMPLITUDE, silk_NLSF_EXT_iCDF[:], 8)
        } else {
            psRangeEnc.enc_icdf(psIndices.NLSFIndices[i+1]+NLSF_QUANT_MAX_AMPLITUDE, psEncC.psNLSF_CB.ec_iCDF, int(ec_ix[i]), 8)
        }
    }

    if psEncC.nb_subfr == MAX_NB_SUBFR {
        OpusAssert(psIndices.NLSFInterpCoef_Q2 >= 0 && psIndices.NLSFInterpCoef_Q2 < 5)
        psRangeEnc.enc_icdf(psIndices.NLSFInterpCoef_Q2, silk_NLSF_interpolation_factor_iCDF[:], 8)
    }

    if psIndices.signalType == TYPE_VOICED {
        encode_absolute_lagIndex = 1
        if condCoding == CODE_CONDITIONALLY && psEncC.ec_prevSignalType == TYPE_VOICED {
            delta_lagIndex = psIndices.lagIndex - psEncC.ec_prevLagIndex
            if delta_lagIndex < -8 || delta_lagIndex > 11 {
                delta_lagIndex = 0
            } else {
                delta_lagIndex += 9
                encode_absolute_lagIndex = 0
            }
            OpusAssert(delta_lagIndex >= 0 && delta_lagIndex < 21)
            psRangeEnc.enc_icdf(delta_lagIndex, silk_pitch_delta_iCDF[:], 8)
        }

        if encode_absolute_lagIndex != 0 {
            pitch_high_bits := psIndices.lagIndex / (psEncC.fs_kHz >> 1)
            pitch_low_bits := psIndices.lagIndex - pitch_high_bits*(psEncC.fs_kHz>>1)
            OpusAssert(pitch_low_bits < psEncC.fs_kHz/2)
            OpusAssert(pitch_high_bits < 32)
            psRangeEnc.enc_icdf(pitch_high_bits, silk_pitch_lag_iCDF[:], 8)
            psRangeEnc.enc_icdf(pitch_low_bits, psEncC.pitch_lag_low_bits_iCDF, 8)
        }
        psEncC.ec_prevLagIndex = psIndices.lagIndex

        OpusAssert(psIndices.contourIndex >= 0)
        OpusAssert((psIndices.contourIndex < 34 && psEncC.fs_kHz > 8 && psEncC.nb_subfr == 4) ||
            (psIndices.contourIndex < 11 && psEncC.fs_kHz == 8 && psEncC.nb_subfr == 4) ||
            (psIndices.contourIndex < 12 && psEncC.fs_kHz > 8 && psEncC.nb_subfr == 2) ||
            (psIndices.contourIndex < 3 && psEncC.fs_kHz == 8 && psEncC.nb_subfr == 2))
        psRangeEnc.enc_icdf(psIndices.contourIndex, psEncC.pitch_contour_iCDF, 8)

        OpusAssert(psIndices.PERIndex >= 0 && psIndices.PERIndex < 3)
        psRangeEnc.enc_icdf(psIndices.PERIndex, silk_LTP_per_index_iCDF[:], 8)

        for k = 0; k < psEncC.nb_subfr; k++ {
            OpusAssert(psIndices.LTPIndex[k] >= 0 && psIndices.LTPIndex[k] < (8<<psIndices.PERIndex))
            psRangeEnc.enc_icdf(psIndices.LTPIndex[k], silk_LTP_gain_iCDF_ptrs[psIndices.PERIndex][:], 8)
        }

        if condCoding == CODE_INDEPENDENTLY {
            OpusAssert(psIndices.LTP_scaleIndex >= 0 && psIndices.LTP_scaleIndex < 3)
            psRangeEnc.enc_icdf(psIndices.LTP_scaleIndex, silk_LTPscale_iCDF[:], 8)
        }

        OpusAssert(condCoding != CODE_INDEPENDENTLY || psIndices.LTP_scaleIndex == 0)
    }

    psEncC.ec_prevSignalType = psIndices.signalType

    OpusAssert(psIndices.Seed >= 0 && psIndices.Seed < 4)
    psRangeEnc.enc_icdf(psIndices.Seed, silk_uniform4_iCDF[:], 8)
}