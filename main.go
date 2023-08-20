package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ebitengine/oto/v3"
)

// QOAAudioReader is a custom io.Reader that reads from QOA audio data.
type QOAAudioReader struct {
	data []int16
	pos  int
}

func (r *QOAAudioReader) Read(p []byte) (n int, err error) {
	samplesToRead := len(p) / 2
	if samplesToRead > len(r.data)-r.pos {
		samplesToRead = len(r.data) - r.pos
	}

	for i := 0; i < samplesToRead; i++ {
		sample := r.data[r.pos]
		p[i*2] = byte(sample & 0xFF)
		p[i*2+1] = byte(sample >> 8)
		r.pos++
	}

	return samplesToRead * 2, nil
}

func main() { // Load the QOA audio file
	fileName := "sting_loss_piano.qoa"
	qoaBytes, err := os.ReadFile(fmt.Sprintf("test/%s", fileName))
	if err != nil {
		log.Fatalf("Error reading QOA file: %v", err)
	}

	// Decode the QOA audio data
	q := QOA{}
	qoaAudioData, err := q.Decode(qoaBytes)
	if err != nil {
		log.Fatalf("Error decoding QOA data: %v", err)
	}

	// Prepare an Oto context (this will use your default audio device)
	ctx, ready, err := oto.NewContext(
		&oto.NewContextOptions{
			SampleRate:   44100,
			ChannelCount: 2,
			Format:       oto.FormatSignedInt16LE,
		})
	if err != nil {
		panic("oto.NewContext failed: " + err.Error())
	}

	// Wait for the context to be ready
	<-ready

	// Create a new player with the custom QOAAudioReader
	player := ctx.NewPlayer(NewQOAAudioReader(qoaAudioData))

	fmt.Printf("Playing song: %v...\n", fileName)

	// Play the audio
	player.Play()

	// Wait for the playback to finish
	time.Sleep(time.Second * 5) // Adjust as needed

	// Close the player and context
	if err := player.Close(); err != nil {
		fmt.Println("Error closing player:", err)
	}
}

// NewQOAAudioReader creates a new QOAAudioReader instance.
func NewQOAAudioReader(data []int16) *QOAAudioReader {
	return &QOAAudioReader{
		data: data,
		pos:  0,
	}
}
