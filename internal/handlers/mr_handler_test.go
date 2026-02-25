package handlers

import (
	"testing"

	"github.com/sanix-darker/prev/internal/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockMRVCSProvider struct {
	mr      *vcs.MergeRequest
	diffs   []vcs.FileDiff
	rawDiff string
}

func (m *mockMRVCSProvider) Info() vcs.ProviderInfo { return vcs.ProviderInfo{Name: "mock"} }
func (m *mockMRVCSProvider) FetchMR(string, int64) (*vcs.MergeRequest, error) {
	return m.mr, nil
}
func (m *mockMRVCSProvider) FetchMRDiffs(string, int64) ([]vcs.FileDiff, error) {
	return m.diffs, nil
}
func (m *mockMRVCSProvider) FetchMRRawDiff(string, int64) (string, error) {
	return m.rawDiff, nil
}
func (m *mockMRVCSProvider) ListMRDiscussions(string, int64) ([]vcs.MRDiscussion, error) {
	return nil, nil
}
func (m *mockMRVCSProvider) ListMRNotes(string, int64) ([]vcs.MRNote, error) { return nil, nil }
func (m *mockMRVCSProvider) ListOpenMRs(string) ([]*vcs.MergeRequest, error) { return nil, nil }
func (m *mockMRVCSProvider) PostSummaryNote(string, int64, string) error     { return nil }
func (m *mockMRVCSProvider) PostInlineComment(string, int64, vcs.DiffRefs, vcs.InlineComment) error {
	return nil
}
func (m *mockMRVCSProvider) ReplyToMRDiscussion(string, int64, string, string) error { return nil }
func (m *mockMRVCSProvider) FormatSuggestionBlock(s string) string                   { return s }
func (m *mockMRVCSProvider) Validate() error                                         { return nil }

func TestNormalizeDiffSource(t *testing.T) {
	assert.Equal(t, "auto", normalizeDiffSource(""))
	assert.Equal(t, "auto", normalizeDiffSource("unknown"))
	assert.Equal(t, "git", normalizeDiffSource("git"))
	assert.Equal(t, "raw", normalizeDiffSource("RAW"))
	assert.Equal(t, "api", normalizeDiffSource("api"))
}

func TestExtractMRHandlerWithOptions_RawDiffPreferred(t *testing.T) {
	provider := &mockMRVCSProvider{
		mr: &vcs.MergeRequest{
			IID:          42,
			Title:        "test",
			Description:  "desc",
			SourceBranch: "feature",
			TargetBranch: "main",
			DiffRefs: vcs.DiffRefs{
				BaseSHA:  "aaa",
				HeadSHA:  "bbb",
				StartSHA: "ccc",
			},
		},
		rawDiff: "diff --git a/public/index.php b/public/index.php\n--- a/public/index.php\n+++ b/public/index.php\n@@ -1,1 +1,2 @@\n <?php\n+echo json_encode($x);\n",
	}
	got, err := ExtractMRHandlerWithOptions(provider, "grp/proj", 42, "normal", MRExtractOptions{
		DiffSource: "raw",
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEmpty(t, got.Changes)
}

func TestExtractMRHandlerWithOptions_FailsOnNoHunks(t *testing.T) {
	provider := &mockMRVCSProvider{
		mr: &vcs.MergeRequest{
			IID:          42,
			Title:        "test",
			Description:  "desc",
			SourceBranch: "feature",
			TargetBranch: "main",
		},
		diffs: []vcs.FileDiff{
			{OldPath: "public/index.php", NewPath: "public/index.php", Diff: ""},
		},
	}
	_, err := ExtractMRHandlerWithOptions(provider, "grp/proj", 42, "normal", MRExtractOptions{
		DiffSource: "api",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no reviewable modified hunks found")
}
