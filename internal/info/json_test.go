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

// Expectation: No specified run interval should return an error.
func Test_Service_PrintJSON_NoRunInterval_Error(t *testing.T) {
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
	err := prog.PrintJSON(t.Context(), "/data", args)

	require.ErrorIs(t, err, schema.ErrExitBadInvocation)
	require.ErrorIs(t, err, errNoCalcInterval)
}

// Expectation: No duration data should return a warning, and the summary.
func Test_Service_PrintJSON_NoKnownDurations_Success(t *testing.T) {
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
	require.NoError(t, prog.PrintJSON(t.Context(), "/data", args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotZero(t, result.Time)
	require.NotNil(t, result.Summary)
	require.NotNil(t, result.Options)

	require.Contains(t, result.Summary.Warning, "No duration data available")

	require.Nil(t, result.AgeInfo)
	require.Nil(t, result.DurationInfo)
	require.Nil(t, result.BacklogInfo)
	require.Nil(t, result.CycleInfo)
}

// Expectation: The JSON output should be valid and decode back to the Result struct.
func Test_Service_PrintJSON_WithOptions_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
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
	require.NoError(t, prog.PrintJSON(t.Context(), "/data", args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotZero(t, result.Time)
	require.NotNil(t, result.Summary)
	require.NotNil(t, result.Options)
}

// Expectation: The JSON output should be valid and decode back to the Result struct.
func Test_Service_PrintJSON_WithJobs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
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
	require.NoError(t, prog.PrintJSON(t.Context(), "/data", args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotZero(t, result.Time)
	require.NotNil(t, result.Summary)
	require.Equal(t, 1, result.Summary.JobCount)
	require.Equal(t, 1, result.Summary.KnownCount)
	require.Equal(t, 5*time.Minute, result.Summary.TotalDuration)
	require.NotNil(t, result.Summary.LastVerification)
	require.NotZero(t, *result.Summary.LastVerification)
}

// Expectation: The JSON output should be valid and decode back to the Result struct.
func Test_Service_PrintJSON_WithJobs_ZeroLastVerified_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
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
	require.NoError(t, prog.PrintJSON(t.Context(), "/data", args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotZero(t, result.Time)
	require.NotNil(t, result.Summary)
	require.Equal(t, 1, result.Summary.JobCount)
	require.Equal(t, 1, result.Summary.KnownCount)
	require.Equal(t, 5*time.Minute, result.Summary.TotalDuration)
	require.Nil(t, result.Summary.LastVerification)
}

// Expectation: The JSON output should include AgeInfo when --age is set.
func Test_Service_PrintJSON_WithAgeFlag_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
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
	require.NoError(t, prog.PrintJSON(t.Context(), "/data", args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotNil(t, result.AgeInfo)
	require.Equal(t, 7*24*time.Hour, result.Options.MinAge.Value)
	require.Equal(t, 24*time.Hour, result.Options.RunInterval.Value)
	require.Equal(t, 7, result.AgeInfo.RunsPerCycle)
}

// Expectation: The JSON output should include DurationInfo when --duration is set.
func Test_Service_PrintJSON_WithDurationFlag_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
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
	require.NoError(t, prog.PrintJSON(t.Context(), "/data", args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotNil(t, result.DurationInfo)
	require.Equal(t, 10*time.Minute, result.Options.MaxDuration.Value)
	require.Equal(t, 24*time.Hour, result.Options.RunInterval.Value)
	require.True(t, result.DurationInfo.CompleteInOneRun)
}

// Expectation: The JSON output should include BacklogInfo when both --age and --duration are set.
func Test_Service_PrintJSON_HealthyBacklog_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
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
	require.NoError(t, prog.PrintJSON(t.Context(), "/data", args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotNil(t, result.BacklogInfo)
	require.True(t, result.BacklogInfo.Healthy)
	require.Empty(t, result.BacklogInfo.Warning)
	require.Greater(t, result.BacklogInfo.Margin, time.Duration(0))
}

// Expectation: The JSON output should indicate unhealthy backlog with a warning.
func Test_Service_PrintJSON_UnhealthyBacklog_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
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
	require.NoError(t, prog.PrintJSON(t.Context(), "/data", args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotNil(t, result.BacklogInfo)
	require.False(t, result.BacklogInfo.Healthy)
	require.NotEmpty(t, result.BacklogInfo.Warning)
	require.Less(t, result.BacklogInfo.Margin, time.Duration(0))
}

// Expectation: The JSON output should indicate unknown jobs in backlog info.
func Test_Service_PrintJSON_HealthyBacklog_UnknownDurations_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/test2"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
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
	require.NoError(t, prog.PrintJSON(t.Context(), "/data", args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotNil(t, result.BacklogInfo)
	require.Equal(t, 1, result.BacklogInfo.UnknownCount)
}

// Expectation: The JSON output should include a warning when largest job exceeds duration.
func Test_Service_PrintJSON_LargeJobWarning_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
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
	require.NoError(t, prog.PrintJSON(t.Context(), "/data", args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotNil(t, result.DurationInfo)
	require.NotEmpty(t, result.DurationInfo.Warning)
	require.NotEmpty(t, result.DurationInfo.LargestJob)
	require.False(t, result.DurationInfo.CompleteInOneRun)
}

// Expectation: The JSON output should include a warning when age is less than run interval.
func Test_Service_PrintJSON_AgeLessThanInterval_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
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
	_ = args.MinAge.Set("12h")
	require.NoError(t, prog.PrintJSON(t.Context(), "/data", args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotNil(t, result.AgeInfo)
	require.NotEmpty(t, result.AgeInfo.Warning)
}

// Expectation: The JSON output should include CycleInfo with correct verification progress.
func Test_Service_PrintJSON_CycleInfo_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
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
	require.NoError(t, prog.PrintJSON(t.Context(), "/data", args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotNil(t, result.CycleInfo)
	require.Equal(t, 1, result.CycleInfo.VerifiedCount)
	require.Equal(t, 1, result.CycleInfo.TotalCount)
	require.InDelta(t, float64(100), result.CycleInfo.VerifiedPct, 0.01)
	require.Equal(t, 5*time.Minute, result.CycleInfo.VerifiedDuration)
	require.InDelta(t, float64(100), result.CycleInfo.DurationCoveredPct, 0.01)
}

// Expectation: The JSON output should include unknown count in CycleInfo when applicable.
func Test_Service_PrintJSON_CycleInfo_UnknownDurations_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/test2"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
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
	require.NoError(t, prog.PrintJSON(t.Context(), "/data", args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotNil(t, result.CycleInfo)
	require.Equal(t, 1, result.CycleInfo.UnknownCount)
	require.Contains(t, result.CycleInfo.Warning, "excludes 1 unknown duration jobs")
}

// Expectation: A cancellation should be respected and the correct error returned.
func Test_Service_PrintJSON_CtxCancel_Error(t *testing.T) {
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
	err := prog.PrintJSON(ctx, "/data", args)

	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: The JSON output should include all sections when all flags are set.
func Test_Service_PrintJSON_AllSections_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
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
	require.NoError(t, prog.PrintJSON(t.Context(), "/data", args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotZero(t, result.Time)
	require.NotNil(t, result.Summary)
	require.NotNil(t, result.AgeInfo)
	require.NotNil(t, result.DurationInfo)
	require.NotNil(t, result.BacklogInfo)
	require.NotNil(t, result.CycleInfo)
	require.Empty(t, result.Warning)
}

// Expectation: The JSON output should omit optional sections when flags are not set.
func Test_Service_PrintJSON_MinimalSections_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
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
	require.NoError(t, prog.PrintJSON(t.Context(), "/data", args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotZero(t, result.Time)
	require.NotNil(t, result.Summary)
	require.Nil(t, result.AgeInfo)
	require.Nil(t, result.DurationInfo)
	require.Nil(t, result.BacklogInfo)
	require.Nil(t, result.CycleInfo)
}

// Expectation: The AgeInfo should be built with correct values.
func Test_Service_buildAgeInfo_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
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

	info := prog.buildAgeInfo(js, args)

	require.Equal(t, 7, info.RunsPerCycle)
	require.Empty(t, info.Warning)
}

// Expectation: The AgeInfo should include a warning when age is less than run interval.
func Test_Service_buildAgeInfo_AgeLessThanInterval_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
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

	info := prog.buildAgeInfo(js, args)

	require.Equal(t, 1, info.RunsPerCycle)
	require.NotEmpty(t, info.Warning)
}

// Expectation: The AgeInfo should enforce minimum required duration of 1 second.
func Test_Service_buildAgeInfo_MinRequiredDuration_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := verify.Stats{
		TotalDuration: 1 * time.Millisecond,
		KnownCount:    1,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")

	info := prog.buildAgeInfo(js, args)

	require.Equal(t, 1*time.Second, info.MinDuration)
}

// Expectation: The AgeInfo should calculate correct required duration.
func Test_Service_buildAgeInfo_RequiredDuration_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
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

	info := prog.buildAgeInfo(js, args)

	require.Equal(t, 7, info.RunsPerCycle)
	require.Equal(t, 1*time.Hour, info.MinDuration)
}

// Expectation: The DurationInfo should be built with correct values for single run completion.
func Test_Service_buildDurationInfo_SingleRun_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
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

	info := prog.buildDurationInfo(js, args)

	require.Equal(t, 1, info.RunsNeeded)
	require.True(t, info.CompleteInOneRun)
	require.Zero(t, info.FullCycleEvery)
	require.Empty(t, info.Warning)
}

// Expectation: The DurationInfo should be built with correct values for multiple run completion.
func Test_Service_buildDurationInfo_MultipleRuns_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
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

	info := prog.buildDurationInfo(js, args)

	require.Equal(t, 3, info.RunsNeeded)
	require.False(t, info.CompleteInOneRun)
	require.Equal(t, 3*24*time.Hour, info.FullCycleEvery)
	require.Empty(t, info.Warning)
}

// Expectation: The DurationInfo should include a warning when largest job exceeds duration.
func Test_Service_buildDurationInfo_LargeJobWarning_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
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

	info := prog.buildDurationInfo(js, args)

	require.NotEmpty(t, info.Warning)
	require.Equal(t, "large.par2", info.LargestJob)
}

// Expectation: The DurationInfo should not have warning when largest job fits.
func Test_Service_buildDurationInfo_NoWarning_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
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

	info := prog.buildDurationInfo(js, args)

	require.Empty(t, info.Warning)
	require.Empty(t, info.LargestJob)
}

// Expectation: The BacklogInfo should be built with correct values for healthy backlog.
func Test_Service_buildBacklogInfo_Healthy_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
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

	info := prog.buildBacklogInfo(js, args)

	require.Equal(t, 7*time.Hour, info.Capacity)
	require.Equal(t, 1*time.Hour, info.MinRequired)
	require.Equal(t, 6*time.Hour, info.Margin)
	require.True(t, info.Healthy)
	require.Empty(t, info.Warning)
}

// Expectation: The BacklogInfo should be built with correct values for unhealthy backlog.
func Test_Service_buildBacklogInfo_Unhealthy_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
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

	info := prog.buildBacklogInfo(js, args)

	require.Equal(t, 7*time.Hour, info.Capacity)
	require.Equal(t, 10*time.Hour, info.MinRequired)
	require.Equal(t, -3*time.Hour, info.Margin)
	require.False(t, info.Healthy)
	require.NotEmpty(t, info.Warning)
}

// Expectation: The BacklogInfo should indicate unknown jobs when present.
func Test_Service_buildBacklogInfo_UnknownJobs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
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

	info := prog.buildBacklogInfo(js, args)

	require.Equal(t, 2, info.UnknownCount)
}

// Expectation: The BacklogInfo should not have warning when healthy.
func Test_Service_buildBacklogInfo_NoWarningWhenHealthy_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := verify.Stats{
		TotalDuration: 1 * time.Hour,
		KnownCount:    1,
		UnknownCount:  0,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")

	info := prog.buildBacklogInfo(js, args)

	require.True(t, info.Healthy)
	require.Empty(t, info.Warning)
	require.Zero(t, info.UnknownCount)
}

// Expectation: The CycleInfo should be built with correct values.
func Test_Service_buildCycleInfo_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	now := time.Now()

	manifest := schema.NewManifest(context.Background(), "test"+schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     now.Add(-1 * time.Hour),
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

	info := prog.buildCycleInfo(js, jobs, args, now)

	require.Equal(t, 1, info.VerifiedCount)
	require.Equal(t, 1, info.TotalCount)
	require.InDelta(t, float64(100), info.VerifiedPct, 0.01)
	require.Equal(t, 5*time.Minute, info.VerifiedDuration)
	require.Equal(t, 5*time.Minute, info.TotalDuration)
	require.InDelta(t, float64(100), info.DurationCoveredPct, 0.01)
	require.Zero(t, info.UnknownCount)
}

// Expectation: The CycleInfo should exclude jobs verified outside the window.
func Test_Service_buildCycleInfo_OutsideWindow_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	now := time.Now()

	manifest := schema.NewManifest(context.Background(), "test"+schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     now.Add(-8 * 24 * time.Hour),
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

	info := prog.buildCycleInfo(js, jobs, args, now)

	require.Equal(t, 0, info.VerifiedCount)
	require.Equal(t, 1, info.TotalCount)
	require.InDelta(t, float64(0), info.VerifiedPct, 0.01)
	require.Zero(t, info.VerifiedDuration)
}

// Expectation: The CycleInfo should include unknown count when present.
func Test_Service_buildCycleInfo_UnknownJobs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	now := time.Now()

	manifest := schema.NewManifest(context.Background(), "test"+schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     now.Add(-1 * time.Hour),
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

	info := prog.buildCycleInfo(js, jobs, args, now)

	require.Equal(t, 1, info.UnknownCount)
}

// Expectation: The CycleInfo should handle jobs without verification data.
func Test_Service_buildCycleInfo_NoVerification_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	now := time.Now()

	manifest := schema.NewManifest(context.Background(), "test"+schema.Par2Extension)

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

	info := prog.buildCycleInfo(js, jobs, args, now)

	require.Equal(t, 0, info.VerifiedCount)
	require.Equal(t, 1, info.TotalCount)
}

// Expectation: The CycleInfo should handle nil manifests.
func Test_Service_buildCycleInfo_NilManifest_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	now := time.Now()

	jobs := []*verify.Job{
		verify.NewJob("/data/test"+schema.Par2Extension, verify.Options{}, nil),
	}

	js := verify.Stats{
		JobCount:      1,
		KnownCount:    1,
		TotalDuration: 5 * time.Minute,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")

	info := prog.buildCycleInfo(js, jobs, args, now)

	require.Equal(t, 0, info.VerifiedCount)
	require.Equal(t, 1, info.TotalCount)
}

// Expectation: The CycleInfo should handle multiple jobs with mixed verification states.
func Test_Service_buildCycleInfo_MixedJobs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	now := time.Now()

	manifest1 := schema.NewManifest(context.Background(), "test1"+schema.Par2Extension)
	manifest1.Verification = &schema.VerificationManifest{
		Time:     now.Add(-1 * time.Hour),
		Duration: 5 * time.Minute,
	}

	manifest2 := schema.NewManifest(context.Background(), "test2"+schema.Par2Extension)
	manifest2.Verification = &schema.VerificationManifest{
		Time:     now.Add(-10 * 24 * time.Hour),
		Duration: 10 * time.Minute,
	}

	manifest3 := schema.NewManifest(context.Background(), "test3"+schema.Par2Extension)

	jobs := []*verify.Job{
		verify.NewJob("/data/test1"+schema.Par2Extension, verify.Options{}, manifest1),
		verify.NewJob("/data/test2"+schema.Par2Extension, verify.Options{}, manifest2),
		verify.NewJob("/data/test3"+schema.Par2Extension, verify.Options{}, manifest3),
		verify.NewJob("/data/test4"+schema.Par2Extension, verify.Options{}, nil),
	}

	js := verify.Stats{
		JobCount:      4,
		KnownCount:    3,
		UnknownCount:  1,
		TotalDuration: 20 * time.Minute,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")

	info := prog.buildCycleInfo(js, jobs, args, now)

	require.Equal(t, 1, info.VerifiedCount)
	require.Equal(t, 4, info.TotalCount)
	require.Equal(t, 5*time.Minute, info.VerifiedDuration)
	require.Equal(t, 1, info.UnknownCount)
}
