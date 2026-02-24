package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dev-dhg/yaocc/pkg/config"
)

type Client struct {
	BaseURL    string
	APIKey     string
	Model      string
	MaxTokens  int
	Reasoning  interface{}
	HTTPClient *http.Client
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"` // Used sometimes for tool responses
}

type Tool struct {
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"` // JSON Schema object
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

type ChatRequest struct {
	Model     string      `json:"model"`
	Messages  []Message   `json:"messages"`
	Tools     []Tool      `json:"tools,omitempty"`
	MaxTokens int         `json:"max_tokens,omitempty"`
	Stream    bool        `json:"stream"`
	Reasoning interface{} `json:"reasoning,omitempty"`
	// OpenRouter specific
	Transforms []string `json:"transforms,omitempty"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message Message `json:"message"`
}

func NewClient(cfg config.ProviderConfig, selectedModel string) *Client {
	timeout := 30 * time.Second
	if cfg.TimeoutMs > 0 {
		timeout = time.Duration(cfg.TimeoutMs) * time.Millisecond
	}

	// Find MaxTokens, Reasoning, and Timeout if defined in config for this model
	maxTokens := 0
	var reasoning interface{}
	modelTimeout := timeout // Default to provider timeout

	for _, m := range cfg.Models {
		// Matches either the ID or the Model name
		if m.Model == selectedModel || m.ID == selectedModel {
			if m.MaxTokens > 0 {
				maxTokens = m.MaxTokens
			}
			if m.Reasoning != nil {
				reasoning = m.Reasoning
			}
			if m.TimeoutMs > 0 {
				modelTimeout = time.Duration(m.TimeoutMs) * time.Millisecond
			}
			break
		}
	}

	return &Client{
		BaseURL:   cfg.BaseURL,
		APIKey:    cfg.APIKey,
		Model:     selectedModel,
		MaxTokens: maxTokens,
		Reasoning: reasoning,
		HTTPClient: &http.Client{
			Timeout: modelTimeout,
		},
	}
}

// Chat sends messages and optional tools. Returns the text response, any tool calls, and error.
func (c *Client) Chat(messages []Message, tools []Tool) (string, []ToolCall, error) {
	reqBody := ChatRequest{
		Model:     c.Model,
		Messages:  messages,
		Tools:     tools,
		MaxTokens: c.MaxTokens,
		Stream:    false,
	}

	if c.Reasoning != nil {
		switch v := c.Reasoning.(type) {
		case bool:
			if v {
				// Pass reasoning parameter. Using "effort": "medium" as a reasonable default/standard object
				// for APIs that support this (like OpenRouter).
				reqBody.Reasoning = map[string]interface{}{
					"effort": "medium",
				}
			}
		default:
			// Pass the object directly (e.g. map[string]interface{})
			reqBody.Reasoning = v
		}
	}

	// OpenRouter Enhancement
	if fmt.Sprintf("%s", c.BaseURL) != "" && (strings.Contains(c.BaseURL, "openrouter") || strings.Contains(c.BaseURL, "openrouter.ai")) {
		reqBody.Transforms = []string{"middle-out"}
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to send request to %s (Model: %s): %w", c.BaseURL, c.Model, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("API request to %s failed with status %d (Model: %s): %s", c.BaseURL, resp.StatusCode, c.Model, string(body))
	}

	type ErrorResponse struct {
		Error struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}

	// Read body for debugging/error checking
	bodyBytes, _ := io.ReadAll(resp.Body)
	// Restore body for decoding
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Check for API Error
	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Error.Message != "" {
		return "", nil, fmt.Errorf("API Error from %s (Model: %s): %s (Code: %d)", c.BaseURL, c.Model, errResp.Error.Message, errResp.Error.Code)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", nil, fmt.Errorf("no choices returned from %s (Model: %s). Raw response: %s", c.BaseURL, c.Model, string(bodyBytes))
	}

	return chatResp.Choices[0].Message.Content, chatResp.Choices[0].Message.ToolCalls, nil
}
