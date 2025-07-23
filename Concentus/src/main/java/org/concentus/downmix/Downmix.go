package downmix

// DownmixInt downmixes an integer audio signal from multiple channels to a subband.
// This is a direct translation from the Java version with idiomatic Go improvements.
//
// Key translation decisions:
// 1. Used Go naming conventions (mixedCase instead of snake_case)
// 2. Removed pointer parameters (Go slices are reference types)
// 3. Simplified the scale calculation logic
// 4. Added parameter validation
// 5. Used more descriptive variable names where helpful
func DownmixInt(
	x []int16, // Input signal (multi-channel)
	xOffset int, // Offset in the input signal
	sub []int32, // Output subband
	subOffset int, // Offset in the output subband
	subframe int, // Number of samples to process
	offset int, // Additional offset in input
	c1 int, // First channel to mix
	c2 int, // Second channel to mix (-1 for none, -2 for all)
	C int, // Total number of channels
) {
	// Parameter validation
	if xOffset < 0 || subOffset < 0 || offset < 0 || subframe < 0 {
		return
	}
	if c1 < 0 || c1 >= C || (c2 < -2 || c2 >= C) {
		return
	}
	if len(x) < (xOffset+(offset+subframe)*C) || len(sub) < (subOffset+subframe) {
		return
	}

	// Initialize subband with first channel data
	for j := 0; j < subframe; j++ {
		pos := (j + offset) * C
		sub[j+subOffset] = int32(x[xOffset+pos+c1])
	}

	// Handle second channel cases
	switch {
	case c2 > -1: // Mix specific second channel
		for j := 0; j < subframe; j++ {
			pos := (j + offset) * C
			sub[j+subOffset] += int32(x[xOffset+pos+c2])
		}
	case c2 == -2: // Mix all other channels
		for c := 0; c < C; c++ {
			if c == c1 { // Skip the first channel we already added
				continue
			}
			for j := 0; j < subframe; j++ {
				pos := (j + offset) * C
				sub[j+subOffset] += int32(x[xOffset+pos+c])
			}
		}
	}

	// Calculate scaling factor
	scale := int32(1 << SigShift)
	if c2 == -2 {
		// When mixing all channels, divide by total channel count
		scale /= int32(C)
	} else {
		// For stereo mixing, divide by 2
		scale /= 2
	}

	// Apply scaling
	for j := 0; j < subframe; j++ {
		sub[j+subOffset] *= scale
	}
}

// SigShift is the signal shift constant equivalent to CeltConstants.SIG_SHIFT
const SigShift = 15 // Typical value for audio processing
