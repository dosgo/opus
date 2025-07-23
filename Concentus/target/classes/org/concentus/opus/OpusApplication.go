package opus

// Application represents the Opus encoder application mode.
// It determines the tradeoffs made by the encoder to optimize for different use cases.
type Application int

const (
	// ApplicationUnimplemented is a placeholder for unimplemented modes
	ApplicationUnimplemented Application = iota

	// ApplicationVoip is best for most VoIP/videoconference applications where
	// listening quality and intelligibility matter most.
	// It optimizes for speech and intelligibility.
	ApplicationVoip

	// ApplicationAudio is best for broadcast/high-fidelity applications where
	// the decoded audio should be as close as possible to the input.
	// It optimizes for music and general audio quality.
	ApplicationAudio

	// ApplicationRestrictedLowDelay is only used when lowest-achievable latency
	// is what matters most. Voice-optimized modes cannot be used in this mode.
	ApplicationRestrictedLowDelay
)
