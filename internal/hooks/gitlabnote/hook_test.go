package gitlabnote

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTriggerRequest_AcceptsMergeRequestNoteCommand(t *testing.T) {
	headers := http.Header{
		"X-Gitlab-Event": []string{"Note Hook"},
		"X-Gitlab-Token": []string{"secret"},
	}
	body := []byte(`{
	  "object_kind": "note",
	  "project": {"path_with_namespace": "grp/proj"},
	  "merge_request": {"iid": 42},
	  "object_attributes": {
	    "note": "prev reply can you expand on this?",
	    "noteable_type": "MergeRequest"
	  }
	}`)

	trigger, ok, err := ParseTriggerRequest("secret", headers, body)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "grp/proj", trigger.ProjectPath)
	assert.EqualValues(t, 42, trigger.MRIID)
	assert.Equal(t, "reply", trigger.Command)
}

func TestParseTriggerRequest_RejectsWrongSecret(t *testing.T) {
	headers := http.Header{
		"X-Gitlab-Event": []string{"Note Hook"},
		"X-Gitlab-Token": []string{"wrong"},
	}
	body := []byte(`{"object_kind":"note"}`)

	_, ok, err := ParseTriggerRequest("secret", headers, body)
	assert.Error(t, err)
	assert.False(t, ok)
}

func TestParseTriggerRequest_IgnoresNonCommandNote(t *testing.T) {
	headers := http.Header{
		"X-Gitlab-Event": []string{"Note Hook"},
	}
	body := []byte(`{
	  "object_kind": "note",
	  "project": {"path_with_namespace": "grp/proj"},
	  "merge_request": {"iid": 42},
	  "object_attributes": {
	    "note": "looks good to me",
	    "noteable_type": "MergeRequest"
	  }
	}`)

	_, ok, err := ParseTriggerRequest("", headers, body)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestBuildPrevReviewArgs(t *testing.T) {
	args := BuildPrevReviewArgs(TriggerRequest{ProjectPath: "grp/proj", MRIID: 7})
	assert.Equal(t, []string{"mr", "review", "grp/proj", "7", "--vcs", "gitlab"}, args)
}
