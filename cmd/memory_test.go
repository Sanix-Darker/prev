package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPruneReviewMemory_RemovesOldFixedAndTrims(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	mem := reviewMemory{
		Version: reviewMemoryVersion,
		Entries: []reviewMemoryEntry{
			{
				ID:       "open-1",
				Status:   "open",
				Severity: "HIGH",
				FilePath: "a.go",
				Line:     10,
				Message:  "open",
				LastSeen: now.Add(-24 * time.Hour).Format(time.RFC3339),
			},
			{
				ID:       "fixed-old",
				Status:   "fixed",
				Severity: "MEDIUM",
				FilePath: "b.go",
				Line:     20,
				Message:  "fixed old",
				LastSeen: now.Add(-45 * 24 * time.Hour).Format(time.RFC3339),
			},
			{
				ID:       "fixed-recent",
				Status:   "fixed",
				Severity: "LOW",
				FilePath: "c.go",
				Line:     30,
				Message:  "fixed recent",
				LastSeen: now.Add(-5 * 24 * time.Hour).Format(time.RFC3339),
			},
		},
	}

	removed := pruneReviewMemory(&mem, 1, 30, now)
	assert.Equal(t, 2, removed)
	require.Len(t, mem.Entries, 1)
	assert.Equal(t, "open", mem.Entries[0].Status)
}

func TestParseMemoryTime(t *testing.T) {
	ts := "2026-03-01T12:00:00Z"
	got := parseMemoryTime(ts)
	assert.Equal(t, ts, got.Format(time.RFC3339))

	epoch := parseMemoryTime("1700000000")
	assert.False(t, epoch.IsZero())

	assert.True(t, parseMemoryTime("nope").IsZero())
}
