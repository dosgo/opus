package concentus

import (
	"math"
)

// SilkEncoder represents the encoder super struct for SILK audio codec.
// It maintains the state for multiple audio channels and stereo processing.
type SilkEncoder struct {
	stateFxx                 []*SilkChannelEncoder // Channel encoder states
	sStereo                  StereoEncodeState     // Stereo encoding state
	nBitsUsedLBRR            int                   // Number of bits used for LBRR
	nBitsExceeded            int                   // Number of bits exceeded
	nChannelsAPI             int                   // Number of channels from API
	nChannelsInternal        int                   // Internal number of channels
	nPrevChannelsInternal    int                   // Previous internal channel count
	timeSinceSwitchAllowedMs int                   // Time since last allowed bandwidth switch
	allowBandwidthSwitch     int                   // Flag indicating if bandwidth switch is allowed
	prevDecodeOnlyMiddle     int                   // Previous decode only middle flag
}

// NewSilkEncoder creates and initializes a new SilkEncoder instance.
// This is the idiomatic Go constructor pattern.
func NewSilkEncoder() *SilkEncoder {
	enc := &SilkEncoder{
		stateFxx: make([]*SilkChannelEncoder, EncoderNumChannels),
	}

	// Initialize each channel encoder
	for c := range enc.stateFxx {
		enc.stateFxx[c] = NewSilkChannelEncoder()
	}

	return enc
}

// Reset resets the encoder state to default values.
// In Go, we use receiver methods instead of Java's class methods.
func (enc *SilkEncoder) Reset() {
	// Reset each channel encoder
	for _, ch := range enc.stateFxx {
		ch.Reset()
	}

	enc.sStereo.Reset()
	enc.nBitsUsedLBRR = 0
	enc.nBitsExceeded = 0
	enc.nChannelsAPI = 0
	enc.nChannelsInternal = 0
	enc.nPrevChannelsInternal = 0
	enc.timeSinceSwitchAllowedMs = 0
	enc.allowBandwidthSwitch = 0
	enc.prevDecodeOnlyMiddle = 0
}

// InitEncoder initializes a Silk channel encoder state.
// In Go, we typically don't use static methods but rather package-level functions.
// We also return errors explicitly rather than using numeric return codes.
func InitEncoder(psEnc *SilkChannelEncoder) error {
	// Clear the entire encoder state
	psEnc.Reset()

	// Convert min cutoff frequency from Hz to Q15 format
	minCutoffHz := VariableHPMinCutoffHz
	minCutoffQ15 := int32(math.Round(minCutoffHz * float64(1<<16)))
	logMinCutoff := Lin2log(minCutoffQ15)

	// Initialize smoothing filters (left shift by 8 is equivalent to multiplication by 256)
	psEnc.variableHPSmth1Q15 = LSHIFT(logMinCutoff-(16<<7), 8)
	psEnc.variableHPSmth2Q15 = psEnc.variableHPSmth1Q15

	// Used to deactivate LSF interpolation, pitch prediction
	psEnc.firstFrameAfterReset = 1

	// Initialize Silk VAD
	if err := VADInit(psEnc.sVAD); err != nil {
		return err
	}

	return nil
}
