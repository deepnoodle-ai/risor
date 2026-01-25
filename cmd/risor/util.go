package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/hokaccha/go-prettyjson"
	"github.com/mattn/go-isatty"
	"github.com/risor-io/risor/object"
	"github.com/spf13/viper"
)

func fatal(msg interface{}) {
	var s string
	switch msg := msg.(type) {
	case string:
		s = msg
	case error:
		s = msg.Error()
	default:
		s = fmt.Sprintf("%v", msg)
	}
	fmt.Fprintf(os.Stderr, "%s\n", red(s))
	os.Exit(1)
}

func isTerminalIO() bool {
	stdin := os.Stdin.Fd()
	stdout := os.Stdout.Fd()
	inTerm := isatty.IsTerminal(stdin) || isatty.IsCygwinTerminal(stdin)
	outTerm := isatty.IsTerminal(stdout) || isatty.IsCygwinTerminal(stdout)
	return inTerm && outTerm
}

func indexOf(arr []string, val string) int {
	for i, v := range arr {
		if v == val {
			return i
		}
	}
	return -1
}

// Separate arguments pertaining to the Risor CLI from arguments meant for the
// Risor script. Related Cobra issue: https://github.com/spf13/cobra/issues/1877
func getScriptArgs(positionalArgs []string) ([]string, []string) {
	dashIndex := indexOf(os.Args, "--")
	if dashIndex < 1 {
		return positionalArgs, []string{}
	}
	risorArgs := []string{}
	scriptArgs := []string{}
	if len(os.Args) > dashIndex+1 {
		risorArgs = append(risorArgs, os.Args[dashIndex+1])
		scriptArgs = os.Args[dashIndex+1:]
	}
	return risorArgs, scriptArgs
}

var outputFormatsCompletion = []string{"json", "text"}

func getOutput(result object.Object, format string) (string, error) {
	switch strings.ToLower(format) {
	case "":
		// With an unspecified format, we'll try to do the most helpful thing:
		//  1. If the result is nil, we want to print nothing
		//  2. If the result marshals to JSON, we'll print that
		//  3. Otherwise, we'll print the result's string representation
		if result == object.Nil {
			return "", nil
		}
		output, err := getOutputJSON(result)
		if err != nil {
			return fmt.Sprintf("%v", result), nil
		}
		return string(output), nil
	case "json":
		output, err := getOutputJSON(result)
		if err != nil {
			return "", err
		}
		return string(output), nil
	case "text":
		return fmt.Sprintf("%v", result), nil
	default:
		return "", fmt.Errorf("unknown output format: %s", format)
	}
}

func getOutputJSON(result object.Object) ([]byte, error) {
	if viper.GetBool("no-color") {
		return json.MarshalIndent(result, "", "  ")
	} else {
		return prettyjson.Marshal(result)
	}
}

func handleSigForProfiler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		pprof.StopCPUProfile()
		os.Exit(1)
	}()
}

// Reads global flags from Viper and adjusts the environment accordingly.
func processGlobalFlags() {
	if viper.GetBool("no-color") {
		color.NoColor = true
	}
}
