package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/dev-dhg/yaocc/pkg/agent"
	"github.com/dev-dhg/yaocc/pkg/config"
	"github.com/dev-dhg/yaocc/pkg/cron"
	"github.com/dev-dhg/yaocc/pkg/messaging"
	"github.com/dev-dhg/yaocc/pkg/messaging/telegram"
	"github.com/dev-dhg/yaocc/pkg/server"
	"github.com/dev-dhg/yaocc/pkg/version"
)

func main() {
	configPath := flag.String("config", "config.json", "path to config file")
	logLevel := flag.String("level", "info", "log level (info, verbose)")
	logFile := flag.String("file", "", "path to log file for verbose output")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("YAOCC Server %s\n", version.String())
		os.Exit(0)
	}

	log.Printf("Starting YAOCC Server %s...", version.Version)
	log.Printf("Loading configuration from %s...", *configPath)


	cfg, configDir, loadedPath, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}
	log.Printf("Configuration loaded from: %s", loadedPath)

	// Determine verbosity
	verbose := *logLevel == "verbose"

	// Resolve Log File Path
	resolvedLogFile := *logFile
	if resolvedLogFile != "" {
		resolvedLogFile = config.ResolvePath(configDir, resolvedLogFile)
	}

	// Initialize Agent
	myAgent, err := agent.NewAgent(cfg, configDir, verbose, resolvedLogFile)
	if err != nil {
		log.Fatalf("Error initializing agent: %v", err)
	}

	log.Printf("Agent initialized with soul: %s...", string(myAgent.Soul[:20]))

	// Start Telegram Bot
	// Start Messaging Clients
	providers := make(map[string]messaging.Provider)
	for _, msgCfg := range cfg.Messaging {
		if msgCfg.Provider == "telegram" && msgCfg.Telegram.Enabled {
			log.Printf("Initializing Telegram Bot...")
			tgClient := telegram.NewClient(msgCfg.Telegram, myAgent)
			go tgClient.Start() // Use interface method
			providers["telegram"] = tgClient
			// For now, we only support one telegram client in the scheduler/server
			break
		}
	}

	// Legacy fallback removed as requested.

	// Start Cron/Heartbeat Scheduler
	scheduler := cron.NewScheduler(cfg, configDir, myAgent, providers)
	scheduler.Start()
	defer scheduler.Stop()

	// Start Config Watcher (for general config changes: models, messaging, etc.)
	go config.WatchConfig(loadedPath, func(newCfg *config.Config) {
		mu := "Server" // Just a label
		log.Printf("[%s] Applying new configuration...", mu)

		myAgent.UpdateConfig(newCfg)
		// Note: We no longer reload the scheduler here since cron jobs
		// are managed independently via cron.json watcher below.

		// If we had a mechanism to update Telegram client, we would do it here too.
		// For now, most telegram changes require restart, but we can update allowed users if we refactor Client.
	})

	// Start independent cron.json watcher
	cronPath := filepath.Join(configDir, "cron.json")
	go config.WatchFile(cronPath, func() {
		log.Println("[CronWatcher] cron.json changed, reloading scheduler...")
		jobs, err := config.LoadCronJobs(configDir)
		if err != nil {
			log.Printf("[CronWatcher] Error loading cron.json: %v", err)
			return
		}
		// Update in-memory config and reload scheduler
		cfg.Cron = jobs
		scheduler.Reload(cfg)
	})

	// Start independent skills_register.json watcher
	skillsRegPath := filepath.Join(configDir, "skills_register.json")
	go config.WatchFile(skillsRegPath, func() {
		log.Println("[SkillsWatcher] skills_register.json changed, updating agent skills registry...")
		reg, err := config.LoadSkillsRegister(configDir)
		if err != nil {
			log.Printf("[SkillsWatcher] Error loading skills_register.json: %v", err)
			return
		}
		// Update in-memory config with new registration
		cfg.Skills.Registered = reg.Registered
		log.Printf("[SkillsWatcher] Skills registry updated: %d registered skills", len(reg.Registered))
	})

	// Start Server
	srv := server.NewServer(cfg, myAgent, providers, scheduler)
	if err := srv.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
