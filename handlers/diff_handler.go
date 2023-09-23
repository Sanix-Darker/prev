package handlers

import (
	"fmt"
	"strings"

	common "github.com/sanix-darker/prev/common"
	core "github.com/sanix-darker/prev/core"
)

// ExtractDiffHandler: this handler just extract the diff changes from a file1,file2 argument
func ExtractDiffHandler(
	inputString string,
	helper func() error,
	debug bool,
) ([]string, error) {

	inputParts := strings.Split(inputString, ",")
	if len(inputParts) < 2 {
		common.PrintError(
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
		common.PrintError(
			"[x] Please provide two valids files.",
			true,
			true,
			helper,
		)
	}

	diffList, _ := core.BuildDiff(file1[0], file2[0])
	if debug {
		for _, d := range diffList {
			fmt.Println(d)
		}
	}

	fmt.Println(file1[0])
	fmt.Println(file2[0])

	return diffList, nil
}
