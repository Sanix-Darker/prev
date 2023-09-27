package cmd

import (
	"github.com/sanix-darker/prev/internal/apis"
	common "github.com/sanix-darker/prev/internal/common"
	config "github.com/sanix-darker/prev/internal/config"
	handlers "github.com/sanix-darker/prev/internal/handlers"

	"github.com/spf13/cobra"
)

// NewOptimizeCmd, with a given file or from your keyboard, this command will build a prompt to optimize your code.
func NewOptimizeCmd(conf config.Config) *cobra.Command {
	optimCmd := &cobra.Command{
		Use:     "optim <file>...",
		Short:   "optimize any given code or snippet.",
		Example: "prev optim code_ok.py \nprev optim # will take the input code from your clipboard",
		Run: func(cmd *cobra.Command, args []string) {

			prompt, err := handlers.ExtractOptimtHandler(
				conf,
				args,
				cmd.Help,
			)
			if err != nil {
				common.LogError(err.Error(), true, true, cmd.Help)
			}

			if conf.Debug {
				common.LogInfo("From your clipboard : ", nil)
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

	return optimCmd
}
