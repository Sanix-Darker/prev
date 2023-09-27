package core

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/sanix-darker/prev/internal/common"
	"github.com/sanix-darker/prev/internal/config"
	"github.com/sergi/go-diff/diffmatchpatch"
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

	prompt := fmt.Sprintf(`
Given + and - in this code, First, try to understand what's it about,
then choose what is the best approach, then review what have beend added :

%s

Respect those rules :
- Respond in a Markdown format styling.
%s
- Give a better approash to prevent regressions, add optimisations.
- Keep it Simple, compact and clear.
- Try to respect DRY, SOLID principles while reviewing.
- Provide the best optimized suggestion at the end.
	`, changes, explainIt)

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

func cleanDiffLine(formating string, line string) string {
	return strings.ReplaceAll(fmt.Sprintf(formating, line), "\n", "")
}

func BuildDiff(filePath1, filePath2 string) (string, error) {

	// Read the contents of the first file
	content1, err := ioutil.ReadFile(filePath1)
	if err != nil {
		return "", err
	}

	// Read the contents of the second file
	content2, err := ioutil.ReadFile(filePath2)
	if err != nil {
		return "", err
	}

	// Compare the contents using diff-match-patch
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(content1), string(content2), true)

	// Generate the diff output
	var changes []string
	for i, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			changes = append(changes, cleanDiffLine("+ %s", diff.Text))
		case diffmatchpatch.DiffDelete:
			changes = append(changes, cleanDiffLine("- %s", diff.Text))
		default:
			if len(diff.Text) > 0 && i < 2 {
				changes = append(changes, cleanDiffLine("%s", diff.Text))
			}
		}
	}

	return strings.Join(changes, "\n"), nil
}
