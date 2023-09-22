package handlers

import (
	"errors"

	common "github.com/sanix-darker/prev/common"
)

func ExtractCommitHandler(
	commitHash string,
	repoPath string,
	gitPath string,
	help common.HelpCallback,
) ([]string, error) {

	return []string{}, errors.New("")
}
