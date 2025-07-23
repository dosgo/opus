// Package silk defines error codes for the Silk encoder/decoder.
// Original Java code ported to Go with idiomatic style.
package silk

// Error codes for Silk encoder/decoder operations.
const (
	// SILK_NO_ERROR indicates successful operation with no errors.
	SILK_NO_ERROR = 0

	// Encoder error messages

	// SILK_ENC_INPUT_INVALID_NO_OF_SAMPLES indicates input length is not a multiple
	// of 10 ms, or length is longer than the packet length.
	SILK_ENC_INPUT_INVALID_NO_OF_SAMPLES = -101

	// SILK_ENC_FS_NOT_SUPPORTED indicates sampling frequency not 8000, 12000 or 16000 Hertz.
	SILK_ENC_FS_NOT_SUPPORTED = -102

	// SILK_ENC_PACKET_SIZE_NOT_SUPPORTED indicates packet size not 10, 20, 40, or 60 ms.
	SILK_ENC_PACKET_SIZE_NOT_SUPPORTED = -103

	// SILK_ENC_PAYLOAD_BUF_TOO_SHORT indicates allocated payload buffer too short.
	SILK_ENC_PAYLOAD_BUF_TOO_SHORT = -104

	// SILK_ENC_INVALID_LOSS_RATE indicates loss rate not between 0 and 100 percent.
	SILK_ENC_INVALID_LOSS_RATE = -105

	// SILK_ENC_INVALID_COMPLEXITY_SETTING indicates complexity setting not valid (use 0...10).
	SILK_ENC_INVALID_COMPLEXITY_SETTING = -106

	// SILK_ENC_INVALID_INBAND_FEC_SETTING indicates inband FEC setting not valid (use 0 or 1).
	SILK_ENC_INVALID_INBAND_FEC_SETTING = -107

	// SILK_ENC_INVALID_DTX_SETTING indicates DTX setting not valid (use 0 or 1).
	SILK_ENC_INVALID_DTX_SETTING = -108

	// SILK_ENC_INVALID_CBR_SETTING indicates CBR setting not valid (use 0 or 1).
	SILK_ENC_INVALID_CBR_SETTING = -109

	// SILK_ENC_INTERNAL_ERROR indicates internal encoder error.
	SILK_ENC_INTERNAL_ERROR = -110

	// SILK_ENC_INVALID_NUMBER_OF_CHANNELS_ERROR indicates invalid number of channels.
	SILK_ENC_INVALID_NUMBER_OF_CHANNELS_ERROR = -111

	// Decoder error messages

	// SILK_DEC_INVALID_SAMPLING_FREQUENCY indicates output sampling frequency lower than
	// internal decoded sampling frequency.
	SILK_DEC_INVALID_SAMPLING_FREQUENCY = -200

	// SILK_DEC_PAYLOAD_TOO_LARGE indicates payload size exceeded the maximum allowed 1024 bytes.
	SILK_DEC_PAYLOAD_TOO_LARGE = -201

	// SILK_DEC_PAYLOAD_ERROR indicates payload has bit errors.
	SILK_DEC_PAYLOAD_ERROR = -202

	// SILK_DEC_INVALID_FRAME_SIZE indicates invalid frame size.
	SILK_DEC_INVALID_FRAME_SIZE = -203
)
