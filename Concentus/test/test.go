package main

import (
	"concentus/opus"
	"fmt"
	"io"
	"os"
)

func main() {

	encoder, err := opus.NewOpusEncoder(48000, 2, opus.OPUS_APPLICATION_AUDIO)
	encoder.SetBitrate(96000)
	encoder.SetForceMode(opus.MODE_CELT_ONLY)
	encoder.SetSignalType(opus.OPUS_SIGNAL_MUSIC)
	encoder.SetComplexity(0)

	decoder, err := opus.NewOpusDecoder(48000, 2)
	fileIn, err := os.Open("48Khz Stereo.raw")
	if err != nil {
		panic(err)
	}
	defer fileIn.Close()

	var packetSamples = 960
	inBuf := make([]byte, packetSamples*2*2)
	data_packet := make([]byte, 1275)

	for {
		_, err := io.ReadFull(fileIn, inBuf)
		if err != nil {
			break
		}
		pcm, _ := BytesToShorts(inBuf, 0, len(inBuf))
		fmt.Printf("pcm %+v\r\n", len(pcm))
		bytesEncoded, err := encoder.Encode(pcm, 0, packetSamples, data_packet, 0, 1275)
		//System.out.println(bytesEncoded + " bytes encoded");
		fmt.Printf("bytesEncoded:%d data_packet:%+v\r\n", bytesEncoded, data_packet)
		if bytesEncoded > 0 {
			break
		}

		samplesDecoded, err := decoder.Decode(data_packet, 0, bytesEncoded, pcm, 0, packetSamples, false)
		fmt.Printf("samplesDecoded:%d samplesDecoded:%+v\r\n", samplesDecoded, samplesDecoded)
		//System.out.println(samplesDecoded + " samples decoded");
		// bytesOut = ShortsToBytes(pcm);
		// fileOut.write(bytesOut, 0, bytesOut.length);

	}

}

func BytesToShorts(input []byte, offset, length int) ([]int16, error) {
	// 1. 输入验证
	totalBytes := offset + length
	if totalBytes > len(input) {
		return nil, fmt.Errorf("offset + length exceeds input length (%d > %d)", totalBytes, len(input))
	}
	if length%2 != 0 {
		return nil, fmt.Errorf("length must be multiple of 2, got %d", length)
	}
	if offset < 0 {
		return nil, fmt.Errorf("offset cannot be negative")
	}

	// 2. 创建结果数组 (Java中的short对应Go的int16)
	processedValues := make([]int16, length/2)

	// 3. 按照Java原始算法逐字节处理
	for c := 0; c < len(processedValues); c++ {
		// 计算字节位置 - 与Java完全一致
		posLow := (c * 2) + offset
		posHigh := (c * 2) + 1 + offset

		// 低字节 (无符号处理)
		a := int16(input[posLow] & 0xFF)

		// 高字节 (带符号扩展)
		b := int16(input[posHigh]) << 8

		// 组合值 (保留位运算)
		processedValues[c] = a | b
	}

	return processedValues, nil
}
