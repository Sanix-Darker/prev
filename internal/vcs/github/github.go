package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sanix-darker/prev/internal/vcs"
)

// Provider implements vcs.VCSProvider for GitHub.
type Provider struct {
	client  *http.Client
	baseURL string
	token   string
}

func init() {
	vcs.Register("github", NewProvider)
}

// NewProvider creates a GitHub VCSProvider.
func NewProvider(token, baseURL string) (vcs.VCSProvider, error) {
	if token == "" {
		return nil, fmt.Errorf("github: token is required")
	}
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}

	return &Provider{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
	}, nil
}

func (p *Provider) Info() vcs.ProviderInfo {
	return vcs.ProviderInfo{Name: "github", BaseURL: p.baseURL}
}

func (p *Provider) Validate() error {
	if p.token == "" {
		return fmt.Errorf("github: token is required")
	}
	return nil
}

func (p *Provider) FetchMR(projectID string, mrIID int64) (*vcs.MergeRequest, error) {
	var pr struct {
		Number int64  `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		User   struct {
			Login string `json:"login"`
		} `json:"user"`
		Head struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"base"`
		State   string `json:"state"`
		HTMLURL string `json:"html_url"`
	}

	if err := p.getJSON(context.Background(), fmt.Sprintf("/repos/%s/pulls/%d", projectID, mrIID), &pr); err != nil {
		return nil, fmt.Errorf("github: failed to fetch PR #%d: %w", mrIID, err)
	}

	return &vcs.MergeRequest{
		IID:          pr.Number,
		Title:        pr.Title,
		Description:  pr.Body,
		Author:       pr.User.Login,
		SourceBranch: pr.Head.Ref,
		TargetBranch: pr.Base.Ref,
		State:        pr.State,
		WebURL:       pr.HTMLURL,
		DiffRefs: vcs.DiffRefs{
			BaseSHA:  pr.Base.SHA,
			HeadSHA:  pr.Head.SHA,
			StartSHA: pr.Base.SHA,
		},
	}, nil
}

func (p *Provider) FetchMRDiffs(projectID string, mrIID int64) ([]vcs.FileDiff, error) {
	type prFile struct {
		Filename         string `json:"filename"`
		PreviousFilename string `json:"previous_filename"`
		Status           string `json:"status"`
		Patch            string `json:"patch"`
	}

	var all []vcs.FileDiff
	page := 1
	for {
		endpoint := fmt.Sprintf("/repos/%s/pulls/%d/files?per_page=100&page=%d", projectID, mrIID, page)
		var files []prFile
		resp, err := p.getJSONWithResponse(context.Background(), endpoint, &files)
		if err != nil {
			return nil, fmt.Errorf("github: failed to fetch PR files: %w", err)
		}

		for _, f := range files {
			oldPath := f.PreviousFilename
			if oldPath == "" {
				oldPath = f.Filename
			}
			newPath := f.Filename
			status := strings.ToLower(f.Status)

			all = append(all, vcs.FileDiff{
				OldPath:     oldPath,
				NewPath:     newPath,
				Diff:        f.Patch,
				NewFile:     status == "added",
				DeletedFile: status == "removed",
				RenamedFile: status == "renamed",
			})
		}

		if !hasNextPage(resp.Header.Get("Link")) {
			break
		}
		page++
	}

	return all, nil
}

func (p *Provider) FetchMRRawDiff(projectID string, mrIID int64) (string, error) {
	req, err := p.newRequest(context.Background(), http.MethodGet,
		fmt.Sprintf("/repos/%s/pulls/%d", projectID, mrIID),
		nil,
	)
	if err != nil {
		return "", err
	}
	// Ask GitHub to return the raw diff.
	req.Header.Set("Accept", "application/vnd.github.v3.diff")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("github: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

func (p *Provider) ListMRDiscussions(projectID string, mrIID int64) ([]vcs.MRDiscussion, error) {
	type reviewComment struct {
		ID           int64  `json:"id"`
		InReplyToID  *int64 `json:"in_reply_to_id"`
		Body         string `json:"body"`
		Path         string `json:"path"`
		Line         int    `json:"line"`
		OriginalLine int    `json:"original_line"`
		User         struct {
			Login string `json:"login"`
		} `json:"user"`
	}

	threads := map[string][]vcs.MRDiscussionNote{}
	order := make([]string, 0, 64)
	page := 1
	for {
		endpoint := fmt.Sprintf("/repos/%s/pulls/%d/comments?per_page=100&page=%d", projectID, mrIID, page)
		var comments []reviewComment
		resp, err := p.getJSONWithResponse(context.Background(), endpoint, &comments)
		if err != nil {
			return nil, fmt.Errorf("github: failed to list PR review comments: %w", err)
		}

		for _, c := range comments {
			threadID := c.ID
			if c.InReplyToID != nil && *c.InReplyToID > 0 {
				threadID = *c.InReplyToID
			}
			key := strconv.FormatInt(threadID, 10)
			if _, ok := threads[key]; !ok {
				order = append(order, key)
			}
			line := c.Line
			if line <= 0 {
				line = c.OriginalLine
			}
			threads[key] = append(threads[key], vcs.MRDiscussionNote{
				ID:         c.ID,
				Author:     c.User.Login,
				Body:       c.Body,
				FilePath:   c.Path,
				Line:       line,
				Resolvable: true,
				Resolved:   false,
			})
		}

		if !hasNextPage(resp.Header.Get("Link")) {
			break
		}
		page++
	}

	out := make([]vcs.MRDiscussion, 0, len(order))
	for _, id := range order {
		out = append(out, vcs.MRDiscussion{
			ID:    id,
			Notes: threads[id],
		})
	}
	return out, nil
}

func (p *Provider) ListMRNotes(projectID string, mrIID int64) ([]vcs.MRNote, error) {
	type note struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	}

	var out []vcs.MRNote
	page := 1
	for {
		endpoint := fmt.Sprintf("/repos/%s/issues/%d/comments?per_page=100&page=%d", projectID, mrIID, page)
		var notes []note
		resp, err := p.getJSONWithResponse(context.Background(), endpoint, &notes)
		if err != nil {
			return nil, fmt.Errorf("github: failed to list PR notes: %w", err)
		}

		for _, n := range notes {
			out = append(out, vcs.MRNote{
				ID:     n.ID,
				Author: n.User.Login,
				Body:   n.Body,
			})
		}

		if !hasNextPage(resp.Header.Get("Link")) {
			break
		}
		page++
	}

	return out, nil
}

func (p *Provider) ListOpenMRs(projectID string) ([]*vcs.MergeRequest, error) {
	var prs []struct {
		Number int64  `json:"number"`
		Title  string `json:"title"`
		User   struct {
			Login string `json:"login"`
		} `json:"user"`
		Head struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		State   string `json:"state"`
		HTMLURL string `json:"html_url"`
	}

	endpoint := fmt.Sprintf("/repos/%s/pulls?state=open&per_page=20", projectID)
	if err := p.getJSON(context.Background(), endpoint, &prs); err != nil {
		return nil, fmt.Errorf("github: failed to list PRs: %w", err)
	}

	var result []*vcs.MergeRequest
	for _, pr := range prs {
		result = append(result, &vcs.MergeRequest{
			IID:          pr.Number,
			Title:        pr.Title,
			Author:       pr.User.Login,
			SourceBranch: pr.Head.Ref,
			TargetBranch: pr.Base.Ref,
			State:        pr.State,
			WebURL:       pr.HTMLURL,
		})
	}
	return result, nil
}

func (p *Provider) PostSummaryNote(projectID string, mrIID int64, body string) error {
	payload := map[string]string{"body": body}
	if err := p.postJSON(context.Background(),
		fmt.Sprintf("/repos/%s/issues/%d/comments", projectID, mrIID),
		payload,
		nil,
	); err != nil {
		return fmt.Errorf("github: failed to post PR summary: %w", err)
	}
	return nil
}

func (p *Provider) PostInlineComment(projectID string, mrIID int64, refs vcs.DiffRefs, comment vcs.InlineComment) error {
	if refs.HeadSHA == "" {
		return fmt.Errorf("github: missing head SHA for inline comment")
	}
	if comment.NewLine <= 0 {
		return fmt.Errorf("github: invalid line number for inline comment")
	}

	payload := map[string]interface{}{
		"body":      comment.Body,
		"commit_id": refs.HeadSHA,
		"path":      comment.FilePath,
		"line":      comment.NewLine,
		"side":      "RIGHT",
	}

	if err := p.postJSON(context.Background(),
		fmt.Sprintf("/repos/%s/pulls/%d/comments", projectID, mrIID),
		payload,
		nil,
	); err != nil {
		return fmt.Errorf("github: failed to post inline comment: %w", err)
	}
	return nil
}

func (p *Provider) ReplyToMRDiscussion(projectID string, mrIID int64, discussionID, body string) error {
	parentID, err := strconv.ParseInt(strings.TrimSpace(discussionID), 10, 64)
	if err != nil || parentID <= 0 {
		return fmt.Errorf("github: invalid discussion id %q for reply", discussionID)
	}
	payload := map[string]interface{}{
		"body":        body,
		"in_reply_to": parentID,
	}
	if err := p.postJSON(context.Background(),
		fmt.Sprintf("/repos/%s/pulls/%d/comments", projectID, mrIID),
		payload,
		nil,
	); err != nil {
		return fmt.Errorf("github: failed to reply to discussion %s: %w", discussionID, err)
	}
	return nil
}

// FormatSuggestionBlock returns a GitHub-native suggestion code block.
func (p *Provider) FormatSuggestionBlock(suggestion string) string {
	return "```suggestion\n" + suggestion + "\n```"
}

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
		return resp, fmt.Errorf("github: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
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
		return fmt.Errorf("github: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
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
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "prev-cli")
	req.Header.Set("Authorization", "Bearer "+p.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func hasNextPage(linkHeader string) bool {
	if linkHeader == "" {
		return false
	}
	parts := strings.Split(linkHeader, ",")
	for _, part := range parts {
		if strings.Contains(part, `rel="next"`) {
			return true
		}
	}
	return false
}
