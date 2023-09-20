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

type Func_help_callback func() error

// CheckArgs: check arguments are correctly passed then help callback if not
func CheckArgs(keycommand string, args []string, help Func_help_callback) {
	if len(args) == 0 {
		_ = help()
		os.Exit(0)
	}
}

// GetArgByKey get an argument value based on a key input + a strict mode for required params
func GetArgByKey(key string, cmdFlags *pflag.FlagSet, strictMode bool) string {
	value, err := cmdFlags.GetString(key)
	if strictMode && err != nil {
		fmt.Printf("[x] %v, is not set and is required for your command.\n", key)
		os.Exit(0)
	}
	return value
}

// ExtractPaths for a give path (like a glob),we want a full path from it
// either it's a dir, or a file with any kind of extension
func ExtractPaths(path string) []string {
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
				fmt.Printf("Error: %v\n", err)
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
					fmt.Printf("Error: %v\n", err)
				}
			} else {
				files = append(files, p)
			}
		}
	}

	return files
}
