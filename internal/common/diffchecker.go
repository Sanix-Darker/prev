package common

import (
	"bufio"
	"fmt"
	"os"
)

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

func compareLines(lines1, lines2 []string) []string {
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

	return differences
}

func generateDiffLine(line1, line2 string) string {
	return fmt.Sprintf("+ %s\n- %s", line2, line1)
}
