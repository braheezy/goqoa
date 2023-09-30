package cmd

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"

	"github.com/braheezy/goqoa/pkg/qoa"
	"github.com/braheezy/shine-mp3/pkg/mp3"
	"github.com/tosone/minimp3"
)

func decodeMp3(inputData *[]byte) ([]int16, *qoa.QOA) {
	fmt.Println("Input format is MP3")
	dec, mp3Data, err := minimp3.DecodeFull(*inputData)
	if err != nil {
		log.Fatalf("Error decoding MP3 data: %v", err)
	}

	// Convert the MP3 audio data to int16 (QOA format)
	decodedData := make([]int16, len(mp3Data)/2)
	for i := 0; i < len(mp3Data)/2; i++ {
		sample := mp3Data[i*2 : (i+1)*2]
		decodedData[i] = int16(binary.LittleEndian.Uint16(sample))
	}

	// Set QOA metadata
	numSamples := len(decodedData) / dec.Channels
	q := qoa.NewEncoder(
		uint32(dec.SampleRate),
		uint32(dec.Channels),
		uint32(numSamples),
	)
	return decodedData, q
}

func encodeMp3(outputFile string, q *qoa.QOA, decodedData []int16) {
	fmt.Println("Output format is MP3")

	mp3File, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("Error creating MP3 file: %v", err)
	}
	defer mp3File.Close()
	mp3Encoder := mp3.NewEncoder(int(q.SampleRate), int(q.Channels))

	mp3Encoder.Write(mp3File, decodedData)
}
