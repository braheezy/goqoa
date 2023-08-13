package qoa

/*
Copyright (c) 2023, braheezy - https://github.com/braheezy
SPDX-License-Identifier: MIT

QOA - The "Quite OK Audio" format for fast, lossy audio compression

Most of the important comments in this file are not mine, they are the original author's.
*/

import (
	"encoding/binary"
)

// QOA constants
const (
	QOAMagic          = 0x716f6166 // 'qoaf'
	QOAMinFilesize    = 16
	QOAMaxChannels    = 8
	QOASliceLen       = 20
	QOASlicesPerFrame = 256
	QOAFrameLen       = QOASlicesPerFrame * QOASliceLen
	QOALMSLen         = 4
)

func qoaFrameSize(channels, slices uint32) uint32 {
	return 8 + QOALMSLen*4*channels + 8*slices*channels
}

// qoaLMS represents the LMS state per channel.
type qoaLMS struct {
	History [QOALMSLen]int16
	Weights [QOALMSLen]int16
}

// qoaSlice represents a quantized slice of audio data.
type qoaSlice [QOASliceLen]byte

// QOA stores the QOA audio file description.
type QOA struct {
	Channels   uint32
	SampleRate uint32
	Samples    uint32
	LMS        []qoaLMS
	errorCount int
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

var qoaDequantTable = [16][8]int{
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
	prediction := 0
	for i := 0; i < QOALMSLen; i++ {
		prediction += int(lms.Weights[i]) * int(lms.History[i])
	}
	return prediction >> 13
}

func (lms *qoaLMS) update(sample int16, residual int16) {
	delta := residual >> 4
	for i := 0; i < QOALMSLen; i++ {
		if lms.History[i] < 0 {
			lms.Weights[i] -= delta
		} else {
			lms.Weights[i] += delta
		}
	}

	for i := 0; i < QOALMSLen-1; i++ {
		lms.History[i] = lms.History[i+1]
	}
	lms.History[QOALMSLen-1] = sample
}

/*
div() implements a rounding division, but avoids rounding to zero for small numbers. E.g. 0.1 will be rounded to 1. Note that 0 itself still returns as 0, which is handled in the qoa_quant_tab[]. qoa_div() takes an index into the .16 fixed point qoa_reciprocal_tab as an argument, so it can do the division with a cheaper integer multiplication.
*/
func div(v, scaleFactor int) int {
	reciprocal := qoaReciprocalTable[scaleFactor]
	n := (v*reciprocal + (1 << 15)) >> 16
	n += (v >> 31) - (n >> 31) // Round away from 0
	return n
}

// clamps a value between a minimum and maximum value.
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

/*
This specialized clamp function for the signed 16 bit range improves decode performance quite a bit. The extra if() statement works nicely with the CPUs branch prediction as this branch is rarely taken.
*/
func clampS16(v int) int16 {
	if uint(v+32768) > 65535 {
		if v < -32768 {
			return -32768
		}
		if v > 32767 {
			return 32767
		}
	}
	return int16(v)
}

// encodeHeader encodes the QOA header.
func (q *QOA) encodeHeader(header []byte) {
	binary.BigEndian.PutUint32(header, QOAMagic)
	binary.BigEndian.PutUint32(header[4:], q.Samples)
}

func (q *QOA) encodeFrame(sampleData []int16, frameLen uint32, bytes []byte) uint {
	channels := q.Channels

	p := uint(0)

	slices := (frameLen + QOASliceLen - 1) / QOASliceLen
	frameSize := qoaFrameSize(channels, slices)
	prevScaleFactor := make([]int, QOAMaxChannels)

	// Write the frame header
	binary.BigEndian.PutUint64(
		bytes,
		uint64(q.Channels)<<56|
			uint64(q.SampleRate)<<32|
			uint64(frameLen)<<16|
			uint64(frameSize),
	)
	p += 8

	for c := uint32(0); c < channels; c++ {
		/* If the weights have grown too large, reset them to 0. This may happen
		with certain high-frequency sounds. This is a last resort and will
		introduce quite a bit of noise, but should at least prevent pops/clicks */
		weightsSum :=
			int(q.LMS[c].Weights[0]*q.LMS[c].Weights[0] +
				q.LMS[c].Weights[1]*q.LMS[c].Weights[1] +
				q.LMS[c].Weights[2]*q.LMS[c].Weights[2] +
				q.LMS[c].Weights[3]*q.LMS[c].Weights[3])
		if weightsSum > 0x2fffffff {
			q.LMS[c].Weights[0] = 0
			q.LMS[c].Weights[1] = 0
			q.LMS[c].Weights[2] = 0
			q.LMS[c].Weights[3] = 0
		}

		// Write the current LMS state
		history := uint64(0)
		weights := uint64(0)
		for i := 0; i < QOALMSLen; i++ {
			history = history<<16 | uint64(q.LMS[c].History[i])&0xffff
			weights = weights<<16 | uint64(q.LMS[c].Weights[i])&0xffff
		}
		binary.BigEndian.PutUint64(bytes[p:], history)
		p += 8
		binary.BigEndian.PutUint64(bytes[p:], weights)
		p += 8
	}

	// Encode all samples with interleaved channels on a slice level. E.g. for stereo: (ch-0, slice 0), (ch 1, slice 0), (ch 0, slice 1), ...
	for sampleIndex := uint32(0); sampleIndex < frameLen; sampleIndex += QOASliceLen {
		for c := uint32(0); c < channels; c++ {
			sliceLen := clamp(QOASliceLen, 0, int(frameLen-sampleIndex))
			sliceStart := sampleIndex*channels + c
			sliceEnd := (sampleIndex+uint32(sliceLen))*channels + c

			/* Brute force search for the best scaleFactor go through all
			16 scaleFactors, encode all samples for the current slice and
			measure the total squared error. */
			bestError := -1
			var bestSlice uint64
			var bestLMS qoaLMS
			var bestScaleFactor int

			for sfi := 0; sfi < 16; sfi++ {
				/* There is a strong correlation between the scaleFactors of
				neighboring slices. As an optimization, start testing
				the best scaleFactor of the previous slice first. */
				scaleFactor := (sfi + prevScaleFactor[c]) % 16

				/* Reset the LMS state to the last known good one
				before trying each scaleFactor, as each pass updates the LMS
				state when encoding. */
				lms := q.LMS[c]
				slice := uint64(scaleFactor)
				currentError := uint64(0)

				for si := sliceStart; si < sliceEnd; si += channels {
					sample := int(sampleData[si])
					predicted := lms.predict()

					residual := sample - predicted
					scaled := div(residual, scaleFactor)
					clamped := clamp(scaled, -8, 8)
					quantized := qoaQuantTable[clamped+8]
					dequantized := qoaDequantTable[scaleFactor][quantized]
					reconstructed := clampS16(predicted + dequantized)

					err := int64(sample - int(reconstructed))
					currentError += uint64(err * err)
					if currentError > uint64(bestError) {
						break
					}

					lms.update(reconstructed, int16(dequantized))
					slice = (slice << 3) | uint64(quantized)
				}

				if currentError < uint64(bestError) {
					bestError = int(currentError)
					bestSlice = slice
					bestLMS = lms
					bestScaleFactor = scaleFactor
				}
			}

			prevScaleFactor[c] = bestScaleFactor

			q.LMS[c] = bestLMS
			q.errorCount += bestError

			/* If this slice was shorter than QOA_SLICE_LEN, we have to left-
			shift all encoded data, to ensure the rightmost bits are the empty
			ones. This should only happen in the last frame of a file as all
			slices are completely filled otherwise. */
			bestSlice <<= (QOASliceLen - sliceLen) * 3
			binary.BigEndian.PutUint64(bytes[p:], bestSlice)
			p += 8
		}
	}

	return p
}

func (q *QOA) encode(sampleData []int16) []byte {
	if q.Samples == 0 || q.SampleRate == 0 || q.SampleRate > 0xffffff ||
		q.Channels == 0 || q.Channels > QOAMaxChannels {
		return nil
	}

	// Calculate the encoded size and allocate
	numFrames := (q.Samples + QOAFrameLen - 1) / QOAFrameLen
	numSlices := (q.Samples + QOASliceLen - 1) / QOASliceLen
	encodedSize := 8 + // 8 byte file header
		numFrames*8 + // 8 byte frame headers
		numFrames*uint32(QOALMSLen)*4*q.Channels + // 4 * 4 bytes lms state per channel
		numSlices*8*q.Channels // 8 byte slices

	bytes := make([]byte, encodedSize)

	for c := uint32(0); c < q.Channels; c++ {
		/* Set the initial LMS weights to {0, 0, -1, 2}. This helps with the
		prediction of the first few ms of a file. */
		q.LMS[c].Weights[0] = 0
		q.LMS[c].Weights[1] = 0
		q.LMS[c].Weights[2] = -(1 << 13)
		q.LMS[c].Weights[3] = 1 << 14

		/* Explicitly set the history samples to 0, as we might have some
		garbage in there. */
		for i := 0; i < QOALMSLen; i++ {
			q.LMS[c].History[i] = 0
		}
	}

	// Encode the header and go through all frames
	q.encodeHeader(bytes)
	p := uint32(8)
	q.errorCount = 0

	frameLen := uint32(QOAFrameLen)
	for sampleIndex := uint32(0); sampleIndex < q.Samples; sampleIndex += uint32(frameLen) {
		frameLen = uint32(clamp(QOAFrameLen, 0, int(q.Samples-sampleIndex)))
		frameSamples := sampleData[sampleIndex*q.Channels : (sampleIndex+frameLen)*q.Channels]
		frameSize := q.encodeFrame(frameSamples, frameLen, bytes[p:])
		p += uint32(frameSize)
	}
	return bytes
}
