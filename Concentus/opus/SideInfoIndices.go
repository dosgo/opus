package opus

import "bytes"

type SideInfoIndices struct {
	GainsIndices      []byte
	LTPIndex          []byte
	NLSFIndices       []int8
	lagIndex          int16
	contourIndex      int8
	signalType        byte
	quantOffsetType   byte
	NLSFInterpCoef_Q2 byte
	PERIndex          int8
	LTP_scaleIndex    byte
	Seed              byte
}

func NewSideInfoIndices() *SideInfoIndices {
	obj := &SideInfoIndices{}
	obj.GainsIndices = make([]byte, SilkConstants.MAX_NB_SUBFR)
	obj.LTPIndex = make([]byte, SilkConstants.MAX_NB_SUBFR)
	obj.NLSFIndices = make([]int8, SilkConstants.MAX_LPC_ORDER+1)
	return obj
}
func (si *SideInfoIndices) Reset() {
	copy(si.GainsIndices, bytes.Repeat([]byte{0}, len(si.GainsIndices)))
	copy(si.LTPIndex, bytes.Repeat([]byte{0}, len(si.LTPIndex)))
	for i := range si.NLSFIndices {
		si.NLSFIndices[i] = 0
	}
	si.lagIndex = 0
	si.contourIndex = 0
	si.signalType = 0
	si.quantOffsetType = 0
	si.NLSFInterpCoef_Q2 = 0
	si.PERIndex = 0
	si.LTP_scaleIndex = 0
	si.Seed = 0
}

func (si *SideInfoIndices) Assign(other *SideInfoIndices) {

	copy(si.GainsIndices, other.GainsIndices)
	copy(si.LTPIndex, other.LTPIndex)
	copy(si.NLSFIndices, other.NLSFIndices)
	si.lagIndex = other.lagIndex
	si.contourIndex = other.contourIndex
	si.signalType = other.signalType
	si.quantOffsetType = other.quantOffsetType
	si.NLSFInterpCoef_Q2 = other.NLSFInterpCoef_Q2
	si.PERIndex = other.PERIndex
	si.LTP_scaleIndex = other.LTP_scaleIndex
	si.Seed = other.Seed
}
