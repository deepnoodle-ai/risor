package main

import (
	"os"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestGetScriptArguments(t *testing.T) {
	test := []struct {
		args      []string
		expOS     []string
		expScript []string
	}{
		{
			[]string{"--foo", "--bar", "1"},
			[]string{"--foo", "--bar", "1"},
			[]string{},
		},
		{
			[]string{"--foo", "1", "--", "/path/to/script"},
			[]string{"/path/to/script"},
			[]string{"/path/to/script"},
		},
		{
			[]string{"--foo", "1", "--"},
			[]string{},
			[]string{},
		},
		{
			[]string{"--", "/path/to/script", "2", "-h", "--bar"},
			[]string{"/path/to/script"},
			[]string{"/path/to/script", "2", "-h", "--bar"},
		},
		{
			[]string{"--"},
			[]string{},
			[]string{},
		},
		{
			[]string{},
			[]string{},
			[]string{},
		},
		{
			[]string{"--no-repl", "1", "--", "/path/to/script", "1", "-f"},
			[]string{"/path/to/script"},
			[]string{"/path/to/script", "1", "-f"},
		},
	}

	for _, tt := range test {
		t.Run(strings.Join(tt.args, "-"), func(t *testing.T) {
			origArgs := os.Args
			testArgs := []string{"risor"}
			testArgs = append(testArgs, tt.args...)
			os.Args = testArgs
			defer func() { os.Args = origArgs }()
			osArgs, scriptArgs := getScriptArgs(tt.args)
			assert.Equal(t, osArgs, tt.expOS)
			assert.Equal(t, scriptArgs, tt.expScript)
		})
	}
}
