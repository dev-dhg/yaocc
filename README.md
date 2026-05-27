# 🤖 Yet Another OpenClaw Clone (YAOCC)

**YAOCC** is a lightweight, Go-based agentic AI assistant inspired by OpenClaw.

It is designed to be sandboxed, Docker-friendly, and has a tiny footprint (4-5mb RAM, 10mb binary), running smoothly even on ARMv7 devices.

## Features

-   **LLM Integration**: Connects to Ollama, OpenRouter, or any OpenAI-compatible provider. Supports multimodal vision and audio inputs depending on configured model capabilities.
-   **Web Search**: Support for SearxNG, Brave, and Perplexity with fallback mechanisms.
-   **Skill System**: Dynamic CLI command execution based on user requests. It can even create its own skills!
-   **Local Chat History (SQLite & FTS5)**: Agents remember your conversations intelligently. Using a self-contained SQLite database with Full-Text Search enabled, YAOCC automatically slices conversation history and gives the LLM built-in abilities to securely query its own past messages without ballooning token contexts. Basic markdown history generation is also supported for highly constrained setups.
-   **Telegram Support**: Integrated bot with long-polling, now supporting full group chat mentions/replies, automatic context retrieval from replies, and receiving media (photos, voice, audio files).
-   **Swagger UI**: API documentation available at `/docs`.

> **Disclaimer:** 90% of this code was made by vibecoding using Google's Antigravity.

## Quick Start
### Prerequisites
-   Go 1.23 or higher (only for building the app)
-   Ollama (running locally) or valid API keys for other providers.

### Usage

1.  **Initialize Project**:
    ```bash
    ./yaocc init
    ```
    This generates necessary workspace files and a default `config.json`. make sure to set your config before using the app. the config file can use expanded variables coming from .env file.

2.  **Start Server**:
    ```bash
    ./yaocc-server
    ```

3.  **Chat**:
    ```bash
    ./yaocc chat "Hello, who are you?"
    ```

please refer to [DOCS.md](DOCS.md). to know more about how to enable Telegram

### Installation

Build files can be found in the [Releases](../../releases) tab.

To build manually:
```bash
# Build CLI
go build -o build/yaocc.exe ./cmd/yaocc

# Build Server
go build -o build/yaocc-server.exe ./cmd/yaocc-server

# Check version
./build/yaocc.exe version
```

> To embed version info into the binary, see the [Versioning section in DEV.md](DEV.md#versioning).

## Docker

### Building

You can build the image using Docker Compose:

```bash
docker compose build
```

Or manually using `docker build` with the default arguments:

```bash
docker build \
  --build-arg YAOCC_VERSION="v1.0.0" \
  --build-arg YAOCC_COMMIT="$(git rev-parse --short HEAD)" \
  --build-arg YAOCC_BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --build-arg YAOCC_BASE_IMAGE="node:24-alpine" \
  --build-arg YAOCC_DOCKER_APK_PACKAGES="git nano curl unzip python3" \
  --build-arg YAOCC_DOCKER_RUN_COMMANDS="" \
  --build-arg YAOCC_USER="node" \
  -t yaocc:v1.0.0 .
```

> The `YAOCC_VERSION`, `YAOCC_COMMIT`, and `YAOCC_BUILD_DATE` args are optional. If omitted, the binary reports `dev` as its version.

### Using Docker Compose (Recommended)

Simply run:
```bash
docker compose up -d
```

### Using Docker Directly

You can run the container directly, ensuring you mount your data directory and set the configuration path:

```bash
docker run -d \
  --name yaocc \
  -e YAOCC_CONFIG_DIR=/app/data \
  -v ./data:/app/data \
  -p 8080:8080 \
  yaocc:latest
```

## Documentation

For detailed configuration options, API usage, Docker deployment, and advanced features, please refer to [DOCS.md](DOCS.md).

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
