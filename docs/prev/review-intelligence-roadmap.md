# Review Intelligence Roadmap

## Current State (March 1, 2026)

`prev` review context currently includes:

- Raw diff hunks (Git/GitLab/GitHub sources).
- Line-based enrichment with configurable context windows.
- Optional Serena symbol-level enrichment (enclosing function/class context).
- Historical MR thread continuity and persistent reviewer memory.

`prev` now includes a deterministic native precheck block in MR reviews:

- changed-symbol impact map (reference counts + changed-file hits)
- concurrency/race-risk hunk signals (goroutine, lock symmetry, channel flow, map mutation hints)

`prev` still does **not** build a full language-level call graph/data-flow graph; transitive impacts remain approximated.

## Gap Analysis

Main gaps vs advanced code-review agents:

1. No native changed-symbol impact graph (caller/callee fan-out).
2. No first-party concurrency/race pre-check pass.
3. No semantic dedup key beyond file/line+message fingerprints.
4. No structured "fix-plan prompt" generation pipeline beyond inline per-finding blocks.

## Native Implementation Plan

### Phase 1: Symbol Impact Index (Implemented baseline)

- Parse changed hunks and extract changed symbols.
- Scan repository text files for symbol-reference counts.
- Inject compact impact map into review prompt.
- Next step: replace textual symbol scan with AST/reference engines per language.

### Phase 2: Concurrency/Race Risk Signals (Implemented heuristic baseline)

- Added deterministic hunk detectors for:
  - goroutine introduction
  - lock/unlock asymmetry
  - channel flow changes
  - potential shared map mutation
- Next step: upgrade to AST-backed concurrency checks and confidence scoring.

### Phase 3: Multi-Pass Prompt Orchestration

- Pass A: deterministic analyzers + impact graph extraction.
- Pass B: model review conditioned on structured outputs from Pass A.
- Pass C: optional fix-plan generation for unresolved HIGH/CRITICAL findings.

### Phase 4: Memory-Aware Validation Loop

- Revalidate memory findings against current impact graph:
  - re-open only if semantic evidence still present
  - avoid re-reporting fixed patterns without regression signal
- Persist semantic keys (`rule_id + symbol_id + behavior_fingerprint`).

## Performance & Stability Constraints

- Hard cap impact payload tokens per MR.
- Cache symbol graph by commit SHA.
- Fallback to current diff context path if analyzers fail.
- Keep deterministic analyzers cheap enough for CI use.

## Milestone Definition of Done

1. Improved true-positive rate on seeded regression MRs.
2. Lower false-positive repeat rate on fixed issues.
3. No >20% median runtime increase in CI review job.
4. Inline anchor precision remains stable.
