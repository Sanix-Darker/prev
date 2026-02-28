# `prev` Project Context (Agent Onboarding)

This file is intended to give any AI/code agent enough context to safely update `prev` without re-discovering core behavior every time.

## 1. Project Identity

- Name: `prev`
- Type: Go CLI
- Purpose: AI-powered code review for:
  - file diffs
  - commits
  - branches
  - merge/pull requests
  - code optimization suggestions
- Module: `github.com/sanix-darker/prev`
- Entry point: `main.go` -> `cmd.Execute()`

## 2. Current Repository State

- Main branch locally: `master`
- Local relation: `master...origin/master [ahead 1]`
- Untracked local directory: `.claude/` (do not assume it belongs to release artifacts)
- Build/test status: unit + e2e tests currently pass on this workspace.

## 3. Core Capabilities

### 3.1 Review Commands

- `prev diff <file1,file2>`
- `prev commit <hash>`
- `prev branch <name>`
- `prev mr review <project> <mr_iid>`
- `prev mr diff <project> <mr_iid>`
- `prev mr list <project>`
- `prev optim <file|clipboard>`

### 3.2 MR/PR Review Behavior (important)

- Supports GitLab and GitHub providers.
- Extracts MR changes from one of: local git refs, raw diff endpoint, API diff endpoint (`review.mr_diff_source` / `--mr-diff-source`).
- Supports:
  - inline comments
  - summary notes
  - inline hunk grouping
  - reuse of unresolved discussions
  - thread commands via `review.mention_handle`
  - optional structured-output parsing
  - optional incremental baseline flow
- Inline placement safety:
  - supports line/hunk fallback
  - includes meta-context filtering
  - includes low-signal filtering to reduce generic mis-anchored comments

### 3.3 Branch Review Pipeline

- Two-pass mode:
  1. walkthrough
  2. detailed per-file review with batching
- Optional Serena/context enrichment:
  - modes: `auto|on|off`
  - line context + symbol-level fallback logic

## 4. AI Provider System

Provider architecture is registry/factory-based under `internal/provider`.

### 4.1 First-class providers

- `openai`
- `anthropic`
- `azure`
- OpenAI-compatible adapter providers:
  - `gemini`
  - `ollama`
  - `groq`
  - `together`
  - `lmstudio`
  - `openai-compat`

### 4.2 Provider resolution

Precedence:
1. CLI flag `--provider`
2. `PREV_PROVIDER`
3. config file `provider`
4. fallback `openai`

### 4.3 Notable env vars

- OpenAI: `OPENAI_API_KEY`, `OPENAI_API_MODEL`, `OPENAI_API_BASE`
- Anthropic: `ANTHROPIC_API_KEY`, `ANTHROPIC_MODEL`, `ANTHROPIC_API_BASE` (+ alias `ANTHROPIC_BASE_URL`)
- Azure: `AZURE_OPENAI_API_KEY`, `AZURE_OPENAI_MODEL` (+ alias `AZURE_OPENAI_DEPLOYMENT`), `AZURE_OPENAI_ENDPOINT`, `AZURE_OPENAI_API_VERSION`
- Gemini: `GEMINI_API_KEY`, `GEMINI_MODEL`, `GEMINI_BASE_URL`
- Generic pattern: `PREV_<PROVIDER>_{API_KEY|MODEL|BASE_URL}`

## 5. Config System

- Default path: `~/.config/prev/config.yml`
- Commands:
  - `prev config init`
  - `prev config show`
  - `prev config effective` (merged effective runtime config, secrets redacted)
  - `prev config validate` (key/provider validation)

Detailed key-by-key reference is in [`WIKI.md`](./WIKI.md).

## 6. Directory Map (high-signal)

- `cmd/`: CLI commands and orchestration
- `internal/core/`: prompt builders, parsing helpers, git diff helpers
- `internal/diffparse/`: diff parsing + context enrichment
- `internal/handlers/`: command-level extraction handlers
- `internal/review/`: branch review pipeline
- `internal/provider/`: AI provider registry + implementations
- `internal/vcs/`: GitLab/GitHub abstractions and providers
- `tests/`: e2e tests (tagged `e2e`)
- `docs/prev/`: user docs (features + integration)
- `WIKI.md`: configuration source-of-truth

## 7. Safety/Quality Rules for Future Changes

1. Preserve MR inline anchoring correctness.
2. Avoid re-introducing docs/runtime drift for env var names and defaults.
3. Keep provider additions reflected in:
   - `internal/provider/config.go`
   - adapter registration (if OpenAI-compatible)
   - tests
   - README + docs + `WIKI.md`
4. Validate with:
   - `go test ./...`
   - `go test -tags=e2e ./tests/...`
5. For CI reviewer behavior, prefer:
   - `review.mr_diff_source: "auto"`
   - `review.filter_mode: "added"`

## 8. Known Constraints / Partial Support

- GitHub discussion reply model differs from GitLab; deep thread-parity is limited.
- Model output quality still depends on prompt compliance; structured parsing is best-effort.
- Serena requires local tooling availability (`uv`/`uvx`) when enforced.

## 9. Typical Agent Workflow (recommended)

1. Read `README.md`, `WIKI.md`, and this file.
2. Check `git status --short --branch`.
3. Implement smallest viable change.
4. Add/update tests.
5. Run full tests.
6. Update docs when behavior changes.

## 10. Fast Verification Commands

```bash
go test ./...
go test -tags=e2e ./tests/...
go run . ai list
go run . config validate
go run . config effective
```

