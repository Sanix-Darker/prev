/*
Copyright © 2023 sanix-darker <s4nixd@gmail.com>
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
)

type HelpCallback func() error

// PrintError: to print an error message
// in case of "critic" at true, the program will stop on code 0
func PrintError(
	message string,
	critic bool,
	help_menu bool,
	help_callback HelpCallback,
) {
	fmt.Printf(message + "\n")

	if critic {
		if help_menu {
			help_callback()
		}
		os.Exit(0)
	}
}

// ExtractTargetRepoAndGitPath; extract target (commmitHash/branchName)
// , the .git path and the repo path
func ExtractTargetRepoAndGitPath(
	args []string,
	cmdFlags *pflag.FlagSet,
	help HelpCallback,
) (string, string, string) {
	targetHash := args[0]
	repoPath, gitPath := GetRepoPathAndTargetPath(cmdFlags, help)
	return targetHash, repoPath, gitPath
}

// just return the repo and the target path from
func GetRepoPathAndTargetPath(cmdFlags *pflag.FlagSet, help HelpCallback) (string, string) {
	repoPath, _ := cmdFlags.GetString("repo")
	gitPath := ExtractPaths(GetArgByKey("path", cmdFlags, false, help), help)

	if len(gitPath) == 0 {
		gitPath = []string{""}
	}

	return repoPath, gitPath[0]
}

// CheckArgs: check arguments are correctly passed then help callback if not
func CheckArgs(keycommand string, args []string, help HelpCallback) {
	if len(args) == 0 {
		PrintError("", true, true, help)
	}
}

// GetArgByKey get an argument value based on a key input + a strict mode for required params
func GetArgByKey(
	key string,
	cmdFlags *pflag.FlagSet,
	strictMode bool,
	help HelpCallback,
) string {
	value, err := cmdFlags.GetString(key)
	if strictMode && err != nil {
		msg := fmt.Sprintf("[x] %v, is not set and is required for your command.\n", key)
		PrintError(msg, true, true, help)
	}
	return value
}

// ExtractPaths for a give path (like a glob),we want a full path from it
// either it's a dir, or a file with any kind of extension
func ExtractPaths(path string, help HelpCallback) []string {
	var files []string

	paths := strings.Split(path, ",")

	for _, p := range paths {
		p = strings.TrimSpace(p)

		// Check if the path contains wildcards like *.py
		if strings.Contains(p, "*") {
			matches, err := filepath.Glob(p)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			files = append(files, matches...)
		} else {
			// maintenatn, check if it's a directory
			info, err := os.Stat(p)
			if err != nil {
				fmt.Printf("[x] Error: %v\n", err)
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
					PrintError(msg, true, true, help)
				}
			} else {
				files = append(files, p)
			}
		}
	}

	return files
}
