// Package concentus provides Opus codec functionality
package concentus

// OpusBandwidth represents the different bandwidth options for Opus encoding.
// This is a direct translation from the Java enum but follows Go naming conventions.
type OpusBandwidth int

const (
	BandwidthUnknown       OpusBandwidth = iota // OPUS_BANDWIDTH_UNKNOWN
	BandwidthAuto                               // OPUS_BANDWIDTH_AUTO
	BandwidthNarrowband                         // OPUS_BANDWIDTH_NARROWBAND
	BandwidthMediumband                         // OPUS_BANDWIDTH_MEDIUMBAND
	BandwidthWideband                           // OPUS_BANDWIDTH_WIDEBAND
	BandwidthSuperwideband                      // OPUS_BANDWIDTH_SUPERWIDEBAND
	BandwidthFullband                           // OPUS_BANDWIDTH_FULLBAND
)

// GetOrdinal returns the numeric value of the bandwidth for comparison purposes.
// This follows the same numbering scheme as the original Java version.
func (bw OpusBandwidth) GetOrdinal() int {
	switch bw {
	case BandwidthNarrowband:
		return 1
	case BandwidthMediumband:
		return 2
	case BandwidthWideband:
		return 3
	case BandwidthSuperwideband:
		return 4
	case BandwidthFullband:
		return 5
	}
	return -1
}

// GetBandwidth returns the OpusBandwidth corresponding to the given ordinal value.
func GetBandwidth(ordinal int) OpusBandwidth {
	switch ordinal {
	case 1:
		return BandwidthNarrowband
	case 2:
		return BandwidthMediumband
	case 3:
		return BandwidthWideband
	case 4:
		return BandwidthSuperwideband
	case 5:
		return BandwidthFullband
	}
	return BandwidthAuto
}

// Min returns the smaller of two bandwidth values.
func Min(a, b OpusBandwidth) OpusBandwidth {
	if a.GetOrdinal() < b.GetOrdinal() {
		return a
	}
	return b
}

// Max returns the larger of two bandwidth values.
func Max(a, b OpusBandwidth) OpusBandwidth {
	if a.GetOrdinal() > b.GetOrdinal() {
		return a
	}
	return b
}

// Subtract reduces the bandwidth by the specified amount.
func Subtract(a OpusBandwidth, b int) OpusBandwidth {
	return GetBandwidth(a.GetOrdinal() - b)
}
