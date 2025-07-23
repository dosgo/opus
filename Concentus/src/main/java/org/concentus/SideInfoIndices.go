package opus
type SideInfoIndices struct {
    GainsIndices [MAX_NB_SUBFR]byte
    LTPIndex [MAX_NB_SUBFR]byte
    NLSFIndices [MAX_LPC_ORDER + 1]byte
    lagIndex int16
    contourIndex byte
    signalType byte
    quantOffsetType byte
    NLSFInterpCoef_Q2 byte
    PERIndex byte
    LTP_scaleIndex byte
    Seed byte
}

func (si *SideInfoIndices) Reset() {
    *si = SideInfoIndices{}
}

func (si *SideInfoIndices) Assign(other *SideInfoIndices) {
    *si = *other
}