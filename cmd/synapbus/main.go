package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	port    int
	dataDir string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "synapbus",
		Short: "SynapBus — MCP-native agent-to-agent messaging",
		Long:  "Local-first, MCP-native messaging service for AI agents. Single binary with embedded storage, semantic search, and a Slack-like Web UI.",
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the SynapBus server",
		RunE:  runServe,
	}

	serveCmd.Flags().IntVar(&port, "port", 8080, "HTTP server port")
	serveCmd.Flags().StringVar(&dataDir, "data", "./data", "Data directory for storage")

	rootCmd.AddCommand(serveCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runServe(cmd *cobra.Command, args []string) error {
	// Check for environment variable overrides
	if p := os.Getenv("SYNAPBUS_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}
	if d := os.Getenv("SYNAPBUS_DATA_DIR"); d != "" {
		dataDir = d
	}

	fmt.Printf("SynapBus starting on port %d with data dir %s\n", port, dataDir)
	fmt.Println("TODO: Initialize storage, MCP server, REST API, and Web UI")

	return nil
}
