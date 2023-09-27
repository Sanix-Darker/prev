/*
Copyright © 2023 sanix-darker <s4nixd@gmail.com>

The main review module that handle:
- diff :given to files or a set of changes, will review it  for your depending on the chosed API.
- optim: given the code inside your keyboard or a file, it will give you the most optimized version of it.
- git-based-eval: given a branch name and a repository.
  - branch : it will review the changes difference from the base branch.
  - commit : it will review the changes difference from the base branch.
*/

package cmd

import (
	config "github.com/sanix-darker/prev/internal/config"
	models "github.com/sanix-darker/prev/internal/models"
	"github.com/spf13/cobra"
)

func addRepoAndPathFlags(commands []*cobra.Command) {
	for _, cmd := range commands {
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
}

func init() {
	conf := config.NewDefaultConfig()
	rootCmd.AddCommand(NewBranchCmd(conf), NewCommitCmd(conf))

	// Set common flags smartly (repo, paths)
	addRepoAndPathFlags(rootCmd.Commands())

	// diff and optim commands
	rootCmd.AddCommand(NewDiffCmd(conf), NewOptimizeCmd(conf))
}
