package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dev-dhg/yaocc/pkg/agent"
	"github.com/dev-dhg/yaocc/pkg/config"
	"github.com/dev-dhg/yaocc/pkg/llm"
)

func TestGeminiThoughtSignaturePreservation(t *testing.T) {
	turn := 0
	var receivedThoughtSig string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		turn++
		w.Header().Set("Content-Type", "application/json")

		if turn == 1 {
			// Turn 1: Model issues a tool call with extra_content.google.thought_signature
			thoughtData := map[string]interface{}{
				"google": map[string]interface{}{
					"thought_signature": "mock_encrypted_thought_signature_xyz_123",
				},
			}
			resp := llm.ChatResponse{
				ID: "resp-turn-1",
				Choices: []llm.Choice{
					{
						Message: llm.Message{
							Role: "assistant",
							ToolCalls: []llm.ToolCall{
								{
									ID:   "call_gemini_1",
									Type: "function",
									Function: llm.FunctionCall{
										Name:      "yaocc_exec",
										Arguments: `{"command":"echo test"}`,
									},
									ExtraContent: thoughtData,
								},
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Turn 2: Inspect request messages to ensure extra_content.google.thought_signature was preserved verbatim
		var req llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		for _, msg := range req.Messages {
			if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
				tc := msg.ToolCalls[0]
				if tc.ExtraContent != nil {
					if extraMap, ok := tc.ExtraContent.(map[string]interface{}); ok {
						if googleMap, ok := extraMap["google"].(map[string]interface{}); ok {
							if sig, ok := googleMap["thought_signature"].(string); ok {
								receivedThoughtSig = sig
							}
						}
					}
				}
			}
		}

		// Send final assistant message
		finalResp := llm.ChatResponse{
			ID: "resp-turn-2",
			Choices: []llm.Choice{
				{
					Message: llm.Message{
						Role:    "assistant",
						Content: "Finished tool execution.",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(finalResp)
	}))
	defer server.Close()

	useNativeTools := true
	cfg := &config.Config{
		UseNativeToolCalling: true,
		Models: config.ModelsConfig{
			Selected: "gemini/gemini-3.1-flash-lite",
			Providers: map[string]config.ProviderConfig{
				"gemini": {
					BaseURL: server.URL,
					APIKey:  "dummy-key",
					Models: []config.ModelConfig{
						{
							ID:    "gemini-3.1-flash-lite",
							Model: "gemini-3.1-flash-lite",
							Tool:  &useNativeTools,
						},
					},
				},
			},
		},
		Cmds: []config.CmdConfig{
			{Name: "exec", Enabled: true},
		},
		Session: config.SessionConfig{
			HistoryLimit: 20,
		},
	}

	tempDir := t.TempDir()
	a, err := agent.NewAgent(cfg, tempDir, false, "")
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	defer a.Close()

	reply, err := a.Run("session-gemini-test", nil, "chat1", "Run echo test")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if reply != "Finished tool execution." {
		t.Errorf("Unexpected reply: %q", reply)
	}

	if receivedThoughtSig != "mock_encrypted_thought_signature_xyz_123" {
		t.Fatalf("Expected thought_signature 'mock_encrypted_thought_signature_xyz_123' to be preserved, got %q", receivedThoughtSig)
	}
}
