# Paravizor

Paravizor is a terminal-first recon orchestration tool for authorized bug bounty and security research workflows. It manages recon projects, runs YAML-defined pipelines, stores normalized results in SQLite, and provides local AI-assisted analysis through Ollama.

## Quick Start

```bash
make build
./paravizor init example -i example.com,*.example.com
./paravizor run --dir ./example
```

Running `paravizor` without arguments opens the TUI.

## TUI Shortcuts

- `n`: create project
- `o`: browse/open projects
- `ctrl+r`: run/resume the active project pipeline
- `ctrl+a`: open the local AI analysis window for the active project
- `tab` / `shift+tab`: switch panels
- `enter`: open selected detail/logs
- `esc`: back/stop/close modal

## Local AI

The default assistant uses Ollama:

- Provider: `ollama`
- Model: `llama3.1:8b-instruct-q4_K_M`
- Base URL: `http://localhost:11434`

Start Ollama before pressing `ctrl+a`:

```bash
ollama serve
ollama pull llama3.1:8b-instruct-q4_K_M
```

The generated analysis is saved as `ai-analysis.md` in the project directory.

## CLI Commands

- `paravizor run --dir <project>`: run/resume with installed tools; missing tools are skipped
- `paravizor tools list`: list configured tools and availability
- `paravizor run --dir <project> --install`: optionally install missing supported tools before running
- `paravizor tools check --install`: check and install missing supported tools
- `paravizor query --dir <project> "SELECT ..."`: run read-only SQL queries
- `paravizor export artifacts --dir <project>`: export text artifacts
- `paravizor export report --dir <project>`: export a markdown report

## Configuration

Global configuration lives under `~/.config/paravizor`. Bootstrap creates default config, theme, pipeline, and tool YAML files on first run.
