package list

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"strings"
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

func newTestMeta(path string) *verify.JobMeta {
	return verify.NewJobMeta(schema.NewJobMeta(path, nil, false))
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
	prog.openCache(t.Context(), "/data", opts)

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
	prog.openCache(t.Context(), "/data", opts)

	require.True(t, loadCalled)
}

// Expectation: openCache should log an error when cache loading fails with a non-ErrNotExist error.
func Test_Service_openCache_LoadError_LogsError_Success(t *testing.T) {
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
	cache := prog.openCache(t.Context(), "/data", opts)

	require.NotNil(t, cache)
	require.Contains(t, logBuf.String(), "Failed to load manifest cache")
}

// Expectation: openCache should silently ignore ErrNotExist when loading the cache.
func Test_Service_openCache_LoadErrNotExist_NoLog_Success(t *testing.T) {
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
	cache := prog.openCache(t.Context(), "/data", opts)

	require.NotNil(t, cache)
	require.NotContains(t, logBuf.String(), "Failed to load manifest cache")
}

// Expectation: An empty root should output only the table header.
func Test_Service_List_NoJobs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout:   &logBuf,
		Stdout:   &stdoutBuf,
		Stderr:   io.Discard,
		WantJSON: false,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})
	require.NoError(t, prog.List(t.Context(), []string{"/data"}, Options{}))

	lines := strings.Split(strings.TrimSpace(stdoutBuf.String()), "\n")
	require.Len(t, lines, 1)
	require.Equal(t, []string{"STATUS", "VERIFIED", "DURATION", "FAILURES", "EDITED", "CACHED", "PATH"}, strings.Fields(lines[0]))
}

// Expectation: The manifest should be parsed and the correct row be shown.
func Test_Service_List_WithJobs_Success(t *testing.T) {
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
		Logout:   &logBuf,
		Stdout:   &stdoutBuf,
		Stderr:   io.Discard,
		WantJSON: false,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})
	require.NoError(t, prog.List(t.Context(), []string{"/data"}, Options{}))

	lines := strings.Split(strings.TrimSpace(stdoutBuf.String()), "\n")
	require.Len(t, lines, 2)
	require.True(t, strings.HasPrefix(lines[1], "healthy"))
	require.Contains(t, lines[1], verifiedAt.Local().Format("2006-01-02T15:04:05"))
	require.Contains(t, lines[1], (5 * time.Minute).String())
	require.True(t, strings.HasSuffix(lines[1], "/data/test"+schema.Par2Extension))
}

// Expectation: Jobs should be ordered by severity before path.
func Test_Service_List_SortedBySeverity_Success(t *testing.T) {
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
		Logout:   &logBuf,
		Stdout:   &stdoutBuf,
		Stderr:   io.Discard,
		WantJSON: false,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})
	require.NoError(t, prog.List(t.Context(), []string{"/data"}, Options{}))

	lines := strings.Split(strings.TrimSpace(stdoutBuf.String()), "\n")
	require.Len(t, lines, 3)
	require.True(t, strings.HasPrefix(lines[1], "unverified"))
	require.True(t, strings.HasSuffix(lines[1], "/data/b"+schema.Par2Extension))
	require.True(t, strings.HasPrefix(lines[2], "healthy"))
	require.True(t, strings.HasSuffix(lines[2], "/data/a"+schema.Par2Extension))
}

// Expectation: Jobs with the same status should be ordered by path.
func Test_Service_List_SortedByPath_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/c"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/a"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest("c" + schema.Par2Extension)
	require.NoError(t, writeTestManifest(t, fs, "/data/c"+schema.Par2Extension+schema.ManifestExtension, manifest))

	manifest = schema.NewManifest("a" + schema.Par2Extension)
	require.NoError(t, writeTestManifest(t, fs, "/data/a"+schema.Par2Extension+schema.ManifestExtension, manifest))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout:   &logBuf,
		Stdout:   &stdoutBuf,
		Stderr:   io.Discard,
		WantJSON: false,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})
	require.NoError(t, prog.List(t.Context(), []string{"/data"}, Options{}))

	lines := strings.Split(strings.TrimSpace(stdoutBuf.String()), "\n")
	require.Len(t, lines, 3)
	require.True(t, strings.HasSuffix(lines[1], "/data/a"+schema.Par2Extension))
	require.True(t, strings.HasSuffix(lines[2], "/data/c"+schema.Par2Extension))
}

// Expectation: The program should handle multiple provided root directories.
func Test_Service_List_MultiRoot_Success(t *testing.T) {
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
		WantJSON: false,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})
	require.NoError(t, prog.List(t.Context(), []string{"/data", "/data2"}, Options{}))

	lines := strings.Split(strings.TrimSpace(stdoutBuf.String()), "\n")
	require.Len(t, lines, 3)
	require.True(t, strings.HasSuffix(lines[1], "/data/test"+schema.Par2Extension))
	require.True(t, strings.HasSuffix(lines[2], "/data2/test"+schema.Par2Extension))
}

// Expectation: List should output JSON instead of a table when JSON is wanted.
func Test_Service_List_WantJSON_Success(t *testing.T) {
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
	require.NoError(t, prog.List(t.Context(), []string{"/data"}, Options{}))

	var result Result
	require.NoError(t, json.Unmarshal(stdoutBuf.Bytes(), &result))

	require.Len(t, result.Jobs, 1)
	require.NotContains(t, stdoutBuf.String(), "STATUS")
}

// Expectation: A cancellation should be respected and the correct error returned.
func Test_Service_List_CtxCancel_Error(t *testing.T) {
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
	err := prog.List(ctx, []string{"/data"}, Options{})

	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: List should not load the cache when CacheDir is empty.
func Test_Service_List_WithoutCacheDir_NotLoadsCache_Success(t *testing.T) {
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

	prog := NewService(fsys, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, cacher)
	require.NoError(t, prog.List(t.Context(), []string{"/data"}, Options{CacheDir: ""}))

	require.False(t, loadCalled)
}

// Expectation: List should load the cache when CacheDir is set.
func Test_Service_List_WithCacheDir_LoadsCache_Success(t *testing.T) {
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

	prog := NewService(fsys, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, cacher)
	require.NoError(t, prog.List(t.Context(), []string{"/data"}, Options{CacheDir: "/cache"}))

	require.True(t, loadCalled)
}

// Expectation: List should call PruneUnwalked on the cache after enumeration.
func Test_Service_List_PrunesCache_Success(t *testing.T) {
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

	prog := NewService(fsys, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, cacher)
	require.NoError(t, prog.List(t.Context(), []string{"/data"}, Options{CacheDir: "/cache"}))

	require.True(t, pruneCalled)
}

// Expectation: List should not save the cache (to avoid races with verification).
func Test_Service_List_DoesNotSaveCache_Success(t *testing.T) {
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

	prog := NewService(fsys, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, cacher)
	require.NoError(t, prog.List(t.Context(), []string{"/data"}, Options{CacheDir: "/cache"}))

	require.False(t, saveCalled)
}

// Expectation: printTable should print a row with the correct status for every job state.
func Test_Service_printTable_AllStatuses_Success(t *testing.T) {
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

	verifiedAt := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	unrepairable := newTestMeta("/data/unrepairable.par2")
	unrepairable.HasManifest = true
	unrepairable.HasVerification = true
	unrepairable.VerifyTime = verifiedAt
	unrepairable.VerifyDuration = 5 * time.Minute
	unrepairable.CountCorrupted = 12
	unrepairable.RepairNeeded = true
	unrepairable.Saved = true

	repairable := newTestMeta("/data/repairable.par2")
	repairable.HasManifest = true
	repairable.HasVerification = true
	repairable.VerifyTime = verifiedAt
	repairable.VerifyDuration = 5 * time.Minute
	repairable.CountCorrupted = 3
	repairable.RepairNeeded = true
	repairable.RepairPossible = true
	repairable.MaybeEdited = true

	unverified := newTestMeta("/data/unverified.par2")
	unverified.HasManifest = true

	healthy := newTestMeta("/data/healthy.par2")
	healthy.HasManifest = true
	healthy.HasVerification = true
	healthy.VerifyTime = verifiedAt
	healthy.VerifyDuration = 5 * time.Minute

	require.NoError(t, prog.printTable([]*verify.JobMeta{unrepairable, repairable, unverified, healthy}, Options{CacheDir: "/tmp"}))

	lines := strings.Split(strings.TrimSpace(stdoutBuf.String()), "\n")
	require.Len(t, lines, 5)

	require.True(t, strings.HasPrefix(lines[1], "unrepairable"))
	require.Contains(t, lines[1], "12")
	require.Contains(t, lines[1], "Y")
	require.True(t, strings.HasSuffix(lines[1], "/data/unrepairable.par2"))

	require.True(t, strings.HasPrefix(lines[2], "repairable"))
	require.Contains(t, lines[2], "3")
	require.Contains(t, lines[2], "N")
	require.Contains(t, lines[2], "Y")
	require.True(t, strings.HasSuffix(lines[2], "/data/repairable.par2"))

	require.True(t, strings.HasPrefix(lines[3], "unverified"))
	require.True(t, strings.HasSuffix(lines[3], "/data/unverified.par2"))

	require.True(t, strings.HasPrefix(lines[4], "healthy"))
	require.Contains(t, lines[4], verifiedAt.Local().Format("2006-01-02T15:04:05"))
	require.True(t, strings.HasSuffix(lines[4], "/data/healthy.par2"))

	require.Equal(t, []string{
		"healthy",
		verifiedAt.Local().Format("2006-01-02T15:04:05"),
		(5 * time.Minute).String(),
		"0", "N", "N", "/data/healthy.par2",
	}, strings.Fields(lines[4]))
}

// Expectation: printTable should print dashes for the fields of an unverified job.
func Test_Service_printTable_Unverified_Dashes_Success(t *testing.T) {
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

	unverified := newTestMeta("/data/unverified.par2")
	unverified.HasManifest = true

	require.NoError(t, prog.printTable([]*verify.JobMeta{unverified}, Options{}))

	lines := strings.Split(strings.TrimSpace(stdoutBuf.String()), "\n")
	require.Len(t, lines, 2)
	require.Equal(t, []string{"unverified", "-", "-", "-", "-", "-", "/data/unverified.par2"}, strings.Fields(lines[1]))
}

// Expectation: printTable should align the path column across all rows.
func Test_Service_printTable_Aligned_Success(t *testing.T) {
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

	unverified := newTestMeta("/data/short.par2")
	unverified.HasManifest = true

	unrepairable := newTestMeta("/data/a/much/longer/path.par2")
	unrepairable.HasManifest = true
	unrepairable.HasVerification = true
	unrepairable.VerifyTime = time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	unrepairable.VerifyDuration = 2 * time.Hour
	unrepairable.CountCorrupted = 12345
	unrepairable.RepairNeeded = true

	require.NoError(t, prog.printTable([]*verify.JobMeta{unrepairable, unverified}, Options{}))

	lines := strings.Split(strings.TrimSpace(stdoutBuf.String()), "\n")
	require.Len(t, lines, 3)

	headerCol := strings.Index(lines[0], "PATH")
	require.Equal(t, headerCol, strings.Index(lines[1], "/data/"))
	require.Equal(t, headerCol, strings.Index(lines[2], "/data/"))
}

// Expectation: printTable should show Y or N in the cached column when a cache directory is configured.
func Test_Service_printTable_CachedColumn_WithCacheDir_Success(t *testing.T) {
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

	cached := newTestMeta("/data/cached.par2")
	cached.HasManifest = true
	cached.Saved = true

	uncached := newTestMeta("/data/uncached.par2")
	uncached.HasManifest = true

	require.NoError(t, prog.printTable([]*verify.JobMeta{cached, uncached}, Options{CacheDir: "/tmp"}))

	lines := strings.Split(strings.TrimSpace(stdoutBuf.String()), "\n")
	require.Len(t, lines, 3)
	require.Equal(t, "Y", strings.Fields(lines[1])[5])
	require.Equal(t, "N", strings.Fields(lines[2])[5])
}
