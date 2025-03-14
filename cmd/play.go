package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/braheezy/qoa"
	"github.com/spf13/cobra"
)

var playCmd = &cobra.Command{
	Use:   "play [<file/directories>]",
	Short: "Play .qoa audio file(s)",
	Long:  "Provide one or more QOA files to play. If none are provided, the current directory is tried by default.",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// Use current directory if no arguments are provided
		if len(args) == 0 {
			args = append(args, ".")
		}

		// Input is one or more files or directories. Find all QOA files, recursively.
		var allFiles []string
		for _, arg := range args {
			info, err := os.Stat(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error accessing %s: %v\n", arg, err)
				continue
			}
			if info.IsDir() {
				files, err := findAllQOAFiles(arg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error walking %s: %v\n", arg, err)
					continue
				}
				allFiles = append(allFiles, files...)
			} else {
				valid, err := qoa.IsValidQOAFile(arg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error checking file %s: %v\n", arg, err)
					continue
				}
				if valid {
					allFiles = append(allFiles, arg)
				}
			}
		}
		if len(allFiles) == 0 {
			fmt.Println("No valid QOA files found :(")
			return
		}

		noTUI, _ := cmd.Flags().GetBool("no-tui")
		if noTUI {
			startMinimalPlayer(allFiles[0]) // Only play first file in minimal mode
		} else {
			startTUI(allFiles)
		}
	},
}

// Recursive function to find all valid QOA files
func findAllQOAFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			valid, _ := qoa.IsValidQOAFile(path)
			if valid {
				files = append(files, path)
			}
		}
		return nil
	})
	return files, err
}
func init() {
	rootCmd.AddCommand(playCmd)
	playCmd.Flags().BoolP("no-tui", "n", false, "Play audio without the TUI interface")
}
