package info

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/desertwitch/par2cron/internal/util"
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

// Expectation: openCache should not attempt to load when CacheDir is empty.
func Test_Service_openCache_NoCacheDir_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	var loadCalled bool
	cacher := &testutil.MockCacheHandler{
		NewCacheFunc: func(fsys afero.Fs, cacheDir string, cacheName string) schema.Cache {
			return &testutil.MockCache{
				LoadFunc: func() error {
					loadCalled = true

					return nil
				},
			}
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, cacher)

	opts := Options{CacheDir: ""}
	prog.openCache("/data", opts)

	require.False(t, loadCalled)
}

// Expectation: openCache should attempt to load when CacheDir is set.
func Test_Service_openCache_WithCacheDir_LoadsCacheFromDisk_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	var loadCalled bool
	cacher := &testutil.MockCacheHandler{
		NewCacheFunc: func(fsys afero.Fs, cacheDir string, cacheName string) schema.Cache {
			return &testutil.MockCache{
				LoadFunc: func() error {
					loadCalled = true

					return nil
				},
			}
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, cacher)

	opts := Options{CacheDir: "/cache"}
	prog.openCache("/data", opts)

	require.True(t, loadCalled)
}

// Expectation: openCache should log an error when cache loading fails with a non-ErrNotExist error.
func Test_Service_openCache_LoadError_LogsError_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: io.Discard,
		Stdout: &logBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("error")

	cacher := &testutil.MockCacheHandler{
		NewCacheFunc: func(fsys afero.Fs, cacheDir string, cacheName string) schema.Cache {
			return &testutil.MockCache{
				LoadFunc: func() error {
					return errors.New("disk I/O error")
				},
			}
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, cacher)

	opts := Options{CacheDir: "/cache"}
	cache := prog.openCache("/data", opts)

	require.NotNil(t, cache)
	require.Contains(t, logBuf.String(), "could not be loaded")
}

// Expectation: openCache should silently ignore ErrNotExist when loading the cache.
func Test_Service_openCache_LoadErrNotExist_NoLog_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: io.Discard,
		Stdout: &logBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("error")

	cacher := &testutil.MockCacheHandler{
		NewCacheFunc: func(fsys afero.Fs, cacheDir string, cacheName string) schema.Cache {
			return &testutil.MockCache{
				LoadFunc: func() error {
					return fs.ErrNotExist
				},
			}
		},
	}

	prog := NewService(fsys, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, cacher)

	opts := Options{CacheDir: "/cache"}
	cache := prog.openCache("/data", opts)

	require.NotNil(t, cache)
	require.NotContains(t, logBuf.String(), "could not be loaded")
}

// Expectation: The filesystem should be walked for jobs.
func Test_Service_Info_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout:   &logBuf,
		Stdout:   &stdoutBuf,
		Stderr:   io.Discard,
		WantJSON: false,
	}
	_ = ls.LogLevel.Set("info")

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")

	prog := NewService(fsys, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})
	require.NoError(t, prog.Info(t.Context(), []string{"/data"}, args))

	require.Contains(t, stdoutBuf.String(), "Scanning filesystem '/data' for jobs")
}

// Expectation: The program should handle multiple provided root directories.
func Test_Service_Info_MultiRoot_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout:   &logBuf,
		Stdout:   &stdoutBuf,
		Stderr:   io.Discard,
		WantJSON: false,
	}
	_ = ls.LogLevel.Set("info")

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")

	prog := NewService(fsys, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})
	require.NoError(t, prog.Info(t.Context(), []string{"/data", "/data2"}, args))

	require.Contains(t, stdoutBuf.String(), "Scanning filesystem '/data' for jobs")
	require.Contains(t, stdoutBuf.String(), "Scanning filesystem '/data2' for jobs")
}

// Expectation: Info should dispatch to the Prometheus exporter and write only metrics to standard output.
func Test_Service_Info_Prometheus_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{Prometheus: true}
	_ = args.RunInterval.Set("24h")
	require.NoError(t, prog.Info(t.Context(), []string{"/data"}, args))

	out := string(stdoutBuf.Bytes())
	checkExposition(t, out)

	require.NotContains(t, out, "Scanning filesystem")
}

// Expectation: Info should dispatch to Prometheus before JSON, so the combination is rejected.
func Test_Service_Info_PrometheusWithJSON_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout:   &logBuf,
		Stdout:   &stdoutBuf,
		Stderr:   io.Discard,
		WantJSON: true,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{Prometheus: true}
	_ = args.RunInterval.Set("24h")
	err := prog.Info(t.Context(), []string{"/data"}, args)

	require.ErrorIs(t, err, schema.ErrExitBadInvocation)
	require.ErrorIs(t, err, errPrometheusWithJSON)
	require.Empty(t, stdoutBuf.Bytes())
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")
	require.NoError(t, prog.Info(t.Context(), []string{"/data"}, args))

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

	require.NotNil(t, result.OverdueInfo)
	require.Zero(t, result.OverdueInfo.OverdueRunCount)
	require.Zero(t, result.OverdueInfo.OverdueCycleCount)
}

// Expectation: The program should handle multiple provided root directories.
func Test_Service_Info_JSON_MultiRoot_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, fs.MkdirAll("/data2", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data2/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 5 * time.Minute,
	}
	require.NoError(t, writeTestManifest(t, fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, manifest))

	manifest = schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 5 * time.Minute,
	}
	require.NoError(t, writeTestManifest(t, fs, "/data2/test"+schema.Par2Extension+schema.ManifestExtension, manifest))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout:   &logBuf,
		Stdout:   &stdoutBuf,
		Stderr:   io.Discard,
		WantJSON: true,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")
	require.NoError(t, prog.Info(t.Context(), []string{"/data", "/data2"}, args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.Len(t, result.Roots, 2)
	require.NotZero(t, result.Time)
	require.NotNil(t, result.Summary)
	require.Equal(t, 2, result.Summary.JobCount)
	require.Equal(t, 2, result.Summary.KnownCount)
	require.Equal(t, 10*time.Minute, result.Summary.TotalDuration)

	require.NotNil(t, result.AgeInfo)
	require.Equal(t, 7*24*time.Hour, result.Options.MinAge.Value)

	require.NotNil(t, result.DurationInfo)
	require.Equal(t, 1*time.Hour, result.Options.MaxDuration.Value)
	require.True(t, result.DurationInfo.CompleteInOneRun)

	require.NotNil(t, result.BacklogInfo)
	require.True(t, result.BacklogInfo.Healthy)

	require.NotNil(t, result.CycleInfo)
	require.Equal(t, 2, result.CycleInfo.VerifiedCount)
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{}
	err := prog.Info(t.Context(), []string{"/data"}, args)

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	err := prog.Info(t.Context(), []string{"/data"}, args)

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	require.NoError(t, prog.Info(t.Context(), []string{"/data"}, args))

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	require.NoError(t, prog.Info(t.Context(), []string{"/data"}, args))

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MaxDuration.Set("10m")
	require.NoError(t, prog.Info(t.Context(), []string{"/data"}, args))

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")
	require.NoError(t, prog.Info(t.Context(), []string{"/data"}, args))

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{IncludeExternal: true}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")
	require.NoError(t, prog.Info(t.Context(), []string{"/data"}, args))

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("10m")
	require.NoError(t, prog.Info(t.Context(), []string{"/data"}, args))

	output := stdoutBuf.String()
	require.Contains(t, output, "UNHEALTHY")
	require.Contains(t, output, "continue to grow")
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MaxDuration.Set("1h")
	require.NoError(t, prog.Info(t.Context(), []string{"/data"}, args))

	output := stdoutBuf.String()
	require.Contains(t, output, "Largest job")
	require.Contains(t, output, "exceeds the given --duration")
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	err := prog.Info(ctx, []string{"/data"}, args)

	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: Info should not load the cache when CacheDir is empty.
func Test_Service_Info_WithoutCacheDir_NotLoadsCache_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/data", 0o755))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout:   &logBuf,
		Stdout:   &stdoutBuf,
		Stderr:   io.Discard,
		WantJSON: false,
	}
	_ = ls.LogLevel.Set("info")

	var loadCalled bool
	cacher := &testutil.MockCacheHandler{
		NewCacheFunc: func(fsys afero.Fs, cacheDir string, cacheName string) schema.Cache {
			return &testutil.MockCache{
				LoadFunc: func() error {
					loadCalled = true

					return nil
				},
				PruneUnwalkedFunc: func() int {
					return 0
				},
			}
		},
	}

	args := Options{CacheDir: ""}
	_ = args.RunInterval.Set("24h")

	prog := NewService(fsys, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, cacher)
	require.NoError(t, prog.Info(t.Context(), []string{"/data"}, args))

	require.False(t, loadCalled)
}

// Expectation: Info should load the cache when CacheDir is set.
func Test_Service_Info_WithCacheDir_LoadsCache_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/data", 0o755))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout:   &logBuf,
		Stdout:   &stdoutBuf,
		Stderr:   io.Discard,
		WantJSON: false,
	}
	_ = ls.LogLevel.Set("info")

	var loadCalled bool
	cacher := &testutil.MockCacheHandler{
		NewCacheFunc: func(fsys afero.Fs, cacheDir string, cacheName string) schema.Cache {
			return &testutil.MockCache{
				LoadFunc: func() error {
					loadCalled = true

					return nil
				},
				PruneUnwalkedFunc: func() int {
					return 0
				},
			}
		},
	}

	args := Options{CacheDir: "/cache"}
	_ = args.RunInterval.Set("24h")

	prog := NewService(fsys, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, cacher)
	require.NoError(t, prog.Info(t.Context(), []string{"/data"}, args))

	require.True(t, loadCalled)
}

// Expectation: Info should call PruneUnwalked on the cache after enumeration.
func Test_Service_Info_PrunesCache_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/data", 0o755))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout:   &logBuf,
		Stdout:   &stdoutBuf,
		Stderr:   io.Discard,
		WantJSON: false,
	}
	_ = ls.LogLevel.Set("info")

	var pruneCalled bool
	cacher := &testutil.MockCacheHandler{
		NewCacheFunc: func(fsys afero.Fs, cacheDir string, cacheName string) schema.Cache {
			return &testutil.MockCache{
				PruneUnwalkedFunc: func() int {
					pruneCalled = true

					return 0
				},
			}
		},
	}

	args := Options{CacheDir: "/cache"}
	_ = args.RunInterval.Set("24h")

	prog := NewService(fsys, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, cacher)
	require.NoError(t, prog.Info(t.Context(), []string{"/data"}, args))

	require.True(t, pruneCalled)
}

// Expectation: Info should not save the cache (to avoid races with verification).
func Test_Service_Info_DoesNotSaveCache_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/data", 0o755))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout:   &logBuf,
		Stdout:   &stdoutBuf,
		Stderr:   io.Discard,
		WantJSON: false,
	}
	_ = ls.LogLevel.Set("info")

	var saveCalled bool
	cacher := &testutil.MockCacheHandler{
		NewCacheFunc: func(fsys afero.Fs, cacheDir string, cacheName string) schema.Cache {
			return &testutil.MockCache{
				SaveFunc: func() error {
					saveCalled = true

					return nil
				},
				PruneUnwalkedFunc: func() int {
					return 0
				},
			}
		},
	}

	args := Options{CacheDir: "/cache"}
	_ = args.RunInterval.Set("24h")

	prog := NewService(fsys, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, cacher)
	require.NoError(t, prog.Info(t.Context(), []string{"/data"}, args))

	require.False(t, saveCalled)
}

// Expectation: Info should show the overdue section with an overdue job.
func Test_Service_Info_Overdue_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now().Add(-20 * 24 * time.Hour),
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	require.NoError(t, prog.Info(t.Context(), []string{"/data"}, args))

	output := stdoutBuf.String()
	require.Contains(t, output, "Overdue jobs")
	require.Contains(t, output, "Due for longer than one full cycle: 1")
	require.Contains(t, output, "Warning: 1 jobs have been due for longer than one full cycle")
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	js := verify.Stats{
		TotalDuration:   2 * time.Hour,
		LargestDuration: 2 * time.Hour,
		LargestJob:      schema.NewJobMeta("/data/large.par2", nil, false),
		KnownCount:      1,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MaxDuration.Set("1h")

	prog.printDurationInfo(js, args)

	output := stdoutBuf.String()
	require.Contains(t, output, "Warning: The largest recorded job")
	require.Contains(t, output, "exceeds the given --duration")
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

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
	require.Contains(t, output, "continue to grow")
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 5 * time.Minute,
	}

	metas := []*verify.JobMeta{
		verify.NewJobMeta(schema.NewJobMeta("/data/test"+schema.Par2Extension, manifest, false)),
	}

	js := verify.Stats{
		JobCount:      1,
		KnownCount:    1,
		TotalDuration: 5 * time.Minute,
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")

	prog.printCycleInfo(js, metas, args, time.Now())

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 5 * time.Minute,
	}

	metas := []*verify.JobMeta{
		verify.NewJobMeta(schema.NewJobMeta("/data/test"+schema.Par2Extension, manifest, false)),
		verify.NewJobMeta(schema.NewJobMeta("/data/test2"+schema.Par2Extension, nil, false)),
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

	prog.printCycleInfo(js, metas, args, time.Now())

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

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

// Expectation: The printOverdueInfo should not output anything when MinAge is zero.
func Test_Service_printOverdueInfo_ZeroMinAge_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     now.Add(-20 * 24 * time.Hour),
		Duration: 5 * time.Minute,
	}

	metas := []*verify.JobMeta{
		verify.NewJobMeta(schema.NewJobMeta("/data/test"+schema.Par2Extension, manifest, false)),
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")

	prog.printOverdueInfo(metas, args, now)

	require.Empty(t, stdoutBuf.String())
}

// Expectation: The printOverdueInfo should report no overdue jobs when all are within one run.
func Test_Service_printOverdueInfo_NoneOverdue_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	// Not yet due.
	notDue := schema.NewManifest("notdue" + schema.Par2Extension)
	notDue.Verification = &schema.VerificationManifest{
		Time:     now.Add(-3 * 24 * time.Hour),
		Duration: 5 * time.Minute,
	}

	// Due, but still within one run interval.
	dueInRun := schema.NewManifest("dueinrun" + schema.Par2Extension)
	dueInRun.Verification = &schema.VerificationManifest{
		Time:     now.Add(-7*24*time.Hour - 12*time.Hour),
		Duration: 5 * time.Minute,
	}

	metas := []*verify.JobMeta{
		verify.NewJobMeta(schema.NewJobMeta("/data/notdue"+schema.Par2Extension, notDue, false)),
		verify.NewJobMeta(schema.NewJobMeta("/data/dueinrun"+schema.Par2Extension, dueInRun, false)),
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")

	prog.printOverdueInfo(metas, args, now)

	output := stdoutBuf.String()
	require.Contains(t, output, "Overdue jobs")
	require.Contains(t, output, "Overdue: NONE")
	require.NotContains(t, output, "Warning")
}

// Expectation: The printOverdueInfo should report a job overdue by one run and note it as normal with --duration.
func Test_Service_printOverdueInfo_OverdueRun_WithDuration_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     now.Add(-7*24*time.Hour - 2*24*time.Hour),
		Duration: 5 * time.Minute,
	}

	metas := []*verify.JobMeta{
		verify.NewJobMeta(schema.NewJobMeta("/data/test"+schema.Par2Extension, manifest, false)),
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")

	prog.printOverdueInfo(metas, args, now)

	output := stdoutBuf.String()
	require.Contains(t, output, "Due for longer than one run: 1")
	require.Contains(t, output, "Due for longer than one full cycle: 0")
	require.Contains(t, output, "Most overdue: test"+schema.Par2Extension)
	require.Contains(t, output, "Short delays are normal with --duration")
	require.NotContains(t, output, "Warning")
}

// Expectation: The printOverdueInfo should report a job overdue by one run without a note when no --duration is set.
func Test_Service_printOverdueInfo_OverdueRun_NoDuration_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     now.Add(-7*24*time.Hour - 2*24*time.Hour),
		Duration: 5 * time.Minute,
	}

	metas := []*verify.JobMeta{
		verify.NewJobMeta(schema.NewJobMeta("/data/test"+schema.Par2Extension, manifest, false)),
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")

	prog.printOverdueInfo(metas, args, now)

	output := stdoutBuf.String()
	require.Contains(t, output, "Due for longer than one run: 1")
	require.Contains(t, output, "Due for longer than one full cycle: 0")
	require.NotContains(t, output, "Short delays are normal")
	require.NotContains(t, output, "Warning")
}

// Expectation: The printOverdueInfo should warn about a job overdue by one full cycle.
func Test_Service_printOverdueInfo_OverdueCycle_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     now.Add(-7*24*time.Hour - 10*24*time.Hour),
		Duration: 5 * time.Minute,
	}

	metas := []*verify.JobMeta{
		verify.NewJobMeta(schema.NewJobMeta("/data/test"+schema.Par2Extension, manifest, false)),
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")

	prog.printOverdueInfo(metas, args, now)

	output := stdoutBuf.String()
	require.Contains(t, output, "Due for longer than one run: 1")
	require.Contains(t, output, "Due for longer than one full cycle: 1")
	require.Contains(t, output, "Warning: 1 jobs have been due for longer than one full cycle")
	require.NotContains(t, output, "Short delays are normal")
}

// Expectation: The printOverdueInfo should report the job with the largest lag as most overdue.
func Test_Service_printOverdueInfo_MostOverdue_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	lesser := schema.NewManifest("lesser" + schema.Par2Extension)
	lesser.Verification = &schema.VerificationManifest{
		Time:     now.Add(-7*24*time.Hour - 2*24*time.Hour),
		Duration: 5 * time.Minute,
	}

	greater := schema.NewManifest("greater" + schema.Par2Extension)
	greater.Verification = &schema.VerificationManifest{
		Time:     now.Add(-7*24*time.Hour - 3*24*time.Hour),
		Duration: 5 * time.Minute,
	}

	metas := []*verify.JobMeta{
		verify.NewJobMeta(schema.NewJobMeta("/data/lesser"+schema.Par2Extension, lesser, false)),
		verify.NewJobMeta(schema.NewJobMeta("/data/greater"+schema.Par2Extension, greater, false)),
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")

	prog.printOverdueInfo(metas, args, now)

	output := stdoutBuf.String()
	require.Contains(t, output, "Due for longer than one run: 2")
	require.Contains(t, output, "Most overdue: greater"+schema.Par2Extension)
}

// Expectation: The printOverdueInfo should skip jobs without verification or with a zero verification time.
func Test_Service_printOverdueInfo_SkipsUnverifiedAndZeroTime_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	zeroTime := schema.NewManifest("zerotime" + schema.Par2Extension)
	zeroTime.Verification = &schema.VerificationManifest{
		Duration: 5 * time.Minute,
	}

	metas := []*verify.JobMeta{
		verify.NewJobMeta(schema.NewJobMeta("/data/unverified"+schema.Par2Extension, nil, false)),
		verify.NewJobMeta(schema.NewJobMeta("/data/zerotime"+schema.Par2Extension, zeroTime, false)),
	}

	args := Options{}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")

	prog.printOverdueInfo(metas, args, now)

	output := stdoutBuf.String()
	require.Contains(t, output, "Overdue: NONE")
	require.NotContains(t, output, "Most overdue")
}
