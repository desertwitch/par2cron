package info

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/desertwitch/par2cron/internal/verify"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func writeTestManifest(t *testing.T, fs afero.Fs, path string, mf *schema.Manifest) error {
	t.Helper()

	data, err := json.Marshal(mf)
	if err != nil {
		return err
	}

	return afero.WriteFile(fs, path, data, 0o644)
}

// Expectation: The JSON output should be valid and decode back to the Result struct.
func Test_Service_Info_JSON_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 5 * time.Minute,
	}
	require.NoError(t, writeTestManifest(t, fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, manifest))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout:   &logBuf,
		Stdout:   &stdoutBuf,
		Stderr:   io.Discard,
		WantJSON: true,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")
	require.NoError(t, prog.Info(t.Context(), "/data", args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotZero(t, result.Time)
	require.NotNil(t, result.Summary)
	require.Equal(t, 1, result.Summary.JobCount)
	require.Equal(t, 1, result.Summary.KnownCount)
	require.Equal(t, 5*time.Minute, result.Summary.TotalDuration)

	require.NotNil(t, result.AgeInfo)
	require.Equal(t, 7*24*time.Hour, result.Options.MinAge.Value)

	require.NotNil(t, result.DurationInfo)
	require.Equal(t, 1*time.Hour, result.Options.MaxDuration.Value)
	require.True(t, result.DurationInfo.CompleteInOneRun)

	require.NotNil(t, result.BacklogInfo)
	require.True(t, result.BacklogInfo.Healthy)

	require.NotNil(t, result.CycleInfo)
	require.Equal(t, 1, result.CycleInfo.VerifiedCount)
}

// Expectation: No specified run interval should output the usage message.
func Test_Service_Info_NoRunInterval_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{}
	err := prog.Info(t.Context(), "/data", args)

	require.ErrorIs(t, err, schema.ErrExitBadInvocation)
	require.ErrorIs(t, err, errNoCalcInterval)

	require.Contains(t, stdoutBuf.String(), "You need to define how often you run par2cron")
}

// Expectation: No duration data should output a respective warning.
func Test_Service_Info_NoKnownDurations_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	err := prog.Info(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Contains(t, stdoutBuf.String(), "No duration data available")
}

// Expectation: The manifest should be parsed and the correct information be shown.
func Test_Service_Info_WithJobs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 5 * time.Minute,
	}
	require.NoError(t, writeTestManifest(t, fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, manifest))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	require.NoError(t, prog.Info(t.Context(), "/data", args))

	output := stdoutBuf.String()
	require.Contains(t, output, "Total jobs found: 1")
	require.Contains(t, output, "Total verification time")
}

// Expectation: The manifest should be parsed and the correct information be shown.
func Test_Service_Info_WithAgeFlag_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 5 * time.Minute,
	}
	require.NoError(t, writeTestManifest(t, fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, manifest))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	require.NoError(t, prog.Info(t.Context(), "/data", args))

	output := stdoutBuf.String()
	require.Contains(t, output, "With just --age")
	require.Contains(t, output, "Runs per verification cycle")
}

// Expectation: The manifest should be parsed and the correct information be shown.
func Test_Service_Info_WithDurationFlag_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 5 * time.Minute,
	}
	require.NoError(t, writeTestManifest(t, fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, manifest))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MaxDuration.Set("10m")
	require.NoError(t, prog.Info(t.Context(), "/data", args))

	output := stdoutBuf.String()
	require.Contains(t, output, "With just --duration")
}

// Expectation: The manifest should be parsed and the correct information be shown.
func Test_Service_Info_HealthyBacklog_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 5 * time.Minute,
	}
	require.NoError(t, writeTestManifest(t, fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, manifest))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")
	require.NoError(t, prog.Info(t.Context(), "/data", args))

	output := stdoutBuf.String()
	require.Contains(t, output, "HEALTHY")
}

// Expectation: The manifest should be parsed and the correct information be shown.
func Test_Service_Info_HealthyBacklog_UnknownDurations_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/test2"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 5 * time.Minute,
	}
	require.NoError(t, writeTestManifest(t, fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, manifest))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{IncludeExternal: true}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")
	require.NoError(t, prog.Info(t.Context(), "/data", args))

	output := stdoutBuf.String()
	require.Contains(t, output, "HEALTHY (based on known durations)")
}

// Expectation: The manifest should be parsed and the correct information be shown.
func Test_Service_Info_UnhealthyBacklog_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 2 * time.Hour,
	}
	require.NoError(t, writeTestManifest(t, fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, manifest))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("10m")
	require.NoError(t, prog.Info(t.Context(), "/data", args))

	output := stdoutBuf.String()
	require.Contains(t, output, "UNHEALTHY")
	require.Contains(t, output, "INSANE CONFIGURATION")
}

// Expectation: The manifest should be parsed and the correct information be shown.
func Test_Service_Info_LargeJobWarning_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 2 * time.Hour,
	}
	require.NoError(t, writeTestManifest(t, fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, manifest))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MaxDuration.Set("1h")
	require.NoError(t, prog.Info(t.Context(), "/data", args))

	output := stdoutBuf.String()
	require.Contains(t, output, "Largest job")
	require.Contains(t, output, "exceeds --duration")
}

// Expectation: A cancellation should be respected and the correct error returned.
func Test_Service_Info_CtxCancel_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	err := prog.Info(ctx, "/data", args)

	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: The printAgeInfo should output correct information.
func Test_Service_printAgeInfo_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := verify.Stats{
		TotalDuration: 7 * time.Hour,
		KnownCount:    7,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")

	prog.printAgeInfo(js, args)

	output := stdoutBuf.String()
	require.Contains(t, output, "With just --age")
	require.Contains(t, output, "Runs per verification cycle: 7")
	require.Contains(t, output, "If using --duration, minimum should be:")
}

// Expectation: The printAgeInfo should not output anything when MinAge is zero.
func Test_Service_printAgeInfo_ZeroMinAge_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := verify.Stats{
		TotalDuration: 1 * time.Hour,
		KnownCount:    1,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")

	prog.printAgeInfo(js, args)

	require.Empty(t, stdoutBuf.String())
}

// Expectation: The stats should be parsed and the correct information be shown.
func Test_Service_printAgeInfo_AgeLessThanInterval_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := verify.Stats{
		TotalDuration: 1 * time.Hour,
		KnownCount:    1,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("12h")

	prog.printAgeInfo(js, args)

	output := stdoutBuf.String()
	require.Contains(t, output, "Warning")
	require.Contains(t, output, "is less than")
}

// Expectation: The printDurationInfo should output correct information.
func Test_Service_printDurationInfo_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := verify.Stats{
		TotalDuration:   3 * time.Hour,
		LargestDuration: 30 * time.Minute,
		KnownCount:      6,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MaxDuration.Set("1h")

	prog.printDurationInfo(js, args)

	output := stdoutBuf.String()
	require.Contains(t, output, "With just --duration")
	require.Contains(t, output, "Runs needed to achieve a full verification: 3")
	require.Contains(t, output, "A full verification is eventually achieved every:")
}

// Expectation: The printDurationInfo should indicate single run completion.
func Test_Service_printDurationInfo_SingleRun_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := verify.Stats{
		TotalDuration:   30 * time.Minute,
		LargestDuration: 30 * time.Minute,
		KnownCount:      1,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MaxDuration.Set("1h")

	prog.printDurationInfo(js, args)

	output := stdoutBuf.String()
	require.Contains(t, output, "A full verification is achieved in a single run")
}

// Expectation: The printDurationInfo should not output anything when MaxDuration is zero.
func Test_Service_printDurationInfo_ZeroMaxDuration_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := verify.Stats{
		TotalDuration: 1 * time.Hour,
		KnownCount:    1,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")

	prog.printDurationInfo(js, args)

	require.Empty(t, stdoutBuf.String())
}

// Expectation: The printDurationInfo should show warning for large job.
func Test_Service_printDurationInfo_LargeJobWarning_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := verify.Stats{
		TotalDuration:   2 * time.Hour,
		LargestDuration: 2 * time.Hour,
		LargestJob:      verify.NewJob("/data/large.par2", verify.Options{}, nil),
		KnownCount:      1,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MaxDuration.Set("1h")

	prog.printDurationInfo(js, args)

	output := stdoutBuf.String()
	require.Contains(t, output, "Warning: Largest job")
	require.Contains(t, output, "exceeds --duration")
	require.Contains(t, output, "large.par2")
}

// Expectation: The printBacklogInfo should output correct information for healthy backlog.
func Test_Service_printBacklogInfo_Healthy_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := verify.Stats{
		TotalDuration: 1 * time.Hour,
		KnownCount:    1,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")

	prog.printBacklogInfo(js, args)

	output := stdoutBuf.String()
	require.Contains(t, output, "With --age")
	require.Contains(t, output, "Processing capacity:")
	require.Contains(t, output, "HEALTHY")
}

// Expectation: The printBacklogInfo should show qualified healthy status with unknown durations.
func Test_Service_printBacklogInfo_HealthyWithUnknownDurations_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := verify.Stats{
		TotalDuration: 1 * time.Hour,
		KnownCount:    1,
		UnknownCount:  2,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")

	prog.printBacklogInfo(js, args)

	output := stdoutBuf.String()
	require.Contains(t, output, "HEALTHY (based on known durations)")
}

// Expectation: The printBacklogInfo should output correct information for unhealthy backlog.
func Test_Service_printBacklogInfo_Unhealthy_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := verify.Stats{
		TotalDuration: 10 * time.Hour,
		KnownCount:    10,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")

	prog.printBacklogInfo(js, args)

	output := stdoutBuf.String()
	require.Contains(t, output, "UNHEALTHY")
	require.Contains(t, output, "INSANE CONFIGURATION")
}

// Expectation: The printBacklogInfo should not output anything when MinAge is zero.
func Test_Service_printBacklogInfo_ZeroMinAge_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := verify.Stats{
		TotalDuration: 1 * time.Hour,
		KnownCount:    1,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MaxDuration.Set("1h")

	prog.printBacklogInfo(js, args)

	require.Empty(t, stdoutBuf.String())
}

// Expectation: The printBacklogInfo should not output anything when MaxDuration is zero.
func Test_Service_printBacklogInfo_ZeroMaxDuration_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := verify.Stats{
		TotalDuration: 1 * time.Hour,
		KnownCount:    1,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")

	prog.printBacklogInfo(js, args)

	require.Empty(t, stdoutBuf.String())
}

// Expectation: The manifest should be parsed and the correct information be shown.
func Test_Service_printCycleInfo_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 5 * time.Minute,
	}

	jobs := []*verify.Job{
		verify.NewJob("/data/test"+schema.Par2Extension, verify.Options{}, manifest),
	}

	js := verify.Stats{
		JobCount:      1,
		KnownCount:    1,
		TotalDuration: 5 * time.Minute,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")

	prog.printCycleInfo(js, jobs, args, time.Now())

	output := stdoutBuf.String()
	require.Contains(t, output, "Verification progress")
	require.Contains(t, output, "Jobs verified")
}

// Expectation: The manifest should be parsed and the correct information be shown.
func Test_Service_printCycleInfo_UnknownDurations_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 5 * time.Minute,
	}

	jobs := []*verify.Job{
		verify.NewJob("/data/test"+schema.Par2Extension, verify.Options{}, manifest),
		verify.NewJob("/data/test2"+schema.Par2Extension, verify.Options{}, nil),
	}

	js := verify.Stats{
		JobCount:      2,
		KnownCount:    1,
		UnknownCount:  1,
		TotalDuration: 5 * time.Minute,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")

	prog.printCycleInfo(js, jobs, args, time.Now())

	output := stdoutBuf.String()
	require.Contains(t, output, "Verification progress")
	require.Contains(t, output, "Jobs verified")
	require.Contains(t, output, "which excludes")
}

// Expectation: The printCycleInfo should not output anything when MinAge is zero.
func Test_Service_printCycleInfo_ZeroMinAge_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := verify.Stats{
		JobCount:      1,
		KnownCount:    1,
		TotalDuration: 5 * time.Minute,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")

	prog.printCycleInfo(js, nil, args, time.Now())

	require.Empty(t, stdoutBuf.String())
}

// Expectation: The printCycleInfo should not output anything when TotalDuration is zero.
func Test_Service_printCycleInfo_ZeroTotalDuration_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := verify.Stats{
		JobCount:      1,
		KnownCount:    1,
		TotalDuration: 0,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")

	prog.printCycleInfo(js, nil, args, time.Now())

	require.Empty(t, stdoutBuf.String())
}
