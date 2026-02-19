package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dev-dhg/yaocc/pkg/config"
)

func runFile(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: yaocc file <read|write|list|delete|mkdir|run> [args]")
		return
	}

	// Load Config to get the base directory
	// We don't need the full config struct, just the directory resolution logic
	// But LoadConfig does it all.
	_, configDir, _, err := config.LoadConfig("")
	// If config loading fails, we might still want to proceed if we can default to CWD?
	// But requirements say "enforce YAOCC_CONFIG_DIR".
	// LoadConfig defaults to CWD if env is not set.
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		return
	}
	// Debug: Print config dir logic
	fmt.Printf("DEBUG: Config Dir resolved to: %s\n", configDir)

	// Helper to resolve paths using the shared logic
	resolvePath := func(inputPath string) (string, error) {
		return resolveSafePath(configDir, inputPath)
	}

	cmd := args[0]
	switch cmd {
	case "list":
		// Usage: yaocc file list [dir]
		subDir := "."
		if len(args) > 1 {
			subDir = args[1]
		}

		targetPath, err := resolvePath(subDir)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		entries, err := os.ReadDir(targetPath)
		if err != nil {
			fmt.Printf("Error listing directory: %v\n", err)
			return
		}
		for _, e := range entries {
			info, _ := e.Info()
			fmt.Printf("%s %d %s\n", (map[bool]string{true: "d", false: "-"}[e.IsDir()]), info.Size(), e.Name())
		}
	case "read":
		// Usage: yaocc file read <path>
		if len(args) < 2 {
			fmt.Println("Usage: yaocc file read <path>")
			return
		}
		targetPath, err := resolvePath(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		content, err := os.ReadFile(targetPath)
		if err != nil {
			fmt.Printf("Error reading file: %v\n", err)
			return
		}
		fmt.Println(string(content))
	case "write":
		// Usage: yaocc file write <path> <content>
		if len(args) < 3 {
			fmt.Println("Usage: yaocc file write <path> <content>")
			return
		}
		targetPath, err := resolvePath(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		content := args[2]
		content = strings.ReplaceAll(content, "\\n", "\n")

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			fmt.Printf("Error creating directories: %v\n", err)
			return
		}

		if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
			fmt.Printf("Error writing file: %v\n", err)
			return
		}
		fmt.Printf("Successfully wrote to %s\n", args[1]) // Don't leak full absolute path if possible, or maybe it's fine.
	case "delete":
		// Usage: yaocc file delete <path>
		if len(args) < 2 {
			fmt.Println("Usage: yaocc file delete <path>")
			return
		}
		targetPath, err := resolvePath(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		if err := os.Remove(targetPath); err != nil {
			fmt.Printf("Error deleting file: %v\n", err)
			return
		}
		fmt.Printf("Successfully deleted %s\n", args[1])
	case "mkdir":
		// Usage: yaocc file mkdir <path>
		if len(args) < 2 {
			fmt.Println("Usage: yaocc file mkdir <path>")
			return
		}
		targetPath, err := resolvePath(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		if err := os.MkdirAll(targetPath, 0755); err != nil {
			fmt.Printf("Error creating directory: %v\n", err)
			return
		}
		fmt.Printf("Successfully created directory %s\n", args[1])

	case "run":
		// Usage: yaocc file run <path> [args...]
		if len(args) < 2 {
			fmt.Println("Usage: yaocc file run <path> [args...]")
			return
		}
		scriptRelPath := args[1]
		targetPath, err := resolvePath(scriptRelPath)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		// Pass remaining args to the script
		scriptArgs := args[2:]
		executeScript(targetPath, scriptArgs)

	default:
		fmt.Printf("Unknown file command: %s\n", cmd)
	}
}
