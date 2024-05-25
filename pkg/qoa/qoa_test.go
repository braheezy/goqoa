package qoa

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"testing"

	"github.com/go-audio/wav"
	"github.com/stretchr/testify/assert"
)

func TestEncodeHeader(t *testing.T) {
	qoa := &QOA{
		Channels:   2,
		SampleRate: 44100,
		Samples:    88200,
		lms:        [QOAMaxChannels]qoaLMS{},
	}

	expectedHeader := []byte{
		0x71, 0x6f, 0x61, 0x66, // 'qoaf'
		0x00, 0x01, 0x58, 0x88, // Samples: 88200 (0x15888)
	}

	header := make([]byte, 8)

	qoa.encodeHeader(header)

	if !bytes.Equal(header, expectedHeader) {
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
			q, err := DecodeHeader(tc.bytes, len(tc.bytes))
			if tc.hasError {
				assert.NotNil(t, err, "Expected error")
			} else {
				assert.Equal(t, tc.expectedQOA, *q, "Incorrect QOA data")
				assert.Nil(t, err, "Unexpected error")
			}
		})
	}
}

func TestBasicDecode(t *testing.T) {
	// Load the QOA audio file
	qoaBytes, err := os.ReadFile("testdata/sting_loss_piano.qoa")
	if err != nil {
		log.Fatalf("Error reading QOA file: %v", err)
	}
	// Decode the QOA audio data
	q := &QOA{}
	q, _, err = Decode(qoaBytes)

	assert.Nil(t, err, "Unexpected error")
	assert.NotEmpty(t, q.Samples, "Expected samples")
	assert.NotEmpty(t, q.Channels, "Expected channels")
	assert.NotEmpty(t, q.SampleRate, "Expected sample rate")
	assert.NotEmpty(t, q.lms[0], "Expected LMS data")
}

func TestBasicEncode(t *testing.T) {
	wavFile, err := os.Open("testdata/sting_loss_piano.wav")
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

	samples := uint32(len(wavBuffer.Data) / wavBuffer.Format.NumChannels)
	q := NewEncoder(
		uint32(wavBuffer.Format.SampleRate),
		uint32(wavBuffer.Format.NumChannels),
		samples)

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
func FuzzEncodeDecode(f *testing.F) {
	// Values to fuzz with, taken from the QOA spec
	MIN_CHANNELS := 1
	MAX_CHANNELS := 255
	MIN_SAMPLE_RATE := 1
	MAX_SAMPLE_RATE := 16777215
	// 1 channel, minimum slices (assuming at least 1 slice is required)
	MIN_SIZE := 8 + (8 + (16 * 1) + (256 * 8 * 1))
	// 1 channel, size to just exceed one frame, requiring part of a second frame
	SIZE_MULTIPLE_FRAMES := 8 + 2*(8+(16*1)+(256*8*1))
	SIZE_MAX_CHANNELS := 8 + (8 + (16 * 255) + (256 * 8 * 255))

	f.Add(generateFuzzData(MIN_SIZE, 0x7FFF), uint32(MIN_CHANNELS), uint32(MIN_SAMPLE_RATE))
	f.Add(generateFuzzData(SIZE_MULTIPLE_FRAMES, -0x8000), uint32(MIN_CHANNELS), uint32(MIN_SAMPLE_RATE))
	f.Add(generateFuzzData(SIZE_MAX_CHANNELS, 0x0000), uint32(MIN_CHANNELS), uint32(MIN_SAMPLE_RATE))

	f.Add(generateFuzzData(MIN_SIZE, 0x0000), uint32(MAX_CHANNELS), uint32(MAX_SAMPLE_RATE))
	f.Add(generateFuzzData(SIZE_MULTIPLE_FRAMES, -0x8000), uint32(MAX_CHANNELS), uint32(MAX_SAMPLE_RATE))
	f.Add(generateFuzzData(SIZE_MAX_CHANNELS, 0x0000), uint32(MAX_CHANNELS), uint32(MIN_SAMPLE_RATE))

	f.Fuzz(func(t *testing.T, data []byte, channels uint32, sampleRate uint32) {
		if len(data)%2 != 0 {
			// Ensure data length is even, as we're converting to int16
			return
		}

		// Convert []byte to []int16 for testing
		var originalSamples []int16
		for i := 0; i < len(data); i += 2 {
			sample := int16(binary.BigEndian.Uint16(data[i : i+2]))
			originalSamples = append(originalSamples, sample)
		}

		// Setup QOA struct with random but valid data
		q := NewEncoder(sampleRate, channels, uint32(len(originalSamples))/channels)

		// Encode the sample data
		encodedBytes, err := q.Encode(originalSamples)
		if err != nil {
			t.Logf("Failed to encode: %v", err)
		}

		// Decode the encoded bytes
		decodedQOA, _, err := Decode(encodedBytes)
		if err != nil {
			t.Logf("Failed to decode: %v", err)
		} else {

			psnr := -20.0 * math.Log10(math.Sqrt(float64(q.ErrorCount)/float64(q.Samples*q.Channels))/32768.0)

			// Check if decoded data is reasonable
			// Is there a better way to check lossy-compressed roundtrip bytes to original?
			assert.Greater(t, psnr, 30.0, "PSNR of decoded QOA bytes is bad")

			// Additional checks can be added here, such as verifying other fields in the QOA struct
			if decodedQOA.Channels != channels || decodedQOA.SampleRate != sampleRate || decodedQOA.Samples != uint32(len(originalSamples))/channels {
				t.Errorf("Decoded QOA struct fields do not match original")
			}
		}
	})
}

func generateFuzzData(size int, seedValue int16) []byte {
	data := make([]byte, size)
	for i := 0; i < size; i += 2 {
		binary.BigEndian.PutUint16(data[i:i+2], uint16(seedValue))
	}
	return data
}
