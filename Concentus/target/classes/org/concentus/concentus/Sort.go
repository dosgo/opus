package concentus

// Sort provides various insertion sort implementations optimized for different use cases.
type Sort struct{}

// InsertionSortIncreasing performs an insertion sort on the first K elements of a slice,
// tracking original indices, and efficiently checks remaining elements.
//
// Parameters:
//
//	a - The slice to be sorted (modified in-place)
//	idx - Slice to store original indices of sorted elements (must be same length as a)
//	L - Total length of the input slice
//	K - Number of elements to fully sort
//
// Notes:
//   - Uses Go's slice bounds checking which is safer than manual checks
//   - Maintains same algorithm but with Go idioms (range loops where appropriate)
//   - Assumes input validation done by caller (like Java assertions)
func (s *Sort) InsertionSortIncreasing(a, idx []int, L, K int) {
	// Write start indices in index slice
	for i := 0; i < K; i++ {
		idx[i] = i
	}

	// Sort first K elements in increasing order
	for i := 1; i < K; i++ {
		value := a[i]
		j := i - 1

		// Shift elements greater than value to the right
		for ; j >= 0 && value < a[j]; j-- {
			a[j+1] = a[j]
			idx[j+1] = idx[j]
		}

		// Insert value at correct position
		a[j+1] = value
		idx[j+1] = i
	}

	// Check remaining elements but only maintain first K sorted
	for i := K; i < L; i++ {
		value := a[i]
		if value < a[K-1] {
			j := K - 2
			for ; j >= 0 && value < a[j]; j-- {
				a[j+1] = a[j]
				idx[j+1] = idx[j]
			}
			a[j+1] = value
			idx[j+1] = i
		}
	}
}

// InsertionSortIncreasingAllValuesInt16 sorts an entire int16 slice in increasing order.
//
// Parameters:
//
//	a - The slice to be sorted (modified in-place)
//	L - Length of the slice to sort
//
// Notes:
//   - Simpler than the indexed version since we don't track original positions
//   - Uses int16 type which is Go's equivalent of Java's short
//   - Could be replaced with sort.Slice in most cases but maintains original algorithm
func (s *Sort) InsertionSortIncreasingAllValuesInt16(a []int16, L int) {
	for i := 1; i < L; i++ {
		value := a[i]
		j := i - 1
		for ; j >= 0 && value < a[j]; j-- {
			a[j+1] = a[j]
		}
		a[j+1] = value
	}
}

// InsertionSortDecreasingInt16 performs a decreasing order insertion sort on first K elements,
// tracking original indices, and efficiently checks remaining elements.
//
// Parameters:
//
//	a - The slice to be sorted (modified in-place)
//	idx - Slice to store original indices of sorted elements (must be same length as a)
//	L - Total length of the input slice
//	K - Number of elements to fully sort
//
// Notes:
//   - Similar to InsertionSortIncreasing but with opposite comparison
//   - Uses int16 type for values but int for indices (matches Java behavior)
func (s *Sort) InsertionSortDecreasingInt16(a []int16, idx []int, L, K int) {
	// Write start indices in index slice
	for i := 0; i < K; i++ {
		idx[i] = i
	}

	// Sort first K elements in decreasing order
	for i := 1; i < K; i++ {
		value := a[i]
		j := i - 1
		for ; j >= 0 && value > a[j]; j-- {
			a[j+1] = a[j]
			idx[j+1] = idx[j]
		}
		a[j+1] = value
		idx[j+1] = i
	}

	// Check remaining elements but only maintain first K sorted
	for i := K; i < L; i++ {
		value := a[i]
		if value > a[K-1] {
			j := K - 2
			for ; j >= 0 && value > a[j]; j-- {
				a[j+1] = a[j]
				idx[j+1] = idx[j]
			}
			a[j+1] = value
			idx[j+1] = i
		}
	}
}
