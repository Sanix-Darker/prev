# Changelog

## v0.0.20 - 2026-03-27

feat: preserve review context across AI calls and add configurable reviewer controls

- preserve logical conversation history across related AI review calls so walkthroughs, detailed reviews, MR re-reviews, inline finding recovery, and thread replies keep prior context
- propagate command request context through provider and VCS flows so cancellation and timeout boundaries reach downstream HTTP calls
- add configurable reviewer controls and config inspection helpers, including `prev config effective`, `prev config validate`, review policy defaults, inline filtering, incremental review, review memory, native impact hints, fix prompts, and Serena/context defaults
- extend test coverage for conversation continuity, config validation, review pipeline behavior, VCS context handling, and version/e2e command paths
- refresh reviewer documentation, contributor docs, binary size guidance, and feature documentation to match the current review pipeline
- update dependencies: `github.com/spf13/pflag` `1.0.9` -> `1.0.10`, `actions/checkout` `v4` -> `v6`, `actions/setup-go` `v5` -> `v6`

Full Changelog: https://github.com/sanix-darker/prev/compare/v0.0.19...v0.0.20
