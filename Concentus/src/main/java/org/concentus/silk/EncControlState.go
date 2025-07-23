package silk

// EncControlState represents the structure for controlling encoder operation.
// The Go version uses proper Go naming conventions and idioms.
type EncControlState struct {
	// Input parameters
	NChannelsAPI              int // Number of channels; 1/2
	NChannelsInternal         int // Number of channels; 1/2
	APISampleRate             int // Input signal sampling rate in Hz; 8000/12000/16000/24000/32000/44100/48000
	MaxInternalSampleRate     int // Maximum internal sampling rate in Hz; 8000/12000/16000
	MinInternalSampleRate     int // Minimum internal sampling rate in Hz; 8000/12000/16000
	DesiredInternalSampleRate int // Soft request for sampling rate in Hz; 8000/12000/16000
	PayloadSizeMs             int // Number of samples per packet in milliseconds; 10/20/40/60
	BitRate                   int // Bitrate during active speech in bits/second; internally limited
	PacketLossPercentage      int // Uplink packet loss in percent (0-100)
	Complexity                int // Complexity mode; 0 is lowest, 10 is highest complexity
	UseInBandFEC              int // Flag to enable in-band Forward Error Correction (FEC); 0/1
	UseDTX                    int // Flag to enable discontinuous transmission (DTX); 0/1
	UseCBR                    int // Flag to use constant bitrate; 0/1
	MaxBits                   int // Maximum number of bits allowed for the frame
	ToMono                    int // Causes a smooth downmix to mono; 0/1
	OpusCanSwitch             int // Opus encoder is allowing us to switch bandwidth; 0/1
	ReducedDependency         int // Make frames as independent as possible; 0/1

	// Output parameters
	InternalSampleRate        int // Internal sampling rate used, in Hz; 8000/12000/16000
	AllowBandwidthSwitch      int // Flag that bandwidth switching is allowed; 0/1
	InWBmodeWithoutVariableLP int // Flag that SILK runs in WB mode without variable LP filter; 0/1
	StereoWidthQ14            int // Stereo width
	SwitchReady               int // Tells the Opus encoder we're ready to switch; 0/1
}

// Reset resets all fields to their zero values.
// In Go, we can simply create a new instance, but this method is provided
// for compatibility with the original API.
func (e *EncControlState) Reset() {
	*e = EncControlState{} // Takes advantage of Go's zero value initialization
}

// CheckControlInput validates the encoder control parameters and returns an error if any are invalid.
// The Go version uses more idiomatic error handling and validation patterns.
func (e *EncControlState) CheckControlInput() error {
	// Validate sample rates
	validAPIRates := map[int]bool{
		8000:  true,
		12000: true,
		16000: true,
		24000: true,
		32000: true,
		44100: true,
		48000: true,
	}
	validInternalRates := map[int]bool{
		8000:  true,
		12000: true,
		16000: true,
	}

	if !validAPIRates[e.APISampleRate] ||
		!validInternalRates[e.DesiredInternalSampleRate] ||
		!validInternalRates[e.MaxInternalSampleRate] ||
		!validInternalRates[e.MinInternalSampleRate] {
		return ErrEncFsNotSupported
	}

	if e.MinInternalSampleRate > e.DesiredInternalSampleRate ||
		e.MaxInternalSampleRate < e.DesiredInternalSampleRate ||
		e.MinInternalSampleRate > e.MaxInternalSampleRate {
		return ErrEncFsNotSupported
	}

	// Validate payload size
	validPayloadSizes := map[int]bool{
		10: true,
		20: true,
		40: true,
		60: true,
	}
	if !validPayloadSizes[e.PayloadSizeMs] {
		return ErrEncPacketSizeNotSupported
	}

	// Validate packet loss percentage
	if e.PacketLossPercentage < 0 || e.PacketLossPercentage > 100 {
		return ErrEncInvalidLossRate
	}

	// Validate binary flags
	if e.UseDTX < 0 || e.UseDTX > 1 ||
		e.UseCBR < 0 || e.UseCBR > 1 ||
		e.UseInBandFEC < 0 || e.UseInBandFEC > 1 {
		return ErrEncInvalidSetting
	}

	// Validate channel counts
	if e.NChannelsAPI < 1 || e.NChannelsAPI > EncoderNumChannels ||
		e.NChannelsInternal < 1 || e.NChannelsInternal > EncoderNumChannels ||
		e.NChannelsInternal > e.NChannelsAPI {
		return ErrEncInvalidNumberOfChannels
	}

	// Validate complexity
	if e.Complexity < 0 || e.Complexity > 10 {
		return ErrEncInvalidComplexitySetting
	}

	return nil
}
