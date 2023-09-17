/*
Copyright © 2023 sanix-darker <s4nixd@gmail.com>
*/
package models

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type FlagStruct struct {
	Label        string
	Short        string
	Description  string
	DefaultValue string
}

func (f FlagStruct) PersistentFlags(cmd cobra.Command) *pflag.FlagSet {
	return cmd.PersistentFlags()
}
