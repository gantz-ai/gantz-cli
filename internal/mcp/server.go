package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gantz-ai/gantz-cli/internal/config"
	"github.com/gantz-ai/gantz-cli/internal/executor"
	"github.com/gantz-ai/gantz-cli/internal/tunnel"
)

// Server implements MCP protocol handler
type Server struct {
	config       *config.Config
	executor     *executor.Executor
	httpExecutor *executor.HTTPExecutor
}

// NewServer creates a new MCP server
func NewServer(cfg *config.Config) *Server {
	return &Server{
		config:       cfg,
		executor:     executor.NewExecutor(),
		httpExecutor: executor.NewHTTPExecutor(),
	}
}

// HandleRequest processes an MCP request (implements tunnel.MCPHandler)
func (s *Server) HandleRequest(req *tunnel.MCPRequest) (*tunnel.MCPResponse, error) {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "ping":
		return s.handlePing(req)
	default:
		return &tunnel.MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &tunnel.MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}, nil
	}
}

func (s *Server) handleInitialize(req *tunnel.MCPRequest) (*tunnel.MCPResponse, error) {
	return &tunnel.MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]interface{}{
				"name":    s.config.Name,
				"version": s.config.Version,
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
		},
	}, nil
}

func (s *Server) handleToolsList(req *tunnel.MCPRequest) (*tunnel.MCPResponse, error) {
	tools := make([]map[string]interface{}, 0, len(s.config.Tools))

	for _, tool := range s.config.Tools {
		// Build JSON Schema for parameters
		properties := make(map[string]interface{})
		required := []string{}

		for _, param := range tool.Parameters {
			prop := map[string]interface{}{
				"type":        param.Type,
				"description": param.Description,
			}
			if param.Default != "" {
				prop["default"] = param.Default
			}
			properties[param.Name] = prop

			if param.Required {
				required = append(required, param.Name)
			}
		}

		inputSchema := map[string]interface{}{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			inputSchema["required"] = required
		}

		tools = append(tools, map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": inputSchema,
		})
	}

	return &tunnel.MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}, nil
}

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

func (s *Server) handleToolsCall(req *tunnel.MCPRequest) (*tunnel.MCPResponse, error) {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &tunnel.MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &tunnel.MCPError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}, nil
	}

	// Find tool
	tool := s.config.GetTool(params.Name)
	if tool == nil {
		return &tunnel.MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &tunnel.MCPError{
				Code:    -32602,
				Message: fmt.Sprintf("Tool not found: %s", params.Name),
			},
		}, nil
	}

	// Execute tool
	fmt.Printf("  → Executing tool: %s\n", params.Name)

	var result *executor.Result
	if tool.IsHTTP() {
		result = s.httpExecutor.Execute(context.Background(), tool, params.Arguments)
	} else {
		result = s.executor.Execute(context.Background(), tool, params.Arguments)
	}

	fmt.Printf("  ← Completed in %v (exit=%d)\n", result.Duration, result.ExitCode)

	// Build response content
	content := []map[string]interface{}{}

	if result.Error != nil && result.Output == "" {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": fmt.Sprintf("Error: %v", result.Error),
		})
	} else {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": result.Output,
		})
	}

	return &tunnel.MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": content,
			"isError": result.ExitCode != 0,
		},
	}, nil
}

func (s *Server) handlePing(req *tunnel.MCPRequest) (*tunnel.MCPResponse, error) {
	return &tunnel.MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{},
	}, nil
}

// ListenAndServe starts HTTP server for local mode
func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()

	// SSE endpoint for MCP
	mux.HandleFunc("/sse", s.handleSSE)

	// Simple JSON-RPC endpoint
	mux.HandleFunc("/mcp", s.handleHTTP)

	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req tunnel.MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	resp, _ := s.HandleRequest(&req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send endpoint info
	fmt.Fprintf(w, "event: endpoint\ndata: /mcp\n\n")
	flusher.Flush()

	// Keep connection open
	<-r.Context().Done()
}
