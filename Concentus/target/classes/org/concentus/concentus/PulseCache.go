// Copyright (c) 2007-2008 CSIRO
// Copyright (c) 2007-2011 Xiph.Org Foundation
// Originally written by Jean-Marc Valin, Gregory Maxwell, Koen Vos,
// Timothy B. Terriberry, and the Opus open-source contributors
// Ported to Java by Logan Stromberg
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

// PulseCache represents a cache structure for pulse data.
// In Go, we use a struct instead of a class and follow Go naming conventions.
type PulseCache struct {
	size  int     // Current size of the cache
	index []int16 // Using int16 which is equivalent to Java's short
	bits  []int16 // Slice is more idiomatic than array pointers in Go
	caps  []int16 // Using slices for dynamic arrays
}

// Reset clears the PulseCache, setting it back to its zero state.
// In Go, we use the receiver type and follow the convention of capitalizing exported methods.
func (pc *PulseCache) Reset() {
	pc.size = 0

	// In Go, setting slices to nil is the proper way to release memory
	// Unlike Java where you set array references to null
	pc.index = nil
	pc.bits = nil
	pc.caps = nil
}
