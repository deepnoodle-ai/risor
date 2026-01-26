package main

import (
	"errors"
	"io"
	"os"

	"github.com/risor-io/risor"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Returns a Risor option for environment configuration.
// By default, the CLI provides the standard builtins.
// Use --no-default-globals to start with an empty environment.
func getEnv() risor.Option {
	if viper.GetBool("no-default-globals") {
		return nil // empty environment
	}
	return risor.WithEnv(risor.Builtins())
}

func getRisorOptions() []risor.Option {
	return []risor.Option{
		getEnv(),
	}
}

func shouldRunRepl(cmd *cobra.Command, args []string) bool {
	if viper.GetBool("no-repl") || viper.GetBool("stdin") {
		return false
	}
	if cmd.Flags().Lookup("code").Changed {
		return false
	}
	if len(args) > 0 {
		return false
	}
	return isTerminalIO()
}

func getRisorCode(cmd *cobra.Command, args []string) (string, error) {
	// Determine what code is to be executed. There three possibilities:
	// 1. --code <code>
	// 2. --stdin (read code from stdin)
	// 3. path as args[0]
	var codeFlagSet bool
	if f := cmd.Flags().Lookup("code"); f != nil && f.Changed {
		codeFlagSet = true
	}
	var stdinFlagSet bool
	if f := cmd.Flags().Lookup("stdin"); f != nil && f.Changed {
		stdinFlagSet = true
	}
	pathSupplied := len(args) > 0
	// Error if multiple input sources are specified
	if pathSupplied && (codeFlagSet || stdinFlagSet) {
		return "", errors.New("multiple input sources specified")
	} else if codeFlagSet && stdinFlagSet {
		return "", errors.New("multiple input sources specified")
	}
	if stdinFlagSet {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(data), nil
	} else if pathSupplied {
		bytes, err := os.ReadFile(args[0])
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}
	return viper.GetString("code"), nil
}
