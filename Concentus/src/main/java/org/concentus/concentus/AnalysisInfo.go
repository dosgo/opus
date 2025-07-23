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

// AnalysisInfo holds audio analysis information including tonality, noisiness,
// activity measurements, and music probability.
type AnalysisInfo struct {
	Enabled       bool    // Whether analysis is enabled
	Valid         int     // Validity flag (0 = invalid)
	Tonality      float32 // Tonality measurement
	TonalitySlope float32 // Tonality slope over time
	Noisiness     float32 // Noisiness measurement
	Activity      float32 // Activity measurement
	MusicProb     float32 // Probability of music (vs speech/noise)
	Bandwidth     int     // Estimated bandwidth
}

// NewAnalysisInfo creates and returns a new initialized AnalysisInfo struct.
// In Go, we typically use constructor functions instead of methods on zero values.
func NewAnalysisInfo() *AnalysisInfo {
	return &AnalysisInfo{
		// All fields are initialized to their zero values by default
		// which matches the Java class's behavior
	}
}

// Assign copies all fields from another AnalysisInfo to this one.
// In Go, we use pointer receivers for methods that modify the struct.
func (a *AnalysisInfo) Assign(other *AnalysisInfo) {
	a.Valid = other.Valid
	a.Tonality = other.Tonality
	a.TonalitySlope = other.TonalitySlope
	a.Noisiness = other.Noisiness
	a.Activity = other.Activity
	a.MusicProb = other.MusicProb
	a.Bandwidth = other.Bandwidth
}

// Reset sets all fields to their zero values (except Enabled).
func (a *AnalysisInfo) Reset() {
	a.Valid = 0
	a.Tonality = 0
	a.TonalitySlope = 0
	a.Noisiness = 0
	a.Activity = 0
	a.MusicProb = 0
	a.Bandwidth = 0
}
