package concentus

// ChannelLayout represents the layout of audio channels in a stream.
// It contains information about the number of channels, streams, and
// coupled streams, along with a channel mapping array.
type ChannelLayout struct {
	// Number of audio channels
	NbChannels int

	// Number of separate encoded streams
	NbStreams int

	// Number of coupled stereo streams
	NbCoupledStreams int

	// Channel mapping array (256 elements as in original Java code)
	Mapping [256]int16
}

// Reset initializes all fields to their zero values and clears the mapping array.
// This follows Go's convention of having methods with receiver parameters.
func (cl *ChannelLayout) Reset() {
	cl.NbChannels = 0
	cl.NbStreams = 0
	cl.NbCoupledStreams = 0

	// In Go, we can simply assign a new zero-value array to clear it
	// This is more efficient than iterating through the array
	cl.Mapping = [256]int16{}
}
