package handlers

import (
	"os"
	"testing"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConf() config.Config {
	return config.Config{Debug: false}
}

func noopHelp() error { return nil }

func TestExtractDiffHandler_ValidInput(t *testing.T) {
	if os.Getenv("PREV_SKIP_DIFF_TESTS") == "1" {
		t.Skip("skipped due to PREV_SKIP_DIFF_TESTS=1")
	}
	d, err := ExtractDiffHandler(
		testConf(),
		"../../fixtures/test_diff1.py,../../fixtures/test_diff2.py",
		noopHelp,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, d)
}
