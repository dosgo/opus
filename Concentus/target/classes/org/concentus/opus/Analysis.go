
package opus

import (
	"math"
)

// Analysis contains constants and methods for audio analysis
type Analysis struct{}

const (
	mPi          = 3.141592653
	cA           = 0.43157974
	cB           = 0.67848403
	cC           = 0.08595542
	cE           = mPi / 2
	nbTonalSkipBands = 9
)

// fastAtan2f approximates the atan2 function with a faster computation
func (a *Analysis) fastAtan2f(y, x float32) float32 {
	// Avoid underflow for very small values
	if abs16(x)+abs16(y) < 1e-9 {
		x *= 1e12
		y *= 1e12
	}

	x2 := x * x
	y2 := y * y

	if x2 < y2 {
		den := (y2 + cB*x2) * (y2 + cC*x2)
		if den != 0 {
			return -x*y*(y2+cA*x2)/den + ternary(y < 0, -cE, cE).(float32)
		}
		return ternary(y < 0, -cE, cE).(float32)
	} else {
		den := (x2 + cB*y2) * (x2 + cC*y2)
		if den != 0 {
			return x*y*(x2+cA*y2)/den + ternary(y < 0, -cE, cE).(float32) - ternary(x*y < 0, -cE, cE).(float32)
		}
		return ternary(y < 0, -cE, cE).(float32) - ternary(x*y < 0, -cE, cE).(float32)
	}
}

// tonalityAnalysisInit initializes the tonality analysis state
func (a *Analysis) tonalityAnalysisInit(tonal *TonalityAnalysisState) {
	tonal.Reset()
}

// tonalityGetInfo extracts tonality information from the analysis state
func (a *Analysis) tonalityGetInfo(tonal *TonalityAnalysisState, infoOut *AnalysisInfo, length int) {
	pos := tonal.readPos
	currLookahead := tonal.writePos - tonal.readPos
	if currLookahead < 0 {
		currLookahead += detectSize
	}

	if length > 480 && pos != tonal.writePos {
		pos++
		if pos == detectSize {
			pos = 0
		}
	}
	if pos == tonal.writePos {
		pos--
	}
	if pos < 0 {
		pos = detectSize - 1
	}

	infoOut.Assign(tonal.info[pos])
	tonal.readSubframe += length / 120
	for tonal.readSubframe >= 4 {
		tonal.readSubframe -= 4
		tonal.readPos++
	}
	if tonal.readPos >= detectSize {
		tonal.readPos -= detectSize
	}

	// Compensate for delay in features
	currLookahead = maxInt(currLookahead-10, 0)

	psum := float32(0)
	for i := 0; i < detectSize-currLookahead; i++ {
		psum += tonal.pmusic[i]
	}
	for ; i < detectSize; i++ {
		psum += tonal.pspeech[i]
	}

	psum = psum*tonal.musicConfidence + (1-psum)*tonal.speechConfidence
	infoOut.musicProb = psum
}

// tonalityAnalysis performs the core tonality analysis
func (a *Analysis) tonalityAnalysis(tonal *TonalityAnalysisState, celtMode *CeltMode, x []int16, xPtr int, length, offset, c1, c2, C, lsbDepth int) {
	var (
		N                          = 480
		N2                         = 240
		pi4                        = float32(mPi * mPi * mPi * mPi)
		bandTonality               [nbTbands]float32
		logE                       [nbTbands]float32
		BFCC                       [8]float32
		features                   [25]float32
		frameProbs                 [2]float32
		bandwidthMask, maxE, slope float32
		bandwidth                  int
	)

	tonal.lastTransition++

	alpha := float32(1.0 / minInt(20, 1+tonal.count))
	alphaE := float32(1.0 / minInt(50, 1+tonal.count))
	alphaE2 := float32(1.0 / minInt(1000, 1+tonal.count))

	if tonal.count < 4 {
		tonal.musicProb = 0.5
	}

	kfft := celtMode.mdct.kfft[0]
	if tonal.count == 0 {
		tonal.memFill = 240
	}

	downmixInt(x, xPtr, tonal.inmem, tonal.memFill, minInt(length, analysisBufSize-tonal.memFill), offset, c1, c2, C)

	if tonal.memFill+length < analysisBufSize {
		tonal.memFill += length
		return
	}

	info := tonal.info[tonal.writePos]
	tonal.writePos++
	if tonal.writePos >= detectSize {
		tonal.writePos -= detectSize
	}

	input := make([]float32, 960)
	output := make([]float32, 960)
	tonality := make([]float32, 240)
	noisiness := make([]float32, 240)

	for i := 0; i < N2; i++ {
		w := analysisWindow[i]
		input[2*i] = w * float32(tonal.inmem[i])
		input[2*i+1] = w * float32(tonal.inmem[N2+i])
		input[2*(N-i-1)] = w * float32(tonal.inmem[N-i-1])
		input[2*(N-i-1)+1] = w * float32(tonal.inmem[N+N2-i-1])
	}

	copy(tonal.inmem[:240], tonal.inmem[analysisBufSize-240:analysisBufSize])

	remaining := length - (analysisBufSize - tonal.memFill)
	downmixInt(x, xPtr, tonal.inmem, 240, remaining, offset+analysisBufSize-tonal.memFill, c1, c2, C)
	tonal.memFill = 240 + remaining

	kissFFT(kfft, input, output)

	for i := 1; i < N2; i++ {
		X1r := output[2*i] + output[2*(N-i)]
		X1i := output[2*i+1] - output[2*(N-i)+1]
		X2r := output[2*i+1] + output[2*(N-i)+1]
		X2i := output[2*(N-i)] - output[2*i]

		angle := float32(0.5/mPi) * a.fastAtan2f(X1i, X1r)
		dAngle := angle - tonal.angle[i]
		d2Angle := dAngle - tonal.dAngle[i]

		angle2 := float32(0.5/mPi) * a.fastAtan2f(X2i, X2r)
		dAngle2 := angle2 - angle
		d2Angle2 := dAngle2 - dAngle

		mod1 := d2Angle - float32(math.Floor(float64(0.5+d2Angle)))
		noisiness[i] = abs16(mod1)
		mod1 *= mod1
		mod1 *= mod1

		mod2 := d2Angle2 - float32(math.Floor(float64(0.5+d2Angle2)))
		noisiness[i] += abs16(mod2)
		mod2 *= mod2
		mod2 *= mod2

		avgMod := 0.25 * (tonal.d2Angle[i] + 2.0*mod1 + mod2)
		tonality[i] = 1.0/(1.0+40.0*16.0*pi4*avgMod) - 0.015

		tonal.angle[i] = angle2
		tonal.dAngle[i] = dAngle2
		tonal.d2Angle[i] = mod2
	}

	var (
		frameTonality, maxFrameTonality float32
		frameNoisiness, frameStationarity float32
		relativeE, frameLoudness float32
	)

	info.activity = 0
	if tonal.count == 0 {
		for b := 0; b < nbTbands; b++ {
			tonal.lowE[b] = 1e10
			tonal.highE[b] = -1e10
		}
	}

	for b := 0; b < nbTbands; b++ {
		var E, tE, nE float32
		for i := tbands[b]; i < tbands[b+1]; i++ {
			binE := output[2*i]*output[2*i] + output[2*(N-i)]*output[2*(N-i)] +
				output[2*i+1]*output[2*i+1] + output[2*(N-i)+1]*output[2*(N-i)+1]
			binE *= 5.55e-17
			E += binE
			tE += binE * tonality[i]
			nE += binE * 2.0 * (0.5 - noisiness[i])
		}

		tonal.E[tonal.ECount][b] = E
		frameNoisiness += nE / (1e-15 + E)
		frameLoudness += float32(math.Sqrt(float64(E + 1e-10)))
		logE[b] = float32(math.Log(float64(E + 1e-10)))
		tonal.lowE[b] = minFloat32(logE[b], tonal.lowE[b]+0.01)
		tonal.highE[b] = maxFloat32(logE[b], tonal.highE[b]-0.1)
		if tonal.highE[b] < tonal.lowE[b]+1.0 {
			tonal.highE[b] += 0.5
			tonal.lowE[b] -= 0.5
		}
		relativeE += (logE[b] - tonal.lowE[b]) / (1e-15 + tonal.highE[b] - tonal.lowE[b])

		var L1, L2 float32
		for i := 0; i < nbFrames; i++ {
			L1 += float32(math.Sqrt(float64(tonal.E[i][b])))
			L2 += tonal.E[i][b]
		}

		stationarity := minFloat32(0.99, L1/float32(math.Sqrt(1e-15+float64(nbFrames*L2))))
		stationarity *= stationarity
		stationarity *= stationarity
		frameStationarity += stationarity

		bandTonality[b] = maxFloat32(tE/(1e-15+E), stationarity*tonal.prevBandTonality[b])
		frameTonality += bandTonality[b]
		if b >= nbTbands-nbTonalSkipBands {
			frameTonality -= bandTonality[b-nbTbands+nbTonalSkipBands]
		}
		maxFrameTonality = maxFloat32(maxFrameTonality, (1.0+0.03*float32(b-nbTbands))*frameTonality)
		slope += bandTonality[b] * float32(b-8)
		tonal.prevBandTonality[b] = bandTonality[b]
	}

	noiseFloor := 5.7e-4 / float32(1<<maxInt(0, lsbDepth-8))
	noiseFloor *= float32(1 << (15 + sigShift))
	noiseFloor *= noiseFloor

	for b := 0; b < nbTotBands; b++ {
		var E float32
		bandStart := extraBands[b]
		bandEnd := extraBands[b+1]
		for i := bandStart; i < bandEnd; i++ {
			binE := output[2*i]*output[2*i] + output[2*(N-i)]*output[2*(N-i)] +
				output[2*i+1]*output[2*i+1] + output[2*(N-i)+1]*output[2*(N-i)+1]
			E += binE
		}
		maxE = maxFloat32(maxE, E)
		tonal.meanE[b] = maxFloat32((1-alphaE2)*tonal.meanE[b], E)
		E = maxFloat32(E, tonal.meanE[b])
		bandwidthMask = maxFloat32(0.05*bandwidthMask, E)
		if E > 0.1*bandwidthMask && E*1e9 > maxE && E > noiseFloor*float32(bandEnd-bandStart) {
			bandwidth = b
		}
	}

	if tonal.count <= 2 {
		bandwidth = 20
	}

	frameLoudness = 20 * float32(math.Log10(float64(frameLoudness)))
	tonal.Etracker = maxFloat32(tonal.Etracker-0.03, frameLoudness)
	tonal.lowECount *= (1 - alphaE)
	if frameLoudness < tonal.Etracker-30 {
		tonal.lowECount += alphaE
	}

	for i := 0; i < 8; i++ {
		var sum float32
		for b := 0; b < 16; b++ {
			sum += dctTable[i*16+b] * logE[b]
		}
		BFCC[i] = sum
	}

	frameStationarity /= nbTbands
	relativeE /= nbTbands
	if tonal.count < 10 {
		relativeE = 0.5
	}
	frameNoisiness /= nbTbands
	info.activity = frameNoisiness + (1-frameNoisiness)*relativeE
	frameTonality = maxFrameTonality / float32(nbTbands-nbTonalSkipBands)
	frameTonality = maxFloat32(frameTonality, tonal.prevTonality*0.8)
	tonal.prevTonality = frameTonality

	slope /= 8 * 8
	info.tonalitySlope = slope

	tonal.ECount = (tonal.ECount + 1) % nbFrames
	tonal.count++
	info.tonality = frameTonality

	for i := 0; i < 4; i++ {
		features[i] = -0.12299*(BFCC[i]+tonal.mem[i+24]) + 0.49195*(tonal.mem[i]+tonal.mem[i+16]) + 0.69693*tonal.mem[i+8] - 1.4349*tonal.cmean[i]
	}

	for i := 0; i < 4; i++ {
		tonal.cmean[i] = (1-alpha)*tonal.cmean[i] + alpha*BFCC[i]
	}

	for i := 0; i < 4; i++ {
		features[4+i] = 0.63246*(BFCC[i]-tonal.mem[i+24]) + 0.31623*(tonal.mem[i]-tonal.mem[i+16])
	}

	for i := 0; i < 3; i++ {
		features[8+i] = 0.53452*(BFCC[i]+tonal.mem[i+24]) - 0.26726*(tonal.mem[i]+tonal.mem[i+16]) - 0.53452*tonal.mem[i+8]
	}

	if tonal.count > 5 {
		for i := 0; i < 9; i++ {
			tonal.std[i] = (1-alpha)*tonal.std[i] + alpha*features[i]*features[i]
		}
	}

	for i := 0; i < 8; i++ {
		tonal.mem[i+24] = tonal.mem[i+16]
		tonal.mem[i+16] = tonal.mem[i+8]
		tonal.mem[i+8] = tonal.mem[i]
		tonal.mem[i] = BFCC[i]
	}

	for i := 0; i < 9; i++ {
		features[11+i] = float32(math.Sqrt(float64(tonal.std[i])))
	}

	features[20] = info.tonality
	features[21] = info.activity
	features[22] = frameStationarity
	features[23] = info.tonalitySlope
	features[24] = tonal.lowECount

	if info.enabled {
		mlpProcess(net, features[:], frameProbs[:])
		frameProbs[0] = 0.5 * (frameProbs[0] + 1)
		frameProbs[0] = 0.01 + 1.21*frameProbs[0]*frameProbs[0] - 0.23*float32(math.Pow(float64(frameProbs[0]), 10))
		frameProbs[1] = 0.5*frameProbs[1] + 0.5
		frameProbs[0] = frameProbs[1]*frameProbs[0] + (1-frameProbs[1])*0.5

		{
			var (
				tau, beta float32
				p0, p1, s0, m0 float32
				psum, speech0, music0 float32
			)

			tau = 0.00005 * frameProbs[1]
			beta = 0.05

			if true { // This was commented out in