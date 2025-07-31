package main

import (
	"concentus/opus"
	"fmt"
)

func main() {

	decoder, err := opus.NewOpusDecoder(48000, 2)
	if err != nil {
		panic(err)
	}
	data_packet := make([]byte, 1000)
	pcm := make([]int16, 1000)
	n, err := decoder.Decode(data_packet, 0, len(data_packet), pcm, 0, len(pcm), false)
	//System.out.println(samplesDecoded + " samples decoded");
	fmt.Printf("%d samples decoded", n)
}
