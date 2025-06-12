//go:build windows
// +build windows

package cmd

import (
	"errors"
	"io"
)

func encodeVorbisToOgg(w io.Writer, pcm []int16, sampleRate, channels int) error {
	return errors.New("not implemented")
}
