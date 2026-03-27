package cmd

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sanix-darker/prev/internal/diffparse"
)

var semanticIdentRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{2,}`)

func semanticBehaviorID(message string) string {
	tokens := semanticKeywordSlice(message)
	if len(tokens) == 0 {
		return memoryRuleID(message)
	}
	sum := sha1.Sum([]byte(strings.Join(tokens, "|")))
	return fmt.Sprintf("%x", sum[:8])
}

func semanticPrimarySymbol(message, filePath string) string {
	candidates := semanticIdentRe.FindAllString(message, -1)
	for _, candidate := range candidates {
		if isNoiseSymbol(candidate) {
			continue
		}
		lower := strings.ToLower(candidate)
		if lower == "high" || lower == "medium" || lower == "low" || lower == "critical" || lower == "issue" {
			continue
		}
		return candidate
	}
	base := strings.TrimSuffix(filepath.Base(strings.TrimSpace(filePath)), filepath.Ext(strings.TrimSpace(filePath)))
	if base == "" || isNoiseSymbol(base) {
		return ""
	}
	return base
}

func semanticKeywordSlice(s string) []string {
	set := toKeywordSet(s)
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for tok := range set {
		out = append(out, tok)
	}
	sort.Strings(out)
	return out
}

func semanticKeywordOverlap(a, b string) int {
	return tokenOverlapScore(a, b)
}

func semanticJaccardScore(a, b string) float64 {
	ta := toKeywordSet(a)
	tb := toKeywordSet(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	intersection := 0
	union := len(ta)
	for tok := range tb {
		if _, ok := ta[tok]; ok {
			intersection++
			continue
		}
		union++
	}
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func semanticMessageScore(aMessage, aSymbol, bMessage, bSymbol string) int {
	score := semanticKeywordOverlap(aMessage, bMessage) * 10
	if semanticBehaviorID(aMessage) == semanticBehaviorID(bMessage) {
		score += 35
	}
	if aSymbol != "" && bSymbol != "" && strings.EqualFold(strings.TrimSpace(aSymbol), strings.TrimSpace(bSymbol)) {
		score += 20
	}
	if semanticJaccardScore(aMessage, bMessage) >= 0.5 {
		score += 15
	}
	return score
}

func changedTextKeywords(changes []diffparse.FileChange) map[string]struct{} {
	out := map[string]struct{}{}
	for _, c := range changes {
		for _, h := range c.Hunks {
			for _, l := range h.Lines {
				if l.Type != diffparse.LineAdded && l.Type != diffparse.LineDeleted {
					continue
				}
				for tok := range toKeywordSet(l.Content) {
					out[tok] = struct{}{}
				}
			}
		}
	}
	return out
}

func semanticEvidenceScore(entry reviewMemoryEntry, changedPaths map[string]struct{}, changedSymbols []string, changedKeywords map[string]struct{}) int {
	score := 0
	filePath := strings.TrimSpace(entry.FilePath)
	if _, ok := changedPaths[strings.ToLower(filePath)]; ok {
		score += 5
	}
	primary := strings.TrimSpace(entry.PrimarySymbol)
	if primary == "" {
		primary = semanticPrimarySymbol(entry.Message, filePath)
	}
	for _, sym := range changedSymbols {
		if strings.EqualFold(strings.TrimSpace(sym), primary) {
			score += 4
			break
		}
	}
	for _, tok := range semanticKeywordSlice(entry.Message) {
		if _, ok := changedKeywords[tok]; ok {
			score++
		}
	}
	return score
}
