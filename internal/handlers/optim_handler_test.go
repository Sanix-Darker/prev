package handlers

import (
	"testing"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractOptimHandler_WithFile(t *testing.T) {
	conf := config.Config{Debug: false}
	prompt, err := ExtractOptimHandler(
		conf,
		[]string{"../../fixtures/test_diff1.py"},
		noopHelp,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "optimal rewrite")
}
