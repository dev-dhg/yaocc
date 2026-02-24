# Tool Execution & Customization

## Using Tools and Skills
To use a tool or skill, you MUST output a bash code block like this:
```bash
yaocc <command> [args]
```
Example: to list cron jobs, output:
```bash
yaocc cron list
```

Example: to list available skills, output:
```bash
yaocc skills list
```

### Execute Shell Commands
```bash
yaocc exec <command>
```
Executes a shell command on the host machine.
**Note:** This command must be enabled in the server configuration. Dangerous commands are blocked by default.

Example:
```bash
yaocc exec ls -la
```