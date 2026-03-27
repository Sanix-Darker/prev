package gitlabnote

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var validCommands = []string{"reply", "summary", "pause", "resume", "review", "ignore"}

type TriggerRequest struct {
	ProjectPath string
	MRIID       int64
	Command     string
	Note        string
}

type noteEvent struct {
	ObjectKind string `json:"object_kind"`
	Project    struct {
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
	MergeRequest struct {
		IID int64 `json:"iid"`
	} `json:"merge_request"`
	ObjectAttributes struct {
		Note         string `json:"note"`
		NoteableType string `json:"noteable_type"`
	} `json:"object_attributes"`
}

func ParseTriggerRequest(secret string, headers http.Header, body []byte) (TriggerRequest, bool, error) {
	if strings.TrimSpace(secret) != "" && strings.TrimSpace(headers.Get("X-Gitlab-Token")) != strings.TrimSpace(secret) {
		return TriggerRequest{}, false, errors.New("gitlab note hook token mismatch")
	}
	if event := strings.TrimSpace(headers.Get("X-Gitlab-Event")); !strings.EqualFold(event, "Note Hook") {
		return TriggerRequest{}, false, nil
	}
	var payload noteEvent
	if err := json.Unmarshal(body, &payload); err != nil {
		return TriggerRequest{}, false, fmt.Errorf("parse gitlab note event: %w", err)
	}
	if !strings.EqualFold(payload.ObjectKind, "note") {
		return TriggerRequest{}, false, nil
	}
	if !strings.EqualFold(payload.ObjectAttributes.NoteableType, "MergeRequest") {
		return TriggerRequest{}, false, nil
	}
	cmd, ok := detectCommand(payload.ObjectAttributes.Note)
	if !ok {
		return TriggerRequest{}, false, nil
	}
	projectPath := strings.TrimSpace(payload.Project.PathWithNamespace)
	if projectPath == "" || payload.MergeRequest.IID <= 0 {
		return TriggerRequest{}, false, errors.New("gitlab note hook payload missing merge request context")
	}
	return TriggerRequest{
		ProjectPath: projectPath,
		MRIID:       payload.MergeRequest.IID,
		Command:     cmd,
		Note:        payload.ObjectAttributes.Note,
	}, true, nil
}

func BuildPrevReviewArgs(trigger TriggerRequest) []string {
	return []string{"mr", "review", trigger.ProjectPath, fmt.Sprintf("%d", trigger.MRIID), "--vcs", "gitlab"}
}

func detectCommand(note string) (string, bool) {
	tokens := tokenize(note)
	if len(tokens) == 0 {
		return "", false
	}
	seenPrev := false
	for _, tok := range tokens {
		if tok == "prev" {
			seenPrev = true
			continue
		}
		if !seenPrev {
			continue
		}
		for _, cmd := range validCommands {
			if tok == cmd {
				return cmd, true
			}
		}
	}
	return "", false
}

func tokenize(note string) []string {
	note = strings.ToLower(note)
	var b strings.Builder
	for _, r := range note {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte(' ')
	}
	return strings.Fields(b.String())
}
