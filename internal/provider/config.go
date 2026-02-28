package provider

import (
	"fmt"
	"os"
	"strings"

	"github.com/sanix-darker/prev/internal/config"
)

// ---------------------------------------------------------------------------
// Configuration helpers
// ---------------------------------------------------------------------------

// ProviderConfig holds the resolved configuration for instantiating a
// provider. It is used by the Resolve* functions below so that the CLI
// layer does not need to know about config paths.
type ProviderConfig struct {
	// Name is the provider name as it appears in the registry (e.g. "openai").
	Name string

	// Viper is a sub-tree scoped to the provider's config block.
	Viper *config.Store
}

// ConfigKeyProvider is the config key that holds the active provider name.
const ConfigKeyProvider = "provider"

// ResolveProvider reads the active provider name and its config block from
// the config store. The lookup order is:
//
//  1. --provider CLI flag (already set on the store)
//  2. PREV_PROVIDER environment variable
//  3. "provider" key in the config file (~/.config/prev/config.yml)
//  4. Fallback to "openai"
//
// The returned ProviderConfig.Viper is scoped to the provider's subtree:
//
//	providers:
//	  openai:
//	    api_key: ...
//	    model: gpt-4o
func ResolveProvider(v *config.Store) ProviderConfig {
	// Determine the active provider name.
	name := v.GetString(ConfigKeyProvider)
	if name == "" {
		name = os.Getenv("PREV_PROVIDER")
	}
	if name == "" {
		name = "openai"
	}
	name = strings.ToLower(strings.TrimSpace(name))

	// Build a sub-store for the provider's config block.
	sub := v.Sub(fmt.Sprintf("providers.%s", name))
	if sub == nil {
		// No config file entry; create an empty store so that env-var
		// and flag bindings still work.
		sub = config.NewStore()
	}

	// Bind common env vars so they override file-based config. Providers
	// that need additional bindings do so in their factory function.
	bindProviderEnvVars(name, sub)

	return ProviderConfig{Name: name, Viper: sub}
}

// bindProviderEnvVars sets up well-known environment variables for each
// provider so that users can configure prev entirely through the shell.
func bindProviderEnvVars(name string, v *config.Store) {
	switch name {
	case "openai":
		v.SetDefault("model", "gpt-4o")
		v.SetDefault("base_url", "https://api.openai.com/v1")
		overrideFromEnv(v, "api_key", "OPENAI_API_KEY")
		overrideFromEnv(v, "model", "OPENAI_API_MODEL")
		overrideFromEnv(v, "base_url", "OPENAI_API_BASE")
	case "anthropic", "claude":
		v.SetDefault("model", "claude-sonnet-4-20250514")
		v.SetDefault("base_url", "https://api.anthropic.com")
		overrideFromEnv(v, "api_key", "ANTHROPIC_API_KEY")
		overrideFromEnv(v, "model", "ANTHROPIC_MODEL")
		overrideFromEnv(v, "base_url", "ANTHROPIC_API_BASE")
		// Backward-compatible alias used in some docs/examples.
		overrideFromEnv(v, "base_url", "ANTHROPIC_BASE_URL")
	case "azure":
		v.SetDefault("api_version", "2024-02-01")
		overrideFromEnv(v, "api_key", "AZURE_OPENAI_API_KEY")
		overrideFromEnv(v, "model", "AZURE_OPENAI_MODEL")
		// Backward-compatible alias used in some docs/examples.
		overrideFromEnv(v, "model", "AZURE_OPENAI_DEPLOYMENT")
		overrideFromEnv(v, "base_url", "AZURE_OPENAI_ENDPOINT")
		overrideFromEnv(v, "api_version", "AZURE_OPENAI_API_VERSION")
	case "gemini":
		// Gemini via Google's OpenAI-compatible endpoint.
		v.SetDefault("model", "gemini-2.0-flash")
		v.SetDefault("base_url", "https://generativelanguage.googleapis.com/v1beta/openai")
		overrideFromEnv(v, "api_key", "GEMINI_API_KEY")
		overrideFromEnv(v, "model", "GEMINI_MODEL")
		overrideFromEnv(v, "base_url", "GEMINI_BASE_URL")
	default:
		// Generic / OpenAI-compatible: try PREV_<PROVIDER>_* env vars.
		prefix := strings.ToUpper(name)
		overrideFromEnv(v, "api_key", fmt.Sprintf("PREV_%s_API_KEY", prefix))
		overrideFromEnv(v, "model", fmt.Sprintf("PREV_%s_MODEL", prefix))
		overrideFromEnv(v, "base_url", fmt.Sprintf("PREV_%s_BASE_URL", prefix))
	}
}

func overrideFromEnv(v *config.Store, key, envName string) {
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		v.Set(key, value)
	}
}

// SampleConfigYAML returns an example config.yml snippet that documents all
// provider settings. It is used by the "prev init" or "prev config" command.
func SampleConfigYAML() string {
	return `# prev configuration
# Active provider (openai | anthropic | azure | gemini | ollama | custom).
provider: openai

# Provider-specific settings. Each block corresponds to a registered provider.
providers:
  openai:
    # api_key can also be set via OPENAI_API_KEY env var.
    api_key: ""
    model: "gpt-4o"
    # base_url: "https://api.openai.com/v1"  # override for proxies
    max_tokens: 1024
    timeout: 30s

  anthropic:
    # api_key can also be set via ANTHROPIC_API_KEY env var.
    api_key: ""
    model: "claude-sonnet-4-20250514"
    max_tokens: 1024
    timeout: 30s

  azure:
    # api_key can also be set via AZURE_OPENAI_API_KEY env var.
    api_key: ""
    base_url: ""  # e.g. https://<resource>.openai.azure.com
    model: ""     # deployment name
    api_version: "2024-02-01"
    max_tokens: 1024
    timeout: 30s

  gemini:
    # api_key can also be set via GEMINI_API_KEY env var.
    api_key: ""
    base_url: "https://generativelanguage.googleapis.com/v1beta/openai"
    model: "gemini-2.0-flash"
    max_tokens: 1024
    timeout: 30s

  # Example: self-hosted Ollama or any OpenAI-compatible endpoint.
  ollama:
    base_url: "http://localhost:11434/v1"
    model: "llama3"
    max_tokens: 1024
    timeout: 60s

# Retry configuration (applies to all providers).
retry:
  max_retries: 3
  initial_interval: 1s
  max_interval: 30s
  multiplier: 2.0

# Review policy and conventions.
review:
  # 1: critical only, 10: include nits and minor suggestions.
  nitpick: 5
  # Optional strictness default for MR review when CLI flag is not provided.
  # Allowed: strict | normal | lenient
  # strictness: "normal"
  # Number of AI review passes (re-review loop) for MR review.
  passes: 1
  # Maximum inline comments for MR review (0 = unlimited).
  max_comments: 0
  # Inline filtering mode: added | diff_context | file | nofilter
  filter_mode: "diff_context"
  # MR diff source strategy: auto | git | raw | api
  mr_diff_source: "auto"
  # Enable structured JSON findings output parsing (with markdown fallback).
  structured_output: false
  # Enable incremental review scope using baseline markers.
  incremental: false
  # Post inline comments only (skip summary notes and thread replies).
  inline_only: false
  # Optional Serena/context defaults for MR review.
  # serena_mode: "auto"
  # context_lines: 10
  # max_tokens: 80000
  # Optional @mention handle used by MR thread commands, e.g. "@my-bot review".
  # When empty, mention-driven actions are disabled.
  mention_handle: ""
  conventions:
    labels: ["issue", "suggestion", "remark"]
  # Optional custom instructions injected into review prompts.
  guidelines: |
    Prioritize correctness, security, and maintainability.
    Keep findings concrete and actionable.

# Display options.
debug: false
max_key_points: 3
max_characters_per_key_point: 100
explain: false
`
}
