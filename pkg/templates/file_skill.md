---
name: file_manager
description: Read, write, run, delete and list files in the workspace.
---

# File System

Use this skill to read, write, and list files in your workspace. This is essential for updating your memory (`MEMORY.md`), identity (`IDENTITY.md`), and learning about the user (`USER.md`).

IMPORTANT: this can't run terminal/shell commands, it can just run the commands listed below as instructed

## Usage

### List files
```bash
yaocc file list [directory]
```

### Read a file
```bash
yaocc file read <path>
```

### Write a file
```bash
yaocc file write <path> "content"
```

### Append to a file
```bash
yaocc file append <path> "content"
```
**IMPORTANT**: All paths are relative to the configuration directory. You cannot access files outside of this directory (e.g., `../` is forbidden).

### Create a directory
```bash
yaocc file mkdir <path>
```

### Delete a file
```bash
yaocc file delete <path>
```

### Run a script
```bash
yaocc file run <script_path>
```
Executes a script (e.g., `.sh`, `.py`, `.js`, `.ps1`) located relative to the configuration directory.
**Security**: Scripts containing dangerous commands (e.g., `rm -rf`, `cmd.exe`) will be blocked.

Example:
```bash
yaocc file run scripts/myscript.sh
```

Example:
```bash
yaocc file write IDENTITY.md "# Identity\n- Name: Atlas"
```
