package provider

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestBindProviderEnvVars_OpenAIEnvOverridesConfig(t *testing.T) {
	t.Setenv("OPENAI_API_MODEL", "gpt-5.3-codex")
	t.Setenv("OPENAI_API_KEY", "sk-test")

	v := viper.New()
	v.Set("model", "gpt-4o")
	v.Set("api_key", "file-key")

	bindProviderEnvVars("openai", v)

	assert.Equal(t, "gpt-5.3-codex", v.GetString("model"))
	assert.Equal(t, "sk-test", v.GetString("api_key"))
}

func TestBindProviderEnvVars_OpenAIDefaultWhenUnset(t *testing.T) {
	t.Setenv("OPENAI_API_MODEL", "")

	v := viper.New()
	bindProviderEnvVars("openai", v)

	assert.Equal(t, "gpt-4o", v.GetString("model"))
}
