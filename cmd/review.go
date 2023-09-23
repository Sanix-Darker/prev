/*
Copyright © 2023 sanix-darker <s4nixd@gmail.com>

The main review module that handle:
- diff :given to files or a set of changes, will review it  for your depending on the chosed API.
- git-based-eval: given a branch name and a repository.
  - branch : it will review the changes difference from the base branch.
  - commit : it will review the changes difference from the base branch.
*/
package cmd

import (
	"fmt"
	"strings"

	common "github.com/sanix-darker/prev/common"
	"github.com/sanix-darker/prev/core"
	handlers "github.com/sanix-darker/prev/handlers"
	models "github.com/sanix-darker/prev/models"
	"github.com/spf13/cobra"
)

// FIXME: hashMap for flags (is this dirty ?)
var ReviewFlags = []models.FlagStruct{
	{
		Label:        "repo",
		Short:        "r",
		DefaultValue: ".",
		Description:  "target git repo (loca-path/git-url).",
	},
	{
		Label:        "path",
		Short:        "p",
		DefaultValue: ".",
		Description:  "target file/directory to inspect",
	},
}

// diffCmd represents the diffCmd for the command
var diffCmd = &cobra.Command{
	Use:     "diff <file1,file2>...",
	Short:   "review diff between two files changes (not git related).",
	Example: "prev diff code_ok.py,code_bad.py",
	Run: func(cmd *cobra.Command, args []string) {
		common.CheckArgs("diff", args, cmd.Help)

		d, err := handlers.ExtractDiffHandler(
			args[0],
			cmd.Help,
			false,
		)
		if err != nil {
			// common.PrintError
		}
		prompt := core.BuildPrompt(strings.Join(d, "\n"), 500, 5)
		fmt.Println(prompt)
	},
}

// commitCmd represents the commit for the command
var commitCmd = &cobra.Command{
	Use:     "commit <commitHash> [--repo] [-p --path]...",
	Short:   "Select a commit from a .git repo(local or remote)",
	Example: "prev commit 44rtff55g --repo /path/to/git/project\nprev commit 867abbeef --repo /path/to/git/project -p app/main.py,tests/",
	Run: func(cmd *cobra.Command, args []string) {
		common.CheckArgs("commit", args, cmd.Help)

		commitHash, repoPath, gitPath := common.ExtractTargetRepoAndGitPath(args,
			cmd.Flags(),
			cmd.Help,
		)

		d, err := handlers.ExtractCommitHandler(
			commitHash,
			repoPath,
			gitPath,
			cmd.Help,
		)

		if err != nil {
			// common.PrintError
		}
		prompt := core.BuildPrompt(strings.Join(d, "\n"), 500, 5)
		fmt.Println(prompt)
	},
}

// branchCmd represents the branch for the command
var branchCmd = &cobra.Command{
	Use:     "branch <branchName> [--repo] [-p --path]...",
	Short:   "Select a branch from your .git repo(local or remote)",
	Example: "prev branch f/hot-fix --repo /path/to/git/project\nprev branch f/hight-feat --repo /path/to/git/project -p Cargo.toml,lib/eraser.rs",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		common.CheckArgs("branch", args, cmd.Help)

		branchName, repoPath, gitPath := common.ExtractTargetRepoAndGitPath(args,
			cmd.Flags(),
			cmd.Help,
		)

		d, err := handlers.ExtractHashHandler(
			branchName,
			repoPath,
			gitPath,
			cmd.Help,
		)

		if err != nil {
			// common.PrintError
		}
		prompt := core.BuildPrompt(
			strings.Join(d, "\n-----------------------------------------\n"),
			500,
			5,
		)
		fmt.Println(prompt)
	},
}

func init() {
	rootCmd.AddCommand(branchCmd, commitCmd, diffCmd)

	// set flags smartly
	for _, cmd := range rootCmd.Commands() {
		if cmd != diffCmd { // those are not needed for diffCmd
			for _, fg := range ReviewFlags {
				cmd.PersistentFlags().StringP(
					fg.Label,
					fg.Short,
					fg.DefaultValue,
					fg.Description,
				)
			}
		}
	}
}
