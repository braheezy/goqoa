package mp3

import "errors"

const (
	MPEG_I int = iota
	MPEG_25
	MPEG_II
)

// This is the struct used to tell the encoder about the input PCM
type channel int

const (
	PCM_MONO channel = iota
	PCM_STEREO
)

// Only Layer III currently implemented
const (
	LAYER_III int = iota
)

type Wave struct {
	channel    channel
	sampleRate int
}

// This is the struct the encoder uses to tell the encoder about the output MP3
type mode int

const (
	STEREO mode = iota
	JOINT_STEREO
	DUAL_CHANNEL
	MONO
)

type emphasis int

const (
	NO_EMPHASIS emphasis = iota
	MU50_15
	CITT
)

type MPEG struct {
	// Stereo mode
	mode mode
	// Must conform to known bitrate
	bitRate int
	// De-emphasis
	emphasis  emphasis
	copyright int
	original  int
}

const SHINE_MAX_SAMPLES = 1152

var mpegGranulesPerFrame = [4]int{
	// MPEG 2.5
	1,
	// Reserved
	-1,
	// MPEG II
	1,
	// MPEG I
	2,
}

func (m *MPEG) setDefaults() {
	m.bitRate = 128
	m.emphasis = NO_EMPHASIS
	m.copyright = 0
	m.original = 1
}

// mpegVersion returns the MPEG version used for the given samplerate index. See below
// for a list of possible values.
/* Tables of supported audio parameters & format.
 *
 * Valid samplerates and bitrates.
 * const int samplerates[9] = {
 *   44100, 48000, 32000, // MPEG-I
 *   22050, 24000, 16000, // MPEG-II
 *   11025, 12000, 8000   // MPEG-2.5
 * };
 *
 * const int bitrates[16][4] = {
 * //  MPEG version:
 * //  2.5, reserved,  II,  I
 *   { -1,  -1,        -1,  -1},
 *   { 8,   -1,         8,  32},
 *   { 16,  -1,        16,  40},
 *   { 24,  -1,        24,  48},
 *   { 32,  -1,        32,  56},
 *   { 40,  -1,        40,  64},
 *   { 48,  -1,        48,  80},
 *   { 56,  -1,        56,  96},
 *   { 64,  -1,        64, 112},
 *   { 80,  -1,        80, 128},
 *   { 96,  -1,        96, 160},
 *   {112,  -1,       112, 192},
 *   {128,  -1,       128, 224},
 *   {144,  -1,       144, 256},
 *   {160,  -1,       160, 320},
 *   { -1,  -1,        -1,  -1}
 *  };
 *
 */
func mpegVersion(sampleRateIndex int) int {
	// Pick mpeg version according to samplerate index.
	if sampleRateIndex < 3 {
		// First 3 sampleRates are for MPEG-I
		return MPEG_I
	} else if sampleRateIndex < 6 {
		return MPEG_II
	} else {
		return MPEG_25
	}
}

// findSampleRateIndex checks if a given samplerate is supported by the encoder
func findSampleRateIndex(freq int) (int, error) {
	for i := 0; i < 9; i++ {
		if freq == int(sampleRates[i]) {
			return i, nil
		}
	}
	return -1, errors.New("Unsupported samplerate")
}

// findBitrateIndex checks if a given bitrate is supported by the encoder
func findBitrateIndex(bitrate, version int) (int, error) {
	for i := 0; i < 16; i++ {
		if bitrate == int(bitRates[i][version]) {
			return i, nil
		}
	}
	return -1, errors.New("Unsupported bitrate")
}

// checkConfig checks if a given bitrate and samplerate is supported by the encoder
func checkConfig(freq, bitrate int) (int, error) {
	sampleRateIndex, err := findSampleRateIndex(freq)
	if err != nil {
		return -1, err
	}

	mpegVersion := mpegVersion(sampleRateIndex)
	_, err = findBitrateIndex(bitrate, mpegVersion)
	if err != nil {
		return -1, err
	}
	return mpegVersion, nil
}

// samplesPerPass returns the audio samples expected in each frame.
func (c *globalConfig) samplesPerPass() int {
	return c.mpeg.GranulesPerFrame * GRANULE_SIZE
}

// Pass a pointer to a `config_t` structure and returns an initialized
// encoder.
//
// Configuration data is copied over to the encoder. It is not possible
// to change its values after initializing the encoder at the moment.
//
// Checking for valid configuration values is left for the application to
// implement. You can use the `shine_find_bitrate_index` and
// `shine_find_samplerate_index` functions or the `bitrates` and
// `samplerates` arrays above to check those parameters. Mone and stereo
// mode for wave and mpeg should also be consistent with each other.
//
// This function returns NULL if it was not able to allocate memory data for
// the encoder.
func (c *globalConfig) NewEncoder() (*globalConfig, error) {
	_, err := checkConfig(c.wave.SampleRate, c.mpeg.Bitrate)
	if err != nil {
		return nil, err
	}

	encoder := new(globalConfig)

	subbandInitialize(c)
	mdctInitialize(c)
	loopInitialize(c)

	encoder.wave.Channels = c.wave.Channels
	encoder.wave.SampleRate = c.wave.SampleRate
	encoder.mpeg.Mode = c.mpeg.Mode
	encoder.mpeg.Bitrate = c.mpeg.Bitrate
	encoder.mpeg.EmpH = c.mpeg.EmpH
	encoder.mpeg.Copyright = c.mpeg.Copyright
	encoder.mpeg.Original = c.mpeg.Original

	encoder.mpeg.Layer = LAYER_III
	encoder.mpeg.BitsPerSlot = 8

	encoder.mpeg.SampleRateIndex, err = findSampleRateIndex(encoder.wave.SampleRate)
	if err != nil {
		return nil, err
	}
	encoder.mpeg.Version = mpegVersion(encoder.mpeg.SampleRateIndex)
	encoder.mpeg.BitrateIndex, err = findBitrateIndex(encoder.mpeg.Bitrate, encoder.mpeg.Version)
	if err != nil {
		return nil, err
	}

	encoder.mpeg.GranulesPerFrame = mpegGranulesPerFrame[encoder.mpeg.Version]

	// Figure average number of 'slots' per frame.
	averageSlotsPerFrame := float64(encoder.mpeg.GranulesPerFrame*GRANULE_SIZE) / float64(encoder.wave.SampleRate*1000.0*encoder.mpeg.Bitrate/encoder.mpeg.BitsPerSlot)
	encoder.mpeg.WholeSlotsPerFrame = int(averageSlotsPerFrame)

	encoder.mpeg.FractionSlotsPerFrame = averageSlotsPerFrame - float64(encoder.mpeg.WholeSlotsPerFrame)
	encoder.mpeg.SlotLag = -encoder.mpeg.FractionSlotsPerFrame

	if encoder.mpeg.FractionSlotsPerFrame == 0 {
		encoder.mpeg.Padding = 0
	}

	encoder.bitstream.openBitstream(BUFFER_SIZE)

	// determine the mean bitrate for main data
	if encoder.mpeg.GranulesPerFrame == 2 {
		// MPEG-I
		lenMultiplier := 4 + 32
		if encoder.wave.Channels == 1 {
			lenMultiplier = 4 + 17
		}
		encoder.sideInfoLen = 8 * lenMultiplier
	} else {
		// MPEG-II
		lenMultiplier := 4 + 17
		if encoder.wave.Channels == 1 {
			lenMultiplier = 4 + 9
		}
		encoder.sideInfoLen = 8 * lenMultiplier
	}
	return encoder, nil
}

func encodeBufferInternal(config *globalConfig, written *int, stride int) []byte {
	if config.mpeg.FractionSlotsPerFrame != 0 {
		if config.mpeg.SlotLag <= (config.mpeg.FractionSlotsPerFrame - 1.0) {
			config.mpeg.Padding = 1
		} else {
			config.mpeg.Padding = 0
		}

		config.mpeg.SlotLag += (float64(config.mpeg.Padding) - config.mpeg.FractionSlotsPerFrame)
	}

	config.mpeg.BitsPerFrame = 8 * (config.mpeg.WholeSlotsPerFrame + config.mpeg.Padding)
	config.meanBits = (config.mpeg.BitsPerFrame - config.sideInfoLen) /
		config.mpeg.GranulesPerFrame

	// Apply mdct to the polyphase output
	mdctSub(config, stride)

	// Bit and noise allocation
	iterationLoop(config)

	// Write the frame to the bitstream
	formatBitstream(config)

	// Return data.
	*written = config.bitstream.dataPosition
	config.bitstream.dataPosition = 0

	return config.bitstream.data
}

func encodeBuffer(config *globalConfig, data [][]int16, written *int) []byte {
	config.buffer[0] = data[0]
	if config.wave.Channels == 2 {
		config.buffer[1] = data[1]
	}
	return encodeBufferInternal(config, written, 1)
}

func encodeBufferInterleaved(config *globalConfig, data []int16, written *int) []byte {
	config.buffer[0] = data
	if config.wave.Channels == 2 {
		config.buffer[1] = data[1:]
	}
	return encodeBufferInternal(config, written, config.wave.Channels)

}

func flush(config *globalConfig, written *int) []byte {
	*written = config.bitstream.dataPosition
	config.bitstream.dataPosition = 0
	return config.bitstream.data
}
