package qoa

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
}

/*
	The Least Mean Squares Filter is the heart of QOA. It predicts the next

sample based on the previous 4 reconstructed samples. It does so by continuously
adjusting 4 weights based on the residual of the previous prediction.

The next sample is predicted as the sum of (weight[i] * history[i]).

The adjustment of the weights is done with a "Sign-Sign-LMS" that adds or
subtracts the residual to each weight, based on the corresponding sample from
the history. This, surprisingly, is sufficient to get worthwhile predictions.

This is all done with fixed point integers. Hence the right-shifts when updating
the weights and calculating the prediction.
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

// qoaEncodeHeader encodes the QOA header.
func (q *QOA) encodeHeader() []byte {
	header := make([]byte, 8)
	binary.BigEndian.PutUint32(header, QOAMagic)
	binary.BigEndian.PutUint32(header[4:], q.Samples)
	return header
}
