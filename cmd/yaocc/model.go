package main

import (
	"fmt"
	"strings"

	"github.com/dev-dhg/yaocc/pkg/config"
)

func runModel(args []string) {
	// Need to identify config location. For CLI, we assume config.json in CWD
	// or we could support a flag, but this is simple CLI
	configPath := "config.json"

	cfg, _, _, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	if len(args) == 0 {
		// Show current model
		fmt.Printf("Current Model: %s\n", cfg.Models.Selected)
		return
	}

	cmd := args[0]
	switch cmd {
	case "list":
		fmt.Println("Available Models:")
		for pKey, provider := range cfg.Models.Providers {
			for _, m := range provider.Models {
				fullID := fmt.Sprintf("%s/%s", pKey, m.ID)
				selected := ""
				if fullID == cfg.Models.Selected {
					selected = "* "
				}
				fmt.Printf("%s%s (%s)\n", selected, m.Name, fullID)
			}
		}
	case "select":
		if len(args) < 2 {
			fmt.Println("Usage: yaocc model select <provider>/<id>")
			return
		}
		newModelID := args[1]

		// Validate model exists
		parts := strings.SplitN(newModelID, "/", 2)
		if len(parts) != 2 {
			fmt.Println("Invalid format. Use <provider>/<id>")
			return
		}
		pKey, mID := parts[0], parts[1]

		provider, ok := cfg.Models.Providers[pKey]
		if !ok {
			fmt.Printf("Provider '%s' not found.\n", pKey)
			return
		}

		found := false
		for _, m := range provider.Models {
			if m.ID == mID {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("Model ID '%s' not found in provider '%s'.\n", mID, pKey)
			return
		}

		// Acquire Lock to prevent server reload
		if err := config.AcquireConfigLock(); err != nil {
			fmt.Printf("Warning: Failed to acquire config lock: %v. Proceeding anyway.\n", err)
		} else {
			defer config.ReleaseConfigLock()
		}

		// Update config using raw update to preserve env var placeholders
		err = config.UpdateConfigRaw(func(rawCfg *config.Config) error {
			rawCfg.Models.Selected = newModelID
			return nil
		})
		if err != nil {
			fmt.Printf("Error updating configuration: %v\n", err)
			return
		}
		fmt.Printf("Selected model updated to: %s\n", newModelID)
		fmt.Println("Please restart the server for changes to take effect.")

	default:
		fmt.Printf("Unknown model command: %s\n", cmd)
	}
}
