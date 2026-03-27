## Binary Size Audit

This document records the current dependency and binary-size audit for `prev`.

## Current Baseline

Stripped Linux amd64 build:

```bash
go build -trimpath -ldflags='-s -w' -o /tmp/prev-audit ./
ls -lh /tmp/prev-audit
```

Observed size during the latest audit: about `7.4M`.

## Production Dependency Set

Direct runtime-relevant dependencies are currently small:

- `github.com/spf13/cobra`
- `github.com/spf13/pflag`
- `github.com/atotto/clipboard`
- `golang.org/x/term`
- `gopkg.in/yaml.v3`

Test-only dependency:

- `github.com/stretchr/testify`

## Why Each Dependency Exists

- `cobra`: command tree, help generation, flag wiring.
- `pflag`: shared flag helpers in `internal/common/utils.go` use `*pflag.FlagSet` directly.
- `clipboard`: clipboard-backed input for `prev optim`.
- `x/term`: terminal-aware streaming/output rendering.
- `yaml.v3`: config parsing and config file emission.
- `testify`: tests only, no runtime binary impact.

## What Is Not an Easy Win

These do not offer meaningful size wins without product tradeoffs:

- Removing `testify`: helps module/download surface only, not the shipped binary.
- Removing direct `pflag` usage from `go.mod` without removing Cobra itself: no real binary reduction, because Cobra already depends on pflag.
- Minor `go.mod` cleanup alone: the dependency graph is already tight.

## Real Reduction Options

These are the realistic ways to cut binary size further:

1. Make clipboard support optional.
   Remove or build-tag `github.com/atotto/clipboard` and require stdin/file input for `prev optim` in minimal builds.

2. Replace Cobra/pflag with a smaller custom CLI layer.
   This is the largest likely dependency-level reduction, but it would increase maintenance cost and risk regressions in command UX, help output, and flag parsing.

3. Make YAML config optional in minimal builds.
   A reduced build could rely on env vars + flags only, dropping `yaml.v3` for a dedicated minimal profile.

4. Reduce terminal rendering features in minimal builds.
   If plain output is acceptable, `golang.org/x/term` usage can be minimized or build-tagged.

5. Offer build profiles.
   Keep the default full-featured binary, but add a minimal build target that disables clipboard, YAML config, and rich terminal behavior.

## Recommended Direction

The safest next step is not a blind dependency purge. It is to introduce an explicit minimal build profile, for example:

- default build: full CLI
- minimal build tag: no clipboard, simpler output, env-only config

That preserves current UX for normal users while allowing smaller release artifacts for constrained environments.

## Audit Notes

`go mod why` confirmed that every current production dependency is referenced by live code.
There is no obvious dead dependency to delete without changing behavior.
