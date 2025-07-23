package concentus

// TuningParams contains various audio processing tuning parameters.
// In Go, we use a const block for related constants and document groups together.
const (
	// BitreservoirDecayTimeMs is the decay time for entropy coder bit reservoir
	BitreservoirDecayTimeMs = 500

	// ****************
	// Pitch estimator
	// ****************

	// FindPitchWhiteNoiseFraction is the level of noise floor for whitening filter
	// LPC analysis in pitch analysis
	FindPitchWhiteNoiseFraction = 1e-3

	// FindPitchBandwidthExpansion is the bandwidth expansion for whitening filter
	// in pitch analysis
	FindPitchBandwidthExpansion = 0.99

	// ******************
	// Linear prediction
	// ******************

	// FindLPCCondFac is the LPC analysis regularization factor
	FindLPCCondFac = 1e-5

	// FindLTPCondFac is the LTP analysis regularization factor
	FindLTPCondFac = 1e-5

	// LTPDamping is the LTP damping factor
	LTPDamping = 0.05

	// LTPSmoothing is the LTP smoothing factor
	LTPSmoothing = 0.1

	// LTP quantization settings
	MuLTPQuantNB = 0.03  // Narrowband
	MuLTPQuantMB = 0.025 // Mediumband
	MuLTPQuantWB = 0.02  // Wideband

	// MaxSumLogGainDB is the max cumulative LTP gain
	MaxSumLogGainDB = 250.0

	// ********************
	// High pass filtering
	// ********************

	// VariableHPSmthCoef1 is a smoothing parameter for low end of pitch frequency range
	VariableHPSmthCoef1 = 0.1

	// VariableHPSmthCoef2 is another smoothing parameter
	VariableHPSmthCoef2 = 0.015

	// VariableHPMaxDeltaFreq is the maximum delta frequency
	VariableHPMaxDeltaFreq = 0.4

	// VariableHPMinCutoffHz is the minimum cut-off frequency (-3 dB point)
	VariableHPMinCutoffHz = 60

	// VariableHPMaxCutoffHz is the maximum cut-off frequency (-3 dB point)
	VariableHPMaxCutoffHz = 100

	// ********
	// Various
	// ********

	// SpeechActivityDTXThres is the VAD threshold
	SpeechActivityDTXThres = 0.05

	// LBRRSpeechActivityThres is the speech activity LBRR enable threshold
	LBRRSpeechActivityThres = 0.3

	// **********************
	// Perceptual parameters
	// **********************

	// BGSNRDecrDB is the reduction in coding SNR during low speech activity
	BGSNRDecrDB = 2.0

	// HarmSNRIncrDB is the factor for reducing quantization noise during voiced speech
	HarmSNRIncrDB = 2.0

	// SparseSNRIncrDB is the factor for reducing quantization noise for unvoiced sparse signals
	SparseSNRIncrDB = 2.0

	// SparsenessThresholdQNTOffset is the threshold for sparseness measure above which to use
	// lower quantization offset during unvoiced
	SparsenessThresholdQNTOffset = 0.75

	// WarpingMultiplier controls warping
	WarpingMultiplier = 0.015

	// ShapeWhiteNoiseFraction is the fraction added to first autocorrelation value
	ShapeWhiteNoiseFraction = 5e-5

	// BandwidthExpansion is the noise shaping filter chirp factor
	BandwidthExpansion = 0.95

	// LowRateBandwidthExpansionDelta is the difference between chirp factors for analysis
	// and synthesis noise shaping filters at low bitrates
	LowRateBandwidthExpansionDelta = 0.01

	// LowRateHarmonicBoost is the extra harmonic boosting (signal shaping) at low bitrates
	LowRateHarmonicBoost = 0.1

	// LowInputQualityHarmonicBoost is the extra harmonic boosting for noisy input signals
	LowInputQualityHarmonicBoost = 0.1

	// HarmonicShaping is the harmonic noise shaping factor
	HarmonicShaping = 0.3

	// HighRateOrLowQualityHarmonicShaping is the extra harmonic noise shaping for
	// high bitrates or noisy input
	HighRateOrLowQualityHarmonicShaping = 0.2

	// HPNoiseCoef is the parameter for shaping noise towards higher frequencies
	HPNoiseCoef = 0.25

	// HarmHPNoiseCoef is the parameter for shaping noise even more towards higher
	// frequencies during voiced speech
	HarmHPNoiseCoef = 0.35

	// InputTilt is the parameter for applying a high-pass tilt to the input signal
	InputTilt = 0.05

	// HighRateInputTilt is the parameter for extra high-pass tilt at high rates
	HighRateInputTilt = 0.1

	// LowFreqShaping is the parameter for reducing noise at very low frequencies
	LowFreqShaping = 4.0

	// LowQualityLowFreqShapingDecr is the reduction factor for low frequency shaping
	// when signal has low SNR at low frequencies
	LowQualityLowFreqShapingDecr = 0.5

	// SubfrSmthCoef is the subframe smoothing coefficient for HarmBoost, HarmShapeGain, Tilt
	// (lower values mean more smoothing)
	SubfrSmthCoef = 0.4

	// Parameters defining the R/D tradeoff in the residual quantizer
	LambdaOffset           = 1.2
	LambdaSpeechAct        = -0.2
	LambdaDelayedDecisions = -0.05
	LambdaInputQuality     = -0.1
	LambdaCodingQuality    = -0.2
	LambdaQuantOffset      = 0.8

	// ReduceBitrate10MsBPS is the compensation in bitrate calculations for 10 ms modes
	ReduceBitrate10MsBPS = 2200

	// MaxBandwidthSwitchDelayMs is the maximum time before allowing a bandwidth transition
	MaxBandwidthSwitchDelayMs = 5000
)
