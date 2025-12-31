# Gantz CLI

A command-line tool that turns your local scripts into MCP (Model Context Protocol) servers, allowing AI agents like Claude to execute them securely via cloud tunneling.

## Overview

Gantz CLI lets you define custom tools in a simple YAML file and expose them to AI agents. When you run `gantz serve`, it:

1. Creates a local MCP server from your `gantz.yaml` configuration
2. Connects to the [Gantz Relay](https://github.com/gantz-ai/gantz-relay) server via WebSocket
3. Provides you with a public tunnel URL that AI agents can connect to

```
┌─────────────┐         ┌──────────────┐         ┌─────────────┐
│  AI Agent   │  HTTPS  │ Gantz Relay  │   WSS   │  Gantz CLI  │
│  (Claude)   │────────►│              │◄───────►│  (you)      │
└─────────────┘         └──────────────┘         └─────────────┘
                                                       │
                                                       ▼
                                                 Local Scripts
                                                 & Commands
```

## Features

- **Simple YAML Configuration**: Define tools with parameters, scripts, and descriptions
- **Cloud Tunneling**: Automatic secure tunnel via `gantz.run`
- **HTTP Tools**: Call REST APIs with headers, body, and JSON extraction
- **Parameter Substitution**: Use `{{param}}` placeholders in scripts
- **Environment Variables**: Set tool-specific environment variables
- **Timeout Control**: Configure execution timeouts per tool
- **Cross-Platform**: Works on macOS, Linux, and Windows

## Installation

### Quick Install (Recommended)

```bash
curl -fsSL https://gantz.run/install.sh | sh
```

### Homebrew (macOS/Linux)

```bash
brew install gantz-ai/tap/gantz
```

### Go Install

```bash
go install github.com/gantz-ai/gantz-cli/cmd/gantz@latest
```

### Build from Source

```bash
git clone https://github.com/gantz-ai/gantz-cli.git
cd gantz-cli
make build && make install
```

## Quick Start

### 1. Create a Configuration File

Create a `gantz.yaml` file in your project directory:

```yaml
name: my-tools
description: My custom AI tools
version: "1.0.0"

server:
  port: 3000

tools:
  - name: hello
    description: Say hello to someone
    parameters:
      - name: name
        type: string
        description: The name to greet
        required: true
    script:
      shell: echo "Hello, {{name}}!"

  - name: list_files
    description: List files in a directory
    parameters:
      - name: path
        type: string
        description: Directory path to list
        required: true
        default: "."
    script:
      shell: ls -la {{path}}
```

### 2. Start the Server

```bash
gantz serve
```

Output:
```
Gantz CLI v0.1.0
Loaded 2 tools from gantz.yaml

Connecting to relay server...

  MCP Server URL: https://abc12345.gantz.run

  Add to Claude Desktop config:
  {
    "mcpServers": {
      "my-tools": {
        "url": "https://abc12345.gantz.run"
      }
    }
  }

Press Ctrl+C to stop
```

### 3. Connect from Claude Desktop

Add the MCP server URL to your Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "my-tools": {
      "url": "https://abc12345.gantz.run"
    }
  }
}
```

Restart Claude Desktop, and your tools will be available!

## Configuration Reference

### Root Configuration

```yaml
name: string          # Server name (default: "gantz-local")
description: string   # Server description
version: string       # Version (default: "1.0.0")

server:
  port: number        # Local server port (default: 3000)

tools:                # List of tool definitions
  - ...
```

### Tool Definition

```yaml
tools:
  - name: string              # Tool name (required, alphanumeric + underscore)
    description: string       # Description shown to AI (required)
    parameters:               # Input parameters
      - name: string          # Parameter name
        type: string          # Type: string, number, boolean, array, object
        description: string   # Description for the AI
        required: boolean     # Is this parameter required?
        default: string       # Default value if not provided
    script:
      shell: string           # Shell command with {{param}} placeholders
      # OR
      command: string         # Executable path
      args: [string]          # Arguments with {{param}} placeholders
      working_dir: string     # Working directory
      timeout: string         # Execution timeout (e.g., "30s", "1m")
    environment:              # Environment variables
      KEY: value
```

### Script Execution

You can define scripts in two ways:

**Shell Command** (simple):
```yaml
script:
  shell: echo "Hello, {{name}}!"
```

**Command with Args** (more control):
```yaml
script:
  command: /usr/bin/python3
  args:
    - "-c"
    - "print('Hello, {{name}}')"
  working_dir: /path/to/project
  timeout: "60s"
```

### Parameter Substitution

Use `{{parameter_name}}` placeholders in your scripts:

```yaml
tools:
  - name: search
    parameters:
      - name: query
        type: string
        required: true
      - name: limit
        type: number
        default: "10"
    script:
      shell: grep -r "{{query}}" . | head -n {{limit}}
```

### Environment Variables

Set tool-specific environment variables:

```yaml
tools:
  - name: api_call
    parameters:
      - name: endpoint
        type: string
    environment:
      API_KEY: ${MY_API_KEY}
      DEBUG: "true"
    script:
      shell: curl -H "Authorization: $API_KEY" {{endpoint}}
```

Note: Config values support `${ENV_VAR}` expansion.

## CLI Reference

### `gantz serve`

Start the MCP server.

```bash
gantz serve [flags]
```

**Flags:**
| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | `gantz.yaml` | Path to config file |
| `--relay` | | `wss://relay.gantz.run` | Relay server URL |

**Examples:**
```bash
# Start with default config
gantz serve

# Use custom config file
gantz serve -c my-tools.yaml
```

### `gantz version`

Print version information.

```bash
gantz version
```

## Examples

### File Operations

```yaml
tools:
  - name: read_file
    description: Read contents of a file
    parameters:
      - name: path
        type: string
        description: File path to read
        required: true
    script:
      shell: cat "{{path}}"

  - name: write_file
    description: Write content to a file
    parameters:
      - name: path
        type: string
        required: true
      - name: content
        type: string
        required: true
    script:
      shell: echo "{{content}}" > "{{path}}"
```

### Git Operations

```yaml
tools:
  - name: git_status
    description: Get git repository status
    script:
      shell: git status

  - name: git_log
    description: Show recent git commits
    parameters:
      - name: count
        type: number
        default: "5"
    script:
      shell: git log --oneline -n {{count}}
```

### Python Integration

```yaml
tools:
  - name: run_python
    description: Execute Python code
    parameters:
      - name: code
        type: string
        description: Python code to execute
        required: true
    script:
      command: python3
      args: ["-c", "{{code}}"]
      timeout: "30s"

  - name: analyze_data
    description: Analyze a CSV file
    parameters:
      - name: file
        type: string
        required: true
    script:
      shell: python3 analyze.py "{{file}}"
      working_dir: /path/to/scripts
```

### API Calls

```yaml
tools:
  - name: weather
    description: Get weather for a city
    parameters:
      - name: city
        type: string
        required: true
    environment:
      API_KEY: ${WEATHER_API_KEY}
    script:
      shell: |
        curl -s "https://api.weather.com/v1/current?city={{city}}&key=$API_KEY" | jq .
```

## Architecture

### Project Structure

```
gantz-cli/
├── cmd/
│   └── gantz/
│       └── main.go           # CLI entry point, Cobra commands
├── internal/
│   ├── config/
│   │   └── config.go         # YAML config parsing
│   ├── mcp/
│   │   └── server.go         # MCP protocol server
│   ├── executor/
│   │   └── script.go         # Script execution engine
│   └── tunnel/
│       └── client.go         # WebSocket tunnel client
├── bin/                      # Built binaries
├── Makefile
└── README.md
```

### MCP Protocol

Gantz CLI implements the [Model Context Protocol](https://spec.modelcontextprotocol.io/) with the following methods:

| Method | Description |
|--------|-------------|
| `initialize` | Returns server info and capabilities |
| `tools/list` | Returns available tools with JSON schemas |
| `tools/call` | Executes a tool with provided arguments |
| `ping` | Keepalive mechanism |

## Development

### Build Commands

```bash
make build        # Build for current platform
make build-all    # Build for all platforms
make run          # Build and run (tunnel mode)
make run-local    # Build and run (local mode)
make test         # Run tests
make clean        # Clean build artifacts
```

### Testing Your Server

After starting `gantz serve`, test with curl using your tunnel URL:

```bash
# List tools
curl -s -X POST https://YOUR-TUNNEL.gantz.run \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq

# Call a tool
curl -s -X POST https://YOUR-TUNNEL.gantz.run \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"hello","arguments":{"name":"World"}}}' | jq
```

## Security Considerations

- **Script Execution**: Tools execute shell commands on your machine. Only define tools you trust.
- **Parameter Injection**: Parameters are substituted directly into scripts. Be careful with user-controlled input.
- **Tunnel Access**: Anyone with your tunnel URL can call your tools. Keep URLs private.
- **Environment Variables**: Sensitive values in config are visible in the file. Use `${ENV_VAR}` expansion.

## Troubleshooting

### Connection Issues

```
Error: connect tunnel: websocket: bad handshake
```
- Check your internet connection
- Verify the relay server is running
- Try a different relay URL

### Config Errors

```
Error: load config: tool my-tool: script.command or script.shell is required
```
- Ensure every tool has either `script.shell` or `script.command` defined

### Tool Execution Fails

- Check the script works in your terminal first
- Verify parameter names match between `parameters` and `{{placeholders}}`
- Check `working_dir` exists if specified

## Links

- [Gantz Portal](https://gantz.ai) - Web platform for API-to-MCP conversion
- [Mock Server](https://mockserver.gantz.ai) - Test API for demos
- [Documentation](https://gantz.run) - Homepage and docs

## License

MIT License - see [LICENSE](LICENSE) for details.
