package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/fatih/color"
	"github.com/hokaccha/go-prettyjson"
	"github.com/mattn/go-isatty"
	"github.com/mitchellh/go-homedir"
	"github.com/risor-io/risor"
	"github.com/risor-io/risor/cmd/risor/repl"
	"github.com/risor-io/risor/errz"
	"github.com/risor-io/risor/modules/aws"
	"github.com/risor-io/risor/modules/cli"
	"github.com/risor-io/risor/modules/gha"
	"github.com/risor-io/risor/modules/image"
	"github.com/risor-io/risor/modules/jmespath"
	k8s "github.com/risor-io/risor/modules/kubernetes"
	"github.com/risor-io/risor/modules/pgx"
	"github.com/risor-io/risor/modules/sql"
	"github.com/risor-io/risor/modules/template"
	"github.com/risor-io/risor/modules/uuid"
	"github.com/risor-io/risor/modules/vault"
	"github.com/risor-io/risor/object"
	ros "github.com/risor-io/risor/os"
	"github.com/risor-io/risor/os/s3fs"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	red     = color.New(color.FgRed).SprintfFunc()
)

func init() {
	cobra.OnInitialize(initConfig)
	viper.SetEnvPrefix("risor")

	// Global flags

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file (default is $HOME/.risor.yaml)")
	rootCmd.PersistentFlags().StringP("code", "c", "", "Code to evaluate")
	rootCmd.PersistentFlags().Bool("stdin", false, "Read code from stdin")
	rootCmd.PersistentFlags().String("cpu-profile", "", "Capture a CPU profile")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().Bool("virtual-os", false, "Enable a virtual operating system")
	rootCmd.PersistentFlags().StringArrayP("mount", "m", []string{}, "Mount a filesystem")
	rootCmd.PersistentFlags().Bool("no-default-globals", false, "Disable the default globals")
	rootCmd.PersistentFlags().String("modules", ".", "Path to library modules")
	rootCmd.PersistentFlags().BoolP("help", "h", false, "Help for Risor")

	viper.BindPFlag("code", rootCmd.PersistentFlags().Lookup("code"))
	viper.BindPFlag("stdin", rootCmd.PersistentFlags().Lookup("stdin"))
	viper.BindPFlag("cpu-profile", rootCmd.PersistentFlags().Lookup("cpu-profile"))
	viper.BindPFlag("no-color", rootCmd.PersistentFlags().Lookup("no-color"))
	viper.BindPFlag("virtual-os", rootCmd.PersistentFlags().Lookup("virtual-os"))
	viper.BindPFlag("mount", rootCmd.PersistentFlags().Lookup("mount"))
	viper.BindPFlag("no-default-globals", rootCmd.PersistentFlags().Lookup("no-default-globals"))
	viper.BindPFlag("modules", rootCmd.PersistentFlags().Lookup("modules"))
	viper.BindPFlag("help", rootCmd.PersistentFlags().Lookup("help"))

	// Root command flags

	rootCmd.Flags().Bool("timing", false, "Show timing information")
	rootCmd.Flags().StringP("output", "o", "", "Set the output format")
	rootCmd.RegisterFlagCompletionFunc("output",
		cobra.FixedCompletions(outputFormatsCompletion, cobra.ShellCompDirectiveNoFileComp))
	rootCmd.Flags().SetInterspersed(false)
	viper.BindPFlag("timing", rootCmd.Flags().Lookup("timing"))
	viper.BindPFlag("output", rootCmd.Flags().Lookup("output"))

	viper.AutomaticEnv()
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		// Search config in home directory with name ".risor"
		viper.AddConfigPath(home)
		viper.SetConfigName(".risor")
	}
	viper.ReadInConfig()
}

func fatal(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func isTerminalIO() bool {
	stdin := os.Stdin.Fd()
	stdout := os.Stdout.Fd()
	inTerm := isatty.IsTerminal(stdin) || isatty.IsCygwinTerminal(stdin)
	outTerm := isatty.IsTerminal(stdout) || isatty.IsCygwinTerminal(stdout)
	return inTerm && outTerm
}

// this works around issues with cobra not passing args after -- to the script
// see: https://github.com/spf13/cobra/issues/1877
// passedargs is everything after the '--'
// If dropped is true, cobra dropped the double dash in its 'args' slice.
// argsout returns the cobra args but without the '--' and the items behind it
// this also supports multiple '--' in the args	list
func getpassthruargs(args []string) (argsout []string, passedargs []string, dropped bool) {
	//		lenArgs := len(args)
	argsout = args
	ddashcnt := 0
	for n, arg := range os.Args {
		if arg == "--" {
			ddashcnt++
			if len(passedargs) == 0 {
				if len(os.Args) > n {
					passedargs = os.Args[n+1:]
				}
			}
		}
	}
	// drop arg0 from count
	noddash := true
	for n2, argz := range args {
		// don't go past the first '--' - this allows one to pass '--' to risor also
		if argz == "--" {
			noddash = false
			argsout = args[:n2]
			break
		}
	}
	// cobra seems to drop the '--' in its args if everything before it was a flag
	// if thats the case then args is just empty b/c evrything else is the '--' and the passed args
	if (noddash || ddashcnt > 1) && len(passedargs) > 0 {
		dropped = true
		argsout = []string{}
	}
	return
}

var rootCmd = &cobra.Command{
	Use:   "risor",
	Short: "Fast and flexible scripting for Go developers and DevOps",
	Long:  `https://risor.io`,
	Args:  cobra.ArbitraryArgs,

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
			}
			files = append(files, filepath.Join(path, name))
		}
		return files, cobra.ShellCompDirectiveNoSpace
	},

	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		var passedargs []string

		args, passedargs, _ = getpassthruargs(args)
		// pass the 'passthru' args to risor's os package
		ros.SetScriptArgs(passedargs)
		// Optionally enable a virtual operating system and add it to
		// the context so that it's made available to Risor VM.
		if viper.GetBool("virtual-os") {
			mounts := map[string]*ros.Mount{}
			m := viper.GetStringSlice("mount")
			for _, v := range m {
				fs, dst, err := mountFromSpec(ctx, v)
				if err != nil {
					fatal(err.Error())
				}
				mounts[dst] = &ros.Mount{
					Source: fs,
					Target: dst,
				}
			}
			vos := ros.NewVirtualOS(ctx, ros.WithMounts(mounts), ros.WithArgs(passedargs))
			ctx = ros.WithOS(ctx, vos)
		}

		// Disable colored output if no-color is specified
		if viper.GetBool("no-color") {
			color.NoColor = true
		}

		// Optionally capture a CPU profile to the given path
		if path := viper.GetString("cpu-profile"); path != "" {
			f, err := os.Create(path)
			if err != nil {
				fatal(red(err.Error()))
			}
			pprof.StartCPUProfile(f)
			defer pprof.StopCPUProfile()
		}

		// Build up a list of options to pass to the VM
		opts := []risor.Option{
			risor.WithConcurrency(),
			risor.WithListenersAllowed(),
		}
		if viper.GetBool("no-default-globals") {
			opts = append(opts, risor.WithoutDefaultGlobals())
		} else {
			globals := map[string]any{
				"cli":      cli.Module(),
				"gha":      gha.Module(),
				"image":    image.Module(),
				"pgx":      pgx.Module(),
				"sql":      sql.Module(),
				"template": template.Module(),
				"uuid":     uuid.Module(),
			}

			for k, v := range jmespath.Builtins() {
				globals[k] = v
			}
			for k, v := range template.Builtins() {
				globals[k] = v
			}
			opts = append(opts, risor.WithGlobals(globals))

			// AWS support may or may not be compiled in based on build tags
			if aws := aws.Module(); aws != nil {
				opts = append(opts, risor.WithGlobal("aws", aws))
			}
			// K8S support may or may not be compiled in based on build tags
			if k8s := k8s.Module(); k8s != nil {
				opts = append(opts, risor.WithGlobal("k8s", k8s))
			}
			// Vault support may or may not be compiled in based on build tags
			if vault := vault.Module(); vault != nil {
				opts = append(opts, risor.WithGlobal("vault", vault))
			}
		}
		if modulesDir := viper.GetString("modules"); modulesDir != "" {
			opts = append(opts, risor.WithLocalImporter(modulesDir))
		}

		// Determine what code is to be executed. The code may be supplied
		// via the --code option, a path supplied as an arg, or stdin.
		codeWasSupplied := cmd.Flags().Lookup("code").Changed
		code := viper.GetString("code")
		if len(args) > 0 && codeWasSupplied {
			fatal(red("cannot specify both code and a filepath"))
		}
		if len(args) == 0 && !codeWasSupplied && !viper.GetBool("stdin") && len(passedargs) == 0 {
			if !isTerminalIO() {
				fatal("cannot show repl: stdin or stdout is not a terminal")
			}
			if err := repl.Run(ctx, opts); err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", red(err.Error()))
				os.Exit(1)
			}
			return
		}
		if viper.GetBool("stdin") {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				fatal(red(err.Error()))
			}
			if len(data) == 0 {
				fatal(red("no code supplied"))
			}
			code = string(data)
		} else if len(args) > 0 {
			bytes, err := os.ReadFile(args[0])
			if err != nil {
				fatal(red(err.Error()))
			}
			code = string(bytes)
		} else if len(passedargs) > 0 {
			bytes, err := os.ReadFile(passedargs[0])
			if err != nil {
				fatal(red(err.Error()))
			}
			code = string(bytes)
		}

		start := time.Now()

		// Execute the code
		result, err := risor.Eval(ctx, code, opts...)
		if err != nil {
			if friendlyErr, ok := err.(errz.FriendlyError); ok {
				fmt.Fprintf(os.Stderr, "%s\n", red(friendlyErr.FriendlyErrorMessage()))
			} else {
				fmt.Fprintf(os.Stderr, "%s\n", red(err.Error()))
			}
			os.Exit(1)
		}

		dt := time.Since(start)

		// Print the result
		output, err := getOutput(result, viper.GetString("output"))
		if err != nil {
			fatal(red(err.Error()))
		} else if output != "" {
			fmt.Println(output)
		}

		// Optionally print the execution time
		if viper.GetBool("timing") {
			fmt.Printf("%v\n", dt)
		}
	},
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

func mountFromSpec(ctx context.Context, spec string) (ros.FS, string, error) {
	parts := strings.Split(spec, ",")
	items := map[string]string{}
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return nil, "", fmt.Errorf("invalid mount spec: %s (expected k=v format)", spec)
		}
		items[kv[0]] = kv[1]
	}
	typ, ok := items["type"]
	if !ok || typ == "" {
		return nil, "", fmt.Errorf("invalid mount spec: %q (missing type)", spec)
	}
	src, ok := items["src"]
	if !ok || src == "" {
		return nil, "", fmt.Errorf("invalid mount spec: %q (missing src)", spec)
	}
	dst, ok := items["dst"]
	if !ok || dst == "" {
		return nil, "", fmt.Errorf("invalid mount spec: %q (missing dst)", spec)
	}
	switch typ {
	case "s3":
		var awsOpts []func(*config.LoadOptions) error
		if r, ok := items["region"]; ok {
			awsOpts = append(awsOpts, config.WithRegion(r))
		}
		if p, ok := items["profile"]; ok {
			awsOpts = append(awsOpts, config.WithSharedConfigProfile(p))
		}
		cfg, err := config.LoadDefaultConfig(ctx, awsOpts...)
		if err != nil {
			return nil, "", err
		}
		s3Opts := []s3fs.Option{
			s3fs.WithBucket(src),
			s3fs.WithClient(s3.NewFromConfig(cfg)),
		}
		if p, ok := items["prefix"]; ok && p != "" {
			s3Opts = append(s3Opts, s3fs.WithBase(p))
		}
		fs, err := s3fs.New(ctx, s3Opts...)
		if err != nil {
			return nil, "", err
		}
		return fs, dst, nil
	default:
		return nil, "", fmt.Errorf("unsupported source: %s", src)
	}
}
