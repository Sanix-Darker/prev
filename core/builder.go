package core

import (
	"io/ioutil"
	"strings"
)

// BuildPrompt build the prompt to ask the AI
func BuildPrompt(changes string) string {
	return ""
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
	content, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(content), ""), nil
}
