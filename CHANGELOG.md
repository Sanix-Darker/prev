# Changelog

## v0.0.22 - 2026-03-27

feat: improve review intelligence and GitLab command automation

- add thread-level `prev ignore` support with persisted ignored-state tracking, immediate `prev review` unignore handling, and stronger tests around thread command state
- add semantic review-memory behavior fingerprints and primary-symbol hints so paraphrased findings can be revalidated and deduplicated more consistently across reruns
- revalidate historical open and ignored findings against current changed files, changed symbols, and changed hunk keywords before injecting them back into review prompts
- upgrade the deterministic native impact precheck for Go repositories with AST-derived caller/callee graphs while keeping text-scan fallback for other languages
- add a tested GitLab note-hook parser plus a runnable webhook receiver example for immediate `prev ...` merge-request note handling
- refresh integration docs, examples, and the review-intelligence roadmap to document the new command flows and current implementation baselines

Full Changelog: https://github.com/sanix-darker/prev/compare/v0.0.21...v0.0.22

## v0.0.21 - 2026-03-27

improve review command handling and comment-driven automation

- switch MR discussion commands from host-style mentions to plain `prev` keyword detection to avoid username conflicts across providers
- trigger the GitHub review workflow on pull request updates, top-level PR comments, and inline review-comment replies so `prev reply` and `prev summary` can rerun review automatically
- render AI agent fix prompts in inline review comments as collapsible sections and widen the bundled GitHub review config so automated runs surface higher-signal findings
- document cross-platform command behavior for GitHub and GitLab, including GitLab rerun requirements for discussion-command processing
- add a README table for all AI providers supported by `prev`
- update dependencies: `golang.org/x/term` `0.40.0` -> `0.41.0`

Full Changelog: https://github.com/sanix-darker/prev/compare/v0.0.20...v0.0.21

## v0.0.20 - 2026-03-27

feat: preserve review context across AI calls and add configurable reviewer controls

- preserve logical conversation history across related AI review calls so walkthroughs, detailed reviews, MR re-reviews, inline finding recovery, and thread replies keep prior context
- propagate command request context through provider and VCS flows so cancellation and timeout boundaries reach downstream HTTP calls
- add configurable reviewer controls and config inspection helpers, including `prev config effective`, `prev config validate`, review policy defaults, inline filtering, incremental review, review memory, native impact hints, fix prompts, and Serena/context defaults
- extend test coverage for conversation continuity, config validation, review pipeline behavior, VCS context handling, and version/e2e command paths
- refresh reviewer documentation, contributor docs, binary size guidance, and feature documentation to match the current review pipeline
- update dependencies: `github.com/spf13/pflag` `1.0.9` -> `1.0.10`, `actions/checkout` `v4` -> `v6`, `actions/setup-go` `v5` -> `v6`

Full Changelog: https://github.com/sanix-darker/prev/compare/v0.0.19...v0.0.20
