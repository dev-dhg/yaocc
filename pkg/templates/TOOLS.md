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

## Creating and Using Skills
You can extend your capabilities by creating new "Skills". A skill is simply a set of instructions and examples stored in a markdown file. and sometimes is accompanied by a script (Python, Bash, JS, etc.)(js preferred) that you register as a new command. Before creating a skill verify it doest exist yet using `yaocc skills list`.

### 1. Create the Script and Skill Definition
First, create the script and the `SKILL.md` file as usual.

 To add a new skill:
1.  Create a directory: `skills/<skill_name>/`
2.  Create a file: `skills/<skill_name>/SKILL.md`
3.  Write the skill definition in frontmatter and instructions in the body.

#### Example: Creating a "Weather" Skill using `file write`

```bash
yaocc file write skills/weather/SKILL.md "---
name: weather
description: Checks the weather of a city.
---
# Weather api

To check the weather of a city run
\`\`\`bash
yaocc skills weather <city>
\`\`\`
```
### 2. Create script and verify it runs 
sometimes you can get away only with SKILL.md
but sometimes you'll need to create the script, in this case

**Note:** You cannot use reserved names like `list`, `register`, `file`, `cron`, etc.

#### Example: Creating a "Weather" Skill script
1. **Create the script:**
```bash
yaocc file write skills/weather/weather.js "console.log(`Weather in ${process.argv[2]}: Sunny`);"
```

2. **Verify it runs:**
verify it runs correctly before you register!
```bash
yaocc file run skills/weather/weather.js Paris
```

3. **Register it:**
Use the `yaocc skills register` command to link a command name to your script.
very important you can register the skill only after it runs correctly in the previous step

```bash
yaocc skills register <command_name> <path_to_script>
```
example:
```bash
yaocc skills register weather skills/weather/weather.js
```

4. **Use it:**
You can now use `yaocc skills weather <city>` directly!
```bash
yaocc skills weather Paris
```

5. **List Skills:**
```bash
yaocc skills list
```

6. **Unregister:**
```bash
yaocc skills unregister weather
```

### 7. Execute Shell Commands
```bash
yaocc exec <command>
```
Executes a shell command on the host machine.
**Note:** This command must be enabled in the server configuration. Dangerous commands are blocked by default.

Example:
```bash
yaocc exec ls -la
```
