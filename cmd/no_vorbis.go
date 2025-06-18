//go:build windows || linux
// +build windows linux

package cmd

import (
	"errors"
	"io"
)

func encodeVorbisToOgg(w io.Writer, pcm []int16, sampleRate, channels int) error {
	return errors.New("not implemented")
}
