package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the gantz.yaml configuration
type Config struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Version     string       `yaml:"version"`
	Server      ServerConfig `yaml:"server"`
	Tools       []Tool       `yaml:"tools"`
}

// ServerConfig holds local server configuration
type ServerConfig struct {
	Port int `yaml:"port"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Parameters  []Parameter       `yaml:"parameters"`
	Script      ScriptConfig      `yaml:"script"`
	HTTP        HTTPConfig        `yaml:"http"`
	Environment map[string]string `yaml:"environment"`
}

// HTTPConfig holds HTTP request configuration
type HTTPConfig struct {
	Method      string            `yaml:"method"`
	URL         string            `yaml:"url"`
	Headers     map[string]string `yaml:"headers"`
	Body        string            `yaml:"body"`
	Timeout     string            `yaml:"timeout"`
	ExtractJSON string            `yaml:"extract_json"` // JSONPath to extract from response
}

// Parameter represents a tool parameter
type Parameter struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Default     string `yaml:"default"`
}

// ScriptConfig holds script execution configuration
type ScriptConfig struct {
	Command    string   `yaml:"command"`
	Args       []string `yaml:"args"`
	Shell      string   `yaml:"shell"`
	WorkingDir string   `yaml:"working_dir"`
	Timeout    string   `yaml:"timeout"`
}

// Load reads and parses the config file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file '%s' not found\n\n  Run 'gantz init' to create a sample config file", path)
		}
		return nil, fmt.Errorf("cannot read '%s': %w", path, err)
	}

	// Expand environment variables
	data = []byte(os.ExpandEnv(string(data)))

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid YAML in '%s': %w\n\n  Check for syntax errors like incorrect indentation or missing colons", path, err)
	}

	// Set defaults
	if cfg.Name == "" {
		cfg.Name = "gantz-local"
	}
	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 3000
	}

	// Validate tools
	if len(cfg.Tools) == 0 {
		return nil, fmt.Errorf("no tools defined in '%s'\n\n  Add at least one tool with either 'script' or 'http' configuration", path)
	}

	for i, tool := range cfg.Tools {
		if tool.Name == "" {
			return nil, fmt.Errorf("tool #%d is missing a name\n\n  Every tool needs a 'name' field", i+1)
		}
		hasScript := tool.Script.Command != "" || tool.Script.Shell != ""
		hasHTTP := tool.HTTP.URL != ""
		if !hasScript && !hasHTTP {
			return nil, fmt.Errorf("tool '%s' has no action defined\n\n  Add either:\n  - script.shell: \"your command\"\n  - script.command: \"/path/to/script\"\n  - http.url: \"https://api.example.com\"", tool.Name)
		}
		if hasScript && hasHTTP {
			return nil, fmt.Errorf("tool '%s' has both script and http defined\n\n  Use only one: either 'script' or 'http'", tool.Name)
		}

		// Validate HTTP config
		if hasHTTP {
			if tool.HTTP.Method == "" {
				cfg.Tools[i].HTTP.Method = "GET" // Default to GET
			}
		}

		// Validate parameters
		for j, param := range tool.Parameters {
			if param.Name == "" {
				return nil, fmt.Errorf("tool '%s' parameter #%d is missing a name", tool.Name, j+1)
			}
			if param.Type == "" {
				cfg.Tools[i].Parameters[j].Type = "string" // Default to string
			}
		}
	}

	return &cfg, nil
}

// GetTool returns a tool by name
func (c *Config) GetTool(name string) *Tool {
	for i := range c.Tools {
		if c.Tools[i].Name == name {
			return &c.Tools[i]
		}
	}
	return nil
}

// IsHTTP returns true if the tool uses HTTP configuration
func (t *Tool) IsHTTP() bool {
	return t.HTTP.URL != ""
}
