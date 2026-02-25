package vcs

// VCSProvider abstracts version control system operations (GitLab, GitHub, etc.).
type VCSProvider interface {
	Info() ProviderInfo
	FetchMR(projectID string, mrIID int64) (*MergeRequest, error)
	FetchMRDiffs(projectID string, mrIID int64) ([]FileDiff, error)
	FetchMRRawDiff(projectID string, mrIID int64) (string, error)
	ListMRDiscussions(projectID string, mrIID int64) ([]MRDiscussion, error)
	ListMRNotes(projectID string, mrIID int64) ([]MRNote, error)
	ListOpenMRs(projectID string) ([]*MergeRequest, error)
	PostSummaryNote(projectID string, mrIID int64, body string) error
	PostInlineComment(projectID string, mrIID int64, refs DiffRefs, comment InlineComment) error
	ReplyToMRDiscussion(projectID string, mrIID int64, discussionID, body string) error
	FormatSuggestionBlock(suggestion string) string
	Validate() error
}

// ProviderInfo describes a VCS provider.
type ProviderInfo struct {
	Name    string
	BaseURL string
}

// MergeRequest holds platform-agnostic merge/pull request metadata.
type MergeRequest struct {
	IID          int64
	Title        string
	Description  string
	Author       string
	SourceBranch string
	TargetBranch string
	State        string
	WebURL       string
	DiffRefs     DiffRefs
}

// DiffRefs holds the SHA references needed for inline comments.
type DiffRefs struct {
	BaseSHA  string
	HeadSHA  string
	StartSHA string
}

// FileDiff represents a single file's diff in a merge/pull request.
type FileDiff struct {
	OldPath     string
	NewPath     string
	Diff        string
	NewFile     bool
	RenamedFile bool
	DeletedFile bool
	AMode       string
	BMode       string
}

// InlineComment holds data for posting an inline comment on a diff.
type InlineComment struct {
	FilePath string
	NewLine  int64
	OldLine  int64
	Body     string
}

// MRDiscussion represents one MR discussion thread.
type MRDiscussion struct {
	ID    string
	Notes []MRDiscussionNote
}

// MRDiscussionNote represents one note in an MR discussion.
type MRDiscussionNote struct {
	ID         int64
	Author     string
	Body       string
	FilePath   string
	Line       int
	Resolved   bool
	Resolvable bool
}

// MRNote represents one top-level MR note/comment (non-thread).
type MRNote struct {
	ID     int64
	Author string
	Body   string
}

// Pipeline holds basic CI pipeline info.
type Pipeline struct {
	ID     int64
	Status string
	Ref    string
	WebURL string
}
