package vorbis

// VorbisLayout represents the channel layout configuration for Vorbis audio.
// It specifies how many streams, coupled streams, and the channel mapping.
type VorbisLayout struct {
	// Number of separate audio streams
	NbStreams int
	// Number of coupled stereo streams
	NbCoupledStreams int
	// Channel mapping from output channels to streams
	Mapping []int16
}

// NewVorbisLayout creates a new VorbisLayout instance.
// This is the idiomatic Go way to create new struct instances with validation if needed.
func NewVorbisLayout(streams, coupledStreams int, mapping []int16) *VorbisLayout {
	return &VorbisLayout{
		NbStreams:        streams,
		NbCoupledStreams: coupledStreams,
		Mapping:          mapping,
	}
}

// VorbisMappings contains predefined channel layouts for various channel counts.
// Index is nb_channel-1 (0-based index for channel counts 1 through 8)
var VorbisMappings = []*VorbisLayout{
	// 1: mono
	NewVorbisLayout(1, 0, []int16{0}),
	// 2: stereo
	NewVorbisLayout(1, 1, []int16{0, 1}),
	// 3: 1-d surround
	NewVorbisLayout(2, 1, []int16{0, 2, 1}),
	// 4: quadraphonic surround
	NewVorbisLayout(2, 2, []int16{0, 1, 2, 3}),
	// 5: 5-channel surround
	NewVorbisLayout(3, 2, []int16{0, 4, 1, 2, 3}),
	// 6: 5.1 surround
	NewVorbisLayout(4, 2, []int16{0, 4, 1, 2, 3, 5}),
	// 7: 6.1 surround
	NewVorbisLayout(4, 3, []int16{0, 4, 1, 2, 3, 5, 6}),
	// 8: 7.1 surround
	NewVorbisLayout(5, 3, []int16{0, 6, 1, 2, 3, 4, 5, 7}),
}
