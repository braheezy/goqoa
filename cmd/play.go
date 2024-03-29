package cmd

import (
	"fmt"
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

func isValidQOAFile(inputFile string) (bool, error) {
	// Read first 4 bytes of the file
	fileBytes := make([]byte, 4)
	file, err := os.Open(inputFile)
	if err != nil {
		return false, err
	}
	defer file.Close()

	_, err = file.Read(fileBytes)
	if err != nil && err != io.EOF {
		return false, err
	}

	// Check if the first 4 bytes are magic word `qoaf`
	if string(fileBytes) != "qoaf" {
		return false, fmt.Errorf("no magic word 'qoaf' found in %s", inputFile)
	}
	return true, nil
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
		_, err := isValidQOAFile(inputFile)
		if err != nil {
			logger.Fatalf("Error validating QOA file: %v", err)
		}

		qoaBytes, err := os.ReadFile(inputFile)
		if err != nil {
			logger.Fatalf("Error reading QOA file: %v", err)
		}

		// Decode the QOA audio data
		qoaMetadata, qoaAudioData, err := qoa.Decode(qoaBytes)
		if err != nil {
			logger.Fatalf("Error decoding QOA data: %v", err)
		}

		// Wait for the context to be ready
		<-ready

		// Create a new player with the custom QOAAudioReader
		player := ctx.NewPlayer(NewQOAAudioReader(qoaAudioData))

		// Play the audio
		logger.Debug(
			"Starting audio",
			"File",
			inputFile,
			"SampleRate",
			qoaMetadata.SampleRate,
			"ChannelCount",
			qoaMetadata.Channels,
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

	if r.pos >= len(r.data) {
		// Return EOF when there is no more data to read
		return 0, io.EOF
	}

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
