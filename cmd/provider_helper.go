package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/sanix-darker/prev/internal/provider"
	"github.com/sanix-darker/prev/internal/renders"
)

// resolveProvider creates an AIProvider from the current config.
func resolveProvider(conf config.Config) (provider.AIProvider, error) {
	pcfg := provider.ResolveProvider(conf.Viper)

	// Override provider name from CLI
	if conf.Provider != "" {
		pcfg.Name = conf.Provider
	}

	// Override model from CLI
	if conf.Model != "" {
		pcfg.Viper.Set("model", conf.Model)
	}

	return provider.Get(pcfg.Name, pcfg.Viper)
}

// callProvider sends a prompt to the configured AI provider and prints the result.
func callProvider(conf config.Config, prompt string) {
	p, err := resolveProvider(conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving provider: %v\n", err)
		os.Exit(1)
	}
	info := p.Info()
	model := resolvedModelForLog(conf, info.DefaultModel)
	fmt.Printf("Model: provider=%s model=%s\n", info.Name, model)

	if conf.Debug {
		fmt.Fprintf(os.Stderr, "[debug] provider=%s model=%s\n", info.Name, info.DefaultModel)
	}

	if conf.Stream {
		streamCallProvider(conf, p, prompt)
	} else {
		blockingCallProvider(conf, p, prompt)
	}
}

func blockingCallProvider(conf config.Config, p provider.AIProvider, prompt string) {
	id, choices, err := provider.SimpleComplete(
		p,
		"You are a helpful assistant and source code reviewer.",
		"You are code reviewer for a project",
		prompt,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if conf.Debug {
		fmt.Fprintf(os.Stderr, "[debug] chatId=%s responses=%d\n", id, len(choices))
	}

	for _, resp := range choices {
		fmt.Print(renders.RenderMarkdown(resp))
	}
}

func streamCallProvider(conf config.Config, p provider.AIProvider, prompt string) {
	provider.ApiCallWithProvider(conf.Debug, p, prompt)
}

func resolvedModelForLog(conf config.Config, fallback string) string {
	if strings.TrimSpace(conf.Model) != "" {
		return strings.TrimSpace(conf.Model)
	}
	if conf.Viper != nil {
		pcfg := provider.ResolveProvider(conf.Viper)
		if strings.TrimSpace(conf.Provider) != "" {
			pcfg.Name = strings.TrimSpace(conf.Provider)
		}
		if pcfg.Viper != nil {
			if model := strings.TrimSpace(pcfg.Viper.GetString("model")); model != "" {
				return model
			}
		}
	}
	return strings.TrimSpace(fallback)
}
