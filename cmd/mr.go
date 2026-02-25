package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/sanix-darker/prev/internal/core"
	"github.com/sanix-darker/prev/internal/handlers"
	"github.com/sanix-darker/prev/internal/provider"
	"github.com/sanix-darker/prev/internal/renders"
	"github.com/sanix-darker/prev/internal/vcs"
	"github.com/spf13/cobra"
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

	var token, baseURL string

	switch vcsName {
	case "gitlab":
		token, _ = cmd.Flags().GetString("gitlab-token")
		baseURL, _ = cmd.Flags().GetString("gitlab-url")
		if token == "" {
			token = os.Getenv("GITLAB_TOKEN")
		}
		if baseURL == "" {
			baseURL = os.Getenv("GITLAB_URL")
		}
	case "github":
		token, _ = cmd.Flags().GetString("github-token")
		baseURL, _ = cmd.Flags().GetString("github-url")
		if token == "" {
			token = os.Getenv("GITHUB_TOKEN")
		}
		if baseURL == "" {
			baseURL = os.Getenv("GITHUB_API_URL")
		}
	default:
		token, _ = cmd.Flags().GetString("gitlab-token")
		baseURL, _ = cmd.Flags().GetString("gitlab-url")
		if token == "" {
			token = os.Getenv("GITLAB_TOKEN")
		}
		if baseURL == "" {
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
			strictness, _ := cmd.Flags().GetString("strictness")
			if strictness == "" {
				strictness = conf.Strictness
			}

			vcsProvider, err := resolveVCSProvider(cmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			review, err := handlers.ExtractMRHandler(vcsProvider, projectID, mrIID, strictness)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Reviewing MR !%d: %s (%s -> %s)\n",
				review.MR.IID, review.MR.Title,
				review.MR.SourceBranch, review.MR.TargetBranch)
			fmt.Printf("Files changed: %d\n\n", len(review.Changes))

			if dryRun {
				callProvider(conf, review.Prompt)
				return
			}

			// Get AI review via blocking call
			p, err := resolveProvider(conf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error resolving provider: %v\n", err)
				os.Exit(1)
			}

			_, choices, err := provider.SimpleComplete(
				p,
				"You are a helpful assistant and source code reviewer.",
				"You are code reviewer for a project",
				review.Prompt,
			)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error from AI provider: %v\n", err)
				os.Exit(1)
			}

			if len(choices) == 0 {
				fmt.Fprintln(os.Stderr, "No response from AI provider")
				os.Exit(1)
			}

			reviewContent := choices[0]
			fmt.Print(renders.RenderMarkdown(reviewContent))

			// Post to VCS
			parsed := core.ParseReviewResponse(reviewContent)

			summaryBody := fmt.Sprintf("## AI Code Review\n\n%s", parsed.Summary)
			if err := vcsProvider.PostSummaryNote(projectID, mrIID, summaryBody); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to post summary note: %v\n", err)
			} else {
				fmt.Println("\nPosted summary comment to MR.")
			}

			// Post inline comments (if not summary-only)
			if !summaryOnly && review.MR.DiffRefs.BaseSHA != "" {
				fileComments := core.FilterBySeverity(parsed.FileComments, strictness)
				for _, fc := range fileComments {
					if fc.Line <= 0 {
						continue
					}
					body := fmt.Sprintf("[%s] %s", fc.Severity, fc.Message)
					if fc.Suggestion != "" {
						body += "\n\n" + vcsProvider.FormatSuggestionBlock(fc.Suggestion)
					}
					err := vcsProvider.PostInlineComment(
						projectID, mrIID,
						review.MR.DiffRefs,
						vcs.InlineComment{
							FilePath: fc.FilePath,
							NewLine:  int64(fc.Line),
							Body:     body,
						},
					)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to post inline comment on %s:%d: %v\n",
							fc.FilePath, fc.Line, err)
					}
				}
				if len(fileComments) > 0 {
					fmt.Printf("Posted %d inline comments.\n", len(fileComments))
				}
			}
		},
	}

	cmd.Flags().Bool("dry-run", false, "Print review without posting to VCS")
	cmd.Flags().Bool("summary-only", false, "Post only a summary comment, no inline comments")
	cmd.Flags().String("gitlab-token", "", "GitLab personal access token (or use GITLAB_TOKEN env)")
	cmd.Flags().String("gitlab-url", "", "GitLab instance URL (or use GITLAB_URL env, default: https://gitlab.com)")
	cmd.Flags().String("github-token", "", "GitHub token (or use GITHUB_TOKEN env)")
	cmd.Flags().String("github-url", "", "GitHub API base URL (or use GITHUB_API_URL env, default: https://api.github.com)")
	cmd.Flags().String("vcs", "", "VCS provider (gitlab, github; auto-detected from env)")
	cmd.Flags().String("strictness", "", "Review strictness: strict, normal, lenient (default: normal)")

	return cmd
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

			review, err := handlers.ExtractMRHandler(vcsProvider, projectID, mrIID, "normal")
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
