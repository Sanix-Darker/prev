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
	return fmt.Sprintf(`
You're on a code review,
this is a diff representations with + for code added and - for code deleted,
review this list of changes please :

%s

Please respect those rules :
- Respond in a Markdown format styling.
- Respond only with important keypoints, no more than %d characters each per points.
- If less optimal give comment and code for better approach.
- No more than %d keypoints.
- Don't mention the keypoint title or enumeration, just the content matter.
- Priotize simplicity over complexity.
- Try to respect DRY, SOLID principles while reviewing.
- Provide only keypoints for code change that should be updated.
- Provide the optimized, clean and simple code you suggest me at the end, with a small title "suggestion:" for each set of changes blocks.
`, changes, maxCharPerPoints, maxKeyPoints)
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

	lines1, err := readFileLines(file1)
	if err != nil {
		fmt.Println("Error reading", file1, ":", err)
		return nil, err
	}

	lines2, err := readFileLines(file2)
	if err != nil {
		fmt.Println("Error reading", file2, ":", err)
		return nil, err
	}

	var differences []string
	var similarLineCount int

	i, j := 0, 0
	for i < len(lines1) && j < len(lines2) {
		if lines1[i] == lines2[j] {
			similarLineCount++
			if similarLineCount == 3 {
				differences = append(differences, "---")
			}
			i++
			j++
		} else {
			similarLineCount = 0
			differences = append(differences, generateDiffLine(lines1[i], lines2[j]))
			i++
			j++
		}
	}

	// Handle remaining lines in case one file has more lines than the other.
	for i < len(lines1) {
		differences = append(differences, generateDiffLine(lines1[i], ""))
		i++
	}

	for j < len(lines2) {
		differences = append(differences, generateDiffLine("", lines2[j]))
		j++
	}

	return differences, nil
}

func generateDiffLine(line1, line2 string) string {
	return fmt.Sprintf("+ %s\n- %s", line2, line1)
}
