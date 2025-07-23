// Copyright (c) 2016 Logan Stromberg
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// - Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//
// - Redistributions in binary form must reproduce the above copyright
// notice, this list of conditions and the following disclaimer in the
// documentation and/or other materials provided with the distribution.
//
// - Neither the name of Internet Society, IETF or IETF Trust, nor the
// names of specific contributors, may be used to endorse or promote
// products derived from this software without specific prior written
// permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// ``AS IS'' AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER
// OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL,
// EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
// PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF
// LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
// NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package concentus

// Arrays provides utility functions for working with slices in Go, similar to
// the Java Arrays utility class but adapted for Go idioms.
type Arrays struct{}

// InitTwoDimensionalArrayInt creates a 2D int slice with the specified dimensions.
// In Go, we return a slice of slices rather than a fixed-size array.
func (a Arrays) InitTwoDimensionalArrayInt(x, y int) [][]int {
	matrix := make([][]int, x)
	for i := range matrix {
		matrix[i] = make([]int, y)
	}
	return matrix
}

// InitTwoDimensionalArrayFloat creates a 2D float32 slice with the specified dimensions.
// Note: Go uses float32 for single-precision floating point (equivalent to Java's float).
func (a Arrays) InitTwoDimensionalArrayFloat(x, y int) [][]float32 {
	matrix := make([][]float32, x)
	for i := range matrix {
		matrix[i] = make([]float32, y)
	}
	return matrix
}

// InitTwoDimensionalArrayShort creates a 2D int16 slice with the specified dimensions.
// Note: Go uses int16 for 16-bit integers (equivalent to Java's short).
func (a Arrays) InitTwoDimensionalArrayShort(x, y int) [][]int16 {
	matrix := make([][]int16, x)
	for i := range matrix {
		matrix[i] = make([]int16, y)
	}
	return matrix
}

// InitTwoDimensionalArrayByte creates a 2D byte slice with the specified dimensions.
func (a Arrays) InitTwoDimensionalArrayByte(x, y int) [][]byte {
	matrix := make([][]byte, x)
	for i := range matrix {
		matrix[i] = make([]byte, y)
	}
	return matrix
}

// InitThreeDimensionalArrayByte creates a 3D byte slice with the specified dimensions.
func (a Arrays) InitThreeDimensionalArrayByte(x, y, z int) [][][]byte {
	matrix := make([][][]byte, x)
	for i := range matrix {
		matrix[i] = make([][]byte, y)
		for j := range matrix[i] {
			matrix[i][j] = make([]byte, z)
		}
	}
	return matrix
}

// MemSet fills a byte slice with the specified value.
// This is equivalent to Java's Arrays.fill() but uses Go's copy() optimization.
func (a Arrays) MemSet(array []byte, value byte) {
	if len(array) == 0 {
		return
	}
	array[0] = value
	for i := 1; i < len(array); i *= 2 {
		copy(array[i:], array[:i])
	}
}

// MemSet fills an int16 slice with the specified value.
func (a Arrays) MemSetShort(array []int16, value int16) {
	if len(array) == 0 {
		return
	}
	array[0] = value
	for i := 1; i < len(array); i *= 2 {
		copy(array[i:], array[:i])
	}
}

// MemSet fills an int slice with the specified value.
func (a Arrays) MemSetInt(array []int, value int) {
	if len(array) == 0 {
		return
	}
	array[0] = value
	for i := 1; i < len(array); i *= 2 {
		copy(array[i:], array[:i])
	}
}

// MemSet fills a float32 slice with the specified value.
func (a Arrays) MemSetFloat(array []float32, value float32) {
	if len(array) == 0 {
		return
	}
	array[0] = value
	for i := 1; i < len(array); i *= 2 {
		copy(array[i:], array[:i])
	}
}

// MemSet fills a portion of a byte slice with the specified value.
func (a Arrays) MemSetLen(array []byte, value byte, length int) {
	if length <= 0 {
		return
	}
	end := length
	if end > len(array) {
		end = len(array)
	}
	sub := array[:end]
	if len(sub) == 0 {
		return
	}
	sub[0] = value
	for i := 1; i < len(sub); i *= 2 {
		copy(sub[i:], sub[:i])
	}
}

// MemSet fills a portion of an int16 slice with the specified value.
func (a Arrays) MemSetShortLen(array []int16, value int16, length int) {
	if length <= 0 {
		return
	}
	end := length
	if end > len(array) {
		end = len(array)
	}
	sub := array[:end]
	if len(sub) == 0 {
		return
	}
	sub[0] = value
	for i := 1; i < len(sub); i *= 2 {
		copy(sub[i:], sub[:i])
	}
}

// MemSet fills a portion of an int slice with the specified value.
func (a Arrays) MemSetIntLen(array []int, value int, length int) {
	if length <= 0 {
		return
	}
	end := length
	if end > len(array) {
		end = len(array)
	}
	sub := array[:end]
	if len(sub) == 0 {
		return
	}
	sub[0] = value
	for i := 1; i < len(sub); i *= 2 {
		copy(sub[i:], sub[:i])
	}
}

// MemSet fills a portion of a float32 slice with the specified value.
func (a Arrays) MemSetFloatLen(array []float32, value float32, length int) {
	if length <= 0 {
		return
	}
	end := length
	if end > len(array) {
		end = len(array)
	}
	sub := array[:end]
	if len(sub) == 0 {
		return
	}
	sub[0] = value
	for i := 1; i < len(sub); i *= 2 {
		copy(sub[i:], sub[:i])
	}
}

// MemSetWithOffset fills a portion of a byte slice starting at offset with the specified value.
func (a Arrays) MemSetWithOffset(array []byte, value byte, offset, length int) {
	if length <= 0 || offset < 0 || offset >= len(array) {
		return
	}
	end := offset + length
	if end > len(array) {
		end = len(array)
	}
	sub := array[offset:end]
	if len(sub) == 0 {
		return
	}
	sub[0] = value
	for i := 1; i < len(sub); i *= 2 {
		copy(sub[i:], sub[:i])
	}
}

// MemSetWithOffset fills a portion of an int16 slice starting at offset with the specified value.
func (a Arrays) MemSetWithOffsetShort(array []int16, value int16, offset, length int) {
	if length <= 0 || offset < 0 || offset >= len(array) {
		return
	}
	end := offset + length
	if end > len(array) {
		end = len(array)
	}
	sub := array[offset:end]
	if len(sub) == 0 {
		return
	}
	sub[0] = value
	for i := 1; i < len(sub); i *= 2 {
		copy(sub[i:], sub[:i])
	}
}

// MemSetWithOffset fills a portion of an int slice starting at offset with the specified value.
func (a Arrays) MemSetWithOffsetInt(array []int, value int, offset, length int) {
	if length <= 0 || offset < 0 || offset >= len(array) {
		return
	}
	end := offset + length
	if end > len(array) {
		end = len(array)
	}
	sub := array[offset:end]
	if len(sub) == 0 {
		return
	}
	sub[0] = value
	for i := 1; i < len(sub); i *= 2 {
		copy(sub[i:], sub[:i])
	}
}

// MemMove copies elements within a byte slice from source to destination.
// This is equivalent to Java's System.arraycopy() but uses Go's copy().
func (a Arrays) MemMove(array []byte, srcIdx, dstIdx, length int) {
	if length <= 0 || srcIdx < 0 || dstIdx < 0 ||
		srcIdx >= len(array) || dstIdx >= len(array) {
		return
	}
	copy(array[dstIdx:dstIdx+length], array[srcIdx:srcIdx+length])
}

// MemMove copies elements within an int16 slice from source to destination.
func (a Arrays) MemMoveShort(array []int16, srcIdx, dstIdx, length int) {
	if length <= 0 || srcIdx < 0 || dstIdx < 0 ||
		srcIdx >= len(array) || dstIdx >= len(array) {
		return
	}
	copy(array[dstIdx:dstIdx+length], array[srcIdx:srcIdx+length])
}

// MemMove copies elements within an int slice from source to destination.
func (a Arrays) MemMoveInt(array []int, srcIdx, dstIdx, length int) {
	if length <= 0 || srcIdx < 0 || dstIdx < 0 ||
		srcIdx >= len(array) || dstIdx >= len(array) {
		return
	}
	copy(array[dstIdx:dstIdx+length], array[srcIdx:srcIdx+length])
}
