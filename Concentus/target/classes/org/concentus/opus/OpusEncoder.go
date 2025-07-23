
package opus

import (
	"errors"
	"fmt"
	"math"
)

// OpusEncoder represents the Opus encoder structure
type OpusEncoder struct {
	silkMode            EncControlState
	application         OpusApplication
	channels            int
	delayCompensation   int
	forceChannels       int
	signalType          OpusSignal
	userBandwidth       OpusBandwidth
	maxBandwidth        OpusBandwidth
	userForcedMode      OpusMode
	voiceRatio          int
	Fs                  int
	useVBR              int
	vbrConstraint       int
	variableDuration    OpusFramesize
	bitrateBps          int
	userBitrateBps      int
	lsbDepth            int
	encoderBuffer       int
	lfe                 int
	analysis            TonalityAnalysisState
	streamChannels      int
	hybridStereoWidthQ14 int16
	variableHPSmth2Q15  int
	prevHBGain          int
	hpMem               [4]int
	mode                OpusMode
	prevMode            OpusMode
	prevChannels        int
	prevFramesize       int
	bandwidth           OpusBandwidth
	silkBwSwitch        int
	first               int
	energyMasking       []int
	widthMem            StereoWidthState
	delayBuffer         []int16
	detectedBandwidth   OpusBandwidth
	rangeFinal          int
	silkEncoder         SilkEncoder
	celtEncoder         CeltEncoder
}

// NewOpusEncoder creates and initializes a new Opus encoder
func NewOpusEncoder(Fs, channels int, application OpusApplication) (*OpusEncoder, error) {
	if !isValidSampleRate(Fs) {
		return nil, errors.New("sample rate is invalid (must be 8/12/16/24/48 kHz)")
	}
	if channels != 1 && channels != 2 {
		return nil, errors.New("number of channels must be 1 or 2")
	}

	enc := &OpusEncoder{
		delayBuffer: make([]int16, MAX_ENCODER_BUFFER*2),
	}
	
	if err := enc.opusInitEncoder(Fs, channels, application); err != nil {
		if errors.Is(err, ErrBadArg) {
			return nil, errors.New("OPUS_BAD_ARG when creating encoder")
		}
		return nil, fmt.Errorf("error while initializing encoder: %w", err)
	}
	
	return enc, nil
}

func isValidSampleRate(Fs int) bool {
	switch Fs {
	case 8000, 12000, 16000, 24000, 48000:
		return true
	default:
		return false
	}
}

func (e *OpusEncoder) opusInitEncoder(Fs, channels int, application OpusApplication) error {
	if !isValidSampleRate(Fs) || (channels != 1 && channels != 2) || 
		application == OPUS_APPLICATION_UNIMPLEMENTED {
		return ErrBadArg
	}

	e.reset()
	e.streamChannels = e.channels = channels
	e.Fs = Fs

	if err := silkInitEncoder(&e.silkEncoder, &e.silkMode); err != nil {
		return ErrInternalError
	}

	// Default SILK parameters
	e.silkMode.nChannelsAPI = channels
	e.silkMode.nChannelsInternal = channels
	e.silkMode.APISampleRate = e.Fs
	e.silkMode.maxInternalSampleRate = 16000
	e.silkMode.minInternalSampleRate = 8000
	e.silkMode.desiredInternalSampleRate = 16000
	e.silkMode.payloadSizeMs = 20
	e.silkMode.bitRate = 25000
	e.silkMode.packetLossPercentage = 0
	e.silkMode.complexity = 9
	e.silkMode.useInBandFEC = 0
	e.silkMode.useDTX = 0
	e.silkMode.useCBR = 0
	e.silkMode.reducedDependency = 0

	// Initialize CELT encoder
	if err := e.celtEncoder.celtEncoderInit(Fs, channels); err != nil {
		return ErrInternalError
	}

	e.celtEncoder.SetSignalling(0)
	e.celtEncoder.SetComplexity(e.silkMode.complexity)

	e.useVBR = 1
	e.vbrConstraint = 1
	e.userBitrateBps = OPUS_AUTO
	e.bitrateBps = 3000 + Fs*channels
	e.application = application
	e.signalType = OPUS_SIGNAL_AUTO
	e.userBandwidth = OPUS_BANDWIDTH_AUTO
	e.maxBandwidth = OPUS_BANDWIDTH_FULLBAND
	e.forceChannels = OPUS_AUTO
	e.userForcedMode = MODE_AUTO
	e.voiceRatio = -1
	e.encoderBuffer = e.Fs / 100
	e.lsbDepth = 24
	e.variableDuration = OPUS_FRAMESIZE_ARG

	// Delay compensation of 4 ms
	e.delayCompensation = e.Fs / 250

	e.hybridStereoWidthQ14 = 1 << 14
	e.prevHBGain = Q15ONE
	e.variableHPSmth2Q15 = silkLSHIFT(silkLin2log(VARIABLE_HP_MIN_CUTOFF_HZ), 8)
	e.first = 1
	e.mode = MODE_HYBRID
	e.bandwidth = OPUS_BANDWIDTH_FULLBAND

	tonalityAnalysisInit(&e.analysis)

	return nil
}

func (e *OpusEncoder) reset() {
	e.silkMode.Reset()
	e.application = OPUS_APPLICATION_UNIMPLEMENTED
	e.channels = 0
	e.delayCompensation = 0
	e.forceChannels = 0
	e.signalType = OPUS_SIGNAL_UNKNOWN
	e.userBandwidth = OPUS_BANDWIDTH_UNKNOWN
	e.maxBandwidth = OPUS_BANDWIDTH_UNKNOWN
	e.userForcedMode = MODE_UNKNOWN
	e.voiceRatio = 0
	e.Fs = 0
	e.useVBR = 0
	e.vbrConstraint = 0
	e.variableDuration = OPUS_FRAMESIZE_UNKNOWN
	e.bitrateBps = 0
	e.userBitrateBps = 0
	e.lsbDepth = 0
	e.encoderBuffer = 0
	e.lfe = 0
	e.analysis.Reset()
	e.partialReset()
}

func (e *OpusEncoder) partialReset() {
	e.streamChannels = 0
	e.hybridStereoWidthQ14 = 0
	e.variableHPSmth2Q15 = 0
	e.prevHBGain = 0
	for i := range e.hpMem {
		e.hpMem[i] = 0
	}
	e.mode = MODE_UNKNOWN
	e.prevMode = MODE_UNKNOWN
	e.prevChannels = 0
	e.prevFramesize = 0
	e.bandwidth = OPUS_BANDWIDTH_UNKNOWN
	e.silkBwSwitch = 0
	e.first = 0
	e.energyMasking = nil
	e.widthMem.Reset()
	for i := range e.delayBuffer {
		e.delayBuffer[i] = 0
	}
	e.detectedBandwidth = OPUS_BANDWIDTH_UNKNOWN
	e.rangeFinal = 0
}

func (e *OpusEncoder) ResetState() {
	dummy := EncControlState{}
	e.analysis.Reset()
	e.partialReset()

	e.celtEncoder.ResetState()
	silkInitEncoder(&e.silkEncoder, &dummy)
	e.streamChannels = e.channels
	e.hybridStereoWidthQ14 = 1 << 14
	e.prevHBGain = Q15ONE
	e.first = 1
	e.mode = MODE_HYBRID
	e.bandwidth = OPUS_BANDWIDTH_FULLBAND
	e.variableHPSmth2Q15 = silkLSHIFT(silkLin2log(VARIABLE_HP_MIN_CUTOFF_HZ), 8)
}

func (e *OpusEncoder) userBitrateToBitrate(frameSize, maxDataBytes int) int {
	if frameSize == 0 {
		frameSize = e.Fs / 400
	}
	switch e.userBitrateBps {
	case OPUS_AUTO:
		return 60*e.Fs/frameSize + e.Fs*e.channels
	case OPUS_BITRATE_MAX:
		return maxDataBytes * 8 * e.Fs / frameSize
	default:
		return e.userBitrateBps
	}
}

// Encode encodes an Opus frame
func (e *OpusEncoder) Encode(pcm []int16, frameSize int, data []byte, maxDataBytes int) (int, error) {
	if len(data) < maxDataBytes {
		return 0, fmt.Errorf("output buffer is too small: stated size is %d bytes, actual size is %d bytes", 
			maxDataBytes, len(data))
	}

	delayCompensation := e.delayCompensation
	if e.application == OPUS_APPLICATION_RESTRICTED_LOWDELAY {
		delayCompensation = 0
	}

	internalFrameSize := computeFrameSize(pcm, frameSize, e.variableDuration, e.channels, e.Fs, 
		e.bitrateBps, delayCompensation, e.analysis.subframeMem, e.analysis.enabled)

	if len(pcm) < internalFrameSize*e.channels {
		return 0, fmt.Errorf("not enough samples provided in input signal: expected %d samples, found %d", 
			internalFrameSize, len(pcm)/e.channels)
	}

	ret, err := e.opusEncodeNative(pcm, internalFrameSize, data, maxDataBytes, 16, 
		pcm, frameSize, 0, -2, e.channels, false)
	if err != nil {
		if errors.Is(err, ErrBadArg) {
			return 0, errors.New("OPUS_BAD_ARG while encoding")
		}
		return 0, fmt.Errorf("error during encoding: %w", err)
	}

	return ret, nil
}

// EncodeBytes encodes an Opus frame from a byte array
func (e *OpusEncoder) EncodeBytes(pcm []byte, frameSize int, data []byte, maxDataBytes int) (int, error) {
	spcm := make([]int16, frameSize*e.channels)
	for i := 0; i < len(spcm); i++ {
		spcm[i] = int16(pcm[i*2]) | int16(pcm[i*2+1])<<8
	}
	return e.Encode(spcm, frameSize, data, maxDataBytes)
}

// Getters and setters for encoder properties
func (e *OpusEncoder) Application() OpusApplication { return e.application }
func (e *OpusEncoder) SetApplication(app OpusApplication) error {
	if e.first == 0 && e.application != app {
		return errors.New("application cannot be changed after encoding has started")
	}
	e.application = app
	return nil
}

func (e *OpusEncoder) Bitrate() int {
	return e.userBitrateToBitrate(e.prevFramesize, 1276)
}

func (e *OpusEncoder) SetBitrate(bitrate int) error {
	if bitrate != OPUS_AUTO && bitrate != OPUS_BITRATE_MAX {
		if bitrate <= 0 {
			return errors.New("bitrate must be positive")
		} else if bitrate <= 500 {
			bitrate = 500
		} else if bitrate > 300000*e.channels {
			bitrate = 300000 * e.channels
		}
	}
	e.userBitrateBps = bitrate
	return nil
}

func (e *OpusEncoder) ForceChannels() int { return e.forceChannels }
func (e *OpusEncoder) SetForceChannels(channels int) error {
	if (channels < 1 || channels > e.channels) && channels != OPUS_AUTO {
		return errors.New("force channels must be <= num. of channels")
	}
	e.forceChannels = channels
	return nil
}

func (e *OpusEncoder) MaxBandwidth() OpusBandwidth { return e.maxBandwidth }
func (e *OpusEncoder) SetMaxBandwidth(bw OpusBandwidth) {
	e.maxBandwidth = bw
	switch bw {
	case OPUS_BANDWIDTH_NARROWBAND:
		e.silkMode.maxInternalSampleRate = 8000
	case OPUS_BANDWIDTH_MEDIUMBAND:
		e.silkMode.maxInternalSampleRate = 12000
	default:
		e.silkMode.maxInternalSampleRate = 16000
	}
}

func (e *OpusEncoder) Bandwidth() OpusBandwidth { return e.bandwidth }
func (e *OpusEncoder) SetBandwidth(bw OpusBandwidth) {
	e.userBandwidth = bw
	switch bw {
	case OPUS_BANDWIDTH_NARROWBAND:
		e.silkMode.maxInternalSampleRate = 8000
	case OPUS_BANDWIDTH_MEDIUMBAND:
		e.silkMode.maxInternalSampleRate = 12000
	default:
		e.silkMode.maxInternalSampleRate = 16000
	}
}

func (e *OpusEncoder) UseDTX() bool { return e.silkMode.useDTX != 0 }
func (e *OpusEncoder) SetUseDTX(use bool) {
	if use {
		e.silkMode.useDTX = 1
	} else {
		e.silkMode.useDTX = 0
	}
}

func (e *OpusEncoder) Complexity() int { return e.silkMode.complexity }
func (e *OpusEncoder) SetComplexity(complexity int) error {
	if complexity < 0 || complexity > 10 {
		return errors.New("complexity must be between 0 and 10")
	}
	e.silkMode.complexity = complexity
	e.celtEncoder.SetComplexity(complexity)
	return nil
}

func (e *OpusEncoder) UseInbandFEC() bool { return e.silkMode.useInBandFEC != 0 }
func (e *OpusEncoder) SetUseInbandFEC(use bool) {
	if use {
		e.silkMode.useInBandFEC = 1
	} else {
		e.silkMode.useInBandFEC = 0
	}
}

func (e *OpusEncoder) PacketLossPercent() int { return e.silkMode.packetLossPercentage }
func (e *OpusEncoder) SetPacketLossPercent(percent int) error {
	if percent < 0 || percent > 100 {
		return errors.New("packet loss must be between 0 and 100")
	}
	e.silkMode.packetLossPercentage = percent
	e.celtEncoder.SetPacketLossPercent(percent)
	return nil
}

func (e *OpusEncoder) UseVBR() bool { return e.useVBR != 0 }
func (e *OpusEncoder) SetUseVBR(use bool) {
	if use {
		e.useVBR = 1
		e.silkMode.useCBR = 0
	} else {
		e.useVBR = 0
		e.silkMode.useCBR = 1
	}
}

func (e *OpusEncoder) UseConstrainedVBR() bool { return e.vbrConstraint != 0 }
func (e *OpusEncoder) SetUseConstrainedVBR(use bool) {
	if use {
		e.vbrConstraint = 1
	} else {
		e.vbrConstraint = 0
	}
}

func (e *OpusEncoder) SignalType() OpusSignal { return e.signalType }
func (e *OpusEncoder) SetSignalType(signal OpusSignal) { e.signalType = signal }

func (e *OpusEncoder) Lookahead() int {
	lookahead := e.Fs / 400
	if e.application != OPUS_APPLICATION_RESTRICTED_LOWDELAY {
		lookahead += e.delayCompensation
	}
	return lookahead
}

func (e *OpusEncoder) SampleRate() int { return e.Fs }
func (e *OpusEncoder) FinalRange() int { return e.rangeFinal }

func (e *OpusEncoder) LSBDepth() int { return e.lsbDepth }
func (e *OpusEncoder) SetLSBDepth(depth int) error {
	if depth < 8 || depth > 24 {
		return errors.New("LSB depth must be between 8 and 24")
	}
	e.lsbDepth = depth
	return nil
}

func (e *OpusEncoder) ExpertFrameDuration() OpusFramesize { return e.variableDuration }
func (e *OpusEncoder) SetExpertFrameDuration(duration OpusFramesize) {
	e.variableDuration = duration
	e.celtEncoder.SetExpertFrameDuration(duration)
}

func (e *OpusEncoder) ForceMode() OpusMode { return e.userForcedMode }
func (e *OpusEncoder) SetForceMode(mode OpusMode) { e.userForcedMode = mode }

func (e *OpusEncoder) IsLFE() bool { return e.lfe != 0 }
func (e *OpusEncoder) SetIsLFE(isLFE bool) {
	if isLFE {
		e.lfe = 1
	} else {
		e.lfe = 0
	}
	e.celtEncoder.SetLFE(e.lfe)
}

func (e *OpusEncoder) PredictionDisabled() bool { return e.silkMode.reducedDependency != 0 }
func (e *OpusEncoder) SetPredictionDisabled(disabled bool) {
	if disabled {
		e.silkMode.reducedDependency = 1
	} else {
		e.silkMode.reducedDependency = 0
	}
}

func (e *OpusEncoder) EnableAnalysis() bool { return e.analysis.enabled }
func (e *OpusEncoder) SetEnableAnalysis(enabled bool) { e.analysis.enabled = enabled }

func (e *OpusEncoder) SetEnergyMask(mask []int)