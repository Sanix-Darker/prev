/*
Copyright © 2023 sanix-darker <s4nixd@gmail.com>
*/
package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/spf13/pflag"
)

// LogError: to print an error message
// in case of "critic" at true, the program will stop on code 0
func LogError(
	message string,
	critic bool,
	help_menu bool,
	help_callback func() error,
) {
	fmt.Fprintf(os.Stderr, "%s\n", message)

	if critic {
		if help_menu {
			help_callback()
		}
		os.Exit(1)
	}
}

// LogInfo: for a simple logging info
func LogInfo(
	message string,
	callback func(),
) {
	fmt.Printf("%s\n", message)

	// for a given callback
	if callback != nil {
		callback()
	}
}

// ExtractTargetRepoAndGitPath; extract target (commmitHash/branchName)
// , the .git path and the repo path
func ExtractTargetRepoAndGitPath(
	conf config.Config,
	args []string,
	cmdFlags *pflag.FlagSet,
	help func() error,
) (string, string, string) {
	targetHash := args[0]
	repoPath, gitPath := GetRepoPathAndTargetPath(cmdFlags, help)

	if conf.Debug {
		LogInfo(fmt.Sprintf("> repo: %v\n", repoPath), nil)
		LogInfo(fmt.Sprintf("> gitPath: %v\n", gitPath), nil)
		LogInfo(fmt.Sprintf("> targetHash: %v\n", targetHash), nil)
	}

	return targetHash, repoPath, gitPath
}

// Just return the repo and the target path from
func GetRepoPathAndTargetPath(cmdFlags *pflag.FlagSet, help func() error) (string, string) {
	repoPath, _ := cmdFlags.GetString("repo")
	gitPath := ExtractPaths(GetArgByKey("path", cmdFlags, false, help), help)

	if len(gitPath) == 0 {
		gitPath = []string{""}
	}

	return repoPath, gitPath[0]
}

// CheckArgs: check arguments are correctly passed then help callback if not
func CheckArgs(keycommand string, args []string, help func() error) {
	if len(args) == 0 {
		help()
		os.Exit(1)
	}
}

// GetArgByKey get an argument value based on a key input + a strict mode for required params
func GetArgByKey(
	key string,
	cmdFlags *pflag.FlagSet,
	strictMode bool,
	help func() error,
) string {
	value, err := cmdFlags.GetString(key)
	if strictMode && err != nil {
		msg := fmt.Sprintf("[x] %v, is not set and is required for your command.\n", key)
		LogError(msg, true, true, help)
	}
	return value
}

// ExtractPaths for a give path (like a glob),we want a full path from it
// either it's a dir, or a file with any kind of extension
func ExtractPaths(path string, help func() error) []string {
	var files []string

	paths := strings.Split(path, ",")

	for _, p := range paths {
		p = strings.TrimSpace(p)

		// Check if the path contains wildcards like *.py
		if strings.Contains(p, "*") {
			matches, err := filepath.Glob(p)
			if err != nil {
				LogError(fmt.Sprintf("[x] Error: %v\n", err), false, false, help)
				continue
			}
			files = append(files, matches...)
		} else {
			// maintenatn, check if it's a directory
			info, err := os.Stat(p)
			if err != nil {
				LogError(fmt.Sprintf("[x] Error: %v\n", err), false, false, help)
				continue
			}

			if info.IsDir() {
				// Walk through the directory and collect file paths
				err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					// Add the file path to the slice if it's not a directory
					if !info.IsDir() {
						files = append(files, path)
					}
					return nil
				})
				if err != nil {
					msg := fmt.Sprintf("[x] Error: %v\n", err)
					LogError(msg, true, true, help)
				}
			} else {
				files = append(files, p)
			}
		}
	}

	return files
}
