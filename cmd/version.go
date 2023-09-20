/*
Copyright © 2023 sanix-darker <s4nixd@gmail.com>
*/

package cmd

import (
	"github.com/sanix-darker/prev/internal/cmd/version"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the application version.",
	Long:  `Print the application version with built/platform informations.`,
	Run: func(cmd *cobra.Command, args []string) {
		version.Print()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
