package agent

import (
	"fmt"
	"strings"

	"github.com/dev-dhg/yaocc/pkg/llm"
)

// GetBuiltinToolSchemas returns the structured schemas for a built-in skill, or nil if none exists.
// Granularly expanding complex tools into multiple action-specific tools improves LLM accuracy.
func GetBuiltinToolSchemas(skillName, description string) []llm.Tool {
	var tools []llm.Tool

	// Helper to add an action-specific tool
	addTool := func(actionName string, actionDesc string, props map[string]interface{}, req []string) {
		name := fmt.Sprintf("yaocc_%s", strings.ReplaceAll(skillName, "-", "_"))
		if actionName != "" {
			name = fmt.Sprintf("%s_%s", name, actionName)
		}

		if props == nil {
			props = map[string]interface{}{}
		}

		fullDesc := description
		if actionDesc != "" {
			fullDesc = fmt.Sprintf("%s - %s", description, actionDesc)
		}

		tools = append(tools, llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        name,
				Description: fullDesc,
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": props,
					"required":   req,
				},
			},
		})
	}

	prop := func(t, desc string) map[string]interface{} {
		return map[string]interface{}{
			"type":        t,
			"description": desc,
		}
	}

	switch skillName {
	case "cron-manager", "cron_manager", "cron":
		addTool("list", "List all configured cron jobs", nil, nil)
		addTool("add", "Add a new cron job. ALWAYS use this to schedule events, recurring tasks, and future actions.", map[string]interface{}{
			"name":            prop("string", "Name of the cron job"),
			"schedule":        prop("string", "Cron schedule expression, e.g. '0 9 * * *'"),
			"prompt":          prop("string", "The prompt to send to the LLM (for prompt-type jobs)"),
			"script":          prop("string", "Path to a script to execute (for script-type jobs). e.g 'scripts/ping_server.js 192.168.1.1'. You can create this script first using the file-manager skill if it doesn't exist. see cron_manager usage for more info"),
			"use_history":     prop("boolean", "Whether to use target session history state"),
			"target_provider": prop("string", "The messaging provider to target (e.g. telegram). Use 'CURRENT_PROVIDER' as a placeholder to target the current session's provider."),
			"target_id":       prop("string", "The ID of the target chat/session. Use 'CURRENT_SESSION_ID' as a placeholder to target the current session's ID."),
		}, []string{"name", "schedule"})
		addTool("remove", "Remove an existing cron job", map[string]interface{}{
			"name": prop("string", "Name of the cron job"),
		}, []string{"name"})
		addTool("run", "Force run a specific cron job by its index", map[string]interface{}{
			"index": prop("integer", "The index of the job to run (obtain via list action)"),
		}, []string{"index"})

	case "file-manager", "file_manager", "file":
		addTool("read", "Read content of a file", map[string]interface{}{
			"path": prop("string", "Path to the file we want to read"),
		}, []string{"path"})
		addTool("write", "Write or overwrite content to a file", map[string]interface{}{
			"path":    prop("string", "Path to the file"),
			"content": prop("string", "Total content to write"),
		}, []string{"path", "content"})
		addTool("append", "Append content to the end of a file", map[string]interface{}{
			"path":    prop("string", "Path to the file"),
			"content": prop("string", "Content to append"),
		}, []string{"path", "content"})
		addTool("list", "List contents of a directory", map[string]interface{}{
			"path": prop("string", "Optional path to the directory"),
		}, nil)
		addTool("delete", "Delete a file or directory", map[string]interface{}{
			"path": prop("string", "Path of the file to delete"),
		}, []string{"path"})
		addTool("mkdir", "Create a new directory", map[string]interface{}{
			"path": prop("string", "Path of the new directory"),
		}, []string{"path"})
		addTool("run", "Run a file script executable", map[string]interface{}{
			"path": prop("string", "Path to the script"),
			"args": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Arguments to pass",
			},
		}, []string{"path"})

	case "fetch":
		addTool("", "", map[string]interface{}{
			"url": prop("string", "The HTTP/HTTPS URL to fetch."),
		}, []string{"url"})

	case "websearch":
		addTool("", "", map[string]interface{}{
			"query": prop("string", "The search query to search the web for."),
		}, []string{"query"})

	case "prompt":
		addTool("", "", map[string]interface{}{
			"message": prop("string", "The prompt or message to immediately pass to the LLM statelessly."),
		}, []string{"message"})

	case "skills":
		addTool("register", "Register a new skill from a script", map[string]interface{}{
			"name": prop("string", "The name of the skill to register."),
			"path": prop("string", "The path to the script."),
		}, []string{"name", "path"})
		addTool("unregister", "Unregister an existing skill", map[string]interface{}{
			"name": prop("string", "The name of the skill."),
		}, []string{"name"})
		addTool("list", "List all registered and built-in skills", nil, nil)
		addTool("get", "Read the SKILL.md instructions for a skill", map[string]interface{}{
			"name": prop("string", "The name of the skill."),
		}, []string{"name"})
		addTool("tutorial", "Read the tutorial on creating skills", nil, nil)
		addTool("help", "Print help for skills management", nil, nil)

		addTool("run", "Execute a registered YAOCC custom skill explicitly.", map[string]interface{}{
			"name": prop("string", "The name of the custom skill you want to run."),
			"args": prop("string", "The command line string arguments to specifically pass to the custom skill."),
		}, []string{"name"})

	default:
		return nil
	}

	return tools
}

// BuildBuiltinCommandArgs converts the newly structured granular tool calls back into CLI strings.
func BuildBuiltinCommandArgs(toolName string, rawArgs map[string]interface{}) ([]string, error) {
	baseName := strings.TrimPrefix(toolName, "yaocc_")

	switch {
	case strings.HasPrefix(baseName, "cron"):
		action := strings.TrimPrefix(baseName, "cron-manager_")
		action = strings.TrimPrefix(action, "cron_manager_")
		action = strings.TrimPrefix(action, "cron_")
		if action == "cron-manager" || action == "cron" || action == "cron_manager" {
			action = ""
		}

		args := []string{"cron"}
		if action != "" {
			args = append(args, action)
		}

		switch action {
		case "add":
			if name, ok := rawArgs["name"].(string); ok && name != "" {
				args = append(args, "--name", name)
			}
			if schedule, ok := rawArgs["schedule"].(string); ok && schedule != "" {
				args = append(args, "--schedule", schedule)
			}
			if prompt, ok := rawArgs["prompt"].(string); ok && prompt != "" {
				args = append(args, "--prompt", prompt)
			}
			if script, ok := rawArgs["script"].(string); ok && script != "" {
				args = append(args, "--script", script)
			}
			if useHistory, ok := rawArgs["use_history"].(bool); ok && useHistory {
				args = append(args, "--use-history")
			}
			if targetProvider, ok := rawArgs["target_provider"].(string); ok && targetProvider != "" {
				args = append(args, "--target-provider", targetProvider)
			}
			if targetID, ok := rawArgs["target_id"].(string); ok && targetID != "" {
				args = append(args, "--target-id", targetID)
			}
		case "remove":
			if name, ok := rawArgs["name"].(string); ok {
				args = append(args, name)
			}
		case "run":
			indexFloat, ok := rawArgs["index"].(float64)
			if !ok {
				if indexInt, ok2 := rawArgs["index"].(int); ok2 {
					indexFloat = float64(indexInt)
					ok = true
				}
			}
			if ok {
				args = append(args, fmt.Sprintf("%d", int(indexFloat)))
			}
		}
		return args, nil

	case strings.HasPrefix(baseName, "file"):
		action := strings.TrimPrefix(baseName, "file-manager_")
		action = strings.TrimPrefix(action, "file_manager_")
		action = strings.TrimPrefix(action, "file_")
		if action == "file-manager" || action == "file" || action == "file_manager" {
			action = ""
		}

		args := []string{"file"}
		if action != "" {
			args = append(args, action)
		}

		if path, ok := rawArgs["path"].(string); ok && path != "" {
			args = append(args, path)
		}

		if content, ok := rawArgs["content"].(string); ok && content != "" {
			args = append(args, content)
		}

		if runArgs, ok := rawArgs["args"].([]interface{}); ok {
			for _, a := range runArgs {
				args = append(args, fmt.Sprintf("%v", a))
			}
		}
		return args, nil

	case baseName == "fetch":
		if url, ok := rawArgs["url"].(string); ok {
			return []string{"fetch", url}, nil
		}
		return []string{"fetch"}, fmt.Errorf("missing url")

	case baseName == "websearch":
		if query, ok := rawArgs["query"].(string); ok {
			return []string{"websearch", query}, nil
		}
		return []string{"websearch"}, fmt.Errorf("missing query")

	case baseName == "prompt":
		if msg, ok := rawArgs["message"].(string); ok {
			return []string{"prompt", msg}, nil
		}
		return []string{"prompt"}, fmt.Errorf("missing message")

	case strings.HasPrefix(baseName, "skills"):
		action := strings.TrimPrefix(baseName, "skills_")
		if action == "skills" {
			action = ""
		}

		// Because yaocc_skills_run is invoked by the CLI using `yaocc <name> <args>`
		if action == "run" {
			name, _ := rawArgs["name"].(string)
			argsStr, _ := rawArgs["args"].(string)
			// Return just the skill name and args. Agent.Run will prepend 'yaocc '
			return []string{name, argsStr}, nil
		}

		args := []string{"skills"}
		if action != "" {
			args = append(args, action)
		}

		if name, ok := rawArgs["name"].(string); ok {
			args = append(args, name)
		}
		if path, ok := rawArgs["path"].(string); ok {
			args = append(args, path)
		}
		return args, nil
	}

	return nil, fmt.Errorf("no string builder mapped for %s", toolName)
}
