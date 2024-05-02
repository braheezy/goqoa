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

// Seek implements the io.Seeker interface
func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	var newPos int64

	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = int64(r.pos) + offset
	case io.SeekEnd:
		newPos = int64(len(r.data)) + offset
	default:
		return 0, ErrInvalidArgument
	}

	if newPos < 0 {
		// prevent seeking before the beginning
		return 0, ErrInvalidArgument
	}
	if newPos >= int64(len(r.data)) {
		// prevent seeking beyond the end, handle as per your need
		return 0, io.EOF
	}
	// set the new position
	r.pos = int(newPos)
	return newPos, nil
}

// Position returns the number of bytes that have been read
func (r *Reader) Position() int {
	return r.pos
}
