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

func TestBuildEffectiveConfig_IncludesReviewControls(t *testing.T) {
	v := config.NewStore()
	v.Set("provider", "openai")
	v.Set("providers.openai.api_key", "sk-test-secret")
	v.Set("review.memory", false)
	v.Set("review.memory_file", ".prev/custom-memory.md")
	v.Set("review.memory_max", 24)
	v.Set("review.native_impact", false)
	v.Set("review.native_impact_max_symbols", 7)
	v.Set("review.fix_prompt", "auto")
	v.Set("review.mention_handle", "@review-bot")

	out := buildEffectiveConfig(config.Config{Viper: v})
	review, ok := out["review"].(map[string]interface{})
	if !assert.True(t, ok) {
		return
	}
	assert.Equal(t, false, review["memory"])
	assert.Equal(t, ".prev/custom-memory.md", review["memory_file"])
	assert.Equal(t, 24, review["memory_max"])
	assert.Equal(t, false, review["native_impact"])
	assert.Equal(t, 7, review["native_impact_max_symbols"])
	assert.Equal(t, "auto", review["fix_prompt"])
	assert.Equal(t, "review-bot", review["mention_handle"])
}

func TestValidateEffectiveConfig_FlagsUnknownProviderAndInvalidReviewControls(t *testing.T) {
	v := config.NewStore()
	v.Set("provider", "mystery")
	v.Set("providers.mystery.base_url", "http://localhost:8080/v1")
	v.Set("review.fix_prompt", "sometimes")
	v.Set("review.mention_handle", "bad handle")
	v.Set("review.memory_max", -1)
	v.Set("review.native_impact_max_symbols", -2)

	err := validateEffectiveConfig(config.Config{Viper: v})
	assert.NotEmpty(t, err)
	assert.Contains(t, err[0], "provider must be one of")
}

func TestValidateEffectiveConfig_FlagsInvalidProviderTimeout(t *testing.T) {
	v := config.NewStore()
	v.Set("provider", "openai")
	v.Set("providers.openai.api_key", "sk-test")
	v.Set("providers.openai.timeout", "not-a-duration")

	err := validateEffectiveConfig(config.Config{Viper: v})
	assert.Len(t, err, 1)
	assert.Contains(t, err[0], "providers.openai.timeout must be a valid duration")
}
