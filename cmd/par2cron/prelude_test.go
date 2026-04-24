package main

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/desertwitch/par2cron/internal/create"
	"github.com/desertwitch/par2cron/internal/info"
	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/repair"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/verify"
	"github.com/spf13/afero"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func newTestLogging() *logging.Options {
	logs := &logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = logs.LogLevel.Set("info")

	return logs
}

func newTestCreateOptions() *create.Options {
	opts := &create.Options{
		Par2Glob: "*",
	}
	_ = opts.Par2Mode.Set(schema.CreateFolderMode)

	return opts
}

func noVisitFlags(fn func(*pflag.Flag)) {}

// Expectation: An error should be returned when DashAt is 0 (no paths before --).
func Test_runPrelude_DashAtZero_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{},
		DashAt:         0,
		CommandOptions: newTestCreateOptions(),
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "need at least one <dir> path before --")
	require.Nil(t, result)
}

// Expectation: DashAt of -1 should not trigger the DashAt validation error.
func Test_runPrelude_DashAtNegativeOne_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	opts := newTestCreateOptions()

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, opts.Par2Args)
}

// Expectation: DashAt should fail if the first par2 argument is a path.
func Test_runPrelude_DashAtFollowedByPath_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data", "/backup"},
		DashAt:         1,
		CommandOptions: newTestCreateOptions(),
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "does not start with -")
	require.Nil(t, result)
}

// Expectation: DashAt pointing to end of args should produce paths and override Par2Args with empty slice.
func Test_runPrelude_DashAtEndOfArgs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	opts := newTestCreateOptions()
	opts.Par2Args = []string{"a", "b"}

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         1,
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.Len(t, result.ResolvedPaths, 1)
	require.NotNil(t, opts.Par2Args)
	require.Empty(t, opts.Par2Args)
}

// Expectation: When DashAt is set and there's external args, they take precedence.
func Test_runPrelude_HasExternalArgsPrecedence_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	yamlContent := `create:
  args: ["-r20", "-n5"]`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	opts := newTestCreateOptions()

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data", "-r30"},
		DashAt:         1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, []string{"-r30"}, opts.Par2Args)
}

// Expectation: When DashAt is set and there's external args, they take precedence.
func Test_runPrelude_HasExternalArgsPrecedence_VerifyType_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	yamlContent := `verify:
  args: ["-r20", "-n5"]`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	opts := &verify.Options{}

	result, err := runPrelude(&preludeInput[*verify.Options, *configFileVerify]{
		FSys:           fs,
		Args:           []string{"/data", "-r30"},
		DashAt:         1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileVerify { return cfg.Verify },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, []string{"-r30"}, opts.Par2Args)
}

// Expectation: When DashAt is set and there's external args, they take precedence.
func Test_runPrelude_HasExternalArgsPrecedence_RepairType_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	yamlContent := `repair:
  args: ["-r20", "-n5"]`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	opts := &repair.Options{}

	result, err := runPrelude(&preludeInput[*repair.Options, *configFileRepair]{
		FSys:           fs,
		Args:           []string{"/data", "-r30"},
		DashAt:         1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileRepair { return cfg.Repair },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, []string{"-r30"}, opts.Par2Args)
}

// Expectation: Args after DashAt should be split into ExternalArgs.
func Test_runPrelude_ExternalArgsSplit_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	opts := newTestCreateOptions()

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data", "-r15", "-n5"},
		DashAt:         1,
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.Len(t, result.ResolvedPaths, 1)
	require.Equal(t, []string{"/data"}, result.ResolvedPaths)
	require.Equal(t, []string{"-r15", "-n5"}, opts.Par2Args)
}

// Expectation: Multiple paths before DashAt should all be resolved.
func Test_runPrelude_MultiplePathsWithExternalArgs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, fs.MkdirAll("/backup", 0o755))

	opts := newTestCreateOptions()

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data", "/backup", "-B"},
		DashAt:         2,
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.Equal(t, []string{"/data", "/backup"}, result.ResolvedPaths)
	require.Equal(t, []string{"-B"}, opts.Par2Args)
}

// Expectation: Path stat failure after DashAt should not be checked (it is an external arg).
func Test_runPrelude_NoCheckPathAfterDash_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	opts := newTestCreateOptions()

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data", "-nonexistent-flag"},
		DashAt:         1,
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.Len(t, result.ResolvedPaths, 1)
	require.Equal(t, []string{"-nonexistent-flag"}, opts.Par2Args)
}

// Expectation: Empty args with no DashAt should return empty resolved paths.
func Test_runPrelude_EmptyArgs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	opts := newTestCreateOptions()

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{},
		DashAt:         -1,
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result.ResolvedPaths)
	require.Empty(t, result.ResolvedPaths)
	require.Nil(t, opts.Par2Args)
}

// Expectation: Paths should be resolved to absolute and returned in ResolvedPaths.
func Test_runPrelude_PathsResolved_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, fs.MkdirAll("/backup", 0o755))

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data", "/backup"},
		DashAt:         -1,
		CommandOptions: newTestCreateOptions(),
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.Len(t, result.ResolvedPaths, 2)
	require.True(t, filepath.IsAbs(result.ResolvedPaths[0]))
	require.True(t, filepath.IsAbs(result.ResolvedPaths[1]))
}

// Expectation: A single path with no DashAt should return one resolved path and no external args.
func Test_runPrelude_SinglePathNoDash_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	opts := newTestCreateOptions()

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.Len(t, result.ResolvedPaths, 1)
	require.Nil(t, opts.Par2Args)
}

// Expectation: An error should be returned when a path does not exist.
func Test_runPrelude_PathNotExist_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/nonexistent"},
		DashAt:         -1,
		CommandOptions: newTestCreateOptions(),
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to access root directory")
	require.Nil(t, result)
}

// Expectation: An error should be returned when a path is not a directory.
func Test_runPrelude_PathNotDirectory_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/notadir", []byte("content"), 0o644))

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/notadir"},
		DashAt:         -1,
		CommandOptions: newTestCreateOptions(),
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "not a directory")
	require.Nil(t, result)
}

// Expectation: An error on the second path should still fail the whole prelude.
func Test_runPrelude_SecondPathNotExist_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data", "/nonexistent"},
		DashAt:         -1,
		CommandOptions: newTestCreateOptions(),
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to access root directory")
	require.Nil(t, result)
}

// Expectation: No config path should skip config loading entirely.
func Test_runPrelude_NoConfigPath_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	var called bool
	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "",
		CommandOptions: newTestCreateOptions(),
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { called = true; return cfg.Create }, //nolint:nlreturn
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, called)
}

// Expectation: A missing config file should return an error.
func Test_runPrelude_ConfigFileNotFound_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "/nonexistent.yaml",
		CommandOptions: newTestCreateOptions(),
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to parse --config file")
	require.Nil(t, result)
}

// Expectation: An error should be returned when the config file cannot be parsed.
func Test_runPrelude_ConfigParseError_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/bad.yaml", []byte("invalid yaml {]"), 0o644))

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "/bad.yaml",
		CommandOptions: newTestCreateOptions(),
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to parse --config file")
	require.Nil(t, result)
}

// Expectation: Config file that fails validation during parseConfigFile should return error.
func Test_runPrelude_ConfigFileValidationFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	yamlContent := `create:
  glob: "**/*.mp4"
  mode: "recursive"`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: newTestCreateOptions(),
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.Error(t, err)
	require.Nil(t, result)
}

// Expectation: A nil config section should not cause a panic or error.
func Test_runPrelude_NilConfigSection_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte("verify:\n  args: [\"-B\"]"), 0o644))

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: newTestCreateOptions(),
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
}

// Expectation: Config values should be merged into CommandOptions when config section exists.
func Test_runPrelude_ConfigMergesIntoCommandOptions_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	yamlContent := `create:
  args: []
  glob: "*.mp4"
  verify: true
  mode: "file"`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	opts := newTestCreateOptions()
	logs := newTestLogging()

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: opts,
		LogSettings:    logs,
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, opts.Par2Args)
	require.Empty(t, opts.Par2Args)
	require.Equal(t, "*.mp4", opts.Par2Glob)
	require.True(t, opts.Par2Verify)
	require.Equal(t, schema.CreateFileMode, opts.Par2Mode.Value)
}

// Expectation: Config log-level and json settings should be merged into logging options.
func Test_runPrelude_ConfigMergesLogSettings_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	yamlContent := `create:
  log-level: "debug"
  json: true`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	opts := newTestCreateOptions()
	logs := newTestLogging()

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: opts,
		LogSettings:    logs,
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, logs.WantJSON)
	require.Equal(t, slog.LevelDebug, logs.LogLevel.Value)
}

// Expectation: Config with duration should merge the duration value.
func Test_runPrelude_ConfigMergesDuration_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	yamlContent := `create:
  duration: "30m"`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	opts := newTestCreateOptions()
	logs := newTestLogging()

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: opts,
		LogSettings:    logs,
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "30m0s", opts.MaxDuration.Value.String())
}

// Expectation: Config merge with hidden files option should propagate.
func Test_runPrelude_ConfigMergesHiddenOption_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	yamlContent := `create:
  hidden: true`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	opts := newTestCreateOptions()

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, opts.HideFiles)
}

// Expectation: Config that causes Validate to fail should return an error after merge.
func Test_runPrelude_ConfigMergeCausesValidateError_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	yamlContent := `create:
  glob: "**/*.mp4"
  mode: "recursive"`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	opts := newTestCreateOptions()

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.Error(t, err)
	require.ErrorIs(t, err, schema.ErrUnsupportedGlob)
	require.Nil(t, result)
}

// Expectation: Flags reported by VisitFlags should prevent YAML config from overriding them.
func Test_runPrelude_VisitFlagsPreventOverride_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	yamlContent := `create:
  glob: "*.mp4"
  mode: "file"`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	opts := &create.Options{
		Par2Glob: "*.txt",
	}
	_ = opts.Par2Mode.Set(schema.CreateFolderMode)

	fakeVisit := func(fn func(*pflag.Flag)) {
		fn(&pflag.Flag{Name: "glob"})
		fn(&pflag.Flag{Name: "mode"})
	}

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     fakeVisit,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "*.txt", opts.Par2Glob)
	require.Equal(t, schema.CreateFolderMode, opts.Par2Mode.Value)
}

// Expectation: ErrUnsupportedGlob from Validate should be wrapped with a descriptive message.
func Test_runPrelude_ValidateUnsupportedGlob_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	opts := &create.Options{
		Par2Glob: "**/*.mp4",
	}
	_ = opts.Par2Mode.Set(schema.CreateRecursiveMode)

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.Error(t, err)
	require.ErrorIs(t, err, schema.ErrUnsupportedGlob)
	require.ErrorContains(t, err, "cannot use deep glob")
	require.Nil(t, result)
}

// Expectation: A generic Validate error should be propagated without the deep glob message.
func Test_runPrelude_ValidateGenericError_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	opts := &create.Options{
		Par2Glob: "{unclosed",
	}

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.Error(t, err)
	require.NotContains(t, err.Error(), "cannot use deep glob")
	require.Nil(t, result)
}

// Expectation: Passing a config with recursive mode and a shallow glob should pass validation.
func Test_runPrelude_ConfigRecursiveShallowGlob_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	yamlContent := `create:
  glob: "*.mp4"
  mode: "recursive"`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	opts := newTestCreateOptions()

	result, err := runPrelude(&preludeInput[*create.Options, *configFileCreate]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileCreate { return cfg.Create },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "*.mp4", opts.Par2Glob)
	require.Equal(t, schema.CreateRecursiveMode, opts.Par2Mode.Value)
}

// Expectation: runPrelude should work with verify.Options as the generic type.
func Test_runPrelude_VerifyType_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	yamlContent := `verify:
  args: ["-B"]
  include-external: true`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	opts := &verify.Options{
		Par2Args: []string{},
	}
	_ = opts.RunInterval.Set("24h")

	result, err := runPrelude(&preludeInput[*verify.Options, *configFileVerify]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileVerify { return cfg.Verify },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, []string{"-B"}, opts.Par2Args)
	require.True(t, opts.IncludeExternal)
}

// Expectation: Verify with config having calc-run-interval should merge correctly.
func Test_runPrelude_VerifyConfigRunInterval_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	yamlContent := `verify:
  calc-run-interval: "12h"
  age: "7d"`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	opts := &verify.Options{}
	_ = opts.RunInterval.Set("24h")

	result, err := runPrelude(&preludeInput[*verify.Options, *configFileVerify]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileVerify { return cfg.Verify },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "12h0m0s", opts.RunInterval.Value.String())
	require.Equal(t, "168h0m0s", opts.MinAge.Value.String())
}

// Expectation: runPrelude should work with repair.Options as the generic type.
func Test_runPrelude_RepairType_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	yamlContent := `repair:
  args: ["-C"]
  purge-backups: true`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	opts := &repair.Options{
		Par2Args: []string{},
	}

	result, err := runPrelude(&preludeInput[*repair.Options, *configFileRepair]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileRepair { return cfg.Repair },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, []string{"-C"}, opts.Par2Args)
	require.True(t, opts.PurgeBackups)
}

// Expectation: Repair with config having min-tested and attempt-unrepairables should merge correctly.
func Test_runPrelude_RepairConfigOptions_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	yamlContent := `repair:
  min-tested: 5
  attempt-unrepairables: true
  restore-backups: true`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	opts := &repair.Options{}

	result, err := runPrelude(&preludeInput[*repair.Options, *configFileRepair]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: opts,
		LogSettings:    newTestLogging(),
		ExtractSection: func(cfg *configFile) *configFileRepair { return cfg.Repair },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 5, opts.MinTestedCount)
	require.True(t, opts.AttemptUnrepairables)
	require.True(t, opts.RestoreBackups)
}

// Expectation: runPrelude should work with info.Options as the generic type.
func Test_runPrelude_InfoType_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	yamlContent := `info:
  include-external: true
  json: true`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	opts := &info.Options{}
	_ = opts.RunInterval.Set("24h")
	logs := newTestLogging()

	result, err := runPrelude(&preludeInput[*info.Options, *configFileInfo]{
		FSys:           fs,
		Args:           []string{"/data"},
		DashAt:         -1,
		ConfigPath:     "/par2cron.yaml",
		CommandOptions: opts,
		LogSettings:    logs,
		ExtractSection: func(cfg *configFile) *configFileInfo { return cfg.Info },
		VisitFlags:     noVisitFlags,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, opts.IncludeExternal)
	require.True(t, logs.WantJSON)
}

// Expectation: A single valid directory should be resolved to its absolute path.
func Test_resolvePathArgs_SingleDir_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	resolved, err := resolvePathArgs(fs, []string{"/data"})

	require.NoError(t, err)
	require.Len(t, resolved, 1)
	require.True(t, filepath.IsAbs(resolved[0]))
}

// Expectation: Multiple valid directories should all be resolved.
func Test_resolvePathArgs_MultipleDirs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, fs.MkdirAll("/backup", 0o755))
	require.NoError(t, fs.MkdirAll("/archive", 0o755))

	resolved, err := resolvePathArgs(fs, []string{"/data", "/backup", "/archive"})

	require.NoError(t, err)
	require.Len(t, resolved, 3)
	for _, p := range resolved {
		require.True(t, filepath.IsAbs(p))
	}
}

// Expectation: An empty slice should return an empty result without error.
func Test_resolvePathArgs_EmptySlice_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	resolved, err := resolvePathArgs(fs, []string{})

	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.Empty(t, resolved)
}

// Expectation: A nonexistent path should return an error.
func Test_resolvePathArgs_PathNotExist_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	resolved, err := resolvePathArgs(fs, []string{"/nonexistent"})

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to access root directory")
	require.Nil(t, resolved)
}

// Expectation: A path that is a file (not a directory) should return an error.
func Test_resolvePathArgs_NotADirectory_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/notadir", []byte("content"), 0o644))

	resolved, err := resolvePathArgs(fs, []string{"/notadir"})

	require.Error(t, err)
	require.ErrorContains(t, err, "not a directory")
	require.Nil(t, resolved)
}

// Expectation: First path valid, second nonexistent should fail on the second path.
func Test_resolvePathArgs_SecondPathNotExist_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	resolved, err := resolvePathArgs(fs, []string{"/data", "/nonexistent"})

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to access root directory")
	require.Nil(t, resolved)
}

// Expectation: First path valid, second is a file should fail on the second path.
func Test_resolvePathArgs_SecondPathNotDir_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/afile", []byte("content"), 0o644))

	resolved, err := resolvePathArgs(fs, []string{"/data", "/afile"})

	require.Error(t, err)
	require.ErrorContains(t, err, "not a directory")
	require.Nil(t, resolved)
}

// Expectation: Resolved paths should preserve order of input.
func Test_resolvePathArgs_PreservesOrder_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/alpha", 0o755))
	require.NoError(t, fs.MkdirAll("/beta", 0o755))
	require.NoError(t, fs.MkdirAll("/gamma", 0o755))

	resolved, err := resolvePathArgs(fs, []string{"/gamma", "/alpha", "/beta"})

	require.NoError(t, err)
	require.Equal(t, []string{"/gamma", "/alpha", "/beta"}, resolved)
}

// Expectation: A nil slice should return an empty result without error.
func Test_resolvePathArgs_NilSlice_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	resolved, err := resolvePathArgs(fs, nil)

	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.Empty(t, resolved)
}

// Expectation: Nested directories should be resolved correctly.
func Test_resolvePathArgs_NestedDir_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/subdir/deep", 0o755))

	resolved, err := resolvePathArgs(fs, []string{"/data/subdir/deep"})

	require.NoError(t, err)
	require.Len(t, resolved, 1)
	require.Equal(t, "/data/subdir/deep", resolved[0])
}
