package opus
type TOCStruct struct {
    VADFlag        int
    VADFlags       [SILK_MAX_FRAMES_PER_PACKET]int
    inbandFECFlag  int
}

func (t *TOCStruct) Reset() {
    *t = TOCStruct{}
}