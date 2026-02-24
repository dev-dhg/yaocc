package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/dev-dhg/yaocc/pkg/config"
	"github.com/dev-dhg/yaocc/pkg/llm"
)

func runPrompt(args []string) {
	// Parse flags
	promptCmd := flag.NewFlagSet("prompt", flag.ExitOnError)
	modelFlag := promptCmd.String("model", "", "Model ID to use (optional)")

	err := promptCmd.Parse(args)
	if err != nil {
		fmt.Printf("Error parsing flags: %v\n", err)
		return
	}

	remainingArgs := promptCmd.Args()
	if len(remainingArgs) == 0 {
		fmt.Println("Usage: yaocc prompt [flags] <message>")
		fmt.Println("Flags:")
		promptCmd.PrintDefaults()
		return
	}

	prompt := strings.Join(remainingArgs, " ")

	// Load configuration
	cfg, _, _, err := config.LoadConfig("")
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		return
	}

	// Determine model to use
	modelID := cfg.Models.Selected
	if *modelFlag != "" {
		modelID = *modelFlag
	}

	// Logic to handle provider/modelID format (copied from agent.go)
	var providerKey, specificModelID string
	if strings.Contains(modelID, "/") {
		parts := strings.SplitN(modelID, "/", 2)
		providerKey = parts[0]
		specificModelID = parts[1]
	} else {
		// If no provider specified, we have to search all providers?
		// Or assume a default? agent.go defaults to "ollama".
		// But prompt command might be more flexible.
		// Let's search all providers if no prefix.
	}

	// Find the provider config for the selected model
	var providerCfg config.ProviderConfig
	found := false

	if providerKey != "" {
		// Look up specific provider
		if p, ok := cfg.Models.Providers[providerKey]; ok {
			for _, m := range p.Models {
				if m.ID == specificModelID {
					providerCfg = p
					// Ensure we use the API model name if available, but NewClient takes specificModelID usually?
					// Wait, llm.NewClient takes (ProviderConfig, selectedModel).
					// In agent.go: `a.LLM = llm.NewClient(provider, actualModelName)` where actualModelName is m.Model.
					// So specificModelID is the ID in config, but we need to pass the real model name to LLM client.
					modelID = m.Model // Update modelID to be the API model name
					found = true
					break
				}
			}
		}
	} else {
		// Search all
		for _, p := range cfg.Models.Providers {
			for _, m := range p.Models {
				if m.ID == modelID || m.Model == modelID {
					providerCfg = p
					modelID = m.Model // Update to API model name
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}

	if !found {
		fmt.Printf("Error: Model '%s' not found in configuration.\n", modelID)
		return
	}

	// Create LLM Client
	client := llm.NewClient(providerCfg, modelID)

	// Send request
	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	fmt.Printf("Sending prompt to %s...\n", modelID)
	response, _, err := client.Chat(messages, nil)
	if err != nil {
		fmt.Printf("Error during chat: %v\n", err)
		return
	}

	fmt.Println(response)
}
