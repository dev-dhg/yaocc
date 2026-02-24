# YAOCC Documentation

Welcome to the detailed documentation for YAOCC (Yet Another OpenClaw Clone)! This document covers configuration options, telegram setup, and advanced agent features.

## Configuration Guide (`config.json`)

The `config.json` file is the heart of YAOCC. It dictates how the backend server connects to LLM providers, search engines, and messaging platforms.

### Skills Configuration

The agentic capabilities of YAOCC are defined in the `skills` section:

```json
"skills": {
  "paths": [
    "./skills"
  ],
  "registered": {
    "crypto": "skills/crypto/crypto.js"
  },
  "injectFullSkillText": false
}
```

*   **`paths`**: An array of directories where the agent will search for `SKILL.md` files.
*   **`registered`**: An optional key-value map linking a custom skill name to its precise execution path.
*   **`injectFullSkillText`**: To optimize LLM context window limits and save tokens, YAOCC injects an XML "manifest" of available skills (just the name and description) into the system prompt by default (`false`).
    *   Set to `true` to inject the entire `SKILL.md` instructions for *all* loaded skills.
    *   Set to an array of specific skill names (e.g., `["crypto", "websearch"]`) to inject the full body for those specific skills while using the XML manifest for all others.

### Messaging (Telegram Setup)

To enable the Telegram bot integration:
1. Search for `BotFather` on Telegram.
2. Create a new bot and copy the HTTP API Token.
3. In your `config.json` or `.env` file, set the token:
   ```json
   "messaging": [
     {
       "provider": "telegram",
       "telegram": {
         "enabled": true,
         "botToken": "${TELEGRAM_BOT_TOKEN}",
         "allowedUsers": [
           "YOUR_TELEGRAM_USER_ID"
         ]
       }
     }
   ]
   ```
4. **Important:** Add your numeric Telegram User ID to the `allowedUsers` list to ensure only you can communicate with the bot.

## Core Features

### Cron Jobs

Schedule periodic tasks or LLM prompts using a cron expression.
Example `config.json` addition:
```json
"cron": [
  {
    "name": "daily_greeting",
    "schedule": "0 9 * * *",
    "type": "prompt",
    "prompt": "Say good morning to the user in a friendly way.",
    "targets": [
      { "provider": "telegram", "id": "YOUR_CHAT_ID" }
    ]
  }
]
```

### Server vs. CLI Prompting

*   **`yaocc-server`**: Runs the full continuous session loop, tracks memory, parses configuration on-the-fly, and attaches to messaging providers.
*   **`yaocc prompt`**: Bypasses the agentic memory/skill system and allows you to submit a quick one-off prompt directly to the configured LLM provider.

---

For developer documentation, including how to build custom providers, refer to [DEV.md](DEV.md).
