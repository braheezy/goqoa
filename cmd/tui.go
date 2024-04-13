package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/braheezy/goqoa/pkg/qoa"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ebitengine/oto/v3"
)

// ==========================================
// =============== Messages =================
// ==========================================
// tickMsg is sent periodically to update the progress bar.
type tickMsg time.Time

// tickCmd is a helper function to create a tickMsg.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// controlsMsg is sent to control various things about the music player.
type controlsMsg int

const (
	start controlsMsg = iota
	stop
)

// sendControlsMsg is a helper function to create a controlsMsg.
func sendControlsMsg(msg controlsMsg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

// changeSongMsg is sent to change the song.
type changeSongMsg int

const (
	next changeSongMsg = iota
	prev
)

// sendChangeSongMsg is a helper function to create a changeSongMsg.
func sendChangeSongMsg(msg changeSongMsg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

// ==========================================
// ================ Models ==================
// ==========================================

// model holds the main state of the application.
type model struct {
	// filenames is a list of filenames to play.
	filenames []string
	// currentIndex is the index of the current song playing
	currentIndex int
	// qoaPlayer is the QOA player
	qoaPlayer *qoaPlayer
	// ctx is the Oto context. There can only be one per process.
	ctx *oto.Context
}

// qoaPlayer handles playing QOA audio files and showing progress.
type qoaPlayer struct {
	// qoaData is the raw QOA encoded audio bytes.
	qoaData []int16
	// player is the Oto player, which does the actually playing of sound.
	player *oto.Player
	// qoaMetadata is the QOA encoder struct.
	qoaMetadata qoa.QOA
	// startTime is the time when the song started playing.
	startTime time.Time
	// lastPauseTime is the time when the last pause started.
	lastPauseTime time.Time
	// totalPausedTime is the total time spent paused.
	totalPausedTime time.Duration
	// totalLength is the total length of the song.
	totalLength time.Duration
	// filename is the filename of the song being played.
	filename string
	// progress is the progress bubble model.
	progress progress.Model
	// paused is whether the song is paused.
	paused bool
}

// initialModel creates a new model with the given filenames.
func initialModel(filenames []string) *model {
	// Prepare an Oto context (this will use the default audio device)
	ctx, ready, err := oto.NewContext(
		&oto.NewContextOptions{
			// Typically 44100 or 48000, we could get it from a QOA file but we'd have to decode one.
			SampleRate: 44100,
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

// newQOAPlayer creates a new QOA player for the given filename.
func (m *model) newQOAPlayer(filename string) *qoaPlayer {
	_, err := qoa.IsValidQOAFile(filename)
	if err != nil {
		logger.Fatalf("Error validating QOA file: %v", err)
	}

	qoaBytes, err := os.ReadFile(filename)
	if err != nil {
		logger.Fatalf("Error reading QOA file: %v", err)
	}

	qoaMetadata, qoaAudioData, err := qoa.Decode(qoaBytes)
	if err != nil {
		logger.Fatalf("Error decoding QOA data: %v", err)
	}

	totalLength := time.Duration(qoaMetadata.Samples/qoaMetadata.SampleRate) * time.Second

	prog := progress.New(progress.WithGradient(qoaRed, qoaPink))
	prog.ShowPercentage = false
	prog.Width = maxWidth

	player := m.ctx.NewPlayer(NewQOAAudioReader(qoaAudioData))
	return &qoaPlayer{
		filename:    filename,
		qoaData:     qoaAudioData,
		qoaMetadata: *qoaMetadata,
		progress:    prog,
		player:      player,
		totalLength: totalLength,
	}
}

// ==========================================
// ================= Main ===================
// ==========================================
// startTUI is the main entry point for the TUI.
func startTUI(inputFiles []string) {
	p := tea.NewProgram(initialModel(inputFiles))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(sendControlsMsg(start))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Handle terminal resizing
	case tea.WindowSizeMsg:
		m.qoaPlayer.progress.Width = msg.Width - padding*2 - 4
		if m.qoaPlayer.progress.Width > maxWidth {
			m.qoaPlayer.progress.Width = maxWidth
		}
		return m, nil

	// Handle key presses
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			if m.qoaPlayer.player.IsPlaying() {
				m.qoaPlayer.player.Close()
			}
			return m, tea.Quit
		case "p":
			// pause/play toggle
			var cmd tea.Cmd
			if m.qoaPlayer.player.IsPlaying() {
				cmd = sendControlsMsg(stop)
			} else if m.qoaPlayer.player != nil {
				cmd = sendControlsMsg(start)
			}
			return m, cmd
		}
	// Handle requests to change controls (play, pause, etc.)
	case controlsMsg:
		switch msg {
		case start:
			if !m.qoaPlayer.player.IsPlaying() {
				m.qoaPlayer.player.Play()
				m.qoaPlayer.paused = false

				// Account for time spent paused, if needed
				if m.qoaPlayer.startTime.IsZero() {
					m.qoaPlayer.startTime = time.Now()
				} else {
					m.qoaPlayer.totalPausedTime += time.Since(m.qoaPlayer.lastPauseTime)
					m.qoaPlayer.lastPauseTime = time.Time{} // Reset last pause time
				}
				// Now that we are definitely playing, start the progress bubble
				return m, tickCmd()
			}
		case stop:
			m.qoaPlayer.player.Pause()
			m.qoaPlayer.lastPauseTime = time.Now()
			m.qoaPlayer.paused = true
		}
	// Handle requests to change song (prev, next, etc.)
	case changeSongMsg:
		switch msg {
		case next:
			m = nextSong(m)
			return m, sendControlsMsg(start)
		}
	// Update the progress. This is called periodically, so also handle songs that are over.
	case tickMsg:
		// Check if the song is over, ignoring progress bubble status in case the song ended before it go to 100%.
		if !m.qoaPlayer.player.IsPlaying() && !m.qoaPlayer.paused {
			// Just go to the next song.
			return m, sendChangeSongMsg(next)
		}
		// If we're still playing, update accordingly
		if m.qoaPlayer.player.IsPlaying() {
			elapsed := time.Since(m.qoaPlayer.startTime) - m.qoaPlayer.totalPausedTime
			newPercent := elapsed.Seconds() / m.qoaPlayer.totalLength.Seconds()
			cmd := m.qoaPlayer.progress.SetPercent(newPercent)
			// Set new progress bar percent and keep ticking
			return m, tea.Batch(cmd, tickCmd())
		} else if m.qoaPlayer.progress.Percent() >= 1.0 {
			// Progress is at 100%, so song must be over.
			return m, tea.Batch(sendChangeSongMsg(next))
		}

	case progress.FrameMsg:
		progressModel, cmd := m.qoaPlayer.progress.Update(msg)
		m.qoaPlayer.progress = progressModel.(progress.Model)
		return m, cmd

	}
	return m, nil
}

// nextSong changes to the next song in the filenames list, wrapping around to 0 if needed.
func nextSong(m model) model {
	m.qoaPlayer.player.Close()

	// Select next song in filenames list, but wrap around to 0 if at end
	nextIndex := (m.currentIndex + 1) % len(m.filenames)
	nextFile := m.filenames[nextIndex]

	// Create a new QOA player for the next song
	m.qoaPlayer = m.newQOAPlayer(nextFile)
	m.currentIndex = nextIndex

	// Return the new QOA player
	return m
}

// ==========================================
// ================= View ===================
// ==========================================
// View renders the current state of the application.
func (m model) View() string {
	pad := strings.Repeat(" ", 2)
	statusLine := "Press 'p' to pause/play, 'q' to quit."
	return fmt.Sprintf("\nPlaying: %s (index: %v)\n\n%s%s\n\n%s%s\n", m.qoaPlayer.filename, m.currentIndex, pad, m.qoaPlayer.progress.View(), pad, statusLine)
}
