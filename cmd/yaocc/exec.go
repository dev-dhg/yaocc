package main

import (
	"fmt"
	"strings"

	"github.com/dev-dhg/yaocc/pkg/config"
	"github.com/dev-dhg/yaocc/pkg/exec"
)

func runExec(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: yaocc exec <command...>")
		return
	}

	// 1. Load Configuration
	cfg, configDir, _, err := config.LoadConfig("")
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		return
	}

	// 2. Check if Enabled
	if !cfg.IsCmdEnabled("exec") {
		fmt.Println("Error: 'exec' command is disabled by default. Enable it in config.json under 'cmds'.")
		return
	}

	// 3. Reconstruct Command
	cmdStr := strings.Join(args, " ")

	// 4. Validate Security
	cmdConfig := cfg.GetCmdConfig("exec")
	var options *config.CmdOptions
	if cmdConfig != nil {
		options = cmdConfig.Options
	}

	if err := exec.ValidateCommand(cmdStr, options); err != nil {
		fmt.Printf("Security Error: %v\n", err)
		return
	}

	// 5. Execute
	output, err := exec.RunCommand(cmdStr, configDir)
	if output != "" {
		fmt.Printf("%s\n", output)
	}
	if err != nil {
		fmt.Printf("Execution Warning/Error: %v\n", err)
	}
}
