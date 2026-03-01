## Features

### Core Review Commands

- **Diff review** (`prev diff`) -- Compare two files and get AI-powered review feedback
- **Commit review** (`prev commit`) -- Review any git commit by hash
- **Branch review** (`prev branch`) -- Two-pass pipeline review of branch changes against base
- **Code optimization** (`prev optim`) -- AI suggestions to improve code from a file or clipboard

### Branch Review Pipeline

- **Two-pass design**: Pass 1 generates a high-level walkthrough; Pass 2 does detailed file-by-file review using the walkthrough as context
- **File categorization**: Changes are grouped by type (tests, commands, core, docs, dependencies, ci/config)
- **Token-aware batching**: Files are batched to stay within AI provider token limits
- **Context enrichment**: Hunks are augmented with surrounding code lines from the target branch

### Merge/Pull Request Integration

- **GitLab and GitHub support**: Auto-detects VCS from environment tokens (`GITLAB_TOKEN` / `GITHUB_TOKEN`)
- **Inline comments**: Posts file-specific comments with severity levels directly to the MR/PR
- **Hunk-level consolidation**: Merges multiple findings in the same changed hunk into one inline thread with key points
- **Thread continuity**: Reuses matching unresolved discussions across pushes instead of opening duplicate threads
- **Summary notes**: Posts an overall review summary as a merge request note
- **Severity filtering**: Filters inline comments based on strictness level (strict/normal/lenient)
- **Dry-run mode**: Preview reviews locally before posting
- **Thread commands**: `@prev pause`, `@prev resume`, `@prev summary`, `@prev reply`, and `@prev review`

### AI Provider System

- **Pluggable architecture**: Registry/factory pattern for provider discovery
- **9 supported providers**: OpenAI, Anthropic (Claude), Azure OpenAI, Gemini, Ollama, Groq, Together, LM Studio, and any OpenAI-compatible API
- **Unified interface**: All providers implement the same `AIProvider` interface with blocking and streaming modes
- **Provider resolution**: CLI flag > `PREV_PROVIDER` env > config file > openai fallback

### Review Strictness Levels

- **strict**: Report all issues including style nits (all severities)
- **normal** (default): Focus on bugs, security, and code quality (MEDIUM and above)
- **lenient**: Only CRITICAL and HIGH severity issues

### Serena MCP Integration

- **Symbol-level context**: Replaces raw context lines with enclosing function/class bodies via [Serena](https://github.com/oraios/serena) MCP server
- **Three modes**: `auto` (use if available), `on` (required), `off` (disabled)
- **Graceful fallback**: In auto mode, falls back to line-based context if Serena is not installed
- **Runtime visibility**: MR review logs whether Serena is active/fallback and which model/provider are used

### Configuration System

- **Config file**: YAML configuration at `~/.config/prev/config.yml`
- **Per-provider settings**: API keys, models, base URLs, timeouts per provider block
- **Environment variables**: All settings can be overridden via environment variables
- **CLI flags**: Runtime overrides for provider, model, streaming, debug
- **Guideline mapping**: Auto-loads repository rules from `AGENTS.md`, `CLAUDE.md`, `.claude/*.md`, and Copilot instruction files to refine review prompts

### CI/CD and Tooling

- GitHub Actions workflows for CI (build + test) and releases (goreleaser)
- Optimized multi-stage Dockerfile
- Homebrew tap, deb/rpm/apk packages via goreleaser
- Shell completions (bash, zsh, fish) and man pages
