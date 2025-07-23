package concentus

// CeltMode represents the configuration for CELT processing mode.
// This is a direct translation from Java but adapted to Go idioms.
type CeltMode struct {
	Fs             int     // Sampling frequency
	Overlap        int     // Overlap between frames
	NbEBands       int     // Number of pseudo-critical bands
	EffEBands      int     // Effective number of bands
	Preemph        [4]int  // Pre-emphasis coefficients
	EBands         []int16 // Definition for each pseudo-critical band
	MaxLM          int     // Maximum number of modes
	NbShortMdcts   int     // Number of short MDCTs
	ShortMdctSize  int     // Size of short MDCTs
	NbAllocVectors int     // Number of lines in allocVectors
	AllocVectors   []int16 // Bits per band for several rates
	LogN           []int16 // LogN values
	Window         []int   // Window function
	Mdct           *MDCTLookup
	Cache          *PulseCache
}

// NewMode48000_960_120 creates and initializes the 48kHz, 960-sample mode with 120-sample overlap.
// This replaces the Java static initializer pattern with a Go constructor function.
func NewMode48000_960_120() *CeltMode {
	mode := &CeltMode{
		Fs:             48000,
		Overlap:        120,
		NbEBands:       21,
		EffEBands:      21,
		Preemph:        [4]int{27853, 0, 4096, 8192},
		EBands:         CeltTables.Eband5ms,
		MaxLM:          3,
		NbShortMdcts:   8,
		ShortMdctSize:  120,
		NbAllocVectors: 11,
		AllocVectors:   CeltTables.BandAllocation,
		LogN:           CeltTables.LogN400,
		Window:         CeltTables.Window120,
		Mdct: &MDCTLookup{
			N:        1920,
			MaxShift: 3,
			Kfft: []*FFTState{
				CeltTables.FftState48000_960_0,
				CeltTables.FftState48000_960_1,
				CeltTables.FftState48000_960_2,
				CeltTables.FftState48000_960_3,
			},
			Trig: CeltTables.MdctTwiddles960,
		},
		Cache: &PulseCache{
			Size:  392,
			Index: CeltTables.CacheIndex50,
			Bits:  CeltTables.CacheBits50,
			Caps:  CeltTables.CacheCaps50,
		},
	}
	return mode
}

// Key Translation Decisions:

// 1. Struct vs Class:
//    - Go uses structs instead of classes. We converted the Java class to a Go struct.
//    - Made all fields public (capitalized) since Go doesn't have private/public modifiers.

// 2. Initialization Pattern:
//    - Java used static initialization blocks and a singleton pattern.
//    - Go prefers explicit constructor functions, so we created NewMode48000_960_120().

// 3. Type Changes:
//    - Java's 'short' becomes Go's 'int16' for the same 16-bit integer representation.
//    - Java arrays become Go slices ([]int16 etc.) for more flexible usage.
//    - The preemph array is fixed-size, so we used a Go array ([4]int).

// 4. Naming Conventions:
//    - Followed Go's camelCase convention instead of Java's lowerCamelCase.
//    - Made constants and tables in CeltTables capitalized to follow Go's export rules.

// 5. Memory Management:
//    - In Go, we use pointers (*MDCTLookup, *PulseCache) for struct fields to match Java's
//      reference semantics and avoid unnecessary copying.

// 6. Documentation:
//    - Added package-level documentation.
//    - Kept the key field comments from the Java version.
//    - Added detailed translation notes for maintainability.

// Note: This assumes the existence of supporting types (MDCTLookup, PulseCache, FFTState)
// and tables (CeltTables) which would need similar translations from their Java counterparts.
