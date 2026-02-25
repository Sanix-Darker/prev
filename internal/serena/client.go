package serena

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// SymbolInfo represents a code symbol (function, class, method) found by Serena.
type SymbolInfo struct {
	Name      string
	Kind      string // "function", "class", "method"
	FilePath  string
	StartLine int
	EndLine   int
	Content   string
}

// Client communicates with the Serena MCP server via JSON-RPC over stdio.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	reqID  int
}

// jsonRPCRequest is a JSON-RPC 2.0 request.
type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonRPCResponse is a JSON-RPC 2.0 response.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var (
	availableOnce   sync.Once
	availableResult bool
)

// IsAvailable checks if uvx and Serena are installed. Cached after first check.
func IsAvailable() bool {
	availableOnce.Do(func() {
		cmd := exec.Command("uvx", "--from", "git+https://github.com/oraios/serena", "serena", "--help")
		err := cmd.Run()
		availableResult = err == nil
	})
	return availableResult
}

// NewClient spawns a Serena MCP server subprocess and performs the MCP handshake.
// Returns nil and no error if Serena is not available (graceful fallback for "auto" mode).
// Returns error only if mode is "on" and Serena is not available.
func NewClient(mode string) (*Client, error) {
	if mode == "off" {
		return nil, nil
	}

	if !IsAvailable() {
		if mode == "on" {
			return nil, fmt.Errorf("serena is required (--serena=on) but unavailable; ensure uvx is installed, git is available, and the runner can reach github.com. For CI, install uv/uvx (for GitHub Actions: astral-sh/setup-uv) then run: uvx --from git+https://github.com/oraios/serena serena --help")
		}
		// mode == "auto": graceful fallback
		return nil, nil
	}

	cmd := exec.Command("uvx", "--from", "git+https://github.com/oraios/serena", "serena", "start-mcp-server")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start serena: %w", err)
	}

	c := &Client{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
		reqID:  0,
	}

	// MCP initialize handshake
	if err := c.initialize(); err != nil {
		c.Close()
		return nil, fmt.Errorf("serena handshake failed: %w", err)
	}

	return c, nil
}

func (c *Client) initialize() error {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]string{
			"name":    "prev",
			"version": "1.0.0",
		},
	}
	_, err := c.call("initialize", params)
	return err
}

func (c *Client) call(method string, params interface{}) (json.RawMessage, error) {
	c.reqID++
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      c.reqID,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Write request followed by newline
	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response line
	line, err := c.stdout.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("serena error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp.Result, nil
}

// FindEnclosingSymbol calls Serena's find_symbol to find the function/class
// enclosing the given line. Returns nil if no symbol is found.
func (c *Client) FindEnclosingSymbol(filePath string, line int) (*SymbolInfo, error) {
	params := map[string]interface{}{
		"name": "find_symbol",
		"arguments": map[string]interface{}{
			"file_path":   filePath,
			"line_number": line,
		},
	}

	result, err := c.call("tools/call", params)
	if err != nil {
		return nil, err
	}

	// Parse the Serena response
	var toolResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to parse symbol result: %w", err)
	}

	if len(toolResult.Content) == 0 {
		return nil, nil
	}

	// Try to parse the text content as symbol info
	var info SymbolInfo
	text := toolResult.Content[0].Text
	if err := json.Unmarshal([]byte(text), &info); err != nil {
		// If not JSON, treat the text as the symbol content directly
		info = SymbolInfo{
			FilePath: filePath,
			Content:  text,
		}
	}

	return &info, nil
}

// Close kills the Serena subprocess.
func (c *Client) Close() {
	if c == nil {
		return
	}
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}
}
