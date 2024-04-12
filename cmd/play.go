package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/braheezy/goqoa/pkg/qoa"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
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

type tickMsg time.Time

type playerMsg int

const (
	start playerMsg = iota
	stop
)

type model struct {
	filename    string
	player      *oto.Player
	progress    progress.Model
	ctx         *oto.Context
	qoaData     []int16
	qoaMetadata qoa.QOA
	startTime   time.Time
	totalLength time.Duration
}

func (m model) Init() tea.Cmd {
	return tea.Batch(func() tea.Msg {
		return playerMsg(start) // Initial command to start playback
	})
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var teaCommands []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.player.IsPlaying() {
				m.player.Close() // Ensure player resources are freed
			}
			return m, tea.Quit
		case "p": // Adding a pause/play toggle
			if m.player.IsPlaying() {
				teaCommands = append(teaCommands, func() tea.Msg {
					return playerMsg(stop) // Initial command to stop playback
				})
			} else if m.player != nil {
				teaCommands = append(teaCommands, func() tea.Msg {
					return playerMsg(start) // Initial command to start playback
				})
			}
		}
	case playerMsg:
		switch msg {
		case start:
			if !m.player.IsPlaying() {
				m.player.Play()
				m.startTime = time.Now()
			}
			teaCommands = append(teaCommands, tickCmd())
		case stop:
			if m.player.IsPlaying() {
				m.player.Pause()
			}
		}

	case tickMsg:
		if m.progress.Percent() >= 1.0 {
			return m, tea.Quit
		}
		if m.player.IsPlaying() {
			elapsed := time.Since(m.startTime)
			newPercent := elapsed.Seconds() / m.totalLength.Seconds()
			cmd := m.progress.SetPercent(newPercent) // Ensure this sets the progress correctly
			// if newPercent >= 1.0 {
			// 	return m, tea.Quit
			// }
			teaCommands = append(teaCommands, tickCmd(), cmd) // Continuously re-trigger tickCmd
		}

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		teaCommands = append(teaCommands, cmd)

	}
	return m, tea.Batch(teaCommands...)
}

func (m model) View() string {
	pad := strings.Repeat(" ", 2)
	statusLine := "Press 'p' to pause/play, 'q' to quit."
	return fmt.Sprintf("\nPlaying: %s\n\n%s%s\n\n%s%s\n", m.filename, pad, m.progress.View(), pad, statusLine)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func startTUI(filename string, ctx *oto.Context, qoaData []int16, qoaMetadata qoa.QOA) {
	prog := progress.New(progress.WithDefaultGradient())
	prog.ShowPercentage = false
	prog.SetSpringOptions(36.0, 1.0)

	player := ctx.NewPlayer(NewQOAAudioReader(qoaData))
	p := tea.NewProgram(model{
		filename:    filename,
		qoaData:     qoaData,
		qoaMetadata: qoaMetadata,
		progress:    prog,
		player:      player,
		totalLength: time.Duration(qoaMetadata.Samples/qoaMetadata.SampleRate) * time.Second,
	})
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
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

		// Play the audio
		logger.Debug(
			"Starting audio",
			"File",
			inputFile,
			"SampleRate",
			qoaMetadata.SampleRate,
			"ChannelCount",
			qoaMetadata.Channels)

		startTUI(inputFile, ctx, qoaAudioData, *qoaMetadata)
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
