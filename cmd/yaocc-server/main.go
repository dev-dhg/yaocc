package main

import (
	"flag"
	"log"

	"github.com/dev-dhg/yaocc/pkg/agent"
	"github.com/dev-dhg/yaocc/pkg/config"
	"github.com/dev-dhg/yaocc/pkg/cron"
	"github.com/dev-dhg/yaocc/pkg/messaging"
	"github.com/dev-dhg/yaocc/pkg/messaging/telegram"
	"github.com/dev-dhg/yaocc/pkg/server"
)

func main() {
	configPath := flag.String("config", "config.json", "path to config file")
	logLevel := flag.String("level", "info", "log level (info, verbose)")
	logFile := flag.String("file", "", "path to log file for verbose output")
	flag.Parse()

	log.Printf("Starting YAOCC Server...")
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

	// Start Config Watcher
	go config.WatchConfig(loadedPath, func(newCfg *config.Config) {
		mu := "Server" // Just a label
		log.Printf("[%s] Applying new configuration...", mu)

		myAgent.UpdateConfig(newCfg)
		scheduler.Reload(newCfg)

		// If we had a mechanism to update Telegram client, we would do it here too.
		// For now, most telegram changes require restart, but we can update allowed users if we refactor Client.
	})

	// Start Server
	srv := server.NewServer(cfg, myAgent, providers, scheduler)
	if err := srv.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
