package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExampleConfigs_ParseAndContainProvider(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "examples", "configs", "*.yml"))
	require.NoError(t, err)
	require.NotEmpty(t, paths, "expected example config files in examples/configs")

	allowedFilterMode := map[string]struct{}{
		"": {}, "added": {}, "diff_context": {}, "file": {}, "nofilter": {},
	}
	allowedDiffSource := map[string]struct{}{
		"": {}, "auto": {}, "git": {}, "raw": {}, "api": {},
	}
	allowedSerenaMode := map[string]struct{}{
		"": {}, "auto": {}, "on": {}, "off": {},
	}

	for _, p := range paths {
		store := NewStore()
		require.NoErrorf(t, store.LoadYAMLFile(p), "failed loading %s", p)

		provider := store.GetString("provider")
		require.NotEmptyf(t, provider, "provider is required in %s", p)

		if mode := store.GetString("review.filter_mode"); mode != "" {
			_, ok := allowedFilterMode[mode]
			assert.Truef(t, ok, "invalid review.filter_mode %q in %s", mode, p)
		}
		if src := store.GetString("review.mr_diff_source"); src != "" {
			_, ok := allowedDiffSource[src]
			assert.Truef(t, ok, "invalid review.mr_diff_source %q in %s", src, p)
		}
		if mode := store.GetString("review.serena_mode"); mode != "" {
			_, ok := allowedSerenaMode[mode]
			assert.Truef(t, ok, "invalid review.serena_mode %q in %s", mode, p)
		}
		if n := store.GetInt("review.nitpick"); n != 0 {
			assert.GreaterOrEqualf(t, n, 0, "review.nitpick must be >= 0 in %s", p)
			assert.LessOrEqualf(t, n, 10, "review.nitpick must be <= 10 in %s", p)
		}
	}
}

func TestExampleConfig_GeminiProfile(t *testing.T) {
	p := filepath.Join("..", "..", "examples", "configs", "v1-gemini.yml")
	store := NewStore()
	require.NoError(t, store.LoadYAMLFile(p))

	assert.Equal(t, "gemini", store.GetString("provider"))
	assert.Equal(t, "gemini-2.0-flash", store.GetString("providers.gemini.model"))
	assert.Equal(t, "https://generativelanguage.googleapis.com/v1beta/openai", store.GetString("providers.gemini.base_url"))
}

func TestCIExamples_ContainPrevMRReview(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "examples", "ci", "*.yml"))
	require.NoError(t, err)
	require.NotEmpty(t, paths, "expected CI examples in examples/ci")

	for _, p := range paths {
		raw, err := os.ReadFile(p)
		require.NoErrorf(t, err, "failed reading %s", p)
		content := string(raw)
		assert.Containsf(t, content, "prev mr review", "CI example must invoke prev mr review: %s", p)
		assert.Truef(t, strings.Contains(content, "go install github.com/sanix-darker/prev@"), "CI example should install prev: %s", p)
	}
}
