// Copyright (c) 2006-2011 Skype Limited. All Rights Reserved
// Ported to Go by [Your Name]
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

// RegularizeCorrelations adds noise to matrix diagonal to regularize correlation matrices
//
// This is a direct translation from the Java version, but with Go idioms:
// 1. Uses slices instead of arrays with explicit pointers
// 2. Uses simple array indexing instead of MatrixGet/MatrixSet helper functions
// 3. Follows Go naming conventions (camelCase instead of snake_case)
func regularizeCorrelations(
	XX []int, // I/O Correlation matrices (represented as a flattened DxD matrix)
	xx []int, // I/O Correlation values
	noise int, // I Noise to add to diagonal elements
	D int, // I Dimension of XX matrix (DxD)
) {
	// In Go, we can directly access matrix elements since we're using a slice
	// The matrix is stored in row-major order (same as the Java version)
	for i := 0; i < D; i++ {
		// Diagonal elements are at positions i*D + i
		XX[i*D+i] += noise
	}

	// Add noise to the first element of xx (equivalent to xx[xx_ptr] in Java)
	if len(xx) > 0 {
		xx[0] += noise
	}
}
