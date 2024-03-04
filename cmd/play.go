package cmd

import (
	"io"
	"os"
	"time"

	"github.com/braheezy/goqoa/pkg/qoa"
	"github.com/ebitengine/oto/v3"
	"github.com/spf13/cobra"
)

var playCmd = &cobra.Command{
	Use:   "play <input-file>",
	Short: "Play .qoa audio file(s)",
	Long:  "Provide one or more QOA files to play.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		playQOA(args[0:])
	},
}

func init() {
	rootCmd.AddCommand(playCmd)
}

func playQOA(inputFiles []string) {

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

	for _, inputFile := range inputFiles {
		qoaBytes, err := os.ReadFile(inputFile)
		if err != nil {
			logger.Fatalf("Error reading QOA file: %v", err)
		}

		// Decode the QOA audio data
		_, qoaAudioData, err := qoa.Decode(qoaBytes)
		if err != nil {
			logger.Fatalf("Error decoding QOA data: %v", err)
		}

		// Wait for the context to be ready
		<-ready

		// Create a new player with the custom QOAAudioReader
		player := ctx.NewPlayer(NewQOAAudioReader(qoaAudioData))

		// Play the audio
		logger.Debug(
			"Starting audio...",
			"File",
			inputFile,
			"SampleRate",
			"44100",
			"ChannelCount",
			"2",
			"BufferedSize",
			player.BufferedSize())
		player.Play()

		for player.IsPlaying() {
			time.Sleep(time.Millisecond)
		}

		// Close the player
		if err := player.Close(); err != nil {
			logger.Fatalf("Error closing player: %v", err)
		}
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
