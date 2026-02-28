## PREV

AI-powered code review CLI tool for diffs, commits, branches, and merge/pull requests.

[![ci](https://github.com/sanix-darker/prev/actions/workflows/ci.yml/badge.svg)](https://github.com/sanix-darker/prev/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/sanix-darker/prev)](https://goreportcard.com/report/github.com/sanix-darker/prev)

Supports multiple AI providers: **OpenAI**, **Anthropic (Claude)**, **Azure OpenAI**, **Gemini**, **Ollama**, **Groq**, **Together**, **LM Studio**, and any **OpenAI-compatible** API.

### Installation

```bash
# Homebrew
brew install sanix-darker/tap/prev

# From source
go install github.com/sanix-darker/prev@latest

# Or clone and build
git clone https://github.com/sanix-darker/prev.git
cd prev
go build -o prev .

# Docker
docker build -t prev .
docker run --rm -e OPENAI_API_KEY=sk-xxx prev version
```

### Quick Start

```bash
# 1. Set up your AI provider credentials
export OPENAI_API_KEY=sk-xxx          # For OpenAI (default)
# OR
export ANTHROPIC_API_KEY=sk-ant-xxx   # For Claude
# OR run locally with Ollama (no key needed)

# 2. Review a diff between two files
prev diff fixtures/test_diff1.py,fixtures/test_diff2.py

# 3. Review a git commit
prev commit abc123 --repo /path/to/repo

# 4. Review a branch (two-pass pipeline with walkthrough + detailed review)
prev branch feature-branch --repo /path/to/repo

# 5. Optimize a code file
prev optim myfile.py

# 6. Review a GitLab/GitHub merge/pull request
export GITLAB_TOKEN=glpat-xxxx
prev mr review my-group/my-project 42

# 7. Review with strict mode
prev branch feature-branch --strictness strict
```

### Commands

#### Review Commands

| Command | Description |
|---------|-------------|
| `prev diff <file1,file2>` | Review diff between two files |
| `prev commit <hash>` | Review a git commit |
| `prev branch <name>` | Review a branch diff (two-pass pipeline) |
| `prev optim <file\|clipboard>` | Optimize code |

#### Merge/Pull Request Commands

| Command | Description |
|---------|-------------|
| `prev mr review <project> <mr_id>` | Review a merge/pull request using AI |
| `prev mr diff <project> <mr_id>` | Show MR diff locally (no AI) |
| `prev mr list <project>` | List open merge requests |

#### Provider & Config Commands

| Command | Description |
|---------|-------------|
| `prev ai list` | List available AI providers |
| `prev ai show` | Show current provider and model |
| `prev config show` | Show current configuration |
| `prev config init` | Create default config file |
| `prev config effective` | Show merged effective configuration (with env/flags applied) |
| `prev config validate` | Validate configuration keys and provider requirements |
| `prev version` | Print version info |

### Branch Review Pipeline

The `prev branch` command uses a **two-pass review pipeline** inspired by CodeRabbit:

1. **Pass 1 -- Walkthrough**: The AI receives abbreviated diffs and diff stats for all changed files, then produces a high-level summary and a changes table.
2. **Pass 2 -- Detailed Review**: Files are batched by token budget and sent for detailed file-by-file review. The walkthrough summary is included as context so the AI understands the broader purpose of each change.

Between the two passes, files are enriched with surrounding code context (configurable via `--context`) so the AI can see the code around each hunk, not just the raw diff.

#### Branch Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--context` | `10` | Number of surrounding context lines for review |
| `--max-tokens` | `80000` | Maximum tokens per AI batch |
| `--per-commit` | `false` | Review each commit individually |
| `--legacy` | `false` | Use legacy single-prompt review mode |
| `--serena` | `auto` | Serena MCP mode: `auto`, `on`, `off` |

#### Example Output

```
# Branch Review: feature-branch -> main

## Walkthrough
This branch adds user authentication with JWT tokens...

## Changes
| File | Type | Summary |
|------|------|---------|
| auth.go | New | JWT token generation and validation |
| middleware.go | Modified | Added auth middleware |

## Detailed Review

### auth.go
**auth.go:42** [HIGH]: JWT secret should not be hardcoded...

### middleware.go
No significant issues found.

## Statistics
- Files reviewed: 2
- Issues: 1 (1 HIGH)
- Changes: +145/-3
```

### Strictness Levels

Control review thoroughness with `--strictness`:

| Level | Severity Filter | Description |
|-------|----------------|-------------|
| `strict` | All (CRITICAL, HIGH, MEDIUM, LOW) | Report all issues including style nits. Be thorough. |
| `normal` (default) | MEDIUM and above | Focus on bugs, security, and significant code quality issues. |
| `lenient` | HIGH and above | Only report CRITICAL and HIGH severity issues. |

```bash
# Strict: catch everything
prev branch feature --strictness strict

# Lenient: only critical issues
prev mr review my-project 42 --strictness lenient
```

### Serena Integration

[Serena](https://github.com/oraios/serena) is an MCP (Model Context Protocol) server that provides symbol-level code intelligence. When enabled, prev replaces raw context lines with enclosing function/class bodies, giving the AI better understanding of surrounding code.

#### Modes

| Mode | Behavior |
|------|----------|
| `auto` (default) | Use Serena if available, fall back to line-based context if not |
| `on` | Require Serena; fail with error if not installed |
| `off` | Disable Serena entirely |

#### Prerequisites

```bash
# Install uv (includes uvx)
python -m pip install --user uv
# or: curl -LsSf https://astral.sh/uv/install.sh | sh

# Serena is fetched automatically via uvx on first use
uvx --from git+https://github.com/oraios/serena serena --help
prev branch feature --serena=on
```

CI runners must install uv/uvx before running commands with `--serena=on`.

GitHub Actions example:

```yaml
- uses: astral-sh/setup-uv@v6
- run: uvx --version
```

### MR/PR Reviews

Review GitLab merge requests or GitHub pull requests directly from your terminal.

VCS provider is auto-detected: if `GITLAB_TOKEN` is set, GitLab is used; if `GITHUB_TOKEN` is set, GitHub is used. Override with `--vcs`.

```bash
# GitLab
export GITLAB_TOKEN=glpat-xxxx
export GITLAB_URL=https://gitlab.com   # optional, defaults to gitlab.com

# GitHub
export GITHUB_TOKEN=ghp_xxxx

# Review an MR (prints review to terminal)
prev mr review my-group/my-project 42 --dry-run

# Review and post comments to the MR/PR
prev mr review my-group/my-project 42

# Post only a summary comment (no inline comments)
prev mr review my-group/my-project 42 --summary-only

# Use a specific AI provider
prev mr review my-group/my-project 42 --provider anthropic

# Control inline comment filtering
prev mr review my-group/my-project 42 --strictness strict
```

#### MR Flags

| Flag | Description |
|------|-------------|
| `--dry-run` | Print review to terminal without posting to VCS |
| `--summary-only` | Post only a summary comment, no inline comments |
| `--vcs` | VCS provider: `gitlab`, `github` (auto-detected from env) |
| `--gitlab-token` | GitLab token (or use `GITLAB_TOKEN` env) |
| `--gitlab-url` | GitLab instance URL (or use `GITLAB_URL` env) |
| `--strictness` | Review strictness: `strict`, `normal`, `lenient` |
| `--max-comments` | Max inline comments to post (0 = unlimited; prioritizes highest severity) |
| `--review-passes` | Number of AI review passes (0 = config/default `1`) |
| `--serena` | Serena MCP mode for MR context: `auto`, `on`, `off` |
| `--context` | Number of surrounding context lines used for MR enrichment |
| `--max-tokens` | Max token budget used by MR context enrichment |

#### MR Thread Commands

When `review.mention_handle` (or `PREV_MENTION_HANDLE`) is set, MR comments can control bot behavior:

- `@<handle> pause`: pause reviews for the MR/thread
- `@<handle> resume`: resume paused MR/thread reviews
- `@<handle> review`: force review processing for that thread
- `@<handle> summary`: post one top-level summary note (idempotent)
- `@<handle> reply` or `@<handle> ... ?`: bot posts a thread reply

Inline continuity behavior:

- Finds multiple issues in one changed hunk and posts them as key points in a single inline comment.
- Reuses matching unresolved discussions on later pushes (reply in-thread) instead of opening duplicate new threads.

### Repository Guidelines Mapping

`prev` can automatically load repository review guidelines and inject them into review prompts so findings align with project conventions.

Auto-discovered files (when present):

- `AGENTS.md`
- `CLAUDE.md`
- `.claude/*.md` (including `.claude/CLAUDE.md`)
- `.github/copilot-instructions.md`
- `.github/instructions/*.md`
- `.copilot-instructions.md`

This applies to `diff`, `commit`, `branch`, and `mr review`.
For MR review, `CI_PROJECT_DIR` is used when available (for CI runners).

You can combine this with config-based guidelines (`review.guidelines` in `~/.config/prev/config.yml`).

### Text-Only Review Scope

`prev` reviews text/code changes only. Binary assets are skipped from AI review and inline findings, including formats such as:

- PDFs (`.pdf`)
- Images (`.png`, `.jpg`, `.gif`, ...)
- Archives/binaries (`.zip`, `.tar`, `.exe`, `.so`, ...)

### AI Providers

Use `--provider` and `--model` flags to override the default provider for any command.

Provider resolution order:
1. `--provider` CLI flag
2. `PREV_PROVIDER` environment variable
3. `provider` key in config file (`~/.config/prev/config.yml`)
4. Fallback to `openai`

#### OpenAI (default)

```bash
export OPENAI_API_KEY=sk-xxx
export OPENAI_API_MODEL=gpt-4o     # optional, defaults to gpt-4o

prev diff file1.py,file2.py
```

#### Anthropic (Claude)

```bash
export ANTHROPIC_API_KEY=sk-ant-xxx
export ANTHROPIC_MODEL=claude-sonnet-4-20250514  # optional

prev diff file1.py,file2.py --provider anthropic
```

#### Azure OpenAI

```bash
export AZURE_OPENAI_API_KEY=xxx
export AZURE_OPENAI_ENDPOINT=https://your-resource.openai.azure.com
export AZURE_OPENAI_DEPLOYMENT=gpt-4o

prev diff file1.py,file2.py --provider azure
```

#### Ollama (local, free)

```bash
# Start ollama
ollama serve &
ollama pull llama3

prev diff file1.py,file2.py --provider ollama --model llama3
```

#### Other OpenAI-compatible APIs

```bash
# Groq
export GROQ_API_KEY=gsk_xxx
prev diff file1.py,file2.py --provider groq --model llama-3.3-70b-versatile

# Together
export TOGETHER_API_KEY=xxx
prev diff file1.py,file2.py --provider together --model meta-llama/Llama-3-70b-chat-hf

# LM Studio
prev diff file1.py,file2.py --provider lmstudio --model local-model

# Gemini (OpenAI-compatible endpoint)
export GEMINI_API_KEY=xxx
prev diff file1.py,file2.py --provider gemini --model gemini-2.0-flash

# Any OpenAI-compatible endpoint
export OPENAI_COMPAT_API_KEY=xxx
export OPENAI_COMPAT_BASE_URL=https://your-api.example.com/v1
prev diff file1.py,file2.py --provider openai-compat --model your-model
```

### Configuration

Create a config file at `~/.config/prev/config.yml`:

```bash
prev config init
```

Full config example:

```yaml
# prev configuration
# Active provider (openai | anthropic | azure | gemini | ollama | custom).
provider: openai

# Provider-specific settings. Each block corresponds to a registered provider.
providers:
  openai:
    # api_key can also be set via OPENAI_API_KEY env var.
    api_key: ""
    model: "gpt-4o"
    # base_url: "https://api.openai.com/v1"  # override for proxies
    max_tokens: 1024
    timeout: 30s

  anthropic:
    # api_key can also be set via ANTHROPIC_API_KEY env var.
    api_key: ""
    model: "claude-sonnet-4-20250514"
    max_tokens: 1024
    timeout: 30s

  azure:
    # api_key can also be set via AZURE_OPENAI_API_KEY env var.
    api_key: ""
    base_url: ""  # e.g. https://<resource>.openai.azure.com
    model: ""     # deployment name
    api_version: "2024-02-01"
    max_tokens: 1024
    timeout: 30s

  gemini:
    # api_key can also be set via GEMINI_API_KEY env var.
    api_key: ""
    base_url: "https://generativelanguage.googleapis.com/v1beta/openai"
    model: "gemini-2.0-flash"
    max_tokens: 1024
    timeout: 30s

  # Example: self-hosted Ollama or any OpenAI-compatible endpoint.
  ollama:
    base_url: "http://localhost:11434/v1"
    model: "llama3"
    max_tokens: 1024
    timeout: 60s

# Retry configuration (applies to all providers).
retry:
  max_retries: 3
  initial_interval: 1s
  max_interval: 30s
  multiplier: 2.0

# Review policy and conventions.
review:
  # 1: critical only, 10: include nits and minor suggestions.
  nitpick: 5
  # Optional strictness default for MR review when CLI flag is not provided.
  # Allowed: strict | normal | lenient
  # strictness: "normal"
  # Number of AI review passes (re-review loop) for MR review.
  passes: 1
  # Maximum inline comments for MR review (0 = unlimited).
  max_comments: 0
  # Inline filtering mode: added | diff_context | file | nofilter
  filter_mode: "diff_context"
  # MR diff source strategy: auto | git | raw | api
  mr_diff_source: "auto"
  # Enable structured JSON findings output parsing (with markdown fallback).
  structured_output: false
  # Enable incremental review scope using baseline markers.
  incremental: false
  # Post inline comments only (skip summary notes and thread replies).
  inline_only: false
  # Optional Serena/context defaults for MR review.
  # serena_mode: "auto"
  # context_lines: 10
  # max_tokens: 80000
  # Optional @mention handle used by MR thread commands, e.g. "@my-bot review".
  # When empty, mention-driven actions are disabled.
  mention_handle: ""
  conventions:
    labels: ["issue", "suggestion", "remark"]
  # Optional custom instructions injected into review prompts.
  guidelines: |
    Prioritize correctness, security, and maintainability.
    Keep findings concrete and actionable.

# Display options.
debug: false
max_key_points: 3
max_characters_per_key_point: 100
explain: false
```

### Global Flags

| Flag | Description |
|------|-------------|
| `--provider, -P` | AI provider to use (openai, anthropic, azure, gemini, ollama, etc.) |
| `--model, -m` | Model to use for the AI provider |
| `--stream, -s` | Enable streaming output (default: true) |
| `--strictness` | Review strictness: `strict`, `normal`, `lenient` (default: normal) |
| `--debug` | Enable debug output |
| `--help, -h` | Help for any command |

Per-command flags:

| Flag | Commands | Description |
|------|----------|-------------|
| `--repo, -r` | commit, branch | Path to git repository |
| `--path, -p` | commit, branch | Filter diff to specific file paths |

### Development

```bash
# Run unit tests
make test-unit

# Run E2E tests
make test-e2e

# Build
go build -o prev .

# Docker build
docker build -t prev .
```

### License

See [LICENSE](LICENSE) file.
