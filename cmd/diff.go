package cmd

import (
	"github.com/sanix-darker/prev/internal/apis"
	common "github.com/sanix-darker/prev/internal/common"
	config "github.com/sanix-darker/prev/internal/config"
	core "github.com/sanix-darker/prev/internal/core"
	handlers "github.com/sanix-darker/prev/internal/handlers"

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

			d, err := handlers.ExtractDiffHandler(
				conf,
				args[0],
				cmd.Help,
			)
			if err != nil {
				common.LogError(err.Error(), true, false, nil)
			}

			prompt := core.BuildReviewPrompt(
				conf,
				d,
			)

			if conf.Debug {
				common.LogInfo(prompt, nil)
			}

			// TODO: add this inside another util that will need a config param
			// to chose the handler directly here, we should not use chatGPT from
			// here, this will help doing more funcionnal programming.

			apis.ApiCall(
				conf,
				prompt,
				apis.ChatGptHandler, // TODO: again this should depend on the prev use command
			)
		},
	}

	return diffCmd
}
