package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var commentHeaderPattern = regexp.MustCompile(
	`(?i)^\s*(?:[-*]\s*)?(?:File:\s*)?([^\s]+?\.\w+)\s*(?:\(line\s*(\d+)\)|:(\d+))?\s*(?:\[(\w+)\])?\s*(?:\[(\w+)\])?\s*:?\s*(.*)\s*$`,
)

var relaxedCommentHeaderPattern = regexp.MustCompile(
	`(?i)^\s*(?:[-*]\s*)?(?:File:\s*)?([^\s]+?\.\w+)\s*(?:\(([^)]*)\))?\s*(?:\[(\w+)\])?\s*(?:\[(\w+)\])?\s*:?\s*(.*)\s*$`,
)

var lineInParensPattern = regexp.MustCompile(`(?i)\bline\s*(\d+)\b`)

// ReviewResult holds the parsed AI review output.
type ReviewResult struct {
	Summary      string
	FileComments []FileComment
}

// FileComment represents a review comment on a specific file/line.
type FileComment struct {
	FilePath   string
	Line       int
	Kind       string // ISSUE, SUGGESTION, REMARK
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

	for i, line := range lines {
		if !commentStarted {
			if _, ok := parseCommentHeader(line); ok {
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

// ParseReviewResponseJSON parses JSON review output into structured review.
// Supported root formats:
// 1) {"summary":"...","findings":[...]}
// 2) {"file_comments":[...]}
// 3) [{"file":"...","line":1,...}]
func ParseReviewResponseJSON(content string) (ReviewResult, bool) {
	payload := extractJSONPayload(content)
	if payload == "" {
		return ReviewResult{}, false
	}

	// Object root
	var obj map[string]any
	if err := json.Unmarshal([]byte(payload), &obj); err == nil && len(obj) > 0 {
		result := ReviewResult{}
		if s, ok := obj["summary"].(string); ok {
			result.Summary = strings.TrimSpace(s)
		}
		items := pickJSONFindings(obj)
		if len(items) == 0 {
			return result, false
		}
		result.FileComments = toFileComments(items)
		return result, len(result.FileComments) > 0
	}

	// Array root
	var arr []map[string]any
	if err := json.Unmarshal([]byte(payload), &arr); err == nil && len(arr) > 0 {
		return ReviewResult{FileComments: toFileComments(arr)}, true
	}

	return ReviewResult{}, false
}

func parseFileComments(lines []string) []FileComment {
	var comments []FileComment

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

		header, ok := parseCommentHeader(line)
		if ok {
			// Save previous comment
			if current != nil {
				msg, suggestion := extractSuggestion(msgLines)
				current.Message = msg
				current.Suggestion = suggestion
				comments = append(comments, *current)
			}

			current = &FileComment{
				FilePath: header.filePath,
				Line:     header.line,
				Kind:     header.kind,
				Severity: header.severity,
			}
			msgLines = nil
			if header.message != "" {
				msgLines = append(msgLines, header.message)
			}
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

func extractJSONPayload(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 3 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
			last := len(lines) - 1
			if strings.TrimSpace(lines[last]) == "```" {
				trimmed = strings.TrimSpace(strings.Join(lines[1:last], "\n"))
			}
		}
	}

	switch trimmed[0] {
	case '[':
		if endArr := strings.LastIndex(trimmed, "]"); endArr > 0 {
			return strings.TrimSpace(trimmed[:endArr+1])
		}
	case '{':
		if endObj := strings.LastIndex(trimmed, "}"); endObj > 0 {
			return strings.TrimSpace(trimmed[:endObj+1])
		}
	default:
		startArr := strings.Index(trimmed, "[")
		endArr := strings.LastIndex(trimmed, "]")
		if startArr >= 0 && endArr > startArr {
			return strings.TrimSpace(trimmed[startArr : endArr+1])
		}
		startObj := strings.Index(trimmed, "{")
		endObj := strings.LastIndex(trimmed, "}")
		if startObj >= 0 && endObj > startObj {
			return strings.TrimSpace(trimmed[startObj : endObj+1])
		}
	}
	return ""
}

func pickJSONFindings(obj map[string]any) []map[string]any {
	for _, k := range []string{"findings", "file_comments", "comments", "issues"} {
		raw, ok := obj[k]
		if !ok {
			continue
		}
		items, ok := raw.([]any)
		if !ok {
			continue
		}
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			m, ok := it.(map[string]any)
			if ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

func toFileComments(items []map[string]any) []FileComment {
	out := make([]FileComment, 0, len(items))
	for _, m := range items {
		path := firstString(m, "file", "file_path", "path", "filename")
		if strings.TrimSpace(path) == "" {
			continue
		}
		line := firstInt(m, "line", "new_line", "line_number")
		kind, sev := parseKindAndSeverity(
			firstString(m, "kind", "type"),
			firstString(m, "severity", "level", "priority"),
		)
		msg := firstString(m, "message", "title", "description")
		sug := firstString(m, "suggestion", "patch", "fix")
		out = append(out, FileComment{
			FilePath:   strings.TrimSpace(path),
			Line:       line,
			Kind:       kind,
			Severity:   sev,
			Message:    strings.TrimSpace(msg),
			Suggestion: trimBlankEdgesString(sug),
		})
	}
	return out
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return t
				}
			case fmt.Stringer:
				s := strings.TrimSpace(t.String())
				if s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func firstInt(m map[string]any, keys ...string) int {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case float64:
			return int(t)
		case int:
			return t
		case string:
			n, err := strconv.Atoi(strings.TrimSpace(t))
			if err == nil {
				return n
			}
		}
	}
	return 0
}

type commentHeader struct {
	filePath string
	line     int
	kind     string
	severity string
	message  string
}

func parseCommentHeader(line string) (commentHeader, bool) {
	normalized := strings.TrimSpace(strings.ReplaceAll(line, "**", ""))
	if normalized == "" {
		return commentHeader{}, false
	}

	match := commentHeaderPattern.FindStringSubmatch(normalized)
	if match != nil && strings.TrimSpace(match[1]) != "" {
		hasStructuredMeta := strings.TrimSpace(match[2]) != "" ||
			strings.TrimSpace(match[3]) != "" ||
			strings.TrimSpace(match[4]) != "" ||
			strings.TrimSpace(match[5]) != ""
		if !hasStructuredMeta && strings.HasPrefix(strings.TrimSpace(match[6]), "(") {
			match = nil
		}
	}
	if match != nil && strings.TrimSpace(match[1]) != "" {
		lineNo := 0
		if strings.TrimSpace(match[2]) != "" {
			lineNo, _ = strconv.Atoi(strings.TrimSpace(match[2]))
		} else if strings.TrimSpace(match[3]) != "" {
			lineNo, _ = strconv.Atoi(strings.TrimSpace(match[3]))
		}
		kind, severity := parseKindAndSeverity(match[4], match[5])

		return commentHeader{
			filePath: strings.TrimSpace(match[1]),
			line:     lineNo,
			kind:     kind,
			severity: severity,
			message:  strings.TrimSpace(match[6]),
		}, true
	}

	// Relaxed fallback: accept headers like
	// "File: public/index.php (modified) [ISSUE] [HIGH]: ..."
	relaxed := relaxedCommentHeaderPattern.FindStringSubmatch(normalized)
	if relaxed == nil || strings.TrimSpace(relaxed[1]) == "" {
		return commentHeader{}, false
	}
	lineNo := 0
	if m := lineInParensPattern.FindStringSubmatch(strings.TrimSpace(relaxed[2])); m != nil && len(m) > 1 {
		lineNo, _ = strconv.Atoi(strings.TrimSpace(m[1]))
	}
	kind, severity := parseKindAndSeverity(relaxed[3], relaxed[4])

	return commentHeader{
		filePath: strings.TrimSpace(relaxed[1]),
		line:     lineNo,
		kind:     kind,
		severity: severity,
		message:  strings.TrimSpace(relaxed[5]),
	}, true
}

func parseKindAndSeverity(first, second string) (string, string) {
	kind := "ISSUE"
	severity := "MEDIUM"
	for _, raw := range []string{first, second} {
		token := strings.ToUpper(strings.TrimSpace(raw))
		if token == "" {
			continue
		}
		switch token {
		case "ISSUE", "SUGGESTION", "REMARK":
			kind = token
		case "CRITICAL", "HIGH", "MEDIUM", "LOW":
			severity = token
		}
	}
	return kind, severity
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
	suggestion = strings.Join(trimBlankEdges(sugParts), "\n")
	return
}

func trimBlankEdges(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if start >= end {
		return nil
	}
	return lines[start:end]
}

func trimBlankEdgesString(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	trimmed := trimBlankEdges(lines)
	return strings.Join(trimmed, "\n")
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
	minRank := minSeverityRank(strictness, 0)

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

// FilterForReview filters comments by severity threshold and allowed kinds.
func FilterForReview(comments []FileComment, strictness string, nitpick int, allowedKinds []string) []FileComment {
	minRank := minSeverityRank(strictness, nitpick)
	allowed := normalizeKinds(allowedKinds)

	var filtered []FileComment
	for _, c := range comments {
		if minRank > 0 && severityRank(c.Severity) < minRank {
			continue
		}
		if len(allowed) > 0 {
			if _, ok := allowed[strings.ToUpper(strings.TrimSpace(c.Kind))]; !ok {
				continue
			}
		}
		filtered = append(filtered, c)
	}
	return filtered
}

func normalizeKinds(kinds []string) map[string]struct{} {
	out := make(map[string]struct{}, len(kinds))
	for _, k := range kinds {
		normalized := strings.ToUpper(strings.TrimSpace(k))
		if normalized == "" {
			continue
		}
		out[normalized] = struct{}{}
	}
	return out
}

func minSeverityRank(strictness string, nitpick int) int {
	if nitpick > 0 {
		switch {
		case nitpick <= 2:
			return 4 // CRITICAL only
		case nitpick <= 4:
			return 3 // HIGH+
		case nitpick <= 6:
			return 2 // MEDIUM+
		case nitpick <= 8:
			return 1 // LOW+
		default:
			return 0 // include all
		}
	}

	switch strings.ToLower(strictness) {
	case "strict":
		return 0 // show all
	case "lenient":
		return 3 // HIGH+
	default: // "normal"
		return 2 // MEDIUM+
	}
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
	return BuildMRReviewPromptWithOptions(
		mrTitle,
		mrDescription,
		sourceBranch,
		targetBranch,
		formattedDiffs,
		strictness,
		0,
		[]string{"issue", "suggestion", "remark"},
		"",
	)
}

// BuildMRReviewPromptWithOptions builds a structured prompt for MR review with
// optional nitpick strictness tuning and user guidelines.
func BuildMRReviewPromptWithOptions(
	mrTitle string,
	mrDescription string,
	sourceBranch string,
	targetBranch string,
	formattedDiffs string,
	strictness string,
	nitpick int,
	conventions []string,
	guidelines string,
) string {
	strictnessInstructions := strictnessBlock(strictness)
	nitpickInstructions := nitpickBlock(nitpick)
	conventionInstructions := conventionBlock(conventions)
	guidelineInstructions := guidelineBlock(guidelines)

	return `You are an expert code reviewer. Review this GitLab Merge Request.

## Merge Request Info
- **Title**: ` + mrTitle + `
- **Description**: ` + mrDescription + `
- **Branch**: ` + sourceBranch + ` -> ` + targetBranch + `

## Changes
` + formattedDiffs + `

## Review Instructions
` + strictnessInstructions + `
` + nitpickInstructions + `
` + conventionInstructions + `
` + guidelineInstructions + `

Please provide:

1. **Summary**: 2-3 sentences.

2. **Project Scope Map (before findings)**:
   - Entry points and execution paths touched.
   - Callers/callees impact and cross-module contracts/schemas/config changes.
   - Import/dependency behavior changes.
   - Test surface and target-branch baseline deltas.
   - Use MR title/description as the intended change contract; when commit context is present, validate against it.

3. **Analysis priority**:
   - Source code first.
   - .md/.txt/.rst/.adoc: typos/spelling/grammar only unless critical correctness/security.
   - Prioritize CRITICAL/HIGH, then MEDIUM/LOW.
   - Review each changed hunk line-by-line, then assess full-hunk interaction.
   - For each finding include hunk impact: runtime behavior, callers/callees, regression/test risk.

4. **File-by-file findings** (exact format):
   **File: path/to/file.ext** (line N) [KIND] [SEVERITY]: Description of the issue

   Where KIND is one of: ISSUE, SUGGESTION, REMARK
   and SEVERITY is one of: CRITICAL, HIGH, MEDIUM, LOW

5. **Remediation plan** grouped by severity (CRITICAL/HIGH -> MEDIUM -> LOW) with target files and tests.

6. **Suggestions**: When you have a code fix, use this format:
   **File: path/to/file.ext** (line N) [SUGGESTION] [SEVERITY]: Description
   ` + "```suggestion" + `
   corrected code here
   ` + "```" + `

7. **Output constraints**:
   - concise findings (one short sentence preferred).
   - every finding line must include (line N) with a concrete changed line number.
   - in "File-by-file findings", output only parseable finding lines (no bullets or prose blocks under a file heading).
   - keep suggestion patches scoped to target hunk only.
   - preserve exact code characters/spacing.
   - keep fixes short, concise, and surgical (no over-engineering).

Order findings by severity: CRITICAL, HIGH, MEDIUM, LOW.
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

func nitpickBlock(nitpick int) string {
	if nitpick <= 0 {
		return ""
	}
	if nitpick > 10 {
		nitpick = 10
	}
	return fmt.Sprintf(`## Nitpick Level: %d/10
1 means critical issues only. 10 means include small nits and minor improvements.
Adjust granularity accordingly.
`, nitpick)
}

func conventionBlock(conventions []string) string {
	var normalized []string
	for _, c := range conventions {
		clean := strings.ToUpper(strings.TrimSpace(c))
		if clean != "" {
			normalized = append(normalized, clean)
		}
	}
	if len(normalized) == 0 {
		return ""
	}
	return fmt.Sprintf(`## Comment Conventions
Use KIND labels from this set only: %s
`, strings.Join(normalized, ", "))
}

func guidelineBlock(guidelines string) string {
	guidelines = strings.TrimSpace(guidelines)
	if guidelines == "" {
		return ""
	}
	return "## Review Guidelines\n" + guidelines + "\n"
}
