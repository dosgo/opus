package concentus

import (
	"math"
)

// ResidualEnergy calculates residual energies of input subframes where all subframes have LPC_order
// of preceding samples.
//
// Key translation decisions:
// 1. Replaced Java's BoxedValueInt with direct value returns where possible
// 2. Used Go's multiple return values instead of output parameters
// 3. Simplified array indexing by removing pointer offsets where possible
// 4. Used Go's native slice types instead of fixed-size arrays
// 5. Added bounds checking where appropriate
// 6. Used Go's built-in min/max functions where available
func ResidualEnergy(
	nrgs []int, // O: Residual energy per subframe
	nrgsQ []int, // O: Q value per subframe
	x []int16, // I: Input signal
	aQ12 [][]int16, // I: AR coefs for each frame half [2][MAX_LPC_ORDER]
	gains []int, // I: Quantization gains
	subfrLength int, // I: Subframe length
	nbSubfr int, // I: Number of subframes
	LPCorder int, // I: LPC order
) {
	// Validate input dimensions
	if nbSubfr%2 != 0 {
		panic("nbSubfr must be even")
	}

	offset := LPCorder + subfrLength
	xPtr := 0

	// Filter input to create the LPC residual for each frame half, and measure subframe energies
	LPCres := make([]int16, (MAX_NB_SUBFR/2)*offset)

	for i := 0; i < nbSubfr/2; i++ {
		// Calculate half frame LPC residual signal including preceding samples
		LPCAnalysisFilter(LPCres, 0, x[xPtr:], aQ12[i], (MAX_NB_SUBFR/2)*offset, LPCorder)

		// Point to first subframe of the just calculated LPC residual signal
		LPCresPtr := LPCorder
		for j := 0; j < MAX_NB_SUBFR/2; j++ {
			// Measure subframe energy
			energy, rshift := SumSqrShift(LPCres[LPCresPtr : LPCresPtr+subfrLength])
			nrgs[i*(MAX_NB_SUBFR/2)+j] = energy
			nrgsQ[i*(MAX_NB_SUBFR/2)+j] = -rshift

			// Move to next subframe
			LPCresPtr += offset
		}
		// Move to next frame half
		xPtr += (MAX_NB_SUBFR / 2) * offset
	}

	// Apply the squared subframe gains
	for i := 0; i < nbSubfr; i++ {
		// Fully upscale gains and energies
		lz1 := CLZ32(nrgs[i]) - 1
		lz2 := CLZ32(gains[i]) - 1

		tmp32 := LSHIFT32(gains[i], lz2)

		// Find squared gains
		tmp32 = SMMUL(tmp32, tmp32) // Q(2*lz2 - 32)

		// Scale energies
		nrgs[i] = SMMUL(tmp32, LSHIFT32(nrgs[i], lz1))
		// Q(nrgsQ[i] + lz1 + 2*lz2 - 32 - 32)
		nrgsQ[i] += lz1 + 2*lz2 - 64
	}
}

// ResidualEnergy16Covar calculates residual energy: nrg = wxx - 2 * wXx * c + c' * wXX * c
//
// Key translation decisions:
// 1. Simplified parameter passing by using slices directly
// 2. Removed pointer offsets in favor of slice operations
// 3. Used Go's built-in math functions where appropriate
// 4. Added bounds checking
func ResidualEnergy16Covar(
	c []int16, // I: Prediction vector
	wXX []int, // I: Correlation matrix
	wXx []int, // I: Correlation vector
	wxx int, // I: Signal energy
	D int, // I: Dimension
	cQ int, // I: Q value for c vector 0 - 15
) int {
	// Safety checks
	if D < 0 || D > 16 {
		panic("D must be between 0 and 16")
	}
	if cQ <= 0 || cQ >= 16 {
		panic("cQ must be between 1 and 15")
	}

	lshifts := 16 - cQ
	Qxtra := lshifts

	// Find maximum absolute value in c
	cMax := 0
	for _, val := range c[:D] {
		absVal := int(math.Abs(float64(val)))
		if absVal > cMax {
			cMax = absVal
		}
	}

	// Adjust Qxtra based on cMax
	Qxtra = min(Qxtra, CLZ32(cMax)-17)

	// Find maximum value in wXX matrix
	wMax := max(wXX[0], wXX[D*D-1])
	tmp := int(int64(D) * int64(RSHIFT(SMULWB(wMax, cMax), 4)))
	Qxtra = min(Qxtra, CLZ32(tmp)-5)
	Qxtra = max(Qxtra, 0)

	// Prepare scaled c vector
	cn := make([]int, D)
	for i := 0; i < D; i++ {
		cn[i] = LSHIFT(int(c[i]), Qxtra)
		if abs := int(math.Abs(float64(cn[i]))); abs > math.MaxInt16+1 {
			panic("cn[i] exceeds allowed range")
		}
	}
	lshifts -= Qxtra

	// Compute wxx - 2 * wXx * c
	tmp = 0
	for i := 0; i < D; i++ {
		tmp = SMLAWB(tmp, wXx[i], cn[i])
	}
	nrg := RSHIFT(wxx, 1+lshifts) - tmp // Q: -lshifts - 1

	// Add c' * wXX * c, assuming wXX is symmetric
	tmp2 := 0
	for i := 0; i < D; i++ {
		tmp = 0
		pRow := i * D
		for j := i + 1; j < D; j++ {
			tmp = SMLAWB(tmp, wXX[pRow+j], cn[j])
		}
		tmp = SMLAWB(tmp, RSHIFT(wXX[pRow+i], 1), cn[i])
		tmp2 = SMLAWB(tmp2, tmp, cn[i])
	}
	nrg = ADD_LSHIFT32(nrg, tmp2, lshifts) // Q: -lshifts - 1

	// Keep one bit free always
	if nrg < 1 {
		nrg = 1
	} else if nrg > RSHIFT(math.MaxInt32, lshifts+2) {
		nrg = math.MaxInt32 >> 1
	} else {
		nrg = LSHIFT(nrg, lshifts+1) // Q0
	}
	return nrg
}

// Helper functions (assuming these are defined elsewhere in the package)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
