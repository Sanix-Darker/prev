# prev - High Context for AI Agents

This file is the primary onboarding context for any AI agent updating `prev`.
It focuses on architecture, behavior guarantees, quality constraints, and safe change workflows.

## 1) Project Snapshot

- Name: `prev`
- Language: Go
- Module: `github.com/sanix-darker/prev`
- Type: CLI for AI-powered code review and code optimization
- Entry point: `main.go` -> `cmd.Execute()`

Primary commands:
- `prev diff`
- `prev commit`
- `prev branch`
- `prev mr review|diff|list`
- `prev memory show|export|prune|reset`
- `prev config init|show|effective|validate`
- `prev ai list|show`

## 2) Capabilities (Current)

### Review Surfaces
- File diff review
- Commit review
- Branch review (two-pass walkthrough + detailed review)
- GitLab/GitHub MR/PR review with inline comments + summary note

### Review Intelligence
- Strictness levels: `strict|normal|lenient`
- Native impact report (changed symbol fan-out and concurrency-risk hints)
- Optional AI fix prompt block in inline comments
- Persistent review memory in markdown (`.prev/review-memory.md`)

### Providers
- First-class: OpenAI, Anthropic, Azure OpenAI
- OpenAI-compatible: Gemini, Ollama, Groq, Together, LM Studio, openai-compat

### Context Enrichment
- Local line-based context enrichment
- Serena MCP integration (`auto|on|off`) with graceful fallback

## 3) Core Architecture

- `cmd/`:
  - CLI orchestration, flag parsing, MR posting logic, memory integration
- `internal/review/`:
  - Two-pass branch pipeline, batching, parsing, severity filtering
- `internal/diffparse/`:
  - Unified diff parsing, GitLab diff conversion, context enrichment
- `internal/provider/`:
  - AI provider interface, registry, provider implementations
- `internal/vcs/`:
  - GitLab/GitHub provider abstraction and posting/fetching APIs
- `internal/handlers/`:
  - Diff extraction per command and MR extraction policy

## 4) Recent Hardening and Dedup Work

### Duplicate Logic Removed
- Unified diff path filtering shared in `internal/handlers/diff_filter.go`
  - consumed by both branch and commit handlers

### Streaming Stability Improvements
- Providers now consistently use their configured HTTP client (`p.client`) for stream paths.
- Stream request marshal errors are handled explicitly.
- SSE scanners use shared limits via `internal/provider/stream_helpers.go`.
- Stream chunk delivery uses shared context-aware helper to avoid blocked sends after cancellation.
- Azure stream path now reports scanner read errors consistently.

### Regression Tests Added
- `internal/handlers/commit_handler_test.go`
- `internal/provider/openai/openai_test.go` (stream path verifies provider client usage)

## 5) Invariants to Preserve

1. Do not break inline anchor correctness for MR comments.
2. Keep backward-compatible defaults for existing CI users.
3. Preserve provider/env/config precedence:
   - CLI flag > env var > config file > default
4. Keep command output deterministic for machine parsing where applicable.
5. Avoid adding heavy dependencies (binary size focus).

## 6) Known Operational Pitfalls

- `go install github.com/sanix-darker/prev@latest` failures were previously linked to version embed assets in tagged versions; verify `internal/cmd/version/*.txt` packaging in release tags.
- GoReleaser Homebrew publication requires valid tap token/permissions; use `.goreleaser.nohomebrew.yaml` fallback when tap credentials are unavailable.
- MR review quality depends on getting real MR hunks. Prefer `review.mr_diff_source: auto` (with repo available in CI when possible).

## 7) Test/Validation Matrix (Minimum)

Run before merging:

```bash
go test ./...
go test -race ./...
go vet ./...
```

Optional/full:

```bash
go test -tags=e2e ./tests/...
```

For MR review logic changes, also validate with a real MR in dry-run and posting modes.

## 8) Performance and Binary Size Guidance

- Prefer stdlib over new dependencies when practical.
- Reuse shared helpers in existing modules instead of introducing new packages.
- Keep docs/examples/skills under docs/examples paths only (not embedded into binary).
- Avoid reflection-heavy or generic-heavy abstractions unless proven necessary.

## 9) Agent Skills for Faster Module Work

Module skill playbooks live under:
- `docs/agent-skills/cmd-mr/SKILL.md`
- `docs/agent-skills/internal-provider/SKILL.md`
- `docs/agent-skills/internal-diffparse/SKILL.md`
- `docs/agent-skills/internal-vcs/SKILL.md`
- `docs/agent-skills/internal-review/SKILL.md`

Use these before editing the corresponding module.

## 10) Practical Safe Workflow for Future Agents

1. Read `README.md`, `WIKI.md`, this file, and relevant module `SKILL.md`.
2. Inspect current branch and local uncommitted state.
3. Make smallest safe refactor first, then behavior change.
4. Add/adjust tests for each non-trivial change.
5. Run full tests + race.
6. Update docs/context in the same PR.

