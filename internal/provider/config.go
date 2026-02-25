package provider

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// ---------------------------------------------------------------------------
// Configuration helpers
// ---------------------------------------------------------------------------

// ProviderConfig holds the resolved configuration for instantiating a
// provider. It is used by the Resolve* functions below so that the CLI
// layer does not need to know about viper paths.
type ProviderConfig struct {
	// Name is the provider name as it appears in the registry (e.g. "openai").
	Name string

	// Viper is a sub-tree scoped to the provider's config block.
	Viper *viper.Viper
}

// ConfigKeyProvider is the viper key that holds the active provider name.
const ConfigKeyProvider = "provider"

// ResolveProvider reads the active provider name and its config block from
// viper. The lookup order is:
//
//  1. --provider CLI flag (already bound to viper)
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
func ResolveProvider(v *viper.Viper) ProviderConfig {
	// Determine the active provider name.
	name := v.GetString(ConfigKeyProvider)
	if name == "" {
		name = os.Getenv("PREV_PROVIDER")
	}
	if name == "" {
		name = "openai"
	}
	name = strings.ToLower(strings.TrimSpace(name))

	// Build a sub-viper for the provider's config block.
	sub := v.Sub(fmt.Sprintf("providers.%s", name))
	if sub == nil {
		// No config file entry; create an empty viper so that env-var
		// and flag bindings still work.
		sub = viper.New()
	}

	// Bind common env vars so they override file-based config. Providers
	// that need additional bindings do so in their factory function.
	bindProviderEnvVars(name, sub)

	return ProviderConfig{Name: name, Viper: sub}
}

// bindProviderEnvVars sets up well-known environment variables for each
// provider so that users can configure prev entirely through the shell.
func bindProviderEnvVars(name string, v *viper.Viper) {
	switch name {
	case "openai":
		v.SetDefault("api_key", os.Getenv("OPENAI_API_KEY"))
		v.SetDefault("model", envOrDefault("OPENAI_API_MODEL", "gpt-4o"))
		v.SetDefault("base_url", envOrDefault("OPENAI_API_BASE", "https://api.openai.com/v1"))
	case "anthropic", "claude":
		v.SetDefault("api_key", os.Getenv("ANTHROPIC_API_KEY"))
		v.SetDefault("model", envOrDefault("ANTHROPIC_MODEL", "claude-sonnet-4-20250514"))
		v.SetDefault("base_url", envOrDefault("ANTHROPIC_API_BASE", "https://api.anthropic.com"))
	case "azure":
		v.SetDefault("api_key", os.Getenv("AZURE_OPENAI_API_KEY"))
		v.SetDefault("model", os.Getenv("AZURE_OPENAI_MODEL"))
		v.SetDefault("base_url", os.Getenv("AZURE_OPENAI_ENDPOINT"))
		v.SetDefault("api_version", envOrDefault("AZURE_OPENAI_API_VERSION", "2024-02-01"))
	default:
		// Generic / OpenAI-compatible: try PREV_<PROVIDER>_* env vars.
		prefix := strings.ToUpper(name)
		v.SetDefault("api_key", os.Getenv(fmt.Sprintf("PREV_%s_API_KEY", prefix)))
		v.SetDefault("model", os.Getenv(fmt.Sprintf("PREV_%s_MODEL", prefix)))
		v.SetDefault("base_url", os.Getenv(fmt.Sprintf("PREV_%s_BASE_URL", prefix)))
	}
}

// BindProviderEnvDefaults applies environment variable defaults for a provider.
// Useful when creating a fresh viper instance (e.g. for listing providers).
func BindProviderEnvDefaults(name string, v *viper.Viper) {
	if v == nil {
		return
	}
	bindProviderEnvVars(name, v)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// SampleConfigYAML returns an example config.yml snippet that documents all
// provider settings. It is used by the "prev init" or "prev config" command.
func SampleConfigYAML() string {
	return `# prev configuration
# Active provider (openai | anthropic | azure | ollama | custom).
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

# Display options.
debug: false
max_key_points: 3
max_characters_per_key_point: 100
explain: false
`
}
