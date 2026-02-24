package review

import "sort"

// FileBatch is a group of files that fit within the token budget.
type FileBatch struct {
	Files       []CategorizedFile
	TotalTokens int
	solo        bool // solo batches don't accept additional files
}

// BatchFiles groups categorized files into batches that fit within maxTokens.
// Uses first-fit-decreasing bin packing.
func BatchFiles(files []CategorizedFile, maxTokens int) []FileBatch {
	if len(files) == 0 {
		return nil
	}

	// Sort by token estimate descending
	sorted := make([]CategorizedFile, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].TokenEstimate > sorted[j].TokenEstimate
	})

	var batches []FileBatch

	for _, file := range sorted {
		// If file is > 80% of maxTokens, give it a solo batch
		if file.TokenEstimate > maxTokens*80/100 {
			batches = append(batches, FileBatch{
				Files:       []CategorizedFile{file},
				TotalTokens: file.TokenEstimate,
				solo:        true,
			})
			continue
		}

		// Try to fit into an existing non-solo batch
		placed := false
		for i := range batches {
			if !batches[i].solo && batches[i].TotalTokens+file.TokenEstimate <= maxTokens {
				batches[i].Files = append(batches[i].Files, file)
				batches[i].TotalTokens += file.TokenEstimate
				placed = true
				break
			}
		}

		if !placed {
			batches = append(batches, FileBatch{
				Files:       []CategorizedFile{file},
				TotalTokens: file.TokenEstimate,
			})
		}
	}

	return batches
}
