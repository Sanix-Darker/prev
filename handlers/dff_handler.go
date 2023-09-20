package handlers

import (
	"errors"
	"fmt"
	"strings"

	common "github.com/sanix-darker/prev/common"
	core "github.com/sanix-darker/prev/core"
)

// ExtractDiffHandler: this handler just extract the diff changes from a file1,file2 argument
func ExtractDiffHandler(
	inputString string,
	helper common.Func_help_callback,
	debug bool,
) ([]string, error) {

	inputParts := strings.Split(inputString, ",")
	if len(inputParts) < 2 {
		fmt.Println("[x] Please provide two files seperated by a comma.")
		helper()
		return nil, errors.New("[x] Insufficient input parts")
	}

	file1, file2 := common.ExtractPaths(
		inputParts[0],
	), common.ExtractPaths(
		inputParts[1],
	)

	if len(file1) == 0 || len(file2) == 0 {
		fmt.Println("[x] Please provide two valids files.")
		helper()
		return nil, errors.New("[x] Insufficient input parts")
	}

	diffList, _ := core.BuildDiff(file1[0], file2[0], debug)

	fmt.Println(file1[0])
	fmt.Println(file2[0])

	return diffList, nil
}
