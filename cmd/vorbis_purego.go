//go:build darwin
// +build darwin

package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/jonas747/ogg"
)

type vorbisInfo struct {
	version         int32
	channels        int32
	rate            int64
	bitrate_upper   int64
	bitrate_nominal int64
	bitrate_lower   int64
	bitrate_window  int64
	codec_setup     uintptr
}

type vorbisComment struct {
	user_comments   uintptr
	comment_lengths uintptr
	comments        int32
	vendor          uintptr
}

// C mirror of struct vorbis_dsp_state in vorbis/codec.h
type vorbisDspState struct {
	analysisp      int32
	_pad0          int32
	vi             *vorbisInfo
	pcm            uintptr
	pcmret         uintptr
	pcmStorage     int32
	pcmCurrent     int32
	pcmReturned    int32
	preextrapolate int32
	eofflag        int32
	_pad1          [4]byte
	lW             int64
	W              int64
	nW             int64
	centerW        int64
	granulepos     int64
	sequence       int64
	glueBits       int64
	timeBits       int64
	floorBits      int64
	resBits        int64
	backendState   uintptr
}

// C mirror of struct oggpack_buffer in ogg/ogg.h
type oggpackBuffer struct {
	endbyte int64
	endbit  int32
	_pad0   int32
	buffer  uintptr
	ptr     uintptr
	storage int64
}

// C mirror of struct vorbis_block in vorbis/codec.h
type vorbisBlock struct {
	pcm        uintptr
	opb        oggpackBuffer
	lW         int64
	W          int64
	nW         int64
	pcmend     int32
	mode       int32
	eofflag    int32
	_pad0      int32
	granulepos int64
	sequence   int64
	vd         *vorbisDspState
	localstore uintptr
	localtop   int64
	localalloc int64
	totaluse   int64
	reap       uintptr
	glueBits   int64
	timeBits   int64
	floorBits  int64
	resBits    int64
	internal   uintptr
}

type oggPacket struct {
	packet     uintptr
	bytes      int64
	b_o_s      int64
	e_o_s      int64
	granulepos int64
	packetno   int64
}

var (
	vorbisOnce               sync.Once
	vorbisLib                uintptr
	vorbisEncLib             uintptr
	vorbisInfoInit           func(*vorbisInfo)
	vorbisEncodeInitVBR      func(*vorbisInfo, int64, int64, float32) int32
	vorbisCommentInit        func(*vorbisComment)
	vorbisCommentAddTag      func(*vorbisComment, *byte, *byte) int32
	vorbisAnalysisInit       func(*vorbisDspState, *vorbisInfo) int32
	vorbisBlockInit          func(*vorbisDspState, *vorbisBlock) int32
	vorbisAnalysisHeaderout  func(*vorbisDspState, *vorbisComment, *oggPacket, *oggPacket, *oggPacket) int32
	vorbisAnalysisBuffer     func(*vorbisDspState, int) uintptr
	vorbisAnalysisWrote      func(*vorbisDspState, int) int32
	vorbisAnalysisBlockout   func(*vorbisDspState, *vorbisBlock) int32
	vorbisAnalysis           func(*vorbisBlock, *oggPacket) int32
	vorbisBitrateAddBlock    func(*vorbisBlock) int32
	vorbisBitrateFlushPacket func(*vorbisDspState, *oggPacket) int32
)

func initVorbis() {
	var names []string
	if runtime.GOOS == "darwin" {
		names = []string{"libvorbis.dylib", "libvorbis.0.dylib"}
		for _, prefix := range []string{"/usr/local", "/opt/homebrew"} {
			names = append(names,
				filepath.Join(prefix, "opt", "libvorbis", "lib", "libvorbis.dylib"),
				filepath.Join(prefix, "opt", "libvorbis", "lib", "libvorbis.0.dylib"),
			)
		}
		// Also probe Homebrew Cellar paths for any installed versions.
		for _, cellar := range []string{"/usr/local/Cellar/libvorbis", "/opt/homebrew/Cellar/libvorbis"} {
			for _, libName := range []string{"libvorbis.dylib", "libvorbis.0.dylib"} {
				if matches, err := filepath.Glob(filepath.Join(cellar, "*", "lib", libName)); err == nil {
					names = append(names, matches...)
				}
			}
		}
	} else {
		names = []string{"libvorbis.so.0", "libvorbis.so"}
	}
	var handle uintptr
	var err error
	for _, n := range names {
		handle, err = purego.Dlopen(n, purego.RTLD_LAZY)
		if err == nil {
			break
		}
	}
	if err != nil {
		panic(fmt.Errorf("loading libvorbis libraries %v: %v", names, err))
	}

	// Load the encoding library (libvorbisenc) which contains encoder symbols.
	var namesEnc []string
	if runtime.GOOS == "darwin" {
		namesEnc = []string{"libvorbisenc.dylib", "libvorbisenc.2.dylib"}
		for _, prefix := range []string{"/usr/local", "/opt/homebrew"} {
			namesEnc = append(namesEnc,
				filepath.Join(prefix, "opt", "libvorbis", "lib", "libvorbisenc.dylib"),
				filepath.Join(prefix, "opt", "libvorbis", "lib", "libvorbisenc.2.dylib"),
			)
		}
		for _, cellar := range []string{"/usr/local/Cellar/libvorbis", "/opt/homebrew/Cellar/libvorbis"} {
			for _, libName := range []string{"libvorbisenc.dylib", "libvorbisenc.2.dylib"} {
				if matches, err := filepath.Glob(filepath.Join(cellar, "*", "lib", libName)); err == nil {
					namesEnc = append(namesEnc, matches...)
				}
			}
		}
	} else {
		namesEnc = []string{"libvorbisenc.so.2", "libvorbisenc.so"}
	}
	var handleEnc uintptr
	for _, n := range namesEnc {
		handleEnc, err = purego.Dlopen(n, purego.RTLD_LAZY)
		if err == nil {
			break
		}
	}
	if err != nil {
		panic(fmt.Errorf("loading libvorbisenc libraries %v: %v", namesEnc, err))
	}
	purego.RegisterLibFunc(&vorbisInfoInit, handle, "vorbis_info_init")
	purego.RegisterLibFunc(&vorbisEncodeInitVBR, handleEnc, "vorbis_encode_init_vbr")
	purego.RegisterLibFunc(&vorbisCommentInit, handle, "vorbis_comment_init")
	purego.RegisterLibFunc(&vorbisCommentAddTag, handle, "vorbis_comment_add_tag")
	purego.RegisterLibFunc(&vorbisAnalysisInit, handle, "vorbis_analysis_init")
	purego.RegisterLibFunc(&vorbisBlockInit, handle, "vorbis_block_init")
	purego.RegisterLibFunc(&vorbisAnalysisHeaderout, handle, "vorbis_analysis_headerout")
	purego.RegisterLibFunc(&vorbisAnalysisBuffer, handle, "vorbis_analysis_buffer")
	purego.RegisterLibFunc(&vorbisAnalysisWrote, handle, "vorbis_analysis_wrote")
	purego.RegisterLibFunc(&vorbisAnalysisBlockout, handle, "vorbis_analysis_blockout")
	purego.RegisterLibFunc(&vorbisAnalysis, handle, "vorbis_analysis")
	purego.RegisterLibFunc(&vorbisBitrateAddBlock, handle, "vorbis_bitrate_addblock")
	purego.RegisterLibFunc(&vorbisBitrateFlushPacket, handle, "vorbis_bitrate_flushpacket")
	vorbisLib = handle
	vorbisEncLib = handleEnc
}

func getPacketData(pkt *oggPacket) []byte {
	n := int(pkt.bytes)
	src := unsafe.Slice((*byte)(unsafe.Pointer(pkt.packet)), n)
	dst := make([]byte, n)
	copy(dst, src)
	return dst
}

func encodeVorbisToOgg(w io.Writer, pcm []int16, sampleRate, channels int) error {
	vorbisOnce.Do(initVorbis)
	var vi vorbisInfo
	vorbisInfoInit(&vi)
	if vorbisEncodeInitVBR(&vi, int64(channels), int64(sampleRate), 0.5) != 0 {
		return fmt.Errorf("vorbis_encode_init_vbr failed")
	}
	var vc vorbisComment
	vorbisCommentInit(&vc)
	vendor := []byte("goqoa")
	vorbisCommentAddTag(&vc, &([]byte("ENCODER"))[0], &vendor[0])
	var vd vorbisDspState
	vorbisAnalysisInit(&vd, &vi)
	var vb vorbisBlock
	vorbisBlockInit(&vd, &vb)
	var header, headerComm, headerCode oggPacket
	vorbisAnalysisHeaderout(&vd, &vc, &header, &headerComm, &headerCode)
	enc := ogg.NewEncoder(1, w)
	enc.EncodeBOS(0, getPacketData(&header))
	enc.Encode(0, getPacketData(&headerComm))
	enc.Encode(0, getPacketData(&headerCode))
	total := len(pcm) / channels
	const maxBlock = 1024
	ptr := 0
	for ptr < total {
		block := maxBlock
		if rem := total - ptr; rem < block {
			block = rem
		}
		dataPtr := vorbisAnalysisBuffer(&vd, block)
		chPtr := unsafe.Slice((*uintptr)(unsafe.Pointer(dataPtr)), channels)
		for c := 0; c < channels; c++ {
			buf := unsafe.Slice((*float32)(unsafe.Pointer(chPtr[c])), block)
			for i := 0; i < block; i++ {
				buf[i] = float32(pcm[(ptr+i)*channels+c]) / 32768.0
			}
		}
		vorbisAnalysisWrote(&vd, block)
		for vorbisAnalysisBlockout(&vd, &vb) == 1 {
			vorbisAnalysis(&vb, nil)
			vorbisBitrateAddBlock(&vb)
			var pkt oggPacket
			for vorbisBitrateFlushPacket(&vd, &pkt) == 1 {
				enc.Encode(pkt.granulepos, getPacketData(&pkt))
			}
		}
		ptr += block
	}
	vorbisAnalysisWrote(&vd, 0)
	for vorbisAnalysisBlockout(&vd, &vb) == 1 {
		vorbisAnalysis(&vb, nil)
		vorbisBitrateAddBlock(&vb)
		var pkt oggPacket
		for vorbisBitrateFlushPacket(&vd, &pkt) == 1 {
			enc.Encode(pkt.granulepos, getPacketData(&pkt))
		}
	}
	if err := enc.EncodeEOS(); err != nil {
		return err
	}
	return nil
}
