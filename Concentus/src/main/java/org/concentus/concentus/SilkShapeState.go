// Copyright (c) 2006-2011 Skype Limited. All Rights Reserved
// Ported to Go from Java
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

// Package concentus contains noise shaping analysis state functionality
package concentus

// ShapeState represents noise shaping analysis state
type ShapeState struct {
	// LastGainIndex stores the last gain index used
	LastGainIndex byte

	// HarmBoost_smth_Q16 represents harmonic boost smoothed value in Q16 format
	HarmBoost_smth_Q16 int32

	// HarmShapeGain_smth_Q16 represents harmonic shape gain smoothed value in Q16 format
	HarmShapeGain_smth_Q16 int32

	// Tilt_smth_Q16 represents tilt smoothed value in Q16 format
	Tilt_smth_Q16 int32
}

// NewShapeState creates a new initialized ShapeState
func NewShapeState() *ShapeState {
	return &ShapeState{
		LastGainIndex:          0,
		HarmBoost_smth_Q16:     0,
		HarmShapeGain_smth_Q16: 0,
		Tilt_smth_Q16:          0,
	}
}

// Reset reinitializes the ShapeState to its default values
func (s *ShapeState) Reset() {
	s.LastGainIndex = 0
	s.HarmBoost_smth_Q16 = 0
	s.HarmShapeGain_smth_Q16 = 0
	s.Tilt_smth_Q16 = 0
}
