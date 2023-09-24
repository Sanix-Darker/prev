package handlers

import (
	"errors"

	"github.com/sanix-darker/prev/internal/config"
)

func ExtractCommitHandler(
	conf config.Config,
	commitHash string,
	repoPath string,
	gitPath string,
	help func() error,
) ([]string, error) {

	return []string{}, errors.New("")
}
