package mp3

import "os"

func (enc *GlobalConfig) Write(filePath string, pcmData []int16) error {
	// Open the output MP3 file for writing.
	outputFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer outputFile.Close()
	var x *int

	// Encoding Loop: Encode PCM data and write to the output file.
	var encodedData []byte
	samplesPerPass := enc.samplesPerPass()
	for len(pcmData) > 0 {
		// Determine the number of samples to encode in this pass.
		if len(pcmData) < samplesPerPass {
			samplesPerPass = len(pcmData)
		}

		encodedData = append(encodedData,
			encodeBufferInterleaved(enc, pcmData[:samplesPerPass], x)...)
		_, err := outputFile.Write(encodedData)
		if err != nil {
			return err
		}

		pcmData = pcmData[samplesPerPass:]
	}

	encodedData = flush(enc, x)
	_, err = outputFile.Write(encodedData)
	if err != nil {
		return err
	}

	return nil
}
