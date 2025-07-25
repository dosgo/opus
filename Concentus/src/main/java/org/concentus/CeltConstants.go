package opus
var CeltConstants = struct {
    Q15ONE                     int32
    CELT_SIG_SCALE             float32
    SIG_SHIFT                  int32
    NORM_SCALING               int32
    DB_SHIFT                   int32
    EPSILON                    int32
    VERY_SMALL                 int32
    VERY_LARGE16               int16
    Q15_ONE                    int16
    COMBFILTER_MAXPERIOD       int32
    COMBFILTER_MINPERIOD       int32
    DECODE_BUFFER_SIZE         int32
    BITALLOC_SIZE              int32
    MAX_PERIOD                 int32
    TOTAL_MODES                int32
    MAX_PSEUDO                 int32
    LOG_MAX_PSEUDO             int32
    CELT_MAX_PULSES            int32
    MAX_FINE_BITS              int32
    FINE_OFFSET                int32
    QTHETA_OFFSET              int32
    QTHETA_OFFSET_TWOPHASE     int32
    PLC_PITCH_LAG_MAX          int32
    PLC_PITCH_LAG_MIN          int32
    LPC_ORDER                  int32
}{
    Q15ONE:                 32767,
    CELT_SIG_SCALE:         32768.0,
    SIG_SHIFT:              12,
    NORM_SCALING:           16384,
    DB_SHIFT:               10,
    EPSILON:                1,
    VERY_SMALL:             0,
    VERY_LARGE16:           32767,
    Q15_ONE:                32767,
    COMBFILTER_MAXPERIOD:   1024,
    COMBFILTER_MINPERIOD:   15,
    DECODE_BUFFER_SIZE:     2048,
    BITALLOC_SIZE:          11,
    MAX_PERIOD:             1024,
    TOTAL_MODES:            1,
    MAX_PSEUDO:             40,
    LOG_MAX_PSEUDO:         6,
    CELT_MAX_PULSES:        128,
    MAX_FINE_BITS:          8,
    FINE_OFFSET:            21,
    QTHETA_OFFSET:          4,
    QTHETA_OFFSET_TWOPHASE: 16,
    PLC_PITCH_LAG_MAX:      720,
    PLC_PITCH_LAG_MIN:      100,
    LPC_ORDER:              24,
}