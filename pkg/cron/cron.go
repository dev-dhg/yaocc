package cron

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dev-dhg/yaocc/pkg/agent"
	"github.com/dev-dhg/yaocc/pkg/config"
	"github.com/dev-dhg/yaocc/pkg/llm"
	"github.com/dev-dhg/yaocc/pkg/messaging"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	Config    *config.Config
	ConfigDir string
	Agent     *agent.Agent
	Providers map[string]messaging.Provider
	Cron      *cron.Cron
	Quit      chan struct{}
}

func NewScheduler(cfg *config.Config, configDir string, a *agent.Agent, providers map[string]messaging.Provider) *Scheduler {
	var opts []cron.Option
	if cfg.Timezone != "" {
		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			log.Printf("Error loading timezone '%s': %v. Falling back to local time.", cfg.Timezone, err)
		} else {
			opts = append(opts, cron.WithLocation(loc))
			log.Printf("Cron scheduler using timezone: %s", cfg.Timezone)
		}
	}

	return &Scheduler{
		Config:    cfg,
		ConfigDir: configDir,
		Agent:     a,
		Providers: providers,
		Cron:      cron.New(opts...),
		Quit:      make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	// Cron Jobs
	for _, job := range s.Config.Cron {
		jobCopy := job // Capture for closure
		_, err := s.Cron.AddFunc(job.Schedule, func() {
			s.RunJob(jobCopy)
		})
		if err != nil {
			log.Printf("Error scheduling job %s: %v", job.Name, err)
		} else {
			log.Printf("Scheduled cron job: %s (%s)", job.Name, job.Schedule)
		}
	}

	s.Cron.Start()
}

func (s *Scheduler) Stop() {
	close(s.Quit)
	s.Cron.Stop()
}

func (s *Scheduler) Reload(newCfg *config.Config) {
	s.Stop()
	s.Config = newCfg

	// Re-initialize cron with new timezone if applicable
	var opts []cron.Option
	if s.Config.Timezone != "" {
		loc, err := time.LoadLocation(s.Config.Timezone)
		if err != nil {
			log.Printf("Error loading timezone '%s': %v. Falling back to local time.", s.Config.Timezone, err)
		} else {
			opts = append(opts, cron.WithLocation(loc))
			log.Printf("Cron scheduler using timezone: %s", s.Config.Timezone)
		}
	}
	s.Cron = cron.New(opts...)
	s.Quit = make(chan struct{})
	s.Start()
	log.Println("Scheduler reloaded.")
}

// RunJob executes a cron job immediately. Exported for use by the server API.
func (s *Scheduler) RunJob(job config.CronJob) {
	log.Printf("Running job: %s (Type: %s)", job.Name, job.Type)

	var output string
	var err error

	// 1. Execute Script if present
	if job.Script != "" {
		output, err = executeScript(s.ConfigDir, job.Script)
		if err != nil {
			log.Printf("Job %s failed execution: %v. Output: %s", job.Name, err, output)
			// Decide: do we want to notify anyway? Maybe only if prompt is present?
			// For now, let's treat script failure as meaningful output if there is any.
			if output == "" {
				output = fmt.Sprintf("Script Execution Failed: %v", err)
			}
		}
	}

	// 2. Determine Action based on Job Configuration
	// Default targets if none specified
	targets := job.Targets
	if len(targets) == 0 {
		sid := job.SessionID
		if sid == "" {
			sid = "general"
		}
		targets = []config.CronTarget{{Provider: "local", ID: sid}}
	}

	// ACTION A: Script ONLY (No Prompt) -> Send Raw Output
	if job.Script != "" && job.Prompt == "" {
		if output == "" {
			return // Nothing to say
		}
		contextMsg := fmt.Sprintf("SYSTEM REPORT [%s]:\n%s", job.Name, output)

		for _, target := range targets {
			s.sendToTarget(target, contextMsg)
		}
		return
	}

	// ACTION B: Prompt Present -> Invoke Agent (with or without script output)
	if job.Prompt != "" {
		// If there is script output, attach it
		contextMsg := output // can be empty if type is just "prompt"
		finalPrompt := job.Prompt
		if contextMsg != "" {
			finalPrompt = fmt.Sprintf("Context:\n%s\n\nInstruction:\n%s", contextMsg, job.Prompt)
		}

		// STATEFUL EXECUTION (History Aware)
		if job.UseHistory {
			// Execute separately for EACH target to maintain individual history
			for _, target := range targets {
				// Derive Session ID from Target ID
				sessionID := target.ID
				if sessionID == "" {
					sessionID = "general"
				}

				// Find Provider
				var provider messaging.Provider
				if p, ok := s.Providers[target.Provider]; ok {
					provider = p
				}
				// If provider not found, Run might handle it or we pass nil/wrapper?
				// Agent.Run handles nil provider gracefully for system prompt generation.

				// Execute Agent Run (simulates user turn)
				// We treat the prompt as the "User Input" in this context?
				// Or do we need a special "System Event" entry?
				// User wants "load all the context as usual when interacting with the agent".
				// So `Agent.Run` is appropriate.
				log.Printf("Running stateful cron for target %s (Session: %s)", target.ID, sessionID)
				response, err := s.Agent.Run(sessionID, provider, target.ID, finalPrompt)
				if err != nil {
					log.Printf("Error running stateful agent for target %s: %v", target.ID, err)
					continue
				}

				// Agent.Run does NOT send the message to the provider automatically (it returns the string).
				// (Wait, `Run` returns string, but does it send? logic in `server.go` calls `Run` then sends result.)
				// So we must send it here.
				s.sendToTarget(target, response)
			}
			return
		}

		// STATELESS EXECUTION (No History) - Default
		// 1. Generate Response once (Stateless)
		sysPrompt := s.Agent.GetBaseSystemPrompt()

		messages := []llm.Message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: finalPrompt},
		}

		response, err := s.Agent.LLM.Chat(messages)
		if err != nil {
			log.Printf("Error running stateless cron job %s: %v", job.Name, err)
			return
		}

		// Send Agent Response to Targets
		for _, target := range targets {
			s.sendToTarget(target, response)
		}
		return
	}
}

func (s *Scheduler) sendToTarget(target config.CronTarget, message string) {
	if target.Provider == "local" {
		// Log to console/file? Agent.RunTask already logs to history if used.
		// If raw output, maybe log it?
		log.Printf("[LOCAL TARGET %s] %s", target.ID, message)
	} else {
		if provider, ok := s.Providers[target.Provider]; ok {
			if err := provider.SendMessage(target.ID, message); err != nil {
				log.Printf("Error sending message to %s via %s: %v", target.ID, target.Provider, err)
			}
		} else {
			log.Printf("Unknown or uninitialized provider: %s", target.Provider)
		}
	}
}

func executeScript(configDir string, scriptString string) (string, error) {
	// 1. Parse potentially quoted arguments
	// For simplicity, we'll try a basic split for now, but usually we need true shell parsing.
	// Since we don't want to add big dependencies, let's assume standard space separation for now.
	// If the user wants complex args, they might need to wrap in a shell script.
	parts := strings.Fields(scriptString)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty script command")
	}

	command := parts[0]
	args := parts[1:]

	// 2. Resolve Path
	// If it's not absolute, resolve relative to configDir
	resolvedCommand := config.ResolvePath(configDir, command)

	// Make it absolute if it isn't already, to avoid issues when changing Dir
	if !filepath.IsAbs(resolvedCommand) {
		if abs, err := filepath.Abs(resolvedCommand); err == nil {
			resolvedCommand = abs
		}
	}

	var cmd *exec.Cmd

	// 3. Detect Execution Mode
	if strings.HasSuffix(resolvedCommand, ".js") {
		// Use node for .js files
		nodeArgs := append([]string{resolvedCommand}, args...)
		cmd = exec.Command("node", nodeArgs...)
		cmd.Dir = configDir
	} else if strings.HasSuffix(resolvedCommand, ".py") {
		// Use python for .py files
		pyArgs := append([]string{resolvedCommand}, args...)
		// Try "python" first, widely used alias
		// If explicit "python3" is needed user might need to symlink or wrapper
		cmd = exec.Command("python", pyArgs...)
		cmd.Dir = configDir
	} else if runtime.GOOS == "windows" {
		// Windows Logic
		if strings.HasSuffix(resolvedCommand, ".ps1") {
			psArgs := append([]string{"-File", resolvedCommand}, args...)
			cmd = exec.Command("powershell", psArgs...)
		} else if strings.HasSuffix(resolvedCommand, ".bat") {
			batArgs := append([]string{"/C", resolvedCommand}, args...)
			cmd = exec.Command("cmd", batArgs...)
		} else {
			// Executable
			cmd = exec.Command(resolvedCommand, args...)
		}
		cmd.Dir = configDir // Run in config dir
	} else {
		// Unix Logic
		if strings.HasSuffix(resolvedCommand, ".sh") {
			shArgs := append([]string{resolvedCommand}, args...)
			cmd = exec.Command("sh", shArgs...)
		} else {
			cmd = exec.Command(resolvedCommand, args...)
		}
		cmd.Dir = configDir
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return out.String(), fmt.Errorf("%v: %s", err, stderr.String())
	}

	return out.String(), nil
}
