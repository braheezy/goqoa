package cmd

import (
	"os"

	"github.com/charmbracelet/log"
)

var logger *log.Logger

func setupLogger() {
	logger = log.New(os.Stdout)
	logger.SetReportTimestamp(false)

	if verbose {
		logger.SetLevel(log.DebugLevel)
	} else if quiet {
		// Redirect output to /dev/null
		os.Stdout, _ = os.Open(os.DevNull)
	}
}
