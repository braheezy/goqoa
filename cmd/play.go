package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/braheezy/qoa"
	"github.com/spf13/cobra"
)

var playCmd = &cobra.Command{
	Use:   "play <file/directories>",
	Short: "Play .qoa audio file(s)",
	Long:  "Provide one or more QOA files to play.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
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
		startTUI(allFiles)
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
}
