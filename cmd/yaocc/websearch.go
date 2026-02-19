package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dev-dhg/yaocc/pkg/config"
	"github.com/dev-dhg/yaocc/pkg/websearch"
)

func runWebSearch(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: yaocc websearch <query>")
		os.Exit(1)
	}

	query := strings.Join(args, " ")

	cfg, _, _, err := config.LoadConfig("config.json")
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	if cfg.WebSearch.Provider == "" {
		fmt.Println("Error: No websearch provider selected in config.")
		os.Exit(1)
	}

	providerCfg, ok := cfg.WebSearch.Providers[cfg.WebSearch.Provider]
	if !ok {
		fmt.Printf("Error: Websearch provider '%s' not found in config.\n", cfg.WebSearch.Provider)
		os.Exit(1)
	}

	// Initialize provider
	// We need tempDir from config
	provider, err := websearch.NewProvider(cfg.WebSearch.Provider, providerCfg, cfg.WebSearch.Providers, cfg.Storage.TempDir)
	if err != nil {
		fmt.Printf("Error initializing websearch provider: %v\n", err)
		os.Exit(1)
	}

	results, err := provider.Search(query)
	if err != nil {
		fmt.Printf("Error performing search: %v\n", err)
		os.Exit(1)
	}

	jsonOutput, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling results: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(jsonOutput))
}
