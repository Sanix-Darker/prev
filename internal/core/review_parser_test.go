package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseReviewResponse_WithSummary(t *testing.T) {
	content := `## Summary
This MR adds authentication to the API.

## Issues
**File: auth.go** (line 42) [HIGH]: Missing error check on token validation
The token could be nil here.

**File: handler.go** (line 15) [MEDIUM]: Consider using context timeout
`

	result := ParseReviewResponse(content)
	assert.NotEmpty(t, result.Summary)
	assert.Contains(t, result.Summary, "authentication")
	assert.Len(t, result.FileComments, 2)

	assert.Equal(t, "auth.go", result.FileComments[0].FilePath)
	assert.Equal(t, 42, result.FileComments[0].Line)
	assert.Equal(t, "HIGH", result.FileComments[0].Severity)

	assert.Equal(t, "handler.go", result.FileComments[1].FilePath)
	assert.Equal(t, 15, result.FileComments[1].Line)
}

func TestParseReviewResponse_NoFileComments(t *testing.T) {
	content := "Everything looks good! No issues found."

	result := ParseReviewResponse(content)
	assert.NotEmpty(t, result.Summary)
	assert.Empty(t, result.FileComments)
}

func TestParseReviewResponse_WithSuggestions(t *testing.T) {
	content := `## Summary
Found one issue.

**File: main.go** (line 10) [HIGH]: Use error wrapping
Consider wrapping the error.
` + "```suggestion" + `
return fmt.Errorf("failed: %w", err)
` + "```" + `

**File: util.go** (line 5) [MEDIUM]: Simplify condition
`

	result := ParseReviewResponse(content)
	assert.Len(t, result.FileComments, 2)

	assert.Equal(t, "main.go", result.FileComments[0].FilePath)
	assert.Equal(t, `return fmt.Errorf("failed: %w", err)`, result.FileComments[0].Suggestion)
	assert.Contains(t, result.FileComments[0].Message, "wrapping")

	assert.Equal(t, "util.go", result.FileComments[1].FilePath)
	assert.Empty(t, result.FileComments[1].Suggestion)
}

func TestParseReviewResponse_NoSuggestion(t *testing.T) {
	content := `## Summary
All good.

**File: server.go** (line 20) [LOW]: Minor style issue
Consider renaming this variable.
`

	result := ParseReviewResponse(content)
	assert.Len(t, result.FileComments, 1)
	assert.Empty(t, result.FileComments[0].Suggestion)
	assert.Contains(t, result.FileComments[0].Message, "renaming")
}

func TestExtractSuggestion(t *testing.T) {
	lines := []string{
		"This function has a bug.",
		"```suggestion",
		"fixed := true",
		"return fixed",
		"```",
		"Please apply the fix.",
	}

	msg, sug := extractSuggestion(lines)
	assert.Contains(t, msg, "This function has a bug.")
	assert.Contains(t, msg, "Please apply the fix.")
	assert.Equal(t, "fixed := true\nreturn fixed", sug)
}

func TestExtractSuggestion_NoBlock(t *testing.T) {
	lines := []string{
		"Just a plain comment.",
		"No code block here.",
	}

	msg, sug := extractSuggestion(lines)
	assert.Contains(t, msg, "Just a plain comment.")
	assert.Empty(t, sug)
}

// --- Severity filtering tests ---

func TestFilterBySeverity_Strict(t *testing.T) {
	comments := []FileComment{
		{Severity: "CRITICAL"}, {Severity: "HIGH"}, {Severity: "MEDIUM"}, {Severity: "LOW"},
	}
	result := FilterBySeverity(comments, "strict")
	assert.Len(t, result, 4)
}

func TestFilterBySeverity_Normal(t *testing.T) {
	comments := []FileComment{
		{Severity: "CRITICAL"}, {Severity: "HIGH"}, {Severity: "MEDIUM"}, {Severity: "LOW"},
	}
	result := FilterBySeverity(comments, "normal")
	assert.Len(t, result, 3) // CRITICAL, HIGH, MEDIUM
}

func TestFilterBySeverity_Lenient(t *testing.T) {
	comments := []FileComment{
		{Severity: "CRITICAL"}, {Severity: "HIGH"}, {Severity: "MEDIUM"}, {Severity: "LOW"},
	}
	result := FilterBySeverity(comments, "lenient")
	assert.Len(t, result, 2) // CRITICAL, HIGH
}

// --- Prompt strictness tests ---

func TestBuildMRReviewPrompt_Strict(t *testing.T) {
	prompt := BuildMRReviewPrompt("title", "desc", "feat", "main", "diffs", "strict")
	assert.Contains(t, prompt, "all issues")
	assert.Contains(t, prompt, "```suggestion")
}

func TestBuildMRReviewPrompt_Normal(t *testing.T) {
	prompt := BuildMRReviewPrompt("title", "desc", "feat", "main", "diffs", "normal")
	assert.Contains(t, prompt, "bugs")
	assert.Contains(t, prompt, "security")
	assert.Contains(t, prompt, "```suggestion")
}

func TestBuildMRReviewPrompt_Lenient(t *testing.T) {
	prompt := BuildMRReviewPrompt("title", "desc", "feat", "main", "diffs", "lenient")
	assert.Contains(t, prompt, "CRITICAL")
	assert.Contains(t, prompt, "HIGH")
	assert.Contains(t, prompt, "```suggestion")
}
