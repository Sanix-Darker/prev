package core

import (
	"bufio"
	"fmt"
	"os"
)

// BuildPrompt build the prompt to ask the AI
func BuildPrompt(
	changes string,
	maxCharPerPoints int,
	maxKeyPoints int,
) string {
	// this function just build the output string that will be passed to
	// the selected API for the initial question to be asked.
	return fmt.Sprintf(`You're on a code review, review this list of changes :

%s

Please respect those rules :
- Respond only with keypoints, no more than %d characters per points.
- If new changes are optimal, don't comment or describe it.
- If regressions detected, or less optimal give comment and code for better approach.
- Don't explain or rexplain source code provided.
- Print the code only if you have a better solution.
- No more than %d keypoints.
- Priotize simplicity over complexity.
- Try to respect DRY, SOLID principles while reviewing.
`, changes, maxCharPerPoints, maxKeyPoints)
}

// BuildDiff builds +/- changes between two files and returns an array of
// string differences.
func BuildDiff(filepath1, filepath2 string) ([]string, error) {
	file1Lines, err := readLines(filepath1)
	if err != nil {
		return nil, err
	}

	file2Lines, err := readLines(filepath2)
	if err != nil {
		return nil, err
	}

	var diffLines []string
	i, j := 0, 0

	for i < len(file1Lines) || j < len(file2Lines) {
		switch {
		case i < len(file1Lines) && j < len(file2Lines) && file1Lines[i] == file2Lines[j]:
			diffLines = append(diffLines, " "+file1Lines[i])
			i++
			j++
		case i < len(file1Lines):
			if i == 0 || file1Lines[i] != file1Lines[i-1] {
				diffLines = append(diffLines, "-"+file1Lines[i])
			}
			i++
		case j < len(file2Lines):
			if j == 0 || file2Lines[j] != file2Lines[j-1] {
				diffLines = append(diffLines, "+"+file2Lines[j])
			}
			j++
		}
	}

	return diffLines, nil
}

func readLines(filepath string) ([]string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	lines := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}
