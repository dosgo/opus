package celt

// CWRS implements the Pyramid Vector Quantizer (PVQ) functions for encoding and decoding pulse vectors.
// This is a direct translation from the Java version, maintaining the same algorithmic approach
// while adapting to Go idioms and conventions.

// CELT_PVQ_U_ROW contains row offsets for the U(N,K) table lookup
var CELT_PVQ_U_ROW = []int{
	0, 176, 351, 525, 698, 870, 1041, 1131, 1178, 1207, 1226, 1240, 1248, 1254, 1257,
}

// CELT_PVQ_U returns the U(N,K) value from precomputed tables
// U(N,K) represents the number of combinations where N-1 objects are taken at most K-1 at a time
func CELT_PVQ_U(n, k int) uint32 {
	// Use the minimum of n and k for row index, maximum for column offset
	min := n
	if k < min {
		min = k
	}
	max := n
	if k > max {
		max = k
	}
	return CeltPVQUData[CELT_PVQ_U_ROW[min]+max]
}

// CELT_PVQ_V returns V(N,K) which is U(N,K) + U(N,K+1)
// V(N,K) represents the number of PVQ codewords for a band of size N with K pulses
func CELT_PVQ_V(n, k int) uint32 {
	return CELT_PVQ_U(n, k) + CELT_PVQ_U(n, k+1)
}

// icwrs computes the index of the pulse vector in the PVQ codebook
func icwrs(n int, y []int) uint32 {
	Assert(n >= 2, "n must be >= 2")
	var i uint32
	j := n - 1
	if y[j] < 0 {
		i = 1
	}
	k := abs(y[j])

	for j > 0 {
		j--
		i += CELT_PVQ_U(n-j, k)
		k += abs(y[j])
		if y[j] < 0 {
			i += CELT_PVQ_U(n-j, k+1)
		}
	}
	return i
}

// EncodePulses encodes a pulse vector into an index using PVQ
func EncodePulses(y []int, n, k int, enc *entropy.EntropyCoder) {
	Assert(k > 0, "k must be > 0")
	enc.EncUint(icwrs(n, y), CELT_PVQ_V(n, k))
}

// cwrsi decodes an index into a pulse vector using PVQ
func cwrsi(n, k int, index uint32, y []int) int {
	var p, q uint32
	var s, k0 int
	var val int16
	yy := 0
	yPtr := 0

	Assert(k > 0, "k must be > 0")
	Assert(n > 1, "n must be > 1")

	for n > 2 {
		// Lots of pulses case
		if k >= n {
			row := CELT_PVQ_U_ROW[n]
			// Are the pulses in this dimension negative?
			p = CeltPVQUData[row+k+1]
			if index >= p {
				s = -1
				index -= p
			} else {
				s = 0
			}

			// Count how many pulses were placed in this dimension
			k0 = k
			q = CeltPVQUData[row+n]

			if q > index {
				Assert(p > q, "p must be > q")
				k = n
				for {
					k--
					p = CeltPVQUData[CELT_PVQ_U_ROW[k]+n]
					if p <= index {
						break
					}
				}
			} else {
				p = CeltPVQUData[row+k]
				for p > index {
					k--
					p = CeltPVQUData[row+k]
				}
			}

			index -= p
			val = int16((k0 - k + s) ^ s)
			y[yPtr] = int(val)
			yPtr++
			yy = MAC16_16(yy, val, val)
		} else { // Lots of dimensions case
			// Are there any pulses in this dimension at all?
			p = CeltPVQUData[CELT_PVQ_U_ROW[k]+n]
			q = CeltPVQUData[CELT_PVQ_U_ROW[k+1]+n]

			if p <= index && index < q {
				index -= p
				y[yPtr] = 0
				yPtr++
			} else {
				// Are the pulses in this dimension negative?
				if index >= q {
					s = -1
					index -= q
				} else {
					s = 0
				}

				// Count how many pulses were placed in this dimension
				k0 = k
				for {
					k--
					p = CeltPVQUData[CELT_PVQ_U_ROW[k]+n]
					if p <= index {
						break
					}
				}

				index -= p
				val = int16((k0 - k + s) ^ s)
				y[yPtr] = int(val)
				yPtr++
				yy = MAC16_16(yy, val, val)
			}
		}
		n--
	}

	// n == 2
	p = uint32(2*k + 1)
	if index >= p {
		s = -1
		index -= p
	} else {
		s = 0
	}

	k0 = k
	k = int((index + 1) >> 1)
	if k != 0 {
		index -= uint32(2*k - 1)
	}

	val = int16((k0 - k + s) ^ s)
	y[yPtr] = int(val)
	yPtr++
	yy = MAC16_16(yy, val, val)

	// n == 1
	s = -int(index)
	val = int16((k + s) ^ s)
	y[yPtr] = int(val)
	yy = MAC16_16(yy, val, val)

	return yy
}

// DecodePulses decodes an index into a pulse vector using PVQ
func DecodePulses(y []int, n, k int, dec *entropy.EntropyCoder) int {
	return cwrsi(n, k, dec.DecUint(CELT_PVQ_V(n, k)), y)
}

// Helper functions

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Assert checks a condition and panics if it's false
func Assert(condition bool, message string) {
	if !condition {
		panic("Assertion failed: " + message)
	}
}
