package cmd

import (
	"fmt"
	"os"
	"strings"

	common "github.com/sanix-darker/prev/internal/common"
	config "github.com/sanix-darker/prev/internal/config"
	core "github.com/sanix-darker/prev/internal/core"
	handlers "github.com/sanix-darker/prev/internal/handlers"
	"github.com/sanix-darker/prev/internal/renders"
	"github.com/sanix-darker/prev/internal/review"

	"github.com/spf13/cobra"
)

func NewBranchCmd(conf config.Config) *cobra.Command {
	branchCmd := &cobra.Command{
		Use:     "branch <branchName> [--repo] [-p --path]...",
		Short:   "Select a branch from your .git repo (local or remote)",
		Example: "prev branch f/hot-fix --repo /path/to/git/project\nprev branch f/hight-feat --repo /path/to/git/project -p Cargo.toml,lib/eraser.rs",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			applyFlags(cmd, &conf)
			common.CheckArgs("branch", args, cmd.Help)

			branchName, repoPath, gitPath := common.ExtractTargetRepoAndGitPath(
				conf,
				args,
				cmd.Flags(),
				cmd.Help,
			)

			legacy, _ := cmd.Flags().GetBool("legacy")
			if legacy {
				runLegacyBranch(conf, branchName, repoPath, gitPath, cmd)
				return
			}

			runEnhancedBranch(conf, cmd, branchName, repoPath, gitPath)
		},
	}

	branchCmd.Flags().Int("context", 10, "Number of surrounding context lines for review")
	branchCmd.Flags().Int("max-tokens", 80000, "Maximum tokens per AI batch")
	branchCmd.Flags().Bool("per-commit", false, "Review each commit individually")
	branchCmd.Flags().Bool("legacy", false, "Use legacy single-prompt review mode")
	branchCmd.Flags().String("serena", "auto", "Serena mode: auto, on, off")

	return branchCmd
}

func runLegacyBranch(conf config.Config, branchName, repoPath, gitPath string, cmd *cobra.Command) {
	d, err := handlers.ExtractBranchHandler(
		branchName,
		repoPath,
		gitPath,
		cmd.Help,
	)
	if err != nil {
		common.LogError(err.Error(), true, false, nil)
	}
	prompt := core.BuildReviewPrompt(
		conf,
		strings.Join(d, "\n-----------------------------------------\n"),
	)
	if conf.Debug {
		common.LogInfo(prompt, nil)
	}
	callProvider(conf, prompt)
}

func runEnhancedBranch(conf config.Config, cmd *cobra.Command, branchName, repoPath, gitPath string) {
	p, err := resolveProvider(conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving provider: %v\n", err)
		os.Exit(1)
	}

	contextLines, _ := cmd.Flags().GetInt("context")
	maxTokens, _ := cmd.Flags().GetInt("max-tokens")
	serenaMode, _ := cmd.Flags().GetString("serena")

	cfg := review.ReviewConfig{
		ContextLines:   contextLines,
		MaxBatchTokens: maxTokens,
		Strictness:     conf.Strictness,
		SerenaMode:     serenaMode,
		Debug:          conf.Debug,
	}

	// Progress callback for spinner updates
	onProgress := func(stage string, current, total int) {
		if total > 0 {
			fmt.Fprintf(os.Stderr, "[%s] %d/%d\n", stage, current, total)
		} else {
			fmt.Fprintf(os.Stderr, "[%s]\n", stage)
		}
	}

	result, err := handlers.ExtractBranchReview(p, branchName, repoPath, gitPath, cfg, onProgress)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	output := review.FormatBranchReview(result)
	fmt.Print(renders.RenderMarkdown(output))
}
