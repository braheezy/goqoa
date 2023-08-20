package main

import (
	"fmt"
	"log"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

func main() {
	// Convert WAV to QOA
	wavToQOA()

	// Convert QOA back to WAV
	qoaToWAV()

	fmt.Println("Conversion completed.")
}

func wavToQOA() {
	// Open the WAV audio file
	wavFile, err := os.Open("sting_loss_piano.wav")
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
	if err != nil {
		log.Fatalf("Error encoding audio data to QOA: %v", err)
	}

	// Write the QOA encoded data to a QOA file
	qoaFile, err := os.Create("my_string_loss_piano.qoa")
	if err != nil {
		log.Fatalf("Error creating QOA file: %v", err)
	}
	defer qoaFile.Close()

	_, err = qoaFile.Write(qoaEncodedData)
	if err != nil {
		log.Fatalf("Error writing QOA encoded data: %v", err)
	}

	fmt.Println("WAV to QOA conversion completed.")
}

func qoaToWAV() {
	// Load the QOA audio file
	qoaBytes, err := os.ReadFile("sting_loss_piano.qoa")
	if err != nil {
		log.Fatalf("Error reading QOA file: %v", err)
	}

	// Decode the QOA audio data
	q := QOA{}
	decodedData, err := q.Decode(qoaBytes, len(qoaBytes))
	if err != nil {
		log.Fatalf("Error decoding QOA data: %v", err)
	}

	// writeWav(decodedData, &q, "my_sting_loss_piano.qoa.wav")

	// Convert int16 to float32 for WAV conversion
	intAudioData := make([]int, len(decodedData))
	for i, val := range decodedData {
		intAudioData[i] = int(val)
	}

	wavBuffer := &audio.IntBuffer{
		Data:           intAudioData,
		Format:         &audio.Format{SampleRate: int(q.SampleRate), NumChannels: int(q.Channels)},
		SourceBitDepth: 16,
	}
	// Write the WAV audio data to a WAV file
	out, err := os.Create("my_sting_loss_piano.qoa.wav")
	if err != nil {
		log.Fatalf("Error creating WAV file: %v", err)
	}
	defer out.Close()

	wavEncoder := wav.NewEncoder(
		out,
		int(q.SampleRate),
		16,
		int(q.Channels),
		1)
	defer wavEncoder.Close()
	if err = wavEncoder.Write(wavBuffer); err != nil {
		log.Fatalf("Error writing WAV data: %v", err)
	}

	fmt.Println("QOA to WAV conversion completed.")
}
