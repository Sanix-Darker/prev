package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/sanix-darker/prev/internal/provider"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	aiCmd := &cobra.Command{
		Use:   "ai",
		Short: "Manage AI providers",
	}

	aiCmd.AddCommand(newAIListCmd())
	aiCmd.AddCommand(newAIShowCmd())
	rootCmd.AddCommand(aiCmd)
}

func newAIListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available AI providers",
		Run: func(cmd *cobra.Command, args []string) {
			names := provider.Names()
			fmt.Println("Available providers:")
			for _, name := range names {
				v := viper.New()
				provider.BindProviderEnvDefaults(name, v)
				p, err := provider.Get(name, v)
				if err != nil {
					fmt.Printf("  - %-15s (not configured)\n", name)
					continue
				}
				info := p.Info()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				status := "configured"
				if err := p.Validate(ctx); err != nil {
					status = "missing credentials"
				}
				cancel()
				fmt.Printf("  - %-15s %s [%s] (default model: %s)\n",
					info.Name, info.DisplayName, status, info.DefaultModel)
			}
		},
	}
}

func newAIShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current AI provider and model",
		Run: func(cmd *cobra.Command, args []string) {
			conf := config.NewDefaultConfig()
			applyFlags(cmd, &conf)

			pcfg := provider.ResolveProvider(conf.Viper)
			if conf.Provider != "" {
				pcfg.Name = conf.Provider
			}

			p, err := provider.Get(pcfg.Name, pcfg.Viper)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			info := p.Info()
			fmt.Printf("Provider: %s (%s)\n", info.Name, info.DisplayName)
			fmt.Printf("Model:    %s\n", info.DefaultModel)
			fmt.Printf("Streaming: %v\n", info.SupportsStreaming)
		},
	}
}
