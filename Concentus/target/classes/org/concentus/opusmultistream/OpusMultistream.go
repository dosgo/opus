// Package opusmultistream provides utilities for working with Opus multistream channel layouts.
// This is a translation from Java to idiomatic Go, maintaining the original functionality
// while adapting to Go's conventions and best practices.
package opusmultistream

// ChannelLayout represents the configuration of audio channels in an Opus multistream.
// This struct is equivalent to the Java version but uses Go naming conventions.
type ChannelLayout struct {
	NbStreams        int   // Number of individual audio streams
	NbCoupledStreams int   // Number of stereo (coupled) streams
	NbChannels       int   // Total number of output channels
	Mapping          []int // Channel mapping array
}

// ValidateLayout checks if a channel layout configuration is valid.
// Returns true if valid, false otherwise.
//
// Translation decisions:
// 1. Changed return type from int to bool for more idiomatic Go
// 2. Used range loop instead of traditional for loop
// 3. Early returns for better readability
func ValidateLayout(layout ChannelLayout) bool {
	maxChannel := layout.NbStreams + layout.NbCoupledStreams

	// Check if max channel exceeds limit
	if maxChannel > 255 {
		return false
	}

	// Validate each channel mapping
	for _, mapping := range layout.Mapping {
		if mapping >= maxChannel && mapping != 255 {
			return false
		}
	}

	return true
}

// GetLeftChannel finds the next left channel for a given stream starting from prev position.
// Returns channel index or -1 if not found.
//
// Translation decisions:
// 1. Used more descriptive parameter names
// 2. Simplified loop initialization with ternary to if-else
// 3. Used range loop with index when we need the index
func GetLeftChannel(layout ChannelLayout, streamID int, prev int) int {
	start := 0
	if prev >= 0 {
		start = prev + 1
	}

	for i := start; i < layout.NbChannels; i++ {
		if layout.Mapping[i] == streamID*2 {
			return i
		}
	}

	return -1
}

// GetRightChannel finds the next right channel for a given stream starting from prev position.
// Returns channel index or -1 if not found.
func GetRightChannel(layout ChannelLayout, streamID int, prev int) int {
	start := 0
	if prev >= 0 {
		start = prev + 1
	}

	for i := start; i < layout.NbChannels; i++ {
		if layout.Mapping[i] == streamID*2+1 {
			return i
		}
	}

	return -1
}

// GetMonoChannel finds the next mono channel for a given stream starting from prev position.
// Returns channel index or -1 if not found.
func GetMonoChannel(layout ChannelLayout, streamID int, prev int) int {
	start := 0
	if prev >= 0 {
		start = prev + 1
	}

	for i := start; i < layout.NbChannels; i++ {
		if layout.Mapping[i] == streamID+layout.NbCoupledStreams {
			return i
		}
	}

	return -1
}
