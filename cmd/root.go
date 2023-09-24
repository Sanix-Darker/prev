/*
Copyright © 2023 sanix-darker <s4nixd@gmail.com>

*/

package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "prev",
	Short: "A CodeReviewer cli friend in your terminal.",
	Long:  `Get code reviews from AI for any kind of changes (diff, commit, branch).`,
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.test.yaml)")

	// for a sub-command flag.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
