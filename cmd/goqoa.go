package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "2.1.0"

var rootCmd = &cobra.Command{
	Use:   "goqoa",
	Short: "A simple QOA utility.",
	Long:  "A CLI tool to play and convert QOA audio files.",
	Run: func(cmd *cobra.Command, args []string) {
		// Display help when no subcommand is provided
		fmt.Println("Usage: goqoa [command]")
		fmt.Println("Use 'goqoa help' for a list of commands.")
	},
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
