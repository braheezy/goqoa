package mp3

const (
	PI          = 3.14159265358979
	PI4         = 0.78539816339745
	PI12        = 0.26179938779915
	PI36        = 0.087266462599717
	PI64        = 0.049087385212
	SQRT2       = 1.41421356237
	LN2         = 0.69314718
	LN_TO_LOG10 = 0.2302585093
	BLKSIZE     = 1024
	/* for loop unrolling, require that HAN_SIZE%8==0 */
	HAN_SIZE      = 512
	SCALE_BLOCK   = 12
	SCALE_RANGE   = 64
	SCALE         = 32768
	SUBBAND_LIMIT = 32
	MAX_CHANNELS  = 2
	//  a granule is a fundamental unit of audio data that corresponds to a specific time interval within the audio stream. Each granule typically represents a short duration of audio, and MP3 encoding divides audio data into granules for efficient compression
	GRANULE_SIZE = 576
	MAX_GRANULES = 2
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type Wave struct {
	Channels   int
	SampleRate int
}

type GranuleInfo struct {
	Part2_3Length         uint
	BigValues             uint
	Count1                uint
	GlobalGain            uint
	ScaleFactorCompress   uint
	TableSelect           [3]uint
	Region0Count          uint
	Region1Count          uint
	PreFlag               uint
	ScaleFactorScale      uint
	Count1TableSelect     uint
	Part2Length           uint
	ScaleFactorBandMaxLen uint
	Address1              uint
	Address2              uint
	Address3              uint
	QuantizerStepSize     int
	ScaleFactorLen        [4]uint
}

type SideInfo struct {
	privateBits           uint
	reservoirDrain        int
	scaleFactorSelectInfo [MAX_CHANNELS][4]int
	granules              [MAX_GRANULES]struct {
		channels [MAX_CHANNELS]GranuleInfo
	}
}
type Mpeg struct {
	Version          int
	Layer            int
	GranulesPerFrame int
	// Stereo mode
	Mode int
	// Must conform to known bitrate
	Bitrate int
	// De-emphasis
	EmpH                  int
	Padding               int
	BitsPerFrame          int
	BitsPerSlot           int
	FractionSlotsPerFrame float64
	SlotLag               float64
	WholeSlotsPerFrame    int
	BitrateIndex          int
	SampleRateIndex       int
	Crc                   int
	Ext                   int
	ModeExt               int
	Copyright             int
	Original              int
}

type PsyRatio struct {
	l [MAX_GRANULES][MAX_CHANNELS][21]float64
}

type L3loop struct {
	// Magnitudes of the spectral values
	XR *int32
	// xr squared
	Xrsq [GRANULE_SIZE]int32
	// xr absolute
	Xrabs [GRANULE_SIZE]int32
	// Maximum of xrabs array
	Xrmax int32
	// gr
	EnTot  [MAX_GRANULES]int32
	En     [MAX_GRANULES][21]int32
	Xm     [MAX_GRANULES][21]int32
	Xrmaxl [MAX_GRANULES]int32
	// 2**(-x/4) for x = -127..0
	Steptab [128]float64
	// 2**(-x/4) for x = -127..0
	Steptabi [128]int32
	// x**(3/4) for x = 0..9999
	Int2idx [10000]int
}

type MDCT struct {
	cosL [18][36]int32
}

type ScaleFactor struct {
	// [cb]
	l [MAX_GRANULES][MAX_CHANNELS][22]int32
	// [window][cb]
	s [MAX_GRANULES][MAX_CHANNELS][13][3]int32
}

type Subband struct {
	off [MAX_CHANNELS]int32
	fl  [SUBBAND_LIMIT][64]int32
	x   [MAX_CHANNELS][HAN_SIZE]int32
}
type globalConfig struct {
	wave             Wave
	mpeg             Mpeg
	bitstream        Bitstream
	sideInfo         SideInfo
	sideInfoLen      int
	meanBits         int
	ratio            PsyRatio
	scaleFactor      ScaleFactor
	buffer           [MAX_CHANNELS]int16
	psychoEnergy     [MAX_CHANNELS][MAX_GRANULES]float64
	l3Encoding       [MAX_CHANNELS][MAX_GRANULES][GRANULE_SIZE]int
	l3SubbandSamples [MAX_CHANNELS][MAX_GRANULES + 1][18][SUBBAND_LIMIT]int32
	mdctFrequency    [MAX_CHANNELS][MAX_GRANULES][GRANULE_SIZE]int32
	reservoirSize    int
	reservoirMaxSize int
	l3loop           L3loop
	mdct             MDCT
	subband          Subband
}
