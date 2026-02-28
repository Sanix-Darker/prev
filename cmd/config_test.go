package cmd

import (
	"testing"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestBuildEffectiveConfig_RedactsProviderSecret(t *testing.T) {
	v := config.NewStore()
	v.Set("provider", "openai")
	v.Set("providers.openai.api_key", "sk-test-secret")
	v.Set("providers.openai.model", "gpt-5.2-chat-latest")
	v.Set("providers.openai.base_url", "https://api.openai.com/v1")

	out := buildEffectiveConfig(config.Config{Viper: v})

	assert.Equal(t, "openai", out["provider"])
	providers, ok := out["providers"].(map[string]interface{})
	if !assert.True(t, ok) {
		return
	}
	openai, ok := providers["openai"].(map[string]interface{})
	if !assert.True(t, ok) {
		return
	}
	assert.Equal(t, "***", openai["api_key"])
	assert.NotEmpty(t, openai["model"])
}

func TestValidateEffectiveConfig_FlagsInvalidReviewMode(t *testing.T) {
	v := config.NewStore()
	v.Set("provider", "openai")
	v.Set("providers.openai.api_key", "sk-test")
	v.Set("review.filter_mode", "bad_mode")

	errs := validateEffectiveConfig(config.Config{Viper: v})
	assert.NotEmpty(t, errs)
	assert.Contains(t, errs[0], "review.filter_mode")
}

func TestValidateEffectiveConfig_AzureRequiresEndpointAndModel(t *testing.T) {
	v := config.NewStore()
	v.Set("provider", "azure")
	v.Set("providers.azure.api_key", "azure-key")

	errs := validateEffectiveConfig(config.Config{Viper: v})
	assert.NotEmpty(t, errs)
	assert.Contains(t, errs, "providers.azure.base_url (or AZURE_OPENAI_ENDPOINT) is required")
	assert.Contains(t, errs, "providers.azure.model (or AZURE_OPENAI_MODEL/AZURE_OPENAI_DEPLOYMENT) is required")
}

func TestValidateEffectiveConfig_GeminiRequiresAPIKey(t *testing.T) {
	v := config.NewStore()
	v.Set("provider", "gemini")

	errs := validateEffectiveConfig(config.Config{Viper: v})
	assert.NotEmpty(t, errs)
	assert.Contains(t, errs, "providers.gemini.api_key (or GEMINI_API_KEY) is required")
}
