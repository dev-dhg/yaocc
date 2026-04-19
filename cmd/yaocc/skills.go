package main

import (
	"fmt"
	"os"
	"path/filepath"
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

	// Load config for general settings (configDir, etc.)
	_, configDir, _, err := config.LoadConfig("")
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
		resolvedPath, err := resolveSafePath(configDir, scriptPath)
		if err != nil {
			fmt.Printf("Error resolving script path: %v\n", err)
			return
		}
		if _, err := os.Stat(resolvedPath); err != nil {
			fmt.Printf("Error: Script file '%s' not found.\n", scriptPath)
			return
		}

		// Update skills_register.json directly (no config lock needed)
		err = config.UpdateSkillsRegisterRaw(func(reg *config.SkillsRegister) error {
			if reg.Registered == nil {
				reg.Registered = make(map[string]string)
			}
			reg.Registered[name] = scriptPath
			return nil
		})

		if err != nil {
			fmt.Printf("Error updating skills_register.json: %v\n", err)
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

		// Update skills_register.json directly (no config lock needed)
		err := config.UpdateSkillsRegisterRaw(func(reg *config.SkillsRegister) error {
			if reg.Registered == nil || len(reg.Registered) == 0 {
				return fmt.Errorf("no registered skills found")
			}
			if _, exists := reg.Registered[name]; !exists {
				return fmt.Errorf("skill '%s' not found", name)
			}
			delete(reg.Registered, name)
			return nil
		})

		if err != nil {
			fmt.Printf("Error updating skills_register.json: %v\n", err)
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

		// Load from skills_register.json
		reg, err := config.LoadSkillsRegister(configDir)
		if err != nil {
			fmt.Printf("Error loading skills_register.json: %v\n", err)
			return
		}

		fmt.Println("\nRegistered Skills:")
		if len(reg.Registered) == 0 {
			fmt.Println("  (No registered skills)")
			return
		}

		// Sort keys for consistent output
		keys := make([]string, 0, len(reg.Registered))
		for k := range reg.Registered {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, name := range keys {
			path := reg.Registered[name]
			fmt.Printf("  - %s -> %s\n", name, path)
		}

	case "get":
		// Usage: yaocc skills get <name>
		if len(args) < 2 {
			fmt.Println("Usage: yaocc skills get <name>")
			return
		}
		name := args[1]

		// Load from skills_register.json
		reg, err := config.LoadSkillsRegister(configDir)
		if err != nil {
			fmt.Printf("Error loading skills_register.json: %v\n", err)
			return
		}

		// Check if it's a registered skill
		if scriptPath, ok := reg.Registered[name]; ok {
			fmt.Printf("Skill '%s' points to script '%s'.\n", name, scriptPath)
			resolvedScript, _ := resolveSafePath(configDir, scriptPath)
			skillDir := filepath.Dir(resolvedScript)
			readmePath := filepath.Join(skillDir, "SKILL.md")
			if content, err := os.ReadFile(readmePath); err == nil {
				fmt.Printf("\n--- SKILL.md ---\n%s\n", string(content))
			} else {
				fmt.Printf("Warning: Could not automatically find SKILL.md near the script at %s\n", readmePath)
			}
			return
		}

		// Otherwise, it might be a built-in skill, or we should look through actual paths.
		fmt.Printf("Skill '%s' is not registered as a custom script skill. If it's built-in (e.g. cron, file_manager, websearch), refer to the core documentation or verify the name.\n", name)

	case "tutorial":
		dir := config.ResolveConfigDir()
		tutorialPath := filepath.Join(dir, "SKILLS_TUTORIAL.md")
		if content, err := os.ReadFile(tutorialPath); err == nil {
			fmt.Printf("\n--- SKILLS_TUTORIAL.md ---\n%s\n", string(content))
		} else {
			fmt.Printf("Warning: Could not find SKILLS_TUTORIAL.md at %s. Try running 'yaocc init' to generate it.\n", tutorialPath)
		}

	case "help":
		printSkillsHelp()

	default:
		name := cmd

		// Load from skills_register.json
		reg, err := config.LoadSkillsRegister(configDir)
		if err != nil {
			fmt.Printf("Error loading skills_register.json: %v\n", err)
			return
		}

		if scriptPath, ok := reg.Registered[name]; ok {
			// Execute it!
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
	fmt.Println("  get <name>               Read the instructions (SKILL.md) for a skill")
	fmt.Println("  tutorial                 Read the comprehensive skill creation tutorial")
	fmt.Println("  <name> [args]            Execute a registered skill")
}
