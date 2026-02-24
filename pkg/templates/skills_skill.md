---
name: skills
description: Register, unregister, list, and manage custom user skills.
tags:
  - built-in
---

# Skills Management

The `skills` command allows you to manage custom, user-provided skills locally in the configuration environment.
You can register new scripts to be executable as skills, unregister them, list all available skills, or fetch the instructions (SKILL.md) for a registered skill.

## Usage

```bash
yaocc skills <command> [args]
```

### Commands

- **`register <name> <path>`**: Registers a new local script or executable as a skill tied to `<name>`.
- **`unregister <name>`**: Removes a registered custom skill from the configuration memory.
- **`list`**: Lists all known built-in skills and custom registered skills.
- **`get <name>`**: Reads the `SKILL.md` instruction manual tied to a specific registered capability.
- **`tutorial`**: Reads the YAOCC comprehensive tutorial on writing your own local scripts!
