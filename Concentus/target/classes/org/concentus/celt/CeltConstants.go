package celt

const (
	// Q15ONE represents 1.0 in Q15 fixed-point format (32767)
	Q15ONE = 32767

	// CELT_SIG_SCALE is the signal scaling factor (32768.0)
	CELT_SIG_SCALE = 32768.0

	// SIG_SHIFT is the signal shift value (12)
	SIG_SHIFT = 12

	// NORM_SCALING is the normalization scaling factor (16384)
	NORM_SCALING = 16384

	// DB_SHIFT is the decibel shift value (10)
	DB_SHIFT = 10

	// EPSILON is a very small value used in calculations (1)
	EPSILON = 1

	// VERY_SMALL represents a very small value (0)
	VERY_SMALL = 0

	// VERY_LARGE16 is a very large 16-bit value (32767)
	VERY_LARGE16 = 32767

	// Q15_ONE represents 1.0 in Q15 fixed-point format (32767)
	Q15_ONE = 32767

	// COMBFILTER_MAXPERIOD is the maximum period for comb filter (1024)
	COMBFILTER_MAXPERIOD = 1024

	// COMBFILTER_MINPERIOD is the minimum period for comb filter (15)
	COMBFILTER_MINPERIOD = 15

	// DECODE_BUFFER_SIZE is the size of the decode buffer (2048)
	DECODE_BUFFER_SIZE = 2048

	// BITALLOC_SIZE is the bit allocation size (11)
	BITALLOC_SIZE = 11

	// MAX_PERIOD is the maximum period value (1024)
	MAX_PERIOD = 1024

	// TOTAL_MODES is the total number of modes (1)
	TOTAL_MODES = 1

	// MAX_PSEUDO is the maximum pseudo value (40)
	MAX_PSEUDO = 40

	// LOG_MAX_PSEUDO is the log of maximum pseudo value (6)
	LOG_MAX_PSEUDO = 6

	// CELT_MAX_PULSES is the maximum number of pulses (128)
	CELT_MAX_PULSES = 128

	// MAX_FINE_BITS is the maximum number of fine bits (8)
	MAX_FINE_BITS = 8

	// FINE_OFFSET is the fine offset value (21)
	FINE_OFFSET = 21

	// QTHETA_OFFSET is the theta quantization offset (4)
	QTHETA_OFFSET = 4

	// QTHETA_OFFSET_TWOPHASE is the two-phase theta quantization offset (16)
	QTHETA_OFFSET_TWOPHASE = 16

	// PLC_PITCH_LAG_MAX is the maximum pitch lag for PLC (720)
	// Corresponds to a pitch of 66.67 Hz
	PLC_PITCH_LAG_MAX = 720

	// PLC_PITCH_LAG_MIN is the minimum pitch lag for PLC (100)
	// Corresponds to a pitch of 480 Hz
	PLC_PITCH_LAG_MIN = 100

	// LPC_ORDER is the LPC order (24)
	LPC_ORDER = 24
)
