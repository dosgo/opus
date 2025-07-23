
package resampler

import (
	"errors"
	"math"
)

// Resampler implements arbitrary sample rate conversion
type Resampler struct {
	inRate        int
	outRate       int
	numRate       int
	denRate       int
	quality       int
	nbChannels    int
	filtLen       int
	memAllocSize  int
	bufferSize    int
	intAdvance    int
	fracAdvance   int
	cutoff        float32
	oversample    int
	initialised   bool
	started       bool
	lastSample    []int
	sampFracNum   []int
	magicSamples  []int
	mem           []int16
	sincTable     []int16
	sincTableLen  int
	inStride      int
	outStride     int
	resamplerFunc func(channelIndex int, input []int16, inputPtr int, inLen *int, output []int16, outputPtr int, outLen *int) int
}

// QualityMapping defines the parameters for each quality level
type QualityMapping struct {
	baseLength          int
	oversample         int
	downsampleBW       float32
	upsampleBW         float32
	windowFunc         *FuncDef
}

// FuncDef defines a window function table
type FuncDef struct {
	table      []float64
	oversample int
}

// New creates a new resampler with integer input/output rates
func New(nbChannels, inRate, outRate, quality int) (*Resampler, error) {
	return NewFractional(nbChannels, inRate, outRate, inRate, outRate, quality)
}

// NewFractional creates a new resampler with fractional rates
func NewFractional(nbChannels, ratioNum, ratioDen, inRate, outRate, quality int) (*Resampler, error) {
	if quality > 10 || quality < 0 {
		return nil, errors.New("quality must be between 0 and 10")
	}

	st := &Resampler{
		nbChannels:   nbChannels,
		inStride:     1,
		outStride:    1,
		bufferSize:   160,
		lastSample:   make([]int, nbChannels),
		magicSamples: make([]int, nbChannels),
		sampFracNum:  make([]int, nbChannels),
	}

	st.SetQuality(quality)
	if err := st.SetRateFraction(ratioNum, ratioDen, inRate, outRate); err != nil {
		return nil, err
	}

	st.updateFilter()
	st.initialised = true

	return st, nil
}

// Result contains resampling operation results
type Result struct {
	InputSamples  int
	OutputSamples int
}

// Process resamples a single channel
func (r *Resampler) Process(channelIndex int, input []int16, inputPtr int, inLen int, output []int16, outputPtr int, outLen int) Result {
	ilen := inLen
	olen := outLen
	x := channelIndex * r.memAllocSize
	filtOffs := r.filtLen - 1
	xlen := r.memAllocSize - filtOffs
	istride := r.inStride

	if r.magicSamples[channelIndex] != 0 {
		used := r.processMagic(channelIndex, output, &outputPtr, olen)
		olen -= used
	}

	if r.magicSamples[channelIndex] == 0 {
		for ilen > 0 && olen > 0 {
			ichunk := ilen
			if ichunk > xlen {
				ichunk = xlen
			}
			ochunk := olen

			if input != nil {
				for j := 0; j < ichunk; j++ {
					r.mem[x+j+filtOffs] = input[inputPtr+j*istride]
				}
			} else {
				for j := 0; j < ichunk; j++ {
					r.mem[x+j+filtOffs] = 0
				}
			}

			r.processNative(channelIndex, &ichunk, output, outputPtr, &ochunk)
			ilen -= ichunk
			olen -= ochunk
			outputPtr += ochunk * r.outStride
			if input != nil {
				inputPtr += ichunk * istride
			}
		}
	}

	return Result{
		InputSamples:  inLen - ilen,
		OutputSamples: outLen - olen,
	}
}

// ProcessInterleaved resamples interleaved channels
func (r *Resampler) ProcessInterleaved(input []int16, inputPtr int, inLen int, output []int16, outputPtr int, outLen int) Result {
	istrideSave := r.inStride
	ostrideSave := r.outStride
	r.inStride = r.nbChannels
	r.outStride = r.nbChannels

	bakOutLen := outLen
	bakInLen := inLen

	for i := 0; i < r.nbChannels; i++ {
		outLen = bakOutLen
		inLen = bakInLen

		var res Result
		if input != nil {
			res = r.Process(i, input, inputPtr+i, inLen, output, outputPtr+i, outLen)
		} else {
			res = r.Process(i, nil, 0, inLen, output, outputPtr+i, outLen)
		}

		inLen = res.InputSamples
		outLen = res.OutputSamples
	}

	r.inStride = istrideSave
	r.outStride = ostrideSave

	return Result{
		InputSamples:  inLen,
		OutputSamples: outLen,
	}
}

// SetQuality sets the resampling quality (0-10)
func (r *Resampler) SetQuality(quality int) error {
	if quality > 10 || quality < 0 {
		return errors.New("quality must be between 0 and 10")
	}
	if r.quality == quality {
		return nil
	}
	r.quality = quality
	if r.initialised {
		r.updateFilter()
	}
	return nil
}

// SetRates sets input/output rates
func (r *Resampler) SetRates(inRate, outRate int) error {
	return r.SetRateFraction(inRate, outRate, inRate, outRate)
}

// SetRateFraction sets fractional rates
func (r *Resampler) SetRateFraction(ratioNum, ratioDen, inRate, outRate int) error {
	if r.inRate == inRate && r.outRate == outRate && r.numRate == ratioNum && r.denRate == ratioDen {
		return nil
	}

	oldDen := r.denRate
	r.inRate = inRate
	r.outRate = outRate
	r.numRate = ratioNum
	r.denRate = ratioDen

	// Simplify the fraction
	fact := 2
	min := ratioNum
	if ratioDen < min {
		min = ratioDen
	}
	for fact <= min {
		for r.numRate%fact == 0 && r.denRate%fact == 0 {
			r.numRate /= fact
			r.denRate /= fact
		}
		fact++
	}

	if oldDen > 0 {
		for i := 0; i < r.nbChannels; i++ {
			r.sampFracNum[i] = r.sampFracNum[i] * r.denRate / oldDen
			if r.sampFracNum[i] >= r.denRate {
				r.sampFracNum[i] = r.denRate - 1
			}
		}
	}

	if r.initialised {
		r.updateFilter()
	}

	return nil
}

// GetRates returns current input/output rates
func (r *Resampler) GetRates() (int, int) {
	return r.inRate, r.outRate
}

// GetRateFraction returns current rate fraction
func (r *Resampler) GetRateFraction() (int, int) {
	return r.numRate, r.denRate
}

// InputLatency returns input latency in samples
func (r *Resampler) InputLatency() int {
	return r.filtLen / 2
}

// OutputLatency returns output latency in samples
func (r *Resampler) OutputLatency() int {
	return ((r.filtLen/2)*r.denRate + (r.numRate >> 1)) / r.numRate
}

// ResetMem clears the resampler buffers
func (r *Resampler) ResetMem() {
	for i := 0; i < r.nbChannels; i++ {
		r.lastSample[i] = 0
		r.magicSamples[i] = 0
		r.sampFracNum[i] = 0
	}
	for i := 0; i < r.nbChannels*(r.filtLen-1); i++ {
		r.mem[i] = 0
	}
}

// SkipZeroes ensures first samples don't have leading zeros
func (r *Resampler) SkipZeroes() {
	for i := 0; i < r.nbChannels; i++ {
		r.lastSample[i] = r.filtLen / 2
	}
}

// ---- Internal implementation ----

var qualityMap = []QualityMapping{
	{8, 4, 0.830, 0.860, &FuncDef{kaiser6Table, 32}},   // Q0
	{16, 4, 0.850, 0.880, &FuncDef{kaiser6Table, 32}},  // Q1
	{32, 4, 0.882, 0.910, &FuncDef{kaiser6Table, 32}},  // Q2
	{48, 8, 0.895, 0.917, &FuncDef{kaiser8Table, 32}},  // Q3
	{64, 8, 0.921, 0.940, &FuncDef{kaiser8Table, 32}},  // Q4
	{80, 16, 0.922, 0.940, &FuncDef{kaiser10Table, 32}}, // Q5
	{96, 16, 0.940, 0.945, &FuncDef{kaiser10Table, 32}}, // Q6
	{128, 16, 0.950, 0.950, &FuncDef{kaiser10Table, 32}}, // Q7
	{160, 16, 0.960, 0.960, &FuncDef{kaiser10Table, 32}}, // Q8
	{192, 32, 0.968, 0.968, &FuncDef{kaiser12Table, 64}}, // Q9
	{256, 32, 0.975, 0.975, &FuncDef{kaiser12Table, 64}}, // Q10
}

var (
	kaiser12Table = []float64{
		0.99859849, 1.00000000, 0.99859849, 0.99440475, 0.98745105, 0.97779076,
		0.96549770, 0.95066529, 0.93340547, 0.91384741, 0.89213598, 0.86843014,
		0.84290116, 0.81573067, 0.78710866, 0.75723148, 0.72629970, 0.69451601,
		0.66208321, 0.62920216, 0.59606986, 0.56287762, 0.52980938, 0.49704014,
		0.46473455, 0.43304576, 0.40211431, 0.37206735, 0.34301800, 0.31506490,
		0.28829195, 0.26276832, 0.23854851, 0.21567274, 0.19416736, 0.17404546,
		0.15530766, 0.13794294, 0.12192957, 0.10723616, 0.09382272, 0.08164178,
		0.07063950, 0.06075685, 0.05193064, 0.04409466, 0.03718069, 0.03111947,
		0.02584161, 0.02127838, 0.01736250, 0.01402878, 0.01121463, 0.00886058,
		0.00691064, 0.00531256, 0.00401805, 0.00298291, 0.00216702, 0.00153438,
		0.00105297, 0.00069463, 0.00043489, 0.00025272, 0.00013031, 0.0000527734,
		0.00001000, 0.00000000,
	}

	kaiser10Table = []float64{
		0.99537781, 1.00000000, 0.99537781, 0.98162644, 0.95908712, 0.92831446,
		0.89005583, 0.84522401, 0.79486424, 0.74011713, 0.68217934, 0.62226347,
		0.56155915, 0.50119680, 0.44221549, 0.38553619, 0.33194107, 0.28205962,
		0.23636152, 0.19515633, 0.15859932, 0.12670280, 0.09935205, 0.07632451,
		0.05731132, 0.04193980, 0.02979584, 0.02044510, 0.01345224, 0.00839739,
		0.00488951, 0.00257636, 0.00115101, 0.00035515, 0.00000000, 0.00000000,
	}

	kaiser8Table = []float64{
		0.99635258, 1.00000000, 0.99635258, 0.98548012, 0.96759014, 0.94302200,
		0.91223751, 0.87580811, 0.83439927, 0.78875245, 0.73966538, 0.68797126,
		0.63451750, 0.58014482, 0.52566725, 0.47185369, 0.41941150, 0.36897272,
		0.32108304, 0.27619388, 0.23465776, 0.19672670, 0.16255380, 0.13219758,
		0.10562887, 0.08273982, 0.06335451, 0.04724088, 0.03412321, 0.02369490,
		0.01563093, 0.00959968, 0.00527363, 0.00233883, 0.00050000, 0.00000000,
	}

	kaiser6Table = []float64{
		0.99733006, 1.00000000, 0.99733006, 0.98935595, 0.97618418, 0.95799003,
		0.93501423, 0.90755855, 0.87598009, 0.84068475, 0.80211977, 0.76076565,
		0.71712752, 0.67172623, 0.62508937, 0.57774224, 0.53019925, 0.48295561,
		0.43647969, 0.39120616, 0.34752997, 0.30580127, 0.26632152, 0.22934058,
		0.19505503, 0.16360756, 0.13508755, 0.10953262, 0.08693120, 0.06722600,
		0.05031820, 0.03607231, 0.02432151, 0.01487334, 0.00752000, 0.00000000,
	}
)

func word2int(x float32) int16 {
	if x < -32768 {
		return -32768
	}
	if x > 32767 {
		return 32767
	}
	return int16(x)
}

func computeFunc(x float32, funcDef *FuncDef) float64 {
	y := x * float32(funcDef.oversample)
	ind := int(math.Floor(float64(y)))
	frac := y - float32(ind)

	interp3 := -0.1666666667*frac + 0.1666666667*(frac*frac*frac)
	interp2 := frac + 0.5*(frac*frac) - 0.5*(frac*frac*frac)
	interp0 := -0.3333333333*frac + 0.5*(frac*frac) - 0.1666666667*(frac*frac*frac)
	interp1 := 1.0 - interp3 - interp2 - interp0

	return interp0*funcDef.table[ind] + interp1*funcDef.table[ind+1] + interp2*funcDef.table[ind+2] + interp3*funcDef.table[ind+3]
}

func sinc(cutoff, x float32, N int, windowFunc *FuncDef) int16 {
	xx := x * cutoff
	if math.Abs(float64(x)) < 1e-6 {
		return word2int(