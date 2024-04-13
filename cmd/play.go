package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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
		var allFiles []string
		for _, arg := range args {
			info, err := os.Stat(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error accessing %s: %v\n", arg, err)
				continue
			}
			if info.IsDir() {
				files, err := findAllQOAFiles(arg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error walking %s: %v\n", arg, err)
					continue
				}
				allFiles = append(allFiles, files...)
			} else {
				valid, err := isValidQOAFile(arg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error checking file %s: %v\n", arg, err)
					continue
				}
				if valid {
					allFiles = append(allFiles, arg)
				}
			}
		}
		startTUI(allFiles)
	},
}

// Recursive function to find all valid QOA files
func findAllQOAFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			valid, err := isValidQOAFile(path)
			if err != nil {
				return err
			}
			if valid {
				files = append(files, path)
			}
		}
		return nil
	})
	return files, err
}
func init() {
	rootCmd.AddCommand(playCmd)
}

type tickMsg time.Time

type playerMsg int

func sendPlayerMsg(msg playerMsg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

type changeSong int

const (
	next changeSong = iota
	prev
)

func sendChangeSongMsg(msg changeSong) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

const (
	start playerMsg = iota
	stop
)

type model struct {
	filenames    []string
	currentIndex int
	qoaPlayer    *qoaPlayer
	ctx          *oto.Context
}

type qoaPlayer struct {
	qoaData         []int16
	player          *oto.Player
	qoaMetadata     qoa.QOA
	startTime       time.Time
	lastPauseTime   time.Time     // Tracks when the last pause started
	totalPausedTime time.Duration // Accumulates total time spent paused
	totalLength     time.Duration
	filename        string
	progress        progress.Model
	paused          bool
}

func newModel(filenames []string) *model {
	_, err := isValidQOAFile(filenames[0])
	if err != nil {
		logger.Fatalf("Error validating QOA file: %v", err)
	}

	qoaBytes, err := os.ReadFile(filenames[0])
	if err != nil {
		logger.Fatalf("Error reading QOA file: %v", err)
	}

	// Decode the QOA audio data
	qoaMetadata, _, err := qoa.Decode(qoaBytes)
	if err != nil {
		logger.Fatalf("Error decoding QOA data: %v", err)
	}

	// Prepare an Oto context (this will use the default audio device)
	ctx, ready, err := oto.NewContext(
		&oto.NewContextOptions{
			SampleRate: int(qoaMetadata.SampleRate),
			// only 1 or 2 are supported by oto
			ChannelCount: 2,
			Format:       oto.FormatSignedInt16LE,
		})
	if err != nil {
		panic("oto.NewContext failed: " + err.Error())
	}
	// Wait for the context to be ready
	<-ready

	m := &model{
		filenames:    filenames,
		currentIndex: 0,
		ctx:          ctx,
	}
	m.qoaPlayer = m.newQOAPlayer(filenames[0])
	return m
}

func (m *model) newQOAPlayer(filename string) *qoaPlayer {
	_, err := isValidQOAFile(filename)
	if err != nil {
		logger.Fatalf("Error validating QOA file: %v", err)
	}

	qoaBytes, err := os.ReadFile(filename)
	if err != nil {
		logger.Fatalf("Error reading QOA file: %v", err)
	}

	// Decode the QOA audio data
	qoaMetadata, qoaAudioData, err := qoa.Decode(qoaBytes)
	if err != nil {
		logger.Fatalf("Error decoding QOA data: %v", err)
	}

	prog := progress.New(progress.WithDefaultGradient())
	prog.ShowPercentage = false

	player := m.ctx.NewPlayer(NewQOAAudioReader(qoaAudioData))
	return &qoaPlayer{
		filename:    filename,
		qoaData:     qoaAudioData,
		qoaMetadata: *qoaMetadata,
		progress:    prog,
		player:      player,
		totalLength: time.Duration(qoaMetadata.Samples/qoaMetadata.SampleRate) * time.Second,
	}
}

func startTUI(inputFiles []string) {
	// If inputFiles[0] is a directory, get the immediate contents. Only files ending in .qoa.
	fileInfo, err := os.Stat(inputFiles[0])
	if err != nil {
		logger.Fatalf("Error reading file: %v", err)
	}
	if fileInfo.IsDir() {
		files, err := os.ReadDir(inputFiles[0])
		if err != nil {
			logger.Fatalf("Error reading directory: %v", err)
		}
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".qoa") {
				inputFiles = append(inputFiles, file.Name())
			}
		}
		if len(inputFiles) == 0 {
			logger.Fatal("No .qoa files found in directory")
		}
		// Remove the first element, the directory name
		inputFiles = inputFiles[1:]
	}
	p := tea.NewProgram(newModel(inputFiles))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
func (m model) Init() tea.Cmd {
	return tea.Batch(sendPlayerMsg(start))
}

// Start playback and initialize timing
func (qp *qoaPlayer) StartPlayback() {
	qp.player.Play()
	if qp.startTime.IsZero() {
		qp.startTime = time.Now()
	} else {
		qp.totalPausedTime += time.Since(qp.lastPauseTime)
		qp.lastPauseTime = time.Time{} // Reset last pause time
	}
}

// Pause playback and track pause timing
func (qp *qoaPlayer) PausePlayback() {
	qp.player.Pause()
	qp.lastPauseTime = time.Now()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var teaCommands []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.qoaPlayer.player.IsPlaying() {
				m.qoaPlayer.player.Close() // Ensure player resources are freed
			}
			return m, tea.Quit
		case "p": // Adding a pause/play toggle
			if m.qoaPlayer.player.IsPlaying() {
				teaCommands = append(teaCommands, sendPlayerMsg(stop))
			} else if m.qoaPlayer.player != nil {
				teaCommands = append(teaCommands, sendPlayerMsg(start))
			}
		}
	case playerMsg:
		switch msg {
		case start:
			if !m.qoaPlayer.player.IsPlaying() {
				m.qoaPlayer.player.Play()
				m.qoaPlayer.paused = false
				if m.qoaPlayer.startTime.IsZero() {
					m.qoaPlayer.startTime = time.Now()
				} else {
					m.qoaPlayer.totalPausedTime += time.Since(m.qoaPlayer.lastPauseTime)
					m.qoaPlayer.lastPauseTime = time.Time{} // Reset last pause time
				}
				teaCommands = append(teaCommands, tickCmd())
			}
		case stop:
			m.qoaPlayer.player.Pause()
			m.qoaPlayer.lastPauseTime = time.Now()
			m.qoaPlayer.paused = true

		}
	case changeSong:
		switch msg {
		case next:
			m = nextSong(m)
			teaCommands = append(teaCommands, sendPlayerMsg(start))
		}
	case tickMsg:
		if !m.qoaPlayer.player.IsPlaying() && !m.qoaPlayer.paused {
			teaCommands = append(teaCommands, sendChangeSongMsg(next))
			return m, tea.Batch(teaCommands...)
		}
		if m.qoaPlayer.player.IsPlaying() {
			elapsed := time.Since(m.qoaPlayer.startTime) - m.qoaPlayer.totalPausedTime
			newPercent := elapsed.Seconds() / m.qoaPlayer.totalLength.Seconds()
			cmd := m.qoaPlayer.progress.SetPercent(newPercent)
			teaCommands = append(teaCommands, cmd, tickCmd())
			return m, tea.Batch(teaCommands...)

		} else if m.qoaPlayer.progress.Percent() >= 1.0 {

			teaCommands = append(teaCommands, sendChangeSongMsg(next))
			return m, tea.Batch(teaCommands...)
		}

	case progress.FrameMsg:
		progressModel, cmd := m.qoaPlayer.progress.Update(msg)
		m.qoaPlayer.progress = progressModel.(progress.Model)
		teaCommands = append(teaCommands, cmd)

	}
	return m, tea.Batch(teaCommands...)
}

func nextSong(m model) model {
	m.qoaPlayer.player.Close()

	// Select next song in filenames list, but wrap around to 0 if at end
	nextIndex := (m.currentIndex + 1) % len(m.filenames)
	nextFile := m.filenames[nextIndex]

	// Create a new QOA player for the next song
	m.qoaPlayer = m.newQOAPlayer(nextFile)
	m.currentIndex = nextIndex

	// Return the new QOA player and a command to update the progress bar
	return m
}

func (m model) View() string {
	pad := strings.Repeat(" ", 2)
	statusLine := "Press 'p' to pause/play, 'q' to quit."
	return fmt.Sprintf("\nPlaying: %s (index: %v)\n\n%s%s\n\n%s%s\n", m.qoaPlayer.filename, m.currentIndex, pad, m.qoaPlayer.progress.View(), pad, statusLine)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
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
