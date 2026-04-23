package bundler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: A new job should be returned with the correct values for a non-bundle.
func Test_NewJob_Success(t *testing.T) {
	t.Parallel()

	mf := &schema.Manifest{}
	opts := Options{Force: true}

	job := NewJob("/data/folder/test"+schema.Par2Extension, opts, mf, false)

	require.Equal(t, "/data/folder", job.workingDir)
	require.Equal(t, "test"+schema.Par2Extension, job.par2Name)
	require.Equal(t, "/data/folder/test"+schema.Par2Extension, job.par2Path)
	require.Equal(t, "test"+schema.Par2Extension+schema.ManifestExtension, job.manifestName)
	require.Equal(t, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension, job.manifestPath)
	require.Equal(t, "/data/folder/test"+schema.Par2Extension+schema.LockExtension, job.lockPath)
	require.True(t, job.force)
	require.False(t, job.isBundle)
	require.Equal(t, mf, job.manifest)
}

// Expectation: A new job for a bundle should reuse the bundle path for manifest and lock.
func Test_NewJob_Bundle_Success(t *testing.T) {
	t.Parallel()

	mf := &schema.Manifest{}
	opts := Options{Force: false}

	job := NewJob("/data/folder/test"+schema.BundleExtension+schema.Par2Extension, opts, mf, true)

	require.Equal(t, "/data/folder", job.workingDir)
	require.Equal(t, "test"+schema.BundleExtension+schema.Par2Extension, job.par2Name)
	require.Equal(t, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, job.par2Path)
	require.True(t, job.isBundle)

	// Bundle reuses its own path for manifest and lock.
	require.Equal(t, "test"+schema.BundleExtension+schema.Par2Extension, job.manifestName)
	require.Equal(t, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, job.manifestPath)
	require.Equal(t, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, job.lockPath)
}

// Expectation: Force should be false by default.
func Test_NewJob_ForceDefault_Success(t *testing.T) {
	t.Parallel()

	job := NewJob("/data/test"+schema.Par2Extension, Options{}, nil, false)

	require.False(t, job.force)
	require.Nil(t, job.manifest)
}

// Expectation: The function should process all jobs and return success results.
func Test_Service_processMode_Pack_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	mfData, err := json.Marshal(mf)
	require.NoError(t, err)
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, mfData, 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{SetID: setID}},
				},
			}, nil
		},
	}

	bndlr := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			require.NoError(t, afero.WriteFile(fsys, bundlePath, []byte("bundledata"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, par2er)

	results, err := prog.Pack(t.Context(), []string{"/data"}, Options{})
	require.NoError(t, err)
	require.Equal(t, 1, results.Selected)
	require.Equal(t, 1, results.Success)
	require.Equal(t, 0, results.Error)

	require.Contains(t, logBuf.String(), "Job completed with success")
}

// Expectation: The Unpack function should find bundles and unpack them.
func Test_Service_processMode_Unpack_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.BundleExtension+schema.Par2Extension, []byte("bundledata"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	mockBundle := &testutil.MockBundle{
		UnpackFunc: func(fsys afero.Fs, destDir string, strict bool) ([]string, error) {
			return []string{"/data/test" + schema.Par2Extension}, nil
		},
		CloseFunc: func() error {
			return nil
		},
	}

	bndlr := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, &testutil.MockPar2Handler{})

	results, err := prog.Unpack(t.Context(), []string{"/data"}, Options{})
	require.NoError(t, err)
	require.Equal(t, 1, results.Selected)
	require.Equal(t, 1, results.Success)
	require.Equal(t, 0, results.Error)

	require.Contains(t, logBuf.String(), "Job completed with success")
}

// Expectation: Multiple root directories should be processed.
func Test_Service_processMode_MultiRoot_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	for _, dir := range []string{"/data1", "/data2"} {
		require.NoError(t, fs.MkdirAll(dir, 0o755))
		require.NoError(t, afero.WriteFile(fs, dir+"/test"+schema.Par2Extension, []byte("par2data"), 0o644))

		mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
		mfData, err := json.Marshal(mf)
		require.NoError(t, err)
		require.NoError(t, afero.WriteFile(fs, dir+"/test"+schema.Par2Extension+schema.ManifestExtension, mfData, 0o644))
	}

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{SetID: setID}},
				},
			}, nil
		},
	}

	bndlr := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			require.NoError(t, afero.WriteFile(fsys, bundlePath, []byte("bundledata"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, par2er)

	results, err := prog.Pack(t.Context(), []string{"/data1", "/data2"}, Options{})
	require.NoError(t, err)
	require.Equal(t, 2, results.Selected)
	require.Equal(t, 2, results.Success)
}

// Expectation: Nothing to do should not error.
func Test_Service_processMode_NoJobs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	results, err := prog.Pack(t.Context(), []string{"/data"}, Options{})
	require.NoError(t, err)
	require.Equal(t, 0, results.Selected)

	require.Contains(t, logBuf.String(), "Nothing to do")
}

// Expectation: Context cancellation should be respected during processing.
func Test_Service_processMode_CtxCancel_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	_, err := prog.Pack(ctx, []string{"/data"}, Options{})
	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: Non-fatal enumeration errors should continue and return partial failure.
func Test_Service_processMode_FailedToEnumerateSomeJobs_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	jobs, err := prog.processMode(
		t.Context(),
		[]string{"/data"},
		Options{},
		func(ctx context.Context, rootDir string, opts Options) ([]*Job, error) {
			return []*Job{
				{par2Path: "/data/test1" + schema.Par2Extension},
				{par2Path: "/data/test2" + schema.Par2Extension},
			}, errors.Join(schema.ErrNonFatal, errors.New("manifest read failed"))
		},
		func(ctx context.Context, job *Job) error {
			return nil
		},
	)

	require.ErrorIs(t, err, schema.ErrExitPartialFailure)
	require.ErrorContains(t, err, "failed to enumerate some jobs")
	require.Equal(t, 2, jobs.Selected)
	require.Equal(t, 2, jobs.Success)
	require.Equal(t, 0, jobs.Error)
}

// Expectation: A failed job should be skipped and counted as a partial failure.
func Test_Service_processMode_JobFailureSkipping_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	results, err := prog.processMode(
		t.Context(),
		[]string{"/data"},
		Options{},
		func(ctx context.Context, rootDir string, opts Options) ([]*Job, error) {
			return []*Job{
				{par2Path: "/data/test" + schema.Par2Extension},
			}, nil
		},
		func(ctx context.Context, job *Job) error {
			return errors.New("simulated job failure")
		},
	)

	require.ErrorIs(t, err, schema.ErrExitPartialFailure)
	require.ErrorContains(t, err, "simulated job failure")
	require.Equal(t, 1, results.Selected)
	require.Equal(t, 0, results.Success)
	require.Equal(t, 1, results.Error)
	require.Contains(t, logBuf.String(), "Job failure (skipping)")
}

// Expectation: Mixed job outcomes should report one success and one partial failure.
func Test_Service_processMode_OneSucceedsOneFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	call := 0
	results, err := prog.processMode(
		t.Context(),
		[]string{"/data"},
		Options{},
		func(ctx context.Context, rootDir string, opts Options) ([]*Job, error) {
			return []*Job{
				{par2Path: "/data/test1" + schema.Par2Extension},
				{par2Path: "/data/test2" + schema.Par2Extension},
			}, nil
		},
		func(ctx context.Context, job *Job) error {
			call++
			if call == 1 {
				return nil
			}

			return errors.New("second job failed")
		},
	)

	require.ErrorIs(t, err, schema.ErrExitPartialFailure)
	require.ErrorContains(t, err, "second job failed")
	require.Equal(t, 2, results.Selected)
	require.Equal(t, 1, results.Success)
	require.Equal(t, 1, results.Error)
}

// Expectation: The function should find PAR2 files with manifests and return them as jobs.
func Test_Service_packEnumerate_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	mfData, err := json.Marshal(mf)
	require.NoError(t, err)
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, mfData, 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	jobs, err := prog.packEnumerate(t.Context(), "/data", Options{})
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, "/data/test"+schema.Par2Extension, jobs[0].par2Path)
	require.NotNil(t, jobs[0].manifest)
}

// Expectation: PAR2 files without manifests should be skipped.
func Test_Service_packEnumerate_NoManifest_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	jobs, err := prog.packEnumerate(t.Context(), "/data", Options{})
	require.NoError(t, err)
	require.Empty(t, jobs)
}

// Expectation: Bundle files should be skipped during pack enumeration.
func Test_Service_packEnumerate_SkipsBundles_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.BundleExtension+schema.Par2Extension, []byte("bundledata"), 0o644))

	mf := &schema.Manifest{Name: "test" + schema.BundleExtension + schema.Par2Extension}
	mfData, err := json.Marshal(mf)
	require.NoError(t, err)
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.BundleExtension+schema.Par2Extension+schema.ManifestExtension, mfData, 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	jobs, err := prog.packEnumerate(t.Context(), "/data", Options{})
	require.NoError(t, err)
	require.Empty(t, jobs)
}

// Expectation: Context cancellation should be respected during enumeration.
func Test_Service_packEnumerate_CtxCancel_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	_, err := prog.packEnumerate(ctx, "/data", Options{})
	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: An invalid manifest should result in a non-fatal error but still continue.
func Test_Service_packEnumerate_InvalidManifest_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, []byte("not valid json"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	jobs, err := prog.packEnumerate(t.Context(), "/data", Options{})
	require.ErrorIs(t, err, schema.ErrNonFatal)
	require.Empty(t, jobs)
}

// Expectation: Fatal manifest-processing errors should stop enumeration.
func Test_Service_packEnumerate_FailedToProcessManifest_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	rootDir := t.TempDir()

	par2Path := filepath.Join(rootDir, "test"+schema.Par2Extension)
	require.NoError(t, afero.WriteFile(fs, par2Path, []byte("par2data"), 0o644))

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	mfData, err := json.Marshal(mf)
	require.NoError(t, err)
	require.NoError(t, afero.WriteFile(fs, par2Path+schema.ManifestExtension, mfData, 0o644))
	require.NoError(t, fs.MkdirAll(par2Path+schema.LockExtension, 0o755)) // as dir to fail

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	jobs, err := prog.packEnumerate(t.Context(), rootDir, Options{})
	require.ErrorContains(t, err, "failed to process manifest")
	require.Nil(t, jobs)
}

// Expectation: Elements in directories with an ignore file should be skipped.
func Test_Service_packEnumerate_IgnoreFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	mfData, err := json.Marshal(mf)
	require.NoError(t, err)
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, mfData, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/"+schema.IgnoreFile, []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	jobs, err := prog.packEnumerate(t.Context(), "/data", Options{})
	require.NoError(t, err)
	require.Empty(t, jobs)
}

// Expectation: Multiple PAR2 files with manifests should all be returned.
func Test_Service_packEnumerate_MultipleJobs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	for _, name := range []string{"test1", "test2"} {
		require.NoError(t, afero.WriteFile(fs, "/data/"+name+schema.Par2Extension, []byte("par2data"), 0o644))

		mf := &schema.Manifest{Name: name + schema.Par2Extension}
		mfData, err := json.Marshal(mf)
		require.NoError(t, err)
		require.NoError(t, afero.WriteFile(fs, "/data/"+name+schema.Par2Extension+schema.ManifestExtension, mfData, 0o644))
	}

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	jobs, err := prog.packEnumerate(t.Context(), "/data", Options{})
	require.NoError(t, err)
	require.Len(t, jobs, 2)
}

// Expectation: The function should return a job when a valid manifest exists.
func Test_Service_packProcessManifest_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	mfData, err := json.Marshal(mf)
	require.NoError(t, err)
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, mfData, 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	job, err := prog.packProcessManifest(t.Context(), "/data/test"+schema.Par2Extension, Options{})
	require.NoError(t, err)
	require.NotNil(t, job)
	require.NotNil(t, job.manifest)
	require.Equal(t, "test"+schema.Par2Extension, job.manifest.Name)
}

// Expectation: The function should return a silent skip when no manifest is found.
func Test_Service_packProcessManifest_NoManifest_SilentSkip(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	job, err := prog.packProcessManifest(t.Context(), "/data/test"+schema.Par2Extension, Options{})
	require.ErrorIs(t, err, schema.ErrSilentSkip)
	require.Nil(t, job)
}

// Expectation: The function should return a non-fatal error when the manifest is invalid JSON.
func Test_Service_packProcessManifest_InvalidManifest_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, []byte("not valid json"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	job, err := prog.packProcessManifest(t.Context(), "/data/test"+schema.Par2Extension, Options{})
	require.ErrorIs(t, err, schema.ErrNonFatal)
	require.Nil(t, job)
}

// Expectation: The function should return a non-fatal error when the manifest file cannot be read.
func Test_Service_packProcessManifest_ReadFails_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))
	require.NoError(t, afero.WriteFile(baseFs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, []byte("{}"), 0o644))

	fs := &testutil.FailingOpenFs{Fs: baseFs, FailPattern: schema.ManifestExtension}

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	job, err := prog.packProcessManifest(t.Context(), "/data/test"+schema.Par2Extension, Options{})
	require.ErrorIs(t, err, schema.ErrNonFatal)
	require.Nil(t, job)
}

// Expectation: The function should pack PAR2 files into a bundle and clean up originals.
func Test_Service_packBundle_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension, []byte("mf"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension+schema.LockExtension, []byte("lck"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol00+01"+schema.Par2Extension, []byte("vol1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol01+02"+schema.Par2Extension, []byte("vol2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{SetID: setID}},
				},
			}, nil
		},
	}

	var packCalled bool
	bndlr := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			packCalled = true

			require.Equal(t, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, bundlePath)
			require.Equal(t, setID, recoverySetID)
			require.NotEmpty(t, manifest.Bytes)
			require.NotEmpty(t, files)

			require.NoError(t, afero.WriteFile(fsys, bundlePath, []byte("bundledata"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, par2er)

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	job := NewJob("/data/folder/test"+schema.Par2Extension, Options{}, mf, false)

	require.NoError(t, prog.packBundle(t.Context(), job))
	require.True(t, packCalled)

	// Original PAR2 files should be cleaned up.
	indexExists, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension)
	require.False(t, indexExists)

	vol1Exists, _ := afero.Exists(fs, "/data/folder/test.vol00+01"+schema.Par2Extension)
	require.False(t, vol1Exists)

	vol2Exists, _ := afero.Exists(fs, "/data/folder/test.vol01+02"+schema.Par2Extension)
	require.False(t, vol2Exists)

	mfExists, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension)
	require.False(t, mfExists)

	lockExists, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension+schema.LockExtension)
	require.False(t, lockExists)

	// Bundle should exist.
	bundleExists, _ := afero.Exists(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension)
	require.True(t, bundleExists)
}

// Expectation: The function should pack PAR2 files into a bundle and clean up originals.
func Test_Service_packBundle_UpperCase_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+strings.ToUpper(schema.Par2Extension), []byte("par2index"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+strings.ToUpper(schema.Par2Extension)+schema.ManifestExtension, []byte("mf"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+strings.ToUpper(schema.Par2Extension)+schema.LockExtension, []byte("lck"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol00+01"+strings.ToUpper(schema.Par2Extension), []byte("vol1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol01+02"+strings.ToUpper(schema.Par2Extension), []byte("vol2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{SetID: setID}},
				},
			}, nil
		},
	}

	var packCalled bool
	bndlr := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			packCalled = true
			require.Equal(t, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, bundlePath)
			require.Equal(t, setID, recoverySetID)
			require.NotEmpty(t, manifest.Bytes)
			require.NotEmpty(t, files)
			require.NoError(t, afero.WriteFile(fsys, bundlePath, []byte("bundledata"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, par2er)

	mf := &schema.Manifest{Name: "test" + strings.ToUpper(schema.Par2Extension)}

	job := NewJob("/data/folder/test"+strings.ToUpper(schema.Par2Extension), Options{}, mf, false)

	require.NoError(t, prog.packBundle(t.Context(), job))
	require.True(t, packCalled)

	// Original PAR2 files should be cleaned up.
	indexExists, _ := afero.Exists(fs, "/data/folder/test"+strings.ToUpper(schema.Par2Extension))
	require.False(t, indexExists)

	vol1Exists, _ := afero.Exists(fs, "/data/folder/test.vol00+01"+strings.ToUpper(schema.Par2Extension))
	require.False(t, vol1Exists)

	vol2Exists, _ := afero.Exists(fs, "/data/folder/test.vol01+02"+strings.ToUpper(schema.Par2Extension))
	require.False(t, vol2Exists)

	mfExists, _ := afero.Exists(fs, "/data/folder/test"+strings.ToUpper(schema.Par2Extension)+schema.ManifestExtension)
	require.False(t, mfExists)

	lockExists, _ := afero.Exists(fs, "/data/folder/test"+strings.ToUpper(schema.Par2Extension)+schema.LockExtension)
	require.False(t, lockExists)

	// Bundle should exist.
	bundleExists, _ := afero.Exists(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension)
	require.True(t, bundleExists)
}

// Expectation: The function should return an error when no bundleable files are found.
func Test_Service_packBundle_NoFilesFound_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/unrelated.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	job := NewJob("/data/folder/test"+schema.Par2Extension, Options{}, mf, false)

	require.ErrorContains(t, prog.packBundle(t.Context(), job), "failed to find files to bundle")
}

// Expectation: The function should return an error when the PAR2 index file cannot be parsed.
func Test_Service_packBundle_ParseFileFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return nil, errors.New("parse error")
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, par2er)

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	job := NewJob("/data/folder/test"+schema.Par2Extension, Options{}, mf, false)

	require.ErrorContains(t, prog.packBundle(t.Context(), job), "failed to parse index par2")
}

// Expectation: The function should return an error when the PAR2 file has no sets.
func Test_Service_packBundle_NoSets_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{Sets: []par2.Set{}}, nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, par2er)

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	job := NewJob("/data/folder/test"+schema.Par2Extension, Options{}, mf, false)

	require.ErrorContains(t, prog.packBundle(t.Context(), job), "malformed file")
}

// Expectation: The function should return an error when the PAR2 file has a nil main packet.
func Test_Service_packBundle_NilMainPacket_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: nil},
				},
			}, nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, par2er)

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	job := NewJob("/data/folder/test"+schema.Par2Extension, Options{}, mf, false)

	require.ErrorContains(t, prog.packBundle(t.Context(), job), "malformed file")
}

// Expectation: The function should return an error when the PAR2 file has multiple sets.
func Test_Service_packBundle_MultipleSets_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{}},
					{MainPacket: &par2.MainPacket{}},
				},
			}, nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, par2er)

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	job := NewJob("/data/folder/test"+schema.Par2Extension, Options{}, mf, false)

	require.ErrorContains(t, prog.packBundle(t.Context(), job), "malformed file")
}

// Expectation: The function should return an error when the bundler Pack fails.
func Test_Service_packBundle_PackFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{SetID: setID}},
				},
			}, nil
		},
	}

	bndlr := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			return errors.New("pack failed")
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, par2er)

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	job := NewJob("/data/folder/test"+schema.Par2Extension, Options{}, mf, false)

	require.ErrorContains(t, prog.packBundle(t.Context(), job), "failed to pack bundle")
}

// Expectation: The manifest data passed to Pack should be valid JSON of the manifest.
func Test_Service_packBundle_ManifestContents_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{SetID: setID}},
				},
			}, nil
		},
	}

	var capturedManifest bundle.ManifestInput
	bndlr := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			capturedManifest = manifest

			require.NoError(t, afero.WriteFile(fsys, bundlePath, []byte("bundledata"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, par2er)

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	job := NewJob("/data/folder/test"+schema.Par2Extension, Options{}, mf, false)

	require.NoError(t, prog.packBundle(t.Context(), job))

	require.Equal(t, "test"+schema.Par2Extension+schema.ManifestExtension, capturedManifest.Name)

	var unmarshaled *schema.Manifest
	require.NoError(t, json.Unmarshal(capturedManifest.Bytes, &unmarshaled))
	require.Equal(t, "test"+schema.Par2Extension, unmarshaled.Name)
	require.Equal(t, mf, unmarshaled)
}

// Expectation: The file list passed to Pack should only contain matching PAR2 files.
func Test_Service_packBundle_CorrectFilesAndManifest_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol00+01"+schema.Par2Extension, []byte("vol1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol01+02"+schema.Par2Extension, []byte("vol2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/other"+schema.Par2Extension, []byte("unrelated"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{SetID: setID}},
				},
			}, nil
		},
	}

	var capturedFiles []bundle.FileInput
	bndlr := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			capturedFiles = files

			require.NoError(t, afero.WriteFile(fsys, bundlePath, []byte("bundledata"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, par2er)

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	job := NewJob("/data/folder/test"+schema.Par2Extension, Options{}, mf, false)

	require.NoError(t, prog.packBundle(t.Context(), job))

	require.Len(t, capturedFiles, 3)

	fileNames := make([]string, 0, len(capturedFiles))
	for _, f := range capturedFiles {
		fileNames = append(fileNames, f.Name)
		require.Equal(t, filepath.Join("/data/folder", f.Name), f.Path)
	}
	require.Contains(t, fileNames, "test"+schema.Par2Extension)
	require.Contains(t, fileNames, "test.vol00+01"+schema.Par2Extension)
	require.Contains(t, fileNames, "test.vol01+02"+schema.Par2Extension)
	require.NotContains(t, fileNames, "other"+schema.Par2Extension)
	require.NotContains(t, fileNames, "file.txt")
}

// Expectation: The function should warn but not error when cleanup of a PAR2 file fails after bundling.
func Test_Service_packBundle_CleanupPar2FileFails_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))
	require.NoError(t, afero.WriteFile(baseFs, "/data/folder/test.vol00+01"+schema.Par2Extension, []byte("vol1"), 0o644))

	fs := &testutil.FailingRemoveFs{Fs: baseFs, FailSuffix: "test.vol00+01" + schema.Par2Extension}

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{SetID: setID}},
				},
			}, nil
		},
	}

	bndlr := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			require.NoError(t, afero.WriteFile(baseFs, bundlePath, []byte("bundledata"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, par2er)

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	job := NewJob("/data/folder/test"+schema.Par2Extension, Options{}, mf, false)

	require.NoError(t, prog.packBundle(t.Context(), job))
	require.Contains(t, logBuf.String(), "Failed to cleanup a file after bundling")

	// The volume file that failed to remove should still exist.
	volExists, _ := afero.Exists(fs, "/data/folder/test.vol00+01"+schema.Par2Extension)
	require.True(t, volExists)

	// Bundle should still exist.
	bundleExists, _ := afero.Exists(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension)
	require.True(t, bundleExists)
}

// Expectation: The function should warn but not error when cleanup of the manifest file fails after bundling.
func Test_Service_packBundle_CleanupManifestFails_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))
	require.NoError(t, afero.WriteFile(baseFs, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension, []byte("{}"), 0o644))

	fs := &testutil.FailingRemoveFs{Fs: baseFs, FailSuffix: schema.ManifestExtension}

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{SetID: setID}},
				},
			}, nil
		},
	}

	bndlr := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			require.NoError(t, afero.WriteFile(baseFs, bundlePath, []byte("bundledata"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, par2er)

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	job := NewJob("/data/folder/test"+schema.Par2Extension, Options{}, mf, false)

	require.NoError(t, prog.packBundle(t.Context(), job))
	require.Contains(t, logBuf.String(), "Failed to cleanup a file after bundling")

	// The manifest file that failed to remove should still exist.
	manifestExists, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension)
	require.True(t, manifestExists)
}

// Expectation: The function should warn but not error when cleanup of the lock file fails after bundling.
func Test_Service_packBundle_CleanupLockFails_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))
	require.NoError(t, afero.WriteFile(baseFs, "/data/folder/test"+schema.Par2Extension+schema.LockExtension, []byte("lock"), 0o644))

	fs := &testutil.FailingRemoveFs{Fs: baseFs, FailSuffix: schema.LockExtension}

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{SetID: setID}},
				},
			}, nil
		},
	}

	bndlr := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			require.NoError(t, afero.WriteFile(baseFs, bundlePath, []byte("bundledata"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, par2er)

	mf := &schema.Manifest{Name: "test" + schema.Par2Extension}
	job := NewJob("/data/folder/test"+schema.Par2Extension, Options{}, mf, false)

	require.NoError(t, prog.packBundle(t.Context(), job))
	require.Contains(t, logBuf.String(), "Failed to cleanup a file after bundling")

	// The lock file that failed to remove should still exist.
	lockExists, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension+schema.LockExtension)
	require.True(t, lockExists)
}

// Expectation: The function should find bundle files and return them as jobs.
func Test_Service_unpackEnumerate_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.BundleExtension+schema.Par2Extension, []byte("bundledata"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	jobs, err := prog.unpackEnumerate(t.Context(), "/data", Options{})
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.True(t, jobs[0].isBundle)
	require.Contains(t, jobs[0].par2Path, schema.BundleExtension)
}

// Expectation: Non-bundle PAR2 files should be skipped during unpack enumeration.
func Test_Service_unpackEnumerate_SkipsNonBundles_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	jobs, err := prog.unpackEnumerate(t.Context(), "/data", Options{})
	require.NoError(t, err)
	require.Empty(t, jobs)
}

// Expectation: Elements in directories with an ignore file should be skipped.
func Test_Service_unpackEnumerate_IgnoreFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.BundleExtension+schema.Par2Extension, []byte("bundledata"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/"+schema.IgnoreFile, []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	jobs, err := prog.unpackEnumerate(t.Context(), "/data", Options{})
	require.NoError(t, err)
	require.Empty(t, jobs)
}

// Expectation: Context cancellation should be respected during unpack enumeration.
func Test_Service_unpackEnumerate_CtxCancel_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.BundleExtension+schema.Par2Extension, []byte("bundledata"), 0o644))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	_, err := prog.unpackEnumerate(ctx, "/data", Options{})
	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: An empty directory should return no jobs.
func Test_Service_unpackEnumerate_NoJobs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	jobs, err := prog.unpackEnumerate(t.Context(), "/data", Options{})
	require.NoError(t, err)
	require.Empty(t, jobs)
}

// Expectation: The function should unpack a bundle and remove the bundle file.
func Test_Service_unpackBundle_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, []byte("bundledata"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	mockBundle := &testutil.MockBundle{
		UnpackFunc: func(fsys afero.Fs, destDir string, strict bool) ([]string, error) {
			return []string{"/data/folder/test" + schema.Par2Extension}, nil
		},
		CloseFunc: func() error {
			return nil
		},
	}

	bndlr := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, &testutil.MockPar2Handler{})

	job := NewJob("/data/folder/test"+schema.BundleExtension+schema.Par2Extension, Options{}, nil, true)

	require.NoError(t, prog.unpackBundle(t.Context(), job))

	// Bundle file should be removed after unpacking.
	bundleExists, _ := afero.Exists(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension)
	require.False(t, bundleExists)
}

// Expectation: The function should return an error when opening the bundle fails.
func Test_Service_unpackBundle_OpenFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, []byte("bundledata"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	bndlr := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return nil, errors.New("corrupt file")
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, &testutil.MockPar2Handler{})

	job := NewJob("/data/folder/test"+schema.BundleExtension+schema.Par2Extension, Options{}, nil, true)

	require.ErrorContains(t, prog.unpackBundle(t.Context(), job), "failed to open bundle")
}

// Expectation: A rebuilt bundle without --force should return an error.
func Test_Service_unpackBundle_RebuiltNoForce_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, []byte("bundledata"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	mockBundle := &testutil.MockBundle{
		IsRebuiltValue: new(true),
		CloseFunc: func() error {
			return nil
		},
	}

	bndlr := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, &testutil.MockPar2Handler{})

	job := NewJob("/data/folder/test"+schema.BundleExtension+schema.Par2Extension, Options{Force: false}, nil, true)

	require.ErrorContains(t, prog.unpackBundle(t.Context(), job), "bundle is not guaranteed unpackable")
}

// Expectation: A rebuilt bundle with --force should proceed with unpacking.
func Test_Service_unpackBundle_RebuiltWithForce_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, []byte("bundledata"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	mockBundle := &testutil.MockBundle{
		IsRebuiltValue: new(true),
		UnpackFunc: func(fsys afero.Fs, destDir string, strict bool) ([]string, error) {
			return []string{"/data/folder/test" + schema.Par2Extension}, nil
		},
		CloseFunc: func() error {
			return nil
		},
	}

	bndlr := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, &testutil.MockPar2Handler{})

	job := NewJob("/data/folder/test"+schema.BundleExtension+schema.Par2Extension, Options{Force: true}, nil, true)

	require.NoError(t, prog.unpackBundle(t.Context(), job))
	require.Contains(t, logBuf.String(), "complete unpack cannot be guaranteed (but --force is set)")
}

// Expectation: Corrupt files during unpack without --force should return an error and clean up.
func Test_Service_unpackBundle_CorruptNoForce_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, []byte("bundledata"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	corruptFile := "/data/folder/test" + schema.Par2Extension
	require.NoError(t, afero.WriteFile(fs, corruptFile, []byte("corrupt"), 0o644))

	mockBundle := &testutil.MockBundle{
		UnpackFunc: func(fsys afero.Fs, destDir string, strict bool) ([]string, error) {
			return []string{corruptFile}, errors.Join(bundle.ErrDataCorrupt)
		},
		CloseFunc: func() error {
			return nil
		},
	}

	bndlr := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, &testutil.MockPar2Handler{})

	job := NewJob("/data/folder/test"+schema.BundleExtension+schema.Par2Extension, Options{Force: false}, nil, true)

	require.ErrorContains(t, prog.unpackBundle(t.Context(), job), "failed to unpack bundle")

	fileExists, _ := afero.Exists(fs, corruptFile)
	require.False(t, fileExists)
}

// Expectation: Corrupt files during unpack with --force should succeed.
func Test_Service_unpackBundle_CorruptWithForce_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, []byte("bundledata"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	corruptFile := "/data/folder/test" + schema.Par2Extension
	require.NoError(t, afero.WriteFile(fs, corruptFile, []byte("corrupt"), 0o644))

	mockBundle := &testutil.MockBundle{
		UnpackFunc: func(fsys afero.Fs, destDir string, strict bool) ([]string, error) {
			return []string{corruptFile}, errors.Join(bundle.ErrDataCorrupt)
		},
		CloseFunc: func() error {
			return nil
		},
	}

	bndlr := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, &testutil.MockPar2Handler{})

	job := NewJob("/data/folder/test"+schema.BundleExtension+schema.Par2Extension, Options{Force: true}, nil, true)

	require.NoError(t, prog.unpackBundle(t.Context(), job))
	require.Contains(t, logBuf.String(), "Some files in the bundle are corrupted (but --force is set)")

	fileExists, _ := afero.Exists(fs, corruptFile)
	require.True(t, fileExists)
}

// Expectation: Non-corrupt errors during unpack should return an error and clean up unpacked files.
func Test_Service_unpackBundle_NonCorruptError_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, []byte("bundledata"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	unpackedFile := "/data/folder/test" + schema.Par2Extension
	require.NoError(t, afero.WriteFile(fs, unpackedFile, []byte("data"), 0o644))

	mockBundle := &testutil.MockBundle{
		UnpackFunc: func(fsys afero.Fs, destDir string, strict bool) ([]string, error) {
			return []string{unpackedFile}, errors.New("I/O error")
		},
		CloseFunc: func() error {
			return nil
		},
	}

	bndlr := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, &testutil.MockPar2Handler{})

	job := NewJob("/data/folder/test"+schema.BundleExtension+schema.Par2Extension, Options{Force: true}, nil, true)

	require.ErrorContains(t, prog.unpackBundle(t.Context(), job), "failed to unpack bundle")

	// Unpacked files should be cleaned up on non-corrupt error.
	fileExists, _ := afero.Exists(fs, unpackedFile)
	require.False(t, fileExists)
}

// Expectation: Failed cleanup after unpack failure should be logged and not mask the unpack error.
func Test_Service_unpackBundle_FailedToCleanupAfterFailure_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, []byte("bundledata"), 0o644))

	cleanupPath := "/data/folder/cleanup" + schema.Par2Extension
	require.NoError(t, afero.WriteFile(baseFs, cleanupPath, []byte("data"), 0o644))

	fs := &testutil.FailingRemoveFs{Fs: baseFs, FailSuffix: "cleanup" + schema.Par2Extension}

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	mockBundle := &testutil.MockBundle{
		UnpackFunc: func(fsys afero.Fs, destDir string, strict bool) ([]string, error) {
			return []string{cleanupPath}, errors.New("I/O error")
		},
		CloseFunc: func() error {
			return nil
		},
	}

	bndlr := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, &testutil.MockPar2Handler{})

	job := NewJob("/data/folder/test"+schema.BundleExtension+schema.Par2Extension, Options{Force: true}, nil, true)

	require.ErrorContains(t, prog.unpackBundle(t.Context(), job), "failed to unpack bundle")
	require.Contains(t, logBuf.String(), "Failed to cleanup a file after failure")

	fileExists, _ := afero.Exists(fs, cleanupPath)
	require.True(t, fileExists)
}

// Expectation: The function should warn but not error when bundle cleanup fails after unpacking.
func Test_Service_unpackBundle_CleanupBundleFails_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, []byte("bundledata"), 0o644))

	fs := &testutil.FailingRemoveFs{Fs: baseFs, FailSuffix: schema.BundleExtension + schema.Par2Extension}

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	mockBundle := &testutil.MockBundle{
		UnpackFunc: func(fsys afero.Fs, destDir string, strict bool) ([]string, error) {
			return []string{"/data/folder/test" + schema.Par2Extension}, nil
		},
		CloseFunc: func() error {
			return nil
		},
	}

	bndlr := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), bndlr, &testutil.MockPar2Handler{})

	job := NewJob("/data/folder/test"+schema.BundleExtension+schema.Par2Extension, Options{}, nil, true)

	require.NoError(t, prog.unpackBundle(t.Context(), job))
	require.Contains(t, logBuf.String(), "Failed to cleanup bundle file after unpacking")

	// Bundle should still exist since removal failed.
	bundleExists, _ := afero.Exists(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension)
	require.True(t, bundleExists)
}

// Expectation: The function should return true when all errors match the sentinel.
func Test_onlyContains_AllMatch_Success(t *testing.T) {
	t.Parallel()

	err := errors.Join(bundle.ErrDataCorrupt, bundle.ErrDataCorrupt)
	require.True(t, onlyContains(err, bundle.ErrDataCorrupt))
}

// Expectation: The function should return false when some errors do not match the sentinel.
func Test_onlyContains_SomeDontMatch_Success(t *testing.T) {
	t.Parallel()

	err := errors.Join(bundle.ErrDataCorrupt, errors.New("other error"))
	require.False(t, onlyContains(err, bundle.ErrDataCorrupt))
}

// Expectation: The function should return false for non-joined errors.
func Test_onlyContains_NonJoined_Success(t *testing.T) {
	t.Parallel()

	err := errors.New("single error")
	require.False(t, onlyContains(err, bundle.ErrDataCorrupt))
}

// Expectation: The function should return true for non-joined errors of sentinel type.
func Test_onlyContains_NonJoined_Sentinel_Success(t *testing.T) {
	t.Parallel()

	err := bundle.ErrDataCorrupt
	require.True(t, onlyContains(err, bundle.ErrDataCorrupt))
}

// Expectation: The function should return true for a single joined error that matches.
func Test_onlyContains_SingleJoined_Success(t *testing.T) {
	t.Parallel()

	err := errors.Join(bundle.ErrDataCorrupt)
	require.True(t, onlyContains(err, bundle.ErrDataCorrupt))
}
