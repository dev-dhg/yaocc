---
name: cron_manager
description: Manage cron jobs and scheduled tasks. You MUST always use this tool to schedule events, recurring tasks, or delayed actions.
metadata: { "type": "remote" }
tags:
  - built-in
---

# Cron Manager

Use this skill to list or manage cron jobs. **You MUST always use this tool when the user asks to schedule an event, set a reminder, or run a task at a specific time.**

## Usage

```bash
# List all configured cron jobs
yaocc cron list
```

### Adding Cron Jobs

**1. Prompt-Based Jobs**
The agent will execute this instruction directly on schedule.
```bash
yaocc cron add --name "morning_greet" --schedule "0 9 * * *" --prompt "Tell me a joke about programming."
```

**2. Script-Based Jobs (NO PROMPT)**
The agent will run the script and send the RAW output to the target. No AI processing occurs. Useful for simple logs/alerts.
```bash
yaocc cron add --name "server_ping" --schedule "@hourly" --script "scripts/ping_server.js"
```

**3. Script + Prompt Jobs**
The agent will run the script, and then use the output as context to answer the prompt. Useful for analyzing logs or making decisions based on script output.
```bash
yaocc cron add --name "smart_ping" --schedule "@hourly" --script "scripts/ping_server.js 192.168.1.1" --prompt "Analyze this ping result. If the server is down or latency is > 500ms, alert me with a scary message."
```

### Dynamic Targets and Context

**With Targets (e.g. Telegram)**
> [!IMPORTANT]
> Use `CURRENT_PROVIDER` and `CURRENT_SESSION_ID` as placeholders when creating a job for the current user. The system will automatically replace them with the current user's session details!

```bash
yaocc cron add --name "morning_quote" --schedule "0 8 * * *" --prompt "Send me a quote" --target-provider "CURRENT_PROVIDER" --target-id "CURRENT_SESSION_ID"
```

**Context Aware (Use History)**
By default, cron jobs are stateless (no memory of past conversations). Add `--use-history` to execute the prompt within the context of the target's session, allowing the agent to reference previous messages.
```bash
yaocc cron add --name "follow_up" --schedule "0 10 * * *" --prompt "Do you have any updates on the last topic we discussed?" --use-history --target-provider "CURRENT_PROVIDER" --target-id "CURRENT_SESSION_ID"
```

### Removing Cron Jobs
```bash
yaocc cron remove "morning_greet"
```

## Workflow for Scripted Tasks

To set up a complex check (e.g., "Check server status"):

1. **Write the script** (using the `file_manager` skill):
   ```bash
   yaocc file write scripts/check_status.js "fetch('https://example.com').then(r => console.log('Status:', r.status)).catch(e => console.error('Error:', e.message));"
   ```

2. **Register the Cron Job** (using Node.js to run the JavaScript file):
   ```bash
   # Simple Alert (Raw Output)
   yaocc cron add --name "status_check" --schedule "*/5 * * * *" --script "scripts/check_status.js" --target-provider "CURRENT_PROVIDER" --target-id "CURRENT_SESSION_ID"
   ```

3. **Result**:
   - The Scheduler runs the script every 5 minutes.
   - If the script outputs text (e.g., "Status: 200"), it is sent directly to your active chat.
   - The AI is NOT invoked for the execution, saving tokens and time.
