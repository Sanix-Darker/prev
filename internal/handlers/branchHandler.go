package handlers

import (
	"errors"
)

func ExtractBranchHandler(
	branchName string,
	repoPath string,
	gitPath string,
	help func() error,
) ([]string, error) {

	return []string{}, errors.New("")
}
