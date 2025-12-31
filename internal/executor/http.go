package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gantz-ai/gantz-cli/internal/config"
)

// HTTPExecutor runs HTTP requests for tools
type HTTPExecutor struct {
	client *http.Client
}

// NewHTTPExecutor creates a new HTTP executor
func NewHTTPExecutor() *HTTPExecutor {
	return &HTTPExecutor{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Execute makes an HTTP request for a tool
func (e *HTTPExecutor) Execute(ctx context.Context, tool *config.Tool, args map[string]interface{}) *Result {
	start := time.Now()

	// Parse timeout
	timeout := 30 * time.Second
	if tool.HTTP.Timeout != "" {
		if d, err := time.ParseDuration(tool.HTTP.Timeout); err == nil {
			timeout = d
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Expand URL with arguments
	url := expandArgs(tool.HTTP.URL, args)
	url = os.ExpandEnv(url)

	// Determine method (default to GET)
	method := tool.HTTP.Method
	if method == "" {
		method = "GET"
	}

	// Prepare body
	var bodyReader io.Reader
	if tool.HTTP.Body != "" {
		body := expandArgs(tool.HTTP.Body, args)
		body = os.ExpandEnv(body)
		bodyReader = strings.NewReader(body)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return &Result{
			Output:   fmt.Sprintf("Failed to create request: %v", err),
			ExitCode: -1,
			Duration: time.Since(start),
			Error:    err,
		}
	}

	// Set headers
	for key, value := range tool.HTTP.Headers {
		expandedValue := expandArgs(value, args)
		expandedValue = os.ExpandEnv(expandedValue)
		req.Header.Set(key, expandedValue)
	}

	// Set default Content-Type for POST/PUT/PATCH if body is present
	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Execute request
	resp, err := e.client.Do(req)
	if err != nil {
		return &Result{
			Output:   fmt.Sprintf("Request failed: %v", err),
			ExitCode: -1,
			Duration: time.Since(start),
			Error:    err,
		}
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &Result{
			Output:   fmt.Sprintf("Failed to read response: %v", err),
			ExitCode: -1,
			Duration: time.Since(start),
			Error:    err,
		}
	}

	output := string(body)

	// Extract JSON path if specified
	if tool.HTTP.ExtractJSON != "" && len(body) > 0 {
		extracted, err := extractJSONPath(body, tool.HTTP.ExtractJSON)
		if err == nil {
			output = extracted
		}
	}

	// Determine exit code based on status
	exitCode := 0
	if resp.StatusCode >= 400 {
		exitCode = 1
	}

	return &Result{
		Output:   strings.TrimSpace(output),
		ExitCode: exitCode,
		Duration: time.Since(start),
	}
}

// extractJSONPath extracts a value from JSON using a simple dot-notation path
// Supports: "data", "data.items", "data.items[0]", "data.items[0].name"
func extractJSONPath(data []byte, path string) (string, error) {
	var obj interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", err
	}

	// Parse path segments
	segments := parseJSONPath(path)

	current := obj
	for _, seg := range segments {
		switch v := current.(type) {
		case map[string]interface{}:
			if val, ok := v[seg.key]; ok {
				if seg.index >= 0 {
					if arr, ok := val.([]interface{}); ok && seg.index < len(arr) {
						current = arr[seg.index]
					} else {
						return "", fmt.Errorf("invalid array index: %d", seg.index)
					}
				} else {
					current = val
				}
			} else {
				return "", fmt.Errorf("key not found: %s", seg.key)
			}
		case []interface{}:
			if seg.index >= 0 && seg.index < len(v) {
				current = v[seg.index]
			} else {
				return "", fmt.Errorf("invalid array index: %d", seg.index)
			}
		default:
			return "", fmt.Errorf("cannot traverse into %T", current)
		}
	}

	// Convert result to string
	switch v := current.(type) {
	case string:
		return v, nil
	case nil:
		return "null", nil
	default:
		result, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", v), nil
		}
		return string(result), nil
	}
}

type pathSegment struct {
	key   string
	index int // -1 if not an array access
}

func parseJSONPath(path string) []pathSegment {
	var segments []pathSegment
	parts := strings.Split(path, ".")

	for _, part := range parts {
		// Check for array notation: key[0]
		if idx := strings.Index(part, "["); idx >= 0 {
			key := part[:idx]
			indexStr := strings.TrimSuffix(part[idx+1:], "]")
			var index int
			fmt.Sscanf(indexStr, "%d", &index)
			segments = append(segments, pathSegment{key: key, index: index})
		} else {
			segments = append(segments, pathSegment{key: part, index: -1})
		}
	}

	return segments
}
