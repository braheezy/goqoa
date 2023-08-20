# Quite OK Audio Format
This is a Go implementation of the [QOA Format Specification](https://qoaformat.org/)

## Background
I saw a post on QOA on HackerNews and found the reference implementation was ~800 lines of C. There were many ports to other languages, but Go was not listed! So here we are.

I developed this with an LLM-based workflow:
- I gave the [formal specification](https://qoaformat.org/qoa-specification.pdf) to `gpt-3.5` via ChatGPT and told it to explain everything I didn't understand about it (which was the entire thing because I don't know anything about audio encoding).
- I gave the entire C reference implementation to `gpt-3.5-turbo-16k` via OpenAI playground because it has the context window to fit the entire file.
- Then we wrote code:
    - `gpt-3.5-turbo-16k` to do the heavy lifting of converting C to Go. I asked it 1 function at a time
    - `gpt-3.5` to explain and tweak the ported code
    - Both models helped write unit tests
- After getting a working decoder/encoder

## TODO
- CLI app `qoa`
    - `qoa convert <in>.{qoa,wav} <out>.{qoa,wav}`
