package review

import (
	"fmt"
	"strings"
)

// FormatBranchReview formats a BranchReviewResult into CLI-friendly markdown.
func FormatBranchReview(result *BranchReviewResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Branch Review: %s → %s\n\n", result.BranchName, result.BaseBranch))

	// Walkthrough
	sb.WriteString("## Walkthrough\n\n")
	if result.Walkthrough.Summary != "" {
		sb.WriteString(result.Walkthrough.Summary)
		sb.WriteString("\n\n")
	}

	// Changes table
	if result.Walkthrough.ChangesTable != "" {
		sb.WriteString("## Changes\n\n")
		sb.WriteString(result.Walkthrough.ChangesTable)
		sb.WriteString("\n\n")
	}

	// Detailed review
	sb.WriteString("## Detailed Review\n\n")
	issueCount := 0
	severityCounts := map[string]int{}

	for _, fr := range result.FileReviews {
		sb.WriteString(fmt.Sprintf("### %s\n\n", fr.FilePath))

		if len(fr.Comments) == 0 {
			if fr.Summary != "" {
				sb.WriteString(fr.Summary)
			} else {
				sb.WriteString("No significant issues found.")
			}
			sb.WriteString("\n\n")
			continue
		}

		for _, c := range fr.Comments {
			issueCount++
			severityCounts[c.Severity]++

			if c.Line > 0 {
				sb.WriteString(fmt.Sprintf("**%s:%d** [%s]: %s\n",
					c.FilePath, c.Line, c.Severity, c.Message))
			} else {
				sb.WriteString(fmt.Sprintf("**%s** [%s]: %s\n",
					c.FilePath, c.Severity, c.Message))
			}

			if c.Suggestion != "" {
				sb.WriteString("```suggestion\n")
				sb.WriteString(c.Suggestion)
				sb.WriteString("\n```\n")
			}
			sb.WriteString("\n")
		}
	}

	// Statistics
	sb.WriteString("## Statistics\n\n")
	sb.WriteString(fmt.Sprintf("- Files reviewed: %d\n", result.TotalFiles))

	if issueCount > 0 {
		parts := []string{}
		for _, sev := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"} {
			if cnt, ok := severityCounts[sev]; ok && cnt > 0 {
				parts = append(parts, fmt.Sprintf("%d %s", cnt, sev))
			}
		}
		sb.WriteString(fmt.Sprintf("- Issues: %d (%s)\n", issueCount, strings.Join(parts, ", ")))
	} else {
		sb.WriteString("- Issues: 0\n")
	}

	sb.WriteString(fmt.Sprintf("- Changes: +%d/-%d\n", result.TotalAdditions, result.TotalDeletions))

	return sb.String()
}

// FormatCommitReview formats a CommitReviewResult into CLI-friendly markdown.
func FormatCommitReview(result *CommitReviewResult) string {
	var sb strings.Builder

	subject := result.CommitSubject
	if subject != "" {
		subject = " - " + subject
	}
	sb.WriteString(fmt.Sprintf("## Commit %s%s\n\n", result.CommitHash, subject))
	formatReviewBody(&sb, result.Walkthrough, result.FileReviews, result.TotalFiles, result.TotalAdditions, result.TotalDeletions)

	return sb.String()
}

// FormatMultiCommitReview formats multiple commit reviews into a single report.
func FormatMultiCommitReview(result *MultiCommitReviewResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Branch Review (Per-Commit): %s → %s\n\n", result.BranchName, result.BaseBranch))

	for i, cr := range result.Commits {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(FormatCommitReview(&cr))
	}

	return sb.String()
}

func formatReviewBody(
	sb *strings.Builder,
	walkthrough WalkthroughResult,
	fileReviews []FileReviewResult,
	totalFiles, totalAdditions, totalDeletions int,
) {
	// Walkthrough
	sb.WriteString("### Walkthrough\n\n")
	if walkthrough.Summary != "" {
		sb.WriteString(walkthrough.Summary)
		sb.WriteString("\n\n")
	}

	// Changes table
	if walkthrough.ChangesTable != "" {
		sb.WriteString("### Changes\n\n")
		sb.WriteString(walkthrough.ChangesTable)
		sb.WriteString("\n\n")
	}

	// Detailed review
	sb.WriteString("### Detailed Review\n\n")
	issueCount := 0
	severityCounts := map[string]int{}

	for _, fr := range fileReviews {
		sb.WriteString(fmt.Sprintf("#### %s\n\n", fr.FilePath))

		if len(fr.Comments) == 0 {
			if fr.Summary != "" {
				sb.WriteString(fr.Summary)
			} else {
				sb.WriteString("No significant issues found.")
			}
			sb.WriteString("\n\n")
			continue
		}

		for _, c := range fr.Comments {
			issueCount++
			severityCounts[c.Severity]++

			if c.Line > 0 {
				sb.WriteString(fmt.Sprintf("**%s:%d** [%s]: %s\n",
					c.FilePath, c.Line, c.Severity, c.Message))
			} else {
				sb.WriteString(fmt.Sprintf("**%s** [%s]: %s\n",
					c.FilePath, c.Severity, c.Message))
			}

			if c.Suggestion != "" {
				sb.WriteString("```suggestion\n")
				sb.WriteString(c.Suggestion)
				sb.WriteString("\n```\n")
			}
			sb.WriteString("\n")
		}
	}

	// Statistics
	sb.WriteString("### Statistics\n\n")
	sb.WriteString(fmt.Sprintf("- Files reviewed: %d\n", totalFiles))

	if issueCount > 0 {
		parts := []string{}
		for _, sev := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"} {
			if cnt, ok := severityCounts[sev]; ok && cnt > 0 {
				parts = append(parts, fmt.Sprintf("%d %s", cnt, sev))
			}
		}
		sb.WriteString(fmt.Sprintf("- Issues: %d (%s)\n", issueCount, strings.Join(parts, ", ")))
	} else {
		sb.WriteString("- Issues: 0\n")
	}

	sb.WriteString(fmt.Sprintf("- Changes: +%d/-%d\n", totalAdditions, totalDeletions))
}
