package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ebitengine/oto/v3"
)

type minimalModel struct {
	player    *qoaPlayer
	lastTick  time.Time
	isPlaying bool
}

func startMinimalPlayer(filename string) {
	fmt.Println("Starting minimal player mode...")

	// Set up clean exit handling
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Print("\nUser interrupted, exiting...\n")
		os.Exit(0)
	}()

	p := tea.NewProgram(
		initialMinimalModel(filename),
		tea.WithoutRenderer(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running player: %v\n", err)
	}
}

func initialMinimalModel(filename string) minimalModel {
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   44100,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
	})
	if err != nil {
		fmt.Printf("Error creating audio context: %v\n", err)
		return minimalModel{}
	}
	<-ready

	qp := newQOAPlayer(filename, ctx)

	fmt.Printf("\nPlaying: %s\n", filename)
	fmt.Printf("Sample Rate: %d Hz, Channels: %d, Bitrate: %d kbps\n",
		qp.qoaMetadata.SampleRate,
		qp.qoaMetadata.Channels,
		qp.bitrate,
	)
	fmt.Printf("Duration: %s\n", formatDuration(qp.totalLength))
	fmt.Println("\nPress Ctrl+C to quit")

	qp.player.Play()

	return minimalModel{
		player:    qp,
		lastTick:  time.Now(),
		isPlaying: true,
	}
}

func (m minimalModel) Init() tea.Cmd {
	return tickCmd()
}

func (m minimalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tickMsg:
		if time.Since(m.lastTick) >= time.Second {
			progress := m.player.getPlayerProgress()
			currentDuration := time.Duration(m.player.currentSeconds * float64(time.Second))
			fmt.Printf("\rTime: %s / %s",
				formatDuration(currentDuration),
				formatDuration(m.player.totalLength),
			)
			m.lastTick = time.Now()

			if progress >= 1.0 {
				fmt.Println("\nPlayback complete")
				return m, tea.Quit
			}
		}
		return m, tickCmd()
	}

	return m, nil
}

func (m minimalModel) View() string {
	return ""
}
