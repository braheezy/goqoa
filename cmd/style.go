package cmd

import "github.com/charmbracelet/lipgloss"

const (
	padding  = 4
	maxWidth = 60
	qoaRed   = "#7b2165"
	qoaPink  = "#dd81c7"
	black    = "#191724"
)

var (
	statusStyle = lipgloss.NewStyle().
		Italic(true).
		Padding(1, 1).
		Foreground(lipgloss.Color(qoaPink))
)
