# Skill: internal/vcs Provider Parity

Use this for GitLab/GitHub fetch/post behavior and MR/PR abstractions.

## Scope
- VCS provider interface
- GitLab/GitHub MR/PR fetch, diffs, discussions, inline posting
- provider auto-detection and registration

## Safe Workflow
1. Keep `internal/vcs/types.go` contracts stable.
2. Handle API pagination and empty responses defensively.
3. Preserve inline comment position compatibility per platform.
4. Ensure thread-reply behavior degrades gracefully.
5. Run `go test ./internal/vcs/...`.

## Hotspots
- diff refs consistency (`base/start/head`)
- position payloads for multi-line/hunk comments
- discussion/thread mapping between providers

## Avoid
- Assuming GitLab and GitHub payload fields are equivalent
- Treating missing optional fields as fatal when recoverable
