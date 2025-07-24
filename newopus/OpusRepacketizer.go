package opus

type OpusRepacketizer struct {
	toc       byte
	nb_frames int
	frames    [48][]byte
	len       [48]int
	framesize int
}

func (this *OpusRepacketizer) Reset() {
	this.nb_frames = 0
}

func NewOpusRepacketizer() *OpusRepacketizer {
	rp := &OpusRepacketizer{}
	rp.Reset()
	return rp
}

func (this *OpusRepacketizer) opus_repacketizer_cat_impl(data []byte, data_ptr int, len_val int, self_delimited int) int {
	dummy_toc := BoxedValueByte{0}
	dummy_offset := BoxedValueInt{0}
	if len_val < 1 {
		return OPUS_INVALID_PACKET
	}

	if this.nb_frames == 0 {
		this.toc = data[data_ptr]
		this.framesize = getNumSamplesPerFrame(data, data_ptr, 8000)
	} else if (this.toc & 0xFC) != (data[data_ptr] & 0xFC) {
		return OPUS_INVALID_PACKET
	}

	curr_nb_frames := getNumFrames(data, data_ptr, len_val)
	if curr_nb_frames < 1 {
		return OPUS_INVALID_PACKET
	}

	if (curr_nb_frames+this.nb_frames)*this.framesize > 960 {
		return OPUS_INVALID_PACKET
	}

	ret := opus_packet_parse_impl(data, data_ptr, len_val, self_delimited, &dummy_toc, this.frames[:], this.nb_frames, this.len[:], this.nb_frames, &dummy_offset, &dummy_offset)
	if ret < 1 {
		return ret
	}

	this.nb_frames += curr_nb_frames
	return OPUS_OK
}

func (this *OpusRepacketizer) addPacket(data []byte, data_offset int, len_val int) int {
	return this.opus_repacketizer_cat_impl(data, data_offset, len_val, 0)
}

func (this *OpusRepacketizer) getNumFrames() int {
	return this.nb_frames
}

func (this *OpusRepacketizer) opus_repacketizer_out_range_impl(begin int, end int, data []byte, data_ptr int, maxlen int, self_delimited int, pad int) int {
	if begin < 0 || begin >= end || end > this.nb_frames {
		return OPUS_BAD_ARG
	}
	count := end - begin

	tot_size := 0
	if self_delimited != 0 {
		tot_size = 1
		if this.len[count-1] >= 252 {
			tot_size += 1
		}
	}

	ptr := data_ptr
	if count == 1 {
		tot_size += this.len[0] + 1
		if tot_size > maxlen {
			return OPUS_BUFFER_TOO_SMALL
		}
		data[ptr] = this.toc & 0xFC
		ptr++
	} else if count == 2 {
		if this.len[1] == this.len[0] {
			tot_size += 2*this.len[0] + 1
			if tot_size > maxlen {
				return OPUS_BUFFER_TOO_SMALL
			}
			data[ptr] = (this.toc & 0xFC) | 0x01
			ptr++
		} else {
			tot_size += this.len[0] + this.len[1] + 2
			if this.len[0] >= 252 {
				tot_size += 1
			}
			if tot_size > maxlen {
				return OPUS_BUFFER_TOO_SMALL
			}
			data[ptr] = (this.toc & 0xFC) | 0x02
			ptr++
			ptr += encode_size(this.len[0], data[ptr:], ptr)
		}
	}
	if count > 2 || (pad != 0 && tot_size < maxlen) {
		vbr := 0
		pad_amount := 0
		ptr = data_ptr
		tot_size = 0
		if self_delimited != 0 {
			tot_size = 1
			if this.len[count-1] >= 252 {
				tot_size += 1
			}
		}

		for i := 1; i < count; i++ {
			if this.len[i] != this.len[0] {
				vbr = 1
				break
			}
		}

		if vbr != 0 {
			tot_size += 2
			for i := 0; i < count-1; i++ {
				tot_size += 1 + this.len[i]
				if this.len[i] >= 252 {
					tot_size += 1
				}
			}
			tot_size += this.len[count-1]
			if tot_size > maxlen {
				return OPUS_BUFFER_TOO_SMALL
			}
			data[ptr] = (this.toc & 0xFC) | 0x03
			ptr++
			data[ptr] = byte(count) | 0x80
			ptr++
		} else {
			tot_size += count*this.len[0] + 2
			if tot_size > maxlen {
				return OPUS_BUFFER_TOO_SMALL
			}
			data[ptr] = (this.toc & 0xFC) | 0x03
			ptr++
			data[ptr] = byte(count)
			ptr++
		}

		if pad != 0 {
			pad_amount = maxlen - tot_size
			if pad_amount > 0 {
				data[data_ptr+1] |= 0x40
				nb_255s := (pad_amount - 1) / 255
				for i := 0; i < nb_255s; i++ {
					data[ptr] = 255
					ptr++
				}
				data[ptr] = byte(pad_amount - 255*nb_255s - 1)
				ptr++
				tot_size += pad_amount
			}
		}

		if vbr != 0 {
			for i := 0; i < count-1; i++ {
				ptr += encode_size(this.len[i], data[ptr:], ptr)
			}
		}
	}

	if self_delimited != 0 {
		sdlen := encode_size(this.len[count-1], data[ptr:], ptr)
		ptr += sdlen
	}

	for i := begin; i < end; i++ {
		copy(data[ptr:], this.frames[i][:this.len[i]])
		ptr += this.len[i]
	}

	if pad != 0 {
		for i := ptr; i < data_ptr+maxlen; i++ {
			data[i] = 0
		}
	}

	return tot_size
}

func (this *OpusRepacketizer) createPacket(begin int, end int, data []byte, data_offset int, maxlen int) int {
	return this.opus_repacketizer_out_range_impl(begin, end, data, data_offset, maxlen, 0, 0)
}

func (this *OpusRepacketizer) createPacketOut(data []byte, data_offset int, maxlen int) int {
	return this.opus_repacketizer_out_range_impl(0, this.nb_frames, data, data_offset, maxlen, 0, 0)
}

func PadPacket(data []byte, data_offset int, len_val int, new_len int) int {
	if len_val < 1 {
		return OPUS_BAD_ARG
	}
	if len_val == new_len {
		return OPUS_OK
	} else if len_val > new_len {
		return OPUS_BAD_ARG
	}

	rp := NewOpusRepacketizer()
	copy(data[data_offset+new_len-len_val:], data[data_offset:data_offset+len_val])
	rp.addPacket(data, data_offset+new_len-len_val, len_val)
	ret := rp.opus_repacketizer_out_range_impl(0, rp.nb_frames, data, data_offset, new_len, 0, 1)
	if ret > 0 {
		return OPUS_OK
	}
	return ret
}

func UnpadPacket(data []byte, data_offset int, len_val int) int {
	if len_val < 1 {
		return OPUS_BAD_ARG
	}

	rp := NewOpusRepacketizer()
	ret := rp.addPacket(data, data_offset, len_val)
	if ret < 0 {
		return ret
	}
	ret = rp.opus_repacketizer_out_range_impl(0, rp.nb_frames, data, data_offset, len_val, 0, 0)
	return ret
}

func PadMultistreamPacket(data []byte, data_offset int, len_val int, new_len int, nb_streams int) int {
	if len_val < 1 {
		return OPUS_BAD_ARG
	}
	if len_val == new_len {
		return OPUS_OK
	} else if len_val > new_len {
		return OPUS_BAD_ARG
	}

	amount := new_len - len_val
	dummy_toc := BoxedValueByte{0}
	size := [48]int{}
	packet_offset := BoxedValueInt{0}
	dummy_offset := BoxedValueInt{0}

	for s := 0; s < nb_streams-1; s++ {
		if len_val <= 0 {
			return OPUS_INVALID_PACKET
		}
		count := opus_packet_parse_impl(data, data_offset, len_val, 1, &dummy_toc, nil, 0, size[:], 0, &dummy_offset, &packet_offset)
		if count < 0 {
			return count
		}
		data_offset += int(packet_offset.Val)
		len_val -= int(packet_offset.Val)
	}
	return PadPacket(data, data_offset, len_val, len_val+amount)
}

func UnpadMultistreamPacket(data []byte, data_offset int, len_val int, nb_streams int) int {
	if len_val < 1 {
		return OPUS_BAD_ARG
	}

	dst := data_offset
	dst_len := 0
	dummy_toc := BoxedValueByte{0}
	size := [48]int{}
	packet_offset := BoxedValueInt{0}
	dummy_offset := BoxedValueInt{0}

	for s := 0; s < nb_streams; s++ {
		self_delimited := 0
		if s != nb_streams-1 {
			self_delimited = 1
		}
		if len_val <= 0 {
			return OPUS_INVALID_PACKET
		}
		rp := NewOpusRepacketizer()
		count := opus_packet_parse_impl(data, data_offset, len_val, self_delimited, &dummy_toc, nil, 0, size[:], 0, &dummy_offset, &packet_offset)
		if count < 0 {
			return count
		}
		ret := rp.opus_repacketizer_cat_impl(data, data_offset, int(packet_offset.Val), self_delimited)
		if ret < 0 {
			return ret
		}
		ret = rp.opus_repacketizer_out_range_impl(0, rp.nb_frames, data, dst, len_val, self_delimited, 0)
		if ret < 0 {
			return ret
		}
		dst_len += ret
		dst += ret
		data_offset += int(packet_offset.Val)
		len_val -= int(packet_offset.Val)
	}
	return dst_len
}

func getNumSamplesPerFrame(data []byte, data_ptr int, fs int) int {
	if data_ptr >= len(data) {
		return 0
	}
	audiosize := 0
	switch (data[data_ptr] >> 3) & 0x03 {
	case 0:
		audiosize = 960
	case 1:
		audiosize = 480
	case 2:
		audiosize = 240
	case 3:
		audiosize = 120
	}
	return audiosize * fs / 8000
}

func getNumFrames(data []byte, data_ptr int, len_val int) int {
	if len_val < 1 {
		return OPUS_INVALID_PACKET
	}
	toc := data[data_ptr]
	if (toc & 0x80) != 0 {
		count := int(toc & 0x3F)
		if count == 0 || count > 48 {
			return OPUS_INVALID_PACKET
		}
		return count
	} else if (toc & 0x60) == 0x60 {
		count := 1
		if (toc & 0x08) != 0 {
			count++
		}
		return count
	} else {
		return 1
	}
}
