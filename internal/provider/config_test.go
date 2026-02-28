package provider

import (
	"testing"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestBindProviderEnvVars_OpenAIEnvOverridesConfig(t *testing.T) {
	t.Setenv("OPENAI_API_MODEL", "gpt-5.3-codex")
	t.Setenv("OPENAI_API_KEY", "sk-test")

	v := config.NewStore()
	v.Set("model", "gpt-4o")
	v.Set("api_key", "file-key")

	bindProviderEnvVars("openai", v)

	assert.Equal(t, "gpt-5.3-codex", v.GetString("model"))
	assert.Equal(t, "sk-test", v.GetString("api_key"))
}

func TestBindProviderEnvVars_OpenAIDefaultWhenUnset(t *testing.T) {
	t.Setenv("OPENAI_API_MODEL", "")

	v := config.NewStore()
	bindProviderEnvVars("openai", v)

	assert.Equal(t, "gpt-4o", v.GetString("model"))
}

func TestBindProviderEnvVars_AnthropicBaseURLAlias(t *testing.T) {
	t.Setenv("ANTHROPIC_BASE_URL", "https://example.anthropic.local")

	v := config.NewStore()
	bindProviderEnvVars("anthropic", v)

	assert.Equal(t, "https://example.anthropic.local", v.GetString("base_url"))
}

func TestBindProviderEnvVars_AzureDeploymentAlias(t *testing.T) {
	t.Setenv("AZURE_OPENAI_DEPLOYMENT", "my-deployment")

	v := config.NewStore()
	bindProviderEnvVars("azure", v)

	assert.Equal(t, "my-deployment", v.GetString("model"))
}

func TestBindProviderEnvVars_GeminiDefaultsAndEnv(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "gem-key")
	t.Setenv("GEMINI_MODEL", "gemini-2.5-pro")

	v := config.NewStore()
	bindProviderEnvVars("gemini", v)

	assert.Equal(t, "gem-key", v.GetString("api_key"))
	assert.Equal(t, "gemini-2.5-pro", v.GetString("model"))
	assert.Equal(t, "https://generativelanguage.googleapis.com/v1beta/openai", v.GetString("base_url"))
}
