//go:build e2e

package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary
	tmpDir, err := os.MkdirTemp("", "prev-e2e-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath = filepath.Join(tmpDir, "prev")
	cmd := exec.Command("go", "build", "-o", binaryPath, "..")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build binary: " + err.Error())
	}

	os.Exit(m.Run())
}

func runPrev(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return stdout.String(), stderr.String(), exitCode
}

func TestE2E_Version(t *testing.T) {
	stdout, _, exitCode := runPrev(t, "version")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "prev")
	assert.Contains(t, stdout, "Go version")
}

func TestE2E_HelpText(t *testing.T) {
	commands := []string{"", "diff", "optim", "branch", "commit", "mr", "ai", "config"}
	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			args := []string{"--help"}
			if cmd != "" {
				args = []string{cmd, "--help"}
			}
			stdout, _, exitCode := runPrev(t, args...)
			assert.Equal(t, 0, exitCode)
			assert.NotEmpty(t, stdout)
		})
	}
}

func TestE2E_AIList(t *testing.T) {
	stdout, _, exitCode := runPrev(t, "ai", "list")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "openai")
	assert.Contains(t, stdout, "anthropic")
}

func TestE2E_ConfigShow(t *testing.T) {
	stdout, _, exitCode := runPrev(t, "config", "show")
	assert.Equal(t, 0, exitCode)
	assert.NotEmpty(t, stdout)
}

func TestE2E_DiffCommand(t *testing.T) {
	// Start a mock OpenAI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":    "chatcmpl-test",
			"model": "gpt-4o",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "## Review\n\nLooks good!",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Run diff command with mock server
	cmd := exec.Command(binaryPath, "diff", "fixtures/test_diff1.py,fixtures/test_diff2.py", "--stream=false")
	cmd.Dir = filepath.Join("..")
	cmd.Env = append(os.Environ(),
		"OPENAI_API_KEY=test-key",
		"OPENAI_API_BASE="+server.URL,
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "output: %s", string(out))
	assert.Contains(t, string(out), "Review")
}

func TestE2E_MRReviewHelp(t *testing.T) {
	stdout, _, exitCode := runPrev(t, "mr", "review", "--help")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "--strictness")
	assert.Contains(t, stdout, "--vcs")
	assert.Contains(t, stdout, "--dry-run")
	assert.Contains(t, stdout, "--summary-only")
}

func TestE2E_BranchReview(t *testing.T) {
	// Create a temp git repo
	repoDir, err := os.MkdirTemp("", "prev-e2e-branch-*")
	require.NoError(t, err)
	defer os.RemoveAll(repoDir)

	gitRun := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, string(out))
	}

	gitRun("init", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "app.go"),
		[]byte("package main\n\nfunc main() {}\n"), 0644))
	gitRun("add", ".")
	gitRun("commit", "-m", "initial")

	gitRun("checkout", "-b", "feature")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "app.go"),
		[]byte("package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello\") }\n"), 0644))
	gitRun("add", ".")
	gitRun("commit", "-m", "add greeting")
	gitRun("checkout", "main")

	// Mock OpenAI server that returns different responses per call
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var content string
		if callCount == 1 {
			content = "## Summary\nThis branch adds a greeting.\n\n## Changes\n| File | Type | Summary |\n|------|------|---------|\n| app.go | Modified | Added hello output |\n"
		} else {
			content = "**app.go:5** [MEDIUM]: Consider using log instead of fmt for production.\n"
		}
		resp := map[string]interface{}{
			"id":    "chatcmpl-test",
			"model": "gpt-4o",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": content,
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens": 100, "completion_tokens": 50, "total_tokens": 150,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cmd := exec.Command(binaryPath, "branch", "feature",
		"--repo", repoDir,
		"--legacy=false",
		"--serena=off",
		"--stream=false",
	)
	cmd.Env = append(os.Environ(),
		"OPENAI_API_KEY=test-key",
		"OPENAI_API_BASE="+server.URL,
	)
	out, err := cmd.CombinedOutput()
	output := string(out)

	require.NoError(t, err, "output: %s", output)
	assert.Contains(t, output, "Branch Review")
	assert.Contains(t, output, "Walkthrough")
	assert.Contains(t, output, "Statistics")
}

func TestE2E_BranchHelp(t *testing.T) {
	stdout, _, exitCode := runPrev(t, "branch", "--help")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "--context")
	assert.Contains(t, stdout, "--max-tokens")
	assert.Contains(t, stdout, "--per-commit")
	assert.Contains(t, stdout, "--legacy")
	assert.Contains(t, stdout, "--serena")
}
