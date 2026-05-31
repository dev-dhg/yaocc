# Skill Creation Tutorial

Here is the comprehensive guide on how to create custom skills for YAOCC.
You can extend your capabilities by creating new "Skills". A skill is simply a subdirectory under `skills/` containing a `SKILL.md` (or `skill.md`) file. The skill manual tells the agent (and users) what the skill is for, how to use it, and what scripts or commands to run if it is script-backed.

### 1. Create the Skill Directory and Manual
To add a new skill:
1.  Create a directory: `skills/<skill_name>/`
2.  Create a file: `skills/<skill_name>/SKILL.md`
3.  Write the skill definition in YAML frontmatter and instructions/manual in the body.

#### Example: Creating a "Weather" Skill

Create the directory and write `skills/weather/SKILL.md`:
```bash
yaocc file write skills/weather/SKILL.md "---
name: weather
description: Checks the weather of a city.
---
# Weather Manual

To check the weather of a city, run:
\`\`\`bash
yaocc file run skills/weather/weather.js <city>
\`\`\`
"
```

### 2. Create the Script
If your skill is backed by a custom script (e.g. JavaScript, Python, Bash), place it in the same directory (or anywhere inside your workspace). JavaScript is preferred as it runs natively using Node.js.

Create `skills/weather/weather.js`:
```bash
yaocc file write skills/weather/weather.js "console.log('Weather in ' + process.argv[2] + ': Sunny');"
```

### 3. Verify and Use
Your skill is automatically discovered! There is no registration step.

1. **List Skills**:
   See it in the list:
   ```bash
   yaocc skills list
   ```

2. **Get Skill Manual**:
   Read its manual instructions:
   ```bash
   yaocc skills get weather
   ```

3. **Run / Query the Skill**:
   Execute the skill command. This will output the skill manual and the arguments passed, so the agent understands the context and can run any underlying script instructions:
   ```bash
   yaocc skills run weather Paris
   # or simply:
   yaocc weather Paris
   # preferable
   yaocc file run skills/weather/weather.js Paris
   ```

That's it! Adding skills is completely automated and lightweight.
