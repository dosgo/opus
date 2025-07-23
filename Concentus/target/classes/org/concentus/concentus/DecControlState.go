// Package concentus provides SILK codec functionality.
package concentus

// DecControlState represents the structure for controlling decoder operation
// and reading decoder status.
type DecControlState struct {
	// NChannelsAPI: Number of channels; 1/2
	NChannelsAPI int

	// NChannelsInternal: Number of channels; 1/2
	NChannelsInternal int

	// APISampleRate: Output signal sampling rate in Hertz;
	// 8000/12000/16000/24000/32000/44100/48000
	APISampleRate int

	// InternalSampleRate: Internal sampling rate used, in Hertz; 8000/12000/16000
	InternalSampleRate int

	// PayloadSizeMs: Number of samples per packet in milliseconds; 10/20/40/60
	PayloadSizeMs int

	// PrevPitchLag: Pitch lag of previous frame (0 if unvoiced), measured in
	// samples at 48 kHz
	PrevPitchLag int
}

// Reset resets all fields of DecControlState to their zero values.
func (d *DecControlState) Reset() {
	*d = DecControlState{}
}
