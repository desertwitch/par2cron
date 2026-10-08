/*
par2cron is a tool that wraps par2cmdline (a parity-based file recovery tool)
to achieve automated periodic integrity creation, verification and repair within
any given directory tree. It is designed for use with non-changing WORM-type of
files, perfect for adding a degree of protection to media libraries or backups.

The driving idea is that you do not need to invest in a filesystem (like ZFS)
that protects all your data, at the disadvantage of additional complexities,
when you really only care that important subsets of your data remain protected.

A given directory tree on any filesystem is scanned for marker files, and a
PAR2 set created for every directory containing such a "_par2cron" file. For
verification, the program loads the PAR2 sets and verifies that the data which
they are protecting is healthy, otherwise flagging the PAR2 set for repair.
Once repair runs, corrupted or missing files are recovered. Many command-line
tunables, as well as configuration directives, are offered for more granular
adjustment of how to create, when to verify and in what situation to repair.

A set-and-forget setup is as easy as adding three commands to crontab:
  - par2cron create
  - par2cron verify
  - par2cron repair

That being set up, you can simply protect any valuable folder by just placing a
"_par2cron" file in it; the tool will create a PAR2 set and pick it up into the
periodic verification and repair cycle - now protected from corruption/bitrot.
*/
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"slices"
	"strings"
	"syscall"

	"github.com/desertwitch/par2cron/docs/configs"
	"github.com/desertwitch/par2cron/docs/specs"
	"github.com/desertwitch/par2cron/internal/bundler"
	"github.com/desertwitch/par2cron/internal/create"
	"github.com/desertwitch/par2cron/internal/info"
	"github.com/desertwitch/par2cron/internal/list"
	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/repair"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/tool"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/desertwitch/par2cron/internal/verify"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"golang.org/x/term"
)

var (
	profFile    *os.File
	profFileMem *os.File
)

func checkForPar2(ctx context.Context, runner schema.CommandRunner, errout io.Writer) error {
	var out bytes.Buffer

	err := runner.Run(ctx, "par2", []string{"-V"}, "", &out, io.Discard)
	if err != nil {
		fmt.Fprintln(errout, "This command requires a \"par2\" (par2cmdline) installation in your $PATH")

		return fmt.Errorf("exec: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(out.Bytes()))
	if scanner.Scan() {
		schema.Par2Version = strings.TrimSpace(scanner.Text())
	}

	return nil
}

func considerNoColors(cmd *cobra.Command, globalOptions *globalOptions) {
	if !cmd.Flags().Changed("log-plain") {
		isTerminal := false
		// Non-*os.File writers (buffers, wrappers) are treated as non-terminals.
		if f, ok := globalOptions.logOptions.Logout.(*os.File); ok {
			isTerminal = term.IsTerminal(int(f.Fd()))
		}
		// NO_COLOR (https://no-color.org/) or non-terminals get uncolored logs.
		if os.Getenv("NO_COLOR") != "" || !isTerminal {
			globalOptions.logOptions.NoColor = true
		}
	}
}

func stopProfile() {
	if profFile != nil {
		pprof.StopCPUProfile()
		_ = profFile.Close()
		profFile = nil
	}
}

func stopProfileMem() {
	if profFileMem != nil {
		runtime.GC()
		_ = pprof.Lookup("allocs").WriteTo(profFileMem, 0)
		_ = profFileMem.Close()
		profFileMem = nil
	}
}

func wrapArgsError(validator cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := validator(cmd, args); err != nil {
			return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
		}

		return nil
	}
}

type globalOptions struct {
	cgroupPath string
	logOptions *logging.Options
}

func newGlobalOptions() *globalOptions {
	opts := &globalOptions{
		logOptions: &logging.Options{},
	}
	_ = opts.logOptions.LogLevel.Set("info")

	return opts
}

func newRunner(opts *globalOptions) (*util.CtxRunner, error) {
	var ropts []util.RunnerOption

	if opts.cgroupPath != "" {
		ropts = append(ropts, util.WithCgroup(opts.cgroupPath))
	}

	runner, err := util.NewCtxRunner(ropts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %w", err)
	}

	return runner, nil
}

// newRootCmd returns the primary [cobra.Command] pointer for the program.
func newRootCmd(ctx context.Context) *cobra.Command {
	globalOptions := newGlobalOptions()

	rootCmd := &cobra.Command{
		Use:               rootUsage,
		Short:             rootHelpShort,
		Long:              rootHelpLong,
		Version:           schema.ProgramVersion,
		SilenceUsage:      true,
		DisableAutoGenTag: true,
		Args:              wrapArgsError(cobra.NoArgs),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			pp, _ := cmd.Flags().GetString("pprof")
			if pp != "" {
				ppf, err := os.Create(pp)
				if err != nil {
					return fmt.Errorf("%w: failed to create --pprof: %w",
						schema.ErrExitBadInvocation, err)
				}
				profFile = ppf
				if err := pprof.StartCPUProfile(ppf); err != nil {
					return fmt.Errorf("%w: failed to start --pprof: %w",
						schema.ErrExitBadInvocation, err)
				}
			}

			pm, _ := cmd.Flags().GetString("mprof")
			if pm != "" {
				pmf, err := os.Create(pm)
				if err != nil {
					return fmt.Errorf("%w: failed to create --mprof: %w",
						schema.ErrExitBadInvocation, err)
				}
				profFileMem = pmf
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	rootCmd.PersistentFlags().String("pprof", "", "write CPU performance profile to file")
	rootCmd.PersistentFlags().String("mprof", "", "write RAM allocation profile to file")
	rootCmd.PersistentFlags().StringVar(&globalOptions.cgroupPath, "cgroup", "", "cgroup v2 directory to constrain par2 processes")
	rootCmd.PersistentFlags().BoolVar(&globalOptions.logOptions.NoColor, "log-plain", false, "emit uncolored plain-text logs with full timestamps")
	rootCmd.PersistentFlags().VarP(&globalOptions.logOptions.LogLevel, "log-level", "l", "minimum level of emitted logs (debug|info|warn|error)")
	rootCmd.PersistentFlags().StringVar(&globalOptions.logOptions.SeqURL, "seq-url", "", "CLEF ingestion URL for a (remote) Seq logging server")
	rootCmd.PersistentFlags().StringVar(&globalOptions.logOptions.SeqKey, "seq-key", "", "API key for a (remote) Seq logging server")
	rootCmd.PersistentFlags().BoolVar(&globalOptions.logOptions.WantJSON, "json", false, "output results/logs in JSON format (where applicable)")

	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
	})

	createCmd := newCreateCmd(ctx, globalOptions)
	verifyCmd := newVerifyCmd(ctx, globalOptions)
	repairCmd := newRepairCmd(ctx, globalOptions)
	infoCmd := newInfoCmd(ctx, globalOptions)
	listCmd := newListCmd(ctx, globalOptions)
	toolCmd := newToolCmd(ctx, globalOptions)
	bundleCmd := newBundleCmd(ctx, globalOptions)
	configCmd := newConfigCmd(ctx, globalOptions)

	genMarkdownCmd := newGenMarkdownCmd(rootCmd)

	rootCmd.AddCommand(createCmd, verifyCmd, repairCmd, infoCmd, listCmd, toolCmd, bundleCmd, configCmd, genMarkdownCmd)

	return rootCmd
}

func newConfigCmd(ctx context.Context, globalOptions *globalOptions) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   configUsage,
		Short: configHelpShort,
		Long:  configHelpLong,
		Args:  wrapArgsError(cobra.NoArgs),
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}

	configExampleCmd := newConfigExampleCmd(ctx, globalOptions)
	configCheckCmd := newConfigCheckCmd(ctx, globalOptions)

	configCmd.AddCommand(configExampleCmd, configCheckCmd)

	return configCmd
}

func newConfigExampleCmd(_ context.Context, globalOptions *globalOptions) *cobra.Command {
	configExampleCmd := &cobra.Command{
		Use:     configExampleUsage,
		Short:   configExampleHelpShort,
		Long:    configExampleHelpLong,
		Example: configExampleHelpExample,
		Args:    wrapArgsError(cobra.NoArgs),
		PreRun: func(cmd *cobra.Command, _ []string) {
			globalOptions.logOptions.Logout = os.Stderr
			globalOptions.logOptions.Stdout = os.Stdout
			globalOptions.logOptions.Stderr = os.Stderr
			considerNoColors(cmd, globalOptions)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(globalOptions.logOptions.Stdout, configs.ExampleConfiguration)
			if err != nil {
				return fmt.Errorf("failed to print: %w", err)
			}

			return nil
		},
	}

	return configExampleCmd
}

func newConfigCheckCmd(_ context.Context, globalOptions *globalOptions) *cobra.Command {
	configCheckCmd := &cobra.Command{
		Use:     configCheckUsage,
		Short:   configCheckHelpShort,
		Long:    configCheckHelpLong,
		Example: configCheckHelpExample,
		Args:    wrapArgsError(cobra.ExactArgs(1)),
		PreRun: func(cmd *cobra.Command, _ []string) {
			globalOptions.logOptions.Logout = os.Stderr
			globalOptions.logOptions.Stdout = os.Stdout
			globalOptions.logOptions.Stderr = os.Stderr
			considerNoColors(cmd, globalOptions)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			if _, err := parseConfigFile(afero.NewOsFs(), args[0]); err != nil {
				fmt.Fprintln(globalOptions.logOptions.Stdout, "Provided configuration file is invalid.")

				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
			}
			fmt.Fprintln(globalOptions.logOptions.Stdout, "Provided configuration file is valid.")

			return nil
		},
	}

	return configCheckCmd
}

func newGenMarkdownCmd(rootCmd *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "gen-markdown [flags] <dir>",
		Short:  "Generates the markdown documentation",
		Args:   wrapArgsError(cobra.ExactArgs(1)),
		Hidden: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if err := doc.GenMarkdownTree(rootCmd, args[0]); err != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
			}

			return nil
		},
	}
}

func newToolCmd(ctx context.Context, globalOptions *globalOptions) *cobra.Command {
	toolCmd := &cobra.Command{
		Use:   toolUsage,
		Short: toolHelpShort,
		Args:  wrapArgsError(cobra.NoArgs),
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}

	toolMD5Cmd := newToolMD5Cmd(ctx, globalOptions)
	toolCmd.AddCommand(toolMD5Cmd)

	return toolCmd
}

func newToolMD5Cmd(ctx context.Context, globalOptions *globalOptions) *cobra.Command {
	var toolOptions tool.Options

	fsys := afero.NewOsFs()

	toolMD5Cmd := &cobra.Command{
		Use:     toolMD5Usage,
		Short:   toolMD5HelpShort,
		Example: toolMD5HelpExample,
		Args:    wrapArgsError(cobra.MinimumNArgs(1)),
		PreRun: func(cmd *cobra.Command, _ []string) {
			globalOptions.logOptions.Logout = os.Stderr
			globalOptions.logOptions.Stdout = os.Stdout
			globalOptions.logOptions.Stderr = os.Stderr
			considerNoColors(cmd, globalOptions)
		},
		RunE: func(_ *cobra.Command, args []string) (ret error) { //nolint:nonamedreturns
			runner, rerr := newRunner(globalOptions)
			if rerr != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, rerr)
			}
			defer runner.Close()

			prog := NewProgram(fsys, *globalOptions.logOptions, runner, &util.BundleHandler{}, &util.Par2Handler{}, util.GobCacheHandler{})
			defer prog.Shutdown()
			defer recoverOperationPanic(&ret, prog.log.With("op", "tool", "mode", "md5"))

			ctx := context.WithValue(ctx, schema.ModeKey, "md5")

			err := prog.ToolService.OutputMD5(ctx, args, toolOptions)
			if err != nil {
				return fmt.Errorf("tool: md5: %w", err)
			}

			return nil
		},
	}
	toolMD5Cmd.Flags().BoolVar(&toolOptions.ParseAll, "all", false, "attempt to parse all provided files (and not just PAR2 index files)")

	return toolMD5Cmd
}

func newBundleCmd(ctx context.Context, globalOptions *globalOptions) *cobra.Command {
	bundleCmd := &cobra.Command{
		Use:   bundleUsage,
		Short: bundleHelpShort,
		Long:  bundleHelpLong,
		Args:  wrapArgsError(cobra.NoArgs),
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}

	bundlePackCmd := newBundlePackCmd(ctx, globalOptions)
	bundleUnpackCmd := newBundleUnpackCmd(ctx, globalOptions)
	bundleDebugCmd := newBundleDebugCmd(ctx, globalOptions)
	bundleSpecCmd := newBundleSpecCmd(ctx, globalOptions)

	bundleCmd.AddCommand(bundlePackCmd, bundleUnpackCmd, bundleDebugCmd, bundleSpecCmd)

	return bundleCmd
}

func newBundlePackCmd(ctx context.Context, globalOptions *globalOptions) *cobra.Command {
	var bundlerOptions bundler.Options
	var resolvedPaths []string

	fsys := afero.NewOsFs()

	bundlePackCmd := &cobra.Command{
		Use:   bundlePackUsage,
		Short: bundlePackHelpShort,
		Long:  bundlePackHelpLong,
		Args:  wrapArgsError(cobra.MinimumNArgs(1)),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			globalOptions.logOptions.Logout = os.Stderr
			globalOptions.logOptions.Stdout = os.Stdout
			globalOptions.logOptions.Stderr = os.Stderr
			considerNoColors(cmd, globalOptions)

			resolved, err := resolvePathArgs(fsys, args)
			if err != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
			}

			resolvedPaths = slices.Clone(resolved)

			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) (ret error) { //nolint:nonamedreturns
			runner, rerr := newRunner(globalOptions)
			if rerr != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, rerr)
			}
			defer runner.Close()

			prog := NewProgram(fsys, *globalOptions.logOptions, runner, &util.BundleHandler{}, &util.Par2Handler{}, util.GobCacheHandler{})
			defer prog.Shutdown()
			defer recoverOperationPanic(&ret, prog.log.With("op", "bundle", "mode", "pack"))

			ctx := context.WithValue(ctx, schema.ModeKey, "pack")

			result, err := prog.BundlerService.Pack(ctx, resolvedPaths, bundlerOptions)
			logOperationResult(err, result, prog.log.With("op", "bundle", "mode", "pack"))
			if err != nil {
				return fmt.Errorf("bundle: pack: %w", err)
			}

			return nil
		},
	}
	bundlePackCmd.Flags().BoolVar(&bundlerOptions.SkipNotCreated, "skip-not-created", false, "skip PAR2 sets without a par2cron manifest containing a creation record")
	bundlePackCmd.Flags().BoolVarP(&bundlerOptions.IncludeExternal, "include-external", "e", false, "include PAR2 sets without a par2cron manifest (and create one)")

	return bundlePackCmd
}

func newBundleUnpackCmd(ctx context.Context, globalOptions *globalOptions) *cobra.Command {
	var bundlerOptions bundler.Options
	var resolvedPaths []string

	fsys := afero.NewOsFs()

	bundleUnpackCmd := &cobra.Command{
		Use:   bundleUnpackUsage,
		Short: bundleUnpackHelpShort,
		Long:  bundleUnpackHelpLong,
		Args:  wrapArgsError(cobra.MinimumNArgs(1)),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			globalOptions.logOptions.Logout = os.Stderr
			globalOptions.logOptions.Stdout = os.Stdout
			globalOptions.logOptions.Stderr = os.Stderr
			considerNoColors(cmd, globalOptions)

			resolved, err := resolvePathArgs(fsys, args)
			if err != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
			}

			resolvedPaths = slices.Clone(resolved)

			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) (ret error) { //nolint:nonamedreturns
			runner, rerr := newRunner(globalOptions)
			if rerr != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, rerr)
			}
			defer runner.Close()

			prog := NewProgram(fsys, *globalOptions.logOptions, runner, &util.BundleHandler{}, &util.Par2Handler{}, util.GobCacheHandler{})
			defer prog.Shutdown()
			defer recoverOperationPanic(&ret, prog.log.With("op", "bundle", "mode", "unpack"))

			ctx := context.WithValue(ctx, schema.ModeKey, "unpack")

			result, err := prog.BundlerService.Unpack(ctx, resolvedPaths, bundlerOptions)
			logOperationResult(err, result, prog.log.With("op", "bundle", "mode", "unpack"))
			if err != nil {
				return fmt.Errorf("bundle: unpack: %w", err)
			}

			return nil
		},
	}
	bundleUnpackCmd.Flags().BoolVar(&bundlerOptions.Force, "force", false, "proceed regardless of errors/corruption (use with care)")

	return bundleUnpackCmd
}

func newBundleDebugCmd(ctx context.Context, globalOptions *globalOptions) *cobra.Command {
	fsys := afero.NewOsFs()

	bundleDebugCmd := &cobra.Command{
		Use:     bundleDebugUsage,
		Short:   bundleDebugHelpShort,
		Long:    bundleDebugHelpLong,
		Example: bundleDebugHelpExample,
		Args:    wrapArgsError(cobra.MinimumNArgs(1)),
		PreRun: func(cmd *cobra.Command, _ []string) {
			globalOptions.logOptions.Logout = os.Stderr
			globalOptions.logOptions.Stdout = os.Stdout
			globalOptions.logOptions.Stderr = os.Stderr
			considerNoColors(cmd, globalOptions)
		},
		RunE: func(_ *cobra.Command, args []string) (ret error) { //nolint:nonamedreturns
			runner, rerr := newRunner(globalOptions)
			if rerr != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, rerr)
			}
			defer runner.Close()

			prog := NewProgram(fsys, *globalOptions.logOptions, runner, &util.BundleHandler{}, &util.Par2Handler{}, util.GobCacheHandler{})
			defer prog.Shutdown()
			defer recoverOperationPanic(&ret, prog.log.With("op", "bundle", "mode", "debug"))

			ctx := context.WithValue(ctx, schema.ModeKey, "debug")

			err := prog.BundlerService.OutputJSON(ctx, args)
			if err != nil {
				return fmt.Errorf("bundle: debug: %w", err)
			}

			return nil
		},
	}

	return bundleDebugCmd
}

func newBundleSpecCmd(_ context.Context, globalOptions *globalOptions) *cobra.Command {
	bundleSpecCmd := &cobra.Command{
		Use:     bundleSpecUsage,
		Short:   bundleSpecHelpShort,
		Long:    bundleSpecHelpLong,
		Example: bundleSpecHelpExample,
		Args:    wrapArgsError(cobra.NoArgs),
		PreRun: func(cmd *cobra.Command, _ []string) {
			globalOptions.logOptions.Logout = os.Stderr
			globalOptions.logOptions.Stdout = os.Stdout
			globalOptions.logOptions.Stderr = os.Stderr
			considerNoColors(cmd, globalOptions)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(globalOptions.logOptions.Stdout, specs.BundleSpecification)
			if err != nil {
				return fmt.Errorf("failed to print: %w", err)
			}

			return nil
		},
	}

	return bundleSpecCmd
}

// newCreateCmd returns the "create" [cobra.Command] pointer for the program.
func newCreateCmd(ctx context.Context, globalOptions *globalOptions) *cobra.Command {
	var createOptions create.Options
	var configPath string
	var resolvedPaths []string

	fsys := afero.NewOsFs()

	_ = createOptions.Par2Mode.Set(schema.CreateFolderMode)

	createCmd := &cobra.Command{
		Use:     createUsage,
		Short:   createHelpShort,
		Long:    createHelpLong,
		Example: createHelpExample,
		Args:    wrapArgsError(cobra.MinimumNArgs(1)),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			globalOptions.logOptions.Logout = os.Stderr
			globalOptions.logOptions.Stdout = os.Stdout
			globalOptions.logOptions.Stderr = os.Stderr
			considerNoColors(cmd, globalOptions)

			if err := checkForPar2(ctx, &util.CtxRunner{}, globalOptions.logOptions.Stderr); err != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
			}

			result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
				FSys:           fsys,
				Args:           args,
				DashAt:         cmd.ArgsLenAtDash(),
				ConfigPath:     configPath,
				CommandOptions: &createOptions, // mutated
				GlobalOptions:  globalOptions,  // mutated
				ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
				VisitFlags:     cmd.Flags().Visit,
			})
			if err != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
			}

			resolvedPaths = slices.Clone(result.ResolvedPaths)

			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) (ret error) { //nolint:nonamedreturns
			runner, rerr := newRunner(globalOptions)
			if rerr != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, rerr)
			}
			defer runner.Close()

			prog := NewProgram(fsys, *globalOptions.logOptions, runner, &util.BundleHandler{}, &util.Par2Handler{}, util.GobCacheHandler{})
			defer prog.Shutdown()
			defer recoverOperationPanic(&ret, prog.log.With("op", "create"))

			result, err := prog.CreationService.Create(ctx, resolvedPaths, createOptions)
			logOperationResult(err, result, prog.log.With("op", "create"))
			if err != nil {
				return fmt.Errorf("create: %w", err)
			}

			return nil
		},
	}
	createCmd.Flags().BoolVar(&createOptions.HideFiles, "hidden", false, "create PAR2 sets and related files as hidden (dotfiles)")
	createCmd.Flags().BoolVarP(&createOptions.Bundle, "bundle", "b", false, "bundle created PAR2 sets into one single file")
	createCmd.Flags().BoolVarP(&createOptions.Par2Verify, "verify", "v", false, "PAR2 sets must pass verification as part of creation")
	createCmd.Flags().StringVarP(&configPath, "config", "c", "", "path to a par2cron YAML configuration file")
	createCmd.Flags().StringVarP(&createOptions.Par2Glob, "glob", "g", "*", "PAR2 set default glob (files to include)")
	createCmd.Flags().VarP(&createOptions.MaxDuration, "duration", "d", "time budget per run (best effort/soft limit)")
	createCmd.Flags().VarP(&createOptions.Par2Mode, "mode", "m", "PAR2 set default mode; creates a set per (folder|nested|file|recursive)")

	return createCmd
}

// newVerifyCmd returns the "verify" [cobra.Command] pointer for the program.
func newVerifyCmd(ctx context.Context, globalOptions *globalOptions) *cobra.Command {
	var verifyOptions verify.Options
	var configPath string
	var resolvedPaths []string

	fsys := afero.NewOsFs()

	_ = verifyOptions.RunInterval.Set("24h")

	verifyCmd := &cobra.Command{
		Use:     verifyUsage,
		Short:   verifyHelpShort,
		Long:    verifyHelpLong,
		Example: verifyHelpExample,
		Args:    wrapArgsError(cobra.MinimumNArgs(1)),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			globalOptions.logOptions.Logout = os.Stderr
			globalOptions.logOptions.Stdout = os.Stdout
			globalOptions.logOptions.Stderr = os.Stderr
			considerNoColors(cmd, globalOptions)

			if err := checkForPar2(ctx, &util.CtxRunner{}, globalOptions.logOptions.Stderr); err != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
			}

			result, err := runPrelude(&preludeInput[*verify.Options, *configFileVerify]{
				FSys:           fsys,
				Args:           args,
				DashAt:         cmd.ArgsLenAtDash(),
				ConfigPath:     configPath,
				CommandOptions: &verifyOptions, // mutated
				GlobalOptions:  globalOptions,  // mutated
				ExtractSection: func(cfg *configFile) *configFileVerify { return cfg.Verify },
				VisitFlags:     cmd.Flags().Visit,
			})
			if err != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
			}

			resolvedPaths = slices.Clone(result.ResolvedPaths)

			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) (ret error) { //nolint:nonamedreturns
			runner, rerr := newRunner(globalOptions)
			if rerr != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, rerr)
			}
			defer runner.Close()

			prog := NewProgram(fsys, *globalOptions.logOptions, runner, &util.BundleHandler{}, &util.Par2Handler{}, util.GobCacheHandler{})
			defer prog.Shutdown()
			defer recoverOperationPanic(&ret, prog.log.With("op", "verify"))

			result, err := prog.VerificationService.Verify(ctx, resolvedPaths, verifyOptions)
			logOperationResult(err, result, prog.log.With("op", "verify"))
			if err != nil {
				return fmt.Errorf("verify: %w", err)
			}

			return nil
		},
	}
	verifyCmd.Flags().BoolVar(&verifyOptions.SkipNotCreated, "skip-not-created", false, "skip PAR2 sets without a par2cron manifest containing a creation record")
	verifyCmd.Flags().BoolVarP(&verifyOptions.IncludeExternal, "include-external", "e", false, "include PAR2 sets without a par2cron manifest (and create one)")
	verifyCmd.Flags().StringVarP(&configPath, "config", "c", "", "path to a par2cron YAML configuration file")
	verifyCmd.Flags().StringVar(&verifyOptions.CacheDir, "cache", "", "directory for optional manifest cache (use same for all commands)")
	verifyCmd.Flags().VarP(&verifyOptions.MaxDuration, "duration", "d", "time budget per run (best effort/soft limit)")
	verifyCmd.Flags().VarP(&verifyOptions.MinAge, "age", "a", "minimum time between re-verifications (skip if verified within this period)")
	verifyCmd.Flags().VarP(&verifyOptions.RunInterval, "calc-run-interval", "i", "how often you run par2cron verify (for backlog calculations)")

	return verifyCmd
}

// newRepairCmd returns the "repair" [cobra.Command] pointer for the program.
func newRepairCmd(ctx context.Context, globalOptions *globalOptions) *cobra.Command {
	var repairOptions repair.Options
	var configPath string
	var resolvedPaths []string

	fsys := afero.NewOsFs()

	repairCmd := &cobra.Command{
		Use:     repairUsage,
		Short:   repairHelpShort,
		Long:    repairHelpLong,
		Example: repairHelpExample,
		Args:    wrapArgsError(cobra.MinimumNArgs(1)),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			globalOptions.logOptions.Logout = os.Stderr
			globalOptions.logOptions.Stdout = os.Stdout
			globalOptions.logOptions.Stderr = os.Stderr
			considerNoColors(cmd, globalOptions)

			if err := checkForPar2(ctx, &util.CtxRunner{}, globalOptions.logOptions.Stderr); err != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
			}

			result, err := runPrelude(&preludeInput[*repair.Options, *configFileRepair]{
				FSys:           fsys,
				Args:           args,
				DashAt:         cmd.ArgsLenAtDash(),
				ConfigPath:     configPath,
				CommandOptions: &repairOptions, // mutated
				GlobalOptions:  globalOptions,  // mutated
				ExtractSection: func(cfg *configFile) *configFileRepair { return cfg.Repair },
				VisitFlags:     cmd.Flags().Visit,
			})
			if err != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
			}

			resolvedPaths = slices.Clone(result.ResolvedPaths)

			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) (ret error) { //nolint:nonamedreturns
			runner, rerr := newRunner(globalOptions)
			if rerr != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, rerr)
			}
			defer runner.Close()

			prog := NewProgram(fsys, *globalOptions.logOptions, runner, &util.BundleHandler{}, &util.Par2Handler{}, util.GobCacheHandler{})
			defer prog.Shutdown()
			defer recoverOperationPanic(&ret, prog.log.With("op", "repair"))

			result, err := prog.RepairService.Repair(ctx, resolvedPaths, repairOptions)
			logOperationResult(err, result, prog.log.With("op", "repair"))
			if err != nil {
				return fmt.Errorf("repair: %w", err)
			}

			return nil
		},
	}
	repairCmd.Flags().BoolVar(&repairOptions.SkipNotCreated, "skip-not-created", false, "skip PAR2 sets without a par2cron manifest containing a creation record")
	repairCmd.Flags().BoolVar(&repairOptions.SkipMaybeEdited, "skip-maybe-edited", false, "skip PAR2 sets where protected files may have been edited (newer mtimes)")
	repairCmd.Flags().BoolVar(&repairOptions.NoRestoreAttributes, "no-restore-attributes", false, "do not restore pre-repair mode, times and ownership on repaired files")
	repairCmd.Flags().BoolVarP(&repairOptions.AttemptUnrepairables, "attempt-unrepairables", "u", false, "attempt to repair PAR2 sets marked as unrepairable")
	repairCmd.Flags().BoolVarP(&repairOptions.Par2Verify, "verify", "v", false, "PAR2 sets must pass verification as part of repair")
	repairCmd.Flags().BoolVarP(&repairOptions.PurgeBackups, "purge-backups", "p", false, "remove obsolete backup files (.1, .2, ...) after successful repair")
	repairCmd.Flags().BoolVarP(&repairOptions.RestoreBackups, "restore-backups", "r", false, "roll back protected files to pre-repair state after unsuccessful repair")
	repairCmd.Flags().IntVarP(&repairOptions.MinTestedCount, "min-tested", "t", 0, "repair only when verified as corrupted at least X times")
	repairCmd.Flags().StringVar(&repairOptions.CacheDir, "cache", "", "directory for optional manifest cache (use same for all commands)")
	repairCmd.Flags().StringVarP(&configPath, "config", "c", "", "path to a par2cron YAML configuration file")
	repairCmd.Flags().VarP(&repairOptions.MaxDuration, "duration", "d", "time budget per run (best effort/soft limit)")

	return repairCmd
}

func newInfoCmd(ctx context.Context, globalOptions *globalOptions) *cobra.Command {
	var infoOptions info.Options
	var configPath string
	var resolvedPaths []string

	fsys := afero.NewOsFs()

	_ = infoOptions.RunInterval.Set("24h")

	infoCmd := &cobra.Command{
		Use:     infoUsage,
		Short:   infoHelpShort,
		Long:    infoHelpLong,
		Example: infoHelpExample,
		Args:    wrapArgsError(cobra.MinimumNArgs(1)),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			globalOptions.logOptions.Logout = os.Stderr
			globalOptions.logOptions.Stdout = os.Stdout
			globalOptions.logOptions.Stderr = os.Stderr
			considerNoColors(cmd, globalOptions)

			result, err := runPrelude(&preludeInput[*info.Options, *configFileInfo]{
				FSys:           fsys,
				Args:           args,
				DashAt:         -1, // no --
				ConfigPath:     configPath,
				CommandOptions: &infoOptions,  // mutated
				GlobalOptions:  globalOptions, // mutated
				ExtractSection: func(cfg *configFile) *configFileInfo { return cfg.Info },
				VisitFlags:     cmd.Flags().Visit,
			})
			if err != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
			}

			resolvedPaths = slices.Clone(result.ResolvedPaths)

			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) (ret error) { //nolint:nonamedreturns
			runner, rerr := newRunner(globalOptions)
			if rerr != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, rerr)
			}
			defer runner.Close()

			prog := NewProgram(fsys, *globalOptions.logOptions, runner, &util.BundleHandler{}, &util.Par2Handler{}, util.GobCacheHandler{})
			defer prog.Shutdown()
			defer recoverOperationPanic(&ret, prog.log.With("op", "info"))

			err := prog.InfoService.Info(ctx, resolvedPaths, infoOptions)
			if err != nil {
				return fmt.Errorf("info: %w", err)
			}

			return nil
		},
	}
	infoCmd.Flags().BoolVar(&infoOptions.SkipNotCreated, "skip-not-created", false, "skip PAR2 sets without a par2cron manifest containing a creation record")
	infoCmd.Flags().BoolVar(&infoOptions.Prometheus, "prometheus", false, "output as Prometheus metrics (e.g. node_exporter textfile, Pushgateway)")
	infoCmd.Flags().BoolVarP(&infoOptions.IncludeExternal, "include-external", "e", false, "include external PAR2 sets without a par2cron manifest")
	infoCmd.Flags().StringVarP(&configPath, "config", "c", "", "path to a par2cron YAML configuration file")
	infoCmd.Flags().StringVar(&infoOptions.CacheDir, "cache", "", "directory for optional manifest cache (use same for all commands)")
	infoCmd.Flags().VarP(&infoOptions.MaxDuration, "duration", "d", "target time budget for each verify run (soft limit)")
	infoCmd.Flags().VarP(&infoOptions.MinAge, "age", "a", "target cycle length (time between re-verifications)")
	infoCmd.Flags().VarP(&infoOptions.RunInterval, "calc-run-interval", "i", "how often you run par2cron verify")

	return infoCmd
}

func newListCmd(ctx context.Context, globalOptions *globalOptions) *cobra.Command {
	var listOptions list.Options
	var resolvedPaths []string

	fsys := afero.NewOsFs()

	listCmd := &cobra.Command{
		Use:     listUsage,
		Short:   listHelpShort,
		Long:    listHelpLong,
		Example: listHelpExample,
		Args:    wrapArgsError(cobra.MinimumNArgs(1)),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			globalOptions.logOptions.Logout = os.Stderr
			globalOptions.logOptions.Stdout = os.Stdout
			globalOptions.logOptions.Stderr = os.Stderr
			considerNoColors(cmd, globalOptions)

			resolved, err := resolvePathArgs(fsys, args)
			if err != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
			}

			resolvedPaths = slices.Clone(resolved)

			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) (ret error) { //nolint:nonamedreturns
			runner, rerr := newRunner(globalOptions)
			if rerr != nil {
				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, rerr)
			}
			defer runner.Close()

			prog := NewProgram(fsys, *globalOptions.logOptions, runner, &util.BundleHandler{}, &util.Par2Handler{}, util.GobCacheHandler{})
			defer prog.Shutdown()
			defer recoverOperationPanic(&ret, prog.log.With("op", "list"))

			err := prog.ListService.List(ctx, resolvedPaths, listOptions)
			if err != nil {
				return fmt.Errorf("list: %w", err)
			}

			return nil
		},
	}
	listCmd.Flags().BoolVar(&listOptions.SkipNotCreated, "skip-not-created", false, "skip PAR2 sets without a par2cron manifest containing a creation record")
	listCmd.Flags().StringVar(&listOptions.CacheDir, "cache", "", "directory for optional manifest cache (use same for all commands)")

	return listCmd
}

type Program struct {
	CreationService     *create.Service
	VerificationService *verify.Service
	RepairService       *repair.Service
	InfoService         *info.Service
	ListService         *list.Service
	BundlerService      *bundler.Service
	ToolService         *tool.Service

	log *logging.Logger
}

func NewProgram(
	fsys afero.Fs,
	o logging.Options,
	r schema.CommandRunner,
	b schema.BundleHandler,
	p schema.Par2Handler,
	c schema.CacheHandler,
) *Program {
	log := logging.NewLogger(o)

	return &Program{
		CreationService:     create.NewService(fsys, log, r, b, p, c),
		VerificationService: verify.NewService(fsys, log, r, b, c),
		RepairService:       repair.NewService(fsys, log, r, b, p, c),
		InfoService:         info.NewService(fsys, log, r, b, c),
		ListService:         list.NewService(fsys, log, r, b, c),
		BundlerService:      bundler.NewService(fsys, log, b, p),
		ToolService:         tool.NewService(fsys, log, b, p),

		log: log,
	}
}

func (prog *Program) Shutdown() {
	prog.log.Close()
}

func recoverOperationPanic(ret *error, log *logging.Logger) {
	if r := recover(); r != nil {
		log.Error("Operation crashed due to a panic (report to developers)",
			"panic", r, "stack", string(debug.Stack()))

		*ret = schema.ErrExitUnclassified
	}
}

func logOperationResult(err error, result util.ResultTracker, log *logging.Logger) {
	processedCount := result.Success + result.Error + result.Skipped

	switch {
	case err == nil && result.Error == 0:
		log.Info(
			fmt.Sprintf("Operation completed (%d/%d jobs processed)",
				processedCount, result.Selected),
			"successCount", result.Success,
			"skipCount", result.Skipped,
			"errorCount", result.Error,
			"processedCount", processedCount,
			"selectedCount", result.Selected,
		)

	case errors.Is(err, context.Canceled):
		log.Error(
			fmt.Sprintf("Operation interrupted (%d/%d jobs processed)",
				processedCount, result.Selected),
			"successCount", result.Success,
			"skipCount", result.Skipped,
			"errorCount", result.Error,
			"processedCount", processedCount,
			"selectedCount", result.Selected,
		)

	default:
		log.Error(
			fmt.Sprintf("Operation completed with errors (%d/%d jobs processed)",
				processedCount, result.Selected),
			"successCount", result.Success,
			"skipCount", result.Skipped,
			"errorCount", result.Error,
			"processedCount", processedCount,
			"selectedCount", result.Selected,
			"error", err,
		)
	}
}

func main() {
	var exitCode int
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr,
				"Program crashed due to a panic (report to developers): \n\n%v\n\n", r)
			debug.PrintStack()
			exitCode = schema.ExitCodeUnclassified
		}
		os.Exit(exitCode)
	}()

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM, syscall.SIGPIPE)
	defer stop()

	cobra.OnFinalize(func() {
		// https://github.com/spf13/cobra/issues/1893#issuecomment-1573951697
		stopProfile()
		stopProfileMem()
	})

	clean, perr := setupPar2()
	if perr != nil {
		fmt.Fprintf(os.Stderr, "Failed to set up embedded \"par2\" (par2cmdline): %v\n", perr)
	}
	defer clean()

	rootCmd := newRootCmd(ctx)
	err := rootCmd.Execute()
	exitCode = schema.ExitCodeFor(err)
}
