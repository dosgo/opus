package concentus

// Package concentus provides audio processing functionality
// Original Java code Copyright notices:
// Copyright (c) 2007-2008 CSIRO
// Copyright (c) 2007-2011 Xiph.Org Foundation
// Originally written by Jean-Marc Valin, Gregory Maxwell, Koen Vos,
// Timothy B. Terriberry, and the Opus open-source contributors
// Ported to Java by Logan Stromberg

// StereoWidthState represents the state for stereo width processing
type StereoWidthState struct {
	XX            int // Cross-correlation of left channel with itself
	XY            int // Cross-correlation between left and right channels
	YY            int // Cross-correlation of right channel with itself
	smoothedWidth int // Smoothed width value
	maxFollower   int // Maximum follower value (used for smoothing/limiting)
}

// Reset initializes all state variables to zero
func (s *StereoWidthState) Reset() {
	s.XX = 0
	s.XY = 0
	s.YY = 0
	s.smoothedWidth = 0
	s.maxFollower = 0
}
