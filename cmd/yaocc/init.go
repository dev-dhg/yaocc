package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dev-dhg/yaocc/pkg/config"
	"github.com/dev-dhg/yaocc/pkg/templates"
	"github.com/joho/godotenv"
)

func runInit() {
	fmt.Println("Initializing new YAOCC project...")

	// 0. Load .env and resolve config dir
	_ = godotenv.Load()
	configDir := config.ResolveConfigDir()

	if configDir != "." {
		fmt.Printf("Using configuration directory: %s\n", configDir)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			fmt.Printf("Error creating config directory: %v\n", err)
			return
		}
	}

	// Helper to prefix paths
	p := func(path string) string {
		return filepath.Join(configDir, path)
	}

	// 1. Create config.json
	createFileFromTemplate(p("config.json"), "config.json")

	// 2. Create Workspace Files
	createFileFromTemplate(p("SOUL.md"), "SOUL.md")
	createFileFromTemplate(p("AGENTS.md"), "AGENTS.md")
	createFileFromTemplate(p("TOOLS.md"), "TOOLS.md")
	createFileFromTemplate(p("IDENTITY.md"), "IDENTITY.md")
	createFileFromTemplate(p("USER.md"), "USER.md")
	createFileFromTemplate(p("BOOTSTRAP.md"), "BOOTSTRAP.md")
	createFileFromTemplate(p("MEMORY.md"), "MEMORY.md")
	createFileFromTemplate(p("SKILLS_TUTORIAL.md"), "SKILLS_TUTORIAL.md")

	// 3. Create sample cron skill
	if err := os.MkdirAll(p("skills/cron"), 0755); err != nil {
		fmt.Printf("Error creating skills/cron directory: %v\n", err)
	}

	createFileFromTemplate(p("skills/cron/SKILL.md"), "cron_skill.md")

	// 4. Create file skill
	if err := os.MkdirAll(p("skills/file"), 0755); err != nil {
		fmt.Printf("Error creating skills/file directory: %v\n", err)
	}
	createFileFromTemplate(p("skills/file/SKILL.md"), "file_skill.md")

	// 5. Create fetch skill
	if err := os.MkdirAll(p("skills/fetch"), 0755); err != nil {
		fmt.Printf("Error creating skills/fetch directory: %v\n", err)
	}
	createFileFromTemplate(p("skills/fetch/SKILL.md"), "fetch_skill.md")

	// 6. Create websearch skill
	if err := os.MkdirAll(p("skills/websearch"), 0755); err != nil {
		fmt.Printf("Error creating skills/websearch directory: %v\n", err)
	}
	createFileFromTemplate(p("skills/websearch/SKILL.md"), "websearch_skill.md")

	// 7. Create prompt skill
	if err := os.MkdirAll(p("skills/prompt"), 0755); err != nil {
		fmt.Printf("Error creating skills/prompt directory: %v\n", err)
	}
	createFileFromTemplate(p("skills/prompt/SKILL.md"), "prompt_skill.md")

	// 8. Create exec skill
	if err := os.MkdirAll(p("skills/exec"), 0755); err != nil {
		fmt.Printf("Error creating skills/exec directory: %v\n", err)
	}
	createFileFromTemplate(p("skills/exec/SKILL.md"), "exec_skill.md")

	fmt.Println("Project initialized successfully!")
	fmt.Println("Run 'yaocc-server' to start the server.")
}

func createFileFromTemplate(targetPath, templateName string) {
	if _, err := os.Stat(targetPath); err == nil {
		fmt.Printf("File %s already exists, skipping.\n", targetPath)
		return
	}

	content, err := templates.Files.ReadFile(templateName)
	if err != nil {
		fmt.Printf("Error reading template %s: %v\n", templateName, err)
		return
	}

	if err := os.WriteFile(targetPath, content, 0644); err != nil {
		fmt.Printf("Error creating %s: %v\n", targetPath, err)
	} else {
		fmt.Printf("Created %s\n", targetPath)
	}
}

func createFile(path, content string) {
	// Deprecated, keeping for compatibility if needed or remove
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("File %s already exists, skipping.\n", path)
		return
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Printf("Error creating %s: %v\n", path, err)
	} else {
		fmt.Printf("Created %s\n", path)
	}
}
