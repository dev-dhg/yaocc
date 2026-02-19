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
    - `messaging/`: Messaging provider implementations.
        - `telegram/`: Telegram bot client.
- `skills/`: Default skill definitions and implementations.

## Building

To build the project, run:

```bash
# Build CLI
go build -o yaocc.exe ./cmd/yaocc

# Build Server (if needed)
go build -o yaocc-server.exe ./cmd/yaocc-server
```

## Configuration

YAOCC uses a `config.json` file for configuration.
- Copy `config.json.example` to `config.json`.
- Environment variables can be used in the config file using `${VAR_NAME}` syntax.

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

The Agent can be extended to support rich media interactions. Currently, the Agent supports sending **Images**, **Audio**, **Video**, and **Documents** through specific text protocols that the messaging clients intercept.

### Media Protocol

To send media files, the Agent (or any Skill) should output a message starting with one of the following prefixes:

| Type | Prefix | Example |
| :--- | :--- | :--- |
| **Image** | `#IMAGE#:` | `#IMAGE#:https://example.com/image.png` or `#IMAGE#:./local_file.png` |
| **Audio** | `#AUDIO#:` | `#AUDIO#:https://example.com/audio.mp3` |
| **Video** | `#VIDEO#:` | `#VIDEO#:https://example.com/video.mp4` |
| **Document** | `#DOC#:` | `#DOC#:./report.pdf` |

**Base64 Support**:
If a tool outputs raw base64 data for an image, use `#BASE64_IMAGE#:<data>`. The Message Client will automatically convert this to a temporary file and send it as an image.

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

The `exec` command allows executing shell commands on the host. This is gated by the `pkg/exec` package.

### Security Architecture

- **Default Blacklist**: `pkg/exec/exec.go` contains a hardcoded list of dangerous patterns (`rm -rf`, `sudo`, etc.).
- **Configuration**: `pkg/config/config.go` defines `Cmds` structure.
    - `whitelist`: If present, ONLY matching commands are allowed.
    - `blacklist`: If regex matches, command is blocked.
- **Timeout**: All commands have a 30-second timeout.
- **Context**: Commands run in the `YAOCC_CONFIG_DIR`.
