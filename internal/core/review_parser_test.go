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
	assert.Contains(t, result.FileComments[0].Message, "Missing error check on token validation")

	assert.Equal(t, "handler.go", result.FileComments[1].FilePath)
	assert.Equal(t, 15, result.FileComments[1].Line)
	assert.Contains(t, result.FileComments[1].Message, "Consider using context timeout")
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

**File: main.go** (line 10) [SUGGESTION] [HIGH]: Use error wrapping
Consider wrapping the error.
` + "```suggestion" + `
return fmt.Errorf("failed: %w", err)
` + "```" + `

**File: util.go** (line 5) [MEDIUM]: Simplify condition
`

	result := ParseReviewResponse(content)
	assert.Len(t, result.FileComments, 2)

	assert.Equal(t, "main.go", result.FileComments[0].FilePath)
	assert.Equal(t, "SUGGESTION", result.FileComments[0].Kind)
	assert.Equal(t, `return fmt.Errorf("failed: %w", err)`, result.FileComments[0].Suggestion)
	assert.Contains(t, result.FileComments[0].Message, "wrapping")

	assert.Equal(t, "util.go", result.FileComments[1].FilePath)
	assert.Empty(t, result.FileComments[1].Suggestion)
	assert.Contains(t, result.FileComments[1].Message, "Simplify condition")
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

func TestParseReviewResponse_InlineHeaderMessageOnly(t *testing.T) {
	content := `## Summary
Review output.

- **File: api/handler.go** (line 88) [ISSUE] [HIGH]: Missing nil check before dereference.
`

	result := ParseReviewResponse(content)
	assert.Len(t, result.FileComments, 1)
	assert.Equal(t, "api/handler.go", result.FileComments[0].FilePath)
	assert.Equal(t, 88, result.FileComments[0].Line)
	assert.Equal(t, "ISSUE", result.FileComments[0].Kind)
	assert.Equal(t, "HIGH", result.FileComments[0].Severity)
	assert.Equal(t, "Missing nil check before dereference.", result.FileComments[0].Message)
}

func TestParseReviewResponse_RelaxedFileHeaderWithoutLine(t *testing.T) {
	content := `## Findings
**File: public/index.php** (modified) [ISSUE] [HIGH]: Verify json_encode error handling.
`

	result := ParseReviewResponse(content)
	assert.Len(t, result.FileComments, 1)
	assert.Equal(t, "public/index.php", result.FileComments[0].FilePath)
	assert.Equal(t, 0, result.FileComments[0].Line)
	assert.Equal(t, "ISSUE", result.FileComments[0].Kind)
	assert.Equal(t, "HIGH", result.FileComments[0].Severity)
	assert.Equal(t, "Verify json_encode error handling.", result.FileComments[0].Message)
}

func TestParseReviewResponseJSON_ObjectRoot(t *testing.T) {
	content := `{
  "summary": "One high issue found.",
  "findings": [
    {
      "file_path": "public/index.php",
      "line": 42,
      "kind": "ISSUE",
      "severity": "HIGH",
      "message": "Missing Content-Type header on JSON responses.",
      "suggestion": "header('Content-Type: application/json');"
    }
  ]
}`
	result, ok := ParseReviewResponseJSON(content)
	assert.True(t, ok)
	assert.Equal(t, "One high issue found.", result.Summary)
	if assert.Len(t, result.FileComments, 1) {
		assert.Equal(t, "public/index.php", result.FileComments[0].FilePath)
		assert.Equal(t, 42, result.FileComments[0].Line)
		assert.Equal(t, "ISSUE", result.FileComments[0].Kind)
		assert.Equal(t, "HIGH", result.FileComments[0].Severity)
	}
}

func TestParseReviewResponseJSON_FencedArrayRoot(t *testing.T) {
	content := "```json\n" + `[
  {
    "file": "src/app.go",
    "new_line": 18,
    "type": "suggestion",
    "level": "medium",
    "description": "Prefer context-aware timeout.",
    "patch": "ctx, cancel := context.WithTimeout(ctx, time.Second)"
  }
]` + "\n```"
	result, ok := ParseReviewResponseJSON(content)
	assert.True(t, ok)
	if assert.Len(t, result.FileComments, 1) {
		assert.Equal(t, "src/app.go", result.FileComments[0].FilePath)
		assert.Equal(t, 18, result.FileComments[0].Line)
		assert.Equal(t, "SUGGESTION", result.FileComments[0].Kind)
		assert.Equal(t, "MEDIUM", result.FileComments[0].Severity)
		assert.Equal(t, "ctx, cancel := context.WithTimeout(ctx, time.Second)", result.FileComments[0].Suggestion)
	}
}

func TestParseReviewResponseJSON_PreservesSuggestionIndentation(t *testing.T) {
	content := `{
  "findings": [
    {
      "file_path": "public/index.php",
      "line": 12,
      "kind": "SUGGESTION",
      "severity": "HIGH",
      "message": "Preserve spacing.",
      "suggestion": "\n\n    $value = trim($value);\n\treturn $value;\n\n"
    }
  ]
}`
	result, ok := ParseReviewResponseJSON(content)
	assert.True(t, ok)
	if assert.Len(t, result.FileComments, 1) {
		assert.Equal(t, "    $value = trim($value);\n\treturn $value;", result.FileComments[0].Suggestion)
	}
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

func TestExtractSuggestion_PreservesPadding(t *testing.T) {
	lines := []string{
		"```suggestion",
		"    $value = trim($value);",
		"\treturn $value;",
		"```",
	}

	_, sug := extractSuggestion(lines)
	assert.Equal(t, "    $value = trim($value);\n\treturn $value;", sug)
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

func TestFilterForReview_NitpickAndKinds(t *testing.T) {
	comments := []FileComment{
		{Kind: "ISSUE", Severity: "CRITICAL"},
		{Kind: "ISSUE", Severity: "MEDIUM"},
		{Kind: "REMARK", Severity: "LOW"},
	}
	result := FilterForReview(comments, "normal", 4, []string{"issue"})
	assert.Len(t, result, 1) // nitpick 4 -> HIGH+ and ISSUE only
	assert.Equal(t, "CRITICAL", result[0].Severity)
	assert.Equal(t, "ISSUE", result[0].Kind)
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

func TestBuildMRReviewPromptWithOptions(t *testing.T) {
	prompt := BuildMRReviewPromptWithOptions(
		"title", "desc", "feat", "main", "diffs", "normal", 9,
		[]string{"issue", "suggestion", "remark"},
		"Use concise comments.",
	)
	assert.Contains(t, prompt, "Nitpick Level: 9/10")
	assert.Contains(t, prompt, "ISSUE, SUGGESTION, REMARK")
	assert.Contains(t, prompt, "Use concise comments.")
	assert.Contains(t, prompt, "[KIND] [SEVERITY]")
	assert.Contains(t, prompt, "callers/callees")
	assert.Contains(t, prompt, "regression/test risk")
	assert.Contains(t, prompt, "MR title/description as the intended change contract")
}
