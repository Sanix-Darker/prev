package cmd

import (
	"strings"

	common "github.com/sanix-darker/prev/internal/common"
	config "github.com/sanix-darker/prev/internal/config"
	core "github.com/sanix-darker/prev/internal/core"
	handlers "github.com/sanix-darker/prev/internal/handlers"

	"github.com/spf13/cobra"
)

// NewCommitCmd for a given repo and commit hash, will provide a review from it
func NewCommitCmd(conf config.Config) *cobra.Command {
	commitCmd := &cobra.Command{
		Use:     "commit <commitHash> [--repo] [-p --path]...",
		Short:   "Select a commit from a .git repo (local or remote)",
		Example: "prev commit 44rtff55g --repo /path/to/git/project\nprev commit 867abbeef --repo /path/to/git/project -p app/main.py,tests/",
		Run: func(cmd *cobra.Command, args []string) {
			applyFlags(cmd, &conf)
			common.CheckArgs("commit", args, cmd.Help)

			commitHash, repoPath, gitPath := common.ExtractTargetRepoAndGitPath(
				conf,
				args,
				cmd.Flags(),
				cmd.Help,
			)

			d, err := handlers.ExtractCommitHandler(
				conf,
				commitHash,
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
		},
	}

	return commitCmd
}
