//go:build unit

package main

import "github.com/spf13/cobra"

// serveCmd stub for unit builds (real implementation excluded via !unit build tag).
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Go-Virtual server (not available in unit builds)",
}
