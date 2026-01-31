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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"

	"github.com/desertwitch/par2cron/internal/create"
	"github.com/desertwitch/par2cron/internal/info"
	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/repair"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/desertwitch/par2cron/internal/verify"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func wrapArgsError(validator cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := validator(cmd, args); err != nil {
			return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
		}

		return nil
	}
}

// newRootCmd returns the primary [cobra.Command] pointer for the program.
func newRootCmd(ctx context.Context) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:               rootUsage,
		Short:             rootHelpShort,
		Long:              rootHelpLong,
		Version:           schema.ProgramVersion,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			par2cmd := exec.CommandContext(ctx, "par2", "-V")
			par2cmd.WaitDelay = util.ProcessKillTimeout

			if err := par2cmd.Run(); err != nil {
				fmt.Fprintln(os.Stderr, "This program requires a \"par2\" (par2cmdline) installation in your $PATH")

				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
			}

			return nil
		},
	}

	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
	})

	createCmd := newCreateCmd(ctx)
	verifyCmd := newVerifyCmd(ctx)
	repairCmd := newRepairCmd(ctx)
	infoCmd := newInfoCmd(ctx)
	checkConfigCmd := newCheckConfigCmd(ctx)

	rootCmd.AddCommand(createCmd, verifyCmd, repairCmd, infoCmd, checkConfigCmd)

	return rootCmd
}

func newCheckConfigCmd(_ context.Context) *cobra.Command {
	checkConfigCmd := &cobra.Command{
		Use:     checkConfigUsage,
		Short:   checkConfigHelpShort,
		Long:    checkConfigHelpLong,
		Example: checkConfigHelpExample,
		Args:    wrapArgsError(cobra.ExactArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			if _, err := parseConfigFile(afero.NewOsFs(), args[0]); err != nil {
				fmt.Fprintln(os.Stdout, "Provided configuration file is invalid.")

				return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
			}
			fmt.Fprintln(os.Stdout, "Provided configuration file is valid.")

			return nil
		},
	}

	return checkConfigCmd
}

// newCreateCmd returns the "create" [cobra.Command] pointer for the program.
func newCreateCmd(ctx context.Context) *cobra.Command {
	var createArgs create.Options
	var logSettings logging.Options
	var configPath string

	fsys := afero.NewOsFs()

	_ = logSettings.LogLevel.Set("info")
	logSettings.Logout = os.Stderr
	logSettings.Stdout = os.Stdout
	logSettings.Stderr = os.Stderr

	_ = createArgs.Par2Mode.Set(schema.CreateFolderMode)

	createCmd := &cobra.Command{
		Use:     createUsage,
		Short:   createHelpShort,
		Long:    createHelpLong,
		Example: createHelpExample,
		Args:    wrapArgsError(cobra.MinimumNArgs(1)),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			dashAt := cmd.ArgsLenAtDash()
			hasExternalArgs := (dashAt != -1)

			if (dashAt == -1 && len(args) > 1) || (dashAt != -1 && dashAt != 1) {
				return fmt.Errorf("%w: unexpected arguments before -- or missing -- before par2 arguments",
					schema.ErrExitBadInvocation)
			}

			if configPath != "" {
				cfg, err := parseConfigFile(fsys, configPath)
				if err != nil {
					return fmt.Errorf("%w: failed to parse --config file: %w",
						schema.ErrExitBadInvocation, err)
				}
				if cfg.Create != nil {
					setFlags := make(map[string]bool)
					cmd.Flags().Visit(func(f *pflag.Flag) {
						setFlags[f.Name] = true
					})
					cfg.Create.Merge(&createArgs, &logSettings, hasExternalArgs, setFlags)
				}
			}

			path, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("%w: failed to convert relative path to absolute: %w",
					schema.ErrExitBadInvocation, err)
			}
			args[0] = path

			if hasExternalArgs {
				createArgs.Par2Args = append([]string{}, args[dashAt:]...)
			}

			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			prog := NewProgram(fsys, logSettings, &util.CtxRunner{})

			result, err := prog.CreationService.Create(ctx, args[0], createArgs)
			if result != nil {
				logOperationResult(err, result, prog.log.With("op", "create"))
			}
			if err != nil {
				return fmt.Errorf("create: %w", err)
			}

			return nil
		},
	}
	createCmd.Flags().BoolVar(&logSettings.WantJSON, "json", false, "output structured logs in JSON format")
	createCmd.Flags().BoolVar(&createArgs.HideFiles, "hidden", false, "create PAR2 sets and related files as hidden (dotfiles)")
	createCmd.Flags().BoolVarP(&createArgs.Par2Verify, "verify", "v", false, "PAR2 sets must pass verification as part of creation")
	createCmd.Flags().StringVarP(&configPath, "config", "c", "", "path to a par2cron YAML configuration file")
	createCmd.Flags().StringVarP(&createArgs.Par2Glob, "glob", "g", "*", "PAR2 set default glob (files to include)")
	createCmd.Flags().VarP(&createArgs.MaxDuration, "duration", "d", "time budget per run (best effort/soft limit)")
	createCmd.Flags().VarP(&createArgs.Par2Mode, "mode", "m", "PAR2 set default mode; per-file or per-folder (file|folder)")
	createCmd.Flags().VarP(&logSettings.LogLevel, "log-level", "l", "minimum level of emitted logs (debug|info|warn|error)")

	return createCmd
}

// newVerifyCmd returns the "verify" [cobra.Command] pointer for the program.
func newVerifyCmd(ctx context.Context) *cobra.Command {
	var verifyArgs verify.Options
	var logSettings logging.Options
	var configPath string

	fsys := afero.NewOsFs()

	_ = logSettings.LogLevel.Set("info")
	logSettings.Logout = os.Stderr
	logSettings.Stdout = os.Stdout
	logSettings.Stderr = os.Stderr

	_ = verifyArgs.RunInterval.Set("24h")

	verifyCmd := &cobra.Command{
		Use:     verifyUsage,
		Short:   verifyHelpShort,
		Long:    verifyHelpLong,
		Example: verifyHelpExample,
		Args:    wrapArgsError(cobra.MinimumNArgs(1)),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			dashAt := cmd.ArgsLenAtDash()
			hasExternalArgs := (dashAt != -1)

			if (dashAt == -1 && len(args) > 1) || (dashAt != -1 && dashAt != 1) {
				return fmt.Errorf("%w: unexpected arguments before -- or missing -- before par2 arguments",
					schema.ErrExitBadInvocation)
			}

			if configPath != "" {
				cfg, err := parseConfigFile(fsys, configPath)
				if err != nil {
					return fmt.Errorf("%w: failed to parse --config file: %w",
						schema.ErrExitBadInvocation, err)
				}
				if cfg.Verify != nil {
					setFlags := make(map[string]bool)
					cmd.Flags().Visit(func(f *pflag.Flag) {
						setFlags[f.Name] = true
					})
					cfg.Verify.Merge(&verifyArgs, &logSettings, hasExternalArgs, setFlags)
				}
			}

			path, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("%w: failed to convert relative path to absolute: %w",
					schema.ErrExitBadInvocation, err)
			}
			args[0] = path

			if hasExternalArgs {
				verifyArgs.Par2Args = append([]string{}, args[dashAt:]...)
			}

			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			prog := NewProgram(fsys, logSettings, &util.CtxRunner{})

			result, err := prog.VerificationService.Verify(ctx, args[0], verifyArgs)
			if result != nil {
				logOperationResult(err, result, prog.log.With("op", "verify"))
			}
			if err != nil {
				return fmt.Errorf("verify: %w", err)
			}

			return nil
		},
	}
	verifyCmd.Flags().BoolVar(&logSettings.WantJSON, "json", false, "output structured logs in JSON format")
	verifyCmd.Flags().BoolVar(&verifyArgs.SkipNotCreated, "skip-not-created", false, "skip PAR2 sets without a par2cron manifest containing a creation record")
	verifyCmd.Flags().BoolVarP(&verifyArgs.IncludeExternal, "include-external", "e", false, "include PAR2 sets without a par2cron manifest (and create one)")
	verifyCmd.Flags().StringVarP(&configPath, "config", "c", "", "path to a par2cron YAML configuration file")
	verifyCmd.Flags().VarP(&logSettings.LogLevel, "log-level", "l", "minimum level of emitted logs (debug|info|warn|error)")
	verifyCmd.Flags().VarP(&verifyArgs.MaxDuration, "duration", "d", "time budget per run (best effort/soft limit)")
	verifyCmd.Flags().VarP(&verifyArgs.MinAge, "age", "a", "minimum time between re-verifications (skip if verified within this period)")
	verifyCmd.Flags().VarP(&verifyArgs.RunInterval, "calc-run-interval", "i", "how often you run par2cron verify (for backlog calculations)")

	return verifyCmd
}

// newRepairCmd returns the "repair" [cobra.Command] pointer for the program.
func newRepairCmd(ctx context.Context) *cobra.Command {
	var repairArgs repair.Options
	var logSettings logging.Options
	var configPath string

	fsys := afero.NewOsFs()

	_ = logSettings.LogLevel.Set("info")
	logSettings.Logout = os.Stderr
	logSettings.Stdout = os.Stdout
	logSettings.Stderr = os.Stderr

	repairCmd := &cobra.Command{
		Use:     repairUsage,
		Short:   repairHelpShort,
		Long:    repairHelpLong,
		Example: repairHelpExample,
		Args:    wrapArgsError(cobra.MinimumNArgs(1)),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			dashAt := cmd.ArgsLenAtDash()
			hasExternalArgs := (dashAt != -1)

			if (dashAt == -1 && len(args) > 1) || (dashAt != -1 && dashAt != 1) {
				return fmt.Errorf("%w: unexpected arguments before -- or missing -- before par2 arguments",
					schema.ErrExitBadInvocation)
			}

			if configPath != "" {
				cfg, err := parseConfigFile(fsys, configPath)
				if err != nil {
					return fmt.Errorf("%w: failed to parse --config file: %w",
						schema.ErrExitBadInvocation, err)
				}
				if cfg.Repair != nil {
					setFlags := make(map[string]bool)
					cmd.Flags().Visit(func(f *pflag.Flag) {
						setFlags[f.Name] = true
					})
					cfg.Repair.Merge(&repairArgs, &logSettings, hasExternalArgs, setFlags)
				}
			}

			path, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("%w: failed to convert relative path to absolute: %w",
					schema.ErrExitBadInvocation, err)
			}
			args[0] = path

			if hasExternalArgs {
				repairArgs.Par2Args = append([]string{}, args[dashAt:]...)
			}

			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			prog := NewProgram(fsys, logSettings, &util.CtxRunner{})

			result, err := prog.RepairService.Repair(ctx, args[0], repairArgs)
			if result != nil {
				logOperationResult(err, result, prog.log.With("op", "repair"))
			}
			if err != nil {
				return fmt.Errorf("repair: %w", err)
			}

			return nil
		},
	}
	repairCmd.Flags().BoolVar(&logSettings.WantJSON, "json", false, "output structured logs in JSON format")
	repairCmd.Flags().BoolVar(&repairArgs.SkipNotCreated, "skip-not-created", false, "skip PAR2 sets without a par2cron manifest containing a creation record")
	repairCmd.Flags().BoolVarP(&repairArgs.AttemptUnrepairables, "attempt-unrepairables", "u", false, "attempt to repair PAR2 sets marked as unrepairable")
	repairCmd.Flags().BoolVarP(&repairArgs.Par2Verify, "verify", "v", false, "PAR2 sets must pass verification as part of repair")
	repairCmd.Flags().BoolVarP(&repairArgs.PurgeBackups, "purge-backups", "p", false, "remove obsolete backup files (.1, .2, ...) after successful repair")
	repairCmd.Flags().BoolVarP(&repairArgs.RestoreBackups, "restore-backups", "r", false, "roll back protected files to pre-repair state after unsuccessful repair")
	repairCmd.Flags().IntVarP(&repairArgs.MinTestedCount, "min-tested", "t", 0, "repair only when verified as corrupted at least X times")
	repairCmd.Flags().StringVarP(&configPath, "config", "c", "", "path to a par2cron YAML configuration file")
	repairCmd.Flags().VarP(&logSettings.LogLevel, "log-level", "l", "minimum level of emitted logs (debug|info|warn|error)")
	repairCmd.Flags().VarP(&repairArgs.MaxDuration, "duration", "d", "time budget per run (best effort/soft limit)")

	return repairCmd
}

func newInfoCmd(ctx context.Context) *cobra.Command {
	var infoArgs info.Options
	var logSettings logging.Options
	var configPath string

	fsys := afero.NewOsFs()

	_ = logSettings.LogLevel.Set("info")
	logSettings.Logout = os.Stderr
	logSettings.Stdout = os.Stdout
	logSettings.Stderr = os.Stderr

	_ = infoArgs.RunInterval.Set("24h")

	infoCmd := &cobra.Command{
		Use:     infoUsage,
		Short:   infoHelpShort,
		Long:    infoHelpLong,
		Example: infoHelpExample,
		Args:    wrapArgsError(cobra.ExactArgs(1)),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if configPath != "" {
				cfg, err := parseConfigFile(fsys, configPath)
				if err != nil {
					return fmt.Errorf("%w: failed to parse --config file: %w",
						schema.ErrExitBadInvocation, err)
				}
				if cfg.Info != nil {
					setFlags := make(map[string]bool)
					cmd.Flags().Visit(func(f *pflag.Flag) {
						setFlags[f.Name] = true
					})
					cfg.Info.Merge(&infoArgs, &logSettings, false, setFlags)
				}
			}

			path, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("%w: failed to convert relative path to absolute: %w",
					schema.ErrExitBadInvocation, err)
			}
			args[0] = path

			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			prog := NewProgram(fsys, logSettings, &util.CtxRunner{})

			return prog.InfoService.Info(ctx, args[0], infoArgs)
		},
	}
	infoCmd.Flags().BoolVar(&infoArgs.SkipNotCreated, "skip-not-created", false, "skip PAR2 sets without a par2cron manifest containing a creation record")
	infoCmd.Flags().BoolVarP(&infoArgs.IncludeExternal, "include-external", "e", false, "include external PAR2 sets without a par2cron manifest")
	infoCmd.Flags().StringVarP(&configPath, "config", "c", "", "path to a par2cron YAML configuration file")
	infoCmd.Flags().VarP(&infoArgs.MaxDuration, "duration", "d", "target time budget for each verify run (soft limit)")
	infoCmd.Flags().VarP(&infoArgs.MinAge, "age", "a", "target cycle length (time between re-verifications)")
	infoCmd.Flags().VarP(&infoArgs.RunInterval, "calc-run-interval", "i", "how often you run par2cron verify")
	infoCmd.Flags().VarP(&logSettings.LogLevel, "log-level", "l", "minimum level of emitted logs (debug|info|warn|error)")
	infoCmd.Flags().BoolVar(&logSettings.WantJSON, "json", false, "output in JSON format (result to stdout, logs to stderr)")

	return infoCmd
}

type Program struct {
	CreationService     *create.Service
	VerificationService *verify.Service
	RepairService       *repair.Service
	InfoService         *info.Service

	log *logging.Logger
}

func NewProgram(fsys afero.Fs, ls logging.Options, runner schema.CommandRunner) *Program {
	log := logging.NewLogger(ls)

	return &Program{
		CreationService:     create.NewService(fsys, log, runner),
		VerificationService: verify.NewService(fsys, log, runner),
		RepairService:       repair.NewService(fsys, log, runner),
		InfoService:         info.NewService(fsys, log, runner),

		log: log,
	}
}

func logOperationResult(err error, result *util.ResultTracker, log *slog.Logger) {
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
			"error", err,
			"successCount", result.Success,
			"skipCount", result.Skipped,
			"errorCount", result.Error,
			"processedCount", processedCount,
			"selectedCount", result.Selected,
		)
	}
}

func main() {
	var exitCode int
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "panic: %v\n\n", r)
			debug.PrintStack()
			exitCode = schema.ExitCodeUnclassified
		}
		os.Exit(exitCode)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
	}()

	rootCmd := newRootCmd(ctx)
	err := rootCmd.Execute()
	exitCode = schema.ExitCodeFor(err)
}
