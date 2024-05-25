package cmd

import "github.com/charmbracelet/lipgloss"

const (
	padding  = 4
	maxWidth = 60
	qoaRed   = "#7b2165"
	qoaPink  = "#dd81c7"
	black    = "#191724"

	greenLight = "#56949f"
	greenDark  = "#9ccfd8"
)

var (
	accent = lipgloss.AdaptiveColor{Dark: greenDark, Light: greenLight}
	main   = lipgloss.AdaptiveColor{Dark: qoaPink, Light: qoaRed}

	listStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Margin(1).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(accent)
	listTitleStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(main).
			Bold(true)
)
