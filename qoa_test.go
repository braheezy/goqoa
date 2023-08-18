package qoa

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"

	"github.com/go-audio/wav"
	"github.com/stretchr/testify/assert"
)

func TestEncodeHeader(t *testing.T) {
	qoa := &QOA{
		Channels:   2,
		SampleRate: 44100,
		Samples:    88200,
		LMS:        [QOAMaxChannels]qoaLMS{},
	}

	expectedHeader := []byte{
		0x71, 0x6f, 0x61, 0x66, // 'qoaf'
		0x00, 0x01, 0x58, 0x88, // Samples: 88200 (0x15888)
	}

	header := make([]byte, 8)

	qoa.encodeHeader(header)

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

func TestDiv(t *testing.T) {
	testCases := []struct {
		v           int
		scaleFactor int
		expected    int
	}{
		{100, 1, 14},
		{-100, 1, -14},
		{70, 2, 3},
		{-70, 2, -3},
		{0, 2, 0},
		{1, 0, 1},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test Case %d", i), func(t *testing.T) {
			result := div(tc.v, tc.scaleFactor)
			assert.Equal(t, tc.expected, result, "Incorrect result")
		})
	}
}

func TestClamp(t *testing.T) {
	testCases := []struct {
		v, min, max int
		expected    int
	}{
		{5, 0, 10, 5},
		{15, 0, 10, 10},
		{-5, -10, 0, -5},
		{-15, -10, 0, -10},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test Case %d", i), func(t *testing.T) {
			result := clamp(tc.v, tc.min, tc.max)
			assert.Equal(t, tc.expected, result, "Incorrect result")
		})
	}
}

func TestClampS16(t *testing.T) {
	testCases := []struct {
		v        int
		expected int16
	}{
		{32767, 32767},
		{32768, 32767},
		{32769, 32767},
		{-32768, -32768},
		{-32769, -32768},
		{-32770, -32768},
		{10000, 10000},
		{-15000, -15000},
		{0, 0},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test Case %d", i), func(t *testing.T) {
			result := clampS16(tc.v)
			assert.Equal(t, tc.expected, result, "Incorrect result")
		})
	}
}

func TestDecodeHeader(t *testing.T) {
	testCases := []struct {
		desc        string
		bytes       []byte
		expectedQOA QOA
		hasError    bool
	}{
		{
			desc:  "Valid header",
			bytes: []byte{0x71, 0x6f, 0x61, 0x66, 0x00, 0x00, 0x00, 0x01, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			expectedQOA: QOA{
				Samples:    1,
				Channels:   1,
				SampleRate: 131844,
			},
			hasError: false,
		},
		{
			desc:     "Invalid magic number",
			bytes:    []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			hasError: true,
		},
		{
			desc:     "Invalid file size",
			bytes:    []byte{},
			hasError: true,
		},
		{
			desc:     "No samples",
			bytes:    []byte{0x71, 0x6f, 0x61, 0x66, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0, 0x00, 0x0, 0x0, 0x0, 0x00, 0x00, 0x00, 0x00},
			hasError: true,
		},
		{
			desc: "Bad first frame header",
			bytes: []byte{
				0x71, 0x6f, 0x61, 0x66, // Magic number: 'qoaf'
				0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, // Total number of samples: 1
				0x00, 0x00, 0x00, 0x00, // Frame header (invalid): Channels = 0, SampleRate = 0
			},
			hasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			q := QOA{}
			err := q.decodeHeader(tc.bytes, len(tc.bytes))
			if tc.hasError {
				assert.NotNil(t, err, "Expected error")
			} else {
				assert.Equal(t, tc.expectedQOA, q, "Incorrect QOA data")
				assert.Nil(t, err, "Unexpected error")
			}
		})
	}
}

func TestDecode(t *testing.T) {
	// Load the QOA audio file
	qoaBytes, err := os.ReadFile("test.qoa")
	if err != nil {
		log.Fatalf("Error reading QOA file: %v", err)
	}
	// Decode the QOA audio data
	q := QOA{}
	_, err = q.Decode(qoaBytes, len(qoaBytes))

	assert.Nil(t, err, "Unexpected error")
	assert.NotEmpty(t, q.Samples, "Expected samples")
	assert.NotEmpty(t, q.Channels, "Expected channels")
	assert.NotEmpty(t, q.SampleRate, "Expected sample rate")
	assert.NotEmpty(t, q.LMS[0], "Expected LMS data")
}

func TestEncode(t *testing.T) {
	wavFile, err := os.Open("test.wav")
	if err != nil {
		log.Fatalf("Error reading WAV file: %v", err)
	}
	defer wavFile.Close()

	// Decode WAV audio data
	wavDecoder := wav.NewDecoder(wavFile)
	wavBuffer, err := wavDecoder.FullPCMBuffer()
	if err != nil {
		log.Fatalf("Error decoding WAV file: %v", err)
	}

	q := QOA{
		Channels:   uint32(wavBuffer.Format.NumChannels),
		SampleRate: uint32(wavBuffer.Format.SampleRate),
		Samples:    uint32(len(wavBuffer.Data) / wavBuffer.Format.NumChannels),
	}

	// Convert the audio data to int16 (QOA format)
	int16AudioData := make([]int16, len(wavBuffer.Data))
	for i, val := range wavBuffer.Data {
		int16AudioData[i] = int16(val)
	}

	// Encode the audio data using QOA
	qoaEncodedData, err := q.Encode(int16AudioData)

	assert.Nil(t, err, "Unexpected error")
	assert.NotEmpty(t, qoaEncodedData, "Expected QOA encoded data")
}
