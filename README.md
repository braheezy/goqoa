# QOA: Quite OK Audio
> The Quite OK Audio Format for Fast, Lossy Compression.

[![Go Reference](https://pkg.go.dev/badge/github.com/braheezy/goqoa.svg)](https://pkg.go.dev/github.com/braheezy/goqoa)
[![Build Status](https://github.com/braheezy/goqoa/actions/workflows/ci.yml/badge.svg)](https://github.com/braheezy/goqoa/actions)

A Go implementation of the [QOA Format Specification](https://qoaformat.org/).

The `goqoa` CLI tool provides basic functions for working with `.qoa` files.

```bash
$ goqoa help
A CLI tool to play and convert QOA audio files.

Usage:
  goqoa [flags]
  goqoa [command]

Available Commands:
  convert     Convert between QOA and other audio formats
  help        Help about any command
  play        Play a .qoa audio file

Flags:
  -h, --help   help for goqoa

Use "goqoa [command] --help" for more information about a command.
```

[This blog post](https://phoboslab.org/log/2023/02/qoa-time-domain-audio-compression) by the author of QOA is a great introduction to the format and how it works.

## Install
The easiest way is a pre-built binary on the [Releases](https://github.com/braheezy/goqoa/releases) page. I tested it works on Linux and Windows. Windows doesn't support `mp3` because I couldn't figure out how to compile `libmp3lame` and link it.

Otherwise, install prerequisites for your platform:

    # Fedora
    yum install gcc alsa-lib-devel
    # Debian
    apt-get install gcc pkg-config libasound2-dev

Then, install directly with Go:

    go install github.com/braheezy/goqoa@latest

## `qoa` Package
The `qoa` package is a pure Go implementation.

Decode a `.qoa` file:
```go
data, _ := os.ReadFile("groovy-tunes.qoa")
qoaMetadata, decodedData, err = qoa.Decode(inputData)
// Do stuff with decodedData
```

Or encode audio samples. This example shows a WAV file:
```go
// Read a WAV
data, _ := os.ReadFile("groovy-tunes.wav")
wavReader := bytes.NewReader(data)
wavDecoder := wav.NewDecoder(wavReader)
wavBuffer, err := wavDecoder.FullPCMBuffer()

// Figure out audio metadata and create a new QOA encoder using the info
numSamples := uint32(len(wavBuffer.Data) / wavBuffer.Format.NumChannels)
qoaFormat := qoa.NewEncoder(
  uint32(wavBuffer.Format.SampleRate),
  uint32(wavBuffer.Format.NumChannels),
  numSamples)
// Convert the audio data to int16 (QOA format)
decodedData = make([]int16, len(wavBuffer.Data))
for i, val := range wavBuffer.Data {
  decodedData[i] = int16(val)
}

// Finally, encode the audio data
qoaEncodedData, err := q.Encode(decodedData)
```

## Development
You'll need the following:
- Go 1.*
- `make`
- The [dependencies that `oto` requires](https://github.com/ebitengine/oto#prerequisite)

Then you can `make build` to get a binary.

`make test` will run Go unit tests.

For fidelity testing against the reference spec, `check_spec.sh` compare the number of bytes in files generated by `goqoa` to the files generated by the original `qoa.h`. Those original files can be [found here](https://qoaformat.org/samples/).

I wanted to use checksums to compare the files (and even do in the [unit tests](./pkg/qoa/qoa_test.go)) but larger files seem to produce different checksums, even if they audibly sound the same.

## Disclaimer
I have never written software that deals with audio files before. I saw a post about QOA on HackerNews and found the name amusing. There were many ports to other languages, but Go was not listed. So here we are!

I developed this with an LLM-based workflow:
- I gave the [formal specification](https://qoaformat.org/qoa-specification.pdf) to `gpt-3.5` via ChatGPT and told it to explain everything I didn't understand about it (which was the entire thing because I don't know anything about audio encoding).
- Next, I gave the entire C reference implementation to `gpt-3.5-turbo-16k` via OpenAI playground because it has the context window to fit the entire file.
- Then we wrote code:
    - `gpt-3.5-turbo-16k` to do the heavy lifting of converting C to Go. I asked it 1 function at a time.
    - `gpt-3.5` to explain and tweak the ported code.
    - Both models helped write unit tests.
- After getting a working decoder/encoder, I worked with ChatGPT to integrate the Cobra framework to implement the CLI.
