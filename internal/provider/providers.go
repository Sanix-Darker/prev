// Package provider/providers.go is an import-side-effect file that ensures
// all built-in provider packages are initialized (their init() functions
// register factories with the global Registry).
//
// Any Go file that needs provider resolution should import this package:
//
//	import _ "github.com/sanix-darker/prev/internal/provider/providers"
//
// or more commonly, the cmd layer imports it once at startup.
package provider

// Import all built-in provider packages for their init() side-effects.
// This file exists so that callers only need a single import to get every
// provider registered.
//
// NOTE: Because Go disallows circular imports and this file lives in the
// provider package itself, the actual blank imports must happen in the
// consumer (e.g. cmd/review.go or main.go). This file documents the
// pattern. See internal/provider/init.go for the concrete imports.
//
// The providers that will be registered:
//
//   - "openai"        -> internal/provider/openai
//   - "anthropic"     -> internal/provider/anthropic
//   - "azure"         -> internal/provider/azure
//   - "ollama"        -> internal/provider/compat (OpenAI-compatible)
//   - "groq"          -> internal/provider/compat (OpenAI-compatible)
//   - "together"      -> internal/provider/compat (OpenAI-compatible)
//   - "lmstudio"      -> internal/provider/compat (OpenAI-compatible)
//   - "openai-compat" -> internal/provider/compat (OpenAI-compatible)
