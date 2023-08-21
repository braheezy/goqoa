# QOA: Quite OK Audio
> The Quite OK Audio Format for Fast, Lossy Compression.

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

## Install
Install directly with Go:

    go install github.com/braheezy/goqoa@latest

## Development
You'll need the following:
- Go 1.21+
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
