package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/risor-io/risor"
	"github.com/risor-io/risor/cmd/risor/repl"
	"github.com/risor-io/risor/errz"
)

var (
	cfgFile string
	red     = color.New(color.FgRed).SprintfFunc()
)

func init() {
	cobra.OnInitialize(initViperConfig)
	viper.SetEnvPrefix("risor")

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file (default is $HOME/.risor.yaml)")
	rootCmd.PersistentFlags().StringP("code", "c", "", "Code to evaluate")
	rootCmd.PersistentFlags().Bool("stdin", false, "Read code from stdin")
	rootCmd.PersistentFlags().String("cpu-profile", "", "Capture a CPU profile")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().Bool("no-default-globals", false, "Disable the default globals")
	rootCmd.PersistentFlags().String("modules", ".", "Path to library modules")
	rootCmd.PersistentFlags().BoolP("help", "h", false, "Help for Risor")

	viper.BindPFlag("code", rootCmd.PersistentFlags().Lookup("code"))
	viper.BindPFlag("stdin", rootCmd.PersistentFlags().Lookup("stdin"))
	viper.BindPFlag("cpu-profile", rootCmd.PersistentFlags().Lookup("cpu-profile"))
	viper.BindPFlag("no-color", rootCmd.PersistentFlags().Lookup("no-color"))
	viper.BindPFlag("no-default-globals", rootCmd.PersistentFlags().Lookup("no-default-globals"))
	viper.BindPFlag("modules", rootCmd.PersistentFlags().Lookup("modules"))
	viper.BindPFlag("help", rootCmd.PersistentFlags().Lookup("help"))

	// Root command flags
	rootCmd.Flags().Bool("timing", false, "Show timing information")
	rootCmd.Flags().StringP("output", "o", "", "Set the output format")
	rootCmd.Flags().Bool("no-repl", false, "Disable the REPL")
	rootCmd.RegisterFlagCompletionFunc("output",
		cobra.FixedCompletions(
			outputFormatsCompletion,
			cobra.ShellCompDirectiveNoFileComp,
		))
	rootCmd.Flags().SetInterspersed(false)

	viper.BindPFlag("timing", rootCmd.Flags().Lookup("timing"))
	viper.BindPFlag("output", rootCmd.Flags().Lookup("output"))
	viper.BindPFlag("no-repl", rootCmd.Flags().Lookup("no-repl"))

	viper.AutomaticEnv()
}

func initViperConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			fatal(err)
		}
		viper.AddConfigPath(home)
		viper.SetConfigName(".risor")
	}
	viper.ReadInConfig()
}

var rootCmd = &cobra.Command{
	Use:   "risor",
	Short: "Fast and flexible scripting for Go developers and DevOps",
	Long: `risor

  Risor is an embeddable scripting language for the Go ecosystem

  Learn more at https://risor.io`,
	Args: cobra.ArbitraryArgs,

	// Manually adds file completions, so they get mixed with the sub-commands
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		prefix := ""
		path := toComplete
		if path == "" {
			path = "."
		}
		dir, err := os.ReadDir(path)
		if err != nil {
			path = filepath.Dir(toComplete)
			prefix = filepath.Base(toComplete)
			dir, err = os.ReadDir(path)
		}
		if err != nil {
			return nil, cobra.ShellCompDirectiveDefault
		}
		files := make([]string, 0, len(dir))
		for _, entry := range dir {
			name := entry.Name()
			if !strings.HasPrefix(prefix, ".") && strings.HasPrefix(name, ".") {
				// ignore hidden files
				continue
			}
			if prefix != "" && !strings.HasPrefix(name, prefix) {
				continue
			}
			if entry.IsDir() {
				// hacky way to add a trailing / on Linux, or trailing \ on Windows
				name = strings.TrimSuffix(filepath.Join(name, "x"), "x")
			} else if !strings.HasSuffix(name, ".risor") {
				continue
			}
			files = append(files, filepath.Join(path, name))
		}
		return files, cobra.ShellCompDirectiveNoSpace
	},

	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		processGlobalFlags()

		if path := viper.GetString("cpu-profile"); path != "" {
			f, err := os.Create(path)
			if err != nil {
				fatal(err)
			}
			pprof.StartCPUProfile(f)
			defer pprof.StopCPUProfile()
			handleSigForProfiler()
		}

		// Separate arguments belonging to the Risor CLI from those that are
		// to be passed to the script.
		var scriptArgs []string
		args, scriptArgs = getScriptArgs(args)
		_ = scriptArgs // Script args are no longer used without virtual OS

		opts := getRisorOptions()

		// Run the REPL if no code was provided
		if shouldRunRepl(cmd, args) {
			if err := repl.Run(ctx, opts); err != nil {
				fatal(err)
			}
			return
		}

		// Read the provided code (from flags, stdin, or a file)
		code, err := getRisorCode(cmd, args)
		if err != nil {
			fatal(err)
		}

		// Execute the code
		start := time.Now()
		evalOpts := getRisorOptions()
		if len(args) > 0 {
			evalOpts = append(evalOpts, risor.WithFilename(args[0]))
		}
		result, err := risor.Eval(ctx, code, evalOpts...)
		if err != nil {
			errMsg := err.Error()
			if friendlyErr, ok := err.(errz.FriendlyError); ok {
				errMsg = friendlyErr.FriendlyErrorMessage()
			}
			fatal(errMsg)
		}
		dt := time.Since(start)

		// Print the result
		output, err := getOutput(result, viper.GetString("output"))
		if err != nil {
			fatal(err)
		} else if output != "" {
			fmt.Println(output)
		}

		// Optionally print the execution time
		if viper.GetBool("timing") {
			fmt.Printf("%v\n", dt)
		}
	},
}
