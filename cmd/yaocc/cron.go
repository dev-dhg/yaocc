package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/dev-dhg/yaocc/pkg/config"
)

func runCron(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: yaocc cron <list|add|remove|run> [args]")
		return
	}

	cmd := args[0]
	switch cmd {
	case "list":
		runCronList(args[1:])
	case "add":
		runCronAdd(args[1:])
	case "remove":
		runCronRemove(args[1:])
	case "run":
		runCronRun(args[1:])
	default:
		fmt.Printf("Unknown cron command: %s\n", cmd)
	}
}

func runCronList(args []string) {
	listCmd := flag.NewFlagSet("list", flag.ExitOnError)
	configPath := listCmd.String("config", "config.json", "Path to config file")

	if err := listCmd.Parse(args); err != nil {
		fmt.Println("Error parsing flags:", err)
		return
	}

	cfg, _, _, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	if len(cfg.Cron) == 0 {
		fmt.Println("No cron jobs configured.")
		return
	}

	fmt.Println("Configured Jobs:")
	for i, job := range cfg.Cron {
		desc := job.Prompt
		if job.Type == "script" {
			desc = fmt.Sprintf("Script: %s", job.Script)
		}

		stateString := "stateless"
		if job.UseHistory {
			stateString = "stateful/history-aware"
		}

		fmt.Printf("  [%d] %s: %s (%s) [%s]\n", i, job.Name, job.Schedule, desc, stateString)
	}
}

func runCronAdd(args []string) {
	addCmd := flag.NewFlagSet("add", flag.ExitOnError)
	configPath := addCmd.String("config", "config.json", "Path to config file")
	name := addCmd.String("name", "", "Name of the cron job")
	schedule := addCmd.String("schedule", "", "Cron schedule (e.g. \"0 9 * * *\")")
	prompt := addCmd.String("prompt", "", "Prompt for the agent (type=prompt)")
	script := addCmd.String("script", "", "Script path (type=script)")
	sessionID := addCmd.String("session", "", "Session ID context (optional)")
	useHistory := addCmd.Bool("use-history", false, "Use target session history. If true, runs for each target separately.")
	targetProvider := addCmd.String("target-provider", "", "Target provider (e.g. telegram)")
	targetID := addCmd.String("target-id", "", "Target ID (e.g. chat_id)")

	if err := addCmd.Parse(args); err != nil {
		fmt.Println("Error parsing flags:", err)
		return
	}

	if *name == "" || *schedule == "" {
		fmt.Println("Usage: yaocc cron add --name <name> --schedule <schedule> [--prompt <prompt> | --script <script>] [--use-history] [--target-provider <provider> --target-id <id>] [--config <path>]")
		return
	}

	// Determine Type
	jobType := "prompt"
	if *script != "" {
		jobType = "script"
	}

	if jobType == "prompt" && *prompt == "" {
		fmt.Println("Error: --prompt is required for prompt-type jobs (default).")
		return
	}

	// Build Targets
	var targets []config.CronTarget
	if *targetProvider != "" && *targetID != "" {
		targets = append(targets, config.CronTarget{
			Provider: *targetProvider,
			ID:       *targetID,
		})
	}

	// Prepare new job
	newJob := config.CronJob{
		Name:       *name,
		Schedule:   *schedule,
		Type:       jobType,
		Prompt:     *prompt,
		Script:     *script,
		SessionID:  *sessionID,
		UseHistory: *useHistory,
		Targets:    targets,
	}

	// Acquire Lock
	if err := config.AcquireConfigLock(); err != nil {
		fmt.Printf("Warning: Failed to acquire config lock: %v. Proceeding anyway.\n", err)
	} else {
		defer config.ReleaseConfigLock()
	}

	// Update Config
	err := config.UpdateConfigRawWithPath(*configPath, func(cfg *config.Config) error {
		// Check duplicates
		for _, job := range cfg.Cron {
			if strings.EqualFold(job.Name, *name) {
				return fmt.Errorf("job with name '%s' already exists", *name)
			}
		}

		cfg.Cron = append(cfg.Cron, newJob)
		return nil
	})

	if err != nil {
		fmt.Printf("Error updating configuration: %v\n", err)
		return
	}

	fmt.Printf("Added cron job: %s\n", *name)
}

func runCronRemove(args []string) {
	removeCmd := flag.NewFlagSet("remove", flag.ExitOnError)
	configPath := removeCmd.String("config", "config.json", "Path to config file")

	if err := removeCmd.Parse(args); err != nil {
		fmt.Println("Error parsing arguments:", err)
		return
	}

	if removeCmd.NArg() < 1 {
		fmt.Println("Usage: yaocc cron remove <name> [--config <path>]")
		return
	}
	name := removeCmd.Arg(0)

	// Acquire Lock
	if err := config.AcquireConfigLock(); err != nil {
		fmt.Printf("Warning: Failed to acquire config lock: %v. Proceeding anyway.\n", err)
	} else {
		defer config.ReleaseConfigLock()
	}

	err := config.UpdateConfigRawWithPath(*configPath, func(cfg *config.Config) error {
		newCron := []config.CronJob{}
		found := false
		for _, job := range cfg.Cron {
			if strings.EqualFold(job.Name, name) {
				found = true
				continue
			}
			newCron = append(newCron, job)
		}

		if !found {
			return fmt.Errorf("job '%s' not found", name)
		}

		cfg.Cron = newCron
		return nil
	})

	if err != nil {
		fmt.Printf("Error updating configuration: %v\n", err)
		return
	}

	fmt.Printf("Removed cron job: %s\n", name)
}

func runCronRun(args []string) {
	runCmd := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := runCmd.String("config", "config.json", "Path to config file")

	if err := runCmd.Parse(args); err != nil {
		fmt.Println("Error parsing args:", err)
		return
	}

	if runCmd.NArg() < 1 {
		fmt.Println("Usage: yaocc cron run <index> [--config <path>]")
		fmt.Println("\nUse 'yaocc cron list' to see job indices.")
		return
	}

	index, err := strconv.Atoi(runCmd.Arg(0))
	if err != nil {
		fmt.Printf("Invalid index: %s (must be a number)\n", runCmd.Arg(0))
		return
	}

	// Load config to get server port
	cfg, _, _, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	port := cfg.Server.Port
	if port == 0 {
		port = 8080
	}

	serverURL := fmt.Sprintf("http://localhost:%d/cron/run", port)

	reqBody, _ := json.Marshal(map[string]int{"index": index})
	resp, err := http.Post(serverURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		fmt.Printf("Error connecting to server: %v\n", err)
		fmt.Println("Make sure the yaocc server is running.")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusAccepted {
		var result map[string]string
		if err := json.Unmarshal(body, &result); err == nil {
			fmt.Printf("✓ Triggered job: %s\n", result["job"])
		} else {
			fmt.Println("Job triggered successfully.")
		}
	} else {
		fmt.Printf("Error: %s\n", string(body))
	}
}
