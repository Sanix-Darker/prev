package printers

import (
	"fmt"
	"strings"

	"github.com/manifoldco/promptui"
)

type IPrinters interface {
	Confirm(message string) bool
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
func Confirm(message string) bool {
	return (&Printers{}).Confirm(message)
}
