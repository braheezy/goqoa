package cmd

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func execute(t *testing.T, command *cobra.Command, args ...string) (string, error) {
	t.Helper()

	buf := new(bytes.Buffer)
	command.SetOut(buf)
	command.SetErr(buf)
	command.SetArgs(args)

	err := command.Execute()
	return strings.TrimSpace(buf.String()), err
}

func TestConvertCmd(t *testing.T) {
	tt := []struct {
		audioFormat  string
		inputFormat  string
		outputFormat string
	}{
		{
			audioFormat:  "wav",
			inputFormat:  "qoa",
			outputFormat: "wav",
		},
		{
			audioFormat:  "wav",
			inputFormat:  "wav",
			outputFormat: "qoa",
		},
		{
			audioFormat:  "ogg",
			inputFormat:  "ogg",
			outputFormat: "qoa",
		},
		{
			audioFormat:  "ogg",
			inputFormat:  "qoa",
			outputFormat: "ogg",
		},
		{
			audioFormat:  "flac",
			inputFormat:  "flac",
			outputFormat: "qoa",
		},
		{
			audioFormat:  "flac",
			inputFormat:  "qoa",
			outputFormat: "flac",
		},
		{
			audioFormat:  "mp3",
			inputFormat:  "mp3",
			outputFormat: "qoa",
		},
		{
			audioFormat:  "mp3",
			inputFormat:  "qoa",
			outputFormat: "mp3",
		},
	}

	for _, tc := range tt {

		// TODO: On newer macos, the output ogg is different than other OSes. Why?
		if runtime.GOOS == "darwin" && tc.inputFormat == "ogg" {
			continue
		}

		// Ogg encoding only support on macos
		if runtime.GOOS != "darwin" && tc.audioFormat == "ogg" {
			continue
		}

		inputFilename := fmt.Sprintf("testdata/%s/test.%s", tc.audioFormat, tc.inputFormat)
		outputFilename := fmt.Sprintf("testdata/%s/temp.%s", tc.audioFormat, tc.outputFormat)
		expectedFilename := fmt.Sprintf("testdata/%s/test.%s.%s", tc.audioFormat, tc.inputFormat, tc.outputFormat)

		args := []string{"convert", inputFilename, outputFilename}
		_, err := execute(t, rootCmd, args...)
		if err != nil {
			t.Fatal(err)
		}

		expectedData, err := os.ReadFile(expectedFilename)
		if err != nil {
			t.Fatal(err)
		}

		actualData, err := os.ReadFile(outputFilename)
		if err != nil {
			t.Fatal(err)
		}

		expectedChecksum := md5.Sum(expectedData)
		expectedChecksumStr := hex.EncodeToString(expectedChecksum[:])
		actualChecksum := md5.Sum(actualData)
		actualChecksumStr := hex.EncodeToString(actualChecksum[:])

		// Compare the checksums
		require.Equalf(t, expectedChecksumStr, actualChecksumStr, "(%s) Conversion failed for %s -> %s", tc.audioFormat, tc.inputFormat, tc.outputFormat)

		os.Remove(outputFilename)
	}
}
