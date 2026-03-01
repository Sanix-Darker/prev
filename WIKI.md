# prev Configuration Wiki

This document is the source-of-truth for `prev` runtime configuration as implemented in code.

## Resolution Order

### Provider selection

`provider` resolves in this order:

1. `--provider` CLI flag
2. `PREV_PROVIDER` env var
3. `provider` in `~/.config/prev/config.yml`
4. fallback: `openai`

### Other settings

- For most settings, command-line flags override config file values.
- For provider internals, env vars override provider config block values.
- If a setting is absent, built-in defaults are used.

## Config File Path

- Linux/macOS: `~/.config/prev/config.yml`
- Generate: `prev config init`
- Inspect merged values: `prev config effective`
- Validate values: `prev config validate`

## Full Config Reference

| Key | Type | Default | Env Override | CLI Override | Used By |
|---|---|---|---|---|---|
| `provider` | string | `openai` | `PREV_PROVIDER` | `--provider` | all commands using AI |
| `providers.<name>.api_key` | string | empty | provider-specific (see below) | none | provider auth |
| `providers.<name>.model` | string | provider default | provider-specific (see below) | `--model` (request-time model) | provider request model |
| `providers.<name>.base_url` | string | provider default/required | provider-specific (see below) | none | provider endpoint |
| `providers.<name>.max_tokens` | int | `1024` | none | none | provider request budget |
| `providers.<name>.timeout` | duration string | `30s` (`60s` in sample for ollama) | none | none | provider HTTP timeout |
| `providers.azure.api_version` | string | `2024-02-01` | `AZURE_OPENAI_API_VERSION` | none | Azure API version |
| `retry.max_retries` | int | `3` | none | none | provider retry wrapper |
| `retry.initial_interval` | duration string | `1s` | none | none | provider retry wrapper |
| `retry.max_interval` | duration string | `30s` | none | none | provider retry wrapper |
| `retry.multiplier` | float | `2.0` | none | none | provider retry wrapper |
| `review.strictness` | string | `normal` | none | `--strictness` | MR filtering and prompt style |
| `review.nitpick` | int | `5` (effective default) | none | `--nitpick` | severity/kind filtering |
| `review.passes` | int | `1` | none | `--review-passes` | MR re-review loop |
| `review.max_comments` | int | `0` | none | `--max-comments` | inline comment cap |
| `review.filter_mode` | string | `diff_context` | none | `--filter-mode` | inline placement filtering |
| `review.mr_diff_source` | string | `auto` | none | `--mr-diff-source` | MR diff acquisition strategy |
| `review.structured_output` | bool | `false` | none | `--structured-output` | JSON finding parser mode |
| `review.incremental` | bool | `false` | none | `--incremental` | baseline-scoped MR reviews |
| `review.inline_only` | bool | `false` | none | `--inline-only` | suppress summary/replies |
| `review.serena_mode` | string | `auto` | none | `--serena` | symbol-level context enrichment |
| `review.context_lines` | int | `10` | none | `--context` | context enrichment |
| `review.max_tokens` | int | `80000` | none | `--max-tokens` | enrichment token budget |
| `review.conventions.labels` | list[string] | `["issue","suggestion","remark"]` | none | none | finding kind filter |
| `review.guidelines` | multiline string | empty | none | none | prompt injection |
| `debug` | bool | `false` | none | `--debug` | runtime logging behavior |
| `stream` | bool | `true` | none | `--stream` | streaming output mode |
| `max_key_points` | int | `3` | none | none | output shaping |
| `max_characters_per_key_point` | int | `100` | none | none | output shaping |
| `explain` | bool | `false` | none | none | output shaping |

Thread command handle is fixed to `@prev` (for example `@prev reply`, `@prev pause`).

### MR Review Memory (CLI Parameters)

These are CLI-only parameters for `prev mr review` (not persisted in config keys yet):

| Parameter | Type | Default | Purpose |
|---|---|---|---|
| `--memory` | bool | `true` | Enable persistent cross-MR memory |
| `--memory-file` | string | `.prev/review-memory.md` | Markdown memory file location |
| `--memory-max` | int | `12` | Max historical items injected into prompt |
| `--native-impact` | bool | `true` | Enable deterministic native impact/risk precheck |
| `--native-impact-max-symbols` | int | `12` | Max changed symbols included in impact map |
| `--fix-prompt` | string | `off` | Inline AI fix prompt mode: `off`, `auto`, `always` |

Memory file format is markdown with a fenced machine block:

- Human-readable sections (snapshot, open/fixed tables)
- ` ```prev-memory-json ` fenced JSON payload used by the CLI

Memory management commands:

- `prev memory show [--json]`
- `prev memory prune [--max-entries N] [--fixed-older-than-days N] [--dry-run]`
- `prev memory export <path> [--format markdown|json]`
- `prev memory reset --yes`

## Provider Env Vars

### OpenAI

- `OPENAI_API_KEY`
- `OPENAI_API_MODEL`
- `OPENAI_API_BASE`

### Anthropic

- `ANTHROPIC_API_KEY`
- `ANTHROPIC_MODEL`
- `ANTHROPIC_API_BASE`
- Alias supported: `ANTHROPIC_BASE_URL`

### Azure OpenAI

- `AZURE_OPENAI_API_KEY`
- `AZURE_OPENAI_MODEL`
- Alias supported: `AZURE_OPENAI_DEPLOYMENT`
- `AZURE_OPENAI_ENDPOINT`
- `AZURE_OPENAI_API_VERSION`

### Gemini

- `GEMINI_API_KEY`
- `GEMINI_MODEL`
- `GEMINI_BASE_URL` (default: `https://generativelanguage.googleapis.com/v1beta/openai`)

### OpenAI-compatible providers (`ollama`, `groq`, `together`, `lmstudio`, `openai-compat`, etc.)

Pattern (provider name uppercased):

- `PREV_<PROVIDER>_API_KEY`
- `PREV_<PROVIDER>_MODEL`
- `PREV_<PROVIDER>_BASE_URL`

Examples:

- `PREV_OLLAMA_BASE_URL=http://localhost:11434/v1`
- `PREV_GROQ_API_KEY=...`

## Valid Values

### `review.strictness`

- `strict`
- `normal`
- `lenient`

### `review.filter_mode`

- `added`
- `diff_context`
- `file`
- `nofilter`

### `review.mr_diff_source`

- `auto`
- `git`
- `raw`
- `api`

### `review.serena_mode`

- `auto`
- `on`
- `off`

## Validation Rules (`prev config validate`)

- `review.nitpick` must be `0..10`
- `review.passes` must be `0..6`
- `review.max_comments` must be `>= 0`
- `review.context_lines` must be `>= 0`
- `review.max_tokens` must be `>= 0`
- Provider required fields:
  - `openai`: `api_key`
  - `anthropic`: `api_key`
  - `azure`: `api_key`, `base_url`, `model`
  - other providers: `base_url`

## Common CI Recommendation

For MR review in CI, prefer:

```yaml
review:
  mr_diff_source: "auto"
  filter_mode: "added"
```

Reason:

- `auto` falls back safely when local refs are incomplete in CI checkouts.
- `added` keeps inline comments pinned to changed lines only.
