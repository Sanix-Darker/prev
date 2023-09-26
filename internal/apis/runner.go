package apis

import (
	"fmt"

	"github.com/sanix-darker/prev/internal/common"
	"github.com/sanix-darker/prev/internal/config"
	"github.com/sanix-darker/prev/internal/renders"
)

func ApiCall(
	conf config.Config,
	prompt string,
	callback func(string, string) (string, []string, error),
) {

	chatId, responses, err := callback("You're a software engineer", prompt)
	if err != nil {
		common.LogError(err.Error(), true, false, nil)
	}

	if conf.Debug {
		common.LogInfo(fmt.Sprintf("> chatId: %v\n", chatId), nil)
		common.LogInfo(fmt.Sprintf("> responses: %d\n", len(responses)), nil)
	}

	for _, resp := range responses {
		if conf.Debug {
			common.LogInfo("> review: ", nil)
		}
		// common.LogInfo(resp, nil)
		fmt.Print(renders.RenderMarkdown(resp))
	}
}
