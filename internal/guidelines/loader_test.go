package guidelines

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPromptSection_Empty(t *testing.T) {
	assert.Equal(t, "", BuildPromptSection(""))
}

func TestBuildPromptSection_DiscoversKnownFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("Use table-driven tests."), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".claude"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".claude", "review.md"), []byte("Favor small functions."), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".github", "instructions"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".github", "instructions", "copilot.md"), []byte("Prefer explicit error checks."), 0644))

	section := BuildPromptSection(dir)
	assert.Contains(t, section, "## Repository Guidelines")
	assert.Contains(t, section, "### AGENTS.md")
	assert.Contains(t, section, "Use table-driven tests.")
	assert.Contains(t, section, "### .claude/review.md")
	assert.Contains(t, section, "Favor small functions.")
	assert.Contains(t, section, "### .github/instructions/copilot.md")
	assert.Contains(t, section, "Prefer explicit error checks.")
}

func TestBuildPromptSection_EnforcesSizeBudget(t *testing.T) {
	dir := t.TempDir()
	large := strings.Repeat("a", maxBytesPerFile+200)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(large), 0644))

	section := BuildPromptSection(dir)
	assert.Contains(t, section, "...[truncated]")
}
