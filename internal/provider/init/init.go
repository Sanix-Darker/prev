// Package init exists solely to trigger provider registration via import
// side-effects. Import this package once in your main or cmd layer:
//
//	import _ "github.com/sanix-darker/prev/internal/provider/init"
//
// This registers all built-in providers (openai, anthropic, azure, and the
// OpenAI-compatible adapters) with the global provider.Registry.
package init

import (
	_ "github.com/sanix-darker/prev/internal/provider/anthropic"
	_ "github.com/sanix-darker/prev/internal/provider/azure"
	_ "github.com/sanix-darker/prev/internal/provider/compat"
	_ "github.com/sanix-darker/prev/internal/provider/openai"
)
