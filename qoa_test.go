package qoa

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeHeader(t *testing.T) {
	qoa := &QOA{
		Channels:   2,
		SampleRate: 44100,
		Samples:    88200,
		LMS:        []qoaLMS{},
	}

	expectedHeader := []byte{
		0x71, 0x6f, 0x61, 0x66, // 'qoaf'
		0x00, 0x01, 0x58, 0x88, // Samples: 88200 (0x15888)
	}

	header := qoa.encodeHeader()

	if !reflect.DeepEqual(header, expectedHeader) {
		t.Errorf("Header encoding mismatch.\nExpected: %#v\nGot:      %#v", expectedHeader, header)
	}
}

func TestLMSPredict(t *testing.T) {
	lms := qoaLMS{
		History: [QOALMSLen]int16{100, -200, 300, -400},
		Weights: [QOALMSLen]int16{1, 2, -1, -2},
	}

	actual := lms.predict()
	expected := (100*1 + (-200)*2 + 300*(-1) + (-400)*(-2)) >> 13
	assert.Equal(t, expected, actual)
}

func TestLMSUpdate(t *testing.T) {
	testCases := []struct {
		name            string
		initialHistory  [QOALMSLen]int16
		initialWeights  [QOALMSLen]int16
		sample          int16
		residual        int16
		expectedWeights [QOALMSLen]int16
		expectedHistory [QOALMSLen]int16
	}{
		{
			name:            "Basic Update",
			initialHistory:  [QOALMSLen]int16{1, 2, 3, 4},
			initialWeights:  [QOALMSLen]int16{1, 1, 1, 1},
			sample:          10,
			residual:        3,
			expectedWeights: [QOALMSLen]int16{1, 1, 1 + (3 >> 4), 1},
			expectedHistory: [QOALMSLen]int16{2, 3, 4, 10},
		},
		{
			name:            "Negative Residual Update",
			initialHistory:  [QOALMSLen]int16{0, 0, 0, 0},
			initialWeights:  [QOALMSLen]int16{1, 2, 3, 4},
			sample:          10,
			residual:        -2,
			expectedWeights: [QOALMSLen]int16{1 + (-2 >> 4), 2 + (-2 >> 4), 3 + (-2 >> 4), 4 + (-2 >> 4)},
			expectedHistory: [QOALMSLen]int16{0, 0, 0, 10},
		},
		{
			name:            "Zero Residual Update",
			initialHistory:  [QOALMSLen]int16{5, 5, 5, 5},
			initialWeights:  [QOALMSLen]int16{1, 2, 3, 4},
			sample:          15,
			residual:        0,
			expectedWeights: [QOALMSLen]int16{1, 2, 3, 4},
			expectedHistory: [QOALMSLen]int16{5, 5, 5, 15},
		},
		{
			name:            "Negative History Update",
			initialHistory:  [QOALMSLen]int16{5, -5, 5, -5},
			initialWeights:  [QOALMSLen]int16{1, 2, 3, 4},
			sample:          69,
			residual:        4,
			expectedWeights: [QOALMSLen]int16{1 + (4 >> 4), 2 - (4 >> 4), 3 + (4 >> 4), 4 - (4 >> 4)},
			expectedHistory: [QOALMSLen]int16{-5, 5, -5, 69},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lms := qoaLMS{
				History: tc.initialHistory,
				Weights: tc.initialWeights,
			}

			lms.update(tc.sample, tc.residual)

			assert.Equal(t, tc.expectedWeights, lms.Weights, "Incorrect updated weights")
			assert.Equal(t, tc.expectedHistory, lms.History, "Incorrect updated history")
		})
	}
}
