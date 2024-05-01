package qoa

import (
	"errors"
	"io"
)

// Reader is a custom io.Reader that reads from QOA audio data.
type Reader struct {
	data []int16
	pos  int
}

var ErrInvalidArgument = errors.New("invalid argument")

// NewReader creates a new Reader instance.
func NewReader(data []int16) *Reader {
	return &Reader{
		data: data,
		pos:  0,
	}
}

// Read implements the io.Reader interface
func (r *Reader) Read(p []byte) (n int, err error) {
	samplesToRead := len(p) / 2

	if r.pos >= len(r.data) {
		// Return EOF when there is no more data to read
		return 0, io.EOF
	}

	if samplesToRead > len(r.data)-r.pos {
		samplesToRead = len(r.data) - r.pos
	}

	for i := 0; i < samplesToRead; i++ {
		sample := r.data[r.pos]
		p[i*2] = byte(sample & 0xFF)
		p[i*2+1] = byte(sample >> 8)
		r.pos++
	}

	return samplesToRead * 2, nil
}

// SamplesPlayed returns the number of samples that have been read
func (r *Reader) SamplesPlayed() int {
	return r.pos
}
