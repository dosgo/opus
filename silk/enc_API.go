package silk

import (
	"math"
	"unsafe"

	"github.com/gotranspile/cxgo/runtime/libc"

	"github.com/gotranspile/opus/celt"
)

func GetEncoderSize(encSizeBytes *int) int {
	var ret int = SILK_NO_ERROR
	*encSizeBytes = int(unsafe.Sizeof(Encoder{}))
	return ret
}
func (psEnc *Encoder) Init(arch int, encStatus *EncControlStruct) int {
	var (
		n   int
		ret int = SILK_NO_ERROR
	)
	*psEnc = Encoder{}
	for n = 0; n < ENCODER_NUM_CHANNELS; n++ {
		if func() int {
			ret += psEnc.State_Fxx[n].Init(arch)
			return ret
		}() != 0 {
		}
	}
	psEnc.NChannelsAPI = 1
	psEnc.NChannelsInternal = 1
	if func() int {
		ret += psEnc.Query(encStatus)
		return ret
	}() != 0 {
	}
	return ret
}
func (psEnc *Encoder) Query(encStatus *EncControlStruct) int {
	state_Fxx := &psEnc.State_Fxx[0]
	encStatus.NChannelsAPI = int32(psEnc.NChannelsAPI)
	encStatus.NChannelsInternal = int32(psEnc.NChannelsInternal)
	encStatus.API_sampleRate = state_Fxx.SCmn.API_fs_Hz
	encStatus.MaxInternalSampleRate = int32(state_Fxx.SCmn.MaxInternal_fs_Hz)
	encStatus.MinInternalSampleRate = int32(state_Fxx.SCmn.MinInternal_fs_Hz)
	encStatus.DesiredInternalSampleRate = int32(state_Fxx.SCmn.DesiredInternal_fs_Hz)
	encStatus.PayloadSize_ms = state_Fxx.SCmn.PacketSize_ms
	encStatus.BitRate = state_Fxx.SCmn.TargetRate_bps
	encStatus.PacketLossPercentage = state_Fxx.SCmn.PacketLoss_perc
	encStatus.Complexity = state_Fxx.SCmn.Complexity
	encStatus.UseInBandFEC = state_Fxx.SCmn.UseInBandFEC
	encStatus.UseDTX = state_Fxx.SCmn.UseDTX
	encStatus.UseCBR = state_Fxx.SCmn.UseCBR
	encStatus.InternalSampleRate = int32(int(int32(int16(state_Fxx.SCmn.Fs_kHz))) * 1000)
	encStatus.AllowBandwidthSwitch = state_Fxx.SCmn.Allow_bandwidth_switch
	encStatus.InWBmodeWithoutVariableLP = int(libc.BoolToInt(state_Fxx.SCmn.Fs_kHz == 16 && state_Fxx.SCmn.SLP.Mode == 0))
	return SILK_NO_ERROR
}
func (psEnc *Encoder) Encode(encControl *EncControlStruct, samplesIn []int16, nSamplesIn int, psRangeEnc *celt.ECEnc, nBytesOut *int32, prefillFlag int, activity int) int {
	var (
		n                            int
		i                            int
		nBits                        int
		flags                        int
		tmp_payloadSize_ms           int = 0
		tmp_complexity               int = 0
		ret                          int = 0
		nSamplesToBuffer             int
		nSamplesToBufferMax          int
		nBlocksOf10ms                int
		nSamplesFromInput            int = 0
		nSamplesFromInputMax         int
		speech_act_thr_for_switch_Q8 int
		TargetRate_bps               int32
		MStargetRates_bps            [2]int32
		channelRate_bps              int32
		LBRR_symbol                  int32
		sum                          int32
		buf                          []int16
		transition                   int
		curr_block                   int
		tot_blocks                   int
	)
	if encControl.ReducedDependency != 0 {
		psEnc.State_Fxx[0].SCmn.First_frame_after_reset = 1
		psEnc.State_Fxx[1].SCmn.First_frame_after_reset = 1
	}
	psEnc.State_Fxx[0].SCmn.NFramesEncoded = func() int {
		p := &psEnc.State_Fxx[1].SCmn.NFramesEncoded
		psEnc.State_Fxx[1].SCmn.NFramesEncoded = 0
		return *p
	}()
	if (func() int {
		ret = CheckControlInput(encControl)
		return ret
	}()) != 0 {
		return ret
	}
	encControl.SwitchReady = 0
	if int(encControl.NChannelsInternal) > psEnc.NChannelsInternal {
		ret += psEnc.State_Fxx[1].Init(psEnc.State_Fxx[0].SCmn.Arch)
		*(*[2]int16)(unsafe.Pointer(&psEnc.SStereo.Pred_prev_Q13[0])) = [2]int16{}
		*(*[2]int16)(unsafe.Pointer(&psEnc.SStereo.SSide[0])) = [2]int16{}
		psEnc.SStereo.Mid_side_amp_Q0[0] = 0
		psEnc.SStereo.Mid_side_amp_Q0[1] = 1
		psEnc.SStereo.Mid_side_amp_Q0[2] = 0
		psEnc.SStereo.Mid_side_amp_Q0[3] = 1
		psEnc.SStereo.Width_prev_Q14 = 0
		psEnc.SStereo.Smth_width_Q14 = int16(int32(math.Floor(1*(1<<14) + 0.5)))
		if psEnc.NChannelsAPI == 2 {
			psEnc.State_Fxx[1].SCmn.Resampler_state = psEnc.State_Fxx[0].SCmn.Resampler_state
			psEnc.State_Fxx[1].SCmn.In_HP_State = psEnc.State_Fxx[0].SCmn.In_HP_State
		}
	}
	transition = int(libc.BoolToInt(encControl.PayloadSize_ms != psEnc.State_Fxx[0].SCmn.PacketSize_ms || psEnc.NChannelsInternal != int(encControl.NChannelsInternal)))
	psEnc.NChannelsAPI = int(encControl.NChannelsAPI)
	psEnc.NChannelsInternal = int(encControl.NChannelsInternal)
	nBlocksOf10ms = int(int32((nSamplesIn * 100) / int(encControl.API_sampleRate)))
	if nBlocksOf10ms > 1 {
		tot_blocks = nBlocksOf10ms >> 1
	} else {
		tot_blocks = 1
	}
	curr_block = 0
	if prefillFlag != 0 {
		var save_LP LPState
		if nBlocksOf10ms != 1 {
			return -101
		}
		if prefillFlag == 2 {
			save_LP = psEnc.State_Fxx[0].SCmn.SLP
			save_LP.Saved_fs_kHz = int32(psEnc.State_Fxx[0].SCmn.Fs_kHz)
		}
		for n = 0; n < int(encControl.NChannelsInternal); n++ {
			ret = psEnc.State_Fxx[n].Init(psEnc.State_Fxx[n].SCmn.Arch)
			if prefillFlag == 2 {
				psEnc.State_Fxx[n].SCmn.SLP = save_LP
			}
		}
		tmp_payloadSize_ms = encControl.PayloadSize_ms
		encControl.PayloadSize_ms = 10
		tmp_complexity = encControl.Complexity
		encControl.Complexity = 0
		for n = 0; n < int(encControl.NChannelsInternal); n++ {
			psEnc.State_Fxx[n].SCmn.Controlled_since_last_payload = 0
			psEnc.State_Fxx[n].SCmn.PrefillFlag = 1
		}
	} else {
		if nBlocksOf10ms*int(encControl.API_sampleRate) != nSamplesIn*100 || nSamplesIn < 0 {
			return -101
		}
		if int(int32(nSamplesIn))*1000 > encControl.PayloadSize_ms*int(encControl.API_sampleRate) {
			return -101
		}
	}
	for n = 0; n < int(encControl.NChannelsInternal); n++ {
		var force_fs_kHz int
		if n == 1 {
			force_fs_kHz = psEnc.State_Fxx[0].SCmn.Fs_kHz
		} else {
			force_fs_kHz = 0
		}
		if (func() int {
			ret = ControlEncoder(&psEnc.State_Fxx[n], encControl, psEnc.AllowBandwidthSwitch, n, force_fs_kHz)
			return ret
		}()) != 0 {
			return ret
		}
		if psEnc.State_Fxx[n].SCmn.First_frame_after_reset != 0 || transition != 0 {
			for i = 0; i < psEnc.State_Fxx[0].SCmn.NFramesPerPacket; i++ {
				psEnc.State_Fxx[n].SCmn.LBRR_flags[i] = 0
			}
		}
		psEnc.State_Fxx[n].SCmn.InDTX = psEnc.State_Fxx[n].SCmn.UseDTX
	}
	nSamplesToBufferMax = nBlocksOf10ms * 10 * psEnc.State_Fxx[0].SCmn.Fs_kHz
	nSamplesFromInputMax = int(int32((nSamplesToBufferMax * int(psEnc.State_Fxx[0].SCmn.API_fs_Hz)) / (psEnc.State_Fxx[0].SCmn.Fs_kHz * 1000)))
	buf = make([]int16, nSamplesFromInputMax)
	for {
		var curr_nBitsUsedLBRR int = 0
		nSamplesToBuffer = psEnc.State_Fxx[0].SCmn.Frame_length - psEnc.State_Fxx[0].SCmn.InputBufIx
		if nSamplesToBuffer < nSamplesToBufferMax {
			nSamplesToBuffer = nSamplesToBuffer
		} else {
			nSamplesToBuffer = nSamplesToBufferMax
		}
		nSamplesFromInput = int(int32((nSamplesToBuffer * int(psEnc.State_Fxx[0].SCmn.API_fs_Hz)) / (psEnc.State_Fxx[0].SCmn.Fs_kHz * 1000)))
		if int(encControl.NChannelsAPI) == 2 && int(encControl.NChannelsInternal) == 2 {
			var id int = psEnc.State_Fxx[0].SCmn.NFramesEncoded
			for n = 0; n < nSamplesFromInput; n++ {
				buf[n] = samplesIn[n*2]
			}
			if psEnc.NPrevChannelsInternal == 1 && id == 0 {
				psEnc.State_Fxx[1].SCmn.Resampler_state = psEnc.State_Fxx[0].SCmn.Resampler_state
			}
			ret += psEnc.State_Fxx[0].SCmn.Resampler_state.Resample(psEnc.State_Fxx[0].SCmn.InputBuf[psEnc.State_Fxx[0].SCmn.InputBufIx+2:], []int16(buf), int32(nSamplesFromInput))
			psEnc.State_Fxx[0].SCmn.InputBufIx += nSamplesToBuffer
			nSamplesToBuffer = psEnc.State_Fxx[1].SCmn.Frame_length - psEnc.State_Fxx[1].SCmn.InputBufIx
			if nSamplesToBuffer < (nBlocksOf10ms * 10 * psEnc.State_Fxx[1].SCmn.Fs_kHz) {
				nSamplesToBuffer = nSamplesToBuffer
			} else {
				nSamplesToBuffer = nBlocksOf10ms * 10 * psEnc.State_Fxx[1].SCmn.Fs_kHz
			}
			for n = 0; n < nSamplesFromInput; n++ {
				buf[n] = samplesIn[n*2+1]
			}
			ret += psEnc.State_Fxx[1].SCmn.Resampler_state.Resample(psEnc.State_Fxx[1].SCmn.InputBuf[psEnc.State_Fxx[1].SCmn.InputBufIx+2:], []int16(buf), int32(nSamplesFromInput))
			psEnc.State_Fxx[1].SCmn.InputBufIx += nSamplesToBuffer
		} else if int(encControl.NChannelsAPI) == 2 && int(encControl.NChannelsInternal) == 1 {
			for n = 0; n < nSamplesFromInput; n++ {
				sum = int32(int(samplesIn[n*2]) + int(samplesIn[n*2+1]))
				if 1 == 1 {
					buf[n] = int16((int(sum) >> 1) + (int(sum) & 1))
				} else {
					buf[n] = int16(((int(sum) >> (1 - 1)) + 1) >> 1)
				}
			}
			ret += psEnc.State_Fxx[0].SCmn.Resampler_state.Resample(psEnc.State_Fxx[0].SCmn.InputBuf[psEnc.State_Fxx[0].SCmn.InputBufIx+2:], []int16(buf), int32(nSamplesFromInput))
			if psEnc.NPrevChannelsInternal == 2 && psEnc.State_Fxx[0].SCmn.NFramesEncoded == 0 {
				ret += psEnc.State_Fxx[1].SCmn.Resampler_state.Resample(psEnc.State_Fxx[1].SCmn.InputBuf[psEnc.State_Fxx[1].SCmn.InputBufIx+2:], []int16(buf), int32(nSamplesFromInput))
				for n = 0; n < psEnc.State_Fxx[0].SCmn.Frame_length; n++ {
					psEnc.State_Fxx[0].SCmn.InputBuf[psEnc.State_Fxx[0].SCmn.InputBufIx+n+2] = int16((int(psEnc.State_Fxx[0].SCmn.InputBuf[psEnc.State_Fxx[0].SCmn.InputBufIx+n+2]) + int(psEnc.State_Fxx[1].SCmn.InputBuf[psEnc.State_Fxx[1].SCmn.InputBufIx+n+2])) >> 1)
				}
			}
			psEnc.State_Fxx[0].SCmn.InputBufIx += nSamplesToBuffer
		} else {
			copy(buf[:nSamplesFromInput], samplesIn[:nSamplesFromInput])
			ret += psEnc.State_Fxx[0].SCmn.Resampler_state.Resample(psEnc.State_Fxx[0].SCmn.InputBuf[psEnc.State_Fxx[0].SCmn.InputBufIx+2:], []int16(buf), int32(nSamplesFromInput))
			psEnc.State_Fxx[0].SCmn.InputBufIx += nSamplesToBuffer
		}
		samplesIn = samplesIn[nSamplesFromInput*int(encControl.NChannelsAPI):]
		nSamplesIn -= nSamplesFromInput
		psEnc.AllowBandwidthSwitch = 0
		if psEnc.State_Fxx[0].SCmn.InputBufIx >= psEnc.State_Fxx[0].SCmn.Frame_length {
			if psEnc.State_Fxx[0].SCmn.NFramesEncoded == 0 && prefillFlag == 0 {
				var iCDF [2]uint8 = [2]uint8{}
				iCDF[0] = uint8(int8(int(256) - (256 >> ((psEnc.State_Fxx[0].SCmn.NFramesPerPacket + 1) * int(encControl.NChannelsInternal)))))
				psRangeEnc.EncIcdf(0, iCDF[:], 8)
				curr_nBitsUsedLBRR = psRangeEnc.Tell()
				for n = 0; n < int(encControl.NChannelsInternal); n++ {
					LBRR_symbol = 0
					for i = 0; i < psEnc.State_Fxx[n].SCmn.NFramesPerPacket; i++ {
						LBRR_symbol |= int32(int(uint32(int32(psEnc.State_Fxx[n].SCmn.LBRR_flags[i]))) << i)
					}
					if int(LBRR_symbol) > 0 {
						psEnc.State_Fxx[n].SCmn.LBRR_flag = 1
					} else {
						psEnc.State_Fxx[n].SCmn.LBRR_flag = 0
					}
					if int(LBRR_symbol) != 0 && psEnc.State_Fxx[n].SCmn.NFramesPerPacket > 1 {
						psRangeEnc.EncIcdf(int(LBRR_symbol)-1, []byte(silk_LBRR_flags_iCDF_ptr[psEnc.State_Fxx[n].SCmn.NFramesPerPacket-2]), 8)
					}
				}
				for i = 0; i < psEnc.State_Fxx[0].SCmn.NFramesPerPacket; i++ {
					for n = 0; n < int(encControl.NChannelsInternal); n++ {
						if psEnc.State_Fxx[n].SCmn.LBRR_flags[i] != 0 {
							var condCoding int
							if int(encControl.NChannelsInternal) == 2 && n == 0 {
								StereoEncodePred(psRangeEnc, psEnc.SStereo.PredIx[i])
								if psEnc.State_Fxx[1].SCmn.LBRR_flags[i] == 0 {
									StereoEncodeMidOnly(psRangeEnc, psEnc.SStereo.Mid_only_flags[i])
								}
							}
							if i > 0 && psEnc.State_Fxx[n].SCmn.LBRR_flags[i-1] != 0 {
								condCoding = CODE_CONDITIONALLY
							} else {
								condCoding = CODE_INDEPENDENTLY
							}
							EncodeIndices(&psEnc.State_Fxx[n].SCmn, psRangeEnc, i, 1, condCoding)
							EncodePulses(psRangeEnc, int(psEnc.State_Fxx[n].SCmn.Indices_LBRR[i].SignalType), int(psEnc.State_Fxx[n].SCmn.Indices_LBRR[i].QuantOffsetType), psEnc.State_Fxx[n].SCmn.Pulses_LBRR[i][:], psEnc.State_Fxx[n].SCmn.Frame_length)
						}
					}
				}
				for n = 0; n < int(encControl.NChannelsInternal); n++ {
					*(*[3]int)(unsafe.Pointer(&psEnc.State_Fxx[n].SCmn.LBRR_flags[0])) = [3]int{}
				}
				curr_nBitsUsedLBRR = psRangeEnc.Tell() - curr_nBitsUsedLBRR
			}
			HP_variable_cutoff(psEnc.State_Fxx[:])
			nBits = int(int32((int(encControl.BitRate) * encControl.PayloadSize_ms) / 1000))
			if prefillFlag == 0 {
				if curr_nBitsUsedLBRR < 10 {
					psEnc.NBitsUsedLBRR = 0
				} else if int(psEnc.NBitsUsedLBRR) < 10 {
					psEnc.NBitsUsedLBRR = int32(curr_nBitsUsedLBRR)
				} else {
					psEnc.NBitsUsedLBRR = int32((int(psEnc.NBitsUsedLBRR) + curr_nBitsUsedLBRR) / 2)
				}
				nBits -= int(psEnc.NBitsUsedLBRR)
			}
			nBits = int(int32(nBits / psEnc.State_Fxx[0].SCmn.NFramesPerPacket))
			if encControl.PayloadSize_ms == 10 {
				TargetRate_bps = int32(int(int32(int16(nBits))) * 100)
			} else {
				TargetRate_bps = int32(int(int32(int16(nBits))) * 50)
			}
			TargetRate_bps -= int32((int(psEnc.NBitsExceeded) * 1000) / BITRESERVOIR_DECAY_TIME_MS)
			if prefillFlag == 0 && psEnc.State_Fxx[0].SCmn.NFramesEncoded > 0 {
				var bitsBalance int32 = int32(psRangeEnc.Tell() - int(psEnc.NBitsUsedLBRR) - nBits*psEnc.State_Fxx[0].SCmn.NFramesEncoded)
				TargetRate_bps -= int32((int(bitsBalance) * 1000) / BITRESERVOIR_DECAY_TIME_MS)
			}
			if int(encControl.BitRate) > 5000 {
				if int(TargetRate_bps) > int(encControl.BitRate) {
					TargetRate_bps = encControl.BitRate
				} else if int(TargetRate_bps) < 5000 {
					TargetRate_bps = 5000
				} else {
					TargetRate_bps = TargetRate_bps
				}
			} else if int(TargetRate_bps) > 5000 {
				TargetRate_bps = 5000
			} else if int(TargetRate_bps) < int(encControl.BitRate) {
				TargetRate_bps = encControl.BitRate
			} else {
				TargetRate_bps = TargetRate_bps
			}
			if int(encControl.NChannelsInternal) == 2 {
				StereoLRtoMS(&psEnc.SStereo, psEnc.State_Fxx[0].SCmn.InputBuf[2:], psEnc.State_Fxx[1].SCmn.InputBuf[2:], psEnc.SStereo.PredIx[psEnc.State_Fxx[0].SCmn.NFramesEncoded], &psEnc.SStereo.Mid_only_flags[psEnc.State_Fxx[0].SCmn.NFramesEncoded], MStargetRates_bps[:], TargetRate_bps, psEnc.State_Fxx[0].SCmn.Speech_activity_Q8, encControl.ToMono, psEnc.State_Fxx[0].SCmn.Fs_kHz, psEnc.State_Fxx[0].SCmn.Frame_length)
				if int(psEnc.SStereo.Mid_only_flags[psEnc.State_Fxx[0].SCmn.NFramesEncoded]) == 0 {
					if psEnc.Prev_decode_only_middle == 1 {
						psEnc.State_Fxx[1].SShape = ShapeStateFLP{}
						psEnc.State_Fxx[1].SCmn.SNSQ = NSQState{}
						psEnc.State_Fxx[1].SCmn.Prev_NLSFq_Q15 = [16]int16{}
						psEnc.State_Fxx[1].SCmn.SLP.In_LP_State = [2]int32{}
						psEnc.State_Fxx[1].SCmn.PrevLag = 100
						psEnc.State_Fxx[1].SCmn.SNSQ.LagPrev = 100
						psEnc.State_Fxx[1].SShape.LastGainIndex = 10
						psEnc.State_Fxx[1].SCmn.PrevSignalType = TYPE_NO_VOICE_ACTIVITY
						psEnc.State_Fxx[1].SCmn.SNSQ.Prev_gain_Q16 = 65536
						psEnc.State_Fxx[1].SCmn.First_frame_after_reset = 1
					}
					EncodeDoVAD_FLP(&psEnc.State_Fxx[1], activity)
				} else {
					psEnc.State_Fxx[1].SCmn.VAD_flags[psEnc.State_Fxx[0].SCmn.NFramesEncoded] = 0
				}
				if prefillFlag == 0 {
					StereoEncodePred(psRangeEnc, psEnc.SStereo.PredIx[psEnc.State_Fxx[0].SCmn.NFramesEncoded])
					if int(psEnc.State_Fxx[1].SCmn.VAD_flags[psEnc.State_Fxx[0].SCmn.NFramesEncoded]) == 0 {
						StereoEncodeMidOnly(psRangeEnc, psEnc.SStereo.Mid_only_flags[psEnc.State_Fxx[0].SCmn.NFramesEncoded])
					}
				}
			} else {
				libc.MemCpy(unsafe.Pointer(&psEnc.State_Fxx[0].SCmn.InputBuf[0]), unsafe.Pointer(&psEnc.SStereo.SMid[0]), int(2*unsafe.Sizeof(int16(0))))
				libc.MemCpy(unsafe.Pointer(&psEnc.SStereo.SMid[0]), unsafe.Pointer(&psEnc.State_Fxx[0].SCmn.InputBuf[psEnc.State_Fxx[0].SCmn.Frame_length]), int(2*unsafe.Sizeof(int16(0))))
			}
			EncodeDoVAD_FLP(&psEnc.State_Fxx[0], activity)
			for n = 0; n < int(encControl.NChannelsInternal); n++ {
				var (
					maxBits int
					useCBR  int
				)
				maxBits = encControl.MaxBits
				if tot_blocks == 2 && curr_block == 0 {
					maxBits = maxBits * 3 / 5
				} else if tot_blocks == 3 {
					if curr_block == 0 {
						maxBits = maxBits * 2 / 5
					} else if curr_block == 1 {
						maxBits = maxBits * 3 / 4
					}
				}
				useCBR = int(libc.BoolToInt(encControl.UseCBR != 0 && curr_block == tot_blocks-1))
				if int(encControl.NChannelsInternal) == 1 {
					channelRate_bps = TargetRate_bps
				} else {
					channelRate_bps = MStargetRates_bps[n]
					if n == 0 && int(MStargetRates_bps[1]) > 0 {
						useCBR = 0
						maxBits -= encControl.MaxBits / (tot_blocks * 2)
					}
				}
				if int(channelRate_bps) > 0 {
					var condCoding int
					silk_control_SNR(&psEnc.State_Fxx[n].SCmn, channelRate_bps)
					if psEnc.State_Fxx[0].SCmn.NFramesEncoded-n <= 0 {
						condCoding = CODE_INDEPENDENTLY
					} else if n > 0 && psEnc.Prev_decode_only_middle != 0 {
						condCoding = CODE_INDEPENDENTLY_NO_LTP_SCALING
					} else {
						condCoding = CODE_CONDITIONALLY
					}
					if (func() int {
						ret = EncodeFrame_FLP(&psEnc.State_Fxx[n], nBytesOut, psRangeEnc, condCoding, maxBits, useCBR)
						return ret
					}()) != 0 {
					}
				}
				psEnc.State_Fxx[n].SCmn.Controlled_since_last_payload = 0
				psEnc.State_Fxx[n].SCmn.InputBufIx = 0
				psEnc.State_Fxx[n].SCmn.NFramesEncoded++
			}
			psEnc.Prev_decode_only_middle = int(psEnc.SStereo.Mid_only_flags[psEnc.State_Fxx[0].SCmn.NFramesEncoded-1])
			if int(*nBytesOut) > 0 && psEnc.State_Fxx[0].SCmn.NFramesEncoded == psEnc.State_Fxx[0].SCmn.NFramesPerPacket {
				flags = 0
				for n = 0; n < int(encControl.NChannelsInternal); n++ {
					for i = 0; i < psEnc.State_Fxx[n].SCmn.NFramesPerPacket; i++ {
						flags = int(int32(int(uint32(int32(flags))) << 1))
						flags |= int(psEnc.State_Fxx[n].SCmn.VAD_flags[i])
					}
					flags = int(int32(int(uint32(int32(flags))) << 1))
					flags |= int(psEnc.State_Fxx[n].SCmn.LBRR_flag)
				}
				if prefillFlag == 0 {
					psRangeEnc.EncPatchInitialBits(uint(flags), uint((psEnc.State_Fxx[0].SCmn.NFramesPerPacket+1)*int(encControl.NChannelsInternal)))
				}
				if psEnc.State_Fxx[0].SCmn.InDTX != 0 && (int(encControl.NChannelsInternal) == 1 || psEnc.State_Fxx[1].SCmn.InDTX != 0) {
					*nBytesOut = 0
				}
				psEnc.NBitsExceeded += int32(int(*nBytesOut) * 8)
				psEnc.NBitsExceeded -= int32((int(encControl.BitRate) * encControl.PayloadSize_ms) / 1000)
				if 0 > 10000 {
					if int(psEnc.NBitsExceeded) > 0 {
						psEnc.NBitsExceeded = 0
					} else if int(psEnc.NBitsExceeded) < 10000 {
						psEnc.NBitsExceeded = 10000
					} else {
						psEnc.NBitsExceeded = psEnc.NBitsExceeded
					}
				} else if int(psEnc.NBitsExceeded) > 10000 {
					psEnc.NBitsExceeded = 10000
				} else if int(psEnc.NBitsExceeded) < 0 {
					psEnc.NBitsExceeded = 0
				} else {
					psEnc.NBitsExceeded = psEnc.NBitsExceeded
				}
				speech_act_thr_for_switch_Q8 = int(int32(int64(int32(math.Floor(SPEECH_ACTIVITY_DTX_THRES*(1<<8)+0.5))) + ((int64(int32(math.Floor(((1-SPEECH_ACTIVITY_DTX_THRES)/MAX_BANDWIDTH_SWITCH_DELAY_MS)*(1<<(16+8))+0.5))) * int64(int16(psEnc.TimeSinceSwitchAllowed_ms))) >> 16)))
				if psEnc.State_Fxx[0].SCmn.Speech_activity_Q8 < speech_act_thr_for_switch_Q8 {
					psEnc.AllowBandwidthSwitch = 1
					psEnc.TimeSinceSwitchAllowed_ms = 0
				} else {
					psEnc.AllowBandwidthSwitch = 0
					psEnc.TimeSinceSwitchAllowed_ms += encControl.PayloadSize_ms
				}
			}
			if nSamplesIn == 0 {
				break
			}
		} else {
			break
		}
		curr_block++
	}
	psEnc.NPrevChannelsInternal = int(encControl.NChannelsInternal)
	encControl.AllowBandwidthSwitch = psEnc.AllowBandwidthSwitch
	encControl.InWBmodeWithoutVariableLP = int(libc.BoolToInt(psEnc.State_Fxx[0].SCmn.Fs_kHz == 16 && psEnc.State_Fxx[0].SCmn.SLP.Mode == 0))
	encControl.InternalSampleRate = int32(int(int32(int16(psEnc.State_Fxx[0].SCmn.Fs_kHz))) * 1000)
	if encControl.ToMono != 0 {
		encControl.StereoWidth_Q14 = 0
	} else {
		encControl.StereoWidth_Q14 = int(psEnc.SStereo.Smth_width_Q14)
	}
	if prefillFlag != 0 {
		encControl.PayloadSize_ms = tmp_payloadSize_ms
		encControl.Complexity = tmp_complexity
		for n = 0; n < int(encControl.NChannelsInternal); n++ {
			psEnc.State_Fxx[n].SCmn.Controlled_since_last_payload = 0
			psEnc.State_Fxx[n].SCmn.PrefillFlag = 0
		}
	}
	encControl.SignalType = int(psEnc.State_Fxx[0].SCmn.Indices.SignalType)
	encControl.Offset = int(silk_Quantization_Offsets_Q10[int(psEnc.State_Fxx[0].SCmn.Indices.SignalType)>>1][psEnc.State_Fxx[0].SCmn.Indices.QuantOffsetType])
	return ret
}
