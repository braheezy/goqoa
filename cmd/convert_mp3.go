//go:build !windows

package cmd

import (
	"encoding/binary"
	"fmt"
	"log"

	"github.com/braheezy/goqoa/pkg/mp3"
	"github.com/braheezy/goqoa/pkg/qoa"
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
	config := mp3.GlobalConfig{}
	config.SetDefaults()
	config.MPEG.Bitrate = 16
	config.Wave.SampleRate = int(q.SampleRate)
	config.Wave.Channels = int(q.Channels)
	if q.Channels > 1 {
		config.MPEG.Mode = mp3.STEREO
	} else {
		config.MPEG.Mode = mp3.MONO
	}

	encoder, err := config.NewEncoder()
	if err != nil {
		log.Fatalf("Error creating MP3 encoder: %v", err)
	}

	encoder.Write(outputFile, decodedData)
	// mp3File, err := os.Create(outputFile)
	// if err != nil {
	// 	log.Fatalf("Error creating MP3 file: %v", err)
	// }
	// defer mp3File.Close()

	// mp3Encoder := lame.NewEncoder(mp3File)
	// defer mp3Encoder.Close()

	// mp3Encoder.SetNumChannels(int(q.Channels))
	// mp3Encoder.SetInSamplerate(int(q.SampleRate))

	// // Convert the PCM data to a []byte
	// pcmBytes := make([]byte, len(decodedData)*2) // Assuming 16-bit PCM (2 bytes per sample)
	// for i, val := range decodedData {
	// 	binary.LittleEndian.PutUint16(pcmBytes[i*2:], uint16(val))
	// }

	// // Encode and write the PCM data to the MP3 file
	// _, err = mp3Encoder.Write(pcmBytes)
	// if err != nil {
	// 	log.Fatalf("Error encoding audio data to MP3: %v", err)
	// }
}
