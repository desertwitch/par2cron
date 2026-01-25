package main

import (
	"log/slog"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/flags"
	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/stretchr/testify/require"
)

// Expectation: A new program should be established.
func Test_NewProgram_Success(t *testing.T) {
	t.Parallel()

	ls := logging.Options{
		Logout: &testutil.SafeBuffer{},
		Stdout: &testutil.SafeBuffer{},
		Stderr: &testutil.SafeBuffer{},
	}
	_ = ls.LogLevel.Set("info")

	prog := NewProgram(nil, ls, &testutil.MockRunner{})

	require.NotNil(t, prog)
	require.NotNil(t, prog.CreationService)
	require.NotNil(t, prog.VerificationService)
	require.NotNil(t, prog.RepairService)
	require.NotNil(t, prog.InfoService)
}

// Expectation: The root command should be returned with the subcommands.
func Test_NewRootCmd_Success(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd(t.Context())

	require.NotNil(t, cmd)
	require.Equal(t, "par2cron", cmd.Use)
	require.True(t, cmd.HasSubCommands())
}

// Expectation: The root command should have a "create" subcommand.
func Test_NewRootCmd_HasCreateCommand_Success(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd(t.Context())

	createCmd, _, err := cmd.Find([]string{"create"})

	require.NoError(t, err)
	require.NotNil(t, createCmd)
	require.Equal(t, "create", createCmd.Name())
}

// Expectation: The root command should have a "verify" subcommand.
func Test_NewRootCmd_HasVerifyCommand_Success(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd(t.Context())

	verifyCmd, _, err := cmd.Find([]string{"verify"})

	require.NoError(t, err)
	require.NotNil(t, verifyCmd)
	require.Equal(t, "verify", verifyCmd.Name())
}

// Expectation: The root command should have a "repair" subcommand.
func Test_NewRootCmd_HasRepairCommand_Success(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd(t.Context())

	repairCmd, _, err := cmd.Find([]string{"repair"})

	require.NoError(t, err)
	require.NotNil(t, repairCmd)
	require.Equal(t, "repair", repairCmd.Name())
}

// Expectation: The root command should have a "info" subcommand.
func Test_NewRootCmd_HasInfoCommand_Success(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd(t.Context())

	infoCmd, _, err := cmd.Find([]string{"info"})

	require.NoError(t, err)
	require.NotNil(t, infoCmd)
	require.Equal(t, "info", infoCmd.Name())
}

// Expectation: The root command should have a "check-config" subcommand.
func Test_NewRootCmd_HasCheckConfigCommand_Success(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd(t.Context())

	checkConfigCmd, _, err := cmd.Find([]string{"check-config"})

	require.NoError(t, err)
	require.NotNil(t, checkConfigCmd)
	require.Equal(t, "check-config", checkConfigCmd.Name())
}

// Expectation: The "create" command should have flags.
func Test_NewCreateCmd_DefaultArgs_Success(t *testing.T) {
	t.Parallel()

	cmd := newCreateCmd(t.Context())

	require.NotNil(t, cmd)
	require.Equal(t, "create", cmd.Name())
	require.True(t, cmd.HasFlags())
}

// Expectation: The "create" command should have a "config" flag.
func Test_NewCreateCmd_HasConfigFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newCreateCmd(t.Context())

	flag := cmd.Flags().Lookup("config")

	require.NotNil(t, flag)
	require.Equal(t, "string", flag.Value.Type())
	require.Empty(t, flag.DefValue)
}

// Expectation: The "create" command should have a "log-level" flag.
func Test_NewCreateCmd_HasLogLevelFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newCreateCmd(t.Context())

	flag := cmd.Flags().Lookup("log-level")
	flagval := flag.Value

	require.NotNil(t, flag)
	require.Equal(t, "level", flag.Value.Type())
	require.Equal(t, "info", flag.DefValue)

	logflag, ok := flagval.(*flags.LogLevel)
	require.True(t, ok)
	require.Equal(t, slog.LevelInfo, logflag.Value)
}

// Expectation: The "create" command should have a "mode" flag.
func Test_NewCreateCmd_HasModeFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newCreateCmd(t.Context())

	flag := cmd.Flags().Lookup("mode")
	flagval := flag.Value

	require.NotNil(t, flag)
	require.Equal(t, "mode", flag.Value.Type())
	require.Equal(t, schema.CreateFolderMode, flag.DefValue)

	logflag, ok := flagval.(*flags.CreateMode)
	require.True(t, ok)
	require.Equal(t, schema.CreateFolderMode, logflag.Value)
}

// Expectation: The "create" command should have a "json" flag.
func Test_NewCreateCmd_HasJsonFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newCreateCmd(t.Context())

	flag := cmd.Flags().Lookup("json")

	require.NotNil(t, flag)
	require.Equal(t, "bool", flag.Value.Type())
	require.Equal(t, "false", flag.Value.String())
}

// Expectation: The "create" command should have a "hidden" flag.
func Test_NewCreateCmd_HasHiddenFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newCreateCmd(t.Context())

	flag := cmd.Flags().Lookup("hidden")

	require.NotNil(t, flag)
	require.Equal(t, "bool", flag.Value.Type())
	require.Equal(t, "false", flag.Value.String())
}

// Expectation: The "create" command should have a "verify" flag.
func Test_NewCreateCmd_HasVerifyFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newCreateCmd(t.Context())

	flag := cmd.Flags().Lookup("verify")

	require.NotNil(t, flag)
	require.Equal(t, "bool", flag.Value.Type())
	require.Equal(t, "false", flag.Value.String())
}

// Expectation: The "create" command cannot run with no arguments.
func Test_NewCreateCmd_RequiresArgs_Error(t *testing.T) {
	t.Parallel()

	cmd := newCreateCmd(t.Context())
	cmd.SetArgs([]string{})

	err := cmd.Execute()

	require.Error(t, err)
}

// Expectation: The "create" command should have a "glob" flag.
func Test_NewCreateCmd_HasGlobFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newCreateCmd(t.Context())

	flag := cmd.Flags().Lookup("glob")

	require.NotNil(t, flag)
	require.Equal(t, "string", flag.Value.Type())
	require.Equal(t, "*", flag.Value.String())
}

// Expectation: The "create" command should have a "duration" flag.
func Test_NewCreateCmd_HasDurationFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newCreateCmd(t.Context())

	flag := cmd.Flags().Lookup("duration")
	flagval := flag.Value

	require.NotNil(t, flag)
	require.Equal(t, "duration", flag.Value.Type())

	durflag, ok := flagval.(*flags.Duration)
	require.True(t, ok)
	require.Zero(t, durflag.Value)
}

// Expectation: The "verify" command should have flags.
func Test_NewVerifyCmd_DefaultArgs_Success(t *testing.T) {
	t.Parallel()

	cmd := newVerifyCmd(t.Context())

	require.NotNil(t, cmd)
	require.Equal(t, "verify", cmd.Name())
	require.True(t, cmd.HasFlags())
}

// Expectation: The "verify" command should have a "config" flag.
func Test_NewVerifyCmd_HasConfigFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newVerifyCmd(t.Context())

	flag := cmd.Flags().Lookup("config")

	require.NotNil(t, flag)
	require.Equal(t, "string", flag.Value.Type())
	require.Empty(t, flag.DefValue)
}

// Expectation: The "verify" command should have an "age" flag.
func Test_NewVerifyCmd_HasAgeFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newVerifyCmd(t.Context())

	flag := cmd.Flags().Lookup("age")
	flagval := flag.Value

	require.NotNil(t, flag)
	require.Equal(t, "duration", flag.Value.Type())

	durflag, ok := flagval.(*flags.Duration)
	require.True(t, ok)
	require.Zero(t, durflag.Value)
}

// Expectation: The "verify" command should have a "duration" flag.
func Test_NewVerifyCmd_HasDurationFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newVerifyCmd(t.Context())

	flag := cmd.Flags().Lookup("duration")
	flagval := flag.Value

	require.NotNil(t, flag)
	require.Equal(t, "duration", flag.Value.Type())

	durflag, ok := flagval.(*flags.Duration)
	require.True(t, ok)
	require.Zero(t, durflag.Value)
}

// Expectation: The "verify" command should have a "calc-run-interval" flag.
func Test_NewVerifyCmd_HasCalcRunIntervalFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newVerifyCmd(t.Context())

	flag := cmd.Flags().Lookup("calc-run-interval")
	flagval := flag.Value

	require.NotNil(t, flag)
	require.Equal(t, "duration", flag.Value.Type())
	require.Equal(t, "24h", flag.DefValue)

	durflag, ok := flagval.(*flags.Duration)
	require.True(t, ok)
	require.Equal(t, 24*time.Hour, durflag.Value)
}

// Expectation: The "verify" command should have a "log-level" flag.
func Test_NewVerifyCmd_HasLogLevelFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newVerifyCmd(t.Context())

	flag := cmd.Flags().Lookup("log-level")
	flagval := flag.Value

	require.NotNil(t, flag)
	require.Equal(t, "level", flag.Value.Type())
	require.Equal(t, "info", flag.DefValue)

	logflag, ok := flagval.(*flags.LogLevel)
	require.True(t, ok)
	require.Equal(t, slog.LevelInfo, logflag.Value)
}

// Expectation: The "verify" command should have a "json" flag.
func Test_NewVerifyCmd_HasJsonFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newVerifyCmd(t.Context())

	flag := cmd.Flags().Lookup("json")

	require.NotNil(t, flag)
	require.Equal(t, "bool", flag.Value.Type())
	require.Equal(t, "false", flag.Value.String())
}

// Expectation: The "verify" command should have a "include-external" flag.
func Test_NewVerifyCmd_HasIncludeExternalFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newVerifyCmd(t.Context())

	flag := cmd.Flags().Lookup("include-external")

	require.NotNil(t, flag)
	require.Equal(t, "bool", flag.Value.Type())
	require.Equal(t, "false", flag.Value.String())
}

// Expectation: The "verify" command should have a "skip-not-created" flag.
func Test_NewVerifyCmd_HasSkipNotCreatedFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newVerifyCmd(t.Context())

	flag := cmd.Flags().Lookup("skip-not-created")

	require.NotNil(t, flag)
	require.Equal(t, "bool", flag.Value.Type())
	require.Equal(t, "false", flag.Value.String())
}

// Expectation: The "verify" command cannot run without arguments.
func Test_NewVerifyCmd_RequiresArgs_Error(t *testing.T) {
	t.Parallel()

	cmd := newVerifyCmd(t.Context())
	cmd.SetArgs([]string{})

	err := cmd.Execute()

	require.Error(t, err)
}

// Expectation: The "repair" command should have flags.
func Test_NewRepairCmd_DefaultArgs_Success(t *testing.T) {
	t.Parallel()

	cmd := newRepairCmd(t.Context())

	require.NotNil(t, cmd)
	require.Equal(t, "repair", cmd.Name())
	require.True(t, cmd.HasFlags())
}

// Expectation: The "repair" command should have a "log-level" flag.
func Test_NewRepairCmd_HasLogLevelFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newRepairCmd(t.Context())

	flag := cmd.Flags().Lookup("log-level")
	flagval := flag.Value

	require.NotNil(t, flag)
	require.Equal(t, "level", flag.Value.Type())
	require.Equal(t, "info", flag.DefValue)

	logflag, ok := flagval.(*flags.LogLevel)
	require.True(t, ok)
	require.Equal(t, slog.LevelInfo, logflag.Value)
}

// Expectation: The "repair" command should have a "json" flag.
func Test_NewRepairCmd_HasJsonFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newRepairCmd(t.Context())

	flag := cmd.Flags().Lookup("json")

	require.NotNil(t, flag)
	require.Equal(t, "bool", flag.Value.Type())
	require.Equal(t, "false", flag.Value.String())
}

// Expectation: The "repair" command should have a "duration" flag.
func Test_NewRepairCmd_HasDurationFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newRepairCmd(t.Context())

	flag := cmd.Flags().Lookup("duration")
	flagval := flag.Value

	require.NotNil(t, flag)
	require.Equal(t, "duration", flag.Value.Type())

	durflag, ok := flagval.(*flags.Duration)
	require.True(t, ok)
	require.Zero(t, durflag.Value)
}

// Expectation: The "repair" command should have a "min-tested" flag.
func Test_NewRepairCmd_HasMinTestedFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newRepairCmd(t.Context())

	flag := cmd.Flags().Lookup("min-tested")

	require.NotNil(t, flag)
	require.Equal(t, "int", flag.Value.Type())
	require.Equal(t, "0", flag.DefValue)
}

// Expectation: The "repair" command should have a "attempt-unrepairables" flag.
func Test_NewRepairCmd_HasAttemptUnrepairablesFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newRepairCmd(t.Context())

	flag := cmd.Flags().Lookup("attempt-unrepairables")

	require.NotNil(t, flag)
	require.Equal(t, "bool", flag.Value.Type())
	require.Equal(t, "false", flag.DefValue)
}

// Expectation: The "repair" command should have a "purge-backups" flag.
func Test_NewRepairCmd_HasPurgeBackupsFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newRepairCmd(t.Context())

	flag := cmd.Flags().Lookup("purge-backups")

	require.NotNil(t, flag)
	require.Equal(t, "bool", flag.Value.Type())
	require.Equal(t, "false", flag.DefValue)
}

// Expectation: The "repair" command should have a "verify" flag.
func Test_NewRepairCmd_HasVerifyFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newRepairCmd(t.Context())

	flag := cmd.Flags().Lookup("verify")

	require.NotNil(t, flag)
	require.Equal(t, "bool", flag.Value.Type())
	require.Equal(t, "false", flag.DefValue)
}

// Expectation: The "repair" command should have a "skip-not-created" flag.
func Test_NewRepairCmd_HasSkipNotCreatedFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newRepairCmd(t.Context())

	flag := cmd.Flags().Lookup("skip-not-created")

	require.NotNil(t, flag)
	require.Equal(t, "bool", flag.Value.Type())
	require.Equal(t, "false", flag.Value.String())
}

// Expectation: The "repair" command should have a "config" flag.
func Test_NewRepairCmd_HasConfigFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newRepairCmd(t.Context())

	flag := cmd.Flags().Lookup("config")

	require.NotNil(t, flag)
	require.Equal(t, "string", flag.Value.Type())
	require.Empty(t, flag.DefValue)
}

// Expectation: The "repair" command cannot run without arguments.
func Test_NewRepairCmd_RequiresArgs_Error(t *testing.T) {
	t.Parallel()

	cmd := newRepairCmd(t.Context())
	cmd.SetArgs([]string{})

	err := cmd.Execute()

	require.Error(t, err)
}

// Expectation: The "info" command should have flags.
func Test_NewInfoCmd_DefaultArgs_Success(t *testing.T) {
	t.Parallel()

	cmd := newInfoCmd(t.Context())

	require.NotNil(t, cmd)
	require.Equal(t, "info", cmd.Name())
	require.True(t, cmd.HasFlags())
}

// Expectation: The "info" command should have a "config" flag.
func Test_NewInfoCmd_HasConfigFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newInfoCmd(t.Context())

	flag := cmd.Flags().Lookup("config")

	require.NotNil(t, flag)
	require.Equal(t, "string", flag.Value.Type())
	require.Empty(t, flag.DefValue)
}

// Expectation: The "info" command should have an "age" flag.
func Test_NewInfoCmd_HasAgeFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newInfoCmd(t.Context())

	flag := cmd.Flags().Lookup("age")
	flagval := flag.Value

	require.NotNil(t, flag)
	require.Equal(t, "duration", flag.Value.Type())

	durflag, ok := flagval.(*flags.Duration)
	require.True(t, ok)
	require.Zero(t, durflag.Value)
}

// Expectation: The "info" command should have an "duration" flag.
func Test_NewInfoCmd_HasDurationFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newInfoCmd(t.Context())

	flag := cmd.Flags().Lookup("duration")
	flagval := flag.Value

	require.NotNil(t, flag)
	require.Equal(t, "duration", flag.Value.Type())

	durflag, ok := flagval.(*flags.Duration)
	require.True(t, ok)
	require.Zero(t, durflag.Value)
}

// Expectation: The "info" command should have an "calc-run-interval" flag.
func Test_NewInfoCmd_HasCalcRunIntervalFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newInfoCmd(t.Context())

	flag := cmd.Flags().Lookup("calc-run-interval")
	flagval := flag.Value

	require.NotNil(t, flag)
	require.Equal(t, "duration", flag.Value.Type())
	require.Equal(t, "24h", flag.DefValue)

	durflag, ok := flagval.(*flags.Duration)
	require.True(t, ok)
	require.Equal(t, 24*time.Hour, durflag.Value)
}

// Expectation: The "info" command should have an "log-level" flag.
func Test_NewInfoCmd_HasLogLevelFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newInfoCmd(t.Context())

	flag := cmd.Flags().Lookup("log-level")

	require.NotNil(t, flag)
	require.Equal(t, "level", flag.Value.Type())
	require.Equal(t, "info", flag.DefValue)

	logflag, ok := flag.Value.(*flags.LogLevel)
	require.True(t, ok)
	require.Equal(t, slog.LevelInfo, logflag.Value)
}

// Expectation: The "info" command should have a "include-external" flag.
func Test_NewInfoCmd_HasIncludeExternalFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newInfoCmd(t.Context())

	flag := cmd.Flags().Lookup("include-external")

	require.NotNil(t, flag)
	require.Equal(t, "bool", flag.Value.Type())
	require.Equal(t, "false", flag.Value.String())
}

// Expectation: The "info" command should have a "skip-not-created" flag.
func Test_NewInfoCmd_HasSkipNotCreatedFlag_Success(t *testing.T) {
	t.Parallel()

	cmd := newInfoCmd(t.Context())

	flag := cmd.Flags().Lookup("skip-not-created")

	require.NotNil(t, flag)
	require.Equal(t, "bool", flag.Value.Type())
	require.Equal(t, "false", flag.Value.String())
}

// Expectation: The "info" command cannot run without arguments.
func Test_NewInfoCmd_RequiresExactOneArg_Error(t *testing.T) {
	t.Parallel()

	cmd := newInfoCmd(t.Context())
	cmd.SetArgs([]string{})

	err := cmd.Execute()

	require.Error(t, err)
}

// Expectation: The "info" command cannot run with too many arguments.
func Test_NewInfoCmd_TooManyArgs_Error(t *testing.T) {
	t.Parallel()

	cmd := newInfoCmd(t.Context())
	cmd.SetArgs([]string{"/data", "/extra"})

	err := cmd.Execute()

	require.Error(t, err)
}
