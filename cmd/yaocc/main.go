package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: yaocc <command> [args]")
		fmt.Println("Commands:")
		fmt.Println("  init    Initialize a new YAOCC project")
		fmt.Println("  chat    Send a message to the YAOCC server")
		fmt.Println("  cron    Manage cron jobs")
		fmt.Println("  file    Manage workspace files")
		fmt.Println("  model   Manage LLM models")
		fmt.Println("  fetch   Fetch a URL content")
		fmt.Println("  websearch Search the web")
		fmt.Println("  skills  Manage and run skills")
		fmt.Println("  prompt  Ask a quick question to the LLM")
		fmt.Println("  exec    Execute shell commands (requires config enable)")
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "init":
		runInit()
	case "chat":
		runChat(os.Args[2:])
	case "cron":
		runCron(os.Args[2:])
	case "model":
		runModel(os.Args[2:])
	case "file":
		runFile(os.Args[2:])
	case "fetch":
		runFetch(os.Args[2:])
	case "websearch":
		runWebSearch(os.Args[2:])
	case "skills":
		runSkills(os.Args[2:])
	case "prompt":
		runPrompt(os.Args[2:])
	case "exec":
		runExec(os.Args[2:])
	default:
		runSkills(os.Args[1:])
	}
}
