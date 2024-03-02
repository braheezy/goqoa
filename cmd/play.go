package cmd

import (
	"log"
	"os"
	"time"

	"github.com/braheezy/goqoa/pkg/qoa"
	"github.com/ebitengine/oto/v3"
	"github.com/spf13/cobra"
)

var playCmd = &cobra.Command{
	Use:   "play <input-file>",
	Short: "Play a .qoa audio file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputFile := args[0]
		playQOA(inputFile)
	},
}

func init() {
	rootCmd.AddCommand(playCmd)
}

func playQOA(inputFile string) {
	qoaBytes, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("Error reading QOA file: %v", err)
	}

	// Decode the QOA audio data
	_, qoaAudioData, err := qoa.Decode(qoaBytes)
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

	// Play the audio
	logger.Debug("Starting audio...", "SampleRate", "44100", "ChannelCount", "2")
	player.Play()

	// player.IsPlaying() is the recommended approach but it never returns false for us.
	// This method of checking the unplayed buffer size also works.
	for player.BufferedSize() != 0 {
		time.Sleep(time.Millisecond)
	}

	// Close the player
	if err := player.Close(); err != nil {
		logger.Fatalf("Error closing player: %v", err)
	}
}

// NewQOAAudioReader creates a new QOAAudioReader instance.
func NewQOAAudioReader(data []int16) *QOAAudioReader {
	return &QOAAudioReader{
		data: data,
		pos:  0,
	}
}

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
