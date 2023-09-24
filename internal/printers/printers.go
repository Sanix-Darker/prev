package printers

import (
	"fmt"
	"strings"

	"github.com/manifoldco/promptui"
)

const selectItemsSize = 10

var defaultPrinters = Printers{}

type IPrinters interface {
	Confirm(message string) bool
	SelectScriptEntry(entryType string) (int, string, error)
}

type Printers struct{}

// NewPrinters returns new printers struct
func NewPrinters() *Printers {
	return &Printers{}
}

func (p Printers) Confirm(message string) bool {
	validate := func(input string) error {
		input = strings.ToLower(strings.TrimSpace(input))
		if input != "y" && input != "n" {
			return fmt.Errorf("wrong input %s, was expecting `y` or `n`", input)
		}

		return nil
	}

	msg := message + " Press (y/n)"
	prompt := promptui.Prompt{
		Label:    msg,
		Validate: validate,
	}

	result, err := prompt.Run()
	if err != nil {
		return false
	}
	input := strings.ToLower(strings.TrimSpace(result))

	return input == "y"
}

// Confirm prompt a confirmation message
//
// Return true if the user entered Y/y and false if entered n/N
func Confirm(message string) bool {
	return defaultPrinters.Confirm(message)
}

func SelectScriptEntry(entryType string) (int, string, error) {
	return defaultPrinters.SelectScriptEntry(entryType)
}

// SelectScriptEntry prompt a search
// returns the selected entry index
func (p Printers) SelectScriptEntry(entryType string) (int, string, error) {

	itemsTobeSearch := []string{"item1", "item2"}
	searchScript := func(input string, index int) bool {
		s := itemsTobeSearch[index]

		// random stuffs here for now
		if entryType == "something1" {
			return true
		} else if entryType == "something2" {
			return true
		} else if entryType == s {
			return false
		}

		// return DoesScriptContain(s, input)
		return false
	}
	// TODO: use this for prompt interaction, maybe as an util
	// i don't know yet
	prompt := promptui.Select{
		Label:             "Enter your search text",
		Items:             []string{"item1", "item2"},
		Size:              selectItemsSize,
		StartInSearchMode: true,
		Searcher:          searchScript,
		Templates:         getTemplates(),
	}

	i, result, err := prompt.Run()
	if err != nil {
		return i, "", fmt.Errorf("prompt failed %v", err)
	}

	return i, result, nil
}

func getTemplates() *promptui.SelectTemplates {
	trimText := func(s string) string {
		if len(s) > 50 {
			return s[:50] + "..."
		}
		return s
	}

	funcMap := promptui.FuncMap
	funcMap["inline"] = func(s string) string {
		return strings.ReplaceAll(trimText(s), "\n", " ")
	}

	//if you find a hard time understand it check out golang templating format documentation
	//here https://golang.org/pkg/text/template
	return &promptui.SelectTemplates{
		Label: "{{ if .Code.Content -}} {{`code:` | bold | green}} " +
			"{{ inline .Code.Content}} {{- else -}} {{ inline .Solution.Content }} {{ end }}",
		Active: "* {{ if .Code.Content -}} {{`code:` | bold | green}} {{ inline .Code.Content | bold}} {{ else }} " +
			"{{`solution:` | bold | yellow }} {{ inline .Solution.Content | bold }} {{ end }}",
		Inactive: "{{ if .Code.Content -}} {{`code:` | green }} {{ inline .Code.Content }} " +
			"{{- else -}} {{`solution:` | yellow}} {{ inline .Solution.Content }} {{ end }}",
		Selected: " {{ `✓` | green }} {{if .Code.Content -}} {{ inline .Code.Content | bold }} {{- else -}} {{ inline .Solution.Content | bold }} {{ end }}",
		Details: "Type: {{- if .Code.Content }} code {{ else }} solution {{- end }}" +
			"{{ if .Code.Alias }} | Alias: {{ .Code.Alias }} {{- end }}" +
			"{{ if .Comment }} | Comment: {{ .Comment }} {{- end }}",
		FuncMap: funcMap,
	}
}
