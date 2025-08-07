package main

import (
	"concentus/opus"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {

	encoder, err := opus.NewOpusEncoder(48000, 2, opus.OPUS_APPLICATION_AUDIO)
	encoder.SetBitrate(96000)
	encoder.SetForceMode(opus.MODE_SILK_ONLY)
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
	start := time.Now().UnixNano()
	i := 0
	for {
		_, err := io.ReadFull(fileIn, inBuf)
		if err != nil {
			break
		}
		/*
			if i < 3 {
				i++
				continue
			}*/
		if i > 4 {
			break
		}
		pcm, _ := BytesToShorts(inBuf, 0, len(inBuf))

		fmt.Printf("imput md5:%s\r\n", ByteSliceToMD5(inBuf))
		//encoder.PrintAllFields()
		bytesEncoded, err := encoder.Encode(pcm, 0, packetSamples, data_packet, 0, 1275)

		//encoder.ResetState()

		if i == 4 {
			fmt.Printf("data_packet:%s\r\n", formatSignedBytes(data_packet))
		}
		//fmt.Printf("encoder:%s\r\n", encoder.ResetState())
		//break
		fmt.Printf("pcmlen:%d\r\n", len(inBuf))
		fmt.Printf("bytesEncoded:%d data_packet:%s\r\n", bytesEncoded, ByteSliceToMD5(data_packet))

		_, err = decoder.Decode(data_packet, 0, bytesEncoded, pcm, 0, packetSamples, false)
		//fmt.Printf("%d samples decoded\r\n", samplesDecoded)
		if err == nil {
			fmt.Printf("pcm:%s\r\n", IntSliceToMD5(pcm))
		}
		i++
	}
	elapsed := time.Duration(time.Now().UnixNano() - start)
	fmt.Printf("Time was: %+v ms\n", float64(elapsed)/1e6)

}
func formatSignedBytes(data []byte) string {
	var builder strings.Builder
	builder.WriteString("[")
	for i, b := range data {
		// 转换为有符号整数
		signed := int8(b)

		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(strconv.Itoa(int(signed)))
	}
	builder.WriteString("]")
	return builder.String()
}
func ByteSliceToMD5(slice []byte) string {
	hasher := md5.New()
	hasher.Write(slice)
	hash := hasher.Sum(nil)
	return hex.EncodeToString(hash)
}

func IntSliceToMD5(slice []int16) string {
	hasher := md5.New()
	buf := make([]byte, 2) // 用于每个整数的缓冲区

	for _, num := range slice {
		// 将int转换为uint32（保留位模式）
		u := uint16(num)
		binary.BigEndian.PutUint16(buf, u)
		hasher.Write(buf)
	}

	hash := hasher.Sum(nil)
	return hex.EncodeToString(hash)
}

func bytesToCSV(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(strconv.Itoa(int(data[0])))

	for i := 1; i < len(data); i++ {
		sb.WriteString(",")
		sb.WriteString(strconv.Itoa(int(data[i])))
	}

	return sb.String()
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
