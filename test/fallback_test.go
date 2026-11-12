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

func TestModelFallbackOn429(t *testing.T) {
	callCountServer1 := 0
	callCountServer2 := 0

	// Server 1 returns 429 Too Many Requests
	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCountServer1++
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"Rate limit exceeded","code":429}}`))
	}))
	defer s1.Close()

	// Server 2 returns 200 OK
	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCountServer2++
		w.Header().Set("Content-Type", "application/json")
		resp := llm.ChatResponse{
			ID: "resp-2",
			Choices: []llm.Choice{
				{
					Message: llm.Message{
						Role:    "assistant",
						Content: "Success from fallback!",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer s2.Close()

	cfg := &config.Config{
		Models: config.ModelsConfig{
			Selected: "p1/m1",
			Fallbacks: []string{"p2/m2"},
			Providers: map[string]config.ProviderConfig{
				"p1": {
					BaseURL: s1.URL,
					APIKey:  "key1",
					Models: []config.ModelConfig{
						{ID: "m1", Model: "model-one"},
					},
				},
				"p2": {
					BaseURL: s2.URL,
					APIKey:  "key2",
					Models: []config.ModelConfig{
						{ID: "m2", Model: "model-two"},
					},
				},
			},
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

	sessionID := "test-fallback-session"
	reply, err := a.Run(sessionID, nil, "chat1", "Hello")
	if err != nil {
		t.Fatalf("Expected success through fallback, got error: %v", err)
	}

	if reply != "Success from fallback!" {
		t.Fatalf("Unexpected reply: %q", reply)
	}

	if callCountServer1 != 1 {
		t.Errorf("Expected server 1 to be called once, got %d", callCountServer1)
	}
	if callCountServer2 != 1 {
		t.Errorf("Expected server 2 to be called once, got %d", callCountServer2)
	}
}
