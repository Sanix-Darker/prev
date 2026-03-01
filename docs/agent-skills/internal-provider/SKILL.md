# Skill: internal/provider Reliability

Use this for provider wiring, retries, streaming, and model compatibility.

## Scope
- provider registry/factory and config resolution
- OpenAI/Anthropic/Azure/compat providers
- streaming chunk delivery and retry behavior

## Safe Workflow
1. Keep provider interface contracts unchanged.
2. Reuse shared helpers in `internal/provider` where possible.
3. Ensure stream goroutines always close channels exactly once.
4. Prefer provider-specific `p.client` over global clients.
5. Run `go test ./internal/provider/...` and race tests.

## Hotspots
- HTTP status mapping to `ProviderError`
- stream scanner limits and malformed SSE chunks
- provider aliases (`gemini`, `ollama`, etc.)
- env var precedence and config defaults

## Avoid
- Silent fallback that hides provider misconfiguration
- Inconsistent headers/auth between sync and stream paths
