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
	Environment map[string]string `yaml:"environment"`
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
		return nil, fmt.Errorf("read file: %w", err)
	}

	// Expand environment variables
	data = []byte(os.ExpandEnv(string(data)))

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
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
	for i, tool := range cfg.Tools {
		if tool.Name == "" {
			return nil, fmt.Errorf("tool %d: name is required", i)
		}
		if tool.Script.Command == "" && tool.Script.Shell == "" {
			return nil, fmt.Errorf("tool %s: script.command or script.shell is required", tool.Name)
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
