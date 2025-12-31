package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"moul.io/banner"

	"github.com/gantz-ai/gantz-cli/internal/config"
	"github.com/gantz-ai/gantz-cli/internal/mcp"
	"github.com/gantz-ai/gantz-cli/internal/tunnel"
)

var (
	version  = "0.1.4"
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
		checkForUpdates()
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a sample gantz.yaml config file",
	Long:  `Generate a sample gantz.yaml configuration file with example tools.`,
	RunE:  runInit,
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the config file",
	Long:  `Check the gantz.yaml configuration file for errors before running.`,
	RunE:  runValidate,
}

func init() {
	runCmd.Flags().StringVarP(&cfgFile, "config", "c", "gantz.yaml", "config file path")
	runCmd.Flags().StringVar(&relayURL, "relay", "wss://relay.gantz.run", "relay server URL")
	validateCmd.Flags().StringVarP(&cfgFile, "config", "c", "gantz.yaml", "config file path")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(validateCmd)
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

// runInit creates a sample gantz.yaml file
func runInit(cmd *cobra.Command, args []string) error {
	filename := "gantz.yaml"

	// Check if file already exists
	if _, err := os.Stat(filename); err == nil {
		return fmt.Errorf("%s already exists. Remove it first or use a different directory", filename)
	}

	sampleConfig := `# Gantz Run Configuration
# Documentation: https://github.com/gantz-ai/gantz-cli

name: my-tools
version: "1.0.0"

tools:
  # Example 1: Inline shell script
  - name: hello
    description: Say hello to someone
    parameters:
      - name: name
        type: string
        description: Name of the person to greet
        required: true
    script:
      shell: echo "Hello, {{name}}!"

  # Example 2: List files in a directory
  - name: list_files
    description: List files in a directory
    parameters:
      - name: path
        type: string
        description: Directory path to list
        default: "."
    script:
      shell: ls -la "{{path}}"

  # Example 3: Run a script file
  # - name: analyze
  #   description: Run analysis script
  #   parameters:
  #     - name: input
  #       type: string
  #       required: true
  #   script:
  #     command: python3
  #     args: ["./scripts/analyze.py", "{{input}}"]
  #     working_dir: "/path/to/project"

  # Example 4: HTTP API call
  # - name: get_weather
  #   description: Get weather for a city
  #   parameters:
  #     - name: city
  #       type: string
  #       required: true
  #   http:
  #     method: GET
  #     url: "https://api.example.com/weather?city={{city}}"
  #     headers:
  #       Authorization: "Bearer ${API_KEY}"
  #     extract_json: "data.temperature"
`

	if err := os.WriteFile(filename, []byte(sampleConfig), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", filename, err)
	}

	fmt.Printf("%s Created %s\n", green("✓"), cyan(filename))
	fmt.Println()
	fmt.Printf("Next steps:\n")
	fmt.Printf("  1. Edit %s to add your tools\n", cyan(filename))
	fmt.Printf("  2. Run %s to start the server\n", cyan("gantz run"))
	fmt.Println()

	return nil
}

// runValidate validates the config file
func runValidate(cmd *cobra.Command, args []string) error {
	fmt.Printf("Validating %s...\n\n", cyan(cfgFile))

	cfg, err := config.Load(cfgFile)
	if err != nil {
		fmt.Printf("%s %s\n", color.RedString("✗"), err.Error())
		return fmt.Errorf("validation failed")
	}

	fmt.Printf("%s Config file is valid\n", green("✓"))
	fmt.Printf("  Name: %s\n", cyan(cfg.Name))
	fmt.Printf("  Version: %s\n", cfg.Version)
	fmt.Printf("  Tools: %s\n", green(fmt.Sprintf("%d", len(cfg.Tools))))
	fmt.Println()

	for i, tool := range cfg.Tools {
		toolType := "script"
		if tool.IsHTTP() {
			toolType = "http"
		}
		fmt.Printf("  %d. %s %s\n", i+1, cyan(tool.Name), dim("("+toolType+")"))
		if tool.Description != "" {
			fmt.Printf("     %s\n", dim(tool.Description))
		}
	}
	fmt.Println()

	return nil
}

// checkForUpdates checks if a newer version is available
func checkForUpdates() {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/gantz-ai/gantz-cli/releases/latest")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return
	}

	latestVersion := release.TagName
	if latestVersion != "" && latestVersion[0] == 'v' {
		latestVersion = latestVersion[1:]
	}

	if latestVersion != "" && latestVersion != version {
		fmt.Println()
		fmt.Printf("%s New version available: %s → %s\n", yellow("!"), dim("v"+version), green("v"+latestVersion))
		fmt.Printf("  Update: %s\n", cyan("curl -fsSL https://gantz.run/install.sh | sh"))
	}
}
