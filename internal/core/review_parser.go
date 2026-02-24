package core

import (
	"regexp"
	"strconv"
	"strings"
)

// ReviewResult holds the parsed AI review output.
type ReviewResult struct {
	Summary      string
	FileComments []FileComment
}

// FileComment represents a review comment on a specific file/line.
type FileComment struct {
	FilePath   string
	Line       int
	Severity   string // CRITICAL, HIGH, MEDIUM, LOW
	Message    string
	Suggestion string
}

// ParseReviewResponse parses an AI markdown response into structured review.
// It looks for patterns like:
//   - **File: path/to/file.go** (line 42) [SEVERITY]: message
//   - ### path/to/file.go:42 [SEVERITY]
func ParseReviewResponse(content string) ReviewResult {
	result := ReviewResult{}

	lines := strings.Split(content, "\n")

	// Extract summary: everything before the first file-specific comment
	var summaryLines []string
	var commentStarted bool

	filePattern := regexp.MustCompile(`(?i)(?:\*\*)?(?:File:\s*)?([^\s*]+\.\w+)(?:\*\*)?\s*(?:\(line\s*(\d+)\)|\:(\d+))?\s*(?:\[(\w+)\])?`)

	for i, line := range lines {
		if !commentStarted {
			match := filePattern.FindStringSubmatch(line)
			if match != nil && match[1] != "" {
				commentStarted = true
				result.Summary = strings.TrimSpace(strings.Join(summaryLines, "\n"))
				// Parse this and remaining lines for file comments
				result.FileComments = parseFileComments(lines[i:])
				break
			}
			summaryLines = append(summaryLines, line)
		}
	}

	if !commentStarted {
		result.Summary = strings.TrimSpace(strings.Join(summaryLines, "\n"))
	}

	return result
}

func parseFileComments(lines []string) []FileComment {
	var comments []FileComment
	filePattern := regexp.MustCompile(`(?i)(?:\*\*)?(?:File:\s*)?([^\s*]+\.\w+)(?:\*\*)?\s*(?:\(line\s*(\d+)\)|\:(\d+))?\s*(?:\[(\w+)\])?`)

	var current *FileComment
	var msgLines []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track fenced code blocks to avoid matching file patterns inside them
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			if current != nil {
				msgLines = append(msgLines, line)
			}
			continue
		}

		if inCodeBlock {
			if current != nil {
				msgLines = append(msgLines, line)
			}
			continue
		}

		match := filePattern.FindStringSubmatch(line)
		if match != nil && match[1] != "" {
			// Save previous comment
			if current != nil {
				msg, suggestion := extractSuggestion(msgLines)
				current.Message = msg
				current.Suggestion = suggestion
				comments = append(comments, *current)
			}

			lineNo := 0
			if match[2] != "" {
				lineNo, _ = strconv.Atoi(match[2])
			} else if match[3] != "" {
				lineNo, _ = strconv.Atoi(match[3])
			}

			severity := strings.ToUpper(match[4])
			if severity == "" {
				severity = "MEDIUM"
			}

			current = &FileComment{
				FilePath: match[1],
				Line:     lineNo,
				Severity: severity,
			}
			msgLines = nil
		} else if current != nil {
			msgLines = append(msgLines, line)
		}
	}

	// Save last comment
	if current != nil {
		msg, suggestion := extractSuggestion(msgLines)
		current.Message = msg
		current.Suggestion = suggestion
		comments = append(comments, *current)
	}

	return comments
}

// extractSuggestion scans message lines for a ```suggestion fenced block.
// Returns the message text (without the suggestion block) and the suggestion content.
func extractSuggestion(msgLines []string) (message, suggestion string) {
	var msgParts []string
	var sugParts []string
	inSuggestion := false

	for _, line := range msgLines {
		trimmed := strings.TrimSpace(line)
		if !inSuggestion && strings.HasPrefix(trimmed, "```suggestion") {
			inSuggestion = true
			continue
		}
		if inSuggestion {
			if trimmed == "```" {
				inSuggestion = false
				continue
			}
			sugParts = append(sugParts, line)
		} else {
			msgParts = append(msgParts, line)
		}
	}

	message = strings.TrimSpace(strings.Join(msgParts, "\n"))
	suggestion = strings.TrimSpace(strings.Join(sugParts, "\n"))
	return
}

// severityRank maps severity strings to numeric ranks for filtering.
func severityRank(sev string) int {
	switch strings.ToUpper(sev) {
	case "CRITICAL":
		return 4
	case "HIGH":
		return 3
	case "MEDIUM":
		return 2
	case "LOW":
		return 1
	default:
		return 0
	}
}

// FilterBySeverity filters comments based on strictness level.
// strict: all severities, normal: MEDIUM+, lenient: HIGH+.
func FilterBySeverity(comments []FileComment, strictness string) []FileComment {
	minRank := 0
	switch strings.ToLower(strictness) {
	case "strict":
		minRank = 0 // show all
	case "lenient":
		minRank = 3 // HIGH+
	default: // "normal"
		minRank = 2 // MEDIUM+
	}

	if minRank == 0 {
		return comments
	}

	var filtered []FileComment
	for _, c := range comments {
		if severityRank(c.Severity) >= minRank {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// BuildMRReviewPrompt builds a structured prompt for MR review.
func BuildMRReviewPrompt(
	mrTitle string,
	mrDescription string,
	sourceBranch string,
	targetBranch string,
	formattedDiffs string,
	strictness string,
) string {
	strictnessInstructions := strictnessBlock(strictness)

	return `You are an expert code reviewer. Review this GitLab Merge Request.

## Merge Request Info
- **Title**: ` + mrTitle + `
- **Description**: ` + mrDescription + `
- **Branch**: ` + sourceBranch + ` -> ` + targetBranch + `

## Changes
` + formattedDiffs + `

## Review Instructions
` + strictnessInstructions + `

Please provide:

1. **Summary**: A brief overview of the changes and their quality (2-3 sentences).

2. **File-by-file analysis**: For each file with issues, format as:
   **File: path/to/file.ext** (line N) [SEVERITY]: Description of the issue

   Where SEVERITY is one of: CRITICAL, HIGH, MEDIUM, LOW

3. **Suggestions**: When you have a code fix, use this format:
   **File: path/to/file.ext** (line N) [SEVERITY]: Description
   ` + "```suggestion" + `
   corrected code here
   ` + "```" + `

Keep the review focused and actionable.
Respond in Markdown format.`
}

func strictnessBlock(strictness string) string {
	switch strings.ToLower(strictness) {
	case "strict":
		return `Report all issues. Be thorough: flag bugs, security issues, performance problems,
style violations, and any code that could be improved. Include LOW severity items.`
	case "lenient":
		return `Only report CRITICAL and HIGH severity issues. Be concise. Skip style nits,
minor improvements, and LOW/MEDIUM issues entirely. Focus on bugs and security vulnerabilities.`
	default: // "normal"
		return `Focus on bugs, security vulnerabilities, and significant code quality issues.
Skip trivial style nits. Report MEDIUM severity and above.`
	}
}
