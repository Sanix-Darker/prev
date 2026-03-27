package review

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sanix-darker/prev/internal/core"
	"github.com/sanix-darker/prev/internal/diffparse"
	"github.com/sanix-darker/prev/internal/provider"
	"github.com/sanix-darker/prev/internal/serena"
)

// ProgressCallback reports pipeline progress to the CLI.
type ProgressCallback func(stage string, current, total int)

// RunBranchReview executes the full two-pass review pipeline.
func RunBranchReview(
	aiProvider provider.AIProvider,
	repoPath string,
	branchName string,
	baseBranch string,
	cfg ReviewConfig,
	onProgress ProgressCallback,
) (*BranchReviewResult, error) {
	if onProgress == nil {
		onProgress = func(string, int, int) {}
	}

	// Step 1: Get raw diff
	onProgress("Getting diff", 0, 0)
	rawDiff, err := core.GetGitDiffForBranch(repoPath, baseBranch, branchName)
	if err != nil {
		return nil, fmt.Errorf("failed to get branch diff: %w", err)
	}
	if rawDiff == "" {
		return nil, fmt.Errorf("no differences found between %s and %s", baseBranch, branchName)
	}

	// Step 2: Parse diff
	onProgress("Parsing diff", 0, 0)
	changes, err := diffparse.ParseGitDiff(rawDiff)
	if err != nil {
		return nil, fmt.Errorf("failed to parse diff: %w", err)
	}
	changes = diffparse.FilterTextChanges(changes)
	if len(changes) == 0 {
		return nil, fmt.Errorf("no reviewable text changes found between %s and %s", baseBranch, branchName)
	}

	// Step 3: Initialize Serena if configured
	var serenaClient *serena.Client
	if cfg.SerenaMode == "off" {
		onProgress("Serena: off (line-based context)", 0, 0)
	} else {
		serenaClient, err = serena.NewClient(cfg.SerenaMode)
		if err != nil {
			return nil, fmt.Errorf("serena initialization failed: %w", err)
		}
		if serenaClient != nil {
			onProgress("Serena: active (symbol-level context via MCP)", 0, 0)
		} else {
			onProgress("Serena: unavailable, fallback to line-based context", 0, 0)
		}
		if serenaClient != nil {
			defer serenaClient.Close()
		}
	}

	// Step 4: Enrich file changes with context
	onProgress("Enriching context", 0, 0)
	enriched, err := diffparse.EnrichFileChanges(
		changes, repoPath, baseBranch, branchName,
		cfg.ContextLines, cfg.MaxBatchTokens, serenaClient,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to enrich changes: %w", err)
	}

	// Step 5: Categorize
	categorized := CategorizeChanges(enriched)

	// Step 6: Get diff stat
	diffStat, _ := core.GetDiffStat(repoPath, baseBranch, branchName)

	walkthroughConv := provider.NewConversation(aiProvider, provider.ConversationOptions{
		SystemPrompt: "You are an expert code reviewer. Keep branch walkthroughs concise and preserve useful context for detailed follow-up review.",
	})

	// Step 7: Pass 1 — Walkthrough
	onProgress("AI walkthrough", 0, 0)
	walkthroughPrompt := BuildWalkthroughPrompt(
		branchName,
		baseBranch,
		categorized,
		diffStat,
		cfg.Strictness,
		cfg.Guidelines,
	)

	if cfg.Debug {
		fmt.Printf("[debug] walkthrough prompt length: %d chars\n", len(walkthroughPrompt))
	}

	walkthroughContent, err := completeConversation(walkthroughConv, walkthroughPrompt)
	if err != nil {
		return nil, fmt.Errorf("walkthrough AI call failed: %w", err)
	}

	walkthrough := parseWalkthrough(walkthroughContent)
	walkthroughContext := formatWalkthroughContext(walkthrough)
	reviewBaseConv := provider.NewConversation(aiProvider, provider.ConversationOptions{
		SystemPrompt: "You are an expert code reviewer performing detailed file-by-file follow-up review. Use prior walkthrough context to stay consistent, but only report findings supported by the current batch diff.",
		Messages: []provider.Message{{
			Role:    provider.RoleAssistant,
			Content: walkthroughContext,
		}},
	})

	// Step 8: Batch files
	batches := BatchFiles(categorized, cfg.MaxBatchTokens)

	// Step 9: Pass 2 — Detailed review per batch
	var allFileReviews []FileReviewResult
	for i, batch := range batches {
		onProgress("Reviewing files", i+1, len(batches))

		reviewPrompt := BuildFileReviewPrompt(
			batch,
			walkthroughContext,
			branchName,
			cfg.Strictness,
			cfg.Guidelines,
		)

		if cfg.Debug {
			fmt.Printf("[debug] batch %d/%d: %d files, prompt length: %d chars, walkthrough_response_id=%s\n",
				i+1, len(batches), len(batch.Files), len(reviewPrompt), walkthroughConv.LastResponseID())
		}

		reviewConv := reviewBaseConv.Clone()
		reviewContent, err := completeConversation(reviewConv, reviewPrompt)
		if err != nil {
			return nil, fmt.Errorf("review AI call (batch %d) failed: %w", i+1, err)
		}

		parsed := core.ParseReviewResponse(reviewContent)
		filtered := core.FilterBySeverity(parsed.FileComments, cfg.Strictness)

		// Group comments by file
		fileMap := map[string]*FileReviewResult{}
		for _, f := range batch.Files {
			name := f.NewName
			if name == "" {
				name = f.OldName
			}
			fileMap[name] = &FileReviewResult{FilePath: name}
		}

		for _, c := range filtered {
			fr, ok := fileMap[c.FilePath]
			if !ok {
				fr = &FileReviewResult{FilePath: c.FilePath}
				fileMap[c.FilePath] = fr
			}
			fr.Comments = append(fr.Comments, c)
		}

		for _, fr := range fileMap {
			allFileReviews = append(allFileReviews, *fr)
		}
	}

	// Step 10: Assemble result
	totalAdd, totalDel := 0, 0
	for _, fc := range changes {
		totalAdd += fc.Stats.Additions
		totalDel += fc.Stats.Deletions
	}

	return &BranchReviewResult{
		BranchName:     branchName,
		BaseBranch:     baseBranch,
		Walkthrough:    walkthrough,
		FileReviews:    allFileReviews,
		TotalFiles:     len(changes),
		TotalAdditions: totalAdd,
		TotalDeletions: totalDel,
	}, nil
}

func completeConversation(conv *provider.Conversation, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := conv.Complete(ctx, prompt)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func formatWalkthroughContext(w WalkthroughResult) string {
	var sb strings.Builder
	summary := strings.TrimSpace(w.Summary)
	if summary != "" {
		sb.WriteString("Walkthrough summary:\n")
		sb.WriteString(summary)
		sb.WriteString("\n")
	}
	changes := strings.TrimSpace(w.ChangesTable)
	if changes != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Walkthrough changes table:\n")
		sb.WriteString(changes)
	}
	out := strings.TrimSpace(sb.String())
	if out == "" {
		return "Walkthrough summary unavailable. Review the batch strictly from the provided diff context."
	}
	return out
}

// parseWalkthrough extracts structured walkthrough from AI response.
func parseWalkthrough(content string) WalkthroughResult {
	result := WalkthroughResult{RawContent: content}

	lines := strings.Split(content, "\n")
	var summaryLines []string
	var tableLines []string
	inTable := false
	inSummary := true

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect table start
		if strings.HasPrefix(trimmed, "| File") || strings.HasPrefix(trimmed, "|---") {
			inTable = true
			inSummary = false
		}

		if inTable {
			if strings.HasPrefix(trimmed, "|") {
				tableLines = append(tableLines, line)
			} else if trimmed == "" && len(tableLines) > 0 {
				inTable = false
			}
		} else if inSummary {
			// Stop summary at first heading after some content
			if strings.HasPrefix(trimmed, "#") && len(summaryLines) > 0 {
				inSummary = false
				continue
			}
			summaryLines = append(summaryLines, line)
		}
	}

	result.Summary = strings.TrimSpace(strings.Join(summaryLines, "\n"))
	if len(tableLines) > 0 {
		result.ChangesTable = strings.Join(tableLines, "\n")
	}

	return result
}
