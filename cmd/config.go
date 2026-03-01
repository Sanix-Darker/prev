package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sanix-darker/prev/internal/config"
	"github.com/sanix-darker/prev/internal/provider"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage prev configuration",
	}

	configCmd.AddCommand(newConfigInitCmd())
	configCmd.AddCommand(newConfigShowCmd())
	configCmd.AddCommand(newConfigEffectiveCmd())
	configCmd.AddCommand(newConfigValidateCmd())
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

func newConfigEffectiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "effective",
		Short: "Print effective config after env/flag overrides",
		Run: func(cmd *cobra.Command, args []string) {
			conf := config.NewDefaultConfig()
			applyFlags(cmd, &conf)

			effective := buildEffectiveConfig(conf)
			out, err := yaml.Marshal(effective)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding config: %v\n", err)
				os.Exit(1)
			}
			fmt.Print(string(out))
		},
	}
}

func newConfigValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate config values and required provider fields",
		Run: func(cmd *cobra.Command, args []string) {
			conf := config.NewDefaultConfig()
			applyFlags(cmd, &conf)

			errs := validateEffectiveConfig(conf)
			if len(errs) > 0 {
				fmt.Println("Configuration is invalid:")
				for _, e := range errs {
					fmt.Printf("- %s\n", e)
				}
				os.Exit(1)
			}
			fmt.Println("Configuration is valid.")
		},
	}
}

func buildEffectiveConfig(conf config.Config) map[string]interface{} {
	v := conf.Viper
	if v == nil {
		v = config.NewStore()
	}

	pcfg := provider.ResolveProvider(v)
	pv := pcfg.Viper

	out := map[string]interface{}{
		"provider": pcfg.Name,
		"providers": map[string]interface{}{
			pcfg.Name: providerBlock(pcfg.Name, pv),
		},
		"retry": map[string]interface{}{
			"max_retries":      intOrDefault(v.GetInt("retry.max_retries"), 3),
			"initial_interval": strOrDefault(v.GetString("retry.initial_interval"), "1s"),
			"max_interval":     strOrDefault(v.GetString("retry.max_interval"), "30s"),
			"multiplier":       floatOrDefault(rawValue(v, "retry.multiplier"), 2.0),
		},
		"review": map[string]interface{}{
			"strictness":        strOrDefault(v.GetString("review.strictness"), "normal"),
			"nitpick":           intOrDefault(v.GetInt("review.nitpick"), 5),
			"passes":            intOrDefault(v.GetInt("review.passes"), 1),
			"max_comments":      intOrDefault(v.GetInt("review.max_comments"), 0),
			"filter_mode":       strOrDefault(v.GetString("review.filter_mode"), "diff_context"),
			"mr_diff_source":    strOrDefault(v.GetString("review.mr_diff_source"), "auto"),
			"structured_output": v.GetBool("review.structured_output"),
			"incremental":       v.GetBool("review.incremental"),
			"inline_only":       v.GetBool("review.inline_only"),
			"serena_mode":       strOrDefault(v.GetString("review.serena_mode"), "auto"),
			"context_lines":     intOrDefault(v.GetInt("review.context_lines"), 10),
			"max_tokens":        intOrDefault(v.GetInt("review.max_tokens"), 80000),
			"conventions": map[string]interface{}{
				"labels": stringSliceOrDefault(v.GetStringSlice("review.conventions.labels"), []string{"issue", "suggestion", "remark"}),
			},
			"guidelines": strings.TrimSpace(v.GetString("review.guidelines")),
		},
		"debug":                        v.GetBool("debug"),
		"stream":                       boolOrDefault(rawValue(v, "stream"), true),
		"max_key_points":               intOrDefault(v.GetInt("max_key_points"), 3),
		"max_characters_per_key_point": intOrDefault(v.GetInt("max_characters_per_key_point"), 100),
		"explain":                      v.GetBool("explain"),
	}

	return out
}

func rawValue(v *config.Store, key string) interface{} {
	if v == nil {
		return nil
	}
	val, ok := v.Get(key)
	if !ok {
		return nil
	}
	return val
}

func providerBlock(name string, v *config.Store) map[string]interface{} {
	if v == nil {
		v = config.NewStore()
	}
	out := map[string]interface{}{
		"api_key":    redactSecret(v.GetString("api_key")),
		"model":      strings.TrimSpace(v.GetString("model")),
		"base_url":   strings.TrimSpace(v.GetString("base_url")),
		"max_tokens": intOrDefault(v.GetInt("max_tokens"), 1024),
		"timeout":    strOrDefault(v.GetString("timeout"), "30s"),
	}
	if name == "azure" {
		out["api_version"] = strOrDefault(v.GetString("api_version"), "2024-02-01")
	}
	return out
}

func validateEffectiveConfig(conf config.Config) []string {
	v := conf.Viper
	if v == nil {
		v = config.NewStore()
	}
	var errs []string
	pcfg := provider.ResolveProvider(v)
	pv := pcfg.Viper

	apiKey := strings.TrimSpace(pv.GetString("api_key"))
	model := strings.TrimSpace(pv.GetString("model"))
	baseURL := strings.TrimSpace(pv.GetString("base_url"))

	switch pcfg.Name {
	case "openai":
		if apiKey == "" {
			errs = append(errs, "providers.openai.api_key (or OPENAI_API_KEY) is required")
		}
	case "gemini":
		if apiKey == "" {
			errs = append(errs, "providers.gemini.api_key (or GEMINI_API_KEY) is required")
		}
	case "anthropic", "claude":
		if apiKey == "" {
			errs = append(errs, "providers.anthropic.api_key (or ANTHROPIC_API_KEY) is required")
		}
	case "azure":
		if apiKey == "" {
			errs = append(errs, "providers.azure.api_key (or AZURE_OPENAI_API_KEY) is required")
		}
		if baseURL == "" {
			errs = append(errs, "providers.azure.base_url (or AZURE_OPENAI_ENDPOINT) is required")
		}
		if model == "" {
			errs = append(errs, "providers.azure.model (or AZURE_OPENAI_MODEL/AZURE_OPENAI_DEPLOYMENT) is required")
		}
	default:
		if baseURL == "" {
			errs = append(errs, fmt.Sprintf("providers.%s.base_url (or PREV_%s_BASE_URL) is required", pcfg.Name, strings.ToUpper(pcfg.Name)))
		}
	}

	strictness := strings.ToLower(strings.TrimSpace(v.GetString("review.strictness")))
	if strictness != "" && strictness != "strict" && strictness != "normal" && strictness != "lenient" {
		errs = append(errs, "review.strictness must be one of: strict, normal, lenient")
	}
	if n := v.GetInt("review.nitpick"); n < 0 || n > 10 {
		errs = append(errs, "review.nitpick must be between 0 and 10")
	}
	if p := v.GetInt("review.passes"); p < 0 || p > 6 {
		errs = append(errs, "review.passes must be between 0 and 6")
	}
	if m := v.GetInt("review.max_comments"); m < 0 {
		errs = append(errs, "review.max_comments must be >= 0")
	}
	if mode := strings.ToLower(strings.TrimSpace(v.GetString("review.filter_mode"))); mode != "" &&
		mode != "added" && mode != "diff_context" && mode != "file" && mode != "nofilter" {
		errs = append(errs, "review.filter_mode must be one of: added, diff_context, file, nofilter")
	}
	if src := strings.ToLower(strings.TrimSpace(v.GetString("review.mr_diff_source"))); src != "" &&
		src != "auto" && src != "git" && src != "raw" && src != "api" {
		errs = append(errs, "review.mr_diff_source must be one of: auto, git, raw, api")
	}
	if mode := strings.ToLower(strings.TrimSpace(v.GetString("review.serena_mode"))); mode != "" &&
		mode != "auto" && mode != "on" && mode != "off" {
		errs = append(errs, "review.serena_mode must be one of: auto, on, off")
	}
	if c := v.GetInt("review.context_lines"); c < 0 {
		errs = append(errs, "review.context_lines must be >= 0")
	}
	if t := v.GetInt("review.max_tokens"); t < 0 {
		errs = append(errs, "review.max_tokens must be >= 0")
	}

	return errs
}

func intOrDefault(v, d int) int {
	if v == 0 {
		return d
	}
	return v
}

func strOrDefault(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func floatOrDefault(v interface{}, d float64) float64 {
	if v == nil {
		return d
	}
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	default:
		return d
	}
}

func boolOrDefault(v interface{}, d bool) bool {
	if v == nil {
		return d
	}
	switch x := v.(type) {
	case bool:
		return x
	default:
		return d
	}
}

func stringSliceOrDefault(v, d []string) []string {
	if len(v) == 0 {
		return d
	}
	return v
}

func redactSecret(v string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return ""
	}
	return "***"
}
