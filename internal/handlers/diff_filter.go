package handlers

import "strings"

// filterUnifiedDiffByPath filters a unified diff to only include file sections
// where the "diff --git" header matches pathFilter.
func filterUnifiedDiffByPath(diff string, pathFilter string) string {
	var result []string
	var include bool

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git") {
			include = strings.Contains(line, pathFilter)
		}
		if include {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}
