package cmd

import (
	"github.com/spf13/cobra"
)

var version = "2.2.0"

var rootCmd = &cobra.Command{
	Use:   "goqoa",
	Short: "A simple QOA utility.",
	Long:  "A CLI tool to play and convert QOA audio files.",
	Args:  cobra.NoArgs,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogger()
	},
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
}

var quiet bool
var verbose bool

func init() {
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress command output")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Increase command output")
}

func Execute() error {
	return rootCmd.Execute()
}
