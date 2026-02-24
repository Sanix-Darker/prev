package cmd

import (
	config "github.com/sanix-darker/prev/internal/config"
	models "github.com/sanix-darker/prev/internal/models"
	_ "github.com/sanix-darker/prev/internal/provider/init"
	_ "github.com/sanix-darker/prev/internal/vcs/init"
	"github.com/spf13/cobra"
)

func addRepoAndPathFlags(commands []*cobra.Command) {
	for _, cmd := range commands {
		for _, fg := range []models.FlagStruct{
			{
				Label:        "repo",
				Short:        "r",
				DefaultValue: ".",
				Description:  "target git repo (local-path/git-url).",
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

// applyFlags reads CLI flags and applies them to the config.
func applyFlags(cmd *cobra.Command, conf *config.Config) {
	if p, _ := cmd.Flags().GetString("provider"); p != "" {
		conf.Provider = p
	}
	if m, _ := cmd.Flags().GetString("model"); m != "" {
		conf.Model = m
	}
	if d, _ := cmd.Flags().GetBool("debug"); d {
		conf.Debug = true
	}
	if s, err := cmd.Flags().GetBool("stream"); err == nil {
		conf.Stream = s
	}
	if st, _ := cmd.Flags().GetString("strictness"); st != "" {
		conf.Strictness = st
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
