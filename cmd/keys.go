package cmd

import "github.com/charmbracelet/bubbles/key"

type helpKeyMap struct {
	togglePlay key.Binding
	quit       key.Binding
}

var helpsKeys = helpKeyMap{
	togglePlay: key.NewBinding(
		key.WithKeys(" ", "p"),
		key.WithHelp("space/p", "play/pause"),
	),
	quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q/esc", "quit"),
	),
}

func (k helpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.togglePlay, k.quit}
}
func (k helpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.togglePlay}, // first column
		{k.quit},       // second column
	}
}
