package printers

import (
	"bufio"
	"fmt"
	"os"
	"strings"
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
	fmt.Fprintf(os.Stderr, "%s Press (y/n): ", message)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.ToLower(strings.TrimSpace(input))

	return input == "y"
}

// Confirm prompt a confirmation message
func Confirm(message string) bool {
	return (&Printers{}).Confirm(message)
}
