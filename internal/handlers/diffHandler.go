package handlers

import (
	"strings"

	common "github.com/sanix-darker/prev/internal/common"
	config "github.com/sanix-darker/prev/internal/config"
	core "github.com/sanix-darker/prev/internal/core"
)

// ExtractDiffHandler: this handler just extract the diff changes from a file1,file2 argument
func ExtractDiffHandler(
	conf config.Config,
	inputString string,
	helper func() error,
) ([]string, error) {

	inputParts := strings.Split(inputString, ",")
	if len(inputParts) < 2 {
		common.LogError(
			"[x] Please provide two files seperated by a comma.",
			true,
			true,
			helper,
		)
	}

	file1, file2 := common.ExtractPaths(
		inputParts[0], helper,
	), common.ExtractPaths(
		inputParts[1], helper,
	)

	if len(file1) == 0 || len(file2) == 0 {
		common.LogError(
			"[x] Please provide two valids files.",
			true,
			true,
			helper,
		)
	}

	diffList, _ := core.BuildDiff(file1[0], file2[0])
	for _, d := range diffList {
		common.LogInfo(d, nil)
	}
	common.LogInfo(file1[0], nil)
	common.LogInfo(file2[0], nil)

	return diffList, nil
}
