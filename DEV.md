# Developer Guide

Welcome to the YAOCC development guide! This document covers how to build, test, and contribute to the project.

## Prerequisites

- **Go**: Version 1.23 or higher.
- **Git**: For version control.

## Project Structure

- `cmd/`: Application entry points.
    - `yaocc/`: The CLI tool.
    - `yaocc-server/`: The backend server (if applicable).
- `pkg/`: Library code.
    - `agent/`: Core agent logic.
    - `config/`: Configuration handling.
    - `llm/`: LLM client implementation.
    - `skills/`: Skill loading and management.
    - `version/`: Build-time version info (injected via ldflags).
    - `messaging/`: Messaging provider implementations.
        - `telegram/`: Telegram bot client.
- `skills/`: Default skill definitions and implementations.

## Building

To build the project, run:

```bash
# Build CLI
go build -o build/yaocc.exe ./cmd/yaocc

# Build Server
go build -o build/yaocc-server.exe ./cmd/yaocc-server
```

## Versioning

YAOCC uses Go **ldflags** to inject version information at build time. The `pkg/version` package exposes three variables:

| Variable | Description | Default |
|---|---|---|
| `Version` | Semantic version tag (e.g. `v1.2.3`) | `dev` |
| `Commit` | Short Git commit SHA | `unknown` |
| `BuildDate` | ISO 8601 build timestamp | `unknown` |

### Checking the Version

```bash
# CLI
yaocc version
# or
yaocc --version

# Server
yaocc-server --version
```

### Building with Version Locally

#### Native Build (PowerShell)

```powershell
$VERSION = "v1.0.0"
$COMMIT = (git rev-parse --short HEAD)
$DATE = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")
$LDFLAGS = "-X github.com/dev-dhg/yaocc/pkg/version.Version=$VERSION -X github.com/dev-dhg/yaocc/pkg/version.Commit=$COMMIT -X github.com/dev-dhg/yaocc/pkg/version.BuildDate=$DATE"

go build -ldflags $LDFLAGS -o build/yaocc.exe ./cmd/yaocc
go build -ldflags $LDFLAGS -o build/yaocc-server.exe ./cmd/yaocc-server
```

#### Native Build (Bash / Linux / macOS)

```bash
VERSION="v1.0.0"
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS="-X github.com/dev-dhg/yaocc/pkg/version.Version=${VERSION} -X github.com/dev-dhg/yaocc/pkg/version.Commit=${COMMIT} -X github.com/dev-dhg/yaocc/pkg/version.BuildDate=${DATE}"

go build -ldflags "$LDFLAGS" -o build/yaocc ./cmd/yaocc
go build -ldflags "$LDFLAGS" -o build/yaocc-server ./cmd/yaocc-server
```

#### Docker Build with Version

The Dockerfile accepts `YAOCC_VERSION`, `YAOCC_COMMIT`, and `YAOCC_BUILD_DATE` as build args:

```bash
docker build \
  --build-arg YAOCC_VERSION="v1.0.0" \
  --build-arg YAOCC_COMMIT="$(git rev-parse --short HEAD)" \
  --build-arg YAOCC_BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --build-arg YAOCC_BASE_IMAGE="node:24-alpine" \
  --build-arg YAOCC_DOCKER_APK_PACKAGES="git nano curl unzip python3" \
  --build-arg YAOCC_USER="node" \
  -t yaocc:v1.0.0 .
```

Or with Docker Compose (PowerShell):

```powershell
$env:YAOCC_VERSION="v1.0.0"
$env:YAOCC_COMMIT=(git rev-parse --short HEAD)
$env:YAOCC_BUILD_DATE=(Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")
docker compose build --build-arg YAOCC_VERSION=$env:YAOCC_VERSION --build-arg YAOCC_COMMIT=$env:YAOCC_COMMIT --build-arg YAOCC_BUILD_DATE=$env:YAOCC_BUILD_DATE
```

> **Note:** If you don't pass version args, the binaries default to `dev` / `unknown`. This is fine for local development.

### CI/CD (Automatic)

Both GitLab CI and GitHub Actions automatically inject version info:
- **Tagged builds** (`v*`): Version is set to the tag name (e.g. `v1.2.3`)
- **Branch builds**: Version is set to `dev-<short-sha>`

## Configuration

YAOCC uses a split configuration architecture:
- **`config.json`**: General settings (models, messaging, server, storage, websearch, session, etc.).
- **`cron.json`**: Cron job definitions (independently watched, changes only reload the scheduler).

Copy the example files to get started:
- `config.json.example` → `config.json`
- `cron.json.example` → `cron.json`

Environment variables can be used in config files using `${VAR_NAME}` syntax.

### Automatic Migration
If you have an existing `config.json` with `cron` fields, YAOCC will automatically migrate them to `cron.json` on first startup and clean up the old fields from `config.json`.

## Testing

Run unit tests with:

```bash
go test ./...
```

To run the manual LLM integration test:

```bash
go run test/manual_llm.go
```

## Messaging Providers

YAOCC supports multiple messaging providers via a unified interface.

### Adding a New Messaging Provider

1.  **Create a new package/directory**: Under `pkg/messaging/<provider_name>`.
2.  **Implement the Provider Interface**:
    Your struct must implement the `Provider` interface defined in `pkg/messaging/provider.go`.

    ```go
    type Provider interface {
        Name() string
        Start()
        SendMessage(targetID string, message string) error
        SendImage(targetID string, url string, caption string) error
        SendAudio(targetID string, url string, caption string) error
        SendVideo(targetID string, url string, caption string) error
        SendDocument(targetID string, url string, caption string) error
    }
    ```

3.  **Register the Provider**:
    In `cmd/yaocc-server/main.go`, initialize your provider based on configuration and add it to the `providers` map before creating the `Scheduler`.

    ```go
    // Example in main.go
    if msgCfg.Provider == "discord" {
        discordClient := discord.NewClient(...)
        go discordClient.Start()
        providers["discord"] = discordClient
    }
    ```

4.  **Configuration**:
    Update `pkg/config/config.go` to include any specific configuration fields your provider needs in `MessagingProviderConfig` or a sub-struct.


## Agent Capabilities & Media Support

The Agent can be extended to support rich media interactions. Currently, the Agent supports both **sending** and **receiving** multimodal media (Images, Audio, Video, and Documents).

### Receiving Media & Multimodal Support

YAOCC supports receiving photos, voice messages, and audio files via Telegram and passing them as base64-encoded multimodal attachments to the LLM (matching the OpenAI completions API spec).

**1. Model Configuration**:
To enable media forwarding for a specific model, define its capabilities in the `input` array inside your `config.json`:
```json
{
  "id": "gemini-flash",
  "model": "gemini-2.5-flash",
  "name": "Gemini 2.5 Flash",
  "input": ["text", "vision", "audio"]
}
```
- `"vision"` or `"image"` enables image inputs.
- `"audio"` or `"voice"` enables audio inputs.

**2. Telegram Group Mentions & Context**:
- **Private Chats**: Media and messages are always processed.
- **Group Chats**: Media and messages are ignored unless the bot is mentioned (e.g. `@BotUsername`) or a message is a reply to the bot's response.
- **Reply Context**: If you reply to another message, the replied-to message context is automatically captured and prepended to your prompt.

### Outbound Media Protocol (Sending)

To send media files, the Agent (or any Skill) should output a message starting with one of the following prefixes:

| Type | Prefix | Example |
| :--- | :--- | :--- |
| **Image** | `#IMAGE#:` | `#IMAGE#:https://example.com/image.png` or `#IMAGE#:./local_file.png` |
| **Audio** | `#AUDIO#:` | `#AUDIO#:https://example.com/audio.mp3` |
| **Video** | `#VIDEO#:` | `#VIDEO#:https://example.com/video.mp4` |
| **Document** | `#DOC#:` | `#DOC#:./report.pdf` |

**Base64 Support**:
If a tool outputs raw base64 data for an image, use `#BASE64_IMAGE#:<data>`. The Message Client will automatically convert this to a temporary file and send it as an image.

### Provider-Specific Formatting

Each messaging provider can inject specific formatting instructions into the Agent's system prompt via `SystemPromptInstruction()`.

**Telegram Formatting (HTML)**:
Current Telegram implementation uses `parse_mode: HTML`. To ensure rich formatting works correctly, the Agent is instructed to:
- Use standard tags like `<b>`, `<i>`, `<u>`, `<s>`, `<a>`.
- Use specialized tags like `<tg-spoiler>`, `<tg-emoji>`, and `<tg-time>`.
- Use `<b>` for headlines (Markdown `#` headers are ignored).
- Use `&lt;`, `&gt;`, and `&amp;` for escaping literal characters.
- Represent tables using ASCII formatting inside `<pre>` or `<blockquote>` blocks.
- Use horizontal separators (e.g., `———`) for visual spacing.

### Extensibility

To add new capabilities:

1.  **Define a Protocol**: Choose a unique prefix (e.g., `#LOCATION#:`).
2.  **Update Messaging Interface**: Add a method to `pkg/messaging/provider.go` (e.g., `SendLocation`).
3.  **Update Clients**: Implement the method in all providers (e.g., `pkg/messaging/telegram/client.go`).
4.  **Update System Prompt**: Add instructions to `pkg/agent/agent.go` so the LLM knows about the new capability.

## Web Search Providers

YAOCC supports multiple web search providers.

### Adding a New Web Search Provider

1.  **Create a new file**: Under `pkg/websearch/<provider>.go`.
2.  **Implement the Provider Interface**:
    Your struct must implement the `Provider` interface defined in `pkg/websearch/provider.go`.

    ```go
    type Provider interface {
        Search(query string) ([]SearchResult, error)
    }
    ```

3.  **Register the Provider**:
    In `pkg/websearch/provider.go`, add your provider to the `NewProvider` factory function.

    ```go
    func NewProvider(...) (Provider, error) {
        switch cfg.Type {
        case "myprovider":
            return NewMyProvider(cfg), nil
        // ...
        }
    }
    ```

5.  **Configuration**:
    Update `pkg/config/config.go` if your provider needs specific configuration fields.
    Standard fields like `maxResults` and `fallback` (generic provider fallback) are available in `SearchProvider`.

## Command Execution & Security

### Terminal
you are using windows 11 and windows powershell as terminal, any command you run you be compatible with windows powershell

### CGO and SQLite
YAOCC uses `modernc.org/sqlite` instead of `github.com/mattn/go-sqlite3`. This means the chat database runs on pure Go without requiring CGO or a C compiler, making it easily cross-compilable to ARM/Windows/Mac from a single environment. gated by the `pkg/exec` package.

### Security Architecture

- **Default Blacklist**: `pkg/exec/exec.go` contains a hardcoded list of dangerous patterns (`rm -rf`, `sudo`, etc.).
- **Configuration**: `pkg/config/config.go` defines `Cmds` structure.
    - `whitelist`: If present, ONLY matching commands are allowed.
    - `blacklist`: If regex matches, command is blocked.
- **Timeout**: All commands have a 30-second timeout.
- **Context**: Commands run in the `YAOCC_CONFIG_DIR`.
