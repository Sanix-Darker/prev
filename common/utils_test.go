package common_test

import (
	"testing"

	"fmt"
	"os"

	common "github.com/sanix-darker/prev/common"

	"github.com/spf13/pflag"
)

func TestCheckArgs(t *testing.T) {
	tests := []struct {
		keycommand string
		args       []string
		expectHelp bool
	}{
		{
			keycommand: "command",
			args:       []string{"arg1", "arg2"},
			expectHelp: false,
		},
		{
			keycommand: "command",
			args:       []string{},
			expectHelp: true,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("Args: %v", test.args), func(t *testing.T) {
			helpCalled := false
			// helpFunc := common.HelpCallback(func() error {
			// 	helpCalled = true
			// 	return nil
			// })

			// common.CheckArgs(test.keycommand, test.args, helpFunc)

			if helpCalled != test.expectHelp {
				t.Errorf("Expected help function to be called: %v, but it was not.", test.expectHelp)
			}
		})
	}
}

func TestGetArgByKey(t *testing.T) {
	tests := []struct {
		key        string
		cmdFlags   *pflag.FlagSet
		strictMode bool
		expected   string
		expectExit bool
	}{
		{
			key:        "arg1",
			cmdFlags:   pflag.NewFlagSet("test", pflag.ContinueOnError),
			strictMode: false,
			expected:   "value1",
			expectExit: false,
		},
		{
			key:        "arg2",
			cmdFlags:   pflag.NewFlagSet("test", pflag.ContinueOnError),
			strictMode: true,
			expected:   "value2",
			expectExit: false,
		},
		{
			key:        "arg3",
			cmdFlags:   pflag.NewFlagSet("test", pflag.ContinueOnError),
			strictMode: true,
			expected:   "",
			expectExit: true,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("Key: %s", test.key), func(t *testing.T) {
			test.cmdFlags.String(test.key, test.expected, "")
			os.Args = []string{"test"}

			value := common.GetArgByKey(test.key, test.cmdFlags, test.strictMode)

			if value != test.expected {
				t.Errorf("Expected value: %s, but got: %s", test.expected, value)
			}

			// Check if the program exits as expected
			if test.expectExit {
				if err := recover(); err == nil {
					t.Errorf("Expected program to exit, but it did not.")
				}
			}
		})
	}
}

func TestExtractPaths(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "*.go,dir1/*.txt,dir2,notexists/*.csv",
			expected: []string{"main.go", "dir1/file.txt", "dir2"},
		},
		{
			input:    "file1.txt",
			expected: []string{"file1.txt"},
		},
		{
			input:    "nonexistentfile.txt",
			expected: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			// result := common.ExtractPaths(test.input, helper)

			// if !reflect.DeepEqual(result, test.expected) {
			// 	t.Errorf("Expected %v, but got %v", test.expected, result)
			// }
		})
	}
}
