package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sanix-darker/prev/internal/diffparse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractChangedSymbols(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "main.go",
			Hunks: []diffparse.Hunk{
				{
					NewStart: 10,
					NewLines: 4,
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineAdded, Content: "func ProcessOrder(ctx context.Context) error {"},
						{Type: diffparse.LineAdded, Content: "return validateOrder(ctx)"},
						{Type: diffparse.LineAdded, Content: "go asyncPublish(evt)"},
					},
				},
			},
		},
	}
	got := extractChangedSymbols(changes, 10)
	assert.Contains(t, got, "ProcessOrder")
	assert.Contains(t, got, "validateOrder")
	assert.Contains(t, got, "asyncPublish")
}

func TestDetectNativeConcurrencySignals(t *testing.T) {
	changes := []diffparse.FileChange{
		{
			NewName: "worker.go",
			Hunks: []diffparse.Hunk{
				{
					NewStart: 20,
					NewLines: 5,
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineAdded, NewLineNo: 21, Content: "mu.Lock()"},
						{Type: diffparse.LineAdded, NewLineNo: 22, Content: "go func(){ shared[\"x\"] = 1 }()"},
						{Type: diffparse.LineAdded, NewLineNo: 23, Content: "ch <- msg"},
					},
				},
			},
		},
	}
	got := detectNativeConcurrencySignals(changes)
	require.NotEmpty(t, got)
	joined := strings.Join(got, "\n")
	assert.Contains(t, joined, "goroutine")
	assert.Contains(t, joined, "lock")
	assert.Contains(t, joined, "channel")
}

func TestBuildNativeImpactReport(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\nfunc Run(){ ProcessOrder(); validateOrder() }\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\nfunc X(){ ProcessOrder() }\n"), 0o644))

	changes := []diffparse.FileChange{
		{
			NewName: "a.go",
			Hunks: []diffparse.Hunk{
				{
					NewStart: 1,
					NewLines: 2,
					Lines: []diffparse.DiffLine{
						{Type: diffparse.LineAdded, Content: "func ProcessOrder() {}"},
						{Type: diffparse.LineAdded, Content: "validateOrder()"},
					},
				},
			},
		},
	}
	out := buildNativeImpactReport(changes, dir, 10)
	assert.Contains(t, out, "Native impact precheck")
	assert.Contains(t, out, "ProcessOrder")
	assert.Contains(t, out, "refs=")
}
