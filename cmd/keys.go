package cmd

import "github.com/charmbracelet/bubbles/key"

type helpKeyMap struct {
	togglePlay   key.Binding
	quit         key.Binding
	seek         key.Binding
	seekBack     key.Binding
	seekForward  key.Binding
	selectSong   key.Binding
	previousSong key.Binding
	nextSong     key.Binding
	pickSong     key.Binding
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
	selectSong: key.NewBinding(
		key.WithKeys("up", "k", "down", "j"),
		key.WithHelp("up/k/down/j", "choose song"),
	),
	previousSong: key.NewBinding(
		key.WithKeys("up", "k"),
	),
	nextSong: key.NewBinding(
		key.WithKeys("down", "j"),
	),
	pickSong: key.NewBinding(
		key.WithKeys("enter", "j"),
		key.WithHelp("enter", "pick song"),
	),
}

func (k helpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.togglePlay, k.seek, k.quit}
}
func (k helpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.togglePlay, k.seek},
		{k.selectSong, k.pickSong},
		{k.quit},
	}
}
