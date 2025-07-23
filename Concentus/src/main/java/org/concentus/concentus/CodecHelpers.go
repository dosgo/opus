
package concentus

import (
	"math"
)

// CodecHelpers provides utility functions for Opus codec operations
type CodecHelpers struct{}

// GenTOC generates a Table of Contents (TOC) byte for an Opus packet
func (ch *CodecHelpers) GenTOC(mode OpusMode, framerate int, bandwidth OpusBandwidth, channels int) byte {
	var period int
	var toc uint16

	// Calculate the period by doubling the framerate until it's >= 400
	for period = 0; framerate < 400; period++ {
		framerate <<= 1
	}

	switch mode {
	case MODE_SILK_ONLY:
		toc = uint16((bandwidth.Ordinal() - OPUS_BANDWIDTH_NARROWBAND.Ordinal()) << 5)
		toc |= uint16((period - 2) << 3)
	case MODE_CELT_ONLY:
		tmp := bandwidth.Ordinal() - OPUS_BANDWIDTH_MEDIUMBAND.Ordinal()
		if tmp < 0 {
			tmp = 0
		}
		toc = 0x80
		toc |= uint16(tmp << 5)
		toc |= uint16(period << 3)
	default: // Hybrid mode
		toc = 0x60
		toc |= uint16((bandwidth.Ordinal()-OPUS_BANDWIDTH_SUPERWIDEBAND.Ordinal()) << 4)
		toc |= uint16((period - 2) << 3)
	}

	// Set stereo flag
	if channels == 2 {
		toc |= 1 << 2
	}

	return byte(toc & 0xFF)
}

// HPCutoff applies a high-pass filter with the given cutoff frequency
func (ch *CodecHelpers) HPCutoff(input []int16, inputPtr int, cutoffHz int, output []int16, outputPtr int, 
    hpMem []int32, length int, channels int, fs int) {
    
    var B_Q28 [3]int32
    var A_Q28 [2]int32
    var Fc_Q19, r_Q28, r_Q22 int32

    // Calculate filter coefficients
    Fc_Q19 = silk_DIV32_16(silk_SMULBB(SILK_CONST(1.5*3.14159/1000, 19), int32(cutoffHz)), int32(fs/1000))
    
    r_Q28 = SILK_CONST(1.0, 28) - silk_MUL(SILK_CONST(0.92, 9), Fc_Q19)

    // Set filter coefficients
    B_Q28[0] = r_Q28
    B_Q28[1] = int32(int64(-r_Q28) << 1
    B_Q28[2] = r_Q28

    r_Q22 = r_Q28 >> 6
    A_Q28[0] = silk_SMULWW(r_Q22, silk_SMULWW(Fc_Q19, Fc_Q19)-SILK_CONST(2.0, 22))
    A_Q28[1] = silk_SMULWW(r_Q22, r_Q22)

    // Apply filter
    silk_biquad_alt(input, inputPtr, B_Q28[:], A_Q28[:], hpMem, 0, output, outputPtr, length, channels)
    if channels == 2 {
        silk_biquad_alt(input, inputPtr+1, B_Q28[:], A_Q28[:], hpMem, 2, output, outputPtr+1, length, channels)
    }
}

// DCReject applies a DC rejection filter
func (ch *CodecHelpers) DCReject(input []int16, inputPtr int, cutoffHz int, output []int16, 
    outputPtr int, hpMem []int32, length int, channels int, fs int) {
    
    // Calculate shift factor
    shift := celt_ilog2(fs / (cutoffHz * 3))

    for c := 0; c < channels; c++ {
        for i := 0; i < length; i++ {
            x := EXTEND32(input[channels*i+c+inputPtr]) << 15
            // First stage
            tmp := x - hpMem[2*c]
            hpMem[2*c] += PSHR32(x-hpMem[2*c], shift)
            // Second stage
            y := tmp - hpMem[2*c+1]
            hpMem[2*c+1] += PSHR32(tmp-hpMem[2*c+1], shift)
            output[channels*i+c+outputPtr] = EXTRACT16(SATURATE(PSHR32(y, 15), 32767))
        }
    }
}

// StereoFade applies a fade between stereo channels
func (ch *CodecHelpers) StereoFade(pcmBuf []int16, g1 int, g2 int, overlap48 int, 
    frameSize int, channels int, window []int16, fs int) {
    
    inc := 48000 / fs
    overlap := overlap48 / inc
    g1 = Q15ONE - g1
    g2 = Q15ONE - g2

    for i := 0; i < overlap; i++ {
        w := MULT16_16_Q15(window[i*inc], window[i*inc])
        g := SHR32(MAC16_16(MULT16_16(w, g2), Q15ONE-w, g1), 15)
        diff := EXTRACT16(HALF32(int32(pcmBuf[i*channels]) - int32(pcmBuf[i*channels+1])))
        diff = MULT16_16_Q15(g, diff)
        pcmBuf[i*channels] -= int16(diff)
        pcmBuf[i*channels+1] += int16(diff)
    }

    for i := overlap; i < frameSize; i++ {
        diff := EXTRACT16(HALF32(int32(pcmBuf[i*channels]) - int32(pcmBuf[i*channels+1])))
        diff = MULT16_16_Q15(g2, diff)
        pcmBuf[i*channels] -= int16(diff)
        pcmBuf[i*channels+1] += int16(diff)
    }
}

// GainFade applies a gain fade to the audio buffer
func (ch *CodecHelpers) GainFade(buffer []int16, bufPtr int, g1 int, g2 int, 
    overlap48 int, frameSize int, channels int, window []int16, fs int) {
    
    inc := 48000 / fs
    overlap := overlap48 / inc

    if channels == 1 {
        for i := 0; i < overlap; i++ {
            w := MULT16_16_Q15(window[i*inc], window[i*inc])
            g := SHR32(MAC16_16(MULT16_16(w, g2), Q15ONE-w, g1), 15)
            buffer[bufPtr+i] = int16(MULT16_16_Q15(g, buffer[bufPtr+i]))
        }
    } else {
        for i := 0; i < overlap; i++ {
            w := MULT16_16_Q15(window[i*inc], window[i*inc])
            g := SHR32(MAC16_16(MULT16_16(w, g2), Q15ONE-w, g1), 15)
            buffer[bufPtr+i*2] = int16(MULT16_16_Q15(g, buffer[bufPtr+i*2]))
            buffer[bufPtr+i*2+1] = int16(MULT16_16_Q15(g, buffer[bufPtr+i*2+1]))
        }
    }

    for c := 0; c < channels; c++ {
        for i := overlap; i < frameSize; i++ {
            buffer[bufPtr+i*channels+c] = int16(MULT16_16_Q15(g2, buffer[bufPtr+i*channels+c]))
        }
    }
}

// TransientBoost estimates bitrate boost based on sub-frame energy
func (ch *CodecHelpers) TransientBoost(E []float32, E_ptr int, E_1 []float32, LM int, maxM int) float32 {
    M := IMIN(maxM, (1<<LM)+1)
    sumE, sumE_1 := float32(0.0), float32(0.0)

    for i := E_ptr; i < M+E_ptr; i++ {
        sumE += E[i]
        sumE_1 += E_1[i]
    }

    metric := sumE * sumE_1 / float32(M*M)
    return MIN16(1, float32(math.Sqrt(float64(MAX16(0, 0.05*(metric-2)))))
}

// TransientViterbi performs Viterbi decoding to find best frame size combination
func (ch *CodecHelpers) TransientViterbi(E []float32, E_1 []float32, N int, frameCost int, rate int) int {
    const MAX_DYNAMIC_FRAMESIZE = 24
    var cost [MAX_DYNAMIC_FRAMESIZE][16]float32
    var states [MAX_DYNAMIC_FRAMESIZE][16]int
    var factor float32

    // Adjust factor based on bitrate
    if rate < 80 {
        factor = 0
    } else if rate > 160 {
        factor = 1
    } else {
        factor = float32(rate-80) / 80.0
    }

    // Initialize first row
    for i := 0; i < 16; i++ {
        states[0][i] = -1
        cost[0][i] = 1e10
    }

    // Initialize first 4 states
    for i := 0; i < 4; i++ {
        cost[0][1<<i] = float32(frameCost+rate*(1<<i)) * (1 + factor*ch.TransientBoost(E, 0, E_1, i, N+1))
        states[0][1<<i] = i
    }

    // Fill the DP table
    for i := 1; i < N; i++ {
        // Follow continuations
        for j := 2; j < 16; j++ {
            cost[i][j] = cost[i-1][j-1]
            states[i][j] = j - 1
        }

        // New frames
        for j := 0; j < 4; j++ {
            minCost := cost[i-1][1]
            states[i][1<<j] = 1

            for k := 1; k < 4; k++ {
                tmp := cost[i-1][(1<<(k+1))-1]
                if tmp < minCost {
                    states[i][1<<j] = (1 << (k + 1)) - 1
                    minCost = tmp
                }
            }

            currCost := float32(frameCost+rate*(1<<j)) * (1 + factor*ch.TransientBoost(E, i, E_1, j, N-i+1))
            cost[i][1<<j] = minCost

            if N-i < (1 << j) {
                cost[i][1<<j] += currCost * float32(N-i) / float32(1<<j)
            } else {
                cost[i][1<<j] += currCost
            }
        }
    }

    // Find best end state
    bestState := 1
    bestCost := cost[N-1][1]
    for i := 2; i < 16; i++ {
        if cost[N-1][i] < bestCost {
            bestCost = cost[N-1][i]
            bestState = i
        }
    }

    // Backtrack to find path
    for i := N - 1; i >= 0; i-- {
        bestState = states[i][bestState]
    }

    return bestState
}

// OptimizeFramesize determines the optimal frame size for encoding
func (ch *CodecHelpers) OptimizeFramesize(x []int16, xPtr int, length int, C int, fs int, 
    bitrate int, tonality int, mem []float32, buffering int) int {
    
    const MAX_DYNAMIC_FRAMESIZE = 24
    var e [MAX_DYNAMIC_FRAMESIZE + 4]float32
    var e_1 [MAX_DYNAMIC_FRAMESIZE + 3]float32
    var subframe, pos, offset int
    var sub []int16

    subframe = fs / 400
    sub = make([]int16, subframe)
    e[0] = mem[0]
    e_1[0] = 1.0 / (EPSILON + mem[0])

    if buffering != 0 {
        offset = 2*subframe - buffering
        length -= offset
        e[1] = mem[1]
        e_1[1] = 1.0 / (EPSILON + mem[1])
        e[2] = mem[2]
        e_1[2] = 1.0 / (EPSILON + mem[2])
        pos = 3
    } else {
        pos = 1
        offset = 0
    }

    N := IMIN(length/subframe, MAX_DYNAMIC_FRAMESIZE)
    var memx int16

    for i := 0; i < N; i++ {
        tmp := EPSILON
        var tmpx int16

        downmix_int(x, xPtr, sub, 0, subframe, i*subframe+offset, 0, -2, C)
        if i == 0 {
            memx = sub[0]
        }

        for j := 0; j < subframe; j++ {
            tmpx = sub[j]
            tmp += float32(tmpx-memx) * float32(tmpx-memx)
            memx = tmpx
        }

        e[i+pos] = tmp
        e_1[i+pos] = 1.0 / tmp
    }

    e[N+pos] = e[N+pos-1]
    if buffering != 0 {
        N = IMIN(MAX_DYNAMIC_FRAMESIZE, N+2)
    }

    bestLM := ch.TransientViterbi(e[:], e_1[:], N, int((1.0+0.5*float32(tonality))*(60*C+40)), bitrate/400)
    mem[0] = e[1<<bestLM]
    if buffering != 0 {
        mem[1] = e[(1<<bestLM)+1]
        mem[2] = e[(1<<bestLM)+2]
    }

    return bestLM
}

// FrameSizeSelect determines the appropriate frame size
func (ch *CodecHelpers) FrameSizeSelect(frameSize int, variableDuration OpusFramesize, fs int) int {
    var newSize int

    if frameSize < fs/400 {
        return -1
    }

    switch variableDuration {
    case OPUS_FRAMESIZE_ARG:
        newSize = frameSize
    case OPUS_FRAMESIZE_VARIABLE:
        newSize = fs / 50
    default:
        if variableDuration.Ordinal() >= OPUS_FRAMESIZE_2_5_MS.Ordinal() && 
           variableDuration.Ordinal() <= OPUS_FRAMESIZE_60_MS.Ordinal() {
            newSize = IMIN(3*fs/50, (fs/400)<<(variableDuration.Ordinal()-OPUS_FRAMESIZE_2_5_MS.Ordinal()))
        } else {
            return -1
        }
    }

    if newSize > frameSize {
        return -1
    }

    // Validate frame size
    if 400*newSize != fs && 200*newSize != fs && 100*newSize != fs &&
        50*newSize != fs && 25*newSize != fs && 50*newSize != 3*fs {
        return -1
    }

    return newSize
}

// ComputeFrameSize calculates the optimal frame size for encoding
func (ch *CodecHelpers) ComputeFrameSize(analysisPcm []int16, analysisPcmPtr int, frameSize int,
    variableDuration OpusFramesize, C int, fs int, bitrateBps int,
    delayCompensation int, subframeMem []float32, analysisEnabled bool) int {

    if analysisEnabled && variableDuration == OPUS_FRAMESIZE_VARIABLE && frameSize >= fs/200 {
        LM := 3
        LM = ch.OptimizeFramesize(analysisPcm, analysisPcmPtr, frameSize, C, fs, bitrateBps,
            0, subframeMem, delayCompensation)
        for (fs/400)<<LM > frameSize {
            LM--
        }
        frameSize = (fs / 400) << LM
    } else {
        frameSize = ch.FrameSizeSelect(frameSize, variableDuration, fs)
    }

    if frameSize < 0 {
        return -1
    }
    return frameSize
}

// ComputeStereoWidth calculates stereo width for the audio frame
func (ch *CodecHelpers) ComputeStereoWidth(pcm []int16, pcmPtr int, frameSize int, fs int, mem *StereoWidthState) int {
    var xx, xy, yy int32
    frameRate := fs / frameSize
    shortAlpha := Q15ONE - (25*Q15ONE)/IMAX(50, frameRate)

    for i := 0; i < frameSize-3; i += 4 {
        pxx := int32(0)
        pxy := int32(0)
        pyy := int32(0)
        p2i := pcmPtr + (2 * i)

        x := int32(pcm[p2i])
        y := int32(pcm[p2i+1])
        pxx = SHR32(MULT16_16(x, x), 2)
        pxy = SHR32(MULT16_16(x, y), 2)
        pyy = SHR