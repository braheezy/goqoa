package cmd

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/braheezy/goqoa/pkg/qoa"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/jfreymuth/oggvorbis"
	"github.com/mewkiz/flac"

	"github.com/spf13/cobra"
)

var convertCmd = &cobra.Command{
	Use:   "convert <input-file> <output-file>",
	Short: "Convert between QOA and other audio formats",
	Long:  fmt.Sprintf("Convert between QOA and other audio formats. The supported audio formats are:\n%v", strings.Join(supportedFormats, "\n")),
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		inputFile := args[0]
		outputFile := args[1]

		if isSupportedConversion(inputFile, outputFile) {
			convertAudio(inputFile, outputFile)
		} else {
			logger.Fatal("Unsupported conversion")
		}
	},
	DisableFlagsInUseLine: true,
}

var supportedFormats = []string{".qoa", ".wav", ".mp3", ".ogg", ".flac"}

func init() {
	rootCmd.AddCommand(convertCmd)
}

// Function to check if the conversion is supported
func isSupportedConversion(inputFile, outputFile string) bool {
	inExt := filepath.Ext(inputFile)
	outExt := filepath.Ext(outputFile)

	notSameFileExt := inExt != outExt
	bothSupportedExt := contains(supportedFormats, inExt) && contains(supportedFormats, outExt)
	atLeastOneQoaExt := hasQOAExtension(inputFile) || hasQOAExtension(outputFile)

	return notSameFileExt && bothSupportedExt && atLeastOneQoaExt
}

func contains(arr []string, target string) bool {
	for _, item := range arr {
		if item == target {
			return true
		}
	}
	return false
}

func hasQOAExtension(filename string) bool {
	return filepath.Ext(filename) == ".qoa"
}

// Function to convert audio between formats
func convertAudio(inputFile, outputFile string) {
	// Load the input audio file
	inputData, err := os.ReadFile(inputFile)
	if err != nil {
		logger.Fatalf("Error loading audio file: %v\n", err)
	}

	// For the given input file type, we will obtain these values
	// decodedData is the audio data converted to int16 (QOA format)
	var decodedData []int16
	// q is the QOA description. It is easiest created while decoding the input file.
	var q *qoa.QOA

	inExt := filepath.Ext(inputFile)
	switch inExt {
	case ".qoa":
		logger.Info("Input format is QOA")
		q, decodedData, err = qoa.Decode(inputData)
		if err != nil {
			log.Fatalf("Error decoding QOA data: %v", err)
		}
	case ".wav":
		logger.Info("Input format is WAV")
		wavReader := bytes.NewReader(inputData)
		wavDecoder := wav.NewDecoder(wavReader)

		// Attempt to estimate total number of samples
		// This part is tricky without decoding, but WAV files have their total data size in the header
		// For the purpose of demonstration, assuming an average bit depth and stereo channels
		fileSize := int64(len(inputData))

		// Preallocate decodedData slice based on the estimation
		decodedData = make([]int16, fileSize)

		// Initialize an audio.IntBuffer to hold the PCM data
		pcmBuffer := &audio.IntBuffer{Data: make([]int, 4096), Format: wavDecoder.Format()}
		numSamples := uint32(0)

		for {
			n, err := wavDecoder.PCMBuffer(pcmBuffer)
			if err != nil {
				log.Fatalf("Error decoding WAV file: %v", err)
			}
			if n == 0 {
				break
			}

			// Directly copy the decoded PCM data to decodedData slice at the correct position
			for i := 0; i < n; i++ {
				decodedData[numSamples*uint32(pcmBuffer.Format.NumChannels)+uint32(i)] = int16(pcmBuffer.Data[i])
			}
			numSamples += uint32(n) / uint32(pcmBuffer.Format.NumChannels)
		}

		decodedData = decodedData[:numSamples*uint32(pcmBuffer.Format.NumChannels)]

		q = qoa.NewEncoder(
			uint32(wavDecoder.Format().SampleRate),
			uint32(wavDecoder.Format().NumChannels),
			numSamples,
		)

		logger.Debug(inputFile, "channels", pcmBuffer.Format.NumChannels, "samplerate(hz)", pcmBuffer.Format.SampleRate, "samples/channel", numSamples, "size", formatSize(len(inputData)))
	case ".mp3":
		decodedData, q = decodeMp3(&inputData, inputFile)
	case ".ogg":
		logger.Info("Input format is OGG")
		oggReader := bytes.NewReader(inputData)
		oggData, format, err := oggvorbis.ReadAll(oggReader)
		if err != nil {
			log.Fatalf("Error decoding OGG data: %v", err)
		}

		decodedData = make([]int16, len(oggData))
		for i, val := range oggData {
			// Scale to int16 range
			decodedData[i] = int16(val * 32767.0)
		}

		// Set QOA metadata
		numSamples := len(decodedData) / format.Channels
		q = qoa.NewEncoder(
			uint32(format.SampleRate),
			uint32(format.Channels),
			uint32(numSamples),
		)

		logger.Debug(inputFile, "channels", format.Channels, "samplerate(hz)", format.SampleRate, "samples/channel", numSamples, "size", formatSize(len(inputData)))
	case ".flac":
		logger.Info("Input format is FLAC")
		flacStream, err := flac.Open(inputFile)
		if err != nil {
			log.Fatalf("Error opening FLAC file: %v", err)
		}
		defer flacStream.Close()

		for {
			// Decode FLAC frame
			flacFrame, err := flacStream.ParseNext()
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Fatalf("Error parsing FLAC frame: %v", err)
			}

			// Collect audio samples
			for i := 0; i < flacFrame.Subframes[0].NSamples; i++ {
				for _, subframe := range flacFrame.Subframes {
					sample := subframe.Samples[i]
					decodedData = append(decodedData, int16(sample))
				}
			}
		}
		// Set QOA metadata
		flacMetadata := flacStream.Info
		numSamples := len(decodedData) / int(flacMetadata.NChannels)
		q = qoa.NewEncoder(
			flacMetadata.SampleRate,
			uint32(flacMetadata.NChannels),
			uint32(numSamples),
		)

		logger.Debug(inputFile, "channels", flacMetadata.NChannels, "samplerate(hz)", flacMetadata.SampleRate, "samples/channel", numSamples, "size", formatSize(len(inputData)))
	}

	outExt := filepath.Ext(outputFile)
	switch outExt {
	case ".qoa":
		logger.Info("Output format is QOA")
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

		psnr := -20.0 * math.Log10(math.Sqrt(float64(q.ErrorCount/int(q.Samples*q.Channels)))/32768.0)

		bitrate := (float64(len(qoaEncodedData)*8) / float64(q.Samples/q.SampleRate)) / 1024
		logger.Debug(outputFile, "size", formatSize(len(qoaEncodedData)), "bitrate", fmt.Sprintf("%0.2f kbit/s", bitrate), "psnr", fmt.Sprintf("%0.2f", psnr))
	case ".wav":
		logger.Info("Output format is WAV")
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
	case ".mp3":
		encodeMp3(outputFile, q, decodedData)
	case ".ogg":
		logger.Info("Output format is OGG")
		logger.Fatal("And that's not supported yet...")
	case ".flac":
		logger.Info("Output format is FLAC")
		logger.Fatal("And that's not supported yet...")
	}

	logger.Infof("Conversion completed: %s -> %s", inputFile, outputFile)
}

// formatSize converts the inputSize to a human readable format
func formatSize(inputSize int) string {
	const unit = 1024
	if inputSize < unit {
		return fmt.Sprintf("%d B", inputSize)
	}
	div, exp := int64(unit), 0
	for n := inputSize / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(inputSize)/float64(div), "KMGTPE"[exp])
}
