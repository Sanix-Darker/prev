package handlers

import (
	"errors"
)

func ExtractHashHandler(
	branchName string,
	repoPath string,
	gitPath string,
	help func() error,
	debug bool,
) ([]string, error) {

	return []string{}, errors.New("")
}
