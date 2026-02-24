package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "prev",
	Short: "A CodeReviewer cli friend in your terminal.",
	Long:  `Get code reviews from AI for any kind of changes (diff, commit, branch, merge request).`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP("provider", "P", "", "AI provider to use (openai, anthropic, azure, ollama, etc.)")
	rootCmd.PersistentFlags().StringP("model", "m", "", "Model to use for the AI provider")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug output")
	rootCmd.PersistentFlags().BoolP("stream", "s", true, "Enable streaming output (default: true)")
	rootCmd.PersistentFlags().String("strictness", "", "Review strictness: strict, normal, lenient (default: normal)")
}
