# Skill: internal/review Two-Pass Pipeline

Use this when editing walkthrough/review pipeline behavior.

## Scope
- pass-1 walkthrough prompt/result parsing
- pass-2 file-batch review execution
- strictness filtering and output shaping

## Safe Workflow
1. Keep pass order and data handoff deterministic.
2. Maintain token-budget batching invariants.
3. Validate strictness filters (`strict|normal|lenient`).
4. Confirm no duplicate/empty file reviews leak into final output.
5. Run `go test ./internal/review/...`.

## Hotspots
- walkthrough parsing fallback logic
- category/group assignment
- severity normalization and sorting

## Avoid
- Coupling parsing to one exact model output format
- Inflating prompt payload with duplicate file context
