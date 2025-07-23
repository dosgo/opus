package concentus

// DecoderAPIFlag defines constants for decoder API flags.
// In Go, we use typed constants for better type safety and documentation.
const (
	// FLAG_DECODE_NORMAL indicates normal decoding mode
	FLAG_DECODE_NORMAL = 0

	// FLAG_PACKET_LOST indicates that a packet was lost
	FLAG_PACKET_LOST = 1

	// FLAG_DECODE_LBRR indicates LBRR (Low Bitrate Redundancy) decoding mode
	FLAG_DECODE_LBRR = 2
)
