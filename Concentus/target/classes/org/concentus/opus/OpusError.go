// Package opus provides error codes for the Opus audio codec.
// Note that since most API-level errors are detected and returned as error types,
// direct use of these constants is not usually needed.
package opus

// Error codes for Opus operations.
// These constants mirror the official Opus error codes.
const (
	// OpusOK indicates no error
	OpusOK = 0

	// OpusBadArg indicates one or more invalid/out of range arguments
	OpusBadArg = -1

	// OpusBufferTooSmall indicates not enough bytes allocated in the buffer
	OpusBufferTooSmall = -2

	// OpusInternalError indicates an internal error was detected
	OpusInternalError = -3

	// OpusInvalidPacket indicates the compressed data passed is corrupted
	OpusInvalidPacket = -4

	// OpusUnimplemented indicates invalid/unsupported request number
	OpusUnimplemented = -5

	// OpusInvalidState indicates an encoder or decoder structure is invalid or already freed
	OpusInvalidState = -6

	// OpusAllocFail indicates memory allocation has failed
	OpusAllocFail = -7
)

// String returns a human-readable description of the Opus error code.
func (code int) String() string {
	switch code {
	case OpusOK:
		return "no error"
	case OpusBadArg:
		return "invalid argument"
	case OpusBufferTooSmall:
		return "buffer too small"
	case OpusInternalError:
		return "internal error"
	case OpusInvalidPacket:
		return "invalid packet"
	case OpusUnimplemented:
		return "unimplemented feature"
	case OpusInvalidState:
		return "invalid state"
	case OpusAllocFail:
		return "allocation failed"
	default:
		return "unknown error"
	}
}
