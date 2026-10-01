package list

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

// Expectation: toJob should copy all fields and include the verification section for a verified job.
func Test_toJob_Verified_Success(t *testing.T) {
	t.Parallel()

	verifiedAt := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	m := newTestMeta("/data/test.par2")
	m.IsBundle = true
	m.HasManifest = true
	m.HasCreation = true
	m.HasVerification = true
	m.VerifyTime = verifiedAt
	m.VerifyDuration = 5 * time.Minute
	m.CountCorrupted = 3
	m.RepairNeeded = true
	m.RepairPossible = true
	m.MaybeEdited = true
	m.Saved = true

	job := toJob(m)

	require.Equal(t, "/data/test.par2", job.Path)
	require.Equal(t, "repairable", job.Status)
	require.True(t, job.IsBundle)
	require.True(t, job.IsCached)
	require.True(t, job.HasManifest)
	require.True(t, job.HasCreation)
	require.True(t, job.HasVerification)

	require.NotNil(t, job.Verification)
	require.Equal(t, verifiedAt, job.Verification.Time)
	require.Equal(t, 5*time.Minute, job.Verification.Duration)
	require.Equal(t, 3, job.Verification.CountCorrupted)
	require.True(t, job.Verification.RepairNeeded)
	require.True(t, job.Verification.RepairPossible)
	require.True(t, job.Verification.MaybeEdited)
}

// Expectation: toJob should omit the verification section for an unverified job.
func Test_toJob_Unverified_Success(t *testing.T) {
	t.Parallel()

	m := newTestMeta("/data/test.par2")
	m.HasManifest = true

	job := toJob(m)

	require.Equal(t, "/data/test.par2", job.Path)
	require.Equal(t, "unverified", job.Status)
	require.False(t, job.HasVerification)
	require.Nil(t, job.Verification)
}

// Expectation: toJob should derive the status the same way as statusOf.
func Test_toJob_StatusMatchesStatusOf_Success(t *testing.T) {
	t.Parallel()

	metas := make([]*verify.JobMeta, 0, 3)
	metas = append(metas, newTestMeta("/data/nomanifest.par2"))

	unrepairable := newTestMeta("/data/unrepairable.par2")
	unrepairable.HasManifest = true
	unrepairable.HasVerification = true
	unrepairable.RepairNeeded = true
	metas = append(metas, unrepairable)

	healthy := newTestMeta("/data/healthy.par2")
	healthy.HasManifest = true
	healthy.HasVerification = true
	metas = append(metas, healthy)

	for _, m := range metas {
		require.Equal(t, statusOf(m).String(), toJob(m).Status)
	}
}

// Expectation: openCacheJSON should not attempt to load when CacheDir is empty.
func Test_Service_openCacheJSON_NoCacheDir_Success(t *testing.T) {
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
	prog.openCacheJSON("/data", opts, &Result{})

	require.False(t, loadCalled)
}

// Expectation: openCacheJSON should attempt to load when CacheDir is set.
func Test_Service_openCacheJSON_WithCacheDir_LoadsCacheFromDisk_Success(t *testing.T) {
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
	prog.openCacheJSON("/data", opts, &Result{})

	require.True(t, loadCalled)
}

// Expectation: openCacheJSON should set a warning when cache loading fails with a non-ErrNotExist error.
func Test_Service_openCacheJSON_LoadError_SetsWarning_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
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
	result := &Result{}
	cache := prog.openCacheJSON("/data", opts, result)

	require.NotNil(t, cache)
	require.Contains(t, result.Warning, "could not be loaded")
}

// Expectation: openCacheJSON should silently ignore ErrNotExist when loading the cache.
func Test_Service_openCacheJSON_LoadErrNotExist_NoWarning_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
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
	result := &Result{}
	cache := prog.openCacheJSON("/data", opts, result)

	require.NotNil(t, cache)
	require.Empty(t, result.Warning)
}

// Expectation: An empty root should output an empty jobs array (not null).
func Test_Service_PrintJSON_NoJobs_Success(t *testing.T) {
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
	require.NoError(t, prog.PrintJSON(t.Context(), []string{"/data"}, Options{}))

	require.Contains(t, stdoutBuf.String(), `"jobs": []`)

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotEmpty(t, result.Roots)
	require.NotZero(t, result.Time)
	require.NotNil(t, result.Options)
	require.Empty(t, result.Jobs)
	require.Empty(t, result.Warning)
}

// Expectation: The JSON output should be valid and decode back to the Result struct.
func Test_Service_PrintJSON_WithJobs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	verifiedAt := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     verifiedAt,
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
	require.NoError(t, prog.PrintJSON(t.Context(), []string{"/data"}, Options{}))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.Len(t, result.Jobs, 1)

	job := result.Jobs[0]
	require.Equal(t, "/data/test"+schema.Par2Extension, job.Path)
	require.Equal(t, "healthy", job.Status)
	require.True(t, job.HasManifest)
	require.True(t, job.HasVerification)

	require.NotNil(t, job.Verification)
	require.True(t, verifiedAt.Equal(job.Verification.Time))
	require.Equal(t, 5*time.Minute, job.Verification.Duration)
	require.Zero(t, job.Verification.CountCorrupted)
	require.False(t, job.Verification.RepairNeeded)
}

// Expectation: An unverified job should have no verification section.
func Test_Service_PrintJSON_UnverifiedJob_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest("test" + schema.Par2Extension)
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
	require.NoError(t, prog.PrintJSON(t.Context(), []string{"/data"}, Options{}))

	require.NotContains(t, stdoutBuf.String(), `"verification":`)

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.Len(t, result.Jobs, 1)
	require.Equal(t, "unverified", result.Jobs[0].Status)
	require.False(t, result.Jobs[0].HasVerification)
	require.Nil(t, result.Jobs[0].Verification)
}

// Expectation: Jobs should be ordered by severity before path.
func Test_Service_PrintJSON_SortedBySeverity_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/a"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/b"+schema.Par2Extension, []byte("par2"), 0o644))

	healthy := schema.NewManifest("a" + schema.Par2Extension)
	healthy.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 5 * time.Minute,
	}
	require.NoError(t, writeTestManifest(t, fs, "/data/a"+schema.Par2Extension+schema.ManifestExtension, healthy))

	unverified := schema.NewManifest("b" + schema.Par2Extension)
	require.NoError(t, writeTestManifest(t, fs, "/data/b"+schema.Par2Extension+schema.ManifestExtension, unverified))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})
	require.NoError(t, prog.PrintJSON(t.Context(), []string{"/data"}, Options{}))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.Len(t, result.Jobs, 2)
	require.Equal(t, "unverified", result.Jobs[0].Status)
	require.Equal(t, "/data/b"+schema.Par2Extension, result.Jobs[0].Path)
	require.Equal(t, "healthy", result.Jobs[1].Status)
	require.Equal(t, "/data/a"+schema.Par2Extension, result.Jobs[1].Path)
}

// Expectation: The program should handle multiple provided root directories.
func Test_Service_PrintJSON_MultiRoot_Success(t *testing.T) {
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
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})
	require.NoError(t, prog.PrintJSON(t.Context(), []string{"/data", "/data2"}, Options{}))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.Len(t, result.Roots, 2)
	require.Len(t, result.Jobs, 2)
	require.Equal(t, "/data/test"+schema.Par2Extension, result.Jobs[0].Path)
	require.Equal(t, "/data2/test"+schema.Par2Extension, result.Jobs[1].Path)
}

// Expectation: The options used for the run should be included in the result.
func Test_Service_PrintJSON_WithOptions_Success(t *testing.T) {
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

	args := Options{SkipNotCreated: true, CacheDir: "/cache"}
	require.NoError(t, prog.PrintJSON(t.Context(), []string{"/data"}, args))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.NotNil(t, result.Options)
	require.True(t, result.Options.SkipNotCreated)
	require.Equal(t, "/cache", result.Options.CacheDir)
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})
	err := prog.PrintJSON(ctx, []string{"/data"}, Options{})

	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: PrintJSON should not load the cache when CacheDir is empty.
func Test_Service_PrintJSON_WithoutCacheDir_NotLoadsCache_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, cacher)
	require.NoError(t, prog.PrintJSON(t.Context(), []string{"/data"}, Options{CacheDir: ""}))

	require.False(t, loadCalled)
}

// Expectation: PrintJSON should load the cache when CacheDir is set.
func Test_Service_PrintJSON_WithCacheDir_LoadsCache_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, cacher)
	require.NoError(t, prog.PrintJSON(t.Context(), []string{"/data"}, Options{CacheDir: "/cache"}))

	require.True(t, loadCalled)
}

// Expectation: PrintJSON should call PruneUnwalked on the cache after enumeration.
func Test_Service_PrintJSON_PrunesCache_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, cacher)
	require.NoError(t, prog.PrintJSON(t.Context(), []string{"/data"}, Options{CacheDir: "/cache"}))

	require.True(t, pruneCalled)
}

// Expectation: PrintJSON should not save the cache (to avoid races with verification).
func Test_Service_PrintJSON_DoesNotSaveCache_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, cacher)
	require.NoError(t, prog.PrintJSON(t.Context(), []string{"/data"}, Options{CacheDir: "/cache"}))

	require.False(t, saveCalled)
}
