package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dev-dhg/yaocc/pkg/config"
)

func runSkills(args []string) {
	// Fallback to "list" if no args, but wait, maybe "help"?
	// If runSkills is called with empty args, show help.
	if len(args) < 1 {
		printSkillsHelp()
		return
	}

	cmd := args[0]

	// Load config to manage skills
	cfg, configDir, _, err := config.LoadConfig("")
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		return
	}

	// Initialize Registered map if nil, but do this inside UpdateConfigRaw modifier
	// So we don't need to LoadConfig here for modification purpose.
	// But we might need it for checking existence or listing?
	// For "list" and "run" we need LoadConfig (with env expansion usually desired for running).
	// For "register/unregister" we want raw update.

	switch cmd {
	case "register":
		// Usage: yaocc skills register <name> <path>
		if len(args) < 3 {
			fmt.Println("Usage: yaocc skills register <name> <path>")
			return
		}
		name := args[1]
		scriptPath := args[2]

		// Validation: Reserved names
		reserved := map[string]bool{
			"register": true, "unregister": true, "list": true, "help": true,
			"file": true, "cron": true, "chat": true, "model": true, "init": true, "fetch": true, "websearch": true, "skills": true, "prompt": true,
		}
		if reserved[strings.ToLower(name)] {
			fmt.Printf("Error: '%s' is a reserved command name.\n", name)
			return
		}

		// Validation: Check if script exists
		configDir := config.ResolveConfigDir()
		resolvedPath, err := resolveSafePath(configDir, scriptPath)
		if err != nil {
			fmt.Printf("Error resolving script path: %v\n", err)
			return
		}
		if _, err := os.Stat(resolvedPath); err != nil {
			fmt.Printf("Error: Script file '%s' not found.\n", scriptPath)
			return
		}

		// Acquire Lock to prevent server reload
		if err := config.AcquireConfigLock(); err != nil {
			fmt.Printf("Warning: Failed to acquire config lock: %v. Proceeding anyway.\n", err)
		} else {
			defer config.ReleaseConfigLock()
		}

		// Register using UpdateConfigRaw
		err = config.UpdateConfigRaw(func(cfg *config.Config) error {
			if cfg.Skills.Registered == nil {
				cfg.Skills.Registered = make(map[string]string)
			}
			cfg.Skills.Registered[name] = scriptPath
			return nil
		})

		if err != nil {
			fmt.Printf("Error updating configuration: %v\n", err)
			return
		}
		fmt.Printf("Skill '%s' registered successfully linked to '%s'.\n", name, scriptPath)

	case "unregister":
		// Usage: yaocc skills unregister <name>
		if len(args) < 2 {
			fmt.Println("Usage: yaocc skills unregister <name>")
			return
		}
		name := args[1]

		// Acquire Lock to prevent server reload
		if err := config.AcquireConfigLock(); err != nil {
			fmt.Printf("Warning: Failed to acquire config lock: %v. Proceeding anyway.\n", err)
		} else {
			defer config.ReleaseConfigLock()
		}

		err := config.UpdateConfigRaw(func(cfg *config.Config) error {
			if cfg.Skills.Registered == nil {
				return fmt.Errorf("no registered skills found")
			}
			if _, exists := cfg.Skills.Registered[name]; !exists {
				return fmt.Errorf("skill '%s' not found", name)
			}
			delete(cfg.Skills.Registered, name)
			return nil
		})

		if err != nil {
			fmt.Printf("Error updating configuration: %v\n", err)
			return
		}
		fmt.Printf("Skill '%s' unregistered successfully.\n", name)

	case "list":
		// Built-in skills
		builtIn := []string{"cron", "file", "fetch", "websearch", "prompt"}
		sort.Strings(builtIn)

		fmt.Println("Built-in Skills:")
		for _, s := range builtIn {
			fmt.Printf("  - %s (built-in)\n", s)
		}

		fmt.Println("\nRegistered Skills:")
		if len(cfg.Skills.Registered) == 0 {
			fmt.Println("  (No registered skills)")
			return
		}

		// Sort keys for consistent output
		keys := make([]string, 0, len(cfg.Skills.Registered))
		for k := range cfg.Skills.Registered {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, name := range keys {
			path := cfg.Skills.Registered[name]
			fmt.Printf("  - %s -> %s\n", name, path)
		}

	case "help":
		printSkillsHelp()

	default:
		// Check if it's a registered skill being called via "yaocc skills <name>"
		// The command is "skills", runSkills receives args starting with subcommand.
		// So `yaocc skills my_skill arg1` -> args=["my_skill", "arg1"]
		// output of switch cmd=args[0] is "my_skill".

		name := cmd
		if scriptPath, ok := cfg.Skills.Registered[name]; ok {
			// Execute it!
			// Resolve path again just to be safe at runtime
			resolvedPath, err := resolveSafePath(configDir, scriptPath)
			if err != nil {
				fmt.Printf("Error resolving skill path: %v\n", err)
				return
			}

			// Pass remaining args
			skillArgs := args[1:]
			executeScript(resolvedPath, skillArgs)
			return
		}

		fmt.Printf("Unknown skills command or skill: %s\n", cmd)
		printSkillsHelp()
	}
}

func printSkillsHelp() {
	fmt.Println("Usage: yaocc skills <command> [args]")
	fmt.Println("Commands:")
	fmt.Println("  register <name> <path>   Register a new skill")
	fmt.Println("  unregister <name>        Unregister an existing skill")
	fmt.Println("  list                     List all skills (built-in and registered)")
	fmt.Println("  <name> [args]            Execute a registered skill")
}
