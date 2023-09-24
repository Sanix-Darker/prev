package core

import (
	"bufio"
	"fmt"
	"os"

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
			"Seems a bad input from your clipbaord ?\nWill not make a call to the API.",
			true,
			false,
			nil,
		)
	}

	prompt := fmt.Sprintf(`
You're a given this snippet of code, give an optimal rewrite of it,
keep it simple, readable and still working.
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
This is a list of diffs with + for adds and - for deletes,
review the list of changes please :

%s

Please respect those rules :
- Respond in a Markdown format styling.
%s
- don't duplicate yourself.
- Priotize simplicity over complexity.
- Try to respect DRY, SOLID principles while reviewing.
- Provide the optimized, clean and simple code you suggest at the end.
	`, changes, explainIt)

	// this function just build the output string that will be passed to
	// the selected API for the initial question to be asked.
	return prompt
}

func readFileLines(filename string) ([]string, error) {
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

func BuildDiff(file1, file2 string) ([]string, error) {

	lines_for_file1, err := readFileLines(file1)
	if err != nil {
		fmt.Println("Error reading", file1, ":", err)
		return nil, err
	}

	lines_for_file2, err := readFileLines(file2)
	if err != nil {
		fmt.Println("Error reading", file2, ":", err)
		return nil, err
	}

	var differences []string
	var similarLineCount int

	i, j := 0, 0
	for i < len(lines_for_file1) && j < len(lines_for_file2) {
		// this comparison looks better but l
		// if strings.EqualFold(
		// 	strings.TrimSpace(lines1[i]),
		// 	strings.TrimSpace(lines2[j]),
		// ) {
		if lines_for_file1[i] == lines_for_file2[j] {
			similarLineCount++
			if i == 0 || i == len(lines_for_file1) {
				// I want to keep lines similars if it's at the top printed
				// or whenit's at the end
				differences = append(differences, lines_for_file1[i])
			} else if similarLineCount >= 2 {
				differences = append(differences, "---")
			}
			i++
			j++
		} else {
			similarLineCount = 0
			differences = append(differences, generateDiffLine(lines_for_file1[i], lines_for_file2[j]))
			i++
			j++
		}
	}

	// Handle remaining lines in case one file has more lines than the other.
	for i < len(lines_for_file1) {
		differences = append(differences, generateDiffLine(lines_for_file1[i], ""))
		i++
	}

	for j < len(lines_for_file2) {
		differences = append(differences, generateDiffLine("", lines_for_file2[j]))
		j++
	}

	return differences, nil
}

func generateDiffLine(line1, line2 string) string {
	return fmt.Sprintf("+ %s\n- %s", line2, line1)
}
