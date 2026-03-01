# Skill: internal/diffparse Accuracy

Use this when changing diff parsing, hunk context, or enrichment.

## Scope
- unified/gitlab diff parsers
- text/binary filtering
- hunk/line metadata extraction
- context enrichment around changed lines

## Safe Workflow
1. Preserve old/new line numbering semantics.
2. Keep binary/new/deleted/renamed flags accurate.
3. Validate parser edge cases with focused fixtures.
4. Re-run downstream tests in `cmd` and `internal/review`.

## Hotspots
- hunk header parsing
- path normalization and rename handling
- context window clipping near file boundaries

## Avoid
- Reordering lines in a way that breaks inline anchors
- Dropping hunks silently on partial parse failures
