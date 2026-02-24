package handlers

import (
	"fmt"
	"strings"

	common "github.com/sanix-darker/prev/internal/common"
	"github.com/sanix-darker/prev/internal/config"
	core "github.com/sanix-darker/prev/internal/core"
)

func ExtractOptimHandler(
	conf config.Config,
	args []string,
	help func() error,
) (string, error) {
	if len(args) == 0 {
		// if no arguments, we get from clipboard
		clipValue, err := common.GetClipboardValue()

		return core.BuildOptimPrompt(conf, clipValue), err
	} else {
		// or we take from the first argument
		lines, err := core.ReadFileLines(args[0])
		fileContent := strings.Join(lines, "\n")

		if len(fileContent) < 1 {
			common.LogError(
				fmt.Sprintf(
					"[x] File content empty, please add code on %s (%d chars found)!",
					args[0],
					len(fileContent),
				),
				true,
				true,
				help,
			)
		}

		return core.BuildOptimPrompt(conf, fileContent), err
	}
}
