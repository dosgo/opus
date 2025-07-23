package concentus

// OpusSignal represents the type of signal being encoded (voice, music, etc.)
type OpusSignal int

const (
	// SignalUnknown indicates the signal type is unknown
	SignalUnknown OpusSignal = iota

	// SignalAuto indicates automatic signal type detection
	SignalAuto

	// SignalVoice indicates the signal being encoded is voice
	SignalVoice

	// SignalMusic indicates the signal being encoded is music
	SignalMusic
)
