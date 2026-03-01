package handlers

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterCommitDiffByPath_Matches(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,3 @@
-old main
+new main
diff --git a/utils.go b/utils.go
--- a/utils.go
+++ b/utils.go
@@ -1,3 +1,3 @@
-old utils
+new utils`

	filtered := filterCommitDiffByPath(diff, "main.go")
	assert.Contains(t, filtered, "main.go")
	assert.NotContains(t, filtered, "utils.go")
}

func TestExtractCommitHandler_PathFilter(t *testing.T) {
	repoPath := setupHandlerRepo(t)

	out, err := exec.Command("git", "-C", repoPath, "rev-parse", "feature").CombinedOutput()
	require.NoError(t, err, string(out))
	commitHash := strings.TrimSpace(string(out))

	result, err := ExtractCommitHandler(testConf(), commitHash, repoPath, "main.go", nil)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Contains(t, result[0], "main.go")
	assert.NotContains(t, result[0], "utils.go")
}
