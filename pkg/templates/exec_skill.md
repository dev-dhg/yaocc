---
name: exec
description: Execute shell commands on the host machine.
---

# Command Execution

Use this skill to execute shell commands on the host machine.

> [!WARNING]
> This skill executes commands directly on the host. Security restrictions (blacklist/whitelist) are applied by the server configuration.

## Usage

### Execute a Command
```bash
yaocc exec <command>
```

Examples:
```bash
yaocc exec ls -la
```
```bash
yaocc exec git status
```
```bash
yaocc exec grep "something" file.txt
```

### Security
- **Restricted Commands**: Dangerous commands like `rm -rf`, `sudo`, `mkfs` are blocked by default.
- **Whitelist**: If the server is configured with a whitelist, ONLY allowed commands will work.
- **Paths**: Commands are executed in the configuration directory.
