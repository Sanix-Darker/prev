/*
Copyright © 2023 sanix-darker <s4nixd@gmail.com>
*/
package cmd

import (
	"fmt"
	"strings"

	"github.com/sanix-darker/prev/models"
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
	Short:   "Get/check/review diff between two files changes (not git related).",
	Example: "prev review diff code_ok.py,code_bad.py",
	Run: func(cmd *cobra.Command, args []string) {
		CheckArgs("review diff", args, cmd.Help)

		file1, file2 := strings.Split(args[0], ",")[0], strings.Split(args[0], ",")[1]

		// get difference between two files and save it into an array of difference
		// that are going to be concatenate to a prompt for the core

		// fmt.Println(repoPath)
		fmt.Println(file1)
		fmt.Println(file2)
	},
}

// commitCmd represents the commit for the command
var commitCmd = &cobra.Command{
	Use:     "commit <commitHash> [--repo] [-p --path]...",
	Short:   "Select a commit from your .git repository(local or remote)",
	Example: "prev review commit 44rtff55g --repo /path/to/git/project\nprev review commit 867abbeef --repo /path/to/git/project -p app/main.py,tests/",
	Run: func(cmd *cobra.Command, args []string) {
		CheckArgs("review commit", args, cmd.Help)
		commitHash := args[0]

		cmdFlags := cmd.Flags()
		repoPath := GetArgByKey("repo", cmdFlags, true)
		// list of multiple files
		// we need to identifiy a file from a directory
		// and also an be n array of paths
		gitPath := GetArgByKey("path", cmdFlags, false)

		fmt.Println(repoPath)
		fmt.Println(gitPath)
		fmt.Println(commitHash)
	},
}

// branchCmd represents the branch for the command
var branchCmd = &cobra.Command{
	Use:     "branch <branchName> [--repo] [-p --path]...",
	Short:   "Select a branch from your .git repository(local or remote)",
	Example: "prev review branch f/hot-fix --repo /path/to/git/project\nprev review branch f/hight-feat --repo /path/to/git/project -p Cargo.toml,lib/eraser.rs",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		CheckArgs("review branch", args, cmd.Help)
		branchName := args[0]

		repoPath, _ := cmd.Flags().GetString("repo")

		fmt.Println(repoPath)
		fmt.Println(branchName)
	},
}

// reviewCmd represents the review command
var reviewCmd = &cobra.Command{
	Use:       "review [branch | commit | diff] [-rp --repo]...",
	Short:     "Review changes on a .git repository(local or remote)",
	Example:   "prev review branch f/hot-fix -r /path/to/git/project\nprev review commit 867abbeef -r /path/to/git/project -p Makefile\nprev review diff previous.txt,next.txt",
	ValidArgs: []string{"branch", "commit", "diff"},
	Run:       func(cmd *cobra.Command, args []string) {},
}

func init() {
	reviewCmd.AddCommand(branchCmd, commitCmd, diffCmd)
	rootCmd.AddCommand(reviewCmd)

	// set flags smartly
	for _, cmd := range reviewCmd.Commands() {
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
