package cmd

import (
	"testing"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestResolvedModelForLog_CLIWins(t *testing.T) {
	v := config.NewStore()
	v.Set("provider", "openai")
	v.Set("providers.openai.model", "gpt-4o")
	conf := config.Config{Viper: v, Provider: "openai", Model: "gpt-5.3-codex"}
	assert.Equal(t, "gpt-5.3-codex", resolvedModelForLog(conf, "fallback"))
}

func TestResolvedModelForLog_ConfigUsed(t *testing.T) {
	t.Setenv("OPENAI_API_MODEL", "")
	v := config.NewStore()
	v.Set("provider", "openai")
	v.Set("providers.openai.model", "gpt-4o")
	conf := config.Config{Viper: v, Provider: "openai"}
	assert.Equal(t, "gpt-4o", resolvedModelForLog(conf, "fallback"))
}

func TestResolvedModelForLog_EnvOverridesConfig(t *testing.T) {
	t.Setenv("OPENAI_API_MODEL", "gpt-5.3-codex")
	v := config.NewStore()
	v.Set("provider", "openai")
	v.Set("providers.openai.model", "gpt-4o")
	conf := config.Config{Viper: v, Provider: "openai"}
	assert.Equal(t, "gpt-5.3-codex", resolvedModelForLog(conf, "fallback"))
}
