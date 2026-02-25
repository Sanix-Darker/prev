package gitlab

import (
	"fmt"
	"strings"

	"github.com/sanix-darker/prev/internal/vcs"
	gl "gitlab.com/gitlab-org/api/client-go"
)

// Provider implements vcs.VCSProvider for GitLab.
type Provider struct {
	api     *gl.Client
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

	client, err := gl.NewClient(token, gl.WithBaseURL(baseURL+"/api/v4"))
	if err != nil {
		return nil, fmt.Errorf("gitlab: failed to create client: %w", err)
	}

	return &Provider{api: client, baseURL: baseURL, token: token}, nil
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
	mr, _, err := p.api.MergeRequests.GetMergeRequest(projectID, mrIID, nil)
	if err != nil {
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
	opts := &gl.ListMergeRequestDiffsOptions{
		ListOptions: gl.ListOptions{PerPage: 100},
	}

	var allDiffs []vcs.FileDiff
	for {
		diffs, resp, err := p.api.MergeRequests.ListMergeRequestDiffs(projectID, mrIID, opts)
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

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allDiffs, nil
}

func (p *Provider) FetchMRRawDiff(projectID string, mrIID int64) (string, error) {
	raw, _, err := p.api.MergeRequests.ShowMergeRequestRawDiffs(
		projectID, mrIID, &gl.ShowMergeRequestRawDiffsOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("gitlab: failed to fetch MR raw diff: %w", err)
	}
	return strings.TrimSpace(string(raw)), nil
}

func (p *Provider) ListMRDiscussions(projectID string, mrIID int64) ([]vcs.MRDiscussion, error) {
	opts := &gl.ListMergeRequestDiscussionsOptions{
		ListOptions: gl.ListOptions{PerPage: 100},
	}

	var out []vcs.MRDiscussion
	for {
		discussions, resp, err := p.api.Discussions.ListMergeRequestDiscussions(projectID, mrIID, opts)
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
					note.Line = int(n.Position.NewLine)
				}
				thread.Notes = append(thread.Notes, note)
			}
			out = append(out, thread)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return out, nil
}

func (p *Provider) ListMRNotes(projectID string, mrIID int64) ([]vcs.MRNote, error) {
	opts := &gl.ListMergeRequestNotesOptions{
		ListOptions: gl.ListOptions{PerPage: 100},
	}

	var out []vcs.MRNote
	for {
		notes, resp, err := p.api.Notes.ListMergeRequestNotes(projectID, mrIID, opts)
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
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return out, nil
}

func (p *Provider) ListOpenMRs(projectID string) ([]*vcs.MergeRequest, error) {
	state := "opened"
	opts := &gl.ListProjectMergeRequestsOptions{
		State:       &state,
		ListOptions: gl.ListOptions{PerPage: 20},
	}

	mrs, _, err := p.api.MergeRequests.ListProjectMergeRequests(projectID, opts)
	if err != nil {
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
	_, _, err := p.api.Notes.CreateMergeRequestNote(projectID, mrIID, &gl.CreateMergeRequestNoteOptions{
		Body: &body,
	})
	if err != nil {
		return fmt.Errorf("gitlab: failed to post MR note: %w", err)
	}
	return nil
}

func (p *Provider) PostInlineComment(projectID string, mrIID int64, refs vcs.DiffRefs, comment vcs.InlineComment) error {
	posType := "text"
	filePath := comment.FilePath
	oldLine := comment.OldLine
	position := &gl.PositionOptions{
		BaseSHA:      &refs.BaseSHA,
		HeadSHA:      &refs.HeadSHA,
		StartSHA:     &refs.StartSHA,
		PositionType: &posType,
		NewPath:      &filePath,
		OldPath:      &filePath,
		NewLine:      &comment.NewLine,
	}
	if oldLine > 0 {
		position.OldLine = &oldLine
	}

	_, _, err := p.api.Discussions.CreateMergeRequestDiscussion(projectID, mrIID, &gl.CreateMergeRequestDiscussionOptions{
		Body:     &comment.Body,
		Position: position,
	})
	if err != nil {
		return fmt.Errorf("gitlab: failed to post inline discussion: %w", err)
	}
	return nil
}

func (p *Provider) ReplyToMRDiscussion(projectID string, mrIID int64, discussionID, body string) error {
	_, _, err := p.api.Discussions.AddMergeRequestDiscussionNote(
		projectID,
		mrIID,
		discussionID,
		&gl.AddMergeRequestDiscussionNoteOptions{Body: &body},
	)
	if err != nil {
		return fmt.Errorf("gitlab: failed to reply to discussion %s: %w", discussionID, err)
	}
	return nil
}

// FormatSuggestionBlock returns a GitLab-native suggestion code block.
func (p *Provider) FormatSuggestionBlock(suggestion string) string {
	return "```suggestion:-0+0\n" + suggestion + "\n```"
}
