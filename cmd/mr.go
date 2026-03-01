package cmd

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/sanix-darker/prev/internal/core"
	"github.com/sanix-darker/prev/internal/diffparse"
	"github.com/sanix-darker/prev/internal/handlers"
	"github.com/sanix-darker/prev/internal/provider"
	"github.com/sanix-darker/prev/internal/renders"
	"github.com/sanix-darker/prev/internal/serena"
	"github.com/sanix-darker/prev/internal/vcs"
	"github.com/spf13/cobra"
)

const (
	prevThreadMarker    = "<!-- prev:thread -->"
	prevCarryOverMarker = "<!-- prev:carry-over -->"
	prevReplyMarker     = "<!-- prev:reply -->"
	prevSummaryMarker   = "<!-- prev:summary -->"
	prevReuseMarker     = "<!-- prev:reuse -->"
	prevBaselinePrefix  = "<!-- prev:baseline "
	prevMentionHandle   = "prev"
)

func init() {
	mrCmd := &cobra.Command{
		Use:   "mr",
		Short: "Merge/Pull Request operations",
	}

	mrCmd.AddCommand(newMRReviewCmd())
	mrCmd.AddCommand(newMRDiffCmd())
	mrCmd.AddCommand(newMRListCmd())
	rootCmd.AddCommand(mrCmd)
}

func resolveVCSProvider(cmd *cobra.Command) (vcs.VCSProvider, error) {
	vcsName, _ := cmd.Flags().GetString("vcs")
	if vcsName == "" {
		// Auto-detect from env vars
		if os.Getenv("GITLAB_TOKEN") != "" {
			vcsName = "gitlab"
		} else if os.Getenv("GITHUB_TOKEN") != "" {
			vcsName = "github"
		} else {
			vcsName = "gitlab"
		}
	}

	token, _ := cmd.Flags().GetString("gitlab-token")
	baseURL, _ := cmd.Flags().GetString("gitlab-url")

	// Fall back to env vars
	if token == "" {
		switch vcsName {
		case "gitlab":
			token = os.Getenv("GITLAB_TOKEN")
		case "github":
			token = os.Getenv("GITHUB_TOKEN")
		}
	}
	if baseURL == "" {
		switch vcsName {
		case "gitlab":
			baseURL = os.Getenv("GITLAB_URL")
		}
	}

	return vcs.Get(vcsName, token, baseURL)
}

func newMRReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "review <project_id> <mr_iid>",
		Short:   "Review a Merge Request using AI",
		Example: "prev mr review my-group/my-project 42\nprev mr review my-group/my-project 42 --dry-run --provider anthropic",
		Args:    cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			conf := config.NewDefaultConfig()
			applyFlags(cmd, &conf)

			projectID := args[0]
			mrIID, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: invalid MR IID %q: %v\n", args[1], err)
				os.Exit(1)
			}

			dryRun, _ := cmd.Flags().GetBool("dry-run")
			summaryOnly, _ := cmd.Flags().GetBool("summary-only")
			strictness := resolveMRStringSetting(
				cmd, "strictness", conf,
				[]string{"review.strictness", "strictness"},
				conf.Strictness,
			)
			nitpick := resolveMRIntSetting(
				cmd, "nitpick", conf,
				[]string{"review.nitpick"},
				0,
			)
			nitpick = normalizeNitpickFromStrictness(nitpick, strictness)
			maxComments := resolveMRIntSetting(
				cmd, "max-comments", conf,
				[]string{"review.max_comments"},
				0,
			)
			if maxComments < 0 {
				maxComments = 0
			}
			reviewPasses := resolveMRIntSetting(
				cmd, "review-passes", conf,
				[]string{"review.passes"},
				0,
			)
			if reviewPasses <= 0 {
				reviewPasses = 1
			}
			if reviewPasses > 6 {
				reviewPasses = 6
			}
			incremental := false
			if conf.Viper != nil {
				incremental = conf.Viper.GetBool("review.incremental")
			}
			if f := cmd.Flags().Lookup("incremental"); f != nil && f.Changed {
				incremental, _ = cmd.Flags().GetBool("incremental")
			}
			filterMode := resolveMRStringSetting(
				cmd, "filter-mode", conf,
				[]string{"review.filter_mode"},
				"diff_context",
			)
			filterMode = normalizeInlineFilterMode(filterMode)
			memoryEnabled := true
			if f := cmd.Flags().Lookup("memory"); f != nil && f.Changed {
				memoryEnabled, _ = cmd.Flags().GetBool("memory")
			}
			memoryFile, _ := cmd.Flags().GetString("memory-file")
			memoryMax := 12
			if f := cmd.Flags().Lookup("memory-max"); f != nil && f.Changed {
				memoryMax, _ = cmd.Flags().GetInt("memory-max")
			}
			if memoryMax <= 0 {
				memoryMax = 12
			}
			nativeImpact := true
			if f := cmd.Flags().Lookup("native-impact"); f != nil && f.Changed {
				nativeImpact, _ = cmd.Flags().GetBool("native-impact")
			}
			nativeImpactMaxSymbols := 12
			if f := cmd.Flags().Lookup("native-impact-max-symbols"); f != nil && f.Changed {
				nativeImpactMaxSymbols, _ = cmd.Flags().GetInt("native-impact-max-symbols")
			}
			fixPromptMode := "off"
			if f := cmd.Flags().Lookup("fix-prompt"); f != nil && f.Changed {
				fixPromptMode, _ = cmd.Flags().GetString("fix-prompt")
			}
			fixPromptMode = normalizeFixPromptMode(fixPromptMode)
			structuredOutput := false
			if conf.Viper != nil {
				structuredOutput = conf.Viper.GetBool("review.structured_output")
			}
			if f := cmd.Flags().Lookup("structured-output"); f != nil && f.Changed {
				structuredOutput, _ = cmd.Flags().GetBool("structured-output")
			}
			inlineOnly := false
			if conf.Viper != nil {
				inlineOnly = conf.Viper.GetBool("review.inline_only")
			}
			if f := cmd.Flags().Lookup("inline-only"); f != nil && f.Changed {
				inlineOnly, _ = cmd.Flags().GetBool("inline-only")
			}
			if inlineOnly && incremental {
				fmt.Println("Incremental mode disabled in inline-only mode (baseline markers require non-inline MR notes).")
				incremental = false
			}
			reviewGuidelines := ""
			if conf.Viper != nil {
				reviewGuidelines = strings.TrimSpace(conf.Viper.GetString("review.guidelines"))
			}
			reviewGuidelines = mergeGuidelines(
				reviewGuidelines,
				repoGuidelineSection(guidelineRootForMR()),
			)
			mrDiffSource := resolveMRStringSetting(
				cmd, "mr-diff-source", conf,
				[]string{"review.mr_diff_source"},
				"auto",
			)
			repoPath := resolveMRRepoPath()
			conventions := conf.Viper.GetStringSlice("review.conventions.labels")
			if len(conventions) == 0 {
				conventions = []string{"issue", "suggestion", "remark"}
			}

			vcsProvider, err := resolveVCSProvider(cmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			review, err := handlers.ExtractMRHandlerWithOptions(
				vcsProvider, projectID, mrIID, strictness,
				handlers.MRExtractOptions{
					DiffSource: mrDiffSource,
					RepoPath:   repoPath,
				},
			)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(detectVCSContextStatus(vcsProvider.Info().Name, exec.LookPath, os.Getenv))
			mentionHandle := resolveMentionHandle(conf)

			discussions, err := vcsProvider.ListMRDiscussions(projectID, mrIID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to fetch MR discussions: %v\n", err)
			}
			notes, err := vcsProvider.ListMRNotes(projectID, mrIID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to fetch MR notes: %v\n", err)
			}
			if isMRPaused(notes, mentionHandle) {
				fmt.Printf("Review paused for MR !%d via @%s pause. Add @%s resume in MR comments to continue.\n",
					mrIID, mentionHandle, mentionHandle)
				return
			}

			currentSignatures := buildFileSignatures(review.Changes)
			if incremental {
				if baseline, ok := latestReviewBaseline(notes); ok && len(baseline.FileSigs) > 0 {
					filtered := filterChangesByBaseline(review.Changes, baseline.FileSigs)
					if len(filtered) == 0 {
						fmt.Printf("Incremental review: no file-level deltas since baseline head %s.\n", baseline.HeadSHA)
						return
					}
					if len(filtered) < len(review.Changes) {
						fmt.Printf("Incremental review: narrowed scope from %d to %d changed files since baseline head %s.\n",
							len(review.Changes), len(filtered), baseline.HeadSHA)
					}
					review.Changes = filtered
					currentSignatures = buildFileSignatures(review.Changes)
				}
			}
			if !hasAnyModifiedLines(review.Changes) {
				fmt.Fprintf(os.Stderr, "Error: insufficient MR diff context: no added/deleted hunk lines were extracted (source=%s). Try --mr-diff-source git or raw.\n", mrDiffSource)
				os.Exit(1)
			}
			validPositionsByFile := collectValidPositions(review.Changes)
			pausedThreads := pausedDiscussions(discussions, mentionHandle)

			carryOver := collectCarryOverFindings(discussions, validPositionsByFile, mentionHandle, pausedThreads)
			if len(carryOver) > 0 {
				reviewGuidelines = appendCarryOverGuidelines(reviewGuidelines, carryOver)
			}
			memoryPath := ""
			var mem reviewMemory
			if memoryEnabled {
				memLoaded, path, merr := loadReviewMemory(repoPath, memoryFile)
				if merr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to load review memory: %v\n", merr)
				} else {
					mem = memLoaded
					memoryPath = path
					reviewGuidelines = appendReviewMemoryGuidelines(reviewGuidelines, mem, review.Changes, memoryMax)
				}
			}
			reviewGuidelines = appendNativeImpactGuidelines(
				reviewGuidelines,
				review.Changes,
				repoPath,
				nativeImpact,
				nativeImpactMaxSymbols,
			)

			serenaMode := resolveMRStringSetting(
				cmd, "serena", conf,
				[]string{"review.serena_mode", "serena_mode"},
				"auto",
			)
			contextLines := resolveMRIntSetting(
				cmd, "context", conf,
				[]string{"review.context_lines"},
				10,
			)
			maxTokens := resolveMRIntSetting(
				cmd, "max-tokens", conf,
				[]string{"review.max_tokens"},
				80000,
			)
			fmt.Printf("Review settings: strictness=%s nitpick=%d max_comments=%d passes=%d inline_only=%t incremental=%t filter_mode=%s structured_output=%t mr_diff_source=%s serena=%s context=%d max_tokens=%d\n",
				strictness, nitpick, maxComments, reviewPasses, inlineOnly, incremental, filterMode, structuredOutput, mrDiffSource, serenaMode, contextLines, maxTokens)
			formattedDiffs, err := buildMRFormattedDiffs(review, serenaMode, contextLines, maxTokens)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			review.Prompt = core.BuildMRReviewPromptWithOptions(
				review.MR.Title,
				review.MR.Description,
				review.MR.SourceBranch,
				review.MR.TargetBranch,
				formattedDiffs,
				strictness,
				nitpick,
				conventions,
				reviewGuidelines,
			)
			review.Prompt = appendLineAnchorInstructions(review.Prompt)
			if structuredOutput {
				review.Prompt = appendStructuredOutputInstructions(review.Prompt)
			}

			fmt.Printf("Reviewing MR !%d: %s (%s -> %s)\n",
				review.MR.IID, review.MR.Title,
				review.MR.SourceBranch, review.MR.TargetBranch)
			fmt.Printf("Files changed: %d\n\n", len(review.Changes))

			if dryRun {
				runReviewPassesDryRun(conf, review.Prompt, reviewPasses)
				return
			}

			// Get AI review via blocking call
			p, err := resolveProvider(conf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error resolving provider: %v\n", err)
				os.Exit(1)
			}
			info := p.Info()
			model := resolvedModelForLog(conf, info.DefaultModel)
			fmt.Printf("Model: provider=%s model=%s\n", info.Name, model)

			if !inlineOnly {
				replyCount := processReplyCommands(
					vcsProvider,
					p,
					projectID,
					mrIID,
					discussions,
					review.Changes,
					mentionHandle,
					pausedThreads,
				)
				if replyCount > 0 {
					fmt.Printf("Posted %d thread replies.\n", replyCount)
				}
				noteReplyCount := processNoteReplyCommands(
					vcsProvider,
					p,
					projectID,
					mrIID,
					notes,
					review.MR,
					validPositionsByFile,
					mentionHandle,
				)
				if noteReplyCount > 0 {
					fmt.Printf("Posted %d top-level replies.\n", noteReplyCount)
				}
			}

			reviewContent, err := runReviewPasses(p, review.Prompt, reviewPasses)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error from AI provider: %v\n", err)
				os.Exit(1)
			}
			fmt.Print(renders.RenderMarkdown(reviewContent))

			// Post to VCS
			parsed := parseReviewContent(reviewContent, structuredOutput)
			if len(parsed.FileComments) == 0 {
				recovered, rerr := recoverInlineFindings(p, review.Prompt, reviewContent)
				if rerr != nil {
					fmt.Fprintf(os.Stderr, "Warning: inline findings recovery failed: %v\n", rerr)
				} else {
					reparsed := parseReviewContent(recovered, structuredOutput)
					if len(reparsed.FileComments) > 0 {
						fmt.Printf("Inline findings recovery: extracted %d findings.\n", len(reparsed.FileComments))
						parsed.FileComments = reparsed.FileComments
					}
				}
			}
			parsed.FileComments = append(parsed.FileComments, detectDeterministicFindings(review.Changes)...)
			parsed.FileComments = filterOutMetaContextFindings(parsed.FileComments)
			parsed.FileComments = filterLowSignalInlineFindings(parsed.FileComments, validPositionsByFile)
			if memoryEnabled && strings.TrimSpace(memoryPath) != "" {
				now := time.Now().UTC()
				mrRef := fmt.Sprintf("%s!%d", projectID, mrIID)
				updated := false
				if updateReviewMemoryFromDiscussions(&mem, discussions, mrRef, now) {
					updated = true
				}
				if updateReviewMemoryFromFindings(&mem, parsed.FileComments, mrRef, now) {
					updated = true
				}
				if updated {
					trimReviewMemory(&mem, 500)
					if err := saveReviewMemory(memoryPath, mem); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to persist review memory: %v\n", err)
					} else {
						openCount, fixedCount := reviewMemoryCounts(mem)
						fmt.Printf("Review memory updated: %s (open=%d fixed=%d)\n", memoryPath, openCount, fixedCount)
					}
				}
			}
			if !inlineOnly && threadHasAnyCommand(discussions, mentionHandle, "summary") {
				if hasTopLevelMarker(notes, prevSummaryMarker) {
					fmt.Println("\nSummary already posted; skipping duplicate summary note.")
				} else {
					summaryBody := fmt.Sprintf("%s\n## AI Code Review\n\n%s", prevSummaryMarker, reviewContent)
					if err := vcsProvider.PostSummaryNote(projectID, mrIID, summaryBody); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to post summary note: %v\n", err)
					} else {
						fmt.Println("\nPosted summary comment to MR.")
					}
				}
			} else {
				if inlineOnly {
					fmt.Println("\nSummary skipped (inline-only mode).")
				} else {
					fmt.Println("\nSummary skipped (no explicit @mention summary request).")
				}
			}

			// Post inline comments (if not summary-only)
			if !summaryOnly && review.MR.DiffRefs.BaseSHA != "" {
				if !inlineOnly {
					carryPosted := postCarryOverReminders(vcsProvider, projectID, mrIID, discussions, carryOver, pausedThreads)
					if carryPosted > 0 {
						fmt.Printf("Posted %d carry-over reminders.\n", carryPosted)
					}
				}

				existingInline := existingInlineKeys(discussions)
				existingSeverity := existingInlineSeverityKeys(discussions)
				reusableThreads := collectReusableThreads(discussions, mentionHandle, pausedThreads)
				postedInlineKeys := make(map[string]struct{})
				reusedDiscussionIDs := make(map[string]struct{})
				rawComments, usedFilterFallback := filterInlineCandidates(
					parsed.FileComments,
					strictness,
					nitpick,
					conventions,
					validPositionsByFile,
					filterMode,
				)
				if usedFilterFallback {
					fmt.Println("Inline filter fallback: severity/kind filtering removed all findings; using parsed findings scoped to changed files.")
				}
				fileComments := filterCommentsByFileFocus(rawComments)
				if len(fileComments) == 0 && len(rawComments) > 0 {
					fmt.Println("Inline filter fallback: typo-only doc filter removed all findings; using broader findings.")
					fileComments = rawComments
				}
				fileComments = aggregateCommentsByChange(fileComments)
				inlineGroups, unplaced := aggregateCommentsByLine(fileComments, validPositionsByFile)
				if len(inlineGroups) == 0 && len(fileComments) > 0 {
					fallbackGroups, fallbackUnplaced := aggregateCommentsByHunk(fileComments, validPositionsByFile)
					if len(fallbackGroups) > 0 {
						fmt.Println("Inline placement fallback: line-level grouping produced no placeable comments; using hunk-level grouping.")
						inlineGroups = fallbackGroups
					}
					if len(fallbackUnplaced) > 0 {
						unplaced = append(unplaced, fallbackUnplaced...)
					}
				}
				fmt.Printf("Inline findings pipeline: parsed=%d filtered=%d focused=%d grouped=%d\n",
					len(parsed.FileComments), len(rawComments), len(fileComments), len(inlineGroups))
				originalCount := len(inlineGroups)
				inlineGroups = prioritizeAndLimitInlineGroups(inlineGroups, maxComments)
				if maxComments > 0 && originalCount > len(inlineGroups) {
					fmt.Printf("Limiting inline comments to top %d by severity (from %d findings).\n", len(inlineGroups), originalCount)
				}
				postedInline := 0
				reusedInline := 0
				skippedExisting := 0
				skippedRunDup := 0
				for _, grp := range inlineGroups {
					anchorContent := validPositionsByFile[grp.FilePath].content[grp.NewLine]
					alignedSuggestion := rebaseSuggestionIndentation(grp.Suggestion, anchorContent)
					body := buildInlineCommentBody(grp.Severity, grp.Message, alignedSuggestion, vcsProvider.FormatSuggestionBlock)
					if fp := buildAgentFixPrompt(grp, fixPromptMode); fp != "" {
						body += "\n\nAI agent fix prompt:\n```text\n" + fp + "\n```"
					}
					body += "\n\n" + prevThreadMarker
					key := inlineKey(grp.FilePath, grp.NewLine, body)
					sevKey := inlineSeverityKey(grp.FilePath, grp.NewLine, grp.Severity)
					if _, ok := existingInline[key]; ok {
						skippedExisting++
						continue
					}
					if _, ok := existingSeverity[sevKey]; ok {
						skippedExisting++
						continue
					}
					if _, ok := postedInlineKeys[key]; ok {
						skippedRunDup++
						continue
					}
					if r, ok := matchReusableThread(reusableThreads, grp); ok {
						if _, used := reusedDiscussionIDs[r.DiscussionID]; !used {
							reply := fmt.Sprintf(
								"%s\nRevalidated on current diff near `%s:%d`.\n\n%s",
								prevReuseMarker, grp.FilePath, grp.NewLine, body,
							)
							if err := vcsProvider.ReplyToMRDiscussion(projectID, mrIID, r.DiscussionID, reply); err == nil {
								postedInline++
								reusedInline++
								reusedDiscussionIDs[r.DiscussionID] = struct{}{}
								postedInlineKeys[key] = struct{}{}
								existingSeverity[sevKey] = struct{}{}
								continue
							}
						}
					}
					err := vcsProvider.PostInlineComment(
						projectID, mrIID,
						review.MR.DiffRefs,
						vcs.InlineComment{
							FilePath: grp.FilePath,
							OldPath:  validPositionsByFile[grp.FilePath].oldPath,
							NewLine:  int64(grp.NewLine),
							OldLine:  int64(grp.OldLine),
							Body:     body,
						},
					)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to post inline comment on %s:%d: %v\n",
							grp.FilePath, grp.NewLine, err)
						continue
					}
					postedInline++
					postedInlineKeys[key] = struct{}{}
					existingSeverity[sevKey] = struct{}{}
				}
				if postedInline > 0 {
					fmt.Printf("Posted %d inline comments.\n", postedInline)
					if reusedInline > 0 {
						fmt.Printf("Reused %d existing discussions for continuity.\n", reusedInline)
					}
				} else if skippedExisting > 0 || skippedRunDup > 0 {
					fmt.Printf("No new inline comments to post (existing threads already cover %d findings).\n", skippedExisting)
				} else if len(inlineGroups) == 0 {
					fmt.Println("No inline findings generated by AI output.")
				} else if len(unplaced) >= len(fileComments) {
					fmt.Println("No inline comments posted (all findings were unplaced for current MR diff).")
				} else {
					fmt.Println("No inline comments were posted.")
				}
				if len(unplaced) > 0 && !inlineOnly {
					sort.Strings(unplaced)
					note := "## Unplaced Inline Findings\n\nGitLab rejected precise inline placement for these findings. They are kept here for visibility:\n\n" + strings.Join(unplaced, "\n")
					if err := vcsProvider.PostSummaryNote(projectID, mrIID, note); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to post unplaced findings note: %v\n", err)
					}
				}
			}

			if incremental {
				baseline := reviewBaseline{
					HeadSHA:  review.MR.DiffRefs.HeadSHA,
					FileSigs: currentSignatures,
				}
				if err := postReviewBaseline(vcsProvider, projectID, mrIID, baseline); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to post incremental baseline marker: %v\n", err)
				}
			}
		},
	}

	cmd.Flags().Bool("dry-run", false, "Print review without posting to VCS")
	cmd.Flags().Bool("summary-only", false, "Post only a summary comment, no inline comments")
	cmd.Flags().String("gitlab-token", "", "GitLab personal access token (or use GITLAB_TOKEN env)")
	cmd.Flags().String("gitlab-url", "", "GitLab instance URL (or use GITLAB_URL env, default: https://gitlab.com)")
	cmd.Flags().String("vcs", "", "VCS provider (gitlab, github; auto-detected from env)")
	cmd.Flags().String("strictness", "", "Review strictness: strict, normal, lenient (default: normal)")
	cmd.Flags().Int("max-comments", 0, "Maximum number of inline comments to post (0 = unlimited)")
	cmd.Flags().Int("review-passes", 0, "Number of AI review passes to run (0 = config/default 1)")
	cmd.Flags().Bool("inline-only", false, "Post inline comments only (disable summary notes, thread replies, and unplaced summary notes)")
	cmd.Flags().Bool("incremental", false, "Review only file-level deltas since the last baseline marker")
	cmd.Flags().String("filter-mode", "diff_context", "Inline filtering mode: added, diff_context, file, nofilter")
	cmd.Flags().Bool("memory", true, "Enable persistent cross-MR reviewer memory")
	cmd.Flags().String("memory-file", defaultReviewMemoryFile, "Path to persistent review memory markdown file")
	cmd.Flags().Int("memory-max", 12, "Maximum historical memory items injected into the review prompt")
	cmd.Flags().Bool("native-impact", true, "Enable native deterministic impact/risk precheck before AI review")
	cmd.Flags().Int("native-impact-max-symbols", 12, "Maximum changed symbols used for native impact mapping")
	cmd.Flags().String("fix-prompt", "off", "Include AI fix prompt block in inline comments: off, auto, always")
	cmd.Flags().Bool("structured-output", false, "Request and parse structured JSON findings with markdown fallback")
	cmd.Flags().String("mr-diff-source", "auto", "MR diff source strategy: auto, git, raw, api")
	cmd.Flags().String("serena", "auto", "Serena mode: auto, on, off")
	cmd.Flags().Int("context", 10, "Number of surrounding context lines for MR review context enrichment")
	cmd.Flags().Int("max-tokens", 80000, "Maximum token budget for MR context enrichment")
	return cmd
}

func normalizeNitpickFromStrictness(nitpick int, strictness string) int {
	if nitpick > 10 {
		nitpick = 10
	}
	if nitpick > 0 {
		return nitpick
	}
	switch strings.ToLower(strictness) {
	case "lenient":
		return 2
	case "strict":
		return 8
	default:
		return 5
	}
}

func resolveMRStringSetting(
	cmd *cobra.Command,
	flagName string,
	conf config.Config,
	configKeys []string,
	fallback string,
) string {
	if f := cmd.Flags().Lookup(flagName); f != nil && f.Changed {
		if v, err := cmd.Flags().GetString(flagName); err == nil && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	if conf.Viper != nil {
		for _, k := range configKeys {
			if conf.Viper.IsSet(k) {
				if v := strings.TrimSpace(conf.Viper.GetString(k)); v != "" {
					return v
				}
			}
		}
	}
	return strings.TrimSpace(fallback)
}

func resolveMRIntSetting(
	cmd *cobra.Command,
	flagName string,
	conf config.Config,
	configKeys []string,
	fallback int,
) int {
	if f := cmd.Flags().Lookup(flagName); f != nil && f.Changed {
		if v, err := cmd.Flags().GetInt(flagName); err == nil {
			return v
		}
	}
	if conf.Viper != nil {
		for _, k := range configKeys {
			if conf.Viper.IsSet(k) {
				return conf.Viper.GetInt(k)
			}
		}
	}
	return fallback
}

type hunkRange struct {
	start int
	end   int
}

type inlinePositions struct {
	oldPath  string
	oldByNew map[int]int
	added    map[int]struct{}
	content  map[int]string
	hunks    []hunkRange
}

func collectValidPositions(changes []diffparse.FileChange) map[string]inlinePositions {
	out := make(map[string]inlinePositions, len(changes))
	for _, c := range changes {
		name := c.NewName
		if name == "" {
			continue
		}
		fp, ok := out[name]
		if !ok {
			fp = inlinePositions{
				oldByNew: make(map[int]int),
				added:    make(map[int]struct{}),
				content:  make(map[int]string),
			}
		}
		if fp.oldPath == "" {
			fp.oldPath = c.OldName
		}
		for _, h := range c.Hunks {
			hStart := h.NewStart
			hEnd := h.NewStart + h.NewLines - 1
			if hEnd < hStart {
				hEnd = hStart
			}
			fp.hunks = append(fp.hunks, hunkRange{start: hStart, end: hEnd})
			for _, l := range h.Lines {
				if l.NewLineNo > 0 {
					fp.oldByNew[l.NewLineNo] = l.OldLineNo
					fp.content[l.NewLineNo] = l.Content
					if l.Type == diffparse.LineAdded {
						fp.added[l.NewLineNo] = struct{}{}
					}
				}
			}
		}
		out[name] = fp
	}
	return out
}

func resolveInlinePosition(valid map[string]inlinePositions, filePath string, requestedLine int) (newLine, oldLine int, ok bool) {
	fp, ok := valid[filePath]
	if !ok {
		return 0, 0, false
	}
	if old, exists := fp.oldByNew[requestedLine]; exists {
		// If AI targeted a context line, try to snap to changed line in same hunk (below first).
		if _, added := fp.added[requestedLine]; !added {
			if snapped, ok := nearestAddedInRelevantHunk(fp, requestedLine); ok {
				return snapped, fp.oldByNew[snapped], true
			}
		}
		return requestedLine, old, true
	}

	if snapped, ok := nearestAddedInRelevantHunk(fp, requestedLine); ok {
		return snapped, fp.oldByNew[snapped], true
	}
	return 0, 0, false
}

func refineInlinePositionByMessage(fp inlinePositions, requestedLine, currentLine int, message string) (int, int) {
	if len(fp.added) == 0 || strings.TrimSpace(message) == "" {
		return currentLine, fp.oldByNew[currentLine]
	}
	// Keep exact added-line anchors stable; refinement is only for snapped/fallback anchors.
	if requestedLine == currentLine {
		if _, ok := fp.added[currentLine]; ok {
			return currentLine, fp.oldByNew[currentLine]
		}
	}
	tokens := anchorTokensFromMessage(message)
	if len(tokens) == 0 {
		return currentLine, fp.oldByNew[currentLine]
	}

	hStart, hEnd := nearestHunkRange(fp, currentLine)
	candidates := make([]int, 0, len(fp.added))
	for ln := range fp.added {
		if hStart > 0 && hEnd > 0 {
			if ln < hStart || ln > hEnd {
				continue
			}
		}
		candidates = append(candidates, ln)
	}
	if len(candidates) == 0 {
		for ln := range fp.added {
			candidates = append(candidates, ln)
		}
	}
	if len(candidates) == 0 {
		return currentLine, fp.oldByNew[currentLine]
	}
	sort.Ints(candidates)

	bestLine := currentLine
	bestScore := 0
	bestDist := int(^uint(0) >> 1)
	for _, ln := range candidates {
		content := strings.ToLower(strings.TrimSpace(fp.content[ln]))
		if content == "" {
			continue
		}
		// Avoid anchoring to distant lines when refining.
		if absInt(ln-requestedLine) > 6 {
			continue
		}
		score := 0
		for _, tok := range tokens {
			if strings.Contains(content, tok) {
				score += 2
			}
		}
		if score == 0 {
			continue
		}
		dist := absInt(ln - requestedLine)
		if score > bestScore || (score == bestScore && dist < bestDist) {
			bestScore = score
			bestDist = dist
			bestLine = ln
		}
	}
	if bestScore == 0 {
		return currentLine, fp.oldByNew[currentLine]
	}
	return bestLine, fp.oldByNew[bestLine]
}

func anchorTokensFromMessage(message string) []string {
	lower := strings.ToLower(message)
	re := regexp.MustCompile(`[a-z_][a-z0-9_]{2,}`)
	raw := re.FindAllString(lower, -1)
	if len(raw) == 0 {
		return nil
	}
	stop := map[string]struct{}{
		"the": {}, "and": {}, "for": {}, "with": {}, "without": {}, "this": {}, "that": {},
		"line": {}, "lines": {}, "hunk": {}, "content": {}, "review": {}, "issue": {},
		"high": {}, "medium": {}, "low": {}, "critical": {}, "json": {}, "result": {},
		"returned": {}, "directly": {}, "check": {}, "which": {}, "can": {}, "silently": {},
		"output": {}, "invalid": {}, "failure": {}, "encoding": {},
	}
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, t := range raw {
		if _, bad := stop[t]; bad {
			continue
		}
		if len(t) < 4 {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func nearestAddedInRelevantHunk(fp inlinePositions, requestedLine int) (int, bool) {
	if len(fp.hunks) == 0 || len(fp.added) == 0 {
		return 0, false
	}

	// Prefer hunk containing the requested line; otherwise nearest hunk by distance.
	bestIdx := -1
	bestDist := int(^uint(0) >> 1)
	for i, h := range fp.hunks {
		if requestedLine >= h.start && requestedLine <= h.end {
			bestIdx = i
			break
		}
		dist := h.start - requestedLine
		if dist < 0 {
			dist = requestedLine - h.end
		}
		if dist < bestDist {
			bestDist = dist
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return 0, false
	}
	h := fp.hunks[bestIdx]

	// Below-first policy, then above, inside the same hunk.
	bestBelow := int(^uint(0) >> 1)
	bestAbove := -1
	for ln := range fp.added {
		if ln < h.start || ln > h.end {
			continue
		}
		if ln >= requestedLine && ln < bestBelow {
			bestBelow = ln
		}
		if ln < requestedLine && ln > bestAbove {
			bestAbove = ln
		}
	}
	if bestBelow != int(^uint(0)>>1) {
		return bestBelow, true
	}
	if bestAbove > 0 {
		return bestAbove, true
	}
	return 0, false
}

type carryOverFinding struct {
	DiscussionID string
	FilePath     string
	Line         int
	Severity     string
	Message      string
}

type reusableThread struct {
	DiscussionID string
	FilePath     string
	Line         int
	Severity     string
	Message      string
}

func resolveMentionHandle(conf config.Config) string {
	_ = conf
	return prevMentionHandle
}

func collectCarryOverFindings(
	discussions []vcs.MRDiscussion,
	valid map[string]inlinePositions,
	mentionHandle string,
	pausedThreads map[string]bool,
) []carryOverFinding {
	markerByDiscussion := make(map[string]struct{}, len(discussions))
	for _, d := range discussions {
		for _, n := range d.Notes {
			if strings.Contains(strings.ToLower(n.Body), strings.ToLower(prevCarryOverMarker)) {
				markerByDiscussion[d.ID] = struct{}{}
				break
			}
		}
	}

	seen := map[string]struct{}{}
	var out []carryOverFinding
	for _, d := range discussions {
		if pausedThreads[d.ID] {
			continue
		}
		if _, already := markerByDiscussion[d.ID]; already {
			continue
		}
		if !isPrevThread(d, mentionHandle) && !threadHasCommand(d, mentionHandle, "review") {
			continue
		}
		for _, n := range d.Notes {
			if !n.Resolvable || n.Resolved || n.FilePath == "" || n.Line <= 0 {
				continue
			}
			if _, _, ok := resolveInlinePosition(valid, n.FilePath, n.Line); !ok {
				continue
			}
			sev, msg, ok := severityAndMessage(n.Body)
			if !ok {
				continue
			}
			key := inlineKey(n.FilePath, n.Line, "["+sev+"] "+msg)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, carryOverFinding{
				DiscussionID: d.ID,
				FilePath:     n.FilePath,
				Line:         n.Line,
				Severity:     sev,
				Message:      msg,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return severityRank(out[i].Severity) > severityRank(out[j].Severity)
	})
	return out
}

func appendCarryOverGuidelines(guidelines string, carry []carryOverFinding) string {
	if len(carry) == 0 {
		return guidelines
	}
	lines := make([]string, 0, len(carry)+2)
	lines = append(lines, "Address unresolved carry-over findings first (if still valid in this diff):")
	for i, c := range carry {
		if i >= 20 {
			break
		}
		lines = append(lines, fmt.Sprintf("- %s:%d [%s] %s", c.FilePath, c.Line, c.Severity, c.Message))
	}
	block := strings.Join(lines, "\n")
	if strings.TrimSpace(guidelines) == "" {
		return block
	}
	return guidelines + "\n" + block
}

func postCarryOverReminders(
	vcsProvider vcs.VCSProvider,
	projectID string,
	mrIID int64,
	discussions []vcs.MRDiscussion,
	carry []carryOverFinding,
	pausedThreads map[string]bool,
) int {
	if len(carry) == 0 {
		return 0
	}
	hasReminder := map[string]struct{}{}
	for _, d := range discussions {
		for _, n := range d.Notes {
			if strings.Contains(strings.ToLower(n.Body), strings.ToLower(prevCarryOverMarker)) {
				hasReminder[d.ID] = struct{}{}
				break
			}
		}
	}

	posted := 0
	for _, c := range carry {
		if pausedThreads[c.DiscussionID] {
			continue
		}
		if _, ok := hasReminder[c.DiscussionID]; ok {
			continue
		}
		body := fmt.Sprintf(
			"%s\nUnresolved prior finding is still present in this revision at `%s:%d` [%s]. Please address this before lower-priority items.",
			prevCarryOverMarker, c.FilePath, c.Line, c.Severity,
		)
		if err := vcsProvider.ReplyToMRDiscussion(projectID, mrIID, c.DiscussionID, body); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to post carry-over reminder in discussion %s: %v\n", c.DiscussionID, err)
			continue
		}
		hasReminder[c.DiscussionID] = struct{}{}
		posted++
	}
	return posted
}

func processReplyCommands(
	vcsProvider vcs.VCSProvider,
	ai provider.AIProvider,
	projectID string,
	mrIID int64,
	discussions []vcs.MRDiscussion,
	changes []diffparse.FileChange,
	mentionHandle string,
	pausedThreads map[string]bool,
) int {
	posted := 0
	for _, d := range discussions {
		if pausedThreads[d.ID] {
			continue
		}
		if !isPrevThread(d, mentionHandle) && !threadHasReplyRequest(d, mentionHandle) {
			continue
		}
		reqIdx := latestReplyRequestIndex(d.Notes, mentionHandle)
		if reqIdx < 0 {
			continue
		}
		if hasMarkerAfter(d.Notes, reqIdx, prevReplyMarker) {
			continue
		}
		path, line := discussionAnchor(d)
		hunk := extractHunkContext(changes, path, line)
		prompt := buildThreadReplyPrompt(d, hunk)
		_, choices, err := provider.SimpleComplete(
			ai,
			"You are an expert code reviewer replying in a merge request discussion.",
			"You answer thread questions with code-aware reasoning tied to the hunk context.",
			prompt,
		)
		if err != nil || len(choices) == 0 || strings.TrimSpace(choices[0]) == "" {
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to generate reply for discussion %s: %v\n", d.ID, err)
			}
			continue
		}
		body := strings.TrimSpace(choices[0]) + "\n\n" + prevReplyMarker
		if err := vcsProvider.ReplyToMRDiscussion(projectID, mrIID, d.ID, body); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to post reply in discussion %s: %v\n", d.ID, err)
			continue
		}
		posted++
	}
	return posted
}

func buildThreadReplyPrompt(d vcs.MRDiscussion, hunk string) string {
	var convo []string
	for _, n := range d.Notes {
		convo = append(convo, fmt.Sprintf("- %s: %s", n.Author, strings.TrimSpace(n.Body)))
	}
	return "Thread conversation:\n" + strings.Join(convo, "\n") + "\n\n" +
		"Hunk context (use this before answering):\n" + hunk + "\n\n" +
		"Task: Reply to the latest @mention in this thread. " +
		"Be concise, technical, and address impact/risk first."
}

func processNoteReplyCommands(
	vcsProvider vcs.VCSProvider,
	ai provider.AIProvider,
	projectID string,
	mrIID int64,
	notes []vcs.MRNote,
	mr *vcs.MergeRequest,
	validPositionsByFile map[string]inlinePositions,
	mentionHandle string,
) int {
	if strings.TrimSpace(mentionHandle) == "" {
		return 0
	}
	posted := 0
	path, newLine, oldLine, hasAnchor := pickInlineAnchor(validPositionsByFile)
	for i := range notes {
		note := notes[i]
		if isBotAuthor(note.Author, mentionHandle) {
			continue
		}
		if !isReplyRequest(note.Body, mentionHandle) {
			continue
		}
		if hasNoteMarkerAfter(notes, i, prevReplyMarker) {
			continue
		}
		if !hasAnchor {
			fmt.Fprintf(os.Stderr, "Warning: no inline anchor available to reply to top-level note %d\n", note.ID)
			continue
		}
		prompt := buildNoteReplyPrompt(note, mr)
		_, choices, err := provider.SimpleComplete(
			ai,
			"You are an expert code reviewer replying to a merge request comment.",
			"Answer questions directly and concisely, referencing the MR context when helpful.",
			prompt,
		)
		if err != nil || len(choices) == 0 || strings.TrimSpace(choices[0]) == "" {
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to generate reply for note %d: %v\n", note.ID, err)
			}
			continue
		}
		body := strings.TrimSpace(choices[0]) + "\n\n" + prevReplyMarker
		if mr == nil || mr.DiffRefs.HeadSHA == "" || mr.DiffRefs.BaseSHA == "" {
			fmt.Fprintf(os.Stderr, "Warning: missing diff refs; cannot post inline reply for note %d\n", note.ID)
			continue
		}
		if err := vcsProvider.PostInlineComment(projectID, mrIID, mr.DiffRefs, vcs.InlineComment{
			FilePath: path,
			OldPath:  validPositionsByFile[path].oldPath,
			NewLine:  int64(newLine),
			OldLine:  int64(oldLine),
			Body:     body,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to post inline reply: %v\n", err)
			continue
		}
		posted++
	}
	return posted
}

func buildNoteReplyPrompt(note vcs.MRNote, mr *vcs.MergeRequest) string {
	var sb strings.Builder
	if mr != nil {
		sb.WriteString(fmt.Sprintf("Merge request: %s\n", strings.TrimSpace(mr.Title)))
		if strings.TrimSpace(mr.Description) != "" {
			sb.WriteString("Description:\n")
			sb.WriteString(strings.TrimSpace(mr.Description))
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\nComment:\n")
	sb.WriteString(strings.TrimSpace(note.Body))
	sb.WriteString("\n\nTask: Reply to the latest @mention in this comment.")
	return sb.String()
}

func pickInlineAnchor(validPositionsByFile map[string]inlinePositions) (string, int, int, bool) {
	if len(validPositionsByFile) == 0 {
		return "", 0, 0, false
	}
	paths := make([]string, 0, len(validPositionsByFile))
	for path := range validPositionsByFile {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		line, ok := fallbackInlineLine(validPositionsByFile, path)
		if !ok || line <= 0 {
			continue
		}
		fp := validPositionsByFile[path]
		oldLine := fp.oldByNew[line]
		return path, line, oldLine, true
	}
	return "", 0, 0, false
}

func extractHunkContext(changes []diffparse.FileChange, filePath string, line int) string {
	if filePath == "" || line <= 0 {
		for _, c := range changes {
			if c.NewName == "" {
				continue
			}
			for _, h := range c.Hunks {
				anchor := h.NewStart
				if anchor <= 0 {
					continue
				}
				fallback := extractHunkContext(changes, c.NewName, anchor)
				if strings.HasPrefix(fallback, "No local hunk slice found") {
					continue
				}
				return fmt.Sprintf("Thread has no inline anchor; using representative MR hunk from %s:%d.\n%s", c.NewName, anchor, fallback)
			}
		}
		return "No inline hunk available for this thread."
	}
	var out []string
	minLine, maxLine := line-3, line+3
	for _, c := range changes {
		if c.NewName != filePath {
			continue
		}
		for _, h := range c.Hunks {
			for _, l := range h.Lines {
				if l.NewLineNo < minLine || l.NewLineNo > maxLine {
					continue
				}
				prefix := " "
				switch l.Type {
				case diffparse.LineAdded:
					prefix = "+"
				case diffparse.LineDeleted:
					prefix = "-"
				}
				out = append(out, fmt.Sprintf("%s %d %s", prefix, l.NewLineNo, l.Content))
			}
		}
	}
	if len(out) == 0 {
		return fmt.Sprintf("No local hunk slice found for %s:%d.", filePath, line)
	}
	return strings.Join(out, "\n")
}

func discussionAnchor(d vcs.MRDiscussion) (string, int) {
	for _, n := range d.Notes {
		if n.FilePath != "" && n.Line > 0 {
			return n.FilePath, n.Line
		}
	}
	return "", 0
}

func isPrevThread(d vcs.MRDiscussion, mentionHandle string) bool {
	if len(d.Notes) == 0 {
		return false
	}
	first := d.Notes[0]
	if strings.Contains(strings.ToLower(first.Body), strings.ToLower(prevThreadMarker)) {
		return true
	}
	if mentionHandle != "" && strings.EqualFold(first.Author, mentionHandle) {
		if _, _, ok := severityAndMessage(first.Body); ok {
			return true
		}
	}
	return false
}

func threadHasCommand(d vcs.MRDiscussion, mentionHandle, command string) bool {
	for _, n := range d.Notes {
		if hasMentionCommand(n.Body, mentionHandle, command) {
			return true
		}
	}
	return false
}

func threadHasAnyCommand(discussions []vcs.MRDiscussion, mentionHandle, command string) bool {
	for _, d := range discussions {
		if threadHasCommand(d, mentionHandle, command) {
			return true
		}
	}
	return false
}

func threadHasReplyRequest(d vcs.MRDiscussion, mentionHandle string) bool {
	return latestReplyRequestIndex(d.Notes, mentionHandle) >= 0
}

func latestCommandIndex(notes []vcs.MRDiscussionNote, mentionHandle, command string) int {
	for i := len(notes) - 1; i >= 0; i-- {
		if hasMentionCommand(notes[i].Body, mentionHandle, command) {
			return i
		}
	}
	return -1
}

func latestReplyRequestIndex(notes []vcs.MRDiscussionNote, mentionHandle string) int {
	for i := len(notes) - 1; i >= 0; i-- {
		if isReplyRequest(notes[i].Body, mentionHandle) {
			return i
		}
	}
	return -1
}

func hasMarkerAfter(notes []vcs.MRDiscussionNote, idx int, marker string) bool {
	marker = strings.ToLower(marker)
	for i := idx + 1; i < len(notes); i++ {
		if strings.Contains(strings.ToLower(notes[i].Body), marker) {
			return true
		}
	}
	return false
}

func hasNoteMarkerAfter(notes []vcs.MRNote, idx int, marker string) bool {
	marker = strings.ToLower(marker)
	for i := idx + 1; i < len(notes); i++ {
		if strings.Contains(strings.ToLower(notes[i].Body), marker) {
			return true
		}
	}
	return false
}

func hasMentionCommand(body, mentionHandle, command string) bool {
	body = strings.ToLower(body)
	handle := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(mentionHandle), "@"))
	if handle == "" {
		return false
	}
	pattern := "@" + handle + " " + strings.ToLower(command)
	return strings.Contains(body, pattern)
}

func isReplyRequest(body, mentionHandle string) bool {
	return hasMentionCommand(body, mentionHandle, "reply")
}

func isBotAuthor(author, mentionHandle string) bool {
	author = strings.TrimSpace(strings.ToLower(author))
	handle := strings.TrimSpace(strings.ToLower(strings.TrimPrefix(mentionHandle, "@")))
	if author == "" || handle == "" {
		return false
	}
	return author == handle
}

func isMRPaused(notes []vcs.MRNote, mentionHandle string) bool {
	if strings.TrimSpace(mentionHandle) == "" {
		return false
	}
	paused := false
	seen := false
	for _, n := range notes {
		if hasMentionCommand(n.Body, mentionHandle, "pause") {
			paused = true
			seen = true
		}
		if hasMentionCommand(n.Body, mentionHandle, "resume") {
			paused = false
			seen = true
		}
	}
	return seen && paused
}

func pausedDiscussions(discussions []vcs.MRDiscussion, mentionHandle string) map[string]bool {
	out := make(map[string]bool, len(discussions))
	if strings.TrimSpace(mentionHandle) == "" {
		return out
	}
	for _, d := range discussions {
		paused := false
		seen := false
		for _, n := range d.Notes {
			if hasMentionCommand(n.Body, mentionHandle, "pause") {
				paused = true
				seen = true
			}
			if hasMentionCommand(n.Body, mentionHandle, "resume") {
				paused = false
				seen = true
			}
		}
		if seen && paused {
			out[d.ID] = true
		}
	}
	return out
}

func severityAndMessage(body string) (string, string, bool) {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "<!--") {
			continue
		}
		if !strings.HasPrefix(line, "[") {
			continue
		}
		closeIdx := strings.Index(line, "]")
		if closeIdx <= 1 {
			continue
		}
		sev := strings.ToUpper(strings.TrimSpace(line[1:closeIdx]))
		switch sev {
		case "CRITICAL", "HIGH", "MEDIUM", "LOW":
		default:
			continue
		}
		msg := strings.TrimSpace(line[closeIdx+1:])
		if msg == "" {
			for j := i + 1; j < len(lines); j++ {
				candidate := strings.TrimSpace(lines[j])
				if candidate == "" || strings.HasPrefix(candidate, "<!--") {
					continue
				}
				candidate = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(candidate, "-"), "*"))
				if candidate != "" {
					msg = candidate
					break
				}
			}
		}
		if msg == "" {
			msg = sev + " finding"
		}
		return sev, msg, true
	}
	return "", "", false
}

func severityRank(sev string) int {
	switch strings.ToUpper(strings.TrimSpace(sev)) {
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

func existingInlineKeys(discussions []vcs.MRDiscussion) map[string]struct{} {
	out := make(map[string]struct{})
	for _, d := range discussions {
		for _, n := range d.Notes {
			if n.FilePath == "" || n.Line <= 0 {
				continue
			}
			out[inlineKey(n.FilePath, n.Line, n.Body)] = struct{}{}
		}
	}
	return out
}

func existingInlineSeverityKeys(discussions []vcs.MRDiscussion) map[string]struct{} {
	out := make(map[string]struct{})
	for _, d := range discussions {
		for _, n := range d.Notes {
			if n.FilePath == "" || n.Line <= 0 {
				continue
			}
			if sev, _, ok := severityAndMessage(n.Body); ok {
				out[inlineSeverityKey(n.FilePath, n.Line, sev)] = struct{}{}
			}
		}
	}
	return out
}

func collectReusableThreads(
	discussions []vcs.MRDiscussion,
	mentionHandle string,
	pausedThreads map[string]bool,
) []reusableThread {
	var out []reusableThread
	for _, d := range discussions {
		if pausedThreads[d.ID] {
			continue
		}
		if !isPrevThread(d, mentionHandle) && !threadHasCommand(d, mentionHandle, "review") {
			continue
		}
		anchorPath, anchorLine := discussionAnchor(d)
		if strings.TrimSpace(anchorPath) == "" || anchorLine <= 0 {
			continue
		}
		for i := len(d.Notes) - 1; i >= 0; i-- {
			n := d.Notes[i]
			if n.Resolved || !n.Resolvable {
				continue
			}
			sev, msg, ok := severityAndMessage(n.Body)
			if !ok {
				continue
			}
			out = append(out, reusableThread{
				DiscussionID: d.ID,
				FilePath:     anchorPath,
				Line:         anchorLine,
				Severity:     sev,
				Message:      msg,
			})
			break
		}
	}
	return out
}

func matchReusableThread(candidates []reusableThread, grp inlineGroup) (reusableThread, bool) {
	bestIdx := -1
	bestScore := -1
	for i, c := range candidates {
		if !strings.EqualFold(strings.TrimSpace(c.FilePath), strings.TrimSpace(grp.FilePath)) {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(c.Severity), strings.TrimSpace(grp.Severity)) {
			continue
		}
		msgScore := tokenOverlapScore(c.Message, grp.Message)
		distPenalty := absInt(c.Line - grp.NewLine)
		score := msgScore*10 - minInt(distPenalty, 50)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return reusableThread{}, false
	}
	if bestScore < 10 {
		return reusableThread{}, false
	}
	return candidates[bestIdx], true
}

func tokenOverlapScore(a, b string) int {
	ta := toKeywordSet(a)
	tb := toKeywordSet(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	score := 0
	for tok := range ta {
		if _, ok := tb[tok]; ok {
			score++
		}
	}
	return score
}

func toKeywordSet(s string) map[string]struct{} {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	fields := strings.Fields(b.String())
	out := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		if len(f) <= 2 {
			continue
		}
		out[f] = struct{}{}
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func inlineKey(filePath string, line int, body string) string {
	_ = body
	return strings.ToLower(strings.TrimSpace(filePath) + "|" + strconv.Itoa(line))
}

func inlineSeverityKey(filePath string, line int, severity string) string {
	return strings.ToLower(strings.TrimSpace(filePath) + "|" + strconv.Itoa(line) + "|" + strings.ToUpper(strings.TrimSpace(severity)))
}

func hasTopLevelMarker(notes []vcs.MRNote, marker string) bool {
	marker = strings.ToLower(strings.TrimSpace(marker))
	if marker == "" {
		return false
	}
	for _, n := range notes {
		if strings.Contains(strings.ToLower(n.Body), marker) {
			return true
		}
	}
	return false
}

func conciseInlineBody(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return trimmed
	}
	firstPara := strings.Split(trimmed, "\n\n")[0]
	candidate := firstPara
	if !strings.Contains(candidate, "\n- ") && !strings.Contains(candidate, "\n* ") {
		for _, sep := range []string{". ", "! ", "? "} {
			if i := strings.Index(candidate, sep); i >= 0 {
				candidate = candidate[:i+1]
				break
			}
		}
	}
	candidate = strings.TrimSpace(candidate)
	const maxLen = 220
	if len(candidate) > maxLen {
		candidate = strings.TrimSpace(candidate[:maxLen-1]) + "…"
	}
	return candidate
}

func buildInlineCommentBody(
	severity string,
	message string,
	suggestion string,
	formatSuggestion func(string) string,
) string {
	sev := strings.ToUpper(strings.TrimSpace(severity))
	if sev == "" {
		sev = "MEDIUM"
	}

	points := extractKeyPoints(message)
	primary := ""
	for _, p := range points {
		if isNonActionableInlinePoint(p) {
			continue
		}
		primary = p
		break
	}
	if primary == "" && len(points) > 0 {
		primary = points[0]
	}
	if primary == "" {
		primary = "Review this change for correctness and side effects."
	}
	body := conciseInlineBody(fmt.Sprintf("[%s] %s", sev, primary))

	suggestion = normalizeSuggestion(suggestion)
	if suggestion != "" && formatSuggestion != nil {
		body += "\n\nSuggested patch:\n" + formatSuggestion(suggestion)
	}
	return body
}

func normalizeSuggestion(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if start >= end {
		return ""
	}
	return strings.Join(lines[start:end], "\n")
}

func rebaseSuggestionIndentation(suggestion string, anchorLine string) string {
	suggestion = normalizeSuggestion(suggestion)
	if suggestion == "" {
		return ""
	}
	anchorIndent := leadingIndent(anchorLine)
	if anchorIndent == "" {
		return suggestion
	}
	lines := strings.Split(strings.ReplaceAll(suggestion, "\r\n", "\n"), "\n")
	nonEmpty := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}
	if len(nonEmpty) == 0 {
		return suggestion
	}
	commonIndent := leadingIndent(nonEmpty[0])
	for i := 1; i < len(nonEmpty); i++ {
		commonIndent = commonPrefix(commonIndent, leadingIndent(nonEmpty[i]))
		if commonIndent == "" {
			break
		}
	}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines[i] = anchorIndent + strings.TrimPrefix(line, commonIndent)
	}
	return strings.Join(lines, "\n")
}

func leadingIndent(s string) string {
	i := 0
	for i < len(s) {
		if s[i] != ' ' && s[i] != '\t' {
			break
		}
		i++
	}
	return s[:i]
}

func commonPrefix(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return a[:i]
}

func normalizeFixPromptMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "auto", "always":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return "off"
	}
}

func shouldIncludeAgentFixPrompt(grp inlineGroup, mode string) bool {
	mode = normalizeFixPromptMode(mode)
	if mode == "off" {
		return false
	}
	if strings.TrimSpace(grp.FilePath) == "" || grp.NewLine <= 0 {
		return false
	}
	if strings.TrimSpace(grp.Message) == "" {
		return false
	}
	if mode == "always" {
		return true
	}
	// auto mode: include only when no concrete patch is available and issue is high impact.
	if strings.TrimSpace(grp.Suggestion) != "" {
		return false
	}
	return severityRank(grp.Severity) >= severityRank("HIGH")
}

func buildAgentFixPrompt(grp inlineGroup, mode string) string {
	if !shouldIncludeAgentFixPrompt(grp, mode) {
		return ""
	}
	return fmt.Sprintf(
		"You are fixing a code-review finding.\n"+
			"Target file: %s\n"+
			"Target line: %d\n"+
			"Severity: %s\n"+
			"Finding: %s\n\n"+
			"Task:\n"+
			"1) Produce a minimal patch that fixes the issue without changing unrelated behavior.\n"+
			"2) Preserve API/ABI compatibility unless a breaking change is explicitly required.\n"+
			"3) Add or update tests to prevent regression.\n"+
			"4) Explain any concurrency/race-condition implications if shared state is touched.\n"+
			"5) Return: (a) unified diff, (b) test diff, (c) short risk note.",
		strings.TrimSpace(grp.FilePath),
		grp.NewLine,
		strings.ToUpper(strings.TrimSpace(grp.Severity)),
		strings.TrimSpace(conciseInlineBody(grp.Message)),
	)
}

func isNonActionableInlinePoint(point string) bool {
	p := strings.ToLower(strings.TrimSpace(point))
	if p == "" {
		return true
	}
	if strings.HasPrefix(p, "hunk new lines ") || strings.HasPrefix(p, "hunk anchor line ") {
		return true
	}
	switch p {
	case "summary", "analysis priority", "project scope map", "remediation plan", "file-by-file findings":
		return true
	default:
		return false
	}
}

func extractKeyPoints(message string) []string {
	clean := sanitizeInlineMessage(message)
	if clean == "" {
		return nil
	}

	lines := strings.Split(clean, "\n")
	var out []string
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		t = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(t, "-"), "*"))
		if t == "" {
			continue
		}
		lower := strings.ToLower(strings.TrimSpace(t))
		if lower == "key points:" || lower == "key point:" || strings.HasPrefix(lower, "key points (") {
			continue
		}
		t = squeezeSpaces(t)
		if t != "" {
			out = append(out, limitLen(t, 220))
		}
	}

	if len(out) == 0 {
		return nil
	}
	if len(out) > 4 {
		out = out[:4]
	}
	return dedupeStrings(out)
}

func sanitizeInlineMessage(message string) string {
	var out []string
	inFence := false
	for _, line := range strings.Split(strings.TrimSpace(message), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func squeezeSpaces(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func limitLen(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n-1]) + "…"
}

func dedupeStrings(items []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, it := range items {
		k := strings.ToLower(it)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, it)
	}
	return out
}

func aggregateCommentsByChange(comments []core.FileComment) []core.FileComment {
	type grouped struct {
		filePath            string
		line                int
		kind                string
		severity            string
		maxSeverityRank     int
		messages            []string
		seenMessages        map[string]struct{}
		suggestion          string
		multipleSuggestions bool
	}

	var order []string
	byKey := make(map[string]*grouped)

	for _, c := range comments {
		filePath := strings.TrimSpace(c.FilePath)
		if filePath == "" || c.Line <= 0 {
			continue
		}
		key := strings.ToLower(filePath) + "|" + strconv.Itoa(c.Line)
		g, ok := byKey[key]
		if !ok {
			g = &grouped{
				filePath:     filePath,
				line:         c.Line,
				kind:         strings.ToUpper(strings.TrimSpace(c.Kind)),
				severity:     strings.ToUpper(strings.TrimSpace(c.Severity)),
				seenMessages: map[string]struct{}{},
			}
			if g.kind == "" {
				g.kind = "ISSUE"
			}
			if g.severity == "" {
				g.severity = "MEDIUM"
			}
			g.maxSeverityRank = severityRank(g.severity)
			byKey[key] = g
			order = append(order, key)
		}

		rank := severityRank(c.Severity)
		if rank > g.maxSeverityRank {
			g.maxSeverityRank = rank
			g.severity = strings.ToUpper(strings.TrimSpace(c.Severity))
		}
		if g.severity == "" {
			g.severity = "MEDIUM"
		}

		msg := strings.TrimSpace(c.Message)
		if msg != "" {
			norm := strings.ToLower(strings.Join(strings.Fields(msg), " "))
			if _, exists := g.seenMessages[norm]; !exists {
				g.seenMessages[norm] = struct{}{}
				g.messages = append(g.messages, msg)
			}
		}

		sug := strings.TrimSpace(c.Suggestion)
		if sug != "" {
			if g.suggestion == "" {
				g.suggestion = sug
			} else if g.suggestion != sug {
				g.multipleSuggestions = true
			}
		}
	}

	out := make([]core.FileComment, 0, len(order))
	for _, key := range order {
		g := byKey[key]
		if len(g.messages) == 0 {
			continue
		}
		message := g.messages[0]
		if len(g.messages) > 1 {
			var sb strings.Builder
			sb.WriteString("Key points:")
			for _, m := range g.messages {
				sb.WriteString("\n- ")
				sb.WriteString(m)
			}
			message = sb.String()
		}

		suggestion := g.suggestion
		if g.multipleSuggestions {
			suggestion = ""
		}

		out = append(out, core.FileComment{
			FilePath:   g.filePath,
			Line:       g.line,
			Kind:       g.kind,
			Severity:   g.severity,
			Message:    message,
			Suggestion: suggestion,
		})
	}
	return out
}

type inlineGroup struct {
	FilePath   string
	NewLine    int
	OldLine    int
	Severity   string
	Message    string
	Suggestion string
}

func aggregateCommentsByHunk(
	comments []core.FileComment,
	validPositionsByFile map[string]inlinePositions,
) ([]inlineGroup, []string) {
	type grouped struct {
		inlineGroup
		maxSeverityRank     int
		messages            []string
		seenMessages        map[string]struct{}
		multipleSuggestions bool
	}

	byKey := make(map[string]*grouped)
	var order []string
	var unplaced []string

	for _, fc := range comments {
		if strings.TrimSpace(fc.Message) == "" {
			continue
		}
		requestedLine := fc.Line
		if requestedLine <= 0 {
			fallback, ok := fallbackInlineLine(validPositionsByFile, fc.FilePath)
			if !ok {
				continue
			}
			requestedLine = fallback
		}
		newLine, oldLine, ok := resolveInlinePosition(validPositionsByFile, fc.FilePath, requestedLine)
		if !ok {
			unplaced = append(unplaced, fmt.Sprintf("- %s:%d [%s/%s] %s",
				fc.FilePath, requestedLine, strings.ToUpper(fc.Kind), strings.ToUpper(fc.Severity), fc.Message))
			continue
		}
		if fp, ok := validPositionsByFile[fc.FilePath]; ok {
			newLine, oldLine = refineInlinePositionByMessage(fp, requestedLine, newLine, fc.Message)
		}

		hunkStart, hunkEnd := nearestHunkRange(validPositionsByFile[fc.FilePath], newLine)
		key := strings.ToLower(fc.FilePath) + "|" + strconv.Itoa(hunkStart) + "|" + strconv.Itoa(hunkEnd)
		label := fmt.Sprintf("Hunk new lines %d-%d", hunkStart, hunkEnd)
		if hunkStart <= 0 || hunkEnd <= 0 {
			key = strings.ToLower(fc.FilePath) + "|" + strconv.Itoa(newLine)
			label = fmt.Sprintf("Hunk anchor line %d", newLine)
		}

		g, exists := byKey[key]
		if !exists {
			g = &grouped{
				inlineGroup: inlineGroup{
					FilePath: fc.FilePath,
					NewLine:  newLine,
					OldLine:  oldLine,
					Severity: strings.ToUpper(strings.TrimSpace(fc.Severity)),
				},
				messages:     []string{label},
				seenMessages: map[string]struct{}{},
			}
			if g.Severity == "" {
				g.Severity = "MEDIUM"
			}
			g.maxSeverityRank = severityRank(g.Severity)
			byKey[key] = g
			order = append(order, key)
		}

		if r := severityRank(fc.Severity); r > g.maxSeverityRank {
			g.maxSeverityRank = r
			g.Severity = strings.ToUpper(strings.TrimSpace(fc.Severity))
		}
		msg := strings.TrimSpace(fc.Message)
		norm := strings.ToLower(strings.Join(strings.Fields(msg), " "))
		if msg != "" {
			if _, ok := g.seenMessages[norm]; !ok {
				g.seenMessages[norm] = struct{}{}
				g.messages = append(g.messages, msg)
			}
		}
		sug := strings.TrimSpace(fc.Suggestion)
		if sug != "" {
			if g.Suggestion == "" {
				g.Suggestion = sug
			} else if g.Suggestion != sug {
				g.multipleSuggestions = true
			}
		}
	}

	out := make([]inlineGroup, 0, len(order))
	for _, key := range order {
		g := byKey[key]
		if len(g.messages) <= 1 {
			continue
		}
		var sb strings.Builder
		sb.WriteString(g.messages[0])
		sb.WriteString("\nKey points:")
		for _, m := range g.messages[1:] {
			sb.WriteString("\n- ")
			sb.WriteString(m)
		}
		g.Message = sb.String()
		if g.multipleSuggestions {
			g.Suggestion = ""
		}
		out = append(out, g.inlineGroup)
	}
	return out, unplaced
}

func aggregateCommentsByLine(
	comments []core.FileComment,
	validPositionsByFile map[string]inlinePositions,
) ([]inlineGroup, []string) {
	var out []inlineGroup
	var unplaced []string
	for _, fc := range comments {
		if strings.TrimSpace(fc.Message) == "" {
			continue
		}
		requestedLine := fc.Line
		if requestedLine <= 0 {
			fallback, ok := fallbackInlineLine(validPositionsByFile, fc.FilePath)
			if !ok {
				continue
			}
			requestedLine = fallback
		}
		newLine, oldLine, ok := resolveInlinePosition(validPositionsByFile, fc.FilePath, requestedLine)
		if !ok {
			unplaced = append(unplaced, fmt.Sprintf("- %s:%d [%s/%s] %s",
				fc.FilePath, requestedLine, strings.ToUpper(fc.Kind), strings.ToUpper(fc.Severity), fc.Message))
			continue
		}
		if fp, ok := validPositionsByFile[fc.FilePath]; ok {
			newLine, oldLine = refineInlinePositionByMessage(fp, requestedLine, newLine, fc.Message)
		}
		out = append(out, inlineGroup{
			FilePath:   fc.FilePath,
			NewLine:    newLine,
			OldLine:    oldLine,
			Severity:   strings.ToUpper(strings.TrimSpace(fc.Severity)),
			Message:    fc.Message,
			Suggestion: fc.Suggestion,
		})
	}
	return out, unplaced
}

func fallbackInlineLine(valid map[string]inlinePositions, filePath string) (int, bool) {
	fp, ok := valid[filePath]
	if !ok {
		return 0, false
	}
	if len(fp.added) > 0 {
		minLine := 0
		for l := range fp.added {
			if minLine == 0 || l < minLine {
				minLine = l
			}
		}
		if minLine > 0 {
			return minLine, true
		}
	}
	if len(fp.hunks) > 0 {
		minStart := 0
		for _, h := range fp.hunks {
			if h.start <= 0 {
				continue
			}
			if minStart == 0 || h.start < minStart {
				minStart = h.start
			}
		}
		if minStart > 0 {
			return minStart, true
		}
	}
	return 0, false
}

func nearestHunkRange(fp inlinePositions, line int) (int, int) {
	if len(fp.hunks) == 0 {
		return 0, 0
	}
	best := fp.hunks[0]
	bestDist := absInt(line-best.start) + absInt(line-best.end)
	for _, h := range fp.hunks {
		if line >= h.start && line <= h.end {
			return h.start, h.end
		}
		dist := absInt(line-h.start) + absInt(line-h.end)
		if dist < bestDist {
			best = h
			bestDist = dist
		}
	}
	return best.start, best.end
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func prioritizeAndLimitInlineGroups(groups []inlineGroup, max int) []inlineGroup {
	if max <= 0 || len(groups) <= max {
		return groups
	}
	sort.SliceStable(groups, func(i, j int) bool {
		ri := severityRank(groups[i].Severity)
		rj := severityRank(groups[j].Severity)
		if ri != rj {
			return ri > rj
		}
		return i < j
	})
	return groups[:max]
}

func filterCommentsByFileFocus(comments []core.FileComment) []core.FileComment {
	out := make([]core.FileComment, 0, len(comments))
	for _, c := range comments {
		if isDocTextFile(c.FilePath) && severityRank(c.Severity) < severityRank("HIGH") && !isLikelyTypoComment(c.Message) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func filterInlineCandidates(
	parsed []core.FileComment,
	strictness string,
	nitpick int,
	conventions []string,
	validPositionsByFile map[string]inlinePositions,
	filterMode string,
) ([]core.FileComment, bool) {
	mode := normalizeInlineFilterMode(filterMode)
	raw := core.FilterForReview(parsed, strictness, nitpick, conventions)
	base := raw
	usedFallback := false
	if len(base) == 0 && len(parsed) > 0 {
		base = limitToChangedFiles(parsed, validPositionsByFile)
		if len(base) == 0 {
			base = parsed
		}
		usedFallback = true
	}
	if len(base) == 0 {
		return nil, usedFallback
	}

	modeFiltered := applyInlineFilterMode(base, validPositionsByFile, mode)
	if len(modeFiltered) > 0 {
		return modeFiltered, usedFallback
	}
	if mode != "nofilter" {
		usedFallback = true
	}
	return base, usedFallback
}

func limitToChangedFiles(comments []core.FileComment, validPositionsByFile map[string]inlinePositions) []core.FileComment {
	if len(comments) == 0 || len(validPositionsByFile) == 0 {
		return nil
	}

	out := make([]core.FileComment, 0, len(comments))
	for _, c := range comments {
		path := strings.TrimSpace(strings.TrimPrefix(c.FilePath, "./"))
		if path == "" {
			continue
		}
		if _, ok := validPositionsByFile[path]; ok {
			out = append(out, c)
		}
	}
	return out
}

func normalizeInlineFilterMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "added", "diff_context", "file", "nofilter":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return "diff_context"
	}
}

func applyInlineFilterMode(
	comments []core.FileComment,
	validPositionsByFile map[string]inlinePositions,
	mode string,
) []core.FileComment {
	mode = normalizeInlineFilterMode(mode)
	switch mode {
	case "nofilter":
		return comments
	case "file":
		return limitToChangedFiles(comments, validPositionsByFile)
	case "added":
		var out []core.FileComment
		for _, c := range comments {
			if isOnAddedLine(c, validPositionsByFile) {
				out = append(out, c)
			}
		}
		return out
	default: // diff_context
		var out []core.FileComment
		for _, c := range comments {
			if isInDiffContext(c, validPositionsByFile) {
				out = append(out, c)
			}
		}
		return out
	}
}

func isInDiffContext(c core.FileComment, valid map[string]inlinePositions) bool {
	path := strings.TrimSpace(strings.TrimPrefix(c.FilePath, "./"))
	fp, ok := valid[path]
	if !ok {
		return false
	}
	if c.Line <= 0 {
		return len(fp.hunks) > 0 || len(fp.added) > 0
	}
	if _, ok := fp.oldByNew[c.Line]; ok {
		return true
	}
	if _, ok := fp.added[c.Line]; ok {
		return true
	}
	_, _, ok = resolveInlinePosition(valid, path, c.Line)
	if ok {
		return true
	}
	return len(fp.hunks) > 0 || len(fp.added) > 0
}

func isOnAddedLine(c core.FileComment, valid map[string]inlinePositions) bool {
	path := strings.TrimSpace(strings.TrimPrefix(c.FilePath, "./"))
	fp, ok := valid[path]
	if !ok {
		return false
	}
	if c.Line <= 0 {
		fallback, ok := fallbackInlineLine(valid, path)
		if !ok {
			return false
		}
		_, ok = fp.added[fallback]
		return ok
	}
	if _, ok := fp.added[c.Line]; ok {
		return true
	}
	return false
}

func parseReviewContent(content string, structuredOutput bool) core.ReviewResult {
	if structuredOutput {
		if parsed, ok := core.ParseReviewResponseJSON(content); ok {
			return parsed
		}
	}
	return core.ParseReviewResponse(content)
}

func appendLineAnchorInstructions(prompt string) string {
	const block = `
## Line Anchoring Requirement
- The MR context already includes changed hunks with explicit line anchors (` + "`@@ -old,+new`" + `) and numbered lines.
- Do not claim that hunk line numbers are missing.
- Anchor each finding to the most precise changed line available.
`
	return prompt + block
}

func appendStructuredOutputInstructions(prompt string) string {
	const block = `
## Output Format (STRICT JSON)
Return valid JSON only (no markdown) using this schema:
{
  "summary": "2-3 sentence summary",
  "findings": [
    {
      "file_path": "path/to/file.ext",
      "line": 123,
      "kind": "ISSUE|SUGGESTION|REMARK",
      "severity": "CRITICAL|HIGH|MEDIUM|LOW",
      "message": "concise actionable finding",
      "suggestion": "optional replacement code"
    }
  ]
}
If no findings, return {"summary":"...","findings":[]}.
`
	return prompt + block
}

type reviewBaseline struct {
	HeadSHA  string            `json:"head_sha"`
	FileSigs map[string]string `json:"file_sigs"`
}

func buildFileSignatures(changes []diffparse.FileChange) map[string]string {
	out := make(map[string]string, len(changes))
	for _, c := range changes {
		path := strings.TrimSpace(c.NewName)
		if path == "" {
			path = strings.TrimSpace(c.OldName)
		}
		if path == "" {
			continue
		}
		out[path] = fileChangeSignature(c)
	}
	return out
}

func fileChangeSignature(c diffparse.FileChange) string {
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(c.NewName))
	sb.WriteString("|")
	sb.WriteString(strings.TrimSpace(c.OldName))
	for _, h := range c.Hunks {
		sb.WriteString(fmt.Sprintf("|%d:%d:%d:%d", h.OldStart, h.OldLines, h.NewStart, h.NewLines))
		for _, l := range h.Lines {
			sb.WriteString(fmt.Sprintf("|%d:%d:%d:%s", l.Type, l.OldLineNo, l.NewLineNo, l.Content))
		}
	}
	sum := sha1.Sum([]byte(sb.String()))
	return fmt.Sprintf("%x", sum[:])
}

func filterChangesByBaseline(changes []diffparse.FileChange, baseline map[string]string) []diffparse.FileChange {
	if len(changes) == 0 || len(baseline) == 0 {
		return changes
	}
	out := make([]diffparse.FileChange, 0, len(changes))
	for _, c := range changes {
		path := strings.TrimSpace(c.NewName)
		if path == "" {
			path = strings.TrimSpace(c.OldName)
		}
		if path == "" {
			continue
		}
		sig := fileChangeSignature(c)
		if baseline[path] != sig {
			out = append(out, c)
		}
	}
	return out
}

func latestReviewBaseline(notes []vcs.MRNote) (reviewBaseline, bool) {
	for i := len(notes) - 1; i >= 0; i-- {
		body := strings.TrimSpace(notes[i].Body)
		idx := strings.Index(body, prevBaselinePrefix)
		if idx < 0 {
			continue
		}
		start := idx + len(prevBaselinePrefix)
		end := strings.Index(body[start:], "-->")
		if end < 0 {
			continue
		}
		encoded := strings.TrimSpace(body[start : start+end])
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			continue
		}
		var parsed reviewBaseline
		if err := json.Unmarshal(raw, &parsed); err != nil {
			continue
		}
		if parsed.HeadSHA == "" {
			continue
		}
		if parsed.FileSigs == nil {
			parsed.FileSigs = map[string]string{}
		}
		return parsed, true
	}
	return reviewBaseline{}, false
}

func postReviewBaseline(vcsProvider vcs.VCSProvider, projectID string, mrIID int64, baseline reviewBaseline) error {
	if strings.TrimSpace(baseline.HeadSHA) == "" {
		return nil
	}
	raw, err := json.Marshal(baseline)
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	body := prevBaselinePrefix + encoded + " -->"
	return vcsProvider.PostSummaryNote(projectID, mrIID, body)
}

func isDocTextFile(path string) bool {
	p := strings.ToLower(strings.TrimSpace(path))
	if p == "" {
		return false
	}
	switch {
	case strings.HasSuffix(p, ".md"),
		strings.HasSuffix(p, ".markdown"),
		strings.HasSuffix(p, ".txt"),
		strings.HasSuffix(p, ".rst"),
		strings.HasSuffix(p, ".adoc"):
		return true
	default:
		return false
	}
}

func isLikelyTypoComment(message string) bool {
	m := strings.ToLower(strings.TrimSpace(message))
	if m == "" {
		return false
	}
	terms := []string{
		"typo", "spelling", "misspell", "grammar", "punctuation",
		"wording", "capitalization", "capitalisation", "whitespace",
	}
	for _, t := range terms {
		if strings.Contains(m, t) {
			return true
		}
	}
	return false
}

func detectDeterministicFindings(changes []diffparse.FileChange) []core.FileComment {
	var out []core.FileComment
	seen := map[string]struct{}{}
	for _, c := range changes {
		filePath := strings.TrimSpace(c.NewName)
		if filePath == "" {
			filePath = strings.TrimSpace(c.OldName)
		}
		if filePath == "" {
			continue
		}
		for _, h := range c.Hunks {
			for _, l := range h.Lines {
				if l.Type != diffparse.LineAdded {
					continue
				}
				lower := strings.ToLower(l.Content)
				if strings.Contains(lower, "json_dencode") {
					line := l.NewLineNo
					if line <= 0 {
						line = h.NewStart
					}
					key := strings.ToLower(filePath) + "|" + strconv.Itoa(line) + "|json_dencode"
					if _, ok := seen[key]; ok {
						continue
					}
					seen[key] = struct{}{}
					out = append(out, core.FileComment{
						FilePath: filePath,
						Line:     line,
						Kind:     "ISSUE",
						Severity: "HIGH",
						Message:  "Typo `json_dencode` likely intended as `json_encode`; this will trigger undefined function errors at runtime.",
					})
				}
			}
		}
	}
	return out
}

func hasAnyModifiedLines(changes []diffparse.FileChange) bool {
	for _, c := range changes {
		if c.IsBinary {
			continue
		}
		for _, h := range c.Hunks {
			for _, l := range h.Lines {
				if l.Type == diffparse.LineAdded || l.Type == diffparse.LineDeleted {
					return true
				}
			}
		}
	}
	return false
}

func filterOutMetaContextFindings(comments []core.FileComment) []core.FileComment {
	if len(comments) == 0 {
		return comments
	}
	out := make([]core.FileComment, 0, len(comments))
	for _, c := range comments {
		if isMetaContextFinding(c.Message) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func filterLowSignalInlineFindings(
	comments []core.FileComment,
	valid map[string]inlinePositions,
) []core.FileComment {
	if len(comments) == 0 {
		return comments
	}
	out := make([]core.FileComment, 0, len(comments))
	for _, c := range comments {
		if isLowSignalInlineFinding(c, valid) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func isLowSignalInlineFinding(c core.FileComment, valid map[string]inlinePositions) bool {
	msg := strings.TrimSpace(c.Message)
	if msg == "" {
		return false
	}
	lower := strings.ToLower(msg)
	if !looksGenericInlineFinding(lower) {
		return false
	}
	if strings.Contains(msg, "`") {
		return false
	}
	path := strings.TrimSpace(strings.TrimPrefix(c.FilePath, "./"))
	fp, ok := valid[path]
	if !ok || len(fp.content) == 0 {
		return false
	}
	tokens := anchorTokensFromMessage(msg)
	if len(tokens) == 0 {
		return true
	}
	for _, content := range fp.content {
		lc := strings.ToLower(content)
		for _, tok := range tokens {
			if strings.Contains(lc, tok) {
				return false
			}
		}
	}
	return true
}

func looksGenericInlineFinding(lowerMsg string) bool {
	patterns := []string{
		"may affect global request handling",
		"ensure backward compatibility",
		"verify all routes",
		"without corresponding validation or explanation",
		"risks breaking the pipeline",
		"altering job execution semantics",
		"please clarify what specific functionality",
	}
	for _, p := range patterns {
		if strings.Contains(lowerMsg, p) {
			return true
		}
	}
	return false
}

func isMetaContextFinding(message string) bool {
	m := strings.ToLower(strings.TrimSpace(message))
	if m == "" {
		return false
	}
	patterns := []string{
		"modified hunk content is not provided",
		"hunk content is not provided",
		"cannot be reviewed because the modified",
		"actual changed hunks",
		"diff content is not provided",
		"ci configuration changes cannot be reviewed because the modified yaml content is not included",
	}
	for _, p := range patterns {
		if strings.Contains(m, p) {
			return true
		}
	}
	return false
}

func buildMRFormattedDiffs(review *handlers.MRReview, serenaMode string, contextLines, maxTokens int) (string, error) {
	repoPath := resolveMRRepoPath()
	if repoPath == "" {
		fmt.Println("Serena: skipped (repository path unavailable); using line-based diff context.")
		return diffparse.FormatForReview(review.Changes), nil
	}

	var serenaClient *serena.Client
	var err error
	if serenaMode == "off" {
		fmt.Println("Serena: off; using line-based diff context.")
	} else {
		serenaClient, err = serena.NewClient(serenaMode)
		if err != nil {
			return "", fmt.Errorf("serena initialization failed: %w", err)
		}
		if serenaClient != nil {
			fmt.Println("Serena: active; using symbol-level context (enclosing functions/classes around hunks).")
		} else {
			fmt.Println("Serena: unavailable in auto mode; falling back to line-based diff context.")
		}
		if serenaClient != nil {
			defer serenaClient.Close()
		}
	}

	enriched, err := diffparse.EnrichFileChanges(
		review.Changes,
		repoPath,
		review.MR.TargetBranch,
		review.MR.SourceBranch,
		contextLines,
		maxTokens,
		serenaClient,
	)
	if err != nil {
		if serenaMode == "on" {
			return "", fmt.Errorf("failed to enrich MR changes with Serena context: %w", err)
		}
		fmt.Printf("Serena/context enrichment failed (%v); falling back to line-based diff context.\n", err)
		return diffparse.FormatForReview(review.Changes), nil
	}

	var sb strings.Builder
	for i, efc := range enriched {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(diffparse.FormatEnrichedForReview(efc))
	}
	out := strings.TrimSpace(sb.String())
	if out == "" {
		return diffparse.FormatForReview(review.Changes), nil
	}
	return out, nil
}

func runReviewPasses(p provider.AIProvider, basePrompt string, passes int) (string, error) {
	if passes <= 0 {
		passes = 1
	}
	currentPrompt := basePrompt
	latest := ""
	for pass := 1; pass <= passes; pass++ {
		fmt.Printf("Review pass %d/%d...\n", pass, passes)
		_, choices, err := provider.SimpleComplete(
			p,
			"You are a helpful assistant and source code reviewer.",
			"You are code reviewer for a project",
			currentPrompt,
		)
		if err != nil {
			return "", err
		}
		if len(choices) == 0 || strings.TrimSpace(choices[0]) == "" {
			return "", fmt.Errorf("no response from AI provider on pass %d", pass)
		}
		latest = choices[0]
		if pass < passes {
			currentPrompt = buildReReviewPrompt(basePrompt, latest, pass+1, passes)
		}
	}
	return latest, nil
}

func runReviewPassesDryRun(conf config.Config, basePrompt string, passes int) {
	p, err := resolveProvider(conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving provider: %v\n", err)
		os.Exit(1)
	}
	info := p.Info()
	model := conf.Model
	if model == "" {
		model = info.DefaultModel
	}
	fmt.Printf("Model: provider=%s model=%s\n", info.Name, model)
	content, err := runReviewPasses(p, basePrompt, passes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error from AI provider: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(renders.RenderMarkdown(content))
}

func buildReReviewPrompt(basePrompt, priorReview string, pass, total int) string {
	return fmt.Sprintf(`You are running review pass %d/%d.

Goal: re-review the exact same merge request context and improve quality.
- Keep all valid findings.
- Remove weak/duplicate findings.
- Add any missed high-impact issues.
- Preserve required output format from the original prompt.

Original review context:
%s

Prior pass output:
%s

Return a complete final review (not a diff against prior output).`,
		pass, total, basePrompt, priorReview)
}

func recoverInlineFindings(p provider.AIProvider, basePrompt, priorReview string) (string, error) {
	recoveryPrompt := `You must output only parseable file findings from this review context.

Requirements:
- Output only findings lines in this exact format:
  **File: path/to/file.ext** (line N) [KIND] [SEVERITY]: short message
- KIND must be one of: ISSUE, SUGGESTION, REMARK
- SEVERITY must be one of: CRITICAL, HIGH, MEDIUM, LOW
- If none found, output exactly: NO_FINDINGS
- Do not include summary/headers/tables.

Original MR review prompt:
` + basePrompt + `

Prior full review output:
` + priorReview

	_, choices, err := provider.SimpleComplete(
		p,
		"You are an expert code reviewer extracting structured findings.",
		"Extract only parseable file findings.",
		recoveryPrompt,
	)
	if err != nil {
		return "", err
	}
	if len(choices) == 0 {
		return "", fmt.Errorf("no response from AI provider")
	}
	out := strings.TrimSpace(choices[0])
	if strings.EqualFold(out, "NO_FINDINGS") {
		return "", nil
	}
	return out, nil
}

func detectVCSContextStatus(
	vcsName string,
	lookPath func(string) (string, error),
	getenv func(string) string,
) string {
	if strings.EqualFold(strings.TrimSpace(vcsName), "github") {
		return "GitHub context: using standard GitHub API + Serena/local context enrichment."
	}
	if url := strings.TrimSpace(getenv("GITLAB_MCP_URL")); url != "" {
		return fmt.Sprintf("GitLab MCP: configured via GITLAB_MCP_URL (%s).", url)
	}
	for _, bin := range []string{"gitlab-mcp", "glab-mcp"} {
		if p, err := lookPath(bin); err == nil && strings.TrimSpace(p) != "" {
			return fmt.Sprintf("GitLab MCP: detected local server binary (%s).", bin)
		}
	}
	return "GitLab MCP: not detected/configured; using standard GitLab API + Serena/local context enrichment."
}

func resolveMRRepoPath() string {
	if p := strings.TrimSpace(os.Getenv("CI_PROJECT_DIR")); p != "" {
		return p
	}
	p, err := os.Getwd()
	if err != nil {
		return ""
	}
	return p
}

func newMRDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "diff <project_id> <mr_iid>",
		Short:   "Show MR diff locally (no AI)",
		Example: "prev mr diff my-group/my-project 42",
		Args:    cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			projectID := args[0]
			mrIID, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: invalid MR IID %q: %v\n", args[1], err)
				os.Exit(1)
			}

			vcsProvider, err := resolveVCSProvider(cmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			review, err := handlers.ExtractMRHandlerWithOptions(
				vcsProvider, projectID, mrIID, "normal",
				handlers.MRExtractOptions{
					DiffSource: "auto",
					RepoPath:   resolveMRRepoPath(),
				},
			)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("MR !%d: %s (%s -> %s)\n\n",
				review.MR.IID, review.MR.Title,
				review.MR.SourceBranch, review.MR.TargetBranch)

			for _, fc := range review.Changes {
				name := fc.NewName
				if name == "" {
					name = fc.OldName
				}
				fmt.Printf("--- %s (+%d/-%d)\n", name, fc.Stats.Additions, fc.Stats.Deletions)
			}
		},
	}
}

func newMRListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list <project_id>",
		Short:   "List open Merge Requests",
		Example: "prev mr list my-group/my-project",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			projectID := args[0]

			vcsProvider, err := resolveVCSProvider(cmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			mrs, err := vcsProvider.ListOpenMRs(projectID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if len(mrs) == 0 {
				fmt.Println("No open merge requests.")
				return
			}

			fmt.Printf("Open Merge Requests for %s:\n\n", projectID)
			for _, mr := range mrs {
				fmt.Printf("  !%-5d %-50s (%s -> %s) by @%s\n",
					mr.IID, mr.Title, mr.SourceBranch, mr.TargetBranch, mr.Author)
			}
		},
	}
}
