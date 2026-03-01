package core

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/sanix-darker/prev/internal/common"
	"github.com/sanix-darker/prev/internal/config"
)

// BuildReviewPrompt build the prompt to ask the AI
func BuildOptimPrompt(
	conf config.Config,
	code string,
) string {

	if len(code) < 5 {
		common.LogError(
			"Your input data seems too small (<5) ?\nWill not make a call to the API.",
			true,
			false,
			nil,
		)
	}

	prompt := fmt.Sprintf(`
You're a given this snippet of code, give an optimal rewrite of it,
keep it simple and optimized, better.
%s
No need to provide comments, or explanations, just provide the code with a suggestion title.
Respond in a Markdown format styling.
	`, code)

	return prompt

}

// BuildReviewPrompt build the prompt to ask the AI
func BuildReviewPrompt(
	conf config.Config,
	changes string,
	guidelines string,
) string {

	explainIt := "No explanations, just your optimal code suggestion."
	// in this way we can just get either the code or code and comments
	// because some people can understand by just reading the generated code.
	if conf.ExplainItOrNot {
		explainIt = fmt.Sprintf(`
- Respond only with important keypoints, no more than %d characters for each per points.
- If adds are less optimal give comment and code for better approach.
- No more than %d keypoints per set of changes.
- Provide only keypoints for code change that should be updated.
- Add small title "suggestion:" for each set of changes blocks at the end.
		`, conf.MaxCharactersPerKeyPoints, conf.MaxKeyPoints)
	}

	guidelineBlock := ""
	if strings.TrimSpace(guidelines) != "" {
		guidelineBlock = fmt.Sprintf(`
Repository-specific guidelines (follow when applicable):
%s
`, guidelines)
	}

	prompt := fmt.Sprintf(`
Given + and - in this code, First, try to understand what's it about,
then choose what is the best approach, then review what has been added:

%s

%s

Respect those rules :
- Post the code only once, don't repeat yourself.
- Respond in a Markdown format styling.
- + and - are adds and deletions
%s
- Give a better approach to prevent regressions, add optimizations.
- Prioritize source code concerns first.
- For text/documentation files (.md/.txt/.rst/.adoc), focus on typos/spelling/grammar only
  unless there is a critical correctness or security problem.
- For each finding, include impact analysis tied to changed hunks:
  - runtime behavior impact at the modified lines,
  - likely upstream callers and downstream callees affected,
  - cross-file contract/config/schema risks,
  - regression risk and missing/needed tests.
- When Change Intent Context is provided (for example commit message), verify findings against it
  and highlight mismatches between intended and actual behavior.
- Do not over-engineer suggestions; keep fixes short, concise, and surgical.
- Keep it Simple, compact and clear.
- Try to respect DRY, SOLID principles while reviewing.
- Provide the best optimized suggestion at the end.
	`, changes, guidelineBlock, explainIt)

	// this function just build the output string that will be passed to
	// the selected API for the initial question to be asked.
	return prompt
}

func ReadFileLines(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

func BuildDiff(filePath1, filePath2 string) (string, error) {

	// Read the contents of the first file
	content1, err := os.ReadFile(filePath1)
	if err != nil {
		return "", err
	}

	// Read the contents of the second file
	content2, err := os.ReadFile(filePath2)
	if err != nil {
		return "", err
	}

	// Fast path: identical content has no changes to review.
	if bytes.Equal(content1, content2) {
		return "", nil
	}

	oldLines := splitLines(string(content1))
	newLines := splitLines(string(content2))

	// Fast paths to avoid LCS matrix allocation for one-sided files.
	if len(oldLines) == 0 {
		changes := make([]string, 0, len(newLines))
		for _, line := range newLines {
			changes = append(changes, "+ "+line)
		}
		return strings.Join(changes, "\n"), nil
	}
	if len(newLines) == 0 {
		changes := make([]string, 0, len(oldLines))
		for _, line := range oldLines {
			changes = append(changes, "- "+line)
		}
		return strings.Join(changes, "\n"), nil
	}

	ops := computeLineDiff(oldLines, newLines)

	// Keep only the first two context lines before the first change to avoid
	// overwhelming the prompt with unchanged content.
	changes := make([]string, 0, len(ops))
	contextCount := 0
	sawChange := false
	for _, op := range ops {
		switch op.kind {
		case lineOpAdd:
			sawChange = true
			changes = append(changes, "+ "+op.line)
		case lineOpDel:
			sawChange = true
			changes = append(changes, "- "+op.line)
		case lineOpEqual:
			if !sawChange && contextCount < 2 {
				changes = append(changes, op.line)
				contextCount++
			}
		}
	}

	return strings.Join(changes, "\n"), nil
}

type lineOpKind byte

const (
	lineOpEqual lineOpKind = iota
	lineOpAdd
	lineOpDel
)

type lineOp struct {
	kind lineOpKind
	line string
}

func splitLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func computeLineDiff(oldLines, newLines []string) []lineOp {
	m := len(oldLines)
	n := len(newLines)

	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}

	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	ops := make([]lineOp, 0, m+n)
	i, j := 0, 0
	for i < m && j < n {
		switch {
		case oldLines[i] == newLines[j]:
			ops = append(ops, lineOp{kind: lineOpEqual, line: oldLines[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, lineOp{kind: lineOpDel, line: oldLines[i]})
			i++
		default:
			ops = append(ops, lineOp{kind: lineOpAdd, line: newLines[j]})
			j++
		}
	}

	for ; i < m; i++ {
		ops = append(ops, lineOp{kind: lineOpDel, line: oldLines[i]})
	}
	for ; j < n; j++ {
		ops = append(ops, lineOp{kind: lineOpAdd, line: newLines[j]})
	}

	return ops
}
