package cmd

import (
	"strings"

	common "github.com/sanix-darker/prev/internal/common"
	config "github.com/sanix-darker/prev/internal/config"
	core "github.com/sanix-darker/prev/internal/core"
	handlers "github.com/sanix-darker/prev/internal/handlers"

	"github.com/spf13/cobra"
)

// NewDiffCmd: add a new diff command
func NewDiffCmd(conf config.Config) *cobra.Command {
	diffCmd := &cobra.Command{
		Use:     "diff <file1,file2>...",
		Short:   "review diff between two files changes (not git related).",
		Example: "prev diff code_ok.py,code_bad.py",
		Run: func(cmd *cobra.Command, args []string) {
			applyFlags(cmd, &conf)
			common.CheckArgs("diff", args, cmd.Help)

			d, err := handlers.ExtractDiffHandler(
				conf,
				args[0],
				cmd.Help,
			)
			if err != nil {
				common.LogError(err.Error(), true, false, nil)
			}

			configGuidelines := ""
			if conf.Viper != nil {
				configGuidelines = strings.TrimSpace(conf.Viper.GetString("review.guidelines"))
			}
			prompt := core.BuildReviewPrompt(
				conf,
				d,
				mergeGuidelines(
					configGuidelines,
					repoGuidelineSection(guidelineRootForDiffInput(args[0])),
				),
			)

			if conf.Debug {
				common.LogInfo(prompt, nil)
			}

			callProvider(conf, prompt)
		},
	}

	return diffCmd
}
