package concentus

// SumSqrShift computes the number of bits to right shift the sum of squares
// of a vector of int16s to make it fit in an int32.
//
// Parameters:
//
//	x: Input vector ([]int16)
//	xPtr: Starting index in the vector (0 for full array)
//	len: Length of input to process
//
// Returns:
//
//	energy: Energy of x, after shifting to the right
//	shift: Number of bits right shift applied to energy
func SumSqrShift(x []int16, xPtr int, len int) (energy int32, shift int) {
	var nrg int32
	var shft int
	len-- // adjust for 0-based indexing

	// First pass: compute sum of squares with overflow checking
	i := 0
	for ; i < len; i += 2 {
		// Multiply-accumulate two samples at a time
		nrg = smlabbOvflw(nrg, x[xPtr+i], x[xPtr+i])
		nrg = smlabbOvflw(nrg, x[xPtr+i+1], x[xPtr+i+1])

		if nrg < 0 {
			// Scale down if overflow occurred
			nrg = nrg >> 2
			shft = 2
			i += 2
			break
		}
	}

	// Second pass: continue with shifted accumulation
	for ; i < len; i += 2 {
		nrgTmp := smulbb(x[xPtr+i], x[xPtr+i])
		nrgTmp = smlabbOvflw(nrgTmp, x[xPtr+i+1], x[xPtr+i+1])
		nrg += nrgTmp >> uint(shft)

		if nrg < 0 {
			// Scale down if overflow occurred
			nrg = nrg >> 2
			shft += 2
		}
	}

	// Handle odd-length case (one remaining sample)
	if i == len {
		nrgTmp := smulbb(x[xPtr+i], x[xPtr+i])
		nrg += nrgTmp >> uint(shft)
	}

	// Ensure at least two leading zeros (sign bit + one more)
	if int(nrg)&0xC0000000 != 0 {
		nrg = nrg >> 2
		shft += 2
	}

	return nrg, shft
}

// Zero-index variant of SumSqrShift for convenience
func SumSqrShiftZero(x []int16, len int) (energy int32, shift int) {
	return SumSqrShift(x, 0, len)
}

// Helper functions to replicate the Java inline operations:

// smulbb performs signed multiplication of two 16-bit values (bottom*bottom)
func smulbb(a, b int16) int32 {
	return int32(a) * int32(b)
}

// smlabbOvflw performs multiply-accumulate with overflow checking
func smlabbOvflw(acc int32, a, b int16) int32 {
	product := int32(a) * int32(b)
	result := acc + product

	// Check for overflow (same sign inputs producing different sign result)
	if (product > 0 && acc > 0 && result < 0) ||
		(product < 0 && acc < 0 && result > 0) {
		return result // Let it overflow to be caught by caller
	}
	return result
}
