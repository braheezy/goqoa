package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "1.0.0"

var rootCmd = &cobra.Command{
	Use:   "goqoa",
	Short: "A simple QOA utility.",
	Long:  "A CLI tool to play and convert QOA audio files.",
	Run: func(cmd *cobra.Command, args []string) {
		// Display help when no subcommand is provided
		fmt.Println("Usage: goqoa [command]")
		fmt.Println("Use 'goqoa help' for a list of commands.")
	},
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
}

func Execute() error {
	return rootCmd.Execute()
}
