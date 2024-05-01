package cmd

import "github.com/charmbracelet/bubbles/key"

type helpKeyMap struct {
	togglePlay  key.Binding
	quit        key.Binding
	seek        key.Binding
	seekBack    key.Binding
	seekForward key.Binding
}

var helpKeys = helpKeyMap{
	togglePlay: key.NewBinding(
		key.WithKeys(" ", "p"),
		key.WithHelp("space/p", "play/pause"),
	),
	quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q/esc", "quit"),
	),
	seek: key.NewBinding(
		key.WithKeys("left", "h", "right", "l"),
		key.WithHelp("left/h/right/l", "seek"),
	),
	seekBack: key.NewBinding(
		key.WithKeys("left", "h"),
	),
	seekForward: key.NewBinding(
		key.WithKeys("right", "l"),
	),
}

func (k helpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.togglePlay, k.seek, k.quit}
}
func (k helpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.togglePlay, k.seek}, // first column
		{k.quit},               // second column
	}
}
