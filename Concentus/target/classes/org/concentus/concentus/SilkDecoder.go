// Copyright (c) 2006-2011 Skype Limited. All Rights Reserved
// Ported to Go from Java by [Your Name]
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// - Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//
// - Redistributions in binary form must reproduce the above copyright
// notice, this list of conditions and the following disclaimer in the
// documentation and/or other materials provided with the distribution.
//
// - Neither the name of Internet Society, IETF or IETF Trust, nor the
// names of specific contributors, may be used to endorse or promote
// products derived from this software without specific prior written
// permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// ``AS IS'' AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER
// OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL,
// EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
// PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF
// LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
// NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package concentus

// SilkDecoder represents a decoder super struct
type SilkDecoder struct {
	// channel_state contains the decoder state for each channel
	channelState []*SilkChannelDecoder

	// sStereo contains the stereo decoding state
	sStereo *StereoDecodeState

	// nChannelsAPI tracks the number of channels requested by the API
	nChannelsAPI int

	// nChannelsInternal tracks the actual number of channels being decoded
	nChannelsInternal int

	// prevDecodeOnlyMiddle indicates if previous decode was middle-only
	prevDecodeOnlyMiddle int
}

// NewSilkDecoder creates and initializes a new SilkDecoder instance
func NewSilkDecoder() *SilkDecoder {
	decoder := &SilkDecoder{
		channelState: make([]*SilkChannelDecoder, DecoderNumChannels),
		sStereo:      NewStereoDecodeState(),
	}

	// Initialize each channel decoder
	for c := 0; c < DecoderNumChannels; c++ {
		decoder.channelState[c] = NewSilkChannelDecoder()
	}

	return decoder
}

// Reset resets the decoder state to initial conditions
func (d *SilkDecoder) Reset() {
	// Reset each channel decoder
	for _, channel := range d.channelState {
		channel.Reset()
	}

	// Reset stereo state
	d.sStereo.Reset()

	// Reset counters and flags
	d.nChannelsAPI = 0
	d.nChannelsInternal = 0
	d.prevDecodeOnlyMiddle = 0
}
