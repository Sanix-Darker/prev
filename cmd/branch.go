package cmd

import (
	"strings"

	common "github.com/sanix-darker/prev/internal/common"
	config "github.com/sanix-darker/prev/internal/config"
	core "github.com/sanix-darker/prev/internal/core"
	handlers "github.com/sanix-darker/prev/internal/handlers"

	"github.com/spf13/cobra"
)

func NewBranchCmd(conf config.Config) *cobra.Command {
	// branchCmd represents the branch for the command
	branchCmd := &cobra.Command{
		Use:     "branch <branchName> [--repo] [-p --path]...",
		Short:   "Select a branch from your .git repo(local or remote)",
		Example: "prev branch f/hot-fix --repo /path/to/git/project\nprev branch f/hight-feat --repo /path/to/git/project -p Cargo.toml,lib/eraser.rs",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			common.CheckArgs("branch", args, cmd.Help)

			branchName, repoPath, gitPath := common.ExtractTargetRepoAndGitPath(
				conf,
				args,
				cmd.Flags(),
				cmd.Help,
			)

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
		},
	}

	return branchCmd
}
