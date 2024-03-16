package qoa

import (
	"encoding/binary"
	"errors"
)

// encodeHeader encodes the QOA header.
// The QOA header is defined as the magic number `qoaf` followed by the number of samples in the file.
func (q *QOA) encodeHeader(header []byte) {
	binary.BigEndian.PutUint32(header, QOAMagic)
	binary.BigEndian.PutUint32(header[4:], q.Samples)
}

// encodeFrame encodes a QOA frame using the provided sample data and returns the size of the encoded frame.
// Each frame contains an 8 byte frame header, the current 16 byte en-/decoder state per channel and 256 slices per channel. Each slice is 8 bytes wide and encodes 20 samples of audio data.
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
			bestRank := -1
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
				currentRank := uint64(0)
				currentError := uint64(0)

				for si := sliceStart; si < sliceEnd; si += channels {
					sample := int(sampleData[si])
					predicted := lms.predict()

					residual := sample - predicted
					scaled := div(residual, scaleFactor)
					clamped := clamp(scaled, -8, 8)
					quantized := qoaQuantTable[clamped+8]
					dequantized := qoaDequantTable[scaleFactor][quantized]
					reconstructed := clampS16(predicted + int(dequantized))

					// If the weights have grown too large, we introduce a penalty here. This prevents pops/clicks
					// in certain problem cases.
					weightsPenalty := (int(
						q.LMS[c].Weights[0]*q.LMS[c].Weights[0]+
							q.LMS[c].Weights[1]*q.LMS[c].Weights[1]+
							q.LMS[c].Weights[2]*q.LMS[c].Weights[2]+
							q.LMS[c].Weights[3]*q.LMS[c].Weights[3]) >> 18) - 0x8ff
					if weightsPenalty < 0 {
						weightsPenalty = 0
					}

					errDelta := int64(sample - int(reconstructed))
					errorSquared := uint64(errDelta * errDelta)
					currentRank += errorSquared + uint64(weightsPenalty)*uint64(weightsPenalty)
					currentError += errorSquared
					if currentError > uint64(bestRank) {
						break
					}

					lms.update(reconstructed, dequantized)
					slice = (slice << 3) | uint64(quantized)
				}

				if currentError < uint64(bestRank) {
					bestRank = int(currentRank)
					bestError = int(currentError)
					bestSlice = slice
					bestLMS = lms
					bestScaleFactor = scaleFactor
				}
			}

			prevScaleFactor[c] = bestScaleFactor

			q.LMS[c] = bestLMS
			q.ErrorCount += bestError

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

// Encode encodes the provided audio sample data in QOA format and returns the encoded bytes.
func (q *QOA) Encode(sampleData []int16) ([]byte, error) {
	if q.Samples == 0 || q.SampleRate == 0 || q.SampleRate > 0xffffff ||
		q.Channels == 0 || q.Channels > QOAMaxChannels {
		return nil, errors.New("invalid QOA parameters")
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
	q.ErrorCount = 0

	frameLen := uint32(QOAFrameLen)
	for sampleIndex := uint32(0); sampleIndex < q.Samples; sampleIndex += frameLen {
		frameLen = uint32(clamp(QOAFrameLen, 0, int(q.Samples-sampleIndex)))
		if (sampleIndex+frameLen)*q.Channels > uint32(len(sampleData)) {
			return nil, errors.New("not enough samples")
		}
		frameSamples := sampleData[sampleIndex*q.Channels : (sampleIndex+frameLen)*q.Channels]
		frameSize := q.encodeFrame(frameSamples, frameLen, bytes[p:])
		p += uint32(frameSize)
	}
	return bytes, nil
}

// NewEncoder creates a new QOA encoder with the specified sample rate, channels, and samples.
func NewEncoder(sampleRate, channels, samples uint32) *QOA {
	return &QOA{
		SampleRate: sampleRate,
		Channels:   channels,
		Samples:    samples,
	}
}
