package main

import (
	"fmt"
	"log"

	"github.com/dev-dhg/yaocc/pkg/config"
	"github.com/dev-dhg/yaocc/pkg/llm"
)

func main() {
	cfg, _, _, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	testProvider(cfg, "ollama")
	testProvider(cfg, "nvidia")
}

func testProvider(cfg *config.Config, providerName string) {
	fmt.Printf("Testing provider: %s\n", providerName)
	providerCfg, ok := cfg.Models.Providers[providerName]
	if !ok {
		fmt.Printf("Provider %s not found in config\n", providerName)
		return
	}

	// Print loaded models
	fmt.Printf("Loaded %d models for %s:\n", len(providerCfg.Models), providerName)
	for _, model := range providerCfg.Models {
		fmt.Printf("- %s (ID: %s, Context: %d)\n", model.Name, model.ID, model.ContextWindow)
	}

	// For testing, we need to pick a model. defaulting to the first one if available
	testModel := ""
	if len(providerCfg.Models) > 0 {
		testModel = providerCfg.Models[0].Model
	} else {
		// fallback to config default if set
		// But providerCfg doesn't have DefaultModel field anymore in the struct I saw earlier?
		// Let's check config.go, ProviderConfig has Models []ModelConfig.
		// It seems I might have missed that in the previous view.
		// Actually, let's look at llm.NewClient signature. It takes (ProviderConfig, selectedModel string).
		// So we need to pass the model name.
	}

	if testModel == "" {
		fmt.Println("No models found for provider, skipping chat test.")
		return
	}

	client := llm.NewClient(providerCfg, testModel)
	messages := []llm.Message{
		{Role: "user", Content: "Hello, are you functional? Reply with 'Yes, I am functioning'."},
	}

	response, err := client.Chat(messages)
	if err != nil {
		fmt.Printf("Error chatting with %s: %v\n", providerName, err)
		return
	}

	fmt.Printf("Response from %s (%s): %s\n", providerName, testModel, response)
	fmt.Println("--------------------------------------------------")
}
