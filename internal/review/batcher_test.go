package review

import (
	"testing"

	"github.com/sanix-darker/prev/internal/diffparse"
	"github.com/stretchr/testify/assert"
)

func TestBatchFiles_FitsInOne(t *testing.T) {
	files := []CategorizedFile{
		{EnrichedFileChange: diffparse.EnrichedFileChange{TokenEstimate: 1000}},
		{EnrichedFileChange: diffparse.EnrichedFileChange{TokenEstimate: 2000}},
		{EnrichedFileChange: diffparse.EnrichedFileChange{TokenEstimate: 500}},
	}

	batches := BatchFiles(files, 80000)
	assert.Len(t, batches, 1)
	assert.Len(t, batches[0].Files, 3)
	assert.Equal(t, 3500, batches[0].TotalTokens)
}

func TestBatchFiles_SplitsLarge(t *testing.T) {
	files := []CategorizedFile{
		{EnrichedFileChange: diffparse.EnrichedFileChange{TokenEstimate: 70000}}, // > 80% of 80000
		{EnrichedFileChange: diffparse.EnrichedFileChange{TokenEstimate: 1000}},
		{EnrichedFileChange: diffparse.EnrichedFileChange{TokenEstimate: 2000}},
	}

	batches := BatchFiles(files, 80000)
	assert.GreaterOrEqual(t, len(batches), 2)

	// The large file should be in a solo batch
	foundSolo := false
	for _, b := range batches {
		if len(b.Files) == 1 && b.TotalTokens == 70000 {
			foundSolo = true
			break
		}
	}
	assert.True(t, foundSolo, "large file should be in a solo batch")
}

func TestBatchFiles_Empty(t *testing.T) {
	batches := BatchFiles(nil, 80000)
	assert.Nil(t, batches)

	batches = BatchFiles([]CategorizedFile{}, 80000)
	assert.Nil(t, batches)
}
