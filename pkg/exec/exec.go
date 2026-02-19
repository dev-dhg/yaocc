package exec

import (
	"context"
	"fmt"
	osexec "os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/dev-dhg/yaocc/pkg/config"
)

// DefaultBlacklist contains patterns that are blocked by default.
// This list covers common dangerous commands across Bash, PowerShell, and CMD.
var DefaultBlacklist = []string{
	// File System Destruction
	"rm -rf", "rm -r -f", "rm -f -r", // Basic recursive delete
	"del /f /s /q", "rd /s /q", // Windows delete
	"mkfs", "fdisk", "dd if=", // Disk formatting
	"Format-Volume", // PowerShell disk format
	":(){:|:&};:",   // Fork bombs

	// Privilege Escalation
	"sudo", "su -", "runas", "doas",

	// Shell Spawning / Execution
	"bash -i", "/bin/sh -i", // Interactive shells
	"Invoke-Expression", "IEX ", // PowerShell eval
	"eval ", "exec(", // Generic eval
	"cmd.exe", "powershell.exe", // Shell spawning

	// Network Abuse (Bind shells, etc)
	"nc -l", "ncat -l", // Listening ports

	// Sensitive Files
	".env", "config.json",
	"/etc/passwd", "/etc/shadow",
	"C:\\Windows\\System32\\config\\SAM",
}

// ValidateCommand checks if a command is allowed based on the configuration.
func ValidateCommand(cmd string, options *config.CmdOptions) error {
	if options == nil {
		// If no options provided, use default blacklist
		return validateBlacklist(cmd, DefaultBlacklist)
	}

	// 1. Whitelist (Highest Priority)
	if len(options.Whitelist) > 0 {
		allowed := false
		for _, pattern := range options.Whitelist {
			// Simple contains check for now. Could be regex in future.
			if strings.Contains(cmd, pattern) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("command denied: not in whitelist")
		}
		return nil
	}

	// 2. Blacklist
	blacklist := options.Blacklist
	if len(blacklist) == 0 {
		blacklist = DefaultBlacklist
	}
	return validateBlacklist(cmd, blacklist)
}

func validateBlacklist(cmd string, blacklist []string) error {
	for _, pattern := range blacklist {
		if strings.Contains(cmd, pattern) {
			return fmt.Errorf("command denied: blocked by pattern '%s'", pattern)
		}
	}
	return nil
}

// RunCommand executes a command with a timeout and specific working directory.
func RunCommand(cmdStr string, configDir string) (string, error) {
	// 30 Seconds Execution Timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var cmd *osexec.Cmd

	if runtime.GOOS == "windows" {
		cmd = osexec.CommandContext(ctx, "powershell", "-Command", cmdStr)
	} else {
		cmd = osexec.CommandContext(ctx, "sh", "-c", cmdStr)
	}

	// Set Working Directory
	cmd.Dir = configDir

	// Capture output
	out, err := cmd.CombinedOutput()
	output := string(out)

	if ctx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("execution timed out after 30 seconds")
	}

	if err != nil {
		return output, err
	}

	return output, nil
}
