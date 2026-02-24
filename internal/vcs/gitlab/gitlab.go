package gitlab

import (
	"fmt"

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
	_, _, err := p.api.Discussions.CreateMergeRequestDiscussion(projectID, mrIID, &gl.CreateMergeRequestDiscussionOptions{
		Body: &comment.Body,
		Position: &gl.PositionOptions{
			BaseSHA:      &refs.BaseSHA,
			HeadSHA:      &refs.HeadSHA,
			StartSHA:     &refs.StartSHA,
			PositionType: &posType,
			NewPath:      &comment.FilePath,
			NewLine:      &comment.NewLine,
		},
	})
	if err != nil {
		return fmt.Errorf("gitlab: failed to post inline discussion: %w", err)
	}
	return nil
}

// FormatSuggestionBlock returns a GitLab-native suggestion code block.
func (p *Provider) FormatSuggestionBlock(suggestion string) string {
	return "```suggestion:-0+0\n" + suggestion + "\n```"
}
