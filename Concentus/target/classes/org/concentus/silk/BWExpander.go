// Package silk implements bandwidth expansion functions for SILK codec.
package silk

// BWExpander provides functions for bandwidth expansion of AR filters.
type BWExpander struct{}

// Bwexpander32 chirps (bandwidth expands) a 32-bit LP AR filter.
// This is a fixed-point implementation.
//
// Args:
//
//	ar: AR filter coefficients to be expanded (without leading 1), modified in-place
//	d: Length of the AR filter
//	chirpQ16: Chirp factor in Q16 format (typically in range 0..65536)
//
// Note: Q16 means fixed-point number with 16 fractional bits.
func (b *BWExpander) Bwexpander32(ar []int32, d int, chirpQ16 int32) {
	// Calculate chirp - 1 in Q16 format
	chirpMinusOneQ16 := chirpQ16 - 65536 // 65536 = 1.0 in Q16

	// Process all but the last coefficient
	for i := 0; i < d-1; i++ {
		// Multiply coefficient by chirp factor with proper rounding
		ar[i] = SMULWW(chirpQ16, ar[i])

		// Update chirp factor for next iteration
		// This effectively applies the chirp factor incrementally
		chirpQ16 += RSHIFT_ROUND(MUL(chirpQ16, chirpMinusOneQ16), 16)
	}

	// Process the last coefficient
	ar[d-1] = SMULWW(chirpQ16, ar[d-1])
}

// Bwexpander chirps (bandwidth expands) a 16-bit LP AR filter.
// This is a fixed-point implementation.
//
// Args:
//
//	ar: AR filter coefficients to be expanded (without leading 1), modified in-place
//	d: Length of the AR filter
//	chirpQ16: Chirp factor in Q16 format (typically in range 0..65536)
//
// Note: The implementation avoids using SMULWB due to potential stability issues.
func (b *BWExpander) Bwexpander(ar []int16, d int, chirpQ16 int32) {
	// Calculate chirp - 1 in Q16 format
	chirpMinusOneQ16 := chirpQ16 - 65536 // 65536 = 1.0 in Q16

	// Process all but the last coefficient
	for i := 0; i < d-1; i++ {
		// Multiply coefficient by chirp factor with proper rounding
		// Using explicit multiply and shift instead of SMULWB for stability
		ar[i] = int16(RSHIFT_ROUND(MUL(chirpQ16, int32(ar[i])), 16))

		// Update chirp factor for next iteration
		chirpQ16 += RSHIFT_ROUND(MUL(chirpQ16, chirpMinusOneQ16), 16)
	}

	// Process the last coefficient
	ar[d-1] = int16(RSHIFT_ROUND(MUL(chirpQ16, int32(ar[d-1])), 16))
}

// MUL performs 32x32 bit multiplication and returns 32-bit result (low 32 bits)
