package vcs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider implements VCSProvider for testing.
type mockProvider struct{}

func (m *mockProvider) Info() ProviderInfo                                             { return ProviderInfo{Name: "mock"} }
func (m *mockProvider) Validate() error                                                { return nil }
func (m *mockProvider) FormatSuggestionBlock(s string) string                          { return "```\n" + s + "\n```" }
func (m *mockProvider) FetchMR(string, int64) (*MergeRequest, error)                   { return nil, nil }
func (m *mockProvider) FetchMRDiffs(string, int64) ([]FileDiff, error)                 { return nil, nil }
func (m *mockProvider) FetchMRRawDiff(string, int64) (string, error)                   { return "", nil }
func (m *mockProvider) ListMRDiscussions(string, int64) ([]MRDiscussion, error)        { return nil, nil }
func (m *mockProvider) ListMRNotes(string, int64) ([]MRNote, error)                    { return nil, nil }
func (m *mockProvider) ListOpenMRs(string) ([]*MergeRequest, error)                    { return nil, nil }
func (m *mockProvider) PostSummaryNote(string, int64, string) error                    { return nil }
func (m *mockProvider) PostInlineComment(string, int64, DiffRefs, InlineComment) error { return nil }
func (m *mockProvider) ReplyToMRDiscussion(string, int64, string, string) error        { return nil }

func mockFactory(token, baseURL string) (VCSProvider, error) {
	return &mockProvider{}, nil
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register("mock", mockFactory)

	p, err := r.Get("mock", "tok", "http://example.com")
	require.NoError(t, err)
	assert.Equal(t, "mock", p.Info().Name)
}

func TestRegistryGetUnknown(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("nope", "tok", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestRegistryDuplicatePanics(t *testing.T) {
	r := NewRegistry()
	r.Register("dup", mockFactory)

	assert.Panics(t, func() {
		r.Register("dup", mockFactory)
	})
}

func TestRegistryNames(t *testing.T) {
	r := NewRegistry()
	r.Register("beta", mockFactory)
	r.Register("alpha", mockFactory)

	names := r.Names()
	assert.Equal(t, []string{"alpha", "beta"}, names)
}
