package core

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// BuildPrompt build the prompt to ask the AI
func BuildPrompt(changes string, maxCharPerPoints int, maxKeyPoints int) string {

	changesBuilt := []string{""}

	return fmt.Sprintf(`You're on a code review, review this list of changes :

%s

Please respect those rules :
- respond only with keypoint, no more than %d characters per points.
- if new changes are optimal, don't comment.
- if regressions detected, give comment or/and code about better approach.
- don't explain or rexplain any  source code
- print the code only if you have a better solution.
- no more than %d keypoints.
- priotize simplicity over complexity.
- try to respect DRY, SOLID principles while reviewing.
`, strings.Join(changesBuilt, "-----------------------"), maxCharPerPoints, maxKeyPoints)
}

// BuildDiff builds +/- changes between two files and returns an array of string differences.
func BuildDiff(filepath1 string, filepath2 string) ([]string, error) {
	file1Lines, err := readLines(filepath1)
	if err != nil {
		return nil, err
	}

	file2Lines, err := readLines(filepath2)
	if err != nil {
		return nil, err
	}

	diffLines := make([]string, 0)

	// Compare lines
	i := 0
	j := 0
	for i < len(file1Lines) && j < len(file2Lines) {
		if file1Lines[i] == file2Lines[j] {
			diffLines = append(diffLines, " "+file1Lines[i])
			i++
			j++
		} else {
			diffLines = append(diffLines, "-"+file1Lines[i])
			i++
		}
	}

	// Add remaining lines from file1
	for i < len(file1Lines) {
		diffLines = append(diffLines, "-"+file1Lines[i])
		i++
	}

	// Add remaining lines from file2
	for j < len(file2Lines) {
		diffLines = append(diffLines, "+"+file2Lines[j])
		j++
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
