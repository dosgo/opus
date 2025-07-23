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

// Constants for Opus codec configuration and operation
const (
	// OPUS_AUTO represents an auto/default setting
	OPUS_AUTO = -1000

	// OPUS_BITRATE_MAX represents the maximum bitrate
	OPUS_BITRATE_MAX = -1

	// From analysis.c:
	NB_FRAMES           = 8   // Number of frames
	NB_TBANDS           = 18  // Number of tonal bands
	NB_TOT_BANDS        = 21  // Total number of bands
	NB_TONAL_SKIP_BANDS = 9   // Number of tonal skip bands
	ANALYSIS_BUF_SIZE   = 720 // Analysis buffer size
	DETECT_SIZE         = 200 // 15 ms at 48 kHz
	MAX_ENCODER_BUFFER  = 480 // Maximum encoder buffer size
)
