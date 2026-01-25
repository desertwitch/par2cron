package main

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/create"
	"github.com/desertwitch/par2cron/internal/flags"
	"github.com/desertwitch/par2cron/internal/info"
	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/repair"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/desertwitch/par2cron/internal/verify"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: A valid YAML config file should be parsed successfully.
func Test_parseConfigFile_ValidConfig_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	yamlContent := `create:
  args: ["-r15", "-n5"]
  glob: "*.mp4"
  verify: true
  mode: "file"
  log-level: "debug"
  json: true
  duration: "2d"
  hidden: true
verify:
  args: ["-B"]
  duration: "2h"
  age: "7d"
  calc-run-interval: "12h"
  include-external: true
  skip-not-created: true
  log-level: "warn"
  json: false
repair:
  args: ["-C"]
  verify: true
  duration: "2h"
  min-tested: 3
  skip-not-created: true
  attempt-unrepairables: true
  log-level: "warn"
  json: true
  purge-backups: true
info:
  duration: "1h"
  age: "14d"
  calc-run-interval: "6h"
  include-external: true
  skip-not-created: false
  json: true
  log-level: "error"`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	cfg, err := parseConfigFile(fs, "/par2cron.yaml")

	require.NoError(t, err)
	require.NotNil(t, cfg.Create)
	require.NotNil(t, cfg.Verify)
	require.NotNil(t, cfg.Repair)
	require.NotNil(t, cfg.Info)
}

// Expectation: An error should be returned when the file does not exist.
func Test_parseConfigFile_FileNotExist_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	cfg, err := parseConfigFile(fs, "/nonexistent.yaml")

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to read file")
	require.Nil(t, cfg)
}

// Expectation: An error should be returned when the YAML is invalid.
func Test_parseConfigFile_InvalidYAML_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte("invalid yaml {]"), 0o644))

	cfg, err := parseConfigFile(fs, "/par2cron.yaml")

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to decode yaml")
	require.Nil(t, cfg)
}

// Expectation: An error should be returned when an unknown field is present.
func Test_parseConfigFile_UnknownField_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	yamlContent := `create:
  args: ["-r15"]
  unknown_field: "value"`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	cfg, err := parseConfigFile(fs, "/par2cron.yaml")

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to decode yaml")
	require.Nil(t, cfg)
}

// Expectation: A config with only create section should be parsed successfully.
func Test_parseConfigFile_PartialFields_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	yamlContent := `create:
  args: ["-r10", "-n3"]
  glob: "*.txt"`
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte(yamlContent), 0o644))

	cfg, err := parseConfigFile(fs, "/par2cron.yaml")

	require.NoError(t, err)
	require.NotNil(t, cfg.Create)
	require.Nil(t, cfg.Verify)
	require.Nil(t, cfg.Info)
	require.Equal(t, []string{"-r10", "-n3"}, *cfg.Create.Par2Args)
	require.Equal(t, "*.txt", *cfg.Create.Par2Glob)
}

// Expectation: An empty config file should be parsed successfully.
func Test_parseConfigFile_EmptyConfig_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/par2cron.yaml", []byte("{}"), 0o644))

	cfg, err := parseConfigFile(fs, "/par2cron.yaml")

	require.NoError(t, err)
	require.Nil(t, cfg.Create)
	require.Nil(t, cfg.Verify)
	require.Nil(t, cfg.Info)
}

// Expectation: YAML config values should be merged into createArgs.
func Test_configFileCreate_Merge_AllFields_Success(t *testing.T) {
	t.Parallel()

	yamlCfg := &configFileCreate{
		Par2Args:    &[]string{"-r20", "-n5"},
		Par2Glob:    util.Ptr("*.mp4"),
		Par2Verify:  util.Ptr(true),
		Par2Mode:    &flags.CreateMode{Value: schema.CreateFileMode},
		MaxDuration: &flags.Duration{Value: 5 * time.Minute},
		LogLevel:    &flags.LogLevel{},
		WantJSON:    util.Ptr(true),
		HideFiles:   util.Ptr(true),
	}
	_ = yamlCfg.LogLevel.Set("debug")

	cfg := create.Options{
		Par2Args:   []string{"-r10", "-n1"},
		Par2Glob:   "*",
		Par2Verify: false,
	}
	_ = cfg.Par2Mode.Set(schema.CreateFolderMode)

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = logs.LogLevel.Set("info")

	yamlCfg.Merge(&cfg, &logs, false, map[string]bool{})

	require.Equal(t, []string{"-r20", "-n5"}, cfg.Par2Args)
	require.Equal(t, "*.mp4", cfg.Par2Glob)
	require.True(t, cfg.Par2Verify)
	require.Equal(t, schema.CreateFileMode, cfg.Par2Mode.Value)
	require.Equal(t, 5*time.Minute, cfg.MaxDuration.Value)
	require.Equal(t, slog.LevelDebug, logs.LogLevel.Value)
	require.True(t, logs.WantJSON)
	require.True(t, cfg.HideFiles)
}

// Expectation: External args should take precedence over YAML config.
func Test_configFileCreate_Merge_ExternalArgsPrecedence_Success(t *testing.T) {
	t.Parallel()

	yamlCfg := &configFileCreate{
		Par2Args: &[]string{"-r20", "-n5"},
	}

	cfg := create.Options{
		Par2Args: []string{"-r30", "-n10"},
	}

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = logs.LogLevel.Set("info")

	yamlCfg.Merge(&cfg, &logs, true, map[string]bool{})

	require.Equal(t, []string{"-r30", "-n10"}, cfg.Par2Args)
}

// Expectation: CLI flags should take precedence over YAML config.
func Test_configFileCreate_Merge_CLIFlagsPrecedence_Success(t *testing.T) {
	t.Parallel()

	yamlCfg := &configFileCreate{
		Par2Glob:    util.Ptr("*.mp4"),
		Par2Verify:  util.Ptr(true),
		Par2Mode:    &flags.CreateMode{Value: schema.CreateFileMode},
		MaxDuration: &flags.Duration{Value: 5 * time.Minute},
		LogLevel:    &flags.LogLevel{},
		WantJSON:    util.Ptr(true),
		HideFiles:   util.Ptr(true),
	}
	_ = yamlCfg.LogLevel.Set("debug")

	cfg := create.Options{
		Par2Glob:   "*.txt",
		Par2Verify: false,
	}
	_ = cfg.Par2Mode.Set(schema.CreateFolderMode)

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = logs.LogLevel.Set("warn")

	setFlags := map[string]bool{
		"glob":      true,
		"verify":    true,
		"mode":      true,
		"log-level": true,
		"json":      true,
		"duration":  true,
		"hidden":    true,
	}

	yamlCfg.Merge(&cfg, &logs, false, setFlags)

	require.Equal(t, "*.txt", cfg.Par2Glob)
	require.False(t, cfg.Par2Verify)
	require.Equal(t, schema.CreateFolderMode, cfg.Par2Mode.Value)
	require.Equal(t, slog.LevelWarn, logs.LogLevel.Value)
	require.False(t, logs.WantJSON)
	require.Zero(t, cfg.MaxDuration)
	require.False(t, cfg.HideFiles)
}

// Expectation: Nil fields in YAML config should not override existing values.
func Test_configFileCreate_Merge_NilFields_Success(t *testing.T) {
	t.Parallel()

	yamlCfg := &configFileCreate{
		Par2Glob: util.Ptr("*.mp4"),
	}

	cfg := create.Options{
		Par2Args:   []string{"-r10"},
		Par2Glob:   "*",
		Par2Verify: true,
	}
	_ = cfg.Par2Mode.Set(schema.CreateFolderMode)

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = logs.LogLevel.Set("info")

	yamlCfg.Merge(&cfg, &logs, false, map[string]bool{})

	require.Equal(t, []string{"-r10"}, cfg.Par2Args)
	require.Equal(t, "*.mp4", cfg.Par2Glob)
	require.True(t, cfg.Par2Verify)
	require.Equal(t, schema.CreateFolderMode, cfg.Par2Mode.Value)
	require.Equal(t, slog.LevelInfo, logs.LogLevel.Value)
	require.False(t, logs.WantJSON)
}

// Expectation: Args should be cloned, not referenced.
func Test_configFileCreate_Merge_ArgsCloned_Success(t *testing.T) {
	t.Parallel()

	originalArgs := []string{"-r20", "-n5"}
	yamlCfg := &configFileCreate{
		Par2Args: &originalArgs,
	}

	cfg := create.Options{
		Par2Args: []string{"-r10"},
	}

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}

	yamlCfg.Merge(&cfg, &logs, false, map[string]bool{})

	cfg.Par2Args[0] = "-r99"

	require.Equal(t, "-r20", originalArgs[0])
}

// Expectation: YAML config values should be merged into verifyArgs.
func Test_configFileVerify_Merge_AllFields_Success(t *testing.T) {
	t.Parallel()

	maxDur := flags.Duration{}
	_ = maxDur.Set("2h")
	minAge := flags.Duration{}
	_ = minAge.Set("7d")
	RunInterval := flags.Duration{}
	_ = RunInterval.Set("12h")
	LogLevel := flags.LogLevel{}
	_ = LogLevel.Set("debug")

	yamlCfg := &configFileVerify{
		Par2Args:        &[]string{"-B"},
		MaxDuration:     &maxDur,
		MinAge:          &minAge,
		RunInterval:     &RunInterval,
		IncludeExternal: util.Ptr(true),
		SkipNotCreated:  util.Ptr(true),
		LogLevel:        &LogLevel,
		WantJSON:        util.Ptr(true),
	}

	cfg := verify.Options{
		Par2Args: []string{},
	}
	_ = cfg.RunInterval.Set("24h")

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = logs.LogLevel.Set("info")

	yamlCfg.Merge(&cfg, &logs, false, map[string]bool{})

	require.Equal(t, []string{"-B"}, cfg.Par2Args)
	require.Equal(t, "2h0m0s", cfg.MaxDuration.Value.String())
	require.Equal(t, "168h0m0s", cfg.MinAge.Value.String())
	require.Equal(t, "12h0m0s", cfg.RunInterval.Value.String())
	require.True(t, cfg.IncludeExternal)
	require.True(t, cfg.SkipNotCreated)
	require.Equal(t, slog.LevelDebug, logs.LogLevel.Value)
	require.True(t, logs.WantJSON)
}

// Expectation: External args should take precedence over YAML config for verify.
func Test_configFileVerify_Merge_ExternalArgsPrecedence_Success(t *testing.T) {
	t.Parallel()

	yamlCfg := &configFileVerify{
		Par2Args: &[]string{"-B", "-q"},
	}

	cfg := verify.Options{
		Par2Args: []string{"-qq"},
	}

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}

	yamlCfg.Merge(&cfg, &logs, true, map[string]bool{})

	require.Equal(t, []string{"-qq"}, cfg.Par2Args)
}

// Expectation: CLI flags should take precedence over YAML config for verify.
func Test_configFileVerify_Merge_CLIFlagsPrecedence_Success(t *testing.T) {
	t.Parallel()

	maxDur := flags.Duration{}
	_ = maxDur.Set("2h")
	minAge := flags.Duration{}
	_ = minAge.Set("7d")

	yamlCfg := &configFileVerify{
		MaxDuration:     &maxDur,
		MinAge:          &minAge,
		IncludeExternal: util.Ptr(true),
		SkipNotCreated:  util.Ptr(true),
	}

	cfg := verify.Options{}
	_ = cfg.MaxDuration.Set("1h")
	_ = cfg.MinAge.Set("3d")

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}

	setFlags := map[string]bool{
		"duration":         true,
		"age":              true,
		"include-external": true,
		"skip-not-created": true,
	}

	yamlCfg.Merge(&cfg, &logs, false, setFlags)

	require.Equal(t, "1h0m0s", cfg.MaxDuration.Value.String())
	require.Equal(t, "72h0m0s", cfg.MinAge.Value.String())
	require.False(t, cfg.IncludeExternal)
	require.False(t, cfg.SkipNotCreated)
}

// Expectation: Nil fields in YAML config should not override existing values for verify.
func Test_configFileVerify_Merge_NilFields_Success(t *testing.T) {
	t.Parallel()

	maxDur := flags.Duration{}
	_ = maxDur.Set("2h")

	yamlCfg := &configFileVerify{
		MaxDuration: &maxDur,
	}

	cfg := verify.Options{
		Par2Args:        []string{"-q"},
		IncludeExternal: true,
		SkipNotCreated:  true,
	}
	_ = cfg.MinAge.Set("3d")
	_ = cfg.RunInterval.Set("6h")

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = logs.LogLevel.Set("warn")

	yamlCfg.Merge(&cfg, &logs, false, map[string]bool{})

	require.Equal(t, []string{"-q"}, cfg.Par2Args)
	require.Equal(t, "2h0m0s", cfg.MaxDuration.Value.String())
	require.Equal(t, "72h0m0s", cfg.MinAge.Value.String())
	require.Equal(t, "6h0m0s", cfg.RunInterval.Value.String())
	require.True(t, cfg.IncludeExternal)
	require.True(t, cfg.SkipNotCreated)
	require.Equal(t, slog.LevelWarn, logs.LogLevel.Value)
}

// Expectation: Args should be cloned, not referenced for verify.
func Test_configFileVerify_Merge_ArgsCloned_Success(t *testing.T) {
	t.Parallel()

	originalArgs := []string{"-B", "-q"}
	yamlCfg := &configFileVerify{
		Par2Args: &originalArgs,
	}

	cfg := verify.Options{
		Par2Args: []string{},
	}

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}

	yamlCfg.Merge(&cfg, &logs, false, map[string]bool{})

	cfg.Par2Args[0] = "-qq"

	require.Equal(t, "-B", originalArgs[0])
}

// Expectation: YAML config values should be merged into repairArgs.
func Test_configFileRepair_Merge_AllFields_Success(t *testing.T) {
	t.Parallel()

	maxDur := flags.Duration{}
	_ = maxDur.Set("2h")
	LogLevel := flags.LogLevel{}
	_ = LogLevel.Set("debug")

	yamlCfg := &configFileRepair{
		Par2Args:             &[]string{"-B", "-q"},
		MaxDuration:          &maxDur,
		MinTestedCount:       util.Ptr(5),
		SkipNotCreated:       util.Ptr(true),
		LogLevel:             &LogLevel,
		WantJSON:             util.Ptr(true),
		AttemptUnrepairables: util.Ptr(true),
		PurgeBackups:         util.Ptr(true),
		Par2Verify:           util.Ptr(true),
	}

	cfg := repair.Options{
		Par2Args:       []string{},
		MinTestedCount: 0,
		SkipNotCreated: false,
	}
	_ = cfg.MaxDuration.Set("1h")

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = logs.LogLevel.Set("info")

	yamlCfg.Merge(&cfg, &logs, false, map[string]bool{})

	require.Equal(t, []string{"-B", "-q"}, cfg.Par2Args)
	require.Equal(t, "2h0m0s", cfg.MaxDuration.Value.String())
	require.Equal(t, 5, cfg.MinTestedCount)
	require.True(t, cfg.SkipNotCreated)
	require.Equal(t, slog.LevelDebug, logs.LogLevel.Value)
	require.True(t, logs.WantJSON)
	require.True(t, cfg.AttemptUnrepairables)
	require.True(t, cfg.Par2Verify)
	require.True(t, cfg.PurgeBackups)
}

// Expectation: External args should take precedence over YAML config for repair.
func Test_configFileRepair_Merge_ExternalArgsPrecedence_Success(t *testing.T) {
	t.Parallel()

	yamlCfg := &configFileRepair{
		Par2Args: &[]string{"-B", "-q"},
	}

	cfg := repair.Options{
		Par2Args: []string{"-qq"},
	}

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}

	yamlCfg.Merge(&cfg, &logs, true, map[string]bool{})

	require.Equal(t, []string{"-qq"}, cfg.Par2Args)
}

// Expectation: CLI flags should take precedence over YAML config for repair.
func Test_configFileRepair_Merge_CLIFlagsPrecedence_Success(t *testing.T) {
	t.Parallel()

	maxDur := flags.Duration{}
	_ = maxDur.Set("2h")
	LogLevel := flags.LogLevel{}
	_ = LogLevel.Set("debug")

	yamlCfg := &configFileRepair{
		MaxDuration:          &maxDur,
		MinTestedCount:       util.Ptr(10),
		SkipNotCreated:       util.Ptr(true),
		LogLevel:             &LogLevel,
		WantJSON:             util.Ptr(true),
		AttemptUnrepairables: util.Ptr(true),
		PurgeBackups:         util.Ptr(true),
		Par2Verify:           util.Ptr(true),
	}

	cfg := repair.Options{
		MinTestedCount: 3,
		SkipNotCreated: false,
	}
	_ = cfg.MaxDuration.Set("1h")

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = logs.LogLevel.Set("warn")

	setFlags := map[string]bool{
		"verify":                true,
		"duration":              true,
		"min-tested":            true,
		"skip-not-created":      true,
		"log-level":             true,
		"json":                  true,
		"attempt-unrepairables": true,
		"purge-backups":         true,
	}

	yamlCfg.Merge(&cfg, &logs, false, setFlags)

	require.Equal(t, "1h0m0s", cfg.MaxDuration.Value.String())
	require.Equal(t, 3, cfg.MinTestedCount)
	require.False(t, cfg.SkipNotCreated)
	require.Equal(t, slog.LevelWarn, logs.LogLevel.Value)
	require.False(t, logs.WantJSON)
	require.False(t, cfg.AttemptUnrepairables)
	require.False(t, cfg.Par2Verify)
	require.False(t, cfg.PurgeBackups)
}

// Expectation: Nil fields in YAML config should not override existing values for repair.
func Test_configFileRepair_Merge_NilFields_Success(t *testing.T) {
	t.Parallel()

	maxDur := flags.Duration{}
	_ = maxDur.Set("2h")

	yamlCfg := &configFileRepair{
		MaxDuration: &maxDur,
	}

	cfg := repair.Options{
		Par2Args:       []string{"-q"},
		MinTestedCount: 5,
		SkipNotCreated: true,
	}

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = logs.LogLevel.Set("warn")

	yamlCfg.Merge(&cfg, &logs, false, map[string]bool{})

	require.Equal(t, []string{"-q"}, cfg.Par2Args)
	require.Equal(t, "2h0m0s", cfg.MaxDuration.Value.String())
	require.Equal(t, 5, cfg.MinTestedCount)
	require.True(t, cfg.SkipNotCreated)
	require.Equal(t, slog.LevelWarn, logs.LogLevel.Value)
	require.False(t, logs.WantJSON)
}

// Expectation: Args should be cloned, not referenced for repair.
func Test_configFileRepair_Merge_ArgsCloned_Success(t *testing.T) {
	t.Parallel()

	originalArgs := []string{"-B", "-q"}
	yamlCfg := &configFileRepair{
		Par2Args: &originalArgs,
	}

	cfg := repair.Options{
		Par2Args: []string{},
	}

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}

	yamlCfg.Merge(&cfg, &logs, false, map[string]bool{})

	cfg.Par2Args[0] = "-qq"

	require.Equal(t, "-B", originalArgs[0])
}

// Expectation: YAML config values should be merged into infoArgs.
func Test_configFileInfo_Merge_AllFields_Success(t *testing.T) {
	t.Parallel()

	maxDur := flags.Duration{}
	_ = maxDur.Set("1h")
	minAge := flags.Duration{}
	_ = minAge.Set("14d")
	RunInterval := flags.Duration{}
	_ = RunInterval.Set("6h")
	LogLevel := flags.LogLevel{}
	_ = LogLevel.Set("error")

	yamlCfg := &configFileInfo{
		MaxDuration:     &maxDur,
		MinAge:          &minAge,
		RunInterval:     &RunInterval,
		LogLevel:        &LogLevel,
		IncludeExternal: util.Ptr(true),
		SkipNotCreated:  util.Ptr(true),
		WantJSON:        util.Ptr(true),
	}

	cfg := info.Options{}
	_ = cfg.RunInterval.Set("24h")

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = logs.LogLevel.Set("info")

	yamlCfg.Merge(&cfg, &logs, false, map[string]bool{})

	require.Equal(t, "1h0m0s", cfg.MaxDuration.Value.String())
	require.Equal(t, "336h0m0s", cfg.MinAge.Value.String())
	require.Equal(t, "6h0m0s", cfg.RunInterval.Value.String())
	require.Equal(t, slog.LevelError, logs.LogLevel.Value)
	require.True(t, cfg.IncludeExternal)
	require.True(t, cfg.SkipNotCreated)
	require.True(t, logs.WantJSON)
}

// Expectation: CLI flags should take precedence over YAML config for info.
func Test_configFileInfo_Merge_CLIFlagsPrecedence_Success(t *testing.T) {
	t.Parallel()

	maxDur := flags.Duration{}
	_ = maxDur.Set("2h")
	minAge := flags.Duration{}
	_ = minAge.Set("7d")
	LogLevel := flags.LogLevel{}
	_ = LogLevel.Set("debug")

	yamlCfg := &configFileInfo{
		MaxDuration:     &maxDur,
		MinAge:          &minAge,
		LogLevel:        &LogLevel,
		IncludeExternal: util.Ptr(true),
		SkipNotCreated:  util.Ptr(true),
		WantJSON:        util.Ptr(true),
	}

	cfg := info.Options{}
	_ = cfg.MaxDuration.Set("1h")
	_ = cfg.MinAge.Set("3d")

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = logs.LogLevel.Set("warn")

	setFlags := map[string]bool{
		"duration":         true,
		"age":              true,
		"log-level":        true,
		"include-external": true,
		"skip-not-created": true,
		"json":             true,
	}

	yamlCfg.Merge(&cfg, &logs, false, setFlags)

	require.Equal(t, "1h0m0s", cfg.MaxDuration.Value.String())
	require.Equal(t, "72h0m0s", cfg.MinAge.Value.String())
	require.Equal(t, slog.LevelWarn, logs.LogLevel.Value)
	require.False(t, cfg.IncludeExternal)
	require.False(t, cfg.SkipNotCreated)
	require.False(t, logs.WantJSON)
}

// Expectation: Nil fields in YAML config should not override existing values for info.
func Test_configFileInfo_Merge_NilFields_Success(t *testing.T) {
	t.Parallel()

	maxDur := flags.Duration{}
	_ = maxDur.Set("2h")

	yamlCfg := &configFileInfo{
		MaxDuration: &maxDur,
	}

	cfg := info.Options{}
	_ = cfg.MinAge.Set("3d")
	_ = cfg.RunInterval.Set("6h")

	logs := logging.Options{
		Logout: io.Discard,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = logs.LogLevel.Set("warn")

	yamlCfg.Merge(&cfg, &logs, false, map[string]bool{})

	require.Equal(t, "2h0m0s", cfg.MaxDuration.Value.String())
	require.Equal(t, "72h0m0s", cfg.MinAge.Value.String())
	require.Equal(t, "6h0m0s", cfg.RunInterval.Value.String())
	require.Equal(t, slog.LevelWarn, logs.LogLevel.Value)
}
