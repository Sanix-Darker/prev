package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/sanix-darker/prev/internal/core"
	"github.com/sanix-darker/prev/internal/diffparse"
	"github.com/sanix-darker/prev/internal/vcs"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveMentionHandle_FromConfig(t *testing.T) {
	v := config.NewStore()
	v.Set("review.mention_handle", "@ange.saadjio")
	conf := config.Config{Viper: v}

	assert.Equal(t, "ange.saadjio", resolveMentionHandle(conf))
}

func TestResolveMentionHandle_EnvOverridesConfig(t *testing.T) {
	t.Setenv("PREV_MENTION_HANDLE", "@bot-user")
	v := config.NewStore()
	v.Set("review.mention_handle", "@ange.saadjio")
	conf := config.Config{Viper: v}

	assert.Equal(t, "bot-user", resolveMentionHandle(conf))
}

func TestResolveMentionHandle_EmptyWhenUnset(t *testing.T) {
	conf := config.Config{Viper: config.NewStore()}
	assert.Equal(t, "", resolveMentionHandle(conf))
}

func TestHasMentionCommand(t *testing.T) {
	assert.True(t, hasMentionCommand("@ange.saadjio review this", "ange.saadjio", "review"))
	assert.False(t, hasMentionCommand("@ange.saadjio review this", "", "review"))
	assert.False(t, hasMentionCommand("@someoneelse review this", "ange.saadjio", "review"))
}

func TestInlineKey_IgnoresBody(t *testing.T) {
	k1 := inlineKey("a/b.go", 42, "[HIGH] first")
	k2 := inlineKey("a/b.go", 42, "[HIGH] second different message")
	assert.Equal(t, k1, k2)
}

func TestInlineSeverityKey_IncludesSeverity(t *testing.T) {
	k1 := inlineSeverityKey("a/b.go", 42, "HIGH")
	k2 := inlineSeverityKey("a/b.go", 42, "MEDIUM")
	assert.NotEqual(t, k1, k2)
}

func TestIsReplyRequest_WithQuestionMention(t *testing.T) {
	assert.True(t, isReplyRequest("@ange.saadjio you see other issue ?", "ange.saadjio"))
	assert.True(t, isReplyRequest("ange.saadjio can you check this?", "ange.saadjio"))
	assert.False(t, isReplyRequest("@someoneelse can you check this?", "ange.saadjio"))
}

func TestConciseInlineBody(t *testing.T) {
	body := "[HIGH] This is a long first sentence. Additional details should be trimmed.\n\nSecond paragraph."
	assert.Equal(t, "[HIGH] This is a long first sentence.", conciseInlineBody(body))
}

func TestConciseInlineBody_PreservesKeyPointsList(t *testing.T) {
	body := "[HIGH] Key points:\n- First issue with details.\n- Second issue with details."
	got := conciseInlineBody(body)
	assert.Contains(t, got, "Key points:")
	assert.Contains(t, got, "- First issue with details.")
	assert.Contains(t, got, "- Second issue with details.")
}

func TestCollectValidPositions_TracksOldToNewLines(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "public/index.php",
			Hunks: []diffparse.Hunk{
				{
					NewStart: 30,
					NewLines: 2,
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineContext, OldLineNo: 30, NewLineNo: 30},
						{Type: diffparse.LineAdded, OldLineNo: 0, NewLineNo: 31},
					},
				},
			},
		},
	}

	pos := collectValidPositions(changes)
	newLine, old, ok := resolveInlinePosition(pos, "public/index.php", 30)
	assert.True(t, ok)
	assert.Equal(t, 31, newLine) // snaps from context line to added line below in same hunk
	assert.Equal(t, 0, old)

	newLine, old, ok = resolveInlinePosition(pos, "public/index.php", 31)
	assert.True(t, ok)
	assert.Equal(t, 31, newLine)
	assert.Equal(t, 0, old)
}

func TestResolveInlinePosition_PrefersSameHunkBelow(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "public/index.php",
			Hunks: []diffparse.Hunk{
				{
					NewStart: 10,
					NewLines: 5,
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineContext, OldLineNo: 10, NewLineNo: 10},
						{Type: diffparse.LineAdded, OldLineNo: 0, NewLineNo: 11},
						{Type: diffparse.LineContext, OldLineNo: 11, NewLineNo: 12},
					},
				},
				{
					NewStart: 30,
					NewLines: 5,
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineContext, OldLineNo: 30, NewLineNo: 30},
						{Type: diffparse.LineAdded, OldLineNo: 0, NewLineNo: 31},
						{Type: diffparse.LineContext, OldLineNo: 31, NewLineNo: 32},
					},
				},
			},
		},
	}

	pos := collectValidPositions(changes)
	newLine, old, ok := resolveInlinePosition(pos, "public/index.php", 30)
	assert.True(t, ok)
	assert.Equal(t, 31, newLine)
	assert.Equal(t, 0, old)
}

func TestResolveInlinePosition_UsesNearestHunkWhenLineOutside(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "public/index.php",
			Hunks: []diffparse.Hunk{
				{
					NewStart: 50,
					NewLines: 3,
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineAdded, OldLineNo: 0, NewLineNo: 51},
					},
				},
			},
		},
	}

	pos := collectValidPositions(changes)
	newLine, old, ok := resolveInlinePosition(pos, "public/index.php", 49)
	assert.True(t, ok)
	assert.Equal(t, 51, newLine)
	assert.Equal(t, 0, old)

	_, _, ok = resolveInlinePosition(pos, "missing.php", 10)
	assert.False(t, ok)
}

func TestCollectValidPositions_ExactAddedLine(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "public/index.php",
			Hunks: []diffparse.Hunk{
				{
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineAdded, OldLineNo: 0, NewLineNo: 31},
					},
				},
			},
		},
	}

	pos := collectValidPositions(changes)
	newLine, old, ok := resolveInlinePosition(pos, "public/index.php", 31)
	assert.True(t, ok)
	assert.Equal(t, 31, newLine)
	assert.Equal(t, 0, old)
}

func TestResolveInlinePosition_ContextAboveUsesBelowFirst(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "public/index.php",
			Hunks: []diffparse.Hunk{
				{
					NewStart: 100,
					NewLines: 6,
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineAdded, OldLineNo: 0, NewLineNo: 101},
						{Type: diffparse.LineContext, OldLineNo: 101, NewLineNo: 102},
						{Type: diffparse.LineAdded, OldLineNo: 0, NewLineNo: 103},
					},
				},
			},
		},
	}

	pos := collectValidPositions(changes)
	newLine, _, ok := resolveInlinePosition(pos, "public/index.php", 102)
	assert.True(t, ok)
	assert.Equal(t, 103, newLine)
}

func TestResolveInlinePosition_WithOnlyContextFallsBackExact(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "public/index.php",
			Hunks: []diffparse.Hunk{
				{
					NewStart: 10,
					NewLines: 2,
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineContext, OldLineNo: 10, NewLineNo: 10},
					},
				},
			},
		},
	}

	pos := collectValidPositions(changes)
	newLine, old, ok := resolveInlinePosition(pos, "public/index.php", 10)
	assert.True(t, ok)
	assert.Equal(t, 10, newLine)
	assert.Equal(t, 10, old)
}

func TestRefineInlinePositionByMessage_PrefersMatchingAddedLine(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "public/index.php",
			Hunks: []diffparse.Hunk{
				{
					NewStart: 20,
					NewLines: 5,
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineContext, OldLineNo: 20, NewLineNo: 20, Content: "$out = '';"},
						{Type: diffparse.LineAdded, OldLineNo: 0, NewLineNo: 21, Content: "$tmp = build_payload($data);"},
						{Type: diffparse.LineAdded, OldLineNo: 0, NewLineNo: 22, Content: "echo json_encode($tmp);"},
					},
				},
			},
		},
	}
	pos := collectValidPositions(changes)
	newLine, old, ok := resolveInlinePosition(pos, "public/index.php", 20)
	require.True(t, ok)
	require.Equal(t, 21, newLine) // default below-first snap
	require.Equal(t, 0, old)

	refinedLine, refinedOld := refineInlinePositionByMessage(
		pos["public/index.php"],
		20,
		newLine,
		"[HIGH] json_encode() result is returned directly without checking for false.",
	)
	assert.Equal(t, 22, refinedLine)
	assert.Equal(t, 0, refinedOld)
}

func TestRefineInlinePositionByMessage_KeepExactAddedAnchor(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "public/index.php",
			Hunks: []diffparse.Hunk{
				{
					NewStart: 30,
					NewLines: 4,
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineAdded, OldLineNo: 0, NewLineNo: 30, Content: "echo json_encode(['error' => 'Unknown action']);"},
						{Type: diffparse.LineAdded, OldLineNo: 0, NewLineNo: 31, Content: "echo json_decode(['error' => 'Unknown action']);"},
					},
				},
			},
		},
	}

	pos := collectValidPositions(changes)
	newLine, old, ok := resolveInlinePosition(pos, "public/index.php", 31)
	require.True(t, ok)
	require.Equal(t, 31, newLine)
	require.Equal(t, 0, old)

	refinedLine, refinedOld := refineInlinePositionByMessage(
		pos["public/index.php"],
		31,
		newLine,
		"[HIGH] json_decode() expects a JSON string, but receives an array.",
	)
	assert.Equal(t, 31, refinedLine)
	assert.Equal(t, 0, refinedOld)
}

func TestIsMRPaused_RespectsPauseResumeOrder(t *testing.T) {
	notes := []vcs.MRNote{
		{Body: "@ange.saadjio pause"},
		{Body: "some other note"},
	}
	assert.True(t, isMRPaused(notes, "ange.saadjio"))

	notes = append(notes, vcs.MRNote{Body: "@ange.saadjio resume"})
	assert.False(t, isMRPaused(notes, "ange.saadjio"))
}

func TestPausedDiscussions_ScopedPerThread(t *testing.T) {
	discussions := []vcs.MRDiscussion{
		{
			ID: "d1",
			Notes: []vcs.MRDiscussionNote{
				{Body: "@ange.saadjio pause"},
			},
		},
		{
			ID: "d2",
			Notes: []vcs.MRDiscussionNote{
				{Body: "@ange.saadjio pause"},
				{Body: "@ange.saadjio resume"},
			},
		},
	}
	paused := pausedDiscussions(discussions, "ange.saadjio")
	assert.True(t, paused["d1"])
	assert.False(t, paused["d2"])
}

func TestAggregateCommentsByChange_MergesToSingleComment(t *testing.T) {
	comments := []core.FileComment{
		{FilePath: "api/handler.go", Line: 42, Kind: "ISSUE", Severity: "HIGH", Message: "Missing nil check before dereference."},
		{FilePath: "api/handler.go", Line: 42, Kind: "ISSUE", Severity: "MEDIUM", Message: "Error context should include request id."},
		{FilePath: "api/handler.go", Line: 42, Kind: "ISSUE", Severity: "MEDIUM", Message: "Error context should include request id."}, // duplicate
		{FilePath: "api/handler.go", Line: 50, Kind: "ISSUE", Severity: "LOW", Message: "Minor naming cleanup."},
	}

	got := aggregateCommentsByChange(comments)
	assert.Len(t, got, 2)
	assert.Equal(t, "api/handler.go", got[0].FilePath)
	assert.Equal(t, 42, got[0].Line)
	assert.Equal(t, "HIGH", got[0].Severity)
	assert.Contains(t, got[0].Message, "Key points:")
	assert.Contains(t, got[0].Message, "Missing nil check before dereference.")
	assert.Contains(t, got[0].Message, "Error context should include request id.")
	assert.Equal(t, "api/handler.go", got[1].FilePath)
	assert.Equal(t, 50, got[1].Line)
	assert.Equal(t, "Minor naming cleanup.", got[1].Message)
}

func TestFilterCommentsByFileFocus_DocFilesTypoOnly(t *testing.T) {
	comments := []core.FileComment{
		{FilePath: "README.md", Line: 10, Message: "This sentence has a typo in configuration."},
		{FilePath: "README.md", Line: 11, Message: "Architecture section should be reorganized."},
		{FilePath: "src/main.go", Line: 30, Message: "Possible nil dereference in handler."},
	}

	got := filterCommentsByFileFocus(comments)
	assert.Len(t, got, 2)
	assert.Equal(t, "README.md", got[0].FilePath)
	assert.Contains(t, got[0].Message, "typo")
	assert.Equal(t, "src/main.go", got[1].FilePath)
}

func TestFilterCommentsByFileFocus_KeepsHighSeverityDocs(t *testing.T) {
	comments := []core.FileComment{
		{FilePath: "README.md", Line: 10, Severity: "HIGH", Message: "Deployment command is unsafe and can delete data."},
	}
	got := filterCommentsByFileFocus(comments)
	assert.Len(t, got, 1)
}

func TestAggregateCommentsByHunk_MergesLinesInSameHunk(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "api/handler.go",
			Hunks: []diffparse.Hunk{
				{
					NewStart: 40,
					NewLines: 8,
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineAdded, NewLineNo: 42},
						{Type: diffparse.LineAdded, NewLineNo: 45},
					},
				},
			},
		},
	}
	pos := collectValidPositions(changes)
	comments := []core.FileComment{
		{FilePath: "api/handler.go", Line: 42, Severity: "HIGH", Message: "Nil check missing."},
		{FilePath: "api/handler.go", Line: 45, Severity: "MEDIUM", Message: "Error context weak."},
	}
	got, unplaced := aggregateCommentsByHunk(comments, pos)
	assert.Empty(t, unplaced)
	assert.Len(t, got, 1)
	assert.Equal(t, "HIGH", got[0].Severity)
	assert.Contains(t, got[0].Message, "Hunk new lines 40-47")
	assert.Contains(t, got[0].Message, "Nil check missing.")
	assert.Contains(t, got[0].Message, "Error context weak.")
}

func TestAggregateCommentsByLine_Fallback(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "api/handler.go",
			Hunks: []diffparse.Hunk{
				{NewStart: 10, NewLines: 1, Lines: []diffparse.DiffLine{{Type: diffparse.LineAdded, NewLineNo: 10}}},
			},
		},
	}
	pos := collectValidPositions(changes)
	comments := []core.FileComment{
		{FilePath: "api/handler.go", Line: 10, Severity: "MEDIUM", Message: "Potential panic when request is nil."},
	}
	got, unplaced := aggregateCommentsByLine(comments, pos)
	assert.Empty(t, unplaced)
	assert.Len(t, got, 1)
	assert.Equal(t, 10, got[0].NewLine)
}

func TestAggregateCommentsByHunk_FallbackWhenLineMissing(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "public/index.php",
			Hunks: []diffparse.Hunk{
				{
					NewStart: 25,
					NewLines: 4,
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineAdded, NewLineNo: 26},
						{Type: diffparse.LineAdded, NewLineNo: 28},
					},
				},
			},
		},
	}
	pos := collectValidPositions(changes)
	comments := []core.FileComment{
		{FilePath: "public/index.php", Line: 0, Severity: "HIGH", Message: "General file-level risk needs line anchoring."},
	}

	got, unplaced := aggregateCommentsByHunk(comments, pos)
	assert.Empty(t, unplaced)
	assert.Len(t, got, 1)
	assert.Equal(t, 26, got[0].NewLine)
	assert.Contains(t, got[0].Message, "General file-level risk")
}

func TestBuildInlineCommentBody_SeparatesSuggestionBlock(t *testing.T) {
	body := buildInlineCommentBody(
		"HIGH",
		"Key points:\n- Missing nil check in handler.\n- Error context is weak.",
		"if h == nil {\n\treturn err\n}",
		func(s string) string { return "```suggestion\n" + s + "\n```" },
	)
	assert.Contains(t, body, "[HIGH] Missing nil check in handler.")
	assert.Contains(t, body, "Suggested patch:")
	assert.Contains(t, body, "```suggestion")
}

func TestBuildInlineCommentBody_StripsCodeFenceFromMessage(t *testing.T) {
	body := buildInlineCommentBody(
		"MEDIUM",
		"Key points:\n- First issue.\n```go\nfmt.Println(\"noise\")\n```\n- Second issue.",
		"",
		nil,
	)
	assert.Contains(t, body, "[MEDIUM] First issue.")
	assert.NotContains(t, body, "fmt.Println")
	assert.NotContains(t, body, "```")
}

func TestBuildInlineCommentBody_SkipsNonActionableLeadPoints(t *testing.T) {
	body := buildInlineCommentBody(
		"HIGH",
		"Hunk new lines 60-66\nKey points:\n- Remediation Plan\n- Missing null-check before json_encode.",
		"",
		nil,
	)
	assert.Contains(t, body, "[HIGH] Missing null-check before json_encode.")
	assert.NotContains(t, body, "Hunk new lines")
	assert.NotContains(t, body, "Remediation Plan")
}

func TestBuildInlineCommentBody_PreservesSuggestionPadding(t *testing.T) {
	body := buildInlineCommentBody(
		"HIGH",
		"Key points:\n- Keep original indentation.",
		"\n\n    $value = trim($value);\n\treturn $value;\n",
		func(s string) string { return "```suggestion\n" + s + "\n```" },
	)
	assert.Contains(t, body, "```suggestion\n    $value = trim($value);\n\treturn $value;\n```")
}

func TestDetectGitLabMCPStatus_FromEnv(t *testing.T) {
	got := detectGitLabMCPStatus(
		func(string) (string, error) { return "", fmt.Errorf("not found") },
		func(k string) string {
			if k == "GITLAB_MCP_URL" {
				return "http://mcp.local"
			}
			return ""
		},
	)
	assert.Contains(t, got, "configured via GITLAB_MCP_URL")
}

func TestDetectGitLabMCPStatus_FromBinary(t *testing.T) {
	got := detectGitLabMCPStatus(
		func(bin string) (string, error) {
			if bin == "gitlab-mcp" {
				return "/usr/local/bin/gitlab-mcp", nil
			}
			return "", fmt.Errorf("not found")
		},
		func(string) string { return "" },
	)
	assert.Contains(t, got, "detected local server binary")
}

func TestDetectGitLabMCPStatus_Fallback(t *testing.T) {
	got := detectGitLabMCPStatus(
		func(string) (string, error) { return "", fmt.Errorf("not found") },
		func(string) string { return "" },
	)
	assert.Contains(t, got, "not detected/configured")
}

func TestHasTopLevelMarker(t *testing.T) {
	notes := []vcs.MRNote{
		{Body: "hello"},
		{Body: "<!-- prev:summary -->\nsummary"},
	}
	assert.True(t, hasTopLevelMarker(notes, "<!-- prev:summary -->"))
	assert.False(t, hasTopLevelMarker(notes, "<!-- prev:reply -->"))
}

func TestExistingInlineSeverityKeys(t *testing.T) {
	discussions := []vcs.MRDiscussion{
		{
			Notes: []vcs.MRDiscussionNote{
				{FilePath: "a/b.go", Line: 10, Body: "[HIGH] Risky change"},
				{FilePath: "a/b.go", Line: 11, Body: "No severity prefix"},
			},
		},
	}
	keys := existingInlineSeverityKeys(discussions)
	_, okHigh := keys[inlineSeverityKey("a/b.go", 10, "HIGH")]
	_, okNoSev := keys[inlineSeverityKey("a/b.go", 11, "MEDIUM")]
	assert.True(t, okHigh)
	assert.False(t, okNoSev)
}

func TestCollectReusableThreads_FiltersPausedAndResolved(t *testing.T) {
	discussions := []vcs.MRDiscussion{
		{
			ID: "d1",
			Notes: []vcs.MRDiscussionNote{
				{Author: "bot", Body: "<!-- prev:thread -->\n[HIGH] Nil guard missing", FilePath: "a.go", Line: 10, Resolvable: true},
			},
		},
		{
			ID: "d2",
			Notes: []vcs.MRDiscussionNote{
				{Author: "bot", Body: "<!-- prev:thread -->\n[HIGH] Ignored", FilePath: "a.go", Line: 20, Resolvable: true},
			},
		},
		{
			ID: "d3",
			Notes: []vcs.MRDiscussionNote{
				{Author: "bot", Body: "<!-- prev:thread -->\n[HIGH] Already resolved", FilePath: "a.go", Line: 30, Resolvable: true, Resolved: true},
			},
		},
	}
	paused := map[string]bool{"d2": true}
	got := collectReusableThreads(discussions, "bot", paused)
	assert.Len(t, got, 1)
	assert.Equal(t, "d1", got[0].DiscussionID)
	assert.Equal(t, "HIGH", got[0].Severity)
}

func TestMatchReusableThread_SameFileSeverityAndSimilarMessage(t *testing.T) {
	candidates := []reusableThread{
		{
			DiscussionID: "d1",
			FilePath:     "api/handler.go",
			Line:         42,
			Severity:     "HIGH",
			Message:      "Missing nil check before request dereference",
		},
	}
	grp := inlineGroup{
		FilePath: "api/handler.go",
		NewLine:  45,
		Severity: "HIGH",
		Message:  "request dereference can panic when nil check is missing",
	}
	got, ok := matchReusableThread(candidates, grp)
	assert.True(t, ok)
	assert.Equal(t, "d1", got.DiscussionID)
}

func TestMatchReusableThread_RejectsWeakSimilarity(t *testing.T) {
	candidates := []reusableThread{
		{
			DiscussionID: "d1",
			FilePath:     "api/handler.go",
			Line:         42,
			Severity:     "HIGH",
			Message:      "Missing nil check before request dereference",
		},
	}
	grp := inlineGroup{
		FilePath: "api/handler.go",
		NewLine:  45,
		Severity: "HIGH",
		Message:  "Rename variable for readability",
	}
	_, ok := matchReusableThread(candidates, grp)
	assert.False(t, ok)
}

func TestBuildReReviewPrompt(t *testing.T) {
	prompt := buildReReviewPrompt("BASE_PROMPT", "OLD_REVIEW", 2, 3)
	assert.Contains(t, prompt, "review pass 2/3")
	assert.Contains(t, prompt, "Original review context")
	assert.Contains(t, prompt, "BASE_PROMPT")
	assert.Contains(t, prompt, "Prior pass output")
	assert.Contains(t, prompt, "OLD_REVIEW")
	assert.Contains(t, prompt, "complete final review")
}

func TestResolveMRIntSetting_Precedence(t *testing.T) {
	v := config.NewStore()
	v.Set("review.nitpick", 3)
	conf := config.Config{Viper: v}
	cmd := &cobra.Command{Use: "x"}
	cmd.Flags().Int("nitpick", 0, "")
	assert.NoError(t, cmd.Flags().Set("nitpick", "7"))
	got := resolveMRIntSetting(cmd, "nitpick", conf, []string{"review.nitpick"}, 1)
	assert.Equal(t, 7, got)
}

func TestResolveMRIntSetting_FromConfig(t *testing.T) {
	v := config.NewStore()
	v.Set("review.nitpick", 4)
	conf := config.Config{Viper: v}
	cmd := &cobra.Command{Use: "x"}
	cmd.Flags().Int("nitpick", 0, "")
	got := resolveMRIntSetting(cmd, "nitpick", conf, []string{"review.nitpick"}, 1)
	assert.Equal(t, 4, got)
}

func TestResolveMRStringSetting_FromConfig(t *testing.T) {
	v := config.NewStore()
	v.Set("review.strictness", "lenient")
	conf := config.Config{Viper: v}
	cmd := &cobra.Command{Use: "x"}
	cmd.Flags().String("strictness", "", "")
	got := resolveMRStringSetting(cmd, "strictness", conf, []string{"review.strictness", "strictness"}, "normal")
	assert.Equal(t, "lenient", got)
}

func TestFilterInlineCandidates_FallsBackToChangedFiles(t *testing.T) {
	parsed := []core.FileComment{
		{FilePath: "./public/index.php", Line: 10, Kind: "ISSUE", Severity: "MEDIUM", Message: "Changed-file finding"},
		{FilePath: "README.md", Line: 3, Kind: "ISSUE", Severity: "MEDIUM", Message: "Doc finding"},
	}
	valid := map[string]inlinePositions{
		"public/index.php": {
			oldByNew: map[int]int{10: 0},
			added:    map[int]struct{}{10: {}},
			hunks:    []hunkRange{{start: 10, end: 10}},
		},
	}

	got, fallback := filterInlineCandidates(parsed, "strict", 3, []string{"issue", "suggestion", "remark"}, valid, "diff_context")
	assert.True(t, fallback)
	if assert.Len(t, got, 1) {
		assert.Equal(t, "./public/index.php", got[0].FilePath)
	}
}

func TestFilterInlineCandidates_NoFallbackWhenFiltersKeepFindings(t *testing.T) {
	parsed := []core.FileComment{
		{FilePath: "public/index.php", Line: 10, Kind: "ISSUE", Severity: "HIGH", Message: "High finding"},
	}
	valid := map[string]inlinePositions{
		"public/index.php": {
			oldByNew: map[int]int{10: 0},
			added:    map[int]struct{}{10: {}},
			hunks:    []hunkRange{{start: 10, end: 10}},
		},
	}

	got, fallback := filterInlineCandidates(parsed, "strict", 3, []string{"issue", "suggestion", "remark"}, valid, "diff_context")
	assert.False(t, fallback)
	if assert.Len(t, got, 1) {
		assert.Equal(t, "HIGH", got[0].Severity)
	}
}

func TestFilterInlineCandidates_FilterModeAdded(t *testing.T) {
	parsed := []core.FileComment{
		{FilePath: "public/index.php", Line: 10, Kind: "ISSUE", Severity: "HIGH", Message: "Added-line finding"},
		{FilePath: "public/index.php", Line: 11, Kind: "ISSUE", Severity: "HIGH", Message: "Context-line finding"},
	}
	valid := map[string]inlinePositions{
		"public/index.php": {
			oldByNew: map[int]int{10: 0, 11: 11},
			added:    map[int]struct{}{10: {}},
			hunks:    []hunkRange{{start: 10, end: 11}},
		},
	}

	got, fallback := filterInlineCandidates(parsed, "strict", 3, []string{"issue", "suggestion", "remark"}, valid, "added")
	assert.False(t, fallback)
	if assert.Len(t, got, 1) {
		assert.Equal(t, 10, got[0].Line)
	}
}

func TestFilterInlineCandidates_FilterModeFallbackWhenEmpty(t *testing.T) {
	parsed := []core.FileComment{
		{FilePath: "public/index.php", Line: 11, Kind: "ISSUE", Severity: "HIGH", Message: "Context-line finding"},
	}
	valid := map[string]inlinePositions{
		"public/index.php": {
			oldByNew: map[int]int{11: 11},
			added:    map[int]struct{}{},
			hunks:    []hunkRange{{start: 11, end: 11}},
		},
	}

	got, fallback := filterInlineCandidates(parsed, "strict", 3, []string{"issue", "suggestion", "remark"}, valid, "added")
	assert.True(t, fallback)
	if assert.Len(t, got, 1) {
		assert.Equal(t, 11, got[0].Line)
	}
}

func TestLatestReviewBaseline_ParsesMarker(t *testing.T) {
	payload, err := json.Marshal(reviewBaseline{
		HeadSHA:  "abc123",
		FileSigs: map[string]string{"public/index.php": "sig1"},
	})
	assert.NoError(t, err)
	encoded := base64.StdEncoding.EncodeToString(payload)
	notes := []vcs.MRNote{
		{Body: "some other note"},
		{Body: prevBaselinePrefix + encoded + " -->"},
	}
	baseline, ok := latestReviewBaseline(notes)
	assert.True(t, ok)
	assert.Equal(t, "abc123", baseline.HeadSHA)
	assert.Equal(t, "sig1", baseline.FileSigs["public/index.php"])
}

func TestFilterChangesByBaseline_OnlyChangedSignaturesRemain(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "a.php",
			Hunks: []diffparse.Hunk{
				{NewStart: 1, NewLines: 1, Lines: []diffparse.DiffLine{{Type: diffparse.LineAdded, NewLineNo: 1, Content: "a"}}},
			},
		},
		{
			NewName: "b.php",
			Hunks: []diffparse.Hunk{
				{NewStart: 2, NewLines: 1, Lines: []diffparse.DiffLine{{Type: diffparse.LineAdded, NewLineNo: 2, Content: "b"}}},
			},
		},
	}
	sigs := buildFileSignatures(changes)
	baseline := map[string]string{
		"a.php": sigs["a.php"],
		"b.php": "different",
	}
	got := filterChangesByBaseline(changes, baseline)
	if assert.Len(t, got, 1) {
		assert.Equal(t, "b.php", got[0].NewName)
	}
}

func TestParseReviewContent_StructuredFallbackToMarkdown(t *testing.T) {
	markdown := "**File: api/handler.go** (line 42) [ISSUE] [HIGH]: Missing nil check."
	parsed := parseReviewContent(markdown, true)
	if assert.Len(t, parsed.FileComments, 1) {
		assert.Equal(t, "api/handler.go", parsed.FileComments[0].FilePath)
	}
}

func TestExtractKeyPoints_SkipsBulletKeyPointsHeading(t *testing.T) {
	msg := "Hunk new lines 1-10\nKey points:\n- Key points:\n- Real actionable issue."
	points := extractKeyPoints(msg)
	assert.NotContains(t, points, "Key points:")
	assert.Contains(t, points, "Real actionable issue.")
}

func TestDetectDeterministicFindings_JsonDencode(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "public/index.php",
			Hunks: []diffparse.Hunk{
				{
					NewStart: 12,
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineAdded, NewLineNo: 14, Content: "echo json_dencode($payload);"},
					},
				},
			},
		},
	}
	got := detectDeterministicFindings(changes)
	if assert.Len(t, got, 1) {
		assert.Equal(t, "public/index.php", got[0].FilePath)
		assert.Equal(t, 14, got[0].Line)
		assert.Equal(t, "HIGH", got[0].Severity)
		assert.Contains(t, got[0].Message, "json_dencode")
	}
}

func TestFilterOutMetaContextFindings(t *testing.T) {
	in := []core.FileComment{
		{
			FilePath: "public/index.php",
			Line:     5,
			Kind:     "ISSUE",
			Severity: "CRITICAL",
			Message:  "Modified hunk content is not provided, preventing validation.",
		},
		{
			FilePath: "public/index.php",
			Line:     10,
			Kind:     "ISSUE",
			Severity: "HIGH",
			Message:  "json_dencode typo causes undefined function.",
		},
	}
	got := filterOutMetaContextFindings(in)
	if assert.Len(t, got, 1) {
		assert.Equal(t, 10, got[0].Line)
	}
}

func TestFilterLowSignalInlineFindings_DropsGenericKeepsSpecific(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "public/index.php",
			Hunks: []diffparse.Hunk{
				{
					NewStart: 58,
					NewLines: 4,
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineContext, NewLineNo: 58, Content: "'summary' => (string) ($payload['summary'] ?? ''),"},
						{Type: diffparse.LineAdded, NewLineNo: 59, Content: "'category' => (string) ($paylo['category'] ?? 'general'),"},
						{Type: diffparse.LineContext, NewLineNo: 60, Content: "'tags' => (array) ($payload['tags'] ?? []),"},
					},
				},
			},
		},
	}
	valid := collectValidPositions(changes)
	in := []core.FileComment{
		{
			FilePath: "public/index.php",
			Line:     59,
			Kind:     "ISSUE",
			Severity: "HIGH",
			Message:  "Changes in the main entry point may affect global request handling; ensure backward compatibility.",
		},
		{
			FilePath: "public/index.php",
			Line:     59,
			Kind:     "ISSUE",
			Severity: "HIGH",
			Message:  "Typo `$paylo` should be `$payload`; this breaks category extraction.",
		},
	}

	got := filterLowSignalInlineFindings(in, valid)
	if assert.Len(t, got, 1) {
		assert.Contains(t, got[0].Message, "$paylo")
	}
}

func TestExtractHunkContext_NoAnchorFallsBackToRepresentativeHunk(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "public/index.php",
			Hunks: []diffparse.Hunk{
				{
					NewStart: 30,
					NewLines: 2,
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineAdded, NewLineNo: 30, Content: "$title = trim($payload['title'] ?? '');"},
						{Type: diffparse.LineAdded, NewLineNo: 31, Content: "$summary = trim($payload['summary'] ?? '');"},
					},
				},
			},
		},
	}

	got := extractHunkContext(changes, "", 0)
	assert.Contains(t, got, "Thread has no inline anchor; using representative MR hunk")
	assert.Contains(t, got, "public/index.php:30")
	assert.Contains(t, got, "+ 30 $title")
}

func TestHasAnyModifiedLines(t *testing.T) {
	noMods := []diffparse.FileChange{
		{
			NewName: "README.md",
			Hunks: []diffparse.Hunk{
				{
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineContext, NewLineNo: 1, Content: "same"},
					},
				},
			},
		},
	}
	assert.False(t, hasAnyModifiedLines(noMods))

	withMods := []diffparse.FileChange{
		{
			NewName: "public/index.php",
			Hunks: []diffparse.Hunk{
				{
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineAdded, NewLineNo: 3, Content: "echo 1;"},
					},
				},
			},
		},
	}
	assert.True(t, hasAnyModifiedLines(withMods))
}
