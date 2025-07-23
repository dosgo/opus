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

import "fmt"

// OpusError represents an Opus-specific error with an associated error code.
type OpusError struct {
	// The complete error message
	message string
	// The underlying Opus error code
	code int
}

// NewOpusError creates a new OpusError with a message and error code.
// The message is combined with the Opus error description from the code.
func NewOpusError(message string, code int) *OpusError {
	return &OpusError{
		message: fmt.Sprintf("%s: %s", message, opusStrerror(code)),
		code:    code,
	}
}

// Error implements the error interface, returning the complete error message.
func (e *OpusError) Error() string {
	return e.message
}

// Code returns the underlying Opus error code.
func (e *OpusError) Code() int {
	return e.code
}

// opusStrerror is a placeholder for the actual Opus error string function.
// This should be implemented to return the appropriate error string for the given code.
func opusStrerror(code int) string {
	// Implementation should map Opus error codes to their string representations
	return fmt.Sprintf("Opus error %d", code)
}
