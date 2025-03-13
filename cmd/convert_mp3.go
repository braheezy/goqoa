package cmd

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"os"

	"github.com/braheezy/qoa"
	mp3encoder "github.com/braheezy/shine-mp3/pkg/mp3"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
)

func decodeMp3(inputData *[]byte, filename string) ([]int16, *qoa.QOA) {
	logger.Info("Input format is MP3")

	// Create a reader from the input data
	reader := bytes.NewReader(*inputData)

	// Decode the MP3 data using ebiten's mp3 decoder
	stream, err := mp3.DecodeWithoutResampling(reader)
	if err != nil {
		log.Fatalf("Error decoding MP3 data: %v", err)
	}

	// Get audio properties
	sampleRate := stream.SampleRate()
	channels := 2 // MP3 typically has 2 channels

	// Read all audio data
	var audioData []byte
	buf := make([]byte, 1024)
	for {
		n, err := stream.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Error reading MP3 stream: %v", err)
		}
		audioData = append(audioData, buf[:n]...)
	}

	// Convert the MP3 audio data to int16 (QOA format)
	decodedData := make([]int16, len(audioData)/2)
	for i := 0; i < len(audioData)/2; i++ {
		sample := audioData[i*2 : (i+1)*2]
		decodedData[i] = int16(binary.LittleEndian.Uint16(sample))
	}

	// Set QOA metadata
	numSamples := len(decodedData) / channels
	q := qoa.NewEncoder(
		uint32(sampleRate),
		uint32(channels),
		uint32(numSamples),
	)

	logger.Debug(filename, "channels", channels, "samplerate(hz)", sampleRate, "samples/channel", numSamples, "size", formatSize(len(*inputData)))
	return decodedData, q
}

func encodeMp3(outputFile string, q *qoa.QOA, decodedData []int16) {
	logger.Info("Output format is MP3")

	mp3File, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("Error creating MP3 file: %v", err)
	}
	defer mp3File.Close()
	mp3Encoder := mp3encoder.NewEncoder(int(q.SampleRate), int(q.Channels))

	mp3Encoder.Write(mp3File, decodedData)
}
