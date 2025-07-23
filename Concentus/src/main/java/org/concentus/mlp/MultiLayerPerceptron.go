// Package mlp implements a multi-layer perceptron processor.
// Original Java code copyright and authorship information preserved.
package mlp

import (
	"math"
	"opustest/opus"
)

// MAX_NEURONS defines the maximum number of neurons in any layer
const MAX_NEURONS = 100

// tansigApprox approximates the tanh function using a lookup table and interpolation.
// This is a direct translation from Java but uses Go naming conventions.
func tansigApprox(x float32) float32 {
	var y, dy float32
	sign := float32(1)

	// Tests are reversed to catch NaNs (same logic as Java version)
	if !(x < 8) {
		return 1
	}
	if !(x > -8) {
		return -1
	}

	if x < 0 {
		x = -x
		sign = -1
	}

	i := int(math.Floor(float64(0.5 + 25*x)))
	x -= 0.04 * float32(i)
	y = opus.TansigTable[i]
	dy = 1 - y*y
	y = y + x*dy*(1-y*x)
	return sign * y
}

// MLPState represents the state of a multi-layer perceptron
type MLPState struct {
	Topo    [3]int    // Network topology [input, hidden, output]
	Weights []float32 // Weight matrix
}

// Process runs the multi-layer perceptron on the given input and stores results in output.
// This is optimized for Go by:
// 1. Using slices instead of arrays with MAX_NEURONS
// 2. Using range loops where appropriate
// 3. Avoiding manual pointer arithmetic (W_ptr)
func (m *MLPState) Process(input, output []float32) {
	hidden := make([]float32, m.Topo[1])
	W := m.Weights
	wIdx := 0 // weight index replaces W_ptr

	// First layer (input to hidden)
	for j := 0; j < m.Topo[1]; j++ {
		sum := W[wIdx]
		wIdx++
		for k := 0; k < m.Topo[0]; k++ {
			sum += input[k] * W[wIdx]
			wIdx++
		}
		hidden[j] = tansigApprox(sum)
	}

	// Second layer (hidden to output)
	for j := 0; j < m.Topo[2]; j++ {
		sum := W[wIdx]
		wIdx++
		for k := 0; k < m.Topo[1]; k++ {
			sum += hidden[k] * W[wIdx]
			wIdx++
		}
		output[j] = tansigApprox(sum)
	}
}
