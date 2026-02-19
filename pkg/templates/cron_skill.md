---
name: cron_manager
description: Manage cron jobs and scheduled tasks.
metadata: { "type": "remote" }
---

# Cron Manager

Use this skill to list or manage cron jobs.

## Usage

yaocc cron list
```

Add a new cron job:
```bash
# Type: prompt (default)
# The agent will execute this instruction directly.
yaocc cron add --name "morning_greet" --schedule "0 9 * * *" --prompt "Tell me a joke about programming."

# Type: script (NO PROMPT)
# The agent will run the script and send the RAW output to the target.
# No AI processing will occur. Useful for simple logs/alerts.
yaocc cron add --name "disk_check" --schedule "@hourly" --script "scripts/check_disk.ps1"

# Type: script + prompt
# The agent will run the script, and then use the output as context to answer the prompt.
# Useful for analyzing logs or making decisions based on script output.
yaocc cron add --name "smart_disk_check" --schedule "@hourly" --script "scripts/check_disk.ps1" --prompt "Analyze this disk report. If usage is > 90%, alert me with a scary message."

# With Targets (e.g. Telegram)
# IMPORTANT: Use "YOUR_PROVIDER" and "YOUR_CHAT_ID" as placeholders. 
# The system will automatically replace them with the current user's details.
yaocc cron add --name "morning_quote" --schedule "0 8 * * *" --prompt "Send me a quote" --target-provider "YOUR_PROVIDER" --target-id "YOUR_CHAT_ID"

# Context Aware (Use History)
# By default, cron jobs are stateless (no memory of past conversations).
# Add --use-history to execute the prompt within the context of the target's session.
# This allows the agent to reference previous messages.
yaocc cron add --name "follow_up" --schedule "0 10 * * *" --prompt "Do you have any updates on the last topic we discussed?" --use-history --target-provider "YOUR_PROVIDER" --target-id "YOUR_CHAT_ID"
```

Remove a cron job:
```bash
yaocc cron remove "morning_greet"
```

## Workflow for Scripted Tasks

To set up a complex check (e.g., "Check disk space"):

1. **Write the script** (using `file` skill):
   ```bash
   yaocc file write scripts/check_disk.ps1 "Get-PSDrive C | Where-Object { $_.Free / $_.Used -lt 0.1 } | ForEach-Object { Write-Output 'WARNING: Disk C is low on space!' }"
   ```

2. **Register the Cron Job**:
   ```bash
   # Simple Alert (Raw Output)
   yaocc cron add --name "disk_check" --schedule "@hourly" --script "scripts/check_disk.ps1" --target-provider "telegram" --target-id "YOUR_CHAT_ID"
   ```

3. **Result**:
   - The Scheduler runs the script every hour.
   - If the script outputs text (e.g., "WARNING..."), it is sent directly to your Telegram.
   - The AI is NOT invoked, saving tokens and time.
