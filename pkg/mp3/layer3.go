package mp3

import (
	"fmt"
)

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
	return -1, fmt.Errorf("unsupported samplerate: %v", freq)
}

// findBitrateIndex checks if a given bitrate is supported by the encoder
func findBitrateIndex(bitrate, version int) (int, error) {
	for i := 0; i < 16; i++ {
		if bitrate == int(bitRates[i][version]) {
			return i, nil
		}
	}
	return -1, fmt.Errorf("unsupported bitrate: %v for version %v", bitrate, version)
}

// CheckConfig checks if a given bitrate and samplerate is supported by the encoder
func CheckConfig(freq, bitrate int) (int, error) {
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
func (c *GlobalConfig) samplesPerPass() int {
	return c.MPEG.GranulesPerFrame * GRANULE_SIZE
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
func (c *GlobalConfig) NewEncoder() (*GlobalConfig, error) {

	var err error
	encoder := new(GlobalConfig)

	subbandInitialize(c)
	mdctInitialize(c)
	loopInitialize(c)

	encoder.Wave.Channels = c.Wave.Channels
	encoder.Wave.SampleRate = c.Wave.SampleRate
	encoder.MPEG.Mode = c.MPEG.Mode
	encoder.MPEG.Bitrate = c.MPEG.Bitrate
	encoder.MPEG.EmpH = c.MPEG.EmpH
	encoder.MPEG.Copyright = c.MPEG.Copyright
	encoder.MPEG.Original = c.MPEG.Original

	encoder.MPEG.Layer = LAYER_III
	encoder.MPEG.BitsPerSlot = 8

	encoder.MPEG.SampleRateIndex, err = findSampleRateIndex(encoder.Wave.SampleRate)
	if err != nil {
		return nil, err
	}
	encoder.MPEG.Version = mpegVersion(encoder.MPEG.SampleRateIndex)
	encoder.MPEG.BitrateIndex, err = findBitrateIndex(encoder.MPEG.Bitrate, encoder.MPEG.Version)
	if err != nil {
		return nil, err
	}

	encoder.MPEG.GranulesPerFrame = mpegGranulesPerFrame[encoder.MPEG.Version]

	// Figure average number of 'slots' per frame.
	averageSlotsPerFrame := float64(encoder.MPEG.GranulesPerFrame*GRANULE_SIZE) / float64(encoder.Wave.SampleRate*1000.0*encoder.MPEG.Bitrate/encoder.MPEG.BitsPerSlot)
	encoder.MPEG.WholeSlotsPerFrame = int(averageSlotsPerFrame)

	encoder.MPEG.FractionSlotsPerFrame = averageSlotsPerFrame - float64(encoder.MPEG.WholeSlotsPerFrame)
	encoder.MPEG.SlotLag = -encoder.MPEG.FractionSlotsPerFrame

	if encoder.MPEG.FractionSlotsPerFrame == 0 {
		encoder.MPEG.Padding = 0
	}

	encoder.bitstream.openBitstream(BUFFER_SIZE)

	// determine the mean bitrate for main data
	if encoder.MPEG.GranulesPerFrame == 2 {
		// MPEG-I
		lenMultiplier := 4 + 32
		if encoder.Wave.Channels == 1 {
			lenMultiplier = 4 + 17
		}
		encoder.sideInfoLen = 8 * lenMultiplier
	} else {
		// MPEG-II
		lenMultiplier := 4 + 17
		if encoder.Wave.Channels == 1 {
			lenMultiplier = 4 + 9
		}
		encoder.sideInfoLen = 8 * lenMultiplier
	}
	return encoder, nil
}

func encodeBufferInternal(config *GlobalConfig, stride int) []byte {
	if config.MPEG.FractionSlotsPerFrame != 0 {
		if config.MPEG.SlotLag <= (config.MPEG.FractionSlotsPerFrame - 1.0) {
			config.MPEG.Padding = 1
		} else {
			config.MPEG.Padding = 0
		}

		config.MPEG.SlotLag += (float64(config.MPEG.Padding) - config.MPEG.FractionSlotsPerFrame)
	}

	config.MPEG.BitsPerFrame = 8 * (config.MPEG.WholeSlotsPerFrame + config.MPEG.Padding)
	config.meanBits = (config.MPEG.BitsPerFrame - config.sideInfoLen) /
		config.MPEG.GranulesPerFrame

	// Apply mdct to the polyphase output
	mdctSub(config, stride)

	// Bit and noise allocation
	iterationLoop(config)

	// Write the frame to the bitstream
	formatBitstream(config)

	// Return data.
	// *written = config.bitstream.dataPosition
	config.bitstream.dataPosition = 0

	return config.bitstream.data
}

func encodeBuffer(config *GlobalConfig, data [][]int16, written *int) []byte {
	config.buffer[0] = data[0]
	if config.Wave.Channels == 2 {
		config.buffer[1] = data[1]
	}
	return encodeBufferInternal(config, 1)
}

func encodeBufferInterleaved(config *GlobalConfig, data []int16) []byte {
	config.buffer[0] = data
	if config.Wave.Channels == 2 {
		config.buffer[1] = data[1:]
	}
	return encodeBufferInternal(config, config.Wave.Channels)

}

func flush(config *GlobalConfig, written *int) []byte {
	*written = config.bitstream.dataPosition
	config.bitstream.dataPosition = 0
	return config.bitstream.data
}
