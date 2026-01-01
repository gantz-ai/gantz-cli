package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"moul.io/banner"

	"github.com/gantz-ai/gantz-cli/internal/config"
	"github.com/gantz-ai/gantz-cli/internal/mcp"
	"github.com/gantz-ai/gantz-cli/internal/tunnel"
)

var (
	version  = "0.3.4"
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
	Run: func(cmd *cobra.Command, args []string) {
		printBanner()
		fmt.Printf("  %s %s\n", cyan("Gantz"), blue("Run"))
		fmt.Printf("  %s\n\n", dim("Local MCP tunnel for AI agents"))

		fmt.Printf("  %s\n", dim("Commands"))
		fmt.Printf("  %s %-22s %s\n", dim("•"), cyan("gantz run"), dim("Start server with gantz.yaml"))
		fmt.Printf("  %s %-22s %s\n", dim("•"), cyan("gantz init"), dim("Create sample gantz.yaml"))
		fmt.Printf("  %s %-22s %s\n", dim("•"), cyan("gantz validate"), dim("Validate config file"))
		fmt.Printf("  %s %-22s %s\n", dim("•"), cyan("gantz version"), dim("Show version info"))
		fmt.Println()

		fmt.Printf("  %s\n", dim("Sample Files"))
		fmt.Printf("  %s %-22s %s\n", dim("•"), cyan("https://gantz.run/gantz.yaml"), dim("Sample config"))
		fmt.Printf("  %s %-22s %s\n", dim("•"), cyan("https://gantz.run/client.py"), dim("Sample Python client"))
		fmt.Println()

		fmt.Printf("  %s\n", dim("Quick Start"))
		fmt.Printf("  %s\n", dim("  curl -O https://gantz.run/gantz.yaml"))
		fmt.Printf("  %s\n", dim("  gantz run"))
		fmt.Println()
	},
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

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update gantz to the latest version",
	Long:  `Download and install the latest version of gantz.`,
	RunE:  runUpdate,
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall gantz from your system",
	Long:  `Remove the gantz binary from your system.`,
	RunE:  runUninstall,
}

func init() {
	runCmd.Flags().StringVarP(&cfgFile, "config", "c", "gantz.yaml", "config file path")
	runCmd.Flags().StringVar(&relayURL, "relay", "wss://relay.gantz.run", "relay server URL")
	validateCmd.Flags().StringVarP(&cfgFile, "config", "c", "gantz.yaml", "config file path")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(uninstallCmd)
}

func printBanner() {
	fmt.Println()
	color.HiCyan(banner.Inline("gantz"))
	fmt.Println()
}

func runServer(cmd *cobra.Command, args []string) error {
	printBanner()

	// Load config
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Check for updates in background
	go checkForUpdates()

	// Create MCP server
	mcpServer := mcp.NewServer(cfg)

	// Start config file watcher
	go watchConfig(cfgFile, mcpServer)

	// Connect to relay
	fmt.Printf("  %s %s\n", dim("●"), yellow("Connecting to relay..."))

	tunnelClient := tunnel.NewClient(relayURL, mcpServer, version, len(cfg.Tools))
	tunnelClient.OnClientConnected(func(clientIP string) {
		fmt.Printf("\n  %s %s %s\n", blue("●"), blue("Client connected"), dim(clientIP))
	})
	tunnelURL, err := tunnelClient.Connect()
	if err != nil {
		return fmt.Errorf("connect tunnel: %w", err)
	}

	// Clear connecting line and print success
	fmt.Printf("\r  %s %s                    \n", green("●"), green("Connected"))
	fmt.Println()

	// Print server URL prominently
	fmt.Printf("  %s\n", dim("Server URL"))
	fmt.Printf("  %s\n", green(tunnelURL))
	fmt.Println()

	// Print sample client link (clickable in most terminals)
	fmt.Printf("  %s\n", dim("Sample Client"))
	fmt.Printf("  %s %s\n", dim("•"), cyan(tunnelURL+"/sample-client.py"))
	fmt.Println()

	// Print loaded tools
	fmt.Printf("  %s %s\n", dim("Tools"), dim("("+filepath.Base(cfgFile)+")"))
	for _, tool := range cfg.Tools {
		toolType := dim("script")
		if tool.IsHTTP() {
			toolType = magenta("http")
		}
		fmt.Printf("  %s %-20s %s\n", dim("•"), tool.Name, toolType)
	}
	fmt.Println()

	// Footer
	fmt.Printf("  %s  %s\n", dim("v"+version), dim("Hot-reload enabled"))
	fmt.Printf("  %s\n\n", dim("Ctrl+C to stop"))

	return tunnelClient.Wait()
}

// watchConfig watches the config file for changes and reloads it
func watchConfig(cfgPath string, mcpServer *mcp.Server) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("%s Failed to create file watcher: %v\n", yellow("!"), err)
		return
	}
	defer watcher.Close()

	// Get absolute path for the config file
	absPath, err := filepath.Abs(cfgPath)
	if err != nil {
		fmt.Printf("%s Failed to get absolute path: %v\n", yellow("!"), err)
		return
	}

	// Watch the directory containing the config file (to catch editor save patterns)
	dir := filepath.Dir(absPath)
	if err := watcher.Add(dir); err != nil {
		fmt.Printf("%s Failed to watch config directory: %v\n", yellow("!"), err)
		return
	}

	// Debounce timer to avoid multiple reloads
	var debounceTimer *time.Timer
	debounceDelay := 100 * time.Millisecond

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// Only react to writes on our config file
			if filepath.Base(event.Name) == filepath.Base(absPath) {
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					// Debounce: reset timer on each event
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
					debounceTimer = time.AfterFunc(debounceDelay, func() {
						reloadConfig(cfgPath, mcpServer)
					})
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("%s File watcher error: %v\n", yellow("!"), err)
		}
	}
}

// reloadConfig reloads the config file and updates the MCP server
func reloadConfig(cfgPath string, mcpServer *mcp.Server) {
	newCfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Printf("\n  %s %s %v\n", color.RedString("●"), color.RedString("Reload failed:"), err)
		return
	}

	mcpServer.UpdateConfig(newCfg)
	fmt.Printf("\n  %s %s %s tools\n", green("●"), green("Reloaded"), green(fmt.Sprintf("%d", len(newCfg.Tools))))
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

	// Strip "v" prefix from current version for comparison
	currentVersion := version
	if currentVersion != "" && currentVersion[0] == 'v' {
		currentVersion = currentVersion[1:]
	}

	if latestVersion != "" && latestVersion != currentVersion {
		fmt.Println()
		fmt.Printf("  %s %s %s → %s\n", yellow("●"), yellow("Update available"), dim("v"+currentVersion), green("v"+latestVersion))
		fmt.Printf("    %s\n", dim("Run: gantz update"))
	}
}

// runUpdate downloads and installs the latest version
func runUpdate(cmd *cobra.Command, args []string) error {
	fmt.Printf("Checking for updates...\n")

	// Get latest version
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/gantz-ai/gantz-cli/releases/latest")
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to check for updates: HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release info: %w", err)
	}

	latestVersion := release.TagName
	if latestVersion != "" && latestVersion[0] == 'v' {
		latestVersion = latestVersion[1:]
	}

	currentVersion := version
	if currentVersion != "" && currentVersion[0] == 'v' {
		currentVersion = currentVersion[1:]
	}

	if latestVersion == currentVersion {
		fmt.Printf("%s Already on latest version: %s\n", green("✓"), cyan("v"+currentVersion))
		return nil
	}

	fmt.Printf("Updating %s → %s\n\n", dim("v"+currentVersion), green("v"+latestVersion))

	// Run install script
	if runtime.GOOS == "windows" {
		return fmt.Errorf("automatic update not supported on Windows\n\nDownload manually from: https://github.com/gantz-ai/gantz-cli/releases")
	}

	installCmd := exec.Command("sh", "-c", "curl -fsSL https://gantz.run/install.sh | sh")
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr

	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Printf("\n%s Updated to %s\n", green("✓"), green("v"+latestVersion))
	return nil
}

// runUninstall removes gantz from the system
func runUninstall(cmd *cobra.Command, args []string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("automatic uninstall not supported on Windows\n\nManually delete the gantz.exe file")
	}

	// Find where gantz is installed
	gantzPath, err := exec.LookPath("gantz")
	if err != nil {
		return fmt.Errorf("gantz not found in PATH")
	}

	fmt.Printf("Found gantz at: %s\n", cyan(gantzPath))
	fmt.Printf("Removing...\n")

	// Try to remove directly first
	if err := os.Remove(gantzPath); err != nil {
		// If permission denied, try with sudo
		if os.IsPermission(err) {
			fmt.Printf("Permission denied, trying with sudo...\n")
			sudoCmd := exec.Command("sudo", "rm", gantzPath)
			sudoCmd.Stdout = os.Stdout
			sudoCmd.Stderr = os.Stderr
			sudoCmd.Stdin = os.Stdin
			if err := sudoCmd.Run(); err != nil {
				return fmt.Errorf("failed to remove gantz: %w", err)
			}
		} else {
			return fmt.Errorf("failed to remove gantz: %w", err)
		}
	}

	fmt.Printf("\n%s Gantz has been uninstalled\n", green("✓"))
	fmt.Printf("\nTo reinstall: %s\n", cyan("curl -fsSL https://gantz.run/install.sh | sh"))
	return nil
}
