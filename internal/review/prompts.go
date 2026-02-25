package review

import (
	"fmt"
	"strings"

	"github.com/sanix-darker/prev/internal/diffparse"
)

// BuildWalkthroughPrompt builds the pass-1 prompt that asks the AI for a
// high-level walkthrough summary of all changes.
func BuildWalkthroughPrompt(
	branchName, baseBranch string,
	files []CategorizedFile,
	diffStat string,
	strictness string,
	guidelines string,
) string {
	var sb strings.Builder

	sb.WriteString("You are an expert code reviewer performing a walkthrough of branch changes.\n\n")
	sb.WriteString(fmt.Sprintf("## Branch: %s → %s\n\n", branchName, baseBranch))

	// File table
	sb.WriteString("## Changed Files\n\n")
	sb.WriteString("| File | Type | Group | +/- |\n")
	sb.WriteString("|------|------|-------|-----|\n")
	for _, f := range files {
		name := f.NewName
		if name == "" {
			name = f.OldName
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | +%d/-%d |\n",
			name, f.Category, f.Group, f.Stats.Additions, f.Stats.Deletions))
	}
	sb.WriteString("\n")

	// Diff stat
	if diffStat != "" {
		sb.WriteString("## Diff Stats\n```\n")
		sb.WriteString(diffStat)
		sb.WriteString("\n```\n\n")
	}

	// Abbreviated diffs (first ~20 diff lines per file)
	sb.WriteString("## Abbreviated Changes\n\n")
	for _, f := range files {
		if f.IsBinary {
			continue
		}
		name := f.NewName
		if name == "" {
			name = f.OldName
		}
		sb.WriteString(fmt.Sprintf("### %s\n```diff\n", name))
		lineCount := 0
		for _, h := range f.Hunks {
			for _, dl := range h.Lines {
				if lineCount >= 20 {
					sb.WriteString("... (truncated)\n")
					break
				}
				switch dl.Type {
				case diffparse.LineAdded:
					sb.WriteString(fmt.Sprintf("+%s\n", dl.Content))
				case diffparse.LineDeleted:
					sb.WriteString(fmt.Sprintf("-%s\n", dl.Content))
				default:
					sb.WriteString(fmt.Sprintf(" %s\n", dl.Content))
				}
				lineCount++
			}
			if lineCount >= 20 {
				break
			}
		}
		sb.WriteString("```\n\n")
	}

	// Instructions
	sb.WriteString(strictnessInstruction(strictness))
	if strings.TrimSpace(guidelines) != "" {
		sb.WriteString("\n")
		sb.WriteString(guidelines)
		sb.WriteString("\n")
	}
	sb.WriteString(`
## Your Task

Provide:
1. **Summary**: A 2-3 sentence overview of what this branch does and its quality.
2. **Changes Table**: A markdown table with columns: | File | Type | Summary |
   where Summary is a one-line description of what changed in each file.
3. **Sequence Diagram** (optional): If the changes involve interactions between components,
   include a short mermaid sequence diagram.

Respond in Markdown format.
`)

	return sb.String()
}

// BuildFileReviewPrompt builds the pass-2 prompt that asks for detailed
// per-file review of a batch of enriched files.
func BuildFileReviewPrompt(
	batch FileBatch,
	walkthroughSummary string,
	branchName string,
	strictness string,
	guidelines string,
) string {
	var sb strings.Builder

	sb.WriteString("You are an expert code reviewer performing a detailed file-by-file review.\n\n")
	sb.WriteString(fmt.Sprintf("## Branch: %s\n\n", branchName))

	// Include walkthrough context
	if walkthroughSummary != "" {
		sb.WriteString("## Walkthrough Context\n\n")
		sb.WriteString(walkthroughSummary)
		sb.WriteString("\n\n")
	}

	// Enriched diffs for each file in the batch
	sb.WriteString("## Files to Review\n\n")
	for _, f := range batch.Files {
		sb.WriteString(diffparse.FormatEnrichedForReview(f.EnrichedFileChange))
	}

	// Instructions
	sb.WriteString(strictnessInstruction(strictness))
	if strings.TrimSpace(guidelines) != "" {
		sb.WriteString("\n")
		sb.WriteString(guidelines)
		sb.WriteString("\n")
	}
	sb.WriteString(`
## Review Instructions

For each file, provide issues in this exact format:

**file.go:42** [SEVERITY]: Description of the issue

Where SEVERITY is one of: CRITICAL, HIGH, MEDIUM, LOW

When you have a code fix, use this format:
**file.go:42** [SEVERITY]: Description
` + "```suggestion" + `
corrected code here
` + "```" + `

If a file has no significant issues, write:
**file.go**: No significant issues found.

Focus on: bugs, security vulnerabilities, race conditions, error handling,
performance issues, and logic errors. Skip trivial style nits unless strictness is "strict".
Prioritize source-code files first.
For text/documentation files (.md/.txt/.rst/.adoc), report typos/spelling/grammar issues only
unless a critical correctness or security issue is present.
For each finding, include hunk-level impact analysis in the same comment:
- Execution impact: what runtime behavior changes at this exact hunk.
- Call-tree impact: likely upstream callers and downstream callees affected.
- Cross-file risk: contracts/interfaces/config/schemas that may break.
- Regression/test risk: which tests should catch this and what gap exists.
Review every changed line in each hunk and then judge the hunk as a whole (not line-by-line in isolation).
If Change Intent Context is present (commit subjects/messages), validate findings against it
and flag mismatches between intended and actual behavior.
Do not over-engineer suggestions; keep fixes short, concise, and surgical.
`)

	return sb.String()
}

func strictnessInstruction(strictness string) string {
	switch strings.ToLower(strictness) {
	case "strict":
		return `## Strictness: STRICT
Report all issues including style nits and minor improvements. Be thorough.
`
	case "lenient":
		return `## Strictness: LENIENT
Only report CRITICAL and HIGH severity issues. Skip style nits and minor improvements.
`
	default:
		return `## Strictness: NORMAL
Focus on bugs, security vulnerabilities, and significant code quality issues.
Skip trivial style nits. Report MEDIUM severity and above.
`
	}
}
