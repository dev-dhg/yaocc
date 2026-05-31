package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dev-dhg/yaocc/pkg/config"
	"github.com/dev-dhg/yaocc/pkg/skills"
)

func runSkills(args []string) {
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

	switch cmd {
	case "list":
		// Load all skills dynamically by scanning configDir/skills
		loader := skills.NewLoader([]string{filepath.Join(configDir, "skills")})
		loadedSkills, err := loader.Load()
		if err != nil {
			fmt.Printf("Error loading skills: %v\n", err)
			return
		}

		fmt.Println("Available Skills:")
		if len(loadedSkills) == 0 {
			fmt.Println("  (No skills found)")
			return
		}

		// Sort skills by Name
		sort.Slice(loadedSkills, func(i, j int) bool {
			return strings.ToLower(loadedSkills[i].Name) < strings.ToLower(loadedSkills[j].Name)
		})

		for _, s := range loadedSkills {
			suffix := ""
			if s.IsBuiltIn() {
				suffix = " (built-in)"
			}
			fmt.Printf("  - %s: %s%s\n", s.Name, s.Description, suffix)
		}

	case "get":
		// Usage: yaocc skills get <name>
		if len(args) < 2 {
			fmt.Println("Usage: yaocc skills get <name>")
			return
		}
		name := args[1]

		loader := skills.NewLoader([]string{filepath.Join(configDir, "skills")})
		loadedSkills, err := loader.Load()
		if err != nil {
			fmt.Printf("Error loading skills: %v\n", err)
			return
		}

		var targetSkill *skills.Skill
		for _, s := range loadedSkills {
			if strings.EqualFold(s.Name, name) || strings.EqualFold(filepath.Base(filepath.Dir(s.Path)), name) {
				targetSkill = &s
				break
			}
		}

		if targetSkill != nil {
			fmt.Printf("\n--- %s Manual ---\n%s\n", targetSkill.Name, strings.TrimSpace(targetSkill.Content))
			return
		}

		fmt.Printf("Skill '%s' not found.\n", name)

	case "run":
		// Usage: yaocc skills run <name> [args...]
		if len(args) < 2 {
			fmt.Println("Usage: yaocc skills run <name> [args...]")
			return
		}
		runSkillByName(configDir, args[1], args[2:])

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
		runSkillByName(configDir, name, args[1:])
	}
}

func runSkillByName(configDir, name string, skillArgs []string) {
	loader := skills.NewLoader([]string{filepath.Join(configDir, "skills")})
	loadedSkills, err := loader.Load()
	if err != nil {
		fmt.Printf("Error loading skills: %v\n", err)
		return
	}

	var targetSkill *skills.Skill
	for _, s := range loadedSkills {
		if strings.EqualFold(s.Name, name) || strings.EqualFold(filepath.Base(filepath.Dir(s.Path)), name) {
			targetSkill = &s
			break
		}
	}

	if targetSkill == nil {
		fmt.Printf("Unknown skills command or skill: %s\n", name)
		printSkillsHelp()
		return
	}

	// Output the skill manual and the arguments passed
	fmt.Printf("=== SKILL MANUAL: %s ===\n", targetSkill.Name)
	if targetSkill.Description != "" {
		fmt.Printf("Description: %s\n\n", targetSkill.Description)
	}
	fmt.Println("--- Instructions ---")
	fmt.Println(strings.TrimSpace(targetSkill.Content))
	fmt.Println()
	fmt.Println("--- Executed with Arguments ---")
	if len(skillArgs) > 0 {
		fmt.Println(strings.Join(skillArgs, " "))
	} else {
		fmt.Println("(None)")
	}
	fmt.Println("==============================")
}

func printSkillsHelp() {
	fmt.Println("Usage: yaocc skills <command> [args]")
	fmt.Println("Commands:")
	fmt.Println("  list                     List all available skills (built-in and custom)")
	fmt.Println("  get <name>               Read the instructions (SKILL.md) for a skill")
	fmt.Println("  run <name> [args]        Run a skill (displays instructions and arguments)")
	fmt.Println("  tutorial                 Read the comprehensive skill creation tutorial")
}
