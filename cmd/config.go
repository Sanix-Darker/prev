package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/sanix-darker/prev/internal/provider"
	"github.com/spf13/cobra"
)

func init() {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage prev configuration",
	}

	configCmd.AddCommand(newConfigInitCmd())
	configCmd.AddCommand(newConfigShowCmd())
	rootCmd.AddCommand(configCmd)
}

func newConfigInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create default config file at ~/.config/prev/config.yml",
		Run: func(cmd *cobra.Command, args []string) {
			conf := config.NewDefaultConfig()
			cfgPath, err := config.GetConfigFilePath(conf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Create directory if needed
			dir := filepath.Dir(cfgPath)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating config directory: %v\n", err)
				os.Exit(1)
			}

			// Don't overwrite existing config
			if _, err := os.Stat(cfgPath); err == nil {
				fmt.Printf("Config file already exists at %s\n", cfgPath)
				return
			}

			if err := os.WriteFile(cfgPath, []byte(provider.SampleConfigYAML()), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Config file created at %s\n", cfgPath)
		},
	}
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print current config",
		Run: func(cmd *cobra.Command, args []string) {
			conf := config.NewDefaultConfig()
			cfgPath, err := config.GetConfigFilePath(conf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			data, err := os.ReadFile(cfgPath)
			if err != nil {
				fmt.Printf("No config file found at %s\n", cfgPath)
				fmt.Println("\nDefault configuration:")
				fmt.Println(provider.SampleConfigYAML())
				return
			}

			fmt.Printf("# Config file: %s\n", cfgPath)
			fmt.Println(string(data))
		},
	}
}
