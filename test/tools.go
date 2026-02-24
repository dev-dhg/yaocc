//go:build ignore

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/dev-dhg/yaocc/pkg/agent"
)

func main() {
	// Simple diagnostic script to output how yaocc constructs CLI aliases natively.
	// Usage: go run test/tools.go --filemanager
	//        go run test/tools.go --cron
	//        go run test/tools.go --req '{"path":"memory/2026-02-24.md","content":"hello"}' --tool yaocc_file_manager_append

	dumpCron := flag.Bool("cron", false, "Dump cron schemas")
	dumpFile := flag.Bool("filemanager", false, "Dump file schemas")
	dumpSkills := flag.Bool("skills", false, "Dump skills schemas")

	reqStr := flag.String("req", "", "JSON string for a simulated LLM request object")
	toolName := flag.String("tool", "", "Tool name to simulate against the req string")

	flag.Parse()

	if *dumpCron {
		schemas := agent.GetBuiltinToolSchemas("cron_manager", "cron description")
		dump("Cron Manager", schemas)
	}
	if *dumpFile {
		schemas := agent.GetBuiltinToolSchemas("file_manager", "file description")
		dump("File Manager", schemas)
	}
	if *dumpSkills {
		schemas := agent.GetBuiltinToolSchemas("skills", "skills description")
		dump("Skills Manager", schemas)
	}

	if *reqStr != "" && *toolName != "" {
		fmt.Printf("--- Simulating Agent.Run intercept ---\n")
		fmt.Printf("Tool Name: %s\n", *toolName)
		fmt.Printf("JSON Payload: %s\n", *reqStr)

		var rawArgs map[string]interface{}
		if err := json.Unmarshal([]byte(*reqStr), &rawArgs); err != nil {
			fmt.Printf("Error unmarshalling req: %v\n", err)
			os.Exit(1)
		}

		args, err := agent.BuildBuiltinCommandArgs(*toolName, rawArgs)
		if err != nil {
			fmt.Printf("Error building command: %v\n", err)
			os.Exit(1)
		}

		var cmdBuilder strings.Builder
		cmdBuilder.WriteString("yaocc")
		for _, a := range args {
			// Add basic shell-safe visual quoting if the string has spaces
			if strings.Contains(a, " ") || strings.Contains(a, "|") || strings.Contains(a, "-") {
				cmdBuilder.WriteString(fmt.Sprintf(" \"%s\"", a))
			} else {
				cmdBuilder.WriteString(fmt.Sprintf(" %s", a))
			}
		}

		fmt.Printf("\nGenerated CLI Execution Command:\n👉 %s\n\n", cmdBuilder.String())

		fmt.Println("Note: You can test this exact string in your terminal to see if the CLI errors out.")
	}

	if !*dumpCron && !*dumpFile && !*dumpSkills && (*reqStr == "" || *toolName == "") {
		fmt.Println("Please provide a flag: --cron, --filemanager, --skills, or --req '...' --tool 'yaocc_...'")
		flag.Usage()
	}
}

func dump(title string, obj interface{}) {
	fmt.Printf("--- %s ---\n", title)
	b, _ := json.MarshalIndent(obj, "", "  ")
	fmt.Println(string(b))
	fmt.Println()
}
