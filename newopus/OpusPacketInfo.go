package opus

type OpusPacketInfo struct {
	TOCByte       byte
	Frames        [][]byte
	PayloadOffset int
}

func NewOpusPacketInfo(toc byte, frames [][]byte, payloadOffset int) *OpusPacketInfo {
	return &OpusPacketInfo{
		TOCByte:       toc,
		Frames:        frames,
		PayloadOffset: payloadOffset,
	}
}

func ParseOpusPacket(packet []byte, packet_offset, len int) (*OpusPacketInfo, error) {
	numFrames := GetNumFrames(packet, packet_offset, len)
	if numFrames < 0 {
		return nil, &OpusError{Code: numFrames}
	}

	var out_toc byte
	frames := make([][]byte, numFrames)
	sizes := make([]int16, numFrames)
	var payload_offset, packet_offset_out int

	errCode := opus_packet_parse_impl(packet, packet_offset, len, 0, &out_toc, frames, sizes, &payload_offset, &packet_offset_out)
	if errCode < 0 {
		return nil, &OpusError{Code: errCode}
	}

	copiedFrames := make([][]byte, len(frames))
	for i := range frames {
		copiedFrames[i] = make([]byte, len(frames[i]))
		copy(copiedFrames[i], frames[i])
	}

	return NewOpusPacketInfo(out_toc, copiedFrames, payload_offset), nil
}

func GetNumSamplesPerFrame(packet []byte, packet_offset, Fs int) int {
	var audiosize int
	if (packet[packet_offset] & 0x80) != 0 {
		audiosize = int((packet[packet_offset] >> 3) & 0x3)
		audiosize = (Fs << audiosize) / 400
	} else if (packet[packet_offset] & 0x60) == 0x60 {
		if (packet[packet_offset] & 0x08) != 0 {
			audiosize = Fs / 50
		} else {
			audiosize = Fs / 100
		}
	} else {
		audiosize = int((packet[packet_offset] >> 3) & 0x3)
		if audiosize == 3 {
			audiosize = Fs * 60 / 1000
		} else {
			audiosize = (Fs << audiosize) / 100
		}
	}
	return audiosize
}

func GetNumEncodedChannels(packet []byte, packet_offset int) int {
	if (packet[packet_offset] & 0x4) != 0 {
		return 2
	}
	return 1
}

func GetNumFrames(packet []byte, packet_offset, len int) int {
	if len < 1 {
		return OPUS_BAD_ARG
	}
	count := packet[packet_offset] & 0x3
	if count == 0 {
		return 1
	} else if count != 3 {
		return 2
	} else if len < 2 {
		return OPUS_INVALID_PACKET
	} else {
		return int(packet[packet_offset+1] & 0x3F)
	}
}

func GetNumSamples(packet []byte, packet_offset, len, Fs int) int {
	count := GetNumFrames(packet, packet_offset, len)
	if count < 0 {
		return count
	}

	samples := count * GetNumSamplesPerFrame(packet, packet_offset, Fs)
	if samples*25 > Fs*3 {
		return OPUS_INVALID_PACKET
	}
	return samples
}

func GetNumSamplesDecoder(dec *OpusDecoder, packet []byte, packet_offset, len int) int {
	return GetNumSamples(packet, packet_offset, len, dec.Fs)
}

func GetEncoderMode(packet []byte, packet_offset int) OpusMode {
	if (packet[packet_offset] & 0x80) != 0 {
		return MODE_CELT_ONLY
	} else if (packet[packet_offset] & 0x60) == 0x60 {
		return MODE_HYBRID
	}
	return MODE_SILK_ONLY
}

func encode_size(size int, data []byte, data_ptr int) int {
	if size < 252 {
		data[data_ptr] = byte(size)
		return 1
	} else {
		dp1 := 252 + (size & 0x3)
		data[data_ptr] = byte(dp1)
		data[data_ptr+1] = byte((size - dp1) >> 2)
		return 2
	}
}

func parse_size(data []byte, data_ptr, len int, size *int16) int {
	if len < 1 {
		*size = -1
		return -1
	} else if int(data[data_ptr]) < 252 {
		*size = int16(data[data_ptr])
		return 1
	} else if len < 2 {
		*size = -1
		return -1
	} else {
		*size = int16(4*int(data[data_ptr+1]) + int(data[data_ptr]))
		return 2
	}
}

func opus_packet_parse_impl(data []byte, data_ptr, len, self_delimited int, out_toc *byte, frames [][]byte, sizes []int16, payload_offset *int, packet_offset *int) int {
	var i, bytes int
	var count, cbr int
	var toc byte
	var ch, framesize, last_size, pad int
	data0 := data_ptr

	*out_toc = 0
	*payload_offset = 0
	*packet_offset = 0

	if sizes == nil || len < 0 {
		return OPUS_BAD_ARG
	}
	if len == 0 {
		return OPUS_INVALID_PACKET
	}

	framesize = GetNumSamplesPerFrame(data, data_ptr, 48000)

	cbr = 0
	toc = data[data_ptr]
	data_ptr++
	len--
	last_size = len

	switch toc & 0x3 {
	case 0:
		count = 1
	case 1:
		count = 2
		cbr = 1
		if self_delimited == 0 {
			if len&0x1 != 0 {
				return OPUS_INVALID_PACKET
			}
			last_size = len / 2
			sizes[0] = int16(last_size)
		}
	case 2:
		count = 2
		var size_val int16
		bytes = parse_size(data, data_ptr, len, &size_val)
		sizes[0] = size_val
		len -= bytes
		if sizes[0] < 0 || int(sizes[0]) > len {
			return OPUS_INVALID_PACKET
		}
		data_ptr += bytes
		last_size = len - int(sizes[0])
	default:
		if len < 1 {
			return OPUS_INVALID_PACKET
		}
		ch = int(data[data_ptr])
		data_ptr++
		count = ch & 0x3F
		if count <= 0 || framesize*count > 5760 {
			return OPUS_INVALID_PACKET
		}
		len--
		if (ch & 0x40) != 0 {
			var p int
			for {
				if len <= 0 {
					return OPUS_INVALID_PACKET
				}
				p = int(data[data_ptr])
				data_ptr++
				len--
				tmp := p
				if p == 255 {
					tmp = 254
				}
				len -= tmp
				pad += tmp
				if p != 255 {
					break
				}
			}
		}
		if len < 0 {
			return OPUS_INVALID_PACKET
		}
		if (ch & 0x80) == 0 {
			cbr = 1
		}
		if cbr == 0 {
			last_size = len
			for i = 0; i < count-1; i++ {
				var size_val int16
				bytes = parse_size(data, data_ptr, len, &size_val)
				sizes[i] = size_val
				len -= bytes
				if sizes[i] < 0 || int(sizes[i]) > len {
					return OPUS_INVALID_PACKET
				}
				data_ptr += bytes
				last_size -= bytes + int(sizes[i])
			}
			if last_size < 0 {
				return OPUS_INVALID_PACKET
			}
		} else if self_delimited == 0 {
			last_size = len / count
			if last_size*count != len {
				return OPUS_INVALID_PACKET
			}
			for i = 0; i < count-1; i++ {
				sizes[i] = int16(last_size)
			}
		}
	}

	if self_delimited != 0 {
		var size_val int16
		bytes = parse_size(data, data_ptr, len, &size_val)
		sizes[count-1] = size_val
		len -= bytes
		if sizes[count-1] < 0 || int(sizes[count-1]) > len {
			return OPUS_INVALID_PACKET
		}
		data_ptr += bytes
		if cbr != 0 {
			if int(sizes[count-1])*count > len {
				return OPUS_INVALID_PACKET
			}
			for i = 0; i < count-1; i++ {
				sizes[i] = sizes[count-1]
			}
		} else if bytes+int(sizes[count-1]) > last_size {
			return OPUS_INVALID_PACKET
		}
	} else {
		if last_size > 1275 {
			return OPUS_INVALID_PACKET
		}
		sizes[count-1] = int16(last_size)
	}

	*payload_offset = data_ptr - data0

	for i = 0; i < count; i++ {
		if frames != nil {
			frames[i] = make([]byte, sizes[i])
			copy(frames[i], data[data_ptr:data_ptr+int(sizes[i])])
		}
		data_ptr += int(sizes[i])
	}

	*packet_offset = pad + (data_ptr - data0)
	*out_toc = toc

	return count
}
