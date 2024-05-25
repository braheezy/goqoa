package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/braheezy/goqoa/pkg/qoa"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// ==========================================
// ================ Models ==================
// ==========================================

// model holds the main state of the application.
type model struct {
	// filenames is a list of filenames to play.
	filenames []string
	fileList  list.Model
	// currentIndex is the index of the current song playing
	currentIndex int
	// qoaPlayer is the QOA player
	qoaPlayer *qoaPlayer
	// ctx is the Oto context. There can only be one per process.
	ctx *oto.Context
	// help is the help bubble model
	help help.Model
	// To support help
	keys helpKeyMap
	// progress is the progress bubble model.
	progress progress.Model
	// Current terminal size
	terminalWidth  int
	terminalHeight int
}

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

// qoaPlayer handles playing QOA audio files and showing progress.
type qoaPlayer struct {
	// qoaData is the raw QOA encoded audio bytes.
	qoaData []int16
	// player is the Oto player, which does the actually playing of sound.
	player *oto.Player
	// qoaMetadata is the QOA encoder struct.
	qoaMetadata qoa.QOA
	// totalLength is the total length of the song.
	totalLength time.Duration
	// filename is the filename of the song being played.
	filename string
	// reader is a pointer to the QOA reader, so we can track song position better.
	// oto doesn't support getting the player position while it's playing but might one day:
	// https://github.com/ebitengine/oto/issues/228
	reader         *qoa.Reader
	currentSeconds float64
	samplesPlayed  int
	bitrate        uint32
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
	// Create the help bubble
	help := help.New()
	help.ShowAll = true

	// Create the progress bubble
	prog := progress.New(progress.WithGradient(qoaRed, qoaPink))
	prog.ShowPercentage = false
	prog.Width = maxWidth

	items := make([]list.Item, len(filenames))
	for i, filename := range filenames {
		qoaBytes := openFile(filename)
		qoaMetadata, err := qoa.DecodeHeader(qoaBytes)
		if err != nil {
			logger.Fatalf("Error decoding QOA header: %v", err)
		}
		desc := formatDuration(calcSongLength(qoaMetadata))
		items[i] = item{title: filename, desc: desc}
	}
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Copy().Foreground(main)
	listModel := list.New(items, delegate, 0, 0)
	listModel.SetShowHelp(false)
	listModel.Title = "goqoa"
	listModel.SetStatusBarItemName("song", "songs")
	listModel.InfiniteScrolling = true
	listModel.Styles.Title = listTitleStyle

	// Wait for the audio context to be ready
	<-ready

	m := &model{
		filenames:    filenames,
		fileList:     listModel,
		currentIndex: 0,
		ctx:          ctx,
		help:         help,
		keys:         helpKeys,
		progress:     prog,
	}

	m.nextSong()

	return m
}

func openFile(filename string) []byte {
	_, err := qoa.IsValidQOAFile(filename)
	if err != nil {
		logger.Fatalf("Error validating QOA file: %v", err)
	}

	qoaBytes, err := os.ReadFile(filename)
	if err != nil {
		logger.Fatalf("Error reading QOA file: %v", err)
	}

	return qoaBytes
}

// newQOAPlayer creates a new QOA player for the given filename.
func newQOAPlayer(filename string, ctx *oto.Context) *qoaPlayer {
	qoaBytes := openFile(filename)
	qoaMetadata, qoaAudioData, err := qoa.Decode(qoaBytes)
	if err != nil {
		logger.Fatalf("Error decoding QOA data: %v", err)
	}

	// Calculate length of song in nanoseconds
	totalLength := calcSongLength(qoaMetadata)

	reader := qoa.NewReader(qoaAudioData)
	player := ctx.NewPlayer(reader)
	bitrate := (qoaMetadata.SampleRate * qoaMetadata.Channels * 16) / 1000

	return &qoaPlayer{
		filename:    filename,
		qoaData:     qoaAudioData,
		qoaMetadata: *qoaMetadata,
		player:      player,
		totalLength: totalLength,
		reader:      reader,
		bitrate:     bitrate,
	}
}

func calcSongLength(qoaMetadata *qoa.QOA) time.Duration {
	return time.Duration((int64(qoaMetadata.Samples) * int64(time.Second)) / int64(qoaMetadata.SampleRate))
}

// initialView returns the initial view of the application.

// togglePlayPause toggles the playing state of the player.
func (qp *qoaPlayer) togglePlayPause() tea.Cmd {
	if qp.player.IsPlaying() {
		qp.player.Pause()
		return nil
	} else {
		qp.player.Play()
		// Get the progress bar updating again.
		return tickCmd()
	}
}

// getPlayerProgress returns the current progress of the player in percent.
func (qp *qoaPlayer) getPlayerProgress() float64 {
	if qp.totalLength == 0 {
		return 0
	}

	// Calculate number of samples in buffer
	// Multiple by 2 for 16-bit samples
	bufferedSamples := float64(qp.player.BufferedSize()) / (float64(qp.qoaMetadata.Channels) * 2.0)

	// Calculate the actual samples played
	samplesPlayed := float64(qp.reader.Position())/2.0 - bufferedSamples

	// Calculate newPercent based on samples
	totalSamples := float64(qp.qoaMetadata.Samples)
	newPercent := samplesPlayed / totalSamples
	if samplesPlayed >= totalSamples {
		newPercent = 1.0
	}

	// Update currentSeconds for potential other uses, calculated from samplesPlayed
	qp.currentSeconds = time.Duration((int64(samplesPlayed) * int64(time.Second)) / int64(qp.qoaMetadata.SampleRate)).Seconds()
	qp.samplesPlayed = int(samplesPlayed)

	return newPercent
}

// seekForward moves the player forward by 5 seconds.
func (qp *qoaPlayer) seekForward() float64 {
	return qp.seekRelative(5 * time.Second)
}

// seekBack moves the player back by 7 seconds.
func (qp *qoaPlayer) seekBack() float64 {
	return qp.seekRelative(-7 * time.Second)
}

// seekRelative moves the player by the given delta and returns the new progress percent.
func (qp *qoaPlayer) seekRelative(delta time.Duration) float64 {
	sampleOffset := int64(delta.Seconds() * float64(qp.qoaMetadata.SampleRate))
	qp.player.Seek(sampleOffset, io.SeekCurrent)
	return qp.getPlayerProgress()
}

// ==========================================
// ================= Main ===================
// ==========================================
// startTUI is the main entry point for the TUI.
func startTUI(inputFiles []string) {
	p := tea.NewProgram(initialModel(inputFiles), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

// Init is the first function called by bubbletea
func (m model) Init() tea.Cmd {
	return tickCmd()
}

// Update is the bubbletea function for handling messages and updating the model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Handle terminal resizing
	case tea.WindowSizeMsg:
		m.progress.Width = msg.Width - padding*2 - 4
		if m.progress.Width > maxWidth {
			m.progress.Width = maxWidth
		}

		listHeight := len(m.fileList.Items()) * 5
		if msg.Height < listHeight {
			listHeight = msg.Height
		}
		m.fileList.SetSize(msg.Width/3, listHeight)
		return m, m.checkRepaint(msg)
	// Handle key presses
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.quit):
			if m.qoaPlayer.player.IsPlaying() {
				m.qoaPlayer.player.Close()
			}
			return m, tea.Quit
		case key.Matches(msg, m.keys.togglePlay):
			cmd := m.qoaPlayer.togglePlayPause()
			return m, cmd
		case key.Matches(msg, m.keys.seekForward):
			newPercent := m.qoaPlayer.seekForward()
			return m, m.progress.SetPercent(newPercent)
		case key.Matches(msg, m.keys.seekBack):
			newPercent := m.qoaPlayer.seekBack()
			return m, m.progress.SetPercent(newPercent)
		case key.Matches(msg, m.keys.pickSong):
			m.loadSong(m.fileList.Index())
		}
	// Update the progress. This is called periodically, so also handle songs that are over.
	case tickMsg:
		if m.progress.Percent() >= 1.0 {
			m.nextSong()
			cmd := m.progress.SetPercent(0.0)
			return m, tea.Batch(tickCmd(), cmd)
		} else {
			percentDone := m.qoaPlayer.getPlayerProgress()
			cmd := m.progress.SetPercent(percentDone)
			// Set new progress bar percent and keep ticking
			return m, tea.Batch(cmd, tickCmd())
		}
	// Update the progress bubble
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
	}

	var cmd tea.Cmd
	m.fileList, cmd = m.fileList.Update(msg)
	return m, cmd
}

func (m *model) loadSong(index int) {
	if m.qoaPlayer != nil {
		m.qoaPlayer.player.Close()
	}
	nextFile := m.filenames[index]

	// Create a new QOA player for the next song
	m.qoaPlayer = newQOAPlayer(nextFile, m.ctx)
	m.qoaPlayer.player.Play()
	m.fileList.Select(m.currentIndex)
	m.currentIndex = index
}

// nextSong changes to the next song in the filenames list, wrapping around to 0 if needed.
func (m *model) nextSong() {
	// Select next song in filenames list, but wrap around to 0 if at end
	nextIndex := (m.currentIndex + 1) % len(m.filenames)
	m.loadSong(nextIndex)
}

func (m *model) checkRepaint(msg tea.WindowSizeMsg) tea.Cmd {
	needsRepaint := false

	if msg.Width < m.terminalWidth {
		needsRepaint = true
	}

	// If we set a width on the help menu it can it can gracefully truncate
	// its view as needed.
	m.help.Width = msg.Width
	m.terminalWidth = msg.Width
	m.terminalHeight = msg.Height

	if needsRepaint {
		return tea.ClearScreen
	}
	return nil
}

// ==========================================
// ================= View ===================
// ==========================================
// View renders the current state of the application.
func (m model) View() string {
	var mainView strings.Builder

	mainView.WriteString(m.renderTitle())
	mainView.WriteRune('\n')

	// Render the stats
	mainView.WriteString(m.renderStats())
	mainView.WriteRune('\n')

	// Song progress
	mainView.WriteString(m.progress.View())
	mainView.WriteString(m.renderTime())
	mainView.WriteRune('\n')

	mainView.WriteRune('\n')
	mainView.WriteString(m.help.View(m.keys))
	mainView.WriteRune('\n')

	fileListView := listStyle.Render(m.fileList.View())

	view := lipgloss.JoinHorizontal(lipgloss.Top, fileListView, mainView.String())

	return lipgloss.PlaceHorizontal(m.terminalWidth, lipgloss.Center, view)
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d/time.Millisecond) // Fixed width for milliseconds
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", d/time.Second) // Fixed width for seconds
	}
	min := d / time.Minute
	sec := (d % time.Minute) / time.Second
	return fmt.Sprintf("%dm%ds", min, sec) // Fixed width for minutes and seconds
}
func (m model) renderTitle() string {
	playingStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(qoaPink))

	songStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accent)

	title := playingStyle.Render("Playing:") + " " + songStyle.Render(m.qoaPlayer.filename)
	view := lipgloss.NewStyle().Padding(1).Render(title)
	return view
}
func (m model) renderStats() string {
	statsStyle := lipgloss.NewStyle().
		Faint(true).
		PaddingBottom(1)

	stats := fmt.Sprintf("sample rate: %d Hz | channels: %d | bitrate: %d kbps",
		m.qoaPlayer.qoaMetadata.SampleRate,
		m.qoaPlayer.qoaMetadata.Channels,
		m.qoaPlayer.bitrate)

	return statsStyle.Render(stats)
}
func (m model) renderTime() string {

	// Convert seconds to time.Duration
	currentDuration := time.Duration(m.qoaPlayer.currentSeconds * float64(time.Second))
	totalDuration := time.Duration(m.qoaPlayer.totalLength.Seconds() * float64(time.Second))

	// Format durations using time.Duration's String method, customized for display
	currentTimeStr := formatDuration(currentDuration)
	totalTimeStr := formatDuration(totalDuration)

	// Calculate the widths of the time strings
	currentTimeWidth := lipgloss.Width(currentTimeStr)
	totalTimeWidth := lipgloss.Width(totalTimeStr)
	separatorWidth := 3 // Width of " / "
	totalWidth := currentTimeWidth + separatorWidth + totalTimeWidth

	// Set a fixed width for the display (e.g., 20 characters)
	fixedWidth := 12
	leftPadding := fixedWidth - totalWidth

	timeStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Foreground(accent).
		MarginLeft(leftPadding)

	// Ensure the entire string is fixed width
	timeProgress := fmt.Sprintf("%s / %s", currentTimeStr, totalTimeStr)
	return timeStyle.Render(timeProgress)
}
