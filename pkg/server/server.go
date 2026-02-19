package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/dev-dhg/yaocc/pkg/agent"
	"github.com/dev-dhg/yaocc/pkg/config"
	"github.com/dev-dhg/yaocc/pkg/cron"
	"github.com/dev-dhg/yaocc/pkg/exec"
	"github.com/dev-dhg/yaocc/pkg/messaging"
)

//go:embed openapi.yaml openapi.html
var openAPIFile embed.FS

type Server struct {
	Config    *config.Config
	Agent     *agent.Agent
	Providers map[string]messaging.Provider
	Scheduler *cron.Scheduler
}

func NewServer(cfg *config.Config, agt *agent.Agent, providers map[string]messaging.Provider, scheduler *cron.Scheduler) *Server {
	return &Server{
		Config:    cfg,
		Agent:     agt,
		Providers: providers,
		Scheduler: scheduler,
	}
}

func (s *Server) Run() error {
	mux := http.NewServeMux()

	// API Endpoints
	mux.HandleFunc("/chat", s.handleChat)
	mux.HandleFunc("/exec", s.handleExec)
	mux.HandleFunc("/cron/run", s.handleCronRun)

	// OpenAPI Documentation
	mux.Handle("/openapi.yaml", http.FileServer(http.FS(openAPIFile)))
	mux.HandleFunc("/docs", s.handleSwaggerUI)

	addr := fmt.Sprintf(":%d", s.Config.Server.Port)
	log.Printf("Server listening on %s", addr)
	log.Printf("OpenAPI Docs available at http://localhost:%d/docs", s.Config.Server.Port)

	return http.ListenAndServe(addr, mux)
}

type ChatRequest struct {
	SessionID string `json:"sessionId,omitempty"`
	Provider  string `json:"provider,omitempty"`
	ChatID    string `json:"chatId,omitempty"`
	Message   string `json:"message"`
}

type ChatResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Default to "local" provider if not specified
	provider := req.Provider
	if provider == "" {
		provider = "local"
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = "general"
	}

	// Default ChatID to SessionID if not specified (for backward compatibility / convenience)
	chatID := req.ChatID
	if chatID == "" {
		chatID = sessionID
	}

	// If SessionID is default ("general") BUT provider and chatID are set,
	// construct a session ID from them to avoid collision with general session.
	if sessionID == "general" && provider != "local" && chatID != "" {
		sessionID = fmt.Sprintf("%s-%s", provider, chatID)
	}

	var providerObj messaging.Provider
	if s.Providers != nil {
		providerObj = s.Providers[provider]
	}

	response, err := s.Agent.Run(sessionID, providerObj, chatID, req.Message)
	if err != nil {
		log.Printf("Agent error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{Error: err.Error()})
		return
	}

	// If provider is NOT local, we should also send the response to the provider
	// This helps in simulation scenarios where we want the actual provider to send the message
	if provider != "local" && s.Providers != nil {
		if prov, ok := s.Providers[provider]; ok {
			log.Printf("Injecting response to provider %s (ID: %s)", provider, chatID)
			go func() {
				// Send async to not block API response
				if err := prov.SendMessage(chatID, response); err != nil {
					log.Printf("Error sending injected message to %s: %v", provider, err)
				}
			}()
		} else {
			log.Printf("Warning: Provider '%s' not found for injection", provider)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ChatResponse{Response: response})
}

func (s *Server) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	html, err := openAPIFile.ReadFile("openapi.html")
	if err != nil {
		log.Printf("Error reading openapi.html: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(html)
}

type CronRunRequest struct {
	Index int `json:"index"`
}

func (s *Server) handleCronRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CronRunRequest

	// Support both JSON body and query parameter
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
	} else {
		// Try query parameter
		indexStr := r.URL.Query().Get("index")
		if indexStr != "" {
			idx, err := strconv.Atoi(indexStr)
			if err != nil {
				http.Error(w, "Invalid index parameter", http.StatusBadRequest)
				return
			}
			req.Index = idx
		}
	}

	if s.Scheduler == nil {
		http.Error(w, "Scheduler not available", http.StatusServiceUnavailable)
		return
	}

	if req.Index < 0 || req.Index >= len(s.Config.Cron) {
		http.Error(w, fmt.Sprintf("Invalid job index: %d (available: 0-%d)", req.Index, len(s.Config.Cron)-1), http.StatusBadRequest)
		return
	}

	job := s.Config.Cron[req.Index]
	log.Printf("Manually triggered cron job: %s (index: %d)", job.Name, req.Index)

	// Run asynchronously so the API returns immediately
	go s.Scheduler.RunJob(job)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "accepted",
		"job":    job.Name,
	})
}

type ExecRequest struct {
	Command string `json:"command"`
}

type ExecResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Check if Enabled
	if !s.Config.IsCmdEnabled("exec") {
		http.Error(w, "Exec command is disabled", http.StatusForbidden)
		return
	}

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Command == "" {
		http.Error(w, "Command is required", http.StatusBadRequest)
		return
	}

	// 2. Validate Security
	cmdConfig := s.Config.GetCmdConfig("exec")
	var options *config.CmdOptions
	if cmdConfig != nil {
		options = cmdConfig.Options
	}

	if err := exec.ValidateCommand(req.Command, options); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(ExecResponse{Error: err.Error()})
		return
	}

	// 3. Execute
	// We use the config directory from the Agent (if available) or resolve it again?
	// The Agent has configDir stored. Server has Agent.
	// But Agent.ConfigDir() is what we need.
	// Let's modify Agent to export ConfigDir() publicly or just access it if field is public?
	// Agent struct field `configDir` is lowercase (private).
	// But `NewAgent` sets it.
	// I saw `a.ConfigDir()` method in agent.go earlier. Let's start by using that if public.
	// Checking agent.go... yes, I saw `func (a *Agent) ConfigDir() string`.

	configDir := s.Agent.ConfigDir()

	output, err := exec.RunCommand(req.Command, configDir)

	resp := ExecResponse{Output: output}
	if err != nil {
		resp.Error = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
