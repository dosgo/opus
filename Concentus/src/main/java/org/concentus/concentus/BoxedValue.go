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

// Byte is a boxed byte value.
// In Go, we typically use pointers to primitive types when we need reference semantics.
type Byte struct {
	Val byte
}

// NewByte creates a new boxed byte value.
// Go convention is to use NewXxx constructors rather than direct struct initialization.
func NewByte(v byte) *Byte {
	return &Byte{Val: v}
}

// Short is a boxed short value (int16 in Go).
// Go doesn't have a separate 'short' type - int16 is the equivalent.
type Short struct {
	Val int16
}

// NewShort creates a new boxed int16 value.
func NewShort(v int16) *Short {
	return &Short{Val: v}
}

// Int is a boxed int value (int32 in Go).
// Note: In Go, 'int' is architecture-dependent (32 or 64 bits).
// For consistent 32-bit integers, we use int32.
type Int struct {
	Val int32
}

// NewInt creates a new boxed int32 value.
func NewInt(v int32) *Int {
	return &Int{Val: v}
}
