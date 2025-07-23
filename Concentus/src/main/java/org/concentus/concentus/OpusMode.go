package concentus

// OpusMode represents the different encoding modes available in the Opus codec.
// Go doesn't have enums like Java, but we can use typed constants with iota
// to achieve similar functionality in a more Go-idiomatic way.
type OpusMode int

const (
	// MODE_UNKNOWN represents an unknown or unspecified mode
	MODE_UNKNOWN OpusMode = iota

	// MODE_AUTO lets the encoder automatically choose the mode
	MODE_AUTO

	// MODE_SILK_ONLY uses only the SILK portion of the codec (best for speech)
	MODE_SILK_ONLY

	// MODE_HYBRID uses both SILK and CELT portions (mixed speech and music)
	MODE_HYBRID

	// MODE_CELT_ONLY uses only the CELT portion (best for music)
	MODE_CELT_ONLY
)

// String provides a human-readable representation of the OpusMode
// This is a common Go pattern for "enum" types
func (m OpusMode) String() string {
	switch m {
	case MODE_UNKNOWN:
		return "MODE_UNKNOWN"
	case MODE_AUTO:
		return "MODE_AUTO"
	case MODE_SILK_ONLY:
		return "MODE_SILK_ONLY"
	case MODE_HYBRID:
		return "MODE_HYBRID"
	case MODE_CELT_ONLY:
		return "MODE_CELT_ONLY"
	default:
		return "MODE_UNKNOWN"
	}
}
