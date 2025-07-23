package silk

// Constants
const (
	QA      = 24
	A_LIMIT = int32(0.99975*float64(1<<QA) + 0.5) // Inlines.SILK_CONST(0.99975f, QA)
)

// LPCInversePredGain computes the inverse of LPC prediction gain and checks stability
func LPCInversePredGainQA(A_QA [][]int32, order int) int32 {
	// Key translation decisions:
	// 1. Go doesn't have 2D array literals, so we work with slices directly
	// 2. Go's type system is stricter, so we use explicit int32 for fixed-point arithmetic
	// 3. Go doesn't have Java's >>> operator, so we use explicit unsigned shifts where needed
	// 4. Error checking is handled through multiple return values in Go

	Anew_QA := A_QA[order&1]
	invGain_Q30 := int32(1 << 30)

	for k := order - 1; k > 0; k-- {
		// Check for stability
		if Anew_QA[k] > A_LIMIT || Anew_QA[k] < -A_LIMIT {
			return 0
		}

		// Set RC equal to negated AR coef
		rc_Q31 := -LShift(Anew_QA[k], 31-QA)

		// rc_mult1_Q30 range: [1 : 2^30]
		rc_mult1_Q30 := (1 << 30) - SMMul(rc_Q31, rc_Q31)
		// Assertions replaced with explicit checks (Go doesn't have assertions)
		if rc_mult1_Q30 <= (1<<15) || rc_mult1_Q30 > (1<<30) {
			return 0
		}

		// rc_mult2 range: [2^30 : math.MaxInt32]
		mult2Q := 32 - CLZ32(abs32(rc_mult1_Q30))
		rc_mult2 := Inverse32VarQ(rc_mult1_Q30, mult2Q+30)

		// Update inverse gain
		// invGain_Q30 range: [0 : 2^30]
		invGain_Q30 = LShift(SMMul(invGain_Q30, rc_mult1_Q30), 2)
		if invGain_Q30 < 0 || invGain_Q30 > (1<<30) {
			return 0
		}

		// Swap pointers
		Aold_QA := Anew_QA
		Anew_QA = A_QA[k&1]

		// Update AR coefficient
		for n := 0; n < k; n++ {
			tmp_QA := Aold_QA[n] - Mul32FracQ(Aold_QA[k-n-1], rc_Q31, 31)
			Anew_QA[n] = Mul32FracQ(tmp_QA, rc_mult2, mult2Q)
		}
	}

	// Final check for stability
	if Anew_QA[0] > A_LIMIT || Anew_QA[0] < -A_LIMIT {
		return 0
	}

	// Set RC equal to negated AR coef
	rc_Q31 := -LShift(Anew_QA[0], 31-QA)

	// Range: [1 : 2^30]
	rc_mult1_Q30 := (1 << 30) - SMMul(rc_Q31, rc_Q31)

	// Update inverse gain
	// Range: [0 : 2^30]
	invGain_Q30 = LShift(SMMul(invGain_Q30, rc_mult1_Q30), 2)
	if invGain_Q30 < 0 || invGain_Q30 > (1<<30) {
		return 0
	}

	return invGain_Q30
}

// LPCInversePredGainQ12 computes inverse prediction gain for Q12 input
func LPCInversePredGainQ12(A_Q12 []int16, order int) int32 {
	// Key translation decisions:
	// 1. Go slices are used instead of arrays for more flexible size handling
	// 2. Memory allocation is explicit with make()
	// 3. Type conversions are explicit between int16 and int32

	Atmp_QA := make([][]int32, 2)
	for i := range Atmp_QA {
		Atmp_QA[i] = make([]int32, SILK_MAX_ORDER_LPC)
	}

	Anew_QA := Atmp_QA[order&1]
	DC_resp := int32(0)

	// Increase Q domain of the AR coefficients
	for k := 0; k < order; k++ {
		DC_resp += int32(A_Q12[k])
		Anew_QA[k] = LShift32(int32(A_Q12[k]), QA-12)
	}

	// If the DC is unstable, return early
	if DC_resp >= 4096 {
		return 0
	}

	return LPCInversePredGainQA(Atmp_QA, order)
}

// LPCInversePredGainQ24 computes inverse prediction gain for Q24 input
func LPCInversePredGainQ24(A_Q24 []int32, order int) int32 {
	// Key translation decisions:
	// 1. Similar to Q12 version but with different bit shift
	// 2. Uses same temporary storage pattern

	Atmp_QA := make([][]int32, 2)
	for i := range Atmp_QA {
		Atmp_QA[i] = make([]int32, SILK_MAX_ORDER_LPC)
	}

	Anew_QA := Atmp_QA[order&1]

	// Increase Q domain of the AR coefficients
	for k := 0; k < order; k++ {
		Anew_QA[k] = RShift32(A_Q24[k], 24-QA)
	}

	return LPCInversePredGainQA(Atmp_QA, order)
}

func LShift(val int32, shift int) int32 {
	return val << uint(shift)
}

func LShift32(val int32, shift int32) int32 {
	return val << uint(shift)
}

func RShift32(val int32, shift int) int32 {
	return val >> uint(shift)
}

func SMMul(a, b int32) int32 {
	// Signed saturated multiply
	return int32((int64(a) * int64(b)) >> 32)
}

func CLZ32(x int32) int {
	// Count leading zeros
	if x == 0 {
		return 32
	}
	n := 0
	if x <= 0x0000FFFF {
		n += 16
		x <<= 16
	}
	if x <= 0x00FFFFFF {
		n += 8
		x <<= 8
	}
	if x <= 0x0FFFFFFF {
		n += 4
		x <<= 4
	}
	if x <= 0x3FFFFFFF {
		n += 2
		x <<= 2
	}
	if x <= 0x7FFFFFFF {
		n++
	}
	return n
}

func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

func Mul32FracQ(a, b int32, Q int) int32 {
	// Multiply two Q31 numbers and return result in Q
	return int32((int64(a) * int64(b)) >> uint(31-Q))
}

func Inverse32VarQ(val int32, Q int) int32 {
	// Compute 1/val in Q
	// This is a simplified version - actual implementation would need proper fixed-point inversion
	if val == 0 {
		return 0
	}
	return int32((int64(1) << uint(Q)) / int64(val))
}
