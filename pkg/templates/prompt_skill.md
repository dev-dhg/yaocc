---
name: prompt
description: Ask a quick question to the LLM
tags:
  - built-in
---

# Prompt Skill

The `prompt` command allows you to send a direct message to the configured LLM. This is useful for quick queries, summaries, translations, analyze text/code or any other task that requires a direct response from the LLM.

## Usage

```bash
yaocc prompt [flags] <your message>
```

## Flags

- `--model <model_id>`: (Optional) Specify which model to use. If not provided, it uses the default selected model from your configuration.

## Examples

### Basic Usage

```bash
yaocc prompt "Explain quantum computing in one sentence"
```

### Using a Specific Model

```bash
yaocc prompt --model ollama/gemma3:4b "Write a hello world in javascript"
```

if you don't know which model are available and which is best for you, use this command

```bash
yaocc model list
```