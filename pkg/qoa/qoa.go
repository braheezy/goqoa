/*
Package qoa provides functionality for encoding and decoding audio data in the QOA format.

The following is from the QOA specification:

# Data Format

QOA encodes pulse-code modulated (PCM) audio data with up to 255 channels,
sample rates from 1 up to 16777215 hertz and a bit depth of 16 bits.

The compression method employed in QOA is lossy; it discards some information
from the uncompressed PCM data. For many types of audio signals this compression
is "transparent", i.e. the difference from the original file is often not
audible.

QOA encodes 20 samples of 16 bit PCM data into slices of 64 bits. A single
sample therefore requires 3.2 bits of storage space, resulting in a 5x
compression (16 / 3.2).

A QOA file consists of an 8 byte file header, followed by a number of frames.
Each frame contains an 8 byte frame header, the current 16 byte en-/decoder
state per channel and 256 slices per channel. Each slice is 8 bytes wide and
encodes 20 samples of audio data.

All values, including the slices, are big endian. The file layout is as follows:

	struct {
		struct {
			char     magic[4];         // magic bytes "qoaf"
			uint32_t samples;          // samples per channel in this file
		} file_header;

		struct {
			struct {
				uint8_t  num_channels; // no. of channels
				uint24_t samplerate;   // samplerate in hz
				uint16_t fsamples;     // samples per channel in this frame
				uint16_t fsize;        // frame size (includes this header)
			} frame_header;

			struct {
				int16_t history[4];    // most recent last
				int16_t weights[4];    // most recent last
			} lms_state[num_channels];

			qoa_slice_t slices[256][num_channels];

		} frames[ceil(samples / (256 * 20))];
	} qoa_file_t;

Each qoa_slice_t contains a quantized scalefactor sf_quant and 20 quantized
residuals qrNN:

	.- QOA_SLICE -- 64 bits, 20 samples --------------------------/  /------------.
	|        Byte[0]         |        Byte[1]         |  Byte[2]  \  \  Byte[7]   |
	| 7  6  5  4  3  2  1  0 | 7  6  5  4  3  2  1  0 | 7  6  5   /  /    2  1  0 |
	|------------+--------+--------+--------+---------+---------+-\  \--+---------|
	|  sf_quant  |  qr00  |  qr01  |  qr02  |  qr03   |  qr04   | /  /  |  qr19   |
	`-------------------------------------------------------------\  \------------`

Each frame except the last must contain exactly 256 slices per channel. The last
frame may contain between 1 .. 256 (inclusive) slices per channel. The last
slice (for each channel) in the last frame may contain less than 20 samples; the
slice still must be 8 bytes wide, with the unused samples zeroed out.

Channels are interleaved per slice. E.g. for 2 channel stereo:
slice[0] = L, slice[1] = R, slice[2] = L, slice[3] = R ...

A valid QOA file or stream must have at least one frame. Each frame must contain
at least one channel and one sample with a samplerate between 1 .. 16777215
(inclusive).

If the total number of samples is not known by the encoder, the samples in the
file header may be set to 0x00000000 to indicate that the encoder is
"streaming". In a streaming context, the samplerate and number of channels may
differ from frame to frame. For static files (those with samples set to a
non-zero value), each frame must have the same number of channels and same
samplerate.

Note that this implementation of QOA only handles files with a known total
number of samples.

A decoder should support at least 8 channels. The channel layout for channel
counts 1 .. 8 is:

 1. Mono
 2. L, R
 3. L, R, C
 4. FL, FR, B/SL, B/SR
 5. FL, FR, C, B/SL, B/SR
 6. FL, FR, C, LFE, B/SL, B/SR
 7. FL, FR, C, LFE, B, SL, SR
 8. FL, FR, C, LFE, BL, BR, SL, SR

QOA predicts each audio sample based on the previously decoded ones using a
"Sign-Sign Least Mean Squares Filter" (LMS). This prediction plus the
dequantized residual forms the final output sample.
*/
package qoa

import (
	"fmt"
	"io"
	"os"
)

const (
	// QOAMagic is the magic number identifying a QOA file
	QOAMagic = 0x716f6166 // 'qoaf'
	// QOAMinFilesize is the minimum valid size of a QOA file.
	QOAMinFilesize = 16
	// QOAMaxChannels is the maximum number of audio channels supported by QOA.
	QOAMaxChannels = 8
	// QOASliceLen is the length of each QOA audio slice.
	QOASliceLen = 20
	// QOASlicesPerFrame is the number of slices per QOA frame.
	QOASlicesPerFrame = 256
	// QOAFrameLen is the length of a QOA frame.
	QOAFrameLen = QOASlicesPerFrame * QOASliceLen
	// QOALMSLen is the length of the LMS state per channel.
	QOALMSLen = 4
)

// qoaFrameSize calculates the size of a QOA frame based on the number of channels and slices.
func qoaFrameSize(channels, slices uint32) uint32 {
	return 8 + QOALMSLen*4*channels + 8*slices*channels
}

// qoaLMS represents the LMS state per channel.
type qoaLMS struct {
	History [4]int16
	Weights [4]int16
}

// QOA stores the QOA audio file description.
type QOA struct {
	Channels   uint32    // Number of audio channels
	SampleRate uint32    // Sample rate of the audio
	Samples    uint32    // Total number of audio samples
	lms        [8]qoaLMS // LMS state per channel
	ErrorCount int       // Sum of best LMS errors encountered during encoding

	prevScaleFactor []int
}

/*
The reciprocal_tab maps each of the 16 scaleFactors to their rounded reciprocals 1/scaleFactor. This allows us to calculate the scaled residuals in the encoder with just one multiplication instead of an expensive division. Do this in .16 fixed point with integers, instead of floats.

The reciprocal_tab is computed as:

qoaReciprocalTable[s] <- ((1<<16) + scaleFactor_tab[s] - 1) / scaleFactor_tab[s]
*/
var qoaReciprocalTable = [16]int{
	65536, 9363, 3121, 1457, 781, 475, 311, 216, 156, 117, 90, 71, 57, 47, 39, 32,
}

/* The quant_tab provides an index into the dequant_tab for residuals in the
range of -8 .. 8. It maps this range to just 3bits and becomes less accurate at
the higher end. Note that the residual zero is identical to the lowest positive
value. This is mostly fine, since the qoa_div() function always rounds away
from zero. */

var qoaQuantTable = [17]int{
	7, 7, 7, 5, 5, 3, 3, 1, /* -8..-1 */
	0,                      /*  0     */
	0, 2, 2, 4, 4, 6, 6, 6, /*  1.. 8 */
}

/* The dequant_tab maps each of the scaleFactors and quantized residuals to
their unscaled & dequantized version.

Since qoa_div rounds away from the zero, the smallest entries are mapped to 3/4
instead of 1. The dequant_tab assumes the following dequantized values for each
of the quant_tab indices and is computed as:
float dqt[8] = {0.75, -0.75, 2.5, -2.5, 4.5, -4.5, 7, -7};
dequant_tab[s][q] <- round_ties_away_from_zero(scaleFactor_tab[s] * dqt[q])

The rounding employed here is "to nearest, ties away from zero",  i.e. positive
and negative values are treated symmetrically.
*/

var qoaDequantTable = [16][8]int16{
	{1, -1, 3, -3, 5, -5, 7, -7},
	{5, -5, 18, -18, 32, -32, 49, -49},
	{16, -16, 53, -53, 95, -95, 147, -147},
	{34, -34, 113, -113, 203, -203, 315, -315},
	{63, -63, 210, -210, 378, -378, 588, -588},
	{104, -104, 345, -345, 621, -621, 966, -966},
	{158, -158, 528, -528, 950, -950, 1477, -1477},
	{228, -228, 760, -760, 1368, -1368, 2128, -2128},
	{316, -316, 1053, -1053, 1895, -1895, 2947, -2947},
	{422, -422, 1405, -1405, 2529, -2529, 3934, -3934},
	{548, -548, 1828, -1828, 3290, -3290, 5117, -5117},
	{696, -696, 2320, -2320, 4176, -4176, 6496, -6496},
	{868, -868, 2893, -2893, 5207, -5207, 8099, -8099},
	{1064, -1064, 3548, -3548, 6386, -6386, 9933, -9933},
	{1286, -1286, 4288, -4288, 7718, -7718, 12005, -12005},
	{1536, -1536, 5120, -5120, 9216, -9216, 14336, -14336},
}

/*
The Least Mean Squares Filter is the heart of QOA. It predicts the next sample based on the previous 4 reconstructed samples. It does so by continuously adjusting 4 weights based on the residual of the previous prediction.

The next sample is predicted as the sum of (weight[i] * history[i]).

The adjustment of the weights is done with a "Sign-Sign-LMS" that adds or subtracts the residual to each weight, based on the corresponding sample from the history. This, surprisingly, is sufficient to get worthwhile predictions.

This is all done with fixed point integers. Hence the right-shifts when updating the weights and calculating the prediction.
*/
func (lms *qoaLMS) predict() int {
	return (int(lms.Weights[0])*int(lms.History[0]) +
		int(lms.Weights[1])*int(lms.History[1]) +
		int(lms.Weights[2])*int(lms.History[2]) +
		int(lms.Weights[3])*int(lms.History[3])) >> 13
}

func (lms *qoaLMS) update(sample int16, residual int16) {
	// NB: From the spec author:
	// "Note that the right shift residual >> 4 in qoa_lms_update() is just there to ensure that the weights will stay within the 16 bit range (I have not proven that they do, but with all my test samples: they do)
	// The right shift prediction >> 13 in qoa_lms_predict() above then does the rest.
	delta := residual >> 4

	for i := 0; i < QOALMSLen; i++ {
		if lms.History[i] < 0 {
			lms.Weights[i] -= delta
		} else {
			lms.Weights[i] += delta
		}
	}

	lms.History[0] = lms.History[1]
	lms.History[1] = lms.History[2]
	lms.History[2] = lms.History[3]
	lms.History[3] = sample
}

// clamps a value between a minimum and maximum value.
func clamp(v, min, max int) int {
	if v <= min {
		return min
	}
	if v >= max {
		return max
	}
	return v
}

/*
This specialized clamp function for the signed 16 bit range improves decode performance quite a bit. The extra if() statement works nicely with the CPUs branch prediction as this branch is rarely taken.
*/
func clampS16(v int) int16 {
	if uint(v+32768) > 65535 {
		if v <= -32768 {
			return -32768
		}
		if v >= 32767 {
			return 32767
		}
	}
	return int16(v)
}

func IsValidQOAFile(inputFile string) (bool, error) {
	// Read first 4 bytes of the file
	fileBytes := make([]byte, 4)
	file, err := os.Open(inputFile)
	if err != nil {
		return false, err
	}
	defer file.Close()

	_, err = file.Read(fileBytes)
	if err != nil && err != io.EOF {
		return false, err
	}

	// Check if the first 4 bytes are magic word `qoaf`
	if string(fileBytes) != "qoaf" {
		return false, fmt.Errorf("no magic word 'qoaf' found in %s", inputFile)
	}
	return true, nil
}
