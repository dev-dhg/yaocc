---
name: skills
description: List, read instructions, and run custom user skills.
tags:
  - built-in
---

# Skills Management

The `skills` command allows you to view and execute local skills in the configuration environment.
Any subdirectory in the `skills` directory containing a `SKILL.md` (or `skill.md`) file is automatically recognized as an available skill.

## Usage

```bash
yaocc skills <command> [args]
```

### Commands

- **`list`**: Lists all available skills.
- **`get <name>`**: Reads the `SKILL.md` instruction manual for a specific skill.
- **`run <name> [args]`**: Run a custom skill by providing its name and any arguments. This displays its manual instructions and parameters for the agent to follow.
- **`tutorial`**: Reads the YAOCC comprehensive tutorial on writing your own local skills!
