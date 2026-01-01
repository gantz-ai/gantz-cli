package tunnel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// MCPHandler handles MCP requests
type MCPHandler interface {
	HandleRequest(req *MCPRequest) (*MCPResponse, error)
}

// MCPRequest represents an incoming MCP request
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse represents an MCP response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ClientConnectedCallback is called when a new client connects to the tunnel
type ClientConnectedCallback func(clientIP string)

// Client manages the WebSocket tunnel connection
type Client struct {
	relayURL          string
	handler           MCPHandler
	conn              *websocket.Conn
	tunnelURL         string
	done              chan struct{}
	mu                sync.Mutex
	version           string
	toolCount         int
	onClientConnected ClientConnectedCallback
}

// NewClient creates a new tunnel client
func NewClient(relayURL string, handler MCPHandler, version string, toolCount int) *Client {
	return &Client{
		relayURL:  relayURL,
		handler:   handler,
		done:      make(chan struct{}),
		version:   version,
		toolCount: toolCount,
	}
}

// OnClientConnected sets the callback for when a client connects
func (c *Client) OnClientConnected(cb ClientConnectedCallback) {
	c.onClientConnected = cb
}

// TunnelMessage represents a message from/to the relay server
type TunnelMessage struct {
	Type      string          `json:"type"`
	TunnelID  string          `json:"tunnel_id,omitempty"`
	TunnelURL string          `json:"tunnel_url,omitempty"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Error     string          `json:"error,omitempty"`
	ClientIP  string          `json:"client_ip,omitempty"`
}

// Connect establishes a tunnel connection and returns the public URL
func (c *Client) Connect() (string, error) {
	header := http.Header{}
	header.Set("User-Agent", "gantz-cli/"+c.version)
	header.Set("X-Gantz-Version", c.version)
	header.Set("X-Gantz-Tool-Count", fmt.Sprintf("%d", c.toolCount))

	conn, resp, err := websocket.DefaultDialer.Dial(c.relayURL+"/tunnel", header)
	if err != nil {
		// Check if it's a version rejection (HTTP 426 Upgrade Required)
		if resp != nil && resp.StatusCode == http.StatusUpgradeRequired {
			return "", fmt.Errorf("version outdated - run: gantz update")
		}
		return "", fmt.Errorf("dial relay: %w", err)
	}

	c.conn = conn

	// Wait for tunnel registration response
	var msg TunnelMessage
	if err := conn.ReadJSON(&msg); err != nil {
		conn.Close()
		return "", fmt.Errorf("read registration: %w", err)
	}

	if msg.Type != "registered" {
		conn.Close()
		return "", fmt.Errorf("unexpected message type: %s", msg.Type)
	}

	c.tunnelURL = msg.TunnelURL

	// Start message handler
	go c.handleMessages()

	// Start ping/pong keepalive
	go c.keepalive()

	return c.tunnelURL, nil
}

func (c *Client) handleMessages() {
	defer close(c.done)

	for {
		var msg TunnelMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}
			fmt.Printf("Error reading message: %v\n", err)
			return
		}

		switch msg.Type {
		case "request":
			go c.handleRequest(msg)
		case "ping":
			c.sendPong()
		case "client_connected":
			if c.onClientConnected != nil && msg.ClientIP != "" {
				c.onClientConnected(msg.ClientIP)
			}
		}
	}
}

func (c *Client) handleRequest(msg TunnelMessage) {
	var req MCPRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		c.sendError(msg.RequestID, -32700, "Parse error")
		return
	}

	resp, err := c.handler.HandleRequest(&req)
	if err != nil {
		c.sendError(msg.RequestID, -32603, err.Error())
		return
	}

	c.sendResponse(msg.RequestID, resp)
}

func (c *Client) sendResponse(requestID string, resp *MCPResponse) {
	payload, _ := json.Marshal(resp)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.conn.WriteJSON(TunnelMessage{
		Type:      "response",
		RequestID: requestID,
		Payload:   payload,
	})
}

func (c *Client) sendError(requestID string, code int, message string) {
	resp := &MCPResponse{
		JSONRPC: "2.0",
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
	}
	c.sendResponse(requestID, resp)
}

func (c *Client) sendPong() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.conn.WriteJSON(TunnelMessage{Type: "pong"})
}

func (c *Client) keepalive() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Send JSON pong message to keep tunnel alive
			c.mu.Lock()
			err := c.conn.WriteJSON(TunnelMessage{Type: "pong"})
			c.mu.Unlock()
			if err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}

// Wait blocks until the tunnel is closed
func (c *Client) Wait() error {
	<-c.done
	return nil
}

// Close closes the tunnel connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// URL returns the public tunnel URL
func (c *Client) URL() string {
	return c.tunnelURL
}
