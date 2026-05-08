package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var healthcheckCmd = &cobra.Command{
	Use:   "healthcheck",
	Short: "Check the health of the running go-virtual server",
	Long: `Sends a GET request to /_health on the local server and exits 0 if healthy.
Intended for use as a Docker HEALTHCHECK instruction.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port := viper.GetInt("server.port")
		if port <= 0 {
			port = 8080
		}

		host := viper.GetString("server.host")
		// The bind address 0.0.0.0 (or empty) means "all interfaces" —
		// not a valid destination. Fall back to loopback for the probe.
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "127.0.0.1"
		}

		url := fmt.Sprintf("http://%s:%d/_health", host, port)

		client := &http.Client{Timeout: 4 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "healthcheck: request failed: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "healthcheck: unexpected status %d\n", resp.StatusCode)
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(healthcheckCmd)
}
