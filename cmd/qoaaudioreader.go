package cmd

import "io"

// NewQOAAudioReader creates a new QOAAudioReader instance.
func NewQOAAudioReader(data []int16) *QOAAudioReader {
	return &QOAAudioReader{
		data: data,
		pos:  0,
	}
}

// QOAAudioReader is a custom io.Reader that reads from QOA audio data.
type QOAAudioReader struct {
	data []int16
	pos  int
}

func (r *QOAAudioReader) Read(p []byte) (n int, err error) {
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

func (r *QOAAudioReader) SamplesPlayed() int {
	return r.pos
}