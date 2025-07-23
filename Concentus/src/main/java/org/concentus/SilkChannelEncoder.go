
package silk

import (
	"math"
)

// ChannelEncoder represents the encoder state for a Silk channel
type ChannelEncoder struct {
	InHPState                  [2]int32 // High pass filter state
	VariableHPSmth1Q15         int32    // State of first smoother
	VariableHPSmth2Q15         int32    // State of second smoother
	SLP                       LPState  // Low pass filter state
	SVAD                      VADState // Voice activity detector state
	SNSQ                      NSQState // Noise Shape Quantizer State
	PrevNLSFqQ15              [MAX_LPC_ORDER]int16 // Previously quantized NLSF vector
	SpeechActivityQ8          int32    // Speech activity
	AllowBandwidthSwitch      int32    // Flag indicating bandwidth switching is allowed
	LBRRPrevLastGainIndex     int8     // Previous gain index for LBRR
	PrevSignalType            int8     // Previous signal type
	PrevLag                   int32    // Previous pitch lag
	PitchLPCWinLength         int32    // Pitch LPC window length
	MaxPitchLag               int32    // Highest possible pitch lag (samples)
	APIFsHz                   int32    // API sampling frequency (Hz)
	PrevAPIFsHz               int32    // Previous API sampling frequency (Hz)
	MaxInternalFsHz           int32    // Maximum internal sampling frequency (Hz)
	MinInternalFsHz           int32    // Minimum internal sampling frequency (Hz)
	DesiredInternalFsHz       int32    // Soft request for sampling frequency (Hz)
	FsKHz                     int32    // Internal sampling frequency (kHz)
	NbSubfr                   int32    // Number of 5 ms subframes in a frame
	FrameLength               int32    // Frame length (samples)
	SubfrLength               int32    // Subframe length (samples)
	LTPMemLength              int32    // Length of LTP memory
	LaPitch                   int32    // Look-ahead for pitch analysis (samples)
	LaShape                   int32    // Look-ahead for noise shape analysis (samples)
	ShapeWinLength            int32    // Window length for noise shape analysis (samples)
	TargetRateBps             int32    // Target bitrate (bps)
	PacketSizeMs              int32    // Number of milliseconds per packet
	PacketLossPerc            int32    // Packet loss rate measured by farend
	FrameCounter              int32    // Frame counter
	Complexity                int32    // Complexity setting
	NStatesDelayedDecision    int32    // Number of states in delayed decision quantization
	UseInterpolatedNLSFs      int32    // Flag for using NLSF interpolation
	ShapingLPCOrder           int32    // Filter order for noise shaping filters
	PredictLPCOrder           int32    // Filter order for prediction filters
	PitchEstimationComplexity int32    // Complexity level for pitch estimator
	PitchEstimationLPCOrder   int32    // Whitening filter order for pitch estimator
	PitchEstimationThresholdQ16 int32  // Threshold for pitch estimator
	LTPQuantLowComplexity     int32    // Flag for low complexity LTP quantization
	MuLTPQ9                   int32    // Rate-distortion tradeoff in LTP quantization
	SumLogGainQ7              int32    // Cumulative max prediction gain
	NLSFMSVQSurvivors         int32    // Number of survivors in NLSF MSVQ
	FirstFrameAfterReset      int32    // Flag for first frame after reset
	ControlledSinceLastPayload int32   // Flag for codec_control running once per packet
	WarpingQ16                int32    // Warping parameter for warped noise shaping
	UseCBR                    int32    // Flag to enable constant bitrate
	PrefillFlag               int32    // Flag for buffer prefill only
	PitchLagLowBitsICDF       []int16  // iCDF table for low bits of pitch lag index
	PitchContourICDF          []int16  // iCDF table for pitch contour index
	PsNLSFCB                  *NLSFCodebook // Pointer to NLSF codebook
	InputQualityBandsQ15      [VAD_N_BANDS]int32 // Input quality bands
	InputTiltQ15              int32    // Input tilt
	SNRdBQ7                   int32    // Quality setting

	VADFlags                 [MAX_FRAMES_PER_PACKET]int8 // VAD flags
	LBRRFlag                 int8     // LBRR flag
	LBRRFlags                [MAX_FRAMES_PER_PACKET]int32 // LBRR flags

	Indices                  SideInfoIndices // Side info indices
	Pulses                   [MAX_FRAME_LENGTH]int8 // Pulses

	// Input/output buffering
	InputBuf                 [MAX_FRAME_LENGTH + 2]int16 // Buffer containing input signal
	InputBufIx               int32    // Input buffer index
	NFramesPerPacket         int32    // Frames per packet
	NFramesEncoded           int32    // Frames analyzed in current packet

	NChannelsAPI             int32    // Number of API channels
	NChannelsInternal        int32    // Number of internal channels
	ChannelNb                int32    // Channel number

	// Parameters for LTP scaling control
	FramesSinceOnset         int32    // Frames since onset

	// Entropy coding
	EcPrevSignalType         int32    // Previous signal type for entropy coding
	EcPrevLagIndex           int16    // Previous lag index for entropy coding

	ResamplerState           ResamplerState // Resampler state

	// DTX
	UseDTX                   int32    // Flag to enable DTX
	InDTX                    int32    // Flag signaling DTX period
	NoSpeechCounter          int32    // Consecutive nonactive frames counter

	// LBRR data
	UseInBandFEC             int32    // API setting for in-band FEC
	LBRREnabled              int32    // LBRR enabled flag
	LBRRGainIncreases        int32    // Gains increment for LBRR frames
	IndicesLBRR              [MAX_FRAMES_PER_PACKET]SideInfoIndices // LBRR indices
	PulsesLBRR               [MAX_FRAMES_PER_PACKET][MAX_FRAME_LENGTH]int8 // LBRR pulses

	// Noise shaping state
	SShape                   ShapeState // Shape state

	// Prefilter state
	SPrefilt                 PrefilterState // Prefilter state

	// Buffer for pitch and noise shape analysis
	XBuf                     [2*MAX_FRAME_LENGTH + LA_SHAPE_MAX]int16 // Analysis buffer

	// Normalized correlation from pitch lag estimator
	LTPCorrQ15               int32    // LTP correlation
}

// NewChannelEncoder creates a new SilkChannelEncoder instance
func NewChannelEncoder() *ChannelEncoder {
	enc := &ChannelEncoder{}
	for c := 0; c < MAX_FRAMES_PER_PACKET; c++ {
		enc.IndicesLBRR[c] = NewSideInfoIndices()
	}
	return enc
}

// Reset resets the encoder state
func (enc *ChannelEncoder) Reset() {
	enc.InHPState = [2]int32{}
	enc.VariableHPSmth1Q15 = 0
	enc.VariableHPSmth2Q15 = 0
	enc.SLP.Reset()
	enc.SVAD.Reset()
	enc.SNSQ.Reset()
	enc.PrevNLSFqQ15 = [MAX_LPC_ORDER]int16{}
	enc.SpeechActivityQ8 = 0
	enc.AllowBandwidthSwitch = 0
	enc.LBRRPrevLastGainIndex = 0
	enc.PrevSignalType = 0
	enc.PrevLag = 0
	enc.PitchLPCWinLength = 0
	enc.MaxPitchLag = 0
	enc.APIFsHz = 0
	enc.PrevAPIFsHz = 0
	enc.MaxInternalFsHz = 0
	enc.MinInternalFsHz = 0
	enc.DesiredInternalFsHz = 0
	enc.FsKHz = 0
	enc.NbSubfr = 0
	enc.FrameLength = 0
	enc.SubfrLength = 0
	enc.LTPMemLength = 0
	enc.LaPitch = 0
	enc.LaShape = 0
	enc.ShapeWinLength = 0
	enc.TargetRateBps = 0
	enc.PacketSizeMs = 0
	enc.PacketLossPerc = 0
	enc.FrameCounter = 0
	enc.Complexity = 0
	enc.NStatesDelayedDecision = 0
	enc.UseInterpolatedNLSFs = 0
	enc.ShapingLPCOrder = 0
	enc.PredictLPCOrder = 0
	enc.PitchEstimationComplexity = 0
	enc.PitchEstimationLPCOrder = 0
	enc.PitchEstimationThresholdQ16 = 0
	enc.LTPQuantLowComplexity = 0
	enc.MuLTPQ9 = 0
	enc.SumLogGainQ7 = 0
	enc.NLSFMSVQSurvivors = 0
	enc.FirstFrameAfterReset = 0
	enc.ControlledSinceLastPayload = 0
	enc.WarpingQ16 = 0
	enc.UseCBR = 0
	enc.PrefillFlag = 0
	enc.PitchLagLowBitsICDF = nil
	enc.PitchContourICDF = nil
	enc.PsNLSFCB = nil
	enc.InputQualityBandsQ15 = [VAD_N_BANDS]int32{}
	enc.InputTiltQ15 = 0
	enc.SNRdBQ7 = 0
	enc.VADFlags = [MAX_FRAMES_PER_PACKET]int8{}
	enc.LBRRFlag = 0
	enc.LBRRFlags = [MAX_FRAMES_PER_PACKET]int32{}
	enc.Indices.Reset()
	enc.Pulses = [MAX_FRAME_LENGTH]int8{}
	enc.InputBuf = [MAX_FRAME_LENGTH + 2]int16{}
	enc.InputBufIx = 0
	enc.NFramesPerPacket = 0
	enc.NFramesEncoded = 0
	enc.NChannelsAPI = 0
	enc.NChannelsInternal = 0
	enc.ChannelNb = 0
	enc.FramesSinceOnset = 0
	enc.EcPrevSignalType = 0
	enc.EcPrevLagIndex = 0
	enc.ResamplerState.Reset()
	enc.UseDTX = 0
	enc.InDTX = 0
	enc.NoSpeechCounter = 0
	enc.UseInBandFEC = 0
	enc.LBRREnabled = 0
	enc.LBRRGainIncreases = 0
	for c := 0; c < MAX_FRAMES_PER_PACKET; c++ {
		enc.IndicesLBRR[c].Reset()
		enc.PulsesLBRR[c] = [MAX_FRAME_LENGTH]int8{}
	}
	enc.SShape.Reset()
	enc.SPrefilt.Reset()
	enc.XBuf = [2*MAX_FRAME_LENGTH + LA_SHAPE_MAX]int16{}
	enc.LTPCorrQ15 = 0
}

// ControlEncoder controls the encoder
func (enc *ChannelEncoder) ControlEncoder(encControl *EncControlState, targetRateBps int32, allowBwSwitch int32, channelNb int32, forceFsKHz int32) int {
	var fsKHz int32
	ret := SILK_NO_ERROR

	enc.UseDTX = encControl.UseDTX
	enc.UseCBR = encControl.UseCBR
	enc.APIFsHz = encControl.APISampleRate
	enc.MaxInternalFsHz = encControl.MaxInternalSampleRate
	enc.MinInternalFsHz = encControl.MinInternalSampleRate
	enc.DesiredInternalFsHz = encControl.DesiredInternalSampleRate
	enc.UseInBandFEC = encControl.UseInBandFEC
	enc.NChannelsAPI = encControl.NChannelsAPI
	enc.NChannelsInternal = encControl.NChannelsInternal
	enc.AllowBandwidthSwitch = allowBwSwitch
	enc.ChannelNb = channelNb

	if enc.ControlledSinceLastPayload != 0 && enc.PrefillFlag == 0 {
		if enc.APIFsHz != enc.PrevAPIFsHz && enc.FsKHz > 0 {
			// Change in API sampling rate during packet encoding
			ret = enc.SetupResamplers(enc.FsKHz)
		}
		return ret
	}

	// Determine sampling rate
	fsKHz = enc.ControlAudioBandwidth(encControl)
	if forceFsKHz != 0 {
		fsKHz = forceFsKHz
	}

	// Prepare resampler and buffered data
	ret = enc.SetupResamplers(fsKHz)

	// Set sampling frequency
	ret = enc.SetupFs(fsKHz, encControl.PayloadSizeMs)

	// Set encoding complexity
	ret = enc.SetupComplexity(encControl.Complexity)

	// Set packet loss rate
	enc.PacketLossPerc = encControl.PacketLossPercentage

	// Set LBRR usage
	ret = enc.SetupLBRR(targetRateBps)

	enc.ControlledSinceLastPayload = 1

	return ret
}

// SetupResamplers configures the resamplers
func (enc *ChannelEncoder) SetupResamplers(fsKHz int32) int {
	ret := 0

	if enc.FsKHz != fsKHz || enc.PrevAPIFsHz != enc.APIFsHz {
		if enc.FsKHz == 0 {
			// Initialize resampler
			ret += enc.ResamplerState.Init(enc.APIFsHz, fsKHz*1000, 1)
		} else {
			var xBufAPIFsHz []int16
			tempResamplerState := NewResamplerState()

			bufLengthMs := LSHIFT(enc.NbSubfr*5, 1) + LA_SHAPE_MS
			oldBufSamples := bufLengthMs * enc.FsKHz

			// Initialize temporary resampler
			ret += tempResamplerState.Init(SMULBB(enc.FsKHz, 1000), enc.APIFsHz, 0)

			// Calculate number of samples to upsample
			apiBufSamples := bufLengthMs * DIV32_16(enc.APIFsHz, 1000)

			// Temporary resampling
			xBufAPIFsHz = make([]int16, apiBufSamples)
			ret += tempResamplerState.Resample(xBufAPIFsHz, 0, enc.XBuf[:], 0, oldBufSamples)

			// Initialize main resampler
			ret += enc.ResamplerState.Init(enc.APIFsHz, SMULBB(fsKHz, 1000), 1)

			// Correct resampler state
			ret += enc.ResamplerState.Resample(enc.XBuf[:], 0, xBufAPIFsHz, 0, apiBufSamples)
		}
	}

	enc.PrevAPIFsHz = enc.APIFsHz
	return ret
}

// SetupFs configures the sampling frequency
func (enc *ChannelEncoder) SetupFs(fsKHz int32, packetSizeMs int32) int {
	ret := SILK_NO_ERROR

	// Set packet size
	if packetSizeMs != enc.PacketSizeMs {
		if packetSizeMs != 10 && packetSizeMs != 20 && packetSizeMs != 40 && packetSizeMs != 60 {
			ret = SILK_ENC_PACKET_SIZE_NOT_SUPPORTED
		}
		if packetSizeMs <= 10 {
			enc.NFramesPerPacket = 1
			if packetSizeMs == 10 {
				enc.NbSubfr = 2
			} else {
				enc.NbSubfr = 1
			}
			enc.FrameLength = SMULBB(packetSizeMs, fsKHz)
			enc.PitchLPCWinLength = SMULBB(FIND_PITCH_LPC_WIN_MS_2_SF, fsKHz)
			if enc.FsKHz == 8 {
				enc.PitchContourICDF = PitchContour10MsNBICDF[:]
			} else {
				enc.PitchContourICDF = PitchContour10MsICDF[:]
			}
		} else {
			enc.NFramesPerPacket = DIV32_16(packetSizeMs, MAX_FRAME_LENGTH_MS)
			enc.NbSubfr = MAX_NB_SUBFR
			enc.FrameLength = SMULBB(20, fsKHz)
			enc.PitchLPCWinLength = SMULBB(FIND_PITCH_LPC_WIN_MS, fsKHz)
			if enc.FsKHz == 8 {
				enc.PitchContourICDF = PitchContourNBICDF[:]
			} else {
				enc.PitchContourICDF = PitchContourICDF[:]
			}
		}
		enc.PacketSizeMs = packetSizeMs
		enc.TargetRateBps = 0 // Trigger new SNR computation
	}

	// Set sampling frequency
	opusAssert(fsKHz == 8 || fsKHz == 12 || fsKHz == 16)
	opusAssert(enc.NbSubfr == 2 || enc.NbSubfr == 4)
	if enc.FsKHz != fsKHz {
		// Reset part of the state
		enc.SShape.Reset()
		enc.SPrefilt.Reset()
		enc.SNSQ.Reset()
		enc.PrevNLSFqQ15