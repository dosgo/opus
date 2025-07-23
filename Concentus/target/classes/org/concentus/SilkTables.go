
package silk

// SilkTables contains various lookup tables used in the SILK codec.
type SilkTables struct{}

// NewSilkTables creates a new instance of SilkTables.
func NewSilkTables() *SilkTables {
	return &SilkTables{}
}

// LSFCosTab_Q12 contains cosine approximation table for LSF conversion (Q12 values)
var LSFCosTab_Q12 = []int16{
	8192, 8190, 8182, 8170,
	8152, 8130, 8104, 8072,
	8034, 7994, 7946, 7896,
	7840, 7778, 7714, 7644,
	7568, 7490, 7406, 7318,
	7226, 7128, 7026, 6922,
	6812, 6698, 6580, 6458,
	6332, 6204, 6070, 5934,
	5792, 5648, 5502, 5352,
	5198, 5040, 4880, 4718,
	4552, 4382, 4212, 4038,
	3862, 3684, 3502, 3320,
	3136, 2948, 2760, 2570,
	2378, 2186, 1990, 1794,
	1598, 1400, 1202, 1002,
	802, 602, 402, 202,
	0, -202, -402, -602,
	-802, -1002, -1202, -1400,
	-1598, -1794, -1990, -2186,
	-2378, -2570, -2760, -2948,
	-3136, -3320, -3502, -3684,
	-3862, -4038, -4212, -4382,
	-4552, -4718, -4880, -5040,
	-5198, -5352, -5502, -5648,
	-5792, -5934, -6070, -6204,
	-6332, -6458, -6580, -6698,
	-6812, -6922, -7026, -7128,
	-7226, -7318, -7406, -7490,
	-7568, -7644, -7714, -7778,
	-7840, -7896, -7946, -7994,
	-8034, -8072, -8104, -8130,
	-8152, -8170, -8182, -8190,
	-8192,
}

// GainICDF contains gain iCDF tables
var GainICDF = [][]int16{
	{224, 112, 44, 15, 3, 2, 1, 0},
	{254, 237, 192, 132, 70, 23, 4, 0},
	{255, 252, 226, 155, 61, 11, 2, 0},
}

// DeltaGainICDF contains delta gain iCDF table
var DeltaGainICDF = []int16{
	250, 245, 234, 203, 71, 50, 42, 38,
	35, 33, 31, 29, 28, 27, 26, 25,
	24, 23, 22, 21, 20, 19, 18, 17,
	16, 15, 14, 13, 12, 11, 10, 9,
	8, 7, 6, 5, 4, 3, 2, 1,
	0,
}

// LTPPerIndexICDF contains LTP per index iCDF table
var LTPPerIndexICDF = []int16{179, 99, 0}

// LTPGainICDF0 contains LTP gain iCDF table 0
var LTPGainICDF0 = []int16{
	71, 56, 43, 30, 21, 12, 6, 0,
}

// LTPGainICDF1 contains LTP gain iCDF table 1
var LTPGainICDF1 = []int16{
	199, 165, 144, 124, 109, 96, 84, 71,
	61, 51, 42, 32, 23, 15, 8, 0,
}

// LTPGainICDF2 contains LTP gain iCDF table 2
var LTPGainICDF2 = []int16{
	241, 225, 211, 199, 187, 175, 164, 153,
	142, 132, 123, 114, 105, 96, 88, 80,
	72, 64, 57, 50, 44, 38, 33, 29,
	24, 20, 16, 12, 9, 5, 2, 0,
}

// LTPGainMiddleAvgRDQ14 contains LTP gain middle average RD Q14 value
const LTPGainMiddleAvgRDQ14 = 12304

// LTPGainBitsQ50 contains LTP gain BITS Q5 table 0
var LTPGainBitsQ50 = []int16{
	15, 131, 138, 138, 155, 155, 173, 173,
}

// LTPGainBitsQ51 contains LTP gain BITS Q5 table 1
var LTPGainBitsQ51 = []int16{
	69, 93, 115, 118, 131, 138, 141, 138,
	150, 150, 155, 150, 155, 160, 166, 160,
}

// LTPGainBitsQ52 contains LTP gain BITS Q5 table 2
var LTPGainBitsQ52 = []int16{
	131, 128, 134, 141, 141, 141, 145, 145,
	145, 150, 155, 155, 155, 155, 160, 160,
	160, 160, 166, 166, 173, 173, 182, 192,
	182, 192, 192, 192, 205, 192, 205, 224,
}

// LTPGainICDFPtrs contains pointers to LTP gain iCDF tables
var LTPGainICDFPtrs = [][]int16{
	LTPGainICDF0,
	LTPGainICDF1,
	LTPGainICDF2,
}

// LTPGainBitsQ5Ptrs contains pointers to LTP gain BITS Q5 tables
var LTPGainBitsQ5Ptrs = [][]int16{
	LTPGainBitsQ50,
	LTPGainBitsQ51,
	LTPGainBitsQ52,
}

// LTPGainVQ0 contains LTP gain VQ table 0
var LTPGainVQ0 = [][]int8{
	{4, 6, 24, 7, 5},
	{0, 0, 2, 0, 0},
	{12, 28, 41, 13, -4},
	{-9, 15, 42, 25, 14},
	{1, -2, 62, 41, -9},
	{-10, 37, 65, -4, 3},
	{-6, 4, 66, 7, -8},
	{16, 14, 38, -3, 33},
}

// LTPGainVQ1 contains LTP gain VQ table 1
var LTPGainVQ1 = [][]int8{
	{13, 22, 39, 23, 12},
	{-1, 36, 64, 27, -6},
	{-7, 10, 55, 43, 17},
	{1, 1, 8, 1, 1},
	{6, -11, 74, 53, -9},
	{-12, 55, 76, -12, 8},
	{-3, 3, 93, 27, -4},
	{26, 39, 59, 3, -8},
	{2, 0, 77, 11, 9},
	{-8, 22, 44, -6, 7},
	{40, 9, 26, 3, 9},
	{-7, 20, 101, -7, 4},
	{3, -8, 42, 26, 0},
	{-15, 33, 68, 2, 23},
	{-2, 55, 46, -2, 15},
	{3, -1, 21, 16, 41},
}

// LTPGainVQ2 contains LTP gain VQ table 2
var LTPGainVQ2 = [][]int8{
	{-6, 27, 61, 39, 5},
	{-11, 42, 88, 4, 1},
	{-2, 60, 65, 6, -4},
	{-1, -5, 73, 56, 1},
	{-9, 19, 94, 29, -9},
	{0, 12, 99, 6, 4},
	{8, -19, 102, 46, -13},
	{3, 2, 13, 3, 2},
	{9, -21, 84, 72, -18},
	{-11, 46, 104, -22, 8},
	{18, 38, 48, 23, 0},
	{-16, 70, 83, -21, 11},
	{5, -11, 117, 22, -8},
	{-6, 23, 117, -12, 3},
	{3, -8, 95, 28, 4},
	{-10, 15, 77, 60, -15},
	{-1, 4, 124, 2, -4},
	{3, 38, 84, 24, -25},
	{2, 13, 42, 13, 31},
	{21, -4, 56, 46, -1},
	{-1, 35, 79, -13, 19},
	{-7, 65, 88, -9, -14},
	{20, 4, 81, 49, -29},
	{20, 0, 75, 3, -17},
	{5, -9, 44, 92, -8},
	{1, -3, 22, 69, 31},
	{-6, 95, 41, -12, 5},
	{39, 67, 16, -4, 1},
	{0, -6, 120, 55, -36},
	{-13, 44, 122, 4, -24},
	{81, 5, 11, 3, 7},
	{2, 0, 9, 10, 88},
}

// LTPVQPtrsQ7 contains pointers to LTP VQ tables (Q7)
var LTPVQPtrsQ7 = [][][]int8{
	LTPGainVQ0,
	LTPGainVQ1,
	LTPGainVQ2,
}

// LTPGainVQ0Gain contains LTP gain VQ 0 gain table
var LTPGainVQ0Gain = []int16{
	46, 2, 90, 87, 93, 91, 82, 98,
}

// LTPGainVQ1Gain contains LTP gain VQ 1 gain table
var LTPGainVQ1Gain = []int16{
	109, 120, 118, 12, 113, 115, 117, 119,
	99, 59, 87, 111, 63, 111, 112, 80,
}

// LTPGainVQ2Gain contains LTP gain VQ 2 gain table
var LTPGainVQ2Gain = []int16{
	126, 124, 125, 124, 129, 121, 126, 23,
	132, 127, 127, 127, 126, 127, 122, 133,
	130, 134, 101, 118, 119, 145, 126, 86,
	124, 120, 123, 119, 170, 173, 107, 109,
}

// LTPVQGainPtrsQ7 contains pointers to LTP VQ gain tables (Q7)
var LTPVQGainPtrsQ7 = [][]int16{
	LTPGainVQ0Gain,
	LTPGainVQ1Gain,
	LTPGainVQ2Gain,
}

// LTPVQSizes contains LTP VQ sizes
var LTPVQSizes = []int8{8, 16, 32}

// NLSFCB1NBMBQ8 contains NLSF CB1 NB MB Q8 table
var NLSFCB1NBMBQ8 = []int16{
	12, 35, 60, 83, 108, 132, 157, 180,
	206, 228, 15, 32, 55, 77, 101, 125,
	151, 175, 201, 225, 19, 42, 66, 89,
	114, 137, 162, 184, 209, 230, 12, 25,
	50, 72, 97, 120, 147, 172, 200, 223,
	26, 44, 69, 90, 114, 135, 159, 180,
	205, 225, 13, 22, 53, 80, 106, 130,
	156, 180, 205, 228, 15, 25, 44, 64,
	90, 115, 142, 168, 196, 222, 19, 24,
	62, 82, 100, 120, 145, 168, 190, 214,
	22, 31, 50, 79, 103, 120, 151, 170,
	203, 227, 21, 29, 45, 65, 106, 124,
	150, 171, 196, 224, 30, 49, 75, 97,
	121, 142, 165, 186, 209, 229, 19, 25,
	52, 70, 93, 116, 143, 166, 192, 219,
	26, 34, 62, 75, 97, 118, 145, 167,
	194, 217, 25, 33, 56, 70, 91, 113,
	143, 165, 196, 223, 21, 34, 51, 72,
	97, 117, 145, 171, 196, 222, 20, 29,
	50, 67, 90, 117, 144, 168, 197, 221,
	22, 31, 48, 66, 95, 117, 146, 168,
	196, 222, 24, 33, 51, 77, 116, 134,
	158, 180, 200, 224, 21, 28, 70, 87,
	106, 124, 149, 170, 194, 217, 26, 33,
	53, 64, 83, 117, 152, 173, 204, 225,
	27, 34, 65, 95, 108, 129, 155, 174,
	210, 225, 20, 26, 72, 99, 113, 131,
	154, 176, 200, 219, 34, 43, 61, 78,
	93, 114, 155, 177, 205, 229, 23, 29,
	54, 97, 124, 138, 163, 179, 209, 229,
	30, 38, 56, 89, 118, 129, 158, 178,
	200, 231, 21, 29, 49, 63, 85, 111,
	142, 163, 193, 222, 27, 48, 77, 103,
	133, 158, 179, 196, 215, 232, 29, 47,
	74, 99, 124, 151, 176, 198, 220, 237,
	33, 42, 61, 76, 93, 121, 155, 174,
	207, 225, 29, 53, 87, 112, 136, 154,
	170, 188, 208, 227, 24, 30, 52, 84,
	131, 150, 166, 186, 203, 229, 37, 48,
	64, 84, 104, 118, 156, 177, 201, 230,
}

// NLSFCB1ICDFNBMB contains NLSF CB1 iCDF NB MB table
var NLSFCB1ICDFNBMB = []int16{
	212, 178, 148, 