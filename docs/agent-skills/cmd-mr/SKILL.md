# Skill: cmd/mr Orchestration

Use this when changing `cmd/mr.go` and adjacent MR CLI flow.

## Scope
- CLI flags for `prev mr review|diff|list`
- Prompt assembly and guideline injection
- Inline finding parsing/filtering/grouping and posting
- Review memory integration and fix-prompt behavior

## Safe Workflow
1. Keep flag normalization backward-compatible.
2. Preserve default behavior for existing CI pipelines.
3. Validate both dry-run and post-comment paths.
4. Confirm JSON/structured output mode still parses with fallbacks.
5. Run `go test ./cmd/...` and `go test ./...`.

## Hotspots
- `normalize*` helpers and defaults
- inline anchor selection and fallback logic
- memory loading/saving and relevance filtering
- provider/vcs auto-detection paths

## Avoid
- Expanding prompts with redundant context blocks
- Breaking `--strictness`, `--max-comments`, `--mr-diff-source`
- Posting comments when `--dry-run` is set
