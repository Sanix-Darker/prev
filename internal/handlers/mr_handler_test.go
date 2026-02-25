package handlers

import (
	"testing"

	"github.com/sanix-darker/prev/internal/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockVCSProvider struct {
	mr    *vcs.MergeRequest
	diffs []vcs.FileDiff
}

func (m *mockVCSProvider) Info() vcs.ProviderInfo { return vcs.ProviderInfo{Name: "mock"} }
func (m *mockVCSProvider) Validate() error        { return nil }

func (m *mockVCSProvider) FetchMR(projectID string, mrIID int64) (*vcs.MergeRequest, error) {
	return m.mr, nil
}

func (m *mockVCSProvider) FetchMRDiffs(projectID string, mrIID int64) ([]vcs.FileDiff, error) {
	return m.diffs, nil
}

func (m *mockVCSProvider) ListOpenMRs(projectID string) ([]*vcs.MergeRequest, error) {
	return nil, nil
}

func (m *mockVCSProvider) PostSummaryNote(projectID string, mrIID int64, body string) error {
	return nil
}

func (m *mockVCSProvider) PostInlineComment(projectID string, mrIID int64, refs vcs.DiffRefs, comment vcs.InlineComment) error {
	return nil
}

func (m *mockVCSProvider) FormatSuggestionBlock(suggestion string) string {
	return "```suggestion\n" + suggestion + "\n```"
}

func TestExtractMRHandler(t *testing.T) {
	provider := &mockVCSProvider{
		mr: &vcs.MergeRequest{
			IID:          7,
			Title:        "Improve recipe endpoint",
			Description:  "Adds a recipe filter",
			SourceBranch: "feature",
			TargetBranch: "main",
		},
		diffs: []vcs.FileDiff{
			{
				OldPath: "public/index.php",
				NewPath: "public/index.php",
				Diff:    "@@ -1,1 +1,2 @@\n-old\n+new\n+line\n",
			},
		},
	}

	review, err := ExtractMRHandler(provider, "acme/blog", 7, "strict")
	require.NoError(t, err)
	require.NotNil(t, review)

	assert.Equal(t, int64(7), review.MR.IID)
	assert.Len(t, review.Changes, 1)
	assert.Contains(t, review.Prompt, "Improve recipe endpoint")
	assert.Contains(t, review.Prompt, "Report all issues")
}
