package concentus

import (
	"math"
)

// CorrelateVector calculates correlation vector X'*t
//
// Parameters:
//
//	x: x vector [L + order - 1] used to form data matrix X
//	xPtr: starting index in x vector
//	t: target vector [L]
//	tPtr: starting index in t vector
//	L: length of vectors
//	order: max lag for correlation
//	Xt: output X'*t correlation vector [order]
//	rshifts: right shifts of correlations
func CorrelateVector(x []int16, xPtr int, t []int16, tPtr int, L int, order int, Xt []int32, rshifts int) {
	ptr1 := xPtr + order - 1 // Points to first sample of column 0 of X: X[:,0]
	ptr2 := tPtr

	// Calculate X'*t
	if rshifts > 0 {
		// Right shifting used
		for lag := 0; lag < order; lag++ {
			var innerProd int32
			for i := 0; i < L; i++ {
				// Equivalent to silk_RSHIFT32(silk_SMULBB(x[ptr1+i], t[ptr2+i]), rshifts)
				product := int32(x[ptr1+i]) * int32(t[ptr2+i])
				innerProd += product >> uint(rshifts)
			}
			Xt[lag] = innerProd // X[:,lag]'*t
			ptr1--              // Go to next column of X
		}
	} else {
		for lag := 0; lag < order; lag++ {
			Xt[lag] = innerProd(x, ptr1, t, ptr2, L) // X[:,lag]'*t
			ptr1--                                   // Go to next column of X
		}
	}
}

// innerProd calculates the inner product of two vectors with bounds checking
func innerProd(x []int16, xPtr int, y []int16, yPtr int, length int) int32 {
	var sum int32
	for i := 0; i < length; i++ {
		sum += int32(x[xPtr+i]) * int32(y[yPtr+i])
	}
	return sum
}

// CorrelateMatrix calculates correlation matrix X'*X
//
// Parameters:
//
//	x: x vector [L + order - 1] used to form data matrix X
//	xPtr: starting index in x vector
//	L: length of vectors
//	order: max lag for correlation
//	headRoom: desired headroom
//	XX: output X'*X correlation matrix [order x order]
//	XXPtr: starting index in XX matrix
//	rshifts: pointer to right shifts of correlations (updated in place)
func CorrelateMatrix(x []int16, xPtr int, L int, order int, headRoom int, XX []int32, XXPtr int, rshifts *int) {
	// Calculate energy to find shift used to fit in 32 bits
	energy, rshiftsLocal := sumSqrShift(x, xPtr, L+order-1)

	// Add shifts to get the desired head room
	headRoomRshifts := max(headRoom-countLeadingZeros32(energy), 0)
	energy >>= uint(headRoomRshifts)
	rshiftsLocal += headRoomRshifts

	// Calculate energy of first column (0) of X: X[:,0]'*X[:,0]
	// Remove contribution of first order-1 samples
	for i := xPtr; i < xPtr+order-1; i++ {
		product := int32(x[i]) * int32(x[i])
		energy -= product >> uint(rshiftsLocal)
	}

	if rshiftsLocal < *rshifts {
		// Adjust energy
		energy >>= uint(*rshifts - rshiftsLocal)
		rshiftsLocal = *rshifts
	}

	// Calculate energy of remaining columns of X: X[:,j]'*X[:,j]
	// Fill out the diagonal of the correlation matrix
	matrixSet(XX, XXPtr, 0, 0, order, energy)
	ptr1 := xPtr + order - 1 // First sample of column 0 of X

	for j := 1; j < order; j++ {
		// Update energy by removing oldest sample and adding newest
		oldSample := int32(x[ptr1+L-j]) * int32(x[ptr1+L-j])
		newSample := int32(x[ptr1-j]) * int32(x[ptr1-j])
		energy = energy - (oldSample >> uint(rshiftsLocal)) + (newSample >> uint(rshiftsLocal))
		matrixSet(XX, XXPtr, j, j, order, energy)
	}

	ptr2 := xPtr + order - 2 // First sample of column 1 of X

	// Calculate the remaining elements of the correlation matrix
	if rshiftsLocal > 0 {
		// Right shifting used
		for lag := 1; lag < order; lag++ {
			// Inner product of column 0 and column lag: X[:,0]'*X[:,lag]
			var energy int32
			for i := 0; i < L; i++ {
				product := int32(x[ptr1+i]) * int32(x[ptr2+i])
				energy += product >> uint(rshiftsLocal)
			}

			// Set symmetric elements in matrix
			matrixSet(XX, XXPtr, lag, 0, order, energy)
			matrixSet(XX, XXPtr, 0, lag, order, energy)

			// Calculate remaining off diagonal: X[:,j]'*X[:,j + lag]
			for j := 1; j < (order - lag); j++ {
				oldSample := int32(x[ptr1+L-j]) * int32(x[ptr2+L-j])
				newSample := int32(x[ptr1-j]) * int32(x[ptr2-j])
				energy = energy - (oldSample >> uint(rshiftsLocal)) + (newSample >> uint(rshiftsLocal))
				matrixSet(XX, XXPtr, lag+j, j, order, energy)
				matrixSet(XX, XXPtr, j, lag+j, order, energy)
			}
			ptr2-- // Update pointer to first sample of next column (lag) in X
		}
	} else {
		for lag := 1; lag < order; lag++ {
			// Inner product of column 0 and column lag: X[:,0]'*X[:,lag]
			energy := innerProd(x, ptr1, x, ptr2, L)
			matrixSet(XX, XXPtr, lag, 0, order, energy)
			matrixSet(XX, XXPtr, 0, lag, order, energy)

			// Calculate remaining off diagonal: X[:,j]'*X[:,j + lag]
			for j := 1; j < (order - lag); j++ {
				energy -= int32(x[ptr1+L-j]) * int32(x[ptr2+L-j])
				energy += int32(x[ptr1-j]) * int32(x[ptr2-j])
				matrixSet(XX, XXPtr, lag+j, j, order, energy)
				matrixSet(XX, XXPtr, j, lag+j, order, energy)
			}
			ptr2-- // Update pointer to first sample of next column (lag) in X
		}
	}
	*rshifts = rshiftsLocal
}

// matrixSet sets a value in a matrix stored as a 1D array
func matrixSet(matrix []int32, offset int, row int, col int, numCols int, value int32) {
	matrix[offset+row*numCols+col] = value
}

// sumSqrShift calculates the sum of squares and determines the needed right shift
func sumSqrShift(x []int16, xPtr int, length int) (int32, int) {
	var sum int64
	for i := 0; i < length; i++ {
		val := int64(x[xPtr+i])
		sum += val * val
	}

	// Find the necessary shift to fit in int32
	shift := 0
	for sum > math.MaxInt32 {
		sum >>= 1
		shift++
	}

	return int32(sum), shift
}

// countLeadingZeros32 counts the number of leading zeros in a 32-bit integer
func countLeadingZeros32(n int32) int {
	if n == 0 {
		return 32
	}
	return 31 - int(math.Floor(math.Log2(float64(n))))
}
