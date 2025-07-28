package opus

type SideInfoIndices struct {
	GainsIndices      []byte
	LTPIndex          []byte
	NLSFIndices       []byte
	lagIndex          int16
	contourIndex      byte
	signalType        byte
	quantOffsetType   byte
	NLSFInterpCoef_Q2 byte
	PERIndex          int8
	LTP_scaleIndex    byte
	Seed              byte
}

func (si *SideInfoIndices) Reset() {
	*si = SideInfoIndices{}
}

func (si *SideInfoIndices) Assign(other *SideInfoIndices) {
	*si = *other
}
