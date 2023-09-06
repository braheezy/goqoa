package mp3

const (
	PI          = 3.14159265358979
	PI4         = 0.78539816339745
	PI12        = 0.26179938779915
	PI36        = 0.087266462599717
	SQRT2       = 1.41421356237
	LN2         = 0.69314718
	LN_TO_LOG10 = 0.2302585093
	BLKSIZE     = 1024
	/* for loop unrolling, require that HAN_SIZE%8==0 */
	HAN_SIZE     = 512
	SCALE_BLOCK  = 12
	SCALE_RANGE  = 64
	SCALE        = 32768
	SBLIMIT      = 32
	MAX_CHANNELS = 2
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

type wave struct {
	channels   int
	sampleRate int
}
