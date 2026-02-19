# Agents in YAOCC

An **Agent** in YAOCC is an autonomous entity powered by an LLM (Large Language Model) that can interact with the world through **Skills**.

## Development Guidelines

## General

after every change always update README.md and DEV.md if needed

### Terminal
you are using windows 11 and windows powershell as terminal, any command you run you be compatible with windows powershell

### Build Instructions
Always build binaries into the `build/` directory to keep the workspace organized.

```bash
# Build Server
go build -o build/yaocc-server.exe ./cmd/yaocc-server

# Build CLI
go build -o build/yaocc.exe ./cmd/yaocc
```

## API
if a new API endpoint is added, always update the `openapi.yaml` file

## Core Components

Every agent in a YAOCC workspace is defined by a set of core Markdown files found in the root directory. These files form the agent's "context" or "personality":

### 1. SOUL.md
The `SOUL.md` file defines the agent's core personality, directives, and high-level goals. It is the most important file for shaping how the agent behaves.
*   **Purpose**: Define who the agent is.
*   **Example**: "You are a helpful coding assistant..."

### 2. IDENTITY.md
The `IDENTITY.md` file provides specific details about the agent's current identity configuration, often used for more transient or role-specific instructions.
*   **Purpose**: Define the current role or specific instructions for this instance.

### 3. USER.md
The `USER.md` file contains information about the human user the agent is assisting. The agent reads this to understand preferences, contact details, or other improved context.
*   **Purpose**: Personalization and user context.

### 4. MEMORY.md
The `MEMORY.md` file acts as the agent's long-term memory. The agent can read this file to recall important facts, decisions, or project context across different sessions.
*   **Purpose**: Long-term persistence.

## Skills

Agents extend their capabilities through **Skills**. A skill is a tool or command that the agent can execute.

*   **Discovery**: Skills are loaded from the `skills/` directory.
*   **Definition**: Each skill is defined by a `SKILL.md` file containing metadata (name, description) and usage instructions.
*   **Execution**: The agent outputs a specific command block (e.g., `yaocc fetch <url>`) to invoke a skill.

### Standard Skills
*   **Cron**: detailed in `skills/cron/SKILL.md`. Manage scheduled tasks.
*   **File**: detailed in `skills/file/SKILL.md`. Manage file system operations.
*   **Fetch**: detailed in `skills/fetch/SKILL.md`. Retrieve web content.
