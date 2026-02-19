package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// resolveSafePath ensures that the path is within the config directory and is not a sensitive file.
func resolveSafePath(configDir, inputPath string) (string, error) {
	// 1. Join with configDir
	fullPath := filepath.Join(configDir, inputPath)

	// 2. Clean path
	cleanPath := filepath.Clean(fullPath)

	// 3. Check for path escape
	absConfigDir, _ := filepath.Abs(configDir)
	absPath, _ := filepath.Abs(cleanPath)

	if !strings.HasPrefix(absPath, absConfigDir) {
		return "", fmt.Errorf("access denied: path escapes configuration directory")
	}

	// 4. Blacklist Check
	baseName := filepath.Base(absPath)
	if baseName == "config.json" || baseName == ".env" || baseName == "agent.log" {
		return "", fmt.Errorf("access denied: cannot access sensitive configuration file '%s'", baseName)
	}

	return absPath, nil
}

// executeScript validates and runs a script file.
func executeScript(targetPath string, args []string) {
	// Security Check 1: Extension Whitelist
	ext := strings.ToLower(filepath.Ext(targetPath))
	allowedExts := map[string]bool{
		".sh": true, ".ps1": true, ".bat": true, ".cmd": true, ".py": true, ".js": true,
	}
	if !allowedExts[ext] {
		fmt.Printf("Error: Execution denied. File extension '%s' is not allowed.\n", ext)
		return
	}

	// Security Check 2: Content Scan
	// Read file content
	contentBytes, err := os.ReadFile(targetPath)
	if err != nil {
		fmt.Printf("Error reading script file: %v\n", err)
		return
	}
	content := string(contentBytes)

	// Basic heuristic blacklist
	forbidden := []string{
		"rm -rf /", "rm -rf /*",
		"sudo ",
		"cmd.exe", "powershell.exe", // Trying to spawn shells
		"Invoke-Expression", "IEX ",
		"bash -i", "/bin/sh -i",
	}

	for _, bad := range forbidden {
		if strings.Contains(content, bad) {
			fmt.Printf("Error: Execution denied. Potentially dangerous pattern detected: '%s'\n", bad)
			return
		}
	}

	// Execution
	var cmd *exec.Cmd

	// Prepare arguments: script path first, then user args
	// Some interpreters need specific flag for args or just append them.
	// Python: python script.py arg1 arg2
	// Node: node script.js arg1 arg2
	// Bash: sh -c script.sh ... wait, sh -c expects a string? No, sh script.sh is better if it's a file.
	// But duplicate logic used "sh -c targetPath".
	// If targetPath is a file, `sh targetPath` is correct. `sh -c` executes the string argument.
	// The original code was: cmd = exec.Command("sh", "-c", targetPath)
	// If targetPath is "scripts/foo.sh", `sh -c scripts/foo.sh` might try to execute the path as a command?
	// Actually `sh -c` takes a command string. If `targetPath` is an executable, it runs it.
	// But usually `sh script.sh` is preferred.
	// let's stick to previous logic to avoid breaking change, OR fix it if it was slightly wrong.
	// `sh -c /path/to/script.sh` works if the script has +x and hashbang?
	// Let's assume the previous logic worked. But wait, I should append args.

	runArgs := append([]string{targetPath}, args...)

	switch ext {
	case ".sh":
		// If we use sh -c, args are passed to $0, $1...
		// cmd = exec.Command("sh", append([]string{"-c", targetPath}, args...)...)
		// But let's try to be more standard: sh script.sh args...
		cmd = exec.Command("sh", runArgs...)
	case ".ps1":
		// powershell -File script.ps1 args...
		cmd = exec.Command("powershell", append([]string{"-File"}, runArgs...)...)
	case ".bat", ".cmd":
		// cmd /c script.bat args...
		cmd = exec.Command("cmd", append([]string{"/c"}, runArgs...)...)
	case ".py":
		cmd = exec.Command("python", runArgs...)
	case ".js":
		cmd = exec.Command("node", runArgs...)
	}

	if cmd == nil {
		fmt.Printf("Error: No executor defined for %s\n", ext)
		return
	}

	// Connect Stdin/Stdout/Stderr?
	// Original used CombinedOutput.
	// If we want interactive or streaming, we should pipe.
	// But original was not interactive.
	// Let's stick to CombinedOutput for now as per original.

	out, err := cmd.CombinedOutput()
	fmt.Printf("Output:\n%s\n", string(out))
	if err != nil {
		fmt.Printf("Execution failed: %v\n", err)
	}
}
