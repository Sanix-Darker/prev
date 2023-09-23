package handlers

import (
	"errors"
)

func ExtractCommitHandler(
	commitHash string,
	repoPath string,
	gitPath string,
	help func() error,
) ([]string, error) {

	return []string{}, errors.New("")
}
