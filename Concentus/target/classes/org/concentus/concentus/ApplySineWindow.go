package concentus

// ApplySineWindow applies a sine window to a signal vector.
// Window types:
//
//	1 - sine window from 0 to pi/2
//	2 - sine window from pi/2 to pi
//
// Every other sample is linearly interpolated for speed.
// Window length must be between 16 and 120 (inclusive) and a multiple of 4.
func ApplySineWindow(
	pxWin []int16, // O: Windowed signal
	pxWinPtr int, // Starting index in pxWin
	px []int16, // I: Input signal
	pxPtr int, // Starting index in px
	winType int, // I: Window type (1 or 2)
	length int, // I: Window length (multiple of 4, 16-120)
) {
	// Frequency table generated with:
	// for k=16:9*4:16+2*9*4, fprintf(' %7.d,', -round(65536*pi ./ (k:4:k+8*4))); fprintf('\n'); end
	var freqTableQ16 = []int16{
		12111, 9804, 8235, 7100, 6239, 5565, 5022, 4575, 4202,
		3885, 3612, 3375, 3167, 2984, 2820, 2674, 2542, 2422,
		2313, 2214, 2123, 2038, 1961, 1889, 1822, 1760, 1702,
	}

	// Input validation
	if winType != 1 && winType != 2 {
		panic("winType must be 1 or 2")
	}
	if length < 16 || length > 120 {
		panic("length must be between 16 and 120")
	}
	if length&3 != 0 {
		panic("length must be a multiple of 4")
	}

	// Frequency calculation
	k := (length >> 2) - 4
	if k < 0 || k > 26 {
		panic("invalid length for frequency table")
	}
	fQ16 := int32(freqTableQ16[k])

	// Factor used for cosine approximation
	cQ16 := SMULWB(fQ16, -fQ16)
	if cQ16 < -32768 {
		panic("cQ16 out of range")
	}

	// Initialize state
	var S0Q16, S1Q16 int32
	if winType == 1 {
		// Start from 0
		S0Q16 = 0
		// Approximation of sin(f)
		S1Q16 = fQ16 + int32(length>>3)
	} else {
		// Start from 1
		S0Q16 = 1 << 16
		// Approximation of cos(f)
		S1Q16 = (1 << 16) + (cQ16 >> 1) + int32(length>>4)
	}

	// Uses the recursive equation: sin(n*f) = 2*cos(f)*sin((n-1)*f) - sin((n-2)*f)
	// Processes 4 samples at a time
	for k := 0; k < length; k += 4 {
		pxwk := pxWinPtr + k
		pxk := pxPtr + k

		pxWin[pxwk] = int16(SMULWB((S0Q16+S1Q16)>>1, int32(px[pxk])))
		pxWin[pxwk+1] = int16(SMULWB(S1Q16, int32(px[pxk+1])))

		S0Q16 = SMULWB(S1Q16, cQ16) + (S1Q16 << 1) - S0Q16 + 1
		if S0Q16 > (1 << 16) {
			S0Q16 = 1 << 16
		}

		pxWin[pxwk+2] = int16(SMULWB((S0Q16+S1Q16)>>1, int32(px[pxk+2])))
		pxWin[pxwk+3] = int16(SMULWB(S0Q16, int32(px[pxk+3])))

		S1Q16 = SMULWB(S0Q16, cQ16) + (S0Q16 << 1) - S1Q16
		if S1Q16 > (1 << 16) {
			S1Q16 = 1 << 16
		}
	}
}
