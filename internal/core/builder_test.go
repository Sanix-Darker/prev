package core

import (
	"strings"
	"testing"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig(explain bool) config.Config {
	return config.Config{
		Debug:                     false,
		MaxKeyPoints:              3,
		MaxCharactersPerKeyPoints: 100,
		ExplainItOrNot:            explain,
	}
}

func TestBuildReviewPrompt_WithExplanation(t *testing.T) {
	conf := testConfig(true)
	prompt := BuildReviewPrompt(conf, "+added line\n-removed line", "")

	assert.Contains(t, prompt, "keypoints")
	assert.Contains(t, prompt, "+added line")
	assert.Contains(t, prompt, "-removed line")
	assert.Contains(t, prompt, "100") // MaxCharactersPerKeyPoints
}

func TestBuildReviewPrompt_WithoutExplanation(t *testing.T) {
	conf := testConfig(false)
	prompt := BuildReviewPrompt(conf, "+some change", "")

	assert.Contains(t, prompt, "No explanations")
	assert.Contains(t, prompt, "+some change")
}

func TestBuildOptimPrompt(t *testing.T) {
	conf := testConfig(false)
	code := "def hello():\n    print('hello world')"
	prompt := BuildOptimPrompt(conf, code)

	assert.Contains(t, prompt, code)
	assert.Contains(t, prompt, "optimal rewrite")
}

func TestBuildReviewPrompt_WithGuidelines(t *testing.T) {
	conf := testConfig(false)
	prompt := BuildReviewPrompt(conf, "+some change", "Use repository logging helpers.")
	assert.Contains(t, prompt, "Repository-specific guidelines")
	assert.Contains(t, prompt, "Use repository logging helpers.")
	assert.Contains(t, prompt, "upstream callers and downstream callees affected")
	assert.Contains(t, prompt, "regression risk and missing/needed tests")
	assert.Contains(t, prompt, "Prioritize source code concerns first.")
	assert.Contains(t, prompt, "focus on typos/spelling/grammar only")
	assert.Contains(t, prompt, "When Change Intent Context is provided")
	assert.Contains(t, prompt, "Do not over-engineer suggestions")
}

func TestReadFileLines(t *testing.T) {
	lines, err := ReadFileLines("../../fixtures/test_diff1.py")
	require.NoError(t, err)
	assert.True(t, len(lines) > 0)
}

func TestReadFileLines_MissingFile(t *testing.T) {
	_, err := ReadFileLines("nonexistent_file.go")
	assert.Error(t, err)
}

func TestBuildDiff(t *testing.T) {
	diff, err := BuildDiff("../../fixtures/test_diff1.py", "../../fixtures/test_diff2.py")
	require.NoError(t, err)
	assert.NotEmpty(t, diff)
	// Should contain + and/or - markers
	assert.True(t, strings.Contains(diff, "+") || strings.Contains(diff, "-"))
}

func TestBuildDiff_SameFile(t *testing.T) {
	diff, err := BuildDiff("../../fixtures/test_diff1.py", "../../fixtures/test_diff1.py")
	require.NoError(t, err)
	// Same file should produce no +/- changes
	assert.NotContains(t, diff, "+ ")
	assert.NotContains(t, diff, "- ")
}

func TestBuildDiff_MissingFile(t *testing.T) {
	_, err := BuildDiff("nonexistent1.py", "../../fixtures/test_diff1.py")
	assert.Error(t, err)
}

func TestBuildMRReviewPrompt(t *testing.T) {
	prompt := BuildMRReviewPrompt(
		"Fix login bug",
		"Fixes the auth token validation",
		"fix/login",
		"main",
		"+ fixed line\n- broken line",
		"normal",
	)

	assert.Contains(t, prompt, "Fix login bug")
	assert.Contains(t, prompt, "fix/login")
	assert.Contains(t, prompt, "main")
	assert.Contains(t, prompt, "CRITICAL")
	assert.Contains(t, prompt, "SEVERITY")
}
