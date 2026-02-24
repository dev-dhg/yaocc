package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Minimal JSON-RPC structures
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// MCP Specific Structures
type InitializeRequestParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation     `json:"clientInfo"`
}

type ClientCapabilities struct{}

type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
}

type ServerCapabilities struct{}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"inputSchema"`
}

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

type CallToolRequestParams struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
}

type CallToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

type Content struct {
	Type string `json:"type"` // "text", "image"
	Text string `json:"text,omitempty"`
	// ignoring base64/data URI for now
}

// Client represents a connection to a specific MCP server via stdio.
type Client struct {
	name   string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	nextID     int64
	pending    map[int64]chan Response
	pendingMut sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewClient creates and starts an MCP server process.
func NewClient(name string, command string, args []string, env map[string]string) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, command, args...)

	// Setup environment
	if len(env) > 0 {
		importEnv := cmd.Environ()
		for k, v := range env {
			importEnv = append(importEnv, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = importEnv
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get stdin: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get stdout: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start MCP server %s: %w", name, err)
	}

	c := &Client{
		name:    name,
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		pending: make(map[int64]chan Response),
		ctx:     ctx,
		cancel:  cancel,
	}

	// Start reading loops
	c.wg.Add(2)
	go c.readStdout()
	go c.readStderr()

	// Wait for process to exit
	go func() {
		cmd.Wait()
		c.Close()
	}()

	return c, nil
}

// Close shuts down the client and the underlying process.
func (c *Client) Close() {
	if c.cancel != nil {
		c.cancel()
	}
	if c.stdin != nil {
		c.stdin.Close()
	}
	// Wait doesn't need to block everything if already killed, but good practice
	c.wg.Wait()
}

func (c *Client) readStderr() {
	defer c.wg.Done()
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		log.Printf("[MCP %s STDERR] %s", c.name, scanner.Text())
	}
}

func (c *Client) readStdout() {
	defer c.wg.Done()
	scanner := bufio.NewScanner(c.stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Only parse lines that look like JSON objects
		if len(line) == 0 || (line[0] != '{' && line[0] != '[') {
			log.Printf("[MCP %s STDOUT text] %s", c.name, string(line))
			continue
		}

		var resp Response
		if err := json.Unmarshal(line, &resp); err == nil && resp.JSONRPC == "2.0" {
			if resp.ID != 0 {
				c.pendingMut.Lock()
				ch, ok := c.pending[resp.ID]
				if ok {
					delete(c.pending, resp.ID)
				}
				c.pendingMut.Unlock()

				if ok {
					ch <- resp
				}
			} else if resp.Method != "" {
				// It's a notification from the server
				log.Printf("[MCP %s Notification] Method: %s", c.name, resp.Method)
			} else {
				log.Printf("[MCP %s Notification/Unknown] %s", c.name, string(line))
			}
		} else {
			log.Printf("[MCP %s STDOUT non-rpc payload] %s", c.name, string(line))
		}
	}
}

// Request sends a JSON-RPC request and waits for a response.
func (c *Client) Request(method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.nextID, 1)

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	data = append(data, '\n') // JSON-RPC over stdio requires newline separation

	ch := make(chan Response, 1)
	c.pendingMut.Lock()
	c.pending[id] = ch
	c.pendingMut.Unlock()

	if _, err := c.stdin.Write(data); err != nil {
		c.pendingMut.Lock()
		delete(c.pending, id)
		c.pendingMut.Unlock()
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	select {
	case <-c.ctx.Done():
		return nil, fmt.Errorf("client closed")
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

// Notify sends a JSON-RPC notification (no response expected).
func (c *Client) Notify(method string, params interface{}) error {
	notif := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	data, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}
	data = append(data, '\n')

	_, err = c.stdin.Write(data)
	return err
}

// Initialize performs the mandatory MCP initialization handshake.
func (c *Client) Initialize() (*InitializeResult, error) {
	params := InitializeRequestParams{
		ProtocolVersion: "2024-11-05", // Latest MCP protocol version usually
		ClientInfo: Implementation{
			Name:    "yaocc",
			Version: "1.0.0",
		},
		Capabilities: ClientCapabilities{},
	}

	resultRaw, err := c.Request("initialize", params)
	if err != nil {
		return nil, err
	}

	var res InitializeResult
	if err := json.Unmarshal(resultRaw, &res); err != nil {
		return nil, fmt.Errorf("failed to parse initialize result: %w", err)
	}

	// Send initialized notification
	err = c.Notify("notifications/initialized", map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("failed to send initialized notification: %w", err)
	}

	return &res, nil
}

// GetTools fetches the available tools from the MCP server.
func (c *Client) GetTools() ([]Tool, error) {
	resultRaw, err := c.Request("tools/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	var res ListToolsResult
	if err := json.Unmarshal(resultRaw, &res); err != nil {
		return nil, fmt.Errorf("failed to parse tools/list result: %w", err)
	}

	return res.Tools, nil
}

// CallTool executes a tool on the MCP server.
func (c *Client) CallTool(name string, args interface{}) (*CallToolResult, error) {
	params := CallToolRequestParams{
		Name:      name,
		Arguments: args,
	}

	resultRaw, err := c.Request("tools/call", params)
	if err != nil {
		return nil, err
	}

	var res CallToolResult
	if err := json.Unmarshal(resultRaw, &res); err != nil {
		return nil, fmt.Errorf("failed to parse tools/call result: %w", err)
	}

	return &res, nil
}
