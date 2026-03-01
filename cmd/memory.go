package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	memoryCmd := &cobra.Command{
		Use:   "memory",
		Short: "Manage persistent review memory",
	}

	memoryCmd.AddCommand(newMemoryShowCmd())
	memoryCmd.AddCommand(newMemoryExportCmd())
	memoryCmd.AddCommand(newMemoryPruneCmd())
	memoryCmd.AddCommand(newMemoryResetCmd())
	rootCmd.AddCommand(memoryCmd)
}

func newMemoryShowCmd() *cobra.Command {
	var memoryFile string
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show review memory",
		Run: func(cmd *cobra.Command, args []string) {
			repoPath := resolveMRRepoPath()
			mem, path, err := loadReviewMemory(repoPath, memoryFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if asJSON {
				out, err := json.MarshalIndent(mem, "", "  ")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Println(string(out))
				return
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				fmt.Printf("No memory file found at %s\n", path)
				return
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Print(string(raw))
		},
	}

	cmd.Flags().StringVar(&memoryFile, "memory-file", defaultReviewMemoryFile, "Path to review memory markdown file")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Show only machine-readable JSON payload")
	return cmd
}

func newMemoryExportCmd() *cobra.Command {
	var memoryFile string
	var format string

	cmd := &cobra.Command{
		Use:   "export <output_path>",
		Short: "Export review memory as markdown or json",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			outPath := strings.TrimSpace(args[0])
			if outPath == "" {
				fmt.Fprintln(os.Stderr, "Error: output path is required")
				os.Exit(1)
			}
			repoPath := resolveMRRepoPath()
			mem, _, err := loadReviewMemory(repoPath, memoryFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			switch strings.ToLower(strings.TrimSpace(format)) {
			case "json":
				raw, err := json.MarshalIndent(mem, "", "  ")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				if err := os.WriteFile(outPath, raw, 0o644); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
			default:
				if err := saveReviewMemory(outPath, mem); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
			}
			fmt.Printf("Exported review memory to %s\n", outPath)
		},
	}

	cmd.Flags().StringVar(&memoryFile, "memory-file", defaultReviewMemoryFile, "Path to review memory markdown file")
	cmd.Flags().StringVar(&format, "format", "markdown", "Export format: markdown, json")
	return cmd
}

func newMemoryPruneCmd() *cobra.Command {
	var memoryFile string
	var maxEntries int
	var fixedOlderThanDays int
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Prune old/low-value review memory entries",
		Run: func(cmd *cobra.Command, args []string) {
			repoPath := resolveMRRepoPath()
			mem, path, err := loadReviewMemory(repoPath, memoryFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			before := len(mem.Entries)
			removed := pruneReviewMemory(&mem, maxEntries, fixedOlderThanDays, time.Now().UTC())
			after := len(mem.Entries)
			if dryRun {
				fmt.Printf("Dry-run prune: removed=%d before=%d after=%d file=%s\n", removed, before, after, path)
				return
			}
			if err := saveReviewMemory(path, mem); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Pruned review memory: removed=%d before=%d after=%d file=%s\n", removed, before, after, path)
		},
	}

	cmd.Flags().StringVar(&memoryFile, "memory-file", defaultReviewMemoryFile, "Path to review memory markdown file")
	cmd.Flags().IntVar(&maxEntries, "max-entries", 500, "Maximum number of entries to keep after pruning")
	cmd.Flags().IntVar(&fixedOlderThanDays, "fixed-older-than-days", 30, "Remove fixed entries older than N days (0 disables age-based fixed pruning)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show prune result without writing changes")
	return cmd
}

func newMemoryResetCmd() *cobra.Command {
	var memoryFile string
	var yes bool

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset review memory to empty",
		Run: func(cmd *cobra.Command, args []string) {
			if !yes {
				fmt.Fprintln(os.Stderr, "Refusing to reset memory without --yes")
				os.Exit(1)
			}
			repoPath := resolveMRRepoPath()
			_, path, err := loadReviewMemory(repoPath, memoryFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			mem := reviewMemory{
				Version:   reviewMemoryVersion,
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
				Entries:   []reviewMemoryEntry{},
			}
			if err := saveReviewMemory(path, mem); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Review memory reset: %s\n", path)
		},
	}

	cmd.Flags().StringVar(&memoryFile, "memory-file", defaultReviewMemoryFile, "Path to review memory markdown file")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm memory reset")
	return cmd
}

func pruneReviewMemory(mem *reviewMemory, maxEntries, fixedOlderThanDays int, now time.Time) int {
	if mem == nil || len(mem.Entries) == 0 {
		return 0
	}
	normalizeReviewMemory(mem)
	orig := len(mem.Entries)
	kept := make([]reviewMemoryEntry, 0, len(mem.Entries))

	for _, e := range mem.Entries {
		if e.Status != "fixed" || fixedOlderThanDays <= 0 {
			kept = append(kept, e)
			continue
		}
		last := parseMemoryTime(e.LastSeen)
		if last.IsZero() {
			kept = append(kept, e)
			continue
		}
		if now.Sub(last) <= (time.Duration(fixedOlderThanDays) * 24 * time.Hour) {
			kept = append(kept, e)
		}
	}
	mem.Entries = kept
	if maxEntries > 0 {
		trimReviewMemory(mem, maxEntries)
	}
	return orig - len(mem.Entries)
}

func parseMemoryTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t
	}
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Unix(n, 0).UTC()
	}
	return time.Time{}
}
