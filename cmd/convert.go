package cmd

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/braheezy/goqoa/pkg/qoa"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/spf13/cobra"
)

var convertCmd = &cobra.Command{
	Use:   "convert <input-file> <output-file>",
	Short: "Convert between QOA and other audio formats",
	Long:  fmt.Sprintf("Convert between QOA and other audio formats. The supported audio formats are:\n%v", strings.Join(supportedFormats, "\n")),
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if quiet {
			// Redirect output to /dev/null
			os.Stdout, _ = os.Open(os.DevNull)
		}

		inputFile := args[0]
		outputFile := args[1]

		if isSupportedConversion(inputFile, outputFile) {
			convertAudio(inputFile, outputFile)
		} else {
			fmt.Println("Unsupported conversion")
		}
	},
	DisableFlagsInUseLine: true,
}

var supportedFormats = []string{".qoa", ".wav"}

func init() {
	rootCmd.AddCommand(convertCmd)
}

// Function to check if the conversion is supported
func isSupportedConversion(inputFile, outputFile string) bool {
	inExt := filepath.Ext(inputFile)
	outExt := filepath.Ext(outputFile)

	notSameFileExt := inExt != outExt
	bothSupportedExt := slices.Contains(supportedFormats, inExt) && slices.Contains(supportedFormats, outExt)
	atLeastOneQoaExt := hasQOAExtension(inputFile) || hasQOAExtension(outputFile)

	return notSameFileExt && bothSupportedExt && atLeastOneQoaExt
}

func hasQOAExtension(filename string) bool {
	return filepath.Ext(filename) == ".qoa"
}

// Function to convert audio between formats
func convertAudio(inputFile, outputFile string) {
	// Load the input audio file
	inputData, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Printf("Error loading audio file: %v\n", err)
		return
	}

	inExt := filepath.Ext(inputFile)
	var decodedData []int16
	var q *qoa.QOA

	switch inExt {
	case ".qoa":
		fmt.Println("Input format is QOA")
		q, decodedData, err = qoa.Decode(inputData)
		if err != nil {
			log.Fatalf("Error decoding QOA data: %v", err)
		}

	case ".wav":
		fmt.Println("Input format is WAV")
		wavReader := bytes.NewReader(inputData)
		wavDecoder := wav.NewDecoder(wavReader)
		wavBuffer, err := wavDecoder.FullPCMBuffer()
		if err != nil {
			log.Fatalf("Error decoding WAV file: %v", err)
		}
		numSamples := uint32(len(wavBuffer.Data) / wavBuffer.Format.NumChannels)
		q = qoa.NewEncoder(
			uint32(wavBuffer.Format.SampleRate),
			uint32(wavBuffer.Format.NumChannels),
			numSamples)
		// Convert the audio data to int16 (QOA format)
		decodedData = make([]int16, len(wavBuffer.Data))
		for i, val := range wavBuffer.Data {
			decodedData[i] = int16(val)
		}

	case ".mp3":
		fmt.Println("Input format is MP3")
		dec, mp3Data, err := minimp3.DecodeFull(inputData)
		if err != nil {
			log.Fatalf("Error decoding MP3 data: %v", err)
		}

		// Convert the MP3 audio data to int16 (QOA format)
		decodedData = make([]int16, len(mp3Data)/2)
		for i := 0; i < len(mp3Data)/2; i++ {
			sample := mp3Data[i*2 : (i+1)*2]
			decodedData[i] = int16(binary.LittleEndian.Uint16(sample))
		}

		// Set QOA metadata
		numSamples := len(decodedData) / dec.Channels
		q = qoa.NewEncoder(
			uint32(dec.SampleRate),
			uint32(dec.Channels),
			uint32(numSamples),
		)

	}

	outExt := filepath.Ext(outputFile)
	switch outExt {
	case ".qoa":
		fmt.Println("Output format is QOA")
		// Encode the audio data
		qoaEncodedData, err := q.Encode(decodedData)
		if err != nil {
			log.Fatalf("Error encoding audio data to QOA: %v", err)
		}
		// Save the QOA audio data to QOA file
		qoaFile, err := os.Create(outputFile)
		if err != nil {
			log.Fatalf("Error creating QOA file: %v", err)
		}
		defer qoaFile.Close()
		_, err = qoaFile.Write(qoaEncodedData)
		if err != nil {
			log.Fatalf("Error writing QOA data: %v", err)
		}

	case ".wav":
		fmt.Println("Output format is WAV")
		// Convert int16 to int for WAV conversion
		intAudioData := make([]int, len(decodedData))
		for i, val := range decodedData {
			intAudioData[i] = int(val)
		}

		wavBuffer := &audio.IntBuffer{
			Data:           intAudioData,
			Format:         &audio.Format{SampleRate: int(q.SampleRate), NumChannels: int(q.Channels)},
			SourceBitDepth: 16,
		}
		// Write the WAV audio data to WAV file
		wavFile, err := os.Create(outputFile)
		if err != nil {
			log.Fatalf("Error creating WAV file: %v", err)
		}
		defer wavFile.Close()

		wavEncoder := wav.NewEncoder(
			wavFile,
			int(q.SampleRate),
			16,
			int(q.Channels),
			1)
		if err = wavEncoder.Write(wavBuffer); err != nil {
			log.Fatalf("Error writing WAV data: %v", err)
		}
		defer wavEncoder.Close()

	}

	fmt.Printf("Conversion completed: %s -> %s\n", inputFile, outputFile)
}
