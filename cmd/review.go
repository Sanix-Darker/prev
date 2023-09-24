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

	"github.com/sanix-darker/prev/internal/apis"
	common "github.com/sanix-darker/prev/internal/common"
	"github.com/sanix-darker/prev/internal/config"
	"github.com/sanix-darker/prev/internal/core"
	handlers "github.com/sanix-darker/prev/internal/handlers"
	models "github.com/sanix-darker/prev/internal/models"
	"github.com/spf13/cobra"
)

// NewDiffCmd: add a new diff command
func NewDiffCmd(conf config.Config) *cobra.Command {

	// diffCmd represents the diffCmd for the command
	diffCmd := &cobra.Command{
		Use:     "diff <file1,file2>...",
		Short:   "review diff between two files changes (not git related).",
		Example: "prev diff code_ok.py,code_bad.py",
		Run: func(cmd *cobra.Command, args []string) {
			common.CheckArgs("diff", args, cmd.Help)

			common.LogInfo("> reviewing diff in progress...", nil)
			d, err := handlers.ExtractDiffHandler(
				conf,
				args[0],
				cmd.Help,
			)
			if err != nil {
				common.LogError(err.Error(), true, false, nil)
			}

			prompt := core.BuildPrompt(
				conf,
				strings.Join(d, "\n-----------------------------------------\n"),
			)

			chatId, responses, err := apis.ChatGptHandler("You're a software engineer", prompt)
			if err != nil {
				common.LogError(err.Error(), true, false, nil)
			}

			if conf.Debug {
				common.LogInfo(fmt.Sprintf("> chatId: %v\n", chatId), nil)
				common.LogInfo(fmt.Sprint("> responses: %v\n", string(len(responses))), nil)
			}

			for _, resp := range responses {
				if conf.Debug {
					common.LogInfo("> review: ", nil)
				}
				common.LogInfo(resp, nil)
				// fmt.Print(renders.RenderMarkdown(resp))
			}
		},
	}

	return diffCmd
}

func NewCommitCmd(conf config.Config) *cobra.Command {
	// commitCmd represents the commit for the command
	commitCmd := &cobra.Command{
		Use:     "commit <commitHash> [--repo] [-p --path]...",
		Short:   "Select a commit from a .git repo (local or remote)",
		Example: "prev commit 44rtff55g --repo /path/to/git/project\nprev commit 867abbeef --repo /path/to/git/project -p app/main.py,tests/",
		Run: func(cmd *cobra.Command, args []string) {
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
				// common.LogError
			}
			prompt := core.BuildPrompt(
				conf,
				strings.Join(d, "\n-----------------------------------------\n"),
			)
			fmt.Println(prompt)
		},
	}

	return commitCmd
}

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
			prompt := core.BuildPrompt(
				conf,
				strings.Join(d, "\n-----------------------------------------\n"),
			)
			fmt.Println(prompt)
		},
	}

	return branchCmd
}

func init() {

	conf := config.NewDefaultConfig()
	rootCmd.AddCommand(NewBranchCmd(conf), NewCommitCmd(conf))

	// set common flags smartly (repo, paths)
	for _, cmd := range rootCmd.Commands() {
		for _, fg := range []models.FlagStruct{
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
		} {
			cmd.PersistentFlags().StringP(
				fg.Label,
				fg.Short,
				fg.DefaultValue,
				fg.Description,
			)
		}
	}

	// diff
	rootCmd.AddCommand(NewDiffCmd(conf))
}
