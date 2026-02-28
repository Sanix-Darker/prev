package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sanix-darker/prev/internal/vcs"
)

// Provider implements vcs.VCSProvider for GitLab.
type Provider struct {
	client  *http.Client
	baseURL string
	token   string
}

func init() {
	vcs.Register("gitlab", NewProvider)
}

// NewProvider creates a GitLab VCSProvider.
func NewProvider(token, baseURL string) (vcs.VCSProvider, error) {
	if token == "" {
		return nil, fmt.Errorf("gitlab: token is required")
	}
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}

	return &Provider{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
	}, nil
}

func (p *Provider) Info() vcs.ProviderInfo {
	return vcs.ProviderInfo{Name: "gitlab", BaseURL: p.baseURL}
}

func (p *Provider) Validate() error {
	if p.token == "" {
		return fmt.Errorf("gitlab: token is required")
	}
	return nil
}

func (p *Provider) FetchMR(projectID string, mrIID int64) (*vcs.MergeRequest, error) {
	var mr struct {
		IID         int64  `json:"iid"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Author      struct {
			Username string `json:"username"`
		} `json:"author"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		State        string `json:"state"`
		WebURL       string `json:"web_url"`
		DiffRefs     struct {
			BaseSha  string `json:"base_sha"`
			HeadSha  string `json:"head_sha"`
			StartSha string `json:"start_sha"`
		} `json:"diff_refs"`
	}

	endpoint := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d", url.PathEscape(projectID), mrIID)
	if err := p.getJSON(context.Background(), endpoint, &mr); err != nil {
		return nil, fmt.Errorf("gitlab: failed to fetch MR !%d: %w", mrIID, err)
	}

	return &vcs.MergeRequest{
		IID:          mr.IID,
		Title:        mr.Title,
		Description:  mr.Description,
		Author:       mr.Author.Username,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		State:        mr.State,
		WebURL:       mr.WebURL,
		DiffRefs: vcs.DiffRefs{
			BaseSHA:  mr.DiffRefs.BaseSha,
			HeadSHA:  mr.DiffRefs.HeadSha,
			StartSHA: mr.DiffRefs.StartSha,
		},
	}, nil
}

func (p *Provider) FetchMRDiffs(projectID string, mrIID int64) ([]vcs.FileDiff, error) {
	type apiDiff struct {
		OldPath     string `json:"old_path"`
		NewPath     string `json:"new_path"`
		Diff        string `json:"diff"`
		NewFile     bool   `json:"new_file"`
		RenamedFile bool   `json:"renamed_file"`
		DeletedFile bool   `json:"deleted_file"`
		AMode       string `json:"a_mode"`
		BMode       string `json:"b_mode"`
	}

	var allDiffs []vcs.FileDiff
	page := 1
	for {
		endpoint := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/diffs?per_page=100&page=%d",
			url.PathEscape(projectID), mrIID, page)
		var diffs []apiDiff
		resp, err := p.getJSONWithResponse(context.Background(), endpoint, &diffs)
		if err != nil {
			return nil, fmt.Errorf("gitlab: failed to fetch MR diffs: %w", err)
		}

		for _, d := range diffs {
			allDiffs = append(allDiffs, vcs.FileDiff{
				OldPath:     d.OldPath,
				NewPath:     d.NewPath,
				Diff:        d.Diff,
				NewFile:     d.NewFile,
				RenamedFile: d.RenamedFile,
				DeletedFile: d.DeletedFile,
				AMode:       d.AMode,
				BMode:       d.BMode,
			})
		}

		if !hasNextPage(resp.Header.Get("X-Next-Page")) {
			break
		}
		page++
	}

	return allDiffs, nil
}

func (p *Provider) FetchMRRawDiff(projectID string, mrIID int64) (string, error) {
	endpoint := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/raw_diffs",
		url.PathEscape(projectID), mrIID)

	req, err := p.newRequest(context.Background(), http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gitlab: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

func (p *Provider) ListMRDiscussions(projectID string, mrIID int64) ([]vcs.MRDiscussion, error) {
	type apiNote struct {
		ID         int64  `json:"id"`
		Body       string `json:"body"`
		Resolved   bool   `json:"resolved"`
		Resolvable bool   `json:"resolvable"`
		Author     struct {
			Username string `json:"username"`
		} `json:"author"`
		Position *struct {
			NewPath string `json:"new_path"`
			NewLine int    `json:"new_line"`
		} `json:"position"`
	}
	type apiDiscussion struct {
		ID    string    `json:"id"`
		Notes []apiNote `json:"notes"`
	}

	var out []vcs.MRDiscussion
	page := 1
	for {
		endpoint := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/discussions?per_page=100&page=%d",
			url.PathEscape(projectID), mrIID, page)
		var discussions []apiDiscussion
		resp, err := p.getJSONWithResponse(context.Background(), endpoint, &discussions)
		if err != nil {
			return nil, fmt.Errorf("gitlab: failed to list MR discussions: %w", err)
		}

		for _, d := range discussions {
			thread := vcs.MRDiscussion{ID: d.ID}
			for _, n := range d.Notes {
				note := vcs.MRDiscussionNote{
					ID:         n.ID,
					Author:     n.Author.Username,
					Body:       n.Body,
					Resolved:   n.Resolved,
					Resolvable: n.Resolvable,
				}
				if n.Position != nil {
					note.FilePath = n.Position.NewPath
					note.Line = n.Position.NewLine
				}
				thread.Notes = append(thread.Notes, note)
			}
			out = append(out, thread)
		}

		if !hasNextPage(resp.Header.Get("X-Next-Page")) {
			break
		}
		page++
	}

	return out, nil
}

func (p *Provider) ListMRNotes(projectID string, mrIID int64) ([]vcs.MRNote, error) {
	type apiNote struct {
		ID     int64  `json:"id"`
		Body   string `json:"body"`
		Author struct {
			Username string `json:"username"`
		} `json:"author"`
	}

	var out []vcs.MRNote
	page := 1
	for {
		endpoint := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/notes?per_page=100&page=%d",
			url.PathEscape(projectID), mrIID, page)
		var notes []apiNote
		resp, err := p.getJSONWithResponse(context.Background(), endpoint, &notes)
		if err != nil {
			return nil, fmt.Errorf("gitlab: failed to list MR notes: %w", err)
		}
		for _, n := range notes {
			out = append(out, vcs.MRNote{
				ID:     n.ID,
				Author: n.Author.Username,
				Body:   n.Body,
			})
		}
		if !hasNextPage(resp.Header.Get("X-Next-Page")) {
			break
		}
		page++
	}

	return out, nil
}

func (p *Provider) ListOpenMRs(projectID string) ([]*vcs.MergeRequest, error) {
	type apiMR struct {
		IID    int64  `json:"iid"`
		Title  string `json:"title"`
		Author struct {
			Username string `json:"username"`
		} `json:"author"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		State        string `json:"state"`
		WebURL       string `json:"web_url"`
	}

	endpoint := fmt.Sprintf("/api/v4/projects/%s/merge_requests?state=opened&per_page=20",
		url.PathEscape(projectID))
	var mrs []apiMR
	if err := p.getJSON(context.Background(), endpoint, &mrs); err != nil {
		return nil, fmt.Errorf("gitlab: failed to list MRs: %w", err)
	}

	var result []*vcs.MergeRequest
	for _, mr := range mrs {
		result = append(result, &vcs.MergeRequest{
			IID:          mr.IID,
			Title:        mr.Title,
			Author:       mr.Author.Username,
			SourceBranch: mr.SourceBranch,
			TargetBranch: mr.TargetBranch,
			State:        mr.State,
			WebURL:       mr.WebURL,
		})
	}

	return result, nil
}

func (p *Provider) PostSummaryNote(projectID string, mrIID int64, body string) error {
	payload := map[string]string{"body": body}
	if err := p.postJSON(context.Background(),
		fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/notes", url.PathEscape(projectID), mrIID),
		payload,
		nil,
	); err != nil {
		return fmt.Errorf("gitlab: failed to post MR note: %w", err)
	}
	return nil
}

func (p *Provider) PostInlineComment(projectID string, mrIID int64, refs vcs.DiffRefs, comment vcs.InlineComment) error {
	oldPath := strings.TrimSpace(comment.OldPath)
	if oldPath == "" {
		oldPath = comment.FilePath
	}
	position := map[string]interface{}{
		"base_sha":      refs.BaseSHA,
		"head_sha":      refs.HeadSHA,
		"start_sha":     refs.StartSHA,
		"position_type": "text",
		"new_path":      comment.FilePath,
		"old_path":      oldPath,
		"new_line":      comment.NewLine,
	}
	if comment.OldLine > 0 {
		position["old_line"] = comment.OldLine
	}

	payload := map[string]interface{}{
		"body":     comment.Body,
		"position": position,
	}

	if err := p.postJSON(context.Background(),
		fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/discussions", url.PathEscape(projectID), mrIID),
		payload,
		nil,
	); err != nil {
		return fmt.Errorf("gitlab: failed to post inline discussion: %w", err)
	}
	return nil
}

func (p *Provider) ReplyToMRDiscussion(projectID string, mrIID int64, discussionID, body string) error {
	payload := map[string]string{"body": body}
	endpoint := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/discussions/%s/notes",
		url.PathEscape(projectID), mrIID, discussionID)

	if err := p.postJSON(context.Background(), endpoint, payload, nil); err != nil {
		return fmt.Errorf("gitlab: failed to reply to discussion %s: %w", discussionID, err)
	}
	return nil
}

// FormatSuggestionBlock returns a GitLab-native suggestion code block.
func (p *Provider) FormatSuggestionBlock(suggestion string) string {
	return "```suggestion:-0+0\n" + suggestion + "\n```"
}

// --- HTTP helpers (same pattern as github provider) ---

func (p *Provider) getJSON(ctx context.Context, endpoint string, out interface{}) error {
	_, err := p.getJSONWithResponse(ctx, endpoint, out)
	return err
}

func (p *Provider) getJSONWithResponse(ctx context.Context, endpoint string, out interface{}) (*http.Response, error) {
	req, err := p.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return resp, fmt.Errorf("gitlab: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp, err
		}
	}

	return resp, nil
}

func (p *Provider) postJSON(ctx context.Context, endpoint string, payload interface{}, out interface{}) error {
	var buf io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		buf = bytes.NewReader(data)
	}

	req, err := p.newRequest(ctx, http.MethodPost, endpoint, buf)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gitlab: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (p *Provider) newRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	u, err := url.Parse(p.baseURL + endpoint)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "prev-cli")
	req.Header.Set("PRIVATE-TOKEN", p.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func hasNextPage(nextPageHeader string) bool {
	return nextPageHeader != "" && nextPageHeader != "0"
}
