package github

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanix-darker/prev/internal/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider_FetchMRAndDiffs(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")

		switch r.URL.Path {
		case "/repos/acme/blog/pulls/42":
			resp := map[string]interface{}{
				"number":   42,
				"title":    "Add recipe endpoints",
				"body":     "Adds API endpoints for posts.",
				"user":     map[string]interface{}{"login": "octo"},
				"head":     map[string]interface{}{"ref": "feature", "sha": "headsha"},
				"base":     map[string]interface{}{"ref": "main", "sha": "basesha"},
				"state":    "open",
				"html_url": "https://example.com/pr/42",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case "/repos/acme/blog/pulls/42/files":
			resp := []map[string]interface{}{
				{
					"filename": "public/index.php",
					"status":   "modified",
					"patch":    "@@ -1,2 +1,2 @@\n- old\n+ new\n",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	p, err := NewProvider("token-123", server.URL)
	require.NoError(t, err)

	mr, err := p.FetchMR("acme/blog", 42)
	require.NoError(t, err)
	assert.Equal(t, int64(42), mr.IID)
	assert.Equal(t, "Add recipe endpoints", mr.Title)
	assert.Equal(t, "feature", mr.SourceBranch)
	assert.Equal(t, "main", mr.TargetBranch)
	assert.Equal(t, "headsha", mr.DiffRefs.HeadSHA)
	assert.Equal(t, "basesha", mr.DiffRefs.BaseSHA)
	assert.Equal(t, "Bearer token-123", gotAuth)

	diffs, err := p.FetchMRDiffs("acme/blog", 42)
	require.NoError(t, err)
	require.Len(t, diffs, 1)
	assert.Equal(t, "public/index.php", diffs[0].NewPath)
	assert.Contains(t, diffs[0].Diff, "+ new")
}

func TestProvider_PostComments(t *testing.T) {
	var summaryBody string
	var inlineBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/blog/issues/42/comments":
			body, _ := io.ReadAll(r.Body)
			defer r.Body.Close()
			var payload map[string]string
			_ = json.Unmarshal(body, &payload)
			summaryBody = payload["body"]
		case "/repos/acme/blog/pulls/42/comments":
			body, _ := io.ReadAll(r.Body)
			defer r.Body.Close()
			_ = json.Unmarshal(body, &inlineBody)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	p, err := NewProvider("token-123", server.URL)
	require.NoError(t, err)

	err = p.PostSummaryNote("acme/blog", 42, "summary")
	require.NoError(t, err)
	assert.Equal(t, "summary", summaryBody)

	err = p.PostInlineComment("acme/blog", 42, vcs.DiffRefs{
		HeadSHA: "headsha",
	}, vcs.InlineComment{
		FilePath: "public/index.php",
		NewLine:  12,
		Body:     "inline",
	})
	require.NoError(t, err)
	assert.Equal(t, "inline", inlineBody["body"])
	assert.Equal(t, "headsha", inlineBody["commit_id"])
	assert.Equal(t, "public/index.php", inlineBody["path"])
	assert.Equal(t, float64(12), inlineBody["line"])
	assert.Equal(t, "RIGHT", inlineBody["side"])
}

func TestHasNextPage(t *testing.T) {
	assert.True(t, hasNextPage(`<https://api.github.com/resource?page=2>; rel="next"`))
	assert.False(t, hasNextPage(`<https://api.github.com/resource?page=2>; rel="prev"`))
}

func TestProvider_ListMRDiscussions_GroupsReviewThreads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/blog/pulls/42/comments" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		resp := []map[string]interface{}{
			{
				"id":             101,
				"body":           "[HIGH] First finding",
				"path":           "public/index.php",
				"line":           31,
				"in_reply_to_id": nil,
				"user":           map[string]interface{}{"login": "bot"},
			},
			{
				"id":             102,
				"body":           "Follow-up",
				"path":           "public/index.php",
				"line":           31,
				"in_reply_to_id": 101,
				"user":           map[string]interface{}{"login": "dev"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p, err := NewProvider("token-123", server.URL)
	require.NoError(t, err)

	discussions, err := p.ListMRDiscussions("acme/blog", 42)
	require.NoError(t, err)
	require.Len(t, discussions, 1)
	assert.Equal(t, "101", discussions[0].ID)
	require.Len(t, discussions[0].Notes, 2)
	assert.Equal(t, "public/index.php", discussions[0].Notes[0].FilePath)
	assert.Equal(t, 31, discussions[0].Notes[0].Line)
}

func TestProvider_ReplyToMRDiscussion(t *testing.T) {
	var payload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/blog/pulls/42/comments" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		_ = json.Unmarshal(body, &payload)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	p, err := NewProvider("token-123", server.URL)
	require.NoError(t, err)

	err = p.ReplyToMRDiscussion("acme/blog", 42, "101", "reply body")
	require.NoError(t, err)
	assert.Equal(t, "reply body", payload["body"])
	assert.Equal(t, float64(101), payload["in_reply_to"])
}
