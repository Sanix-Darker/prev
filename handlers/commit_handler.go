package handlers

import (
	"errors"
)

func ExtractCommitHandler(
	commitHash string,
	repoPath string,
	gitPath string,
	help func() error,
	debug bool,
) ([]string, error) {

	return []string{}, errors.New("")
}
