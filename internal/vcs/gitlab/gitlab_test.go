package gitlab

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanix-darker/prev/internal/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProvider(t *testing.T, handler http.Handler) vcs.VCSProvider {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	p, err := NewProvider("test-token", server.URL)
	require.NoError(t, err)
	return p
}

func TestFetchMR(t *testing.T) {
	p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "merge_requests/42")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"iid":           42,
			"title":         "Add feature",
			"description":   "Adds a new feature",
			"source_branch": "feature",
			"target_branch": "main",
			"state":         "opened",
			"web_url":       "https://gitlab.com/grp/proj/-/merge_requests/42",
			"author":        map[string]interface{}{"username": "dev"},
			"diff_refs": map[string]interface{}{
				"base_sha":  "aaa",
				"head_sha":  "bbb",
				"start_sha": "ccc",
			},
		})
	}))

	mr, err := p.FetchMR("grp/proj", 42)
	require.NoError(t, err)
	assert.Equal(t, int64(42), mr.IID)
	assert.Equal(t, "Add feature", mr.Title)
	assert.Equal(t, "dev", mr.Author)
	assert.Equal(t, "aaa", mr.DiffRefs.BaseSHA)
	assert.Equal(t, "bbb", mr.DiffRefs.HeadSHA)
	assert.Equal(t, "ccc", mr.DiffRefs.StartSHA)
}

func TestFetchMRDiffs(t *testing.T) {
	p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"old_path":     "main.go",
				"new_path":     "main.go",
				"diff":         "@@ -1,3 +1,4 @@\n package main\n \n+import \"fmt\"\n",
				"new_file":     false,
				"renamed_file": false,
				"deleted_file": false,
			},
		})
	}))

	diffs, err := p.FetchMRDiffs("grp/proj", 42)
	require.NoError(t, err)
	assert.Len(t, diffs, 1)
	assert.Equal(t, "main.go", diffs[0].NewPath)
	assert.Contains(t, diffs[0].Diff, "import")
}

func TestFetchMRRawDiff(t *testing.T) {
	p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "merge_requests/42/raw_diffs")
		_, _ = w.Write([]byte("diff --git a/main.go b/main.go\n@@ -1,1 +1,2 @@\n package main\n+import \"fmt\"\n"))
	}))

	raw, err := p.FetchMRRawDiff("grp/proj", 42)
	require.NoError(t, err)
	assert.Contains(t, raw, "diff --git")
	assert.Contains(t, raw, "@@ -1,1 +1,2 @@")
}

func TestPostSummaryNote(t *testing.T) {
	var gotBody string
	p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		gotBody, _ = req["body"].(string)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": 1})
	}))

	err := p.PostSummaryNote("grp/proj", 42, "Looks good!")
	require.NoError(t, err)
	assert.Equal(t, "Looks good!", gotBody)
}

func TestPostInlineComment(t *testing.T) {
	var gotReq map[string]interface{}
	p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotReq)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": "disc-1"})
	}))

	refs := vcs.DiffRefs{BaseSHA: "aaa", HeadSHA: "bbb", StartSHA: "ccc"}
	comment := vcs.InlineComment{FilePath: "main.go", NewLine: 10, OldLine: 9, Body: "Fix this"}

	err := p.PostInlineComment("grp/proj", 42, refs, comment)
	require.NoError(t, err)
	assert.Equal(t, "Fix this", gotReq["body"])

	pos, ok := gotReq["position"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "aaa", pos["base_sha"])
	assert.Equal(t, "main.go", pos["new_path"])
	assert.Equal(t, "main.go", pos["old_path"])
	assert.Equal(t, float64(10), pos["new_line"])
	assert.Equal(t, float64(9), pos["old_line"])
}

func TestPostInlineComment_UsesExplicitOldPath(t *testing.T) {
	var gotReq map[string]interface{}
	p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotReq)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": "disc-1"})
	}))

	refs := vcs.DiffRefs{BaseSHA: "aaa", HeadSHA: "bbb", StartSHA: "ccc"}
	comment := vcs.InlineComment{
		FilePath: "new/name.go",
		OldPath:  "old/name.go",
		NewLine:  10,
		OldLine:  9,
		Body:     "Fix this",
	}

	err := p.PostInlineComment("grp/proj", 42, refs, comment)
	require.NoError(t, err)

	pos, ok := gotReq["position"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "new/name.go", pos["new_path"])
	assert.Equal(t, "old/name.go", pos["old_path"])
}

func TestListOpenMRs(t *testing.T) {
	p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"iid":           1,
				"title":         "MR one",
				"source_branch": "feat-1",
				"target_branch": "main",
				"state":         "opened",
				"web_url":       "https://gitlab.com/grp/proj/-/merge_requests/1",
				"author":        map[string]interface{}{"username": "dev"},
			},
			{
				"iid":           2,
				"title":         "MR two",
				"source_branch": "feat-2",
				"target_branch": "main",
				"state":         "opened",
				"web_url":       "https://gitlab.com/grp/proj/-/merge_requests/2",
				"author":        map[string]interface{}{"username": "dev2"},
			},
		})
	}))

	mrs, err := p.ListOpenMRs("grp/proj")
	require.NoError(t, err)
	assert.Len(t, mrs, 2)
	assert.Equal(t, "MR one", mrs[0].Title)
	assert.Equal(t, "dev2", mrs[1].Author)
}

func TestListMRDiscussions(t *testing.T) {
	p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"id": "d1",
				"notes": []map[string]interface{}{
					{
						"id":         10,
						"body":       "[HIGH] issue",
						"resolvable": true,
						"resolved":   false,
						"author":     map[string]interface{}{"username": "bot"},
						"position": map[string]interface{}{
							"new_path": "public/index.php",
							"new_line": 29,
						},
					},
				},
			},
		})
	}))

	discussions, err := p.ListMRDiscussions("grp/proj", 42)
	require.NoError(t, err)
	require.Len(t, discussions, 1)
	require.Len(t, discussions[0].Notes, 1)
	assert.Equal(t, "d1", discussions[0].ID)
	assert.Equal(t, "bot", discussions[0].Notes[0].Author)
	assert.Equal(t, "public/index.php", discussions[0].Notes[0].FilePath)
	assert.Equal(t, 29, discussions[0].Notes[0].Line)
}

func TestListMRNotes(t *testing.T) {
	p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"id":     101,
				"body":   "@ange.saadjio pause",
				"author": map[string]interface{}{"username": "maintainer"},
			},
		})
	}))

	notes, err := p.ListMRNotes("grp/proj", 42)
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, int64(101), notes[0].ID)
	assert.Equal(t, "maintainer", notes[0].Author)
	assert.Contains(t, notes[0].Body, "pause")
}

func TestReplyToMRDiscussion(t *testing.T) {
	var gotBody string
	p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/discussions/d1/notes")
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		gotBody, _ = req["body"].(string)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": 11})
	}))

	err := p.ReplyToMRDiscussion("grp/proj", 42, "d1", "reply")
	require.NoError(t, err)
	assert.Equal(t, "reply", gotBody)
}

func TestValidate_EmptyToken(t *testing.T) {
	// NewProvider rejects empty token
	_, err := NewProvider("", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token is required")
}

func TestFormatSuggestionBlock(t *testing.T) {
	p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	result := p.FormatSuggestionBlock("fixed code here")
	assert.Equal(t, "```suggestion:-0+0\nfixed code here\n```", result)
}
