package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"moul.io/banner"

	"github.com/gantz-ai/gantz-cli/internal/config"
	"github.com/gantz-ai/gantz-cli/internal/mcp"
	"github.com/gantz-ai/gantz-cli/internal/tunnel"
)

var (
	version  = "0.1.3"
	cfgFile  string
	relayURL string
)

var (
	cyan    = color.New(color.FgHiCyan).SprintFunc()
	green   = color.New(color.FgHiGreen).SprintFunc()
	yellow  = color.New(color.FgHiYellow).SprintFunc()
	blue    = color.New(color.FgHiBlue).SprintFunc()
	magenta = color.New(color.FgHiMagenta).SprintFunc()
	dim     = color.New(color.Faint).SprintFunc()
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "gantz",
	Short: "Gantz Run - Local MCP tunnel for AI agents",
	Long: `Gantz Run allows you to run local scripts and HTTP APIs as MCP tools
and expose them via a secure tunnel URL for AI agents to connect.

Example:
  gantz run              # Start server with gantz.yaml
  gantz run -c my.yaml   # Start with custom config`,
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the MCP server and tunnel",
	Long:  `Start a local MCP server and expose it via tunnel to AI agents.`,
	RunE:  runServer,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("gantz version %s\n", version)
	},
}

func init() {
	runCmd.Flags().StringVarP(&cfgFile, "config", "c", "gantz.yaml", "config file path")
	runCmd.Flags().StringVar(&relayURL, "relay", "wss://relay.gantz.run", "relay server URL")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(versionCmd)
}

func printBanner() {
	color.HiCyan(banner.Inline("gantz run"))
	fmt.Println()
}

func runServer(cmd *cobra.Command, args []string) error {
	printBanner()

	// Load config
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Printf("%s %s\n", dim("version"), cyan("v"+version))
	fmt.Printf("%s %s %s %s\n", dim("loaded"), green(fmt.Sprintf("%d", len(cfg.Tools))), dim("tools from"), cfgFile)

	// Create MCP server
	mcpServer := mcp.NewServer(cfg)

	// Connect to relay
	fmt.Printf("\n%s\n", yellow("Connecting to relay server..."))

	tunnelClient := tunnel.NewClient(relayURL, mcpServer)
	tunnelURL, err := tunnelClient.Connect()
	if err != nil {
		return fmt.Errorf("connect tunnel: %w", err)
	}

	fmt.Println()
	fmt.Printf("  %s %s\n", dim("MCP Server URL:"), green(tunnelURL))
	fmt.Println()
	fmt.Printf("  %s\n", dim("Add to Claude Desktop config:"))
	fmt.Printf("  %s\n", dim("{"))
	fmt.Printf("    %s %s\n", blue("\"mcpServers\":"), dim("{"))
	fmt.Printf("      %s %s\n", magenta(fmt.Sprintf("\"%s\":", cfg.Name)), dim("{"))
	fmt.Printf("        %s %s\n", blue("\"url\":"), green(fmt.Sprintf("\"%s\"", tunnelURL)))
	fmt.Printf("      %s\n", dim("}"))
	fmt.Printf("    %s\n", dim("}"))
	fmt.Printf("  %s\n", dim("}"))
	fmt.Println()
	fmt.Printf("%s\n\n", dim("Press Ctrl+C to stop"))

	return tunnelClient.Wait()
}
