// bitstream.go manages writes to the bitstream
package mp3

import (
	"encoding/binary"
	"fmt"
)

type bitstream struct {
	// Processed data
	data []byte
	// Total data size
	dataSize int
	// Current position of data
	dataPosition int
	// Cache for bitstream
	cache uint32
	// Free bits in cache
	cacheBits uint32
}

const (
	// Minimum size of the buffer in bytes
	MINIMUM = 4
	// Maximum length of word written or read from bit stream
	MAX_LENGTH  = 32
	BUFFER_SIZE = 4096
)

// openBitstream opens the device to write the bit stream into it
func (bs *bitstream) openBitstream(dataSize int) {
	bs.data = make([]byte, dataSize)
	bs.dataSize = dataSize
	bs.dataPosition = 0
	bs.cache = 0
	bs.cacheBits = 32
}

// putBits writes N bits of val into the bit stream.
func (bs *bitstream) putBits(val uint32, N uint32) {
	if N > MAX_LENGTH {
		fmt.Printf("Cannot write more than %v bits at one time.\n", MAX_LENGTH)
	}
	if N < MAX_LENGTH && val>>N != 0 {
		fmt.Printf("Upper bits (higher than %d) are not all zeros.\n", N)
	}

	if bs.cacheBits > N {
		bs.cacheBits -= N
		bs.cache |= val << bs.cacheBits
	} else {
		if bs.dataPosition+4 >= bs.dataSize {
			// Resize the data slice
			newCapacity := bs.dataSize + (bs.dataSize >> 1)
			newSlice := make([]byte, newCapacity)
			copy(newSlice, bs.data)
			bs.data = newSlice
			bs.dataSize = newCapacity
		}

		N -= bs.cacheBits
		bs.cache |= val >> N
		// Write the cache to the data slice
		binary.BigEndian.PutUint32(bs.data[bs.dataPosition:], bs.cache)
		bs.dataPosition += 4
		bs.cacheBits = 32 - N
		if N != 0 {
			bs.cache = val << bs.cacheBits
		} else {
			bs.cache = 0
		}
	}
}

func (bs *bitstream) getBitsCount() int {
	return bs.dataPosition*8 + 32 - int(bs.cacheBits)
}
