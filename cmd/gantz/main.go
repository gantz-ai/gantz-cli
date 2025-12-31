package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/gantz-ai/gantz-cli/internal/config"
	"github.com/gantz-ai/gantz-cli/internal/mcp"
	"github.com/gantz-ai/gantz-cli/internal/tunnel"
)

var (
	version   = "0.1.0"
	cfgFile   string
	relayURL  string
	localOnly bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "gantz",
	Short: "Gantz CLI - Local MCP server with cloud tunneling",
	Long: `Gantz CLI allows you to run local scripts as MCP tools
and expose them via a secure tunnel URL for AI agents to connect.

Example:
  gantz serve              # Start server with gantz.yaml
  gantz serve -c my.yaml   # Start with custom config
  gantz serve --local      # Local only, no tunnel`,
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	Long:  `Start a local MCP server and optionally expose it via tunnel.`,
	RunE:  runServe,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("gantz version %s\n", version)
	},
}

func init() {
	serveCmd.Flags().StringVarP(&cfgFile, "config", "c", "gantz.yaml", "config file path")
	serveCmd.Flags().StringVar(&relayURL, "relay", "wss://relay.gantz.run", "relay server URL")
	serveCmd.Flags().BoolVar(&localOnly, "local", false, "local only mode (no tunnel)")

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Printf("Gantz CLI v%s\n", version)
	fmt.Printf("Loaded %d tools from %s\n", len(cfg.Tools), cfgFile)

	// Create MCP server
	mcpServer := mcp.NewServer(cfg)

	if localOnly {
		// Local mode - just run HTTP server
		fmt.Printf("\nLocal mode - listening on http://localhost:%d\n", cfg.Server.Port)
		return mcpServer.ListenAndServe(fmt.Sprintf(":%d", cfg.Server.Port))
	}

	// Tunnel mode - connect to relay
	fmt.Println("\nConnecting to relay server...")

	tunnelClient := tunnel.NewClient(relayURL, mcpServer)
	tunnelURL, err := tunnelClient.Connect()
	if err != nil {
		return fmt.Errorf("connect tunnel: %w", err)
	}

	fmt.Printf("\n  MCP Server URL: %s\n", tunnelURL)
	fmt.Printf("\n  Add to Claude Desktop config:\n")
	fmt.Printf("  {\n")
	fmt.Printf("    \"mcpServers\": {\n")
	fmt.Printf("      \"%s\": {\n", cfg.Name)
	fmt.Printf("        \"url\": \"%s\"\n", tunnelURL)
	fmt.Printf("      }\n")
	fmt.Printf("    }\n")
	fmt.Printf("  }\n\n")
	fmt.Println("Press Ctrl+C to stop")

	return tunnelClient.Wait()
}
