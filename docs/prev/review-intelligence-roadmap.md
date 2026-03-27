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

1. Native changed-symbol impact graph is now implemented for Go via AST caller/callee extraction, but non-Go languages still fall back to text heuristics.
2. Concurrency/race pre-check is implemented as a heuristic baseline; it still needs AST-backed confidence scoring.
3. Semantic dedup and memory revalidation are implemented as keyword/symbol baselines; they are not yet language-semantic.
4. Structured fix-plan generation is still limited to inline per-finding blocks.

## Current Feature Plans

### Semantic Dedup And Memory Revalidation

Status: implemented baseline on March 27, 2026.

- Added behavior fingerprints and primary-symbol hints to review memory entries.
- Reuse prior memory entries when findings are paraphrased but semantically close in the same file.
- Revalidate historical memory against changed files, changed symbols, and changed hunk keywords before injecting it back into prompts.
- Next pass: promote from keyword/symbol heuristics to AST-backed rule anchors where available.

### Go Native Impact Graph

Status: implemented baseline on March 27, 2026.

- Go repositories now derive caller/callee relationships from AST parsing during the deterministic impact precheck.
- Native impact reports expose inbound callers, outbound callees, and the source used to derive the impact (`go-ast` vs fallback scan).
- Next pass: add package-qualified symbol identity and method-receiver disambiguation.

### Memory-Aware Validation Loop

Status: implemented baseline on March 27, 2026.

- Historical open and ignored findings are re-scored against the current diff before being injected into prompts.
- `prev review` already clears thread-level ignore state immediately, which now feeds the same revalidation path.
- Next pass: teach the loop to downrank or expire stale open findings with repeated no-evidence runs.

### GitLab Note-Triggered Reaction

Status: implemented as a tested example on March 27, 2026.

- Added a reusable GitLab note-hook parser and example webhook receiver that runs `prev mr review` on merge-request note commands.
- Next pass: decide whether to productize this as a first-party `prev hook` command or keep it as an example deployment pattern.

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
