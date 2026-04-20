package create

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/flags"
	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: Validation should pass when mode is recursive with a shallow glob.
func Test_Options_Validate_RecursiveShallowGlob_Success(t *testing.T) {
	t.Parallel()

	opts := Options{Par2Glob: "*.mp4"}
	require.NoError(t, opts.Par2Mode.Set(schema.CreateRecursiveMode))

	require.NoError(t, opts.Validate())
}

// Expectation: Validation should pass when mode is not recursive with a deep glob.
func Test_Options_Validate_NonRecursiveDeepGlob_Success(t *testing.T) {
	t.Parallel()

	opts := Options{Par2Glob: "**/*.mp4"}
	require.NoError(t, opts.Par2Mode.Set(schema.CreateFileMode))

	require.NoError(t, opts.Validate())
}

// Expectation: Validation should fail when mode is recursive with a deep glob.
func Test_Options_Validate_RecursiveDeepGlob_Error(t *testing.T) {
	t.Parallel()

	opts := Options{Par2Glob: "**/*.mp4"}
	require.NoError(t, opts.Par2Mode.Set(schema.CreateRecursiveMode))

	require.ErrorIs(t, opts.Validate(), schema.ErrUnsupportedGlob)
}

// Expectation: Validation should fail when the glob pattern is invalid.
func Test_Options_Validate_InvalidGlobPattern_Error(t *testing.T) {
	t.Parallel()

	opts := Options{Par2Glob: "{unclosed"}
	require.NoError(t, opts.Par2Mode.Set(schema.CreateFolderMode))

	require.ErrorIs(t, opts.Validate(), doublestar.ErrBadPattern)
}

// Expectation: The correct paths should be derived from the [createConfig].
func Test_NewJob_Success(t *testing.T) {
	t.Parallel()

	cfg := MarkerConfig{
		Par2Mode:      &flags.CreateMode{Raw: schema.CreateFolderMode, Value: schema.CreateFolderMode},
		Par2Name:      new("test" + schema.Par2Extension),
		Par2Args:      &[]string{"-r10", "-n5"},
		Par2Glob:      new("*.txt"),
		Par2Verify:    new(true),
		HideFiles:     new(false),
		PersistMarker: new(false),
		Bundle:        new(false),
	}

	job := NewJob("/data/folder/_par2cron", cfg)

	require.Equal(t, "/data/folder", job.workingDir)
	require.Equal(t, "/data/folder/_par2cron", job.markerPath)
	require.Equal(t, schema.CreateFolderMode, job.par2Mode)
	require.Equal(t, "test"+schema.Par2Extension, job.par2Name)
	require.Equal(t, "/data/folder/test"+schema.Par2Extension, job.par2Path)
	require.Equal(t, []string{"-r10", "-n5"}, job.par2Args)
	require.Equal(t, "*.txt", job.par2Glob)
	require.Equal(t, "/data/folder/test"+schema.Par2Extension+schema.LockExtension, job.lockPath)
	require.Equal(t, "test"+schema.Par2Extension+schema.ManifestExtension, job.manifestName)
	require.Equal(t, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension, job.manifestPath)
	require.True(t, job.par2Verify)
	require.False(t, job.hiddenFiles)
	require.False(t, job.markerPersist)
	require.False(t, job.asBundle)
}

// Expectation: The correct paths should be derived from the [createConfig].
func Test_NewJob_HideFiles_Success(t *testing.T) {
	t.Parallel()

	cfg := MarkerConfig{
		Par2Mode:      &flags.CreateMode{Raw: schema.CreateFolderMode, Value: schema.CreateFolderMode},
		Par2Name:      new("test" + schema.Par2Extension),
		Par2Args:      &[]string{"-r10", "-n5"},
		Par2Glob:      new("*.txt"),
		Par2Verify:    new(true),
		HideFiles:     new(true),
		PersistMarker: new(true),
		Bundle:        new(true),
	}

	job := NewJob("/data/folder/_par2cron", cfg)

	require.Equal(t, "/data/folder", job.workingDir)
	require.Equal(t, "/data/folder/_par2cron", job.markerPath)
	require.Equal(t, schema.CreateFolderMode, job.par2Mode)
	require.Equal(t, ".test"+schema.Par2Extension, job.par2Name)
	require.Equal(t, "/data/folder/.test"+schema.Par2Extension, job.par2Path)
	require.Equal(t, []string{"-r10", "-n5"}, job.par2Args)
	require.Equal(t, "*.txt", job.par2Glob)
	require.Equal(t, "/data/folder/.test"+schema.Par2Extension+schema.LockExtension, job.lockPath)
	require.Equal(t, ".test"+schema.Par2Extension+schema.ManifestExtension, job.manifestName)
	require.Equal(t, "/data/folder/.test"+schema.Par2Extension+schema.ManifestExtension, job.manifestPath)
	require.True(t, job.par2Verify)
	require.True(t, job.hiddenFiles)
	require.True(t, job.markerPersist)
	require.True(t, job.asBundle)
}

// Expectation: The relevant fields should be changed for file mode, others not.
func Test_newFileModeJob_Success(t *testing.T) {
	t.Parallel()

	baseJob := Job{
		workingDir: "/data/folder",
		markerPath: "/data/folder/_par2cron",
		par2Mode:   schema.CreateFileMode,
		par2Args:   []string{"-r10", "-n5"},
		par2Glob:   "*.txt",
		par2Verify: true,
	}

	got := newFileModeJob(baseJob, "/data/folder/document.txt")

	require.Equal(t, "document.txt"+schema.Par2Extension, got.par2Name)
	require.Equal(t, "/data/folder/document.txt"+schema.Par2Extension, got.par2Path)
	require.Equal(t, "document.txt"+schema.Par2Extension+schema.ManifestExtension, got.manifestName)
	require.Equal(t, "/data/folder/document.txt"+schema.Par2Extension+schema.ManifestExtension, got.manifestPath)
	require.Equal(t, "/data/folder/document.txt"+schema.Par2Extension+schema.LockExtension, got.lockPath)
	require.Equal(t, "/data/folder", got.workingDir)
	require.Equal(t, "/data/folder/_par2cron", got.markerPath)
	require.Equal(t, schema.CreateFileMode, got.par2Mode)
	require.Equal(t, []string{"-r10", "-n5"}, got.par2Args)
	require.Equal(t, "*.txt", got.par2Glob)
	require.True(t, got.par2Verify)

	require.NotEqual(t, baseJob, got)
}

// Expectation: The relevant fields should be changed for file mode, others not.
func Test_newFileModeJob_HideFiles_Success(t *testing.T) {
	t.Parallel()

	baseJob := Job{
		workingDir: "/data/folder",
		markerPath: "/data/folder/_par2cron",
		par2Name:   ".folder" + schema.Par2Extension,
		par2Mode:   schema.CreateFileMode,
		par2Args:   []string{"-r10", "-n5"},
		par2Glob:   "*.txt",
		par2Verify: true,
	}

	got := newFileModeJob(baseJob, "/data/folder/document.txt")

	require.Equal(t, ".document.txt"+schema.Par2Extension, got.par2Name)
	require.Equal(t, "/data/folder/.document.txt"+schema.Par2Extension, got.par2Path)
	require.Equal(t, ".document.txt"+schema.Par2Extension+schema.ManifestExtension, got.manifestName)
	require.Equal(t, "/data/folder/.document.txt"+schema.Par2Extension+schema.ManifestExtension, got.manifestPath)
	require.Equal(t, "/data/folder/.document.txt"+schema.Par2Extension+schema.LockExtension, got.lockPath)
	require.Equal(t, "/data/folder", got.workingDir)
	require.Equal(t, "/data/folder/_par2cron", got.markerPath)
	require.Equal(t, schema.CreateFileMode, got.par2Mode)
	require.Equal(t, []string{"-r10", "-n5"}, got.par2Args)
	require.Equal(t, "*.txt", got.par2Glob)
	require.True(t, got.par2Verify)

	require.NotEqual(t, baseJob, got)
}

// Expectation: The relevant fields should be changed for nested mode, others not.
func Test_newNestedModeJob_Success(t *testing.T) {
	t.Parallel()

	baseJob := Job{
		workingDir: "/data/folder",
		markerPath: "/data/folder/_par2cron",
		par2Mode:   schema.CreateFileMode,
		par2Args:   []string{"-r10", "-n5"},
		par2Glob:   "*.txt",
		par2Verify: true,
	}

	got := newNestedModeJob(baseJob, "/data/folder/subdir")

	require.Equal(t, "subdir"+schema.Par2Extension, got.par2Name)
	require.Equal(t, "/data/folder/subdir/subdir"+schema.Par2Extension, got.par2Path)
	require.Equal(t, "subdir"+schema.Par2Extension+schema.ManifestExtension, got.manifestName)
	require.Equal(t, "/data/folder/subdir/subdir"+schema.Par2Extension+schema.ManifestExtension, got.manifestPath)
	require.Equal(t, "/data/folder/subdir/subdir"+schema.Par2Extension+schema.LockExtension, got.lockPath)
	require.Equal(t, "/data/folder/subdir", got.workingDir)
	require.Equal(t, "/data/folder/_par2cron", got.markerPath)
	require.Equal(t, schema.CreateFileMode, got.par2Mode)
	require.Equal(t, []string{"-r10", "-n5"}, got.par2Args)
	require.Equal(t, "*.txt", got.par2Glob)
	require.True(t, got.par2Verify)

	require.NotEqual(t, baseJob, got)
}

// Expectation: The relevant fields should be changed for nested mode, others not.
func Test_newNestedModeJob_HideFiles_Success(t *testing.T) {
	t.Parallel()

	baseJob := Job{
		workingDir: "/data/folder",
		markerPath: "/data/folder/_par2cron",
		par2Name:   ".folder" + schema.Par2Extension,
		par2Mode:   schema.CreateFileMode,
		par2Args:   []string{"-r10", "-n5"},
		par2Glob:   "*.txt",
		par2Verify: true,
	}

	got := newNestedModeJob(baseJob, "/data/folder/subdir")

	require.Equal(t, ".subdir"+schema.Par2Extension, got.par2Name)
	require.Equal(t, "/data/folder/subdir/.subdir"+schema.Par2Extension, got.par2Path)
	require.Equal(t, ".subdir"+schema.Par2Extension+schema.ManifestExtension, got.manifestName)
	require.Equal(t, "/data/folder/subdir/.subdir"+schema.Par2Extension+schema.ManifestExtension, got.manifestPath)
	require.Equal(t, "/data/folder/subdir/.subdir"+schema.Par2Extension+schema.LockExtension, got.lockPath)
	require.Equal(t, "/data/folder/subdir", got.workingDir)
	require.Equal(t, "/data/folder/_par2cron", got.markerPath)
	require.Equal(t, schema.CreateFileMode, got.par2Mode)
	require.Equal(t, []string{"-r10", "-n5"}, got.par2Args)
	require.Equal(t, "*.txt", got.par2Glob)
	require.True(t, got.par2Verify)

	require.NotEqual(t, baseJob, got)
}

// Expectation: The program should handle a single job that succeeds.
func Test_Service_Create_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called bool
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called = true
			require.NoError(t, afero.WriteFile(fs, "/data/folder/folder"+schema.Par2Extension, []byte("par2data"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	_, err := prog.Create(t.Context(), []string{"/data"}, args)
	require.NoError(t, err)

	require.True(t, called)
	require.Contains(t, logBuf.String(), "Job completed with success")
}

// Expectation: The program should handle multiple provided root directories.
func Test_Service_Create_MultiRoot_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, fs.MkdirAll("/data2/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data2/folder/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data2/folder/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++
			if called == 1 {
				require.NoError(t, afero.WriteFile(fs, "/data/folder/folder"+schema.Par2Extension, []byte("par2data"), 0o644))
			} else {
				require.NoError(t, afero.WriteFile(fs, "/data2/folder/folder"+schema.Par2Extension, []byte("par2data"), 0o644))
			}

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	_, err := prog.Create(t.Context(), []string{"/data", "/data2"}, args)
	require.NoError(t, err)

	require.Equal(t, 2, called)
	require.Contains(t, logBuf.String(), "Job completed with success")
}

// Expectation: The program should handle a single job that fails due to a file-locked situation.
func Test_Service_Create_FileLocked_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called bool
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called = true

			return schema.ErrFileIsLocked
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	_, err := prog.Create(t.Context(), []string{"/data"}, args)
	require.NoError(t, err)

	require.True(t, called)
	require.Contains(t, logBuf.String(), "Job unavailable (will retry next run)")
}

// Expectation: The program should handle a single job that fails.
func Test_Service_Create_Generic_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called bool
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called = true

			return errors.New("generic error")
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	_, err := prog.Create(t.Context(), []string{"/data"}, args)
	require.ErrorIs(t, err, schema.ErrExitPartialFailure)

	require.True(t, called)
	require.Contains(t, logBuf.String(), "Job failure (will retry next run)")
}

// Expectation: The program should handle multiple jobs that succeed.
func Test_Service_Create_MultipleJobs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, fs.MkdirAll("/data/folder2", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			if called == 1 {
				require.NoError(t, afero.WriteFile(fs, "/data/folder/folder"+schema.Par2Extension, []byte("par2data"), 0o644))
			} else {
				require.NoError(t, afero.WriteFile(fs, "/data/folder2/folder2"+schema.Par2Extension, []byte("par2data"), 0o644))
			}

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	_, err := prog.Create(t.Context(), []string{"/data"}, args)
	require.NoError(t, err)

	require.Equal(t, 2, called)
	require.Equal(t, 2, strings.Count(logBuf.String(), "Job completed with success"))
}

// Expectation: The program should continue if the first job fails.
func Test_Service_Create_MultipleJobs_OneFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, fs.MkdirAll("/data/folder2", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			if called == 1 {
				return testutil.CreateExitError(t, ctx, 5)
			} else {
				require.NoError(t, afero.WriteFile(fs, "/data/folder2/folder2"+schema.Par2Extension, []byte("par2data"), 0o644))
			}

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	_, err := prog.Create(t.Context(), []string{"/data"}, args)
	require.ErrorIs(t, err, schema.ErrExitPartialFailure)

	require.Equal(t, 2, called)
	require.Equal(t, 1, strings.Count(logBuf.String(), "Job failure (will retry next run)"))
	require.Equal(t, 1, strings.Count(logBuf.String(), "Job completed with success"))
}

// Expectation: The program should continue if an enumeration partial (non-fatal) failure occurs.
// Eventually though, an error must be returned so the user knows something went wrong (non-zero exit code).
func Test_Service_Create_MultipleJobs_EnumerationFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/"+createMarkerPathPrefix, []byte("invalid yaml"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, fs.MkdirAll("/data/folder2", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			require.NoError(t, afero.WriteFile(fs, "/data/folder2/folder2"+schema.Par2Extension, []byte("par2data"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})
	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}

	_, err := prog.Create(t.Context(), []string{"/data"}, args)
	require.ErrorIs(t, err, schema.ErrExitPartialFailure)
	require.ErrorIs(t, err, schema.ErrNonFatal)

	require.Equal(t, 1, called)
	require.Equal(t, 1, strings.Count(logBuf.String(), "Job completed with success"))
	require.Equal(t, 1, strings.Count(logBuf.String(), "marker file could not be parsed"))
}

// Expectation: The program should continue if an enumeration partial (non-fatal) failure occurs.
// Eventually though, an error must be returned so the user knows something went wrong (non-zero exit code).
func Test_Service_Create_MultipleJobs_EnumerationFails_NoOtherJobs_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/"+createMarkerPathPrefix, []byte("invalid yaml"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})
	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}

	_, err := prog.Create(t.Context(), []string{"/data"}, args)
	require.ErrorIs(t, err, schema.ErrExitPartialFailure)
	require.ErrorIs(t, err, schema.ErrNonFatal)

	require.Equal(t, 0, called)
	require.Equal(t, 1, strings.Count(logBuf.String(), "Nothing to do"))
	require.Equal(t, 1, strings.Count(logBuf.String(), "marker file could not be parsed"))
}

// Expectation: The program should output that there's nothing to do.
func Test_Service_Create_NoJobs_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	args := Options{Par2Args: []string{"-r10"}}
	_, err := prog.Create(t.Context(), []string{"/data"}, args)
	require.NoError(t, err)
	require.Contains(t, logBuf.String(), "Nothing to do")
}

// Expectation: The program should respect cancellation.
func Test_Service_Create_CtxCancel_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	args := Options{Par2Args: []string{"-r10"}}
	_, err := prog.Create(ctx, []string{"/data"}, args)

	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: The program should skip remaining jobs when --duration is exceeded.
func Test_Service_Create_DurationExceeded_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder1", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder1/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder1/file.txt", []byte("content"), 0o644))
	require.NoError(t, fs.MkdirAll("/data/folder2", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			time.Sleep(50 * time.Millisecond)

			require.NoError(t, afero.WriteFile(fs, "/data/folder1/folder1"+schema.Par2Extension, []byte("par2data"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})
	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	require.NoError(t, args.MaxDuration.Set("10ms"))

	_, err := prog.Create(t.Context(), []string{"/data"}, args)
	require.NoError(t, err)

	require.Equal(t, 1, called)
	require.Equal(t, 1, strings.Count(logBuf.String(), "Job completed with success"))
	require.Contains(t, logBuf.String(), "Exceeded the --duration budget")
	require.Contains(t, logBuf.String(), "unprocessedJobs")
}

// Expectation: The program should process all jobs when --duration is not exceeded.
func Test_Service_Create_DurationNotExceeded_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder1", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder1/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder1/file.txt", []byte("content"), 0o644))
	require.NoError(t, fs.MkdirAll("/data/folder2", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++
			if called == 1 {
				require.NoError(t, afero.WriteFile(fs, "/data/folder1/folder1"+schema.Par2Extension, []byte("par2data"), 0o644))
			} else {
				require.NoError(t, afero.WriteFile(fs, "/data/folder2/folder2"+schema.Par2Extension, []byte("par2data"), 0o644))
			}

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})
	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	require.NoError(t, args.MaxDuration.Set("10s"))

	_, err := prog.Create(t.Context(), []string{"/data"}, args)
	require.NoError(t, err)

	require.Equal(t, 2, called)
	require.Equal(t, 2, strings.Count(logBuf.String(), "Job completed with success"))
	require.NotContains(t, logBuf.String(), "Exceeded the --duration budget")
}

// Expectation: The program should return accumulated errors even when --duration is exceeded.
func Test_Service_Create_DurationExceeded_WithPriorError_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder1", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder1/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder1/file.txt", []byte("content"), 0o644))
	require.NoError(t, fs.MkdirAll("/data/folder2", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			time.Sleep(50 * time.Millisecond)

			return errors.New("job failed")
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})
	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	require.NoError(t, args.MaxDuration.Set("10ms"))

	_, err := prog.Create(t.Context(), []string{"/data"}, args)
	require.ErrorIs(t, err, schema.ErrExitPartialFailure)

	require.Equal(t, 1, called)
	require.Contains(t, logBuf.String(), "Exceeded the --duration budget")
	require.Contains(t, logBuf.String(), "Job failure")
}

// Expectation: The program should process all jobs when no --duration is set.
func Test_Service_Create_NoDuration_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder1", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder1/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder1/file.txt", []byte("content"), 0o644))
	require.NoError(t, fs.MkdirAll("/data/folder2", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++
			if called == 1 {
				require.NoError(t, afero.WriteFile(fs, "/data/folder1/folder1"+schema.Par2Extension, []byte("par2data"), 0o644))
			} else {
				require.NoError(t, afero.WriteFile(fs, "/data/folder2/folder2"+schema.Par2Extension, []byte("par2data"), 0o644))
			}

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})
	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}

	_, err := prog.Create(t.Context(), []string{"/data"}, args)
	require.NoError(t, err)

	require.Equal(t, 2, called)
	require.Equal(t, 2, strings.Count(logBuf.String(), "Job completed with success"))
}

// Expectation: The program should return a bad invocation error when -R is used without recursive mode.
func Test_Service_Create_RecursiveArgWithoutRecursiveMode_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	args := Options{Par2Args: []string{"-r10", "-R"}, Par2Glob: "*"}
	require.NoError(t, args.Par2Mode.Set(schema.CreateFileMode))

	_, err := prog.Create(t.Context(), []string{"/data"}, args)

	require.ErrorIs(t, err, schema.ErrExitBadInvocation)
	require.ErrorIs(t, err, errWrongModeArgument)
	require.Contains(t, logBuf.String(), "par2 default argument -R needs par2cron default --mode recursive")
}

// Expectation: The function should correctly find multiple marker files.
func Test_Service_Enumerate_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder1", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder2", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder1/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/"+createMarkerPathPrefix, []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	args := Options{Par2Args: []string{"-r10"}}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 2)
}

// Expectation: The function should respect the ignore file rules.
func Test_Service_Enumerate_IgnoreFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder1", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder2", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder1/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/"+schema.IgnoreFile, []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	args := Options{Par2Args: []string{"-r10"}}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Contains(t, jobs[0].par2Path, "folder1")
}

// Expectation: The function should respect the ignore file rules.
func Test_Service_Enumerate_IgnoreFileAll_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder1", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder2", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder2/folder3", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder1/"+createMarkerPathPrefix, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/"+schema.IgnoreAllFile, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/folder3/"+createMarkerPathPrefix, []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	args := Options{Par2Args: []string{"-r10"}}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Contains(t, jobs[0].par2Path, "folder1")
}

// Expectation: The function should still return the successful jobs for a partial failure.
func Test_Service_Enumerate_PartialError_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder1", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder2", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder1/"+createMarkerPathPrefix, []byte("invalid yaml"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/"+createMarkerPathPrefix, []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})
	args := Options{Par2Args: []string{"-r10"}}

	jobs, err := prog.Enumerate(t.Context(), "/data", args)
	require.ErrorIs(t, err, schema.ErrNonFatal)

	require.NotNil(t, jobs)
	require.Len(t, jobs, 1)
}

// Expectation: The function should not error when no creation jobs are found.
func Test_Service_Enumerate_NoMarkers_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	args := Options{Par2Args: []string{"-r10"}}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Empty(t, jobs)
}

// Expectation: The function should respect a cancellation and return the correct error.
func Test_Service_Enumerate_CtxCancel_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	args := Options{Par2Args: []string{"-r10"}}
	_, err := prog.Enumerate(ctx, "/data", args)

	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: The creation mode "folder" should be respected.
func Test_Service_createPar2_FolderMode_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file2.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	require.NoError(t, prog.createPar2(t.Context(), job))
	require.Equal(t, 1, called)

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.False(t, markerExists)
}

// Expectation: The "persist" option of a marker configuration should be respected.
func Test_Service_createPar2_FolderMode_PersistMarker_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file2.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:    "/data/folder",
		markerPath:    "/data/folder/_par2cron",
		par2Mode:      schema.CreateFolderMode,
		par2Name:      "folder" + schema.Par2Extension,
		par2Path:      "/data/folder/folder" + schema.Par2Extension,
		par2Args:      []string{"-r10"},
		par2Glob:      "*",
		lockPath:      "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName:  "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath:  "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
		markerPersist: true,
	}

	require.NoError(t, prog.createPar2(t.Context(), job))
	require.Equal(t, 1, called)

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.True(t, markerExists)
}

// Expectation: A deep glob pattern in folder mode should match files in
// subdirectories but create a single par2 set in the marker-containing directory.
func Test_Service_createPar2_FolderMode_DeepGlob_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder/sub", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/sub/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/sub/other.bin", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	var workingDirs []string
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++
			workingDirs = append(workingDirs, workingDir)
			capturedArgs = append(capturedArgs, args...)

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "**/*.txt",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	require.NoError(t, prog.createPar2(t.Context(), job))
	require.Len(t, workingDirs, 1)
	require.Equal(t, "/data/folder", workingDirs[0])

	require.Equal(t, 1, called)
	require.Contains(t, capturedArgs, "/data/folder/file.txt")
	require.Contains(t, capturedArgs, "/data/folder/sub/file.txt")
	require.NotContains(t, capturedArgs, "/data/folder/sub/other.bin")

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.False(t, markerExists)
}

// Expectation: The creation mode "folder" should handle no files to protect.
func Test_Service_createPar2_FolderMode_NoFilesToProtect_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	require.ErrorIs(t, prog.createPar2(t.Context(), job), errNoFilesToProtect)
	require.Equal(t, 0, called)

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.True(t, markerExists)
}

// Expectation: The creation mode "folder" should handle creation failure.
func Test_Service_createPar2_FolderMode_CreateFailure_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file2.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			return testutil.CreateExitError(t, ctx, 5)
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	require.Error(t, prog.createPar2(t.Context(), job))
	require.Equal(t, 1, called)

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.True(t, markerExists)
}

// Expectation: The creation mode "nested" should be respected.
func Test_Service_createPar2_NestedMode_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder/a", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder/b", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/a/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/b/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	var workingDirs []string
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++
			workingDirs = append(workingDirs, workingDir)

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateNestedMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "**/*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	require.NoError(t, prog.createPar2(t.Context(), job))
	require.Equal(t, 2, called)
	require.Contains(t, workingDirs, "/data/folder/a")
	require.Contains(t, workingDirs, "/data/folder/b")

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.False(t, markerExists)
}

// Expectation: Nested mode with a simple (non-deep) glob should only
// include files in the marker-containing directory itself.
func Test_Service_createPar2_NestedMode_ShallowGlob_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder/a", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/a/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	var workingDirs []string
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++
			workingDirs = append(workingDirs, workingDir)
			capturedArgs = append(capturedArgs, args...)

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateNestedMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	require.NoError(t, prog.createPar2(t.Context(), job))
	require.Equal(t, 1, called)
	require.Equal(t, []string{"/data/folder"}, workingDirs)

	require.Contains(t, capturedArgs, "/data/folder/file.txt")
	require.NotContains(t, capturedArgs, "/data/folder/a/file.txt")

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.False(t, markerExists)
}

// Expectation: The "persist" option in nested mode should keep the marker file.
func Test_Service_createPar2_NestedMode_PersistMarker_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder/a", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder/b", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/a/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/b/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:    "/data/folder",
		markerPath:    "/data/folder/_par2cron",
		par2Mode:      schema.CreateNestedMode,
		par2Name:      "folder" + schema.Par2Extension,
		par2Path:      "/data/folder/folder" + schema.Par2Extension,
		par2Args:      []string{"-r10"},
		par2Glob:      "**/*",
		lockPath:      "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName:  "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath:  "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
		markerPersist: true,
	}

	require.NoError(t, prog.createPar2(t.Context(), job))
	require.Equal(t, 2, called)

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.True(t, markerExists)
}

// Expectation: Nested mode with a deep glob should group files by their
// containing folder and create one PAR2 set per folder with matches.
func Test_Service_createPar2_NestedMode_DeepGlob_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder/a", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder/b", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/a/movie.mkv", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/a/subs.srt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/b/movie.mkv", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/b/other.bin", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	var workingDirs []string
	var capturedArgs [][]string
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++
			workingDirs = append(workingDirs, workingDir)
			capturedArgs = append(capturedArgs, args)

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateNestedMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "**/*.mkv",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	require.NoError(t, prog.createPar2(t.Context(), job))
	require.Equal(t, 2, called)
	require.Contains(t, workingDirs, "/data/folder/a")
	require.Contains(t, workingDirs, "/data/folder/b")

	allArgs := []string{}
	for _, args := range capturedArgs {
		allArgs = append(allArgs, args...)
	}
	require.Contains(t, allArgs, "/data/folder/a/movie.mkv")
	require.Contains(t, allArgs, "/data/folder/b/movie.mkv")
	require.NotContains(t, allArgs, "/data/folder/a/subs.srt")
	require.NotContains(t, allArgs, "/data/folder/b/other.bin")

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.False(t, markerExists)
}

// Expectation: Nested mode should include deeply nested folders.
func Test_Service_createPar2_NestedMode_DeeplyNested_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder/a/extras", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder/b", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/a/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/a/extras/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/b/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	var workingDirs []string
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++
			workingDirs = append(workingDirs, workingDir)

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateNestedMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "**/*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	require.NoError(t, prog.createPar2(t.Context(), job))
	require.Equal(t, 3, called)
	require.Contains(t, workingDirs, "/data/folder/a")
	require.Contains(t, workingDirs, "/data/folder/a/extras")
	require.Contains(t, workingDirs, "/data/folder/b")

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.False(t, markerExists)
}

// Expectation: The creation mode "nested" should handle no files to protect.
func Test_Service_createPar2_NestedMode_NoFilesToProtect_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateNestedMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "**/*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	require.ErrorIs(t, prog.createPar2(t.Context(), job), errNoFilesToProtect)
	require.Equal(t, 0, called)

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.True(t, markerExists)
}

// Expectation: The creation mode "nested" should handle creation failure.
func Test_Service_createPar2_NestedMode_CreateFailure_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder/a", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder/b", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/a/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/b/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			return testutil.CreateExitError(t, ctx, 5)
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateNestedMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "**/*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	require.ErrorIs(t, prog.createPar2(t.Context(), job), errSubjobFailure)
	require.Equal(t, 2, called)

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.True(t, markerExists)
}

// Expectation: The creation mode "file" should be respected.
func Test_Service_createPar2_FileMode_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file2.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFileMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	require.NoError(t, prog.createPar2(t.Context(), job))
	require.Equal(t, 2, called)

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.False(t, markerExists)
}

// Expectation: A deep glob pattern in file mode should match files in
// subdirectories and create the par2 in the respective subdirectories.
func Test_Service_createPar2_FileMode_DeepGlob_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder/sub", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/sub/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/sub/other.bin", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	var workingDirs []string
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++
			workingDirs = append(workingDirs, workingDir)
			capturedArgs = append(capturedArgs, args...)

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFileMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "**/*.txt",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	require.NoError(t, prog.createPar2(t.Context(), job))
	require.Len(t, workingDirs, 2)
	require.Contains(t, workingDirs, "/data/folder")
	require.Contains(t, workingDirs, "/data/folder/sub")

	require.Equal(t, 2, called)
	require.Contains(t, capturedArgs, "/data/folder/file.txt")
	require.Contains(t, capturedArgs, "/data/folder/sub/file.txt")
	require.NotContains(t, capturedArgs, "/data/folder/sub/other.bin")

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.False(t, markerExists)
}

// Expectation: The creation mode "file" should handle no files to protect.
func Test_Service_createPar2_FileMode_NoFilesToProtect_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFileMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	require.ErrorIs(t, prog.createPar2(t.Context(), job), errNoFilesToProtect)
	require.Equal(t, 0, called)

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.True(t, markerExists)
}

// Expectation: The creation mode "file" should handle creation failure.
func Test_Service_createPar2_FileMode_CreateFailure_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file2.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			return testutil.CreateExitError(t, ctx, 5)
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFileMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	require.ErrorIs(t, prog.createPar2(t.Context(), job), errSubjobFailure)
	require.Equal(t, 2, called)

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.True(t, markerExists)
}

// Expectation: The function should error if the marker cannot be removed.
func Test_Service_createPar2_CannotRemoveMarker_Error(t *testing.T) {
	t.Parallel()

	fs := &testutil.FailingRemoveFs{Fs: afero.NewMemMapFs(), FailSuffix: createMarkerPathPrefix}

	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file2.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	require.ErrorContains(t, prog.createPar2(t.Context(), job), "failed to delete marker file")
	require.Equal(t, 1, called)

	markerExists, _ := afero.Exists(fs, "/data/folder/_par2cron")
	require.True(t, markerExists)
}

// The function should skip irrelevant files and return the correct files.
func Test_Service_findElementsToProtect_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/existing"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/existing.vol25+22.PAR2", []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/manifest"+schema.Par2Extension+schema.ManifestExtension, []byte("{}"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/manifest"+schema.Par2Extension+schema.LockExtension, []byte("{}"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	files, err := prog.findElementsToProtect(t.Context(), job)

	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "/data/folder/file.txt", files[0].Path)
	require.Equal(t, "file.txt", files[0].Name)
}

// Expectation: A deep glob should preserve the relative path in the element name in folder mode.
func Test_Service_findElementsToProtect_DeepGlobRelativeName_FolderMode_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder/subfolder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/subfolder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/subfolder/ignore.log", []byte("log"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/root.txt", []byte("root"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "**/*.txt",
		par2Mode:     schema.CreateFolderMode,
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	files, err := prog.findElementsToProtect(t.Context(), job)
	require.NoError(t, err)
	require.Len(t, files, 2)

	// Root-level file: Name is just the filename.
	require.Equal(t, "/data/folder/root.txt", files[0].Path)
	require.Equal(t, "root.txt", files[0].Name)

	// Nested file: Name preserves the relative path from workingDir.
	require.Equal(t, "/data/folder/subfolder/file.txt", files[1].Path)
	require.Equal(t, "subfolder/file.txt", files[1].Name)
}

// Expectation: A deep glob should not preserve the relative path in the element name in file mode.
func Test_Service_findElementsToProtect_DeepGlobRelativeName_FileMode_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder/subfolder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/subfolder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/subfolder/ignore.log", []byte("log"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/root.txt", []byte("root"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "**/*.txt",
		par2Mode:     schema.CreateFileMode,
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	files, err := prog.findElementsToProtect(t.Context(), job)
	require.NoError(t, err)
	require.Len(t, files, 2)

	// Root-level file: Name is just the filename.
	require.Equal(t, "/data/folder/root.txt", files[0].Path)
	require.Equal(t, "root.txt", files[0].Name)

	// Nested file: Name is just the filename.
	require.Equal(t, "/data/folder/subfolder/file.txt", files[1].Path)
	require.Equal(t, "file.txt", files[1].Name)
}

// Expectation: The function should include directories and .par2 files in recursive mode.
func Test_Service_findElementsToProtect_RecursiveMode_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder/subfolder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/existing"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/existing.vol25+22.PAR2", []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/manifest"+schema.Par2Extension+schema.ManifestExtension, []byte("{}"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/manifest"+schema.Par2Extension+schema.LockExtension, []byte("{}"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateRecursiveMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10", "-R"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	files, err := prog.findElementsToProtect(t.Context(), job)

	require.NoError(t, err)
	require.Len(t, files, 6)

	names := make([]string, 0, 6)
	var dirCount int
	for _, f := range files {
		names = append(names, f.Name)
		if f.IsDir {
			dirCount++
		}
	}

	// In recursive mode, directories are included
	require.Contains(t, names, "subfolder")
	require.Equal(t, 1, dirCount)

	// In recursive mode, .par2 files are included (par2cmdline -R includes them)
	require.Contains(t, names, "existing"+schema.Par2Extension)
	require.Contains(t, names, "existing.vol25+22.PAR2")
	require.Contains(t, names, "manifest"+schema.Par2Extension+schema.ManifestExtension)
	require.Contains(t, names, "manifest"+schema.Par2Extension+schema.LockExtension)

	// Regular files are included
	require.Contains(t, names, "file.txt")

	// Marker file is always excluded
	require.NotContains(t, names, "_par2cron")
}

// Expectation: The function should only list files and folders in the immediate directory.
func Test_Service_findElementsToProtect_RecursiveMode_NoDeepRecursion_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder/subfolder", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder/subfolder/nested", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/subfolder/subfile.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/subfolder/nested/deepfile.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateRecursiveMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10", "-R"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	files, err := prog.findElementsToProtect(t.Context(), job)

	require.NoError(t, err)
	require.Len(t, files, 2)

	names := make([]string, 0, 2)
	for _, f := range files {
		names = append(names, f.Name)
	}

	// Only immediate children should be listed
	require.Contains(t, names, "file.txt")
	require.Contains(t, names, "subfolder")

	// Files in subdirectories should not be listed (par2cmdline handles that)
	require.NotContains(t, names, "subfile.txt")
	require.NotContains(t, names, "nested")
	require.NotContains(t, names, "deepfile.txt")
}

// Expectation: The function should not break with globbing meta characters in the working directory path.
func Test_Service_findElementsToProtect_GlobMetacharsInWorkingDirPath_Table(t *testing.T) {
	t.Parallel()

	type expectedFile struct {
		Name string
		Path string
	}

	tests := []struct {
		name     string
		dirs     []string
		files    map[string]string // map[path]content
		job      Job
		expected []expectedFile
	}{
		{
			name: "brackets in working directory",
			dirs: []string{"/data/project[1]"},
			files: map[string]string{
				"/data/project[1]/file.txt":  "content",
				"/data/project[1]/_par2cron": "",
			},
			job: Job{
				workingDir: "/data/project[1]",
				markerPath: "/data/project[1]/_par2cron",
				par2Mode:   schema.CreateFileMode,
				par2Glob:   "*",
			},
			expected: []expectedFile{
				{Name: "file.txt", Path: "/data/project[1]/file.txt"},
			},
		},
		{
			name: "asterisk in working directory",
			dirs: []string{"/data/my*folder"},
			files: map[string]string{
				"/data/my*folder/file.txt":  "content",
				"/data/my*folder/_par2cron": "",
			},
			job: Job{
				workingDir: "/data/my*folder",
				markerPath: "/data/my*folder/_par2cron",
				par2Mode:   schema.CreateFolderMode,
				par2Glob:   "*",
			},
			expected: []expectedFile{
				{Name: "file.txt", Path: "/data/my*folder/file.txt"},
			},
		},
		{
			name: "question mark in working directory",
			dirs: []string{"/data/what?"},
			files: map[string]string{
				"/data/what?/file.txt":  "content",
				"/data/what?/_par2cron": "",
			},
			job: Job{
				workingDir: "/data/what?",
				markerPath: "/data/what?/_par2cron",
				par2Mode:   schema.CreateFolderMode,
				par2Glob:   "*",
			},
			expected: []expectedFile{
				{Name: "file.txt", Path: "/data/what?/file.txt"},
			},
		},
		{
			name: "curly braces in working directory",
			dirs: []string{"/data/{project}"},
			files: map[string]string{
				"/data/{project}/file.txt":  "content",
				"/data/{project}/_par2cron": "",
			},
			job: Job{
				workingDir: "/data/{project}",
				markerPath: "/data/{project}/_par2cron",
				par2Mode:   schema.CreateFolderMode,
				par2Glob:   "*",
			},
			expected: []expectedFile{
				{Name: "file.txt", Path: "/data/{project}/file.txt"},
			},
		},
		{
			name: "multiple metacharacters in working directory",
			dirs: []string{"/data/proj[1]*{a}?"},
			files: map[string]string{
				"/data/proj[1]*{a}?/file.txt":  "content",
				"/data/proj[1]*{a}?/_par2cron": "",
			},
			job: Job{
				workingDir: "/data/proj[1]*{a}?",
				markerPath: "/data/proj[1]*{a}?/_par2cron",
				par2Mode:   schema.CreateFolderMode,
				par2Glob:   "*",
			},
			expected: []expectedFile{
				{Name: "file.txt", Path: "/data/proj[1]*{a}?/file.txt"},
			},
		},
		{
			name: "brackets in working directory with deep glob pattern",
			dirs: []string{"/data/project[1]", "/data/project[1]/subdir"},
			files: map[string]string{
				"/data/project[1]/file.txt":        "content",
				"/data/project[1]/file.go":         "content",
				"/data/project[1]/subdir/deep.txt": "content",
				"/data/project[1]/subdir/deep.go":  "content",
				"/data/project[1]/_par2cron":       "",
			},
			job: Job{
				workingDir: "/data/project[1]",
				markerPath: "/data/project[1]/_par2cron",
				par2Mode:   schema.CreateFolderMode,
				par2Glob:   "**/*.txt",
			},
			expected: []expectedFile{
				{Name: "file.txt", Path: "/data/project[1]/file.txt"},
				{Name: "subdir/deep.txt", Path: "/data/project[1]/subdir/deep.txt"},
			},
		},
		{
			name: "asterisk in working directory with deep glob pattern",
			dirs: []string{"/data/my*folder", "/data/my*folder/sub"},
			files: map[string]string{
				"/data/my*folder/a.txt":     "content",
				"/data/my*folder/b.go":      "content",
				"/data/my*folder/sub/c.txt": "content",
				"/data/my*folder/_par2cron": "",
			},
			job: Job{
				workingDir: "/data/my*folder",
				markerPath: "/data/my*folder/_par2cron",
				par2Mode:   schema.CreateFolderMode,
				par2Glob:   "**/*.txt",
			},
			expected: []expectedFile{
				{Name: "a.txt", Path: "/data/my*folder/a.txt"},
				{Name: "sub/c.txt", Path: "/data/my*folder/sub/c.txt"},
			},
		},
		{
			name: "folder mode relative names are clean with metacharacters in path",
			dirs: []string{"/data/{proj}[2]", "/data/{proj}[2]/subdir"},
			files: map[string]string{
				"/data/{proj}[2]/file.txt":        "content",
				"/data/{proj}[2]/subdir/deep.txt": "content",
				"/data/{proj}[2]/_par2cron":       "",
			},
			job: Job{
				workingDir: "/data/{proj}[2]",
				markerPath: "/data/{proj}[2]/_par2cron",
				par2Mode:   schema.CreateFolderMode,
				par2Glob:   "**",
			},
			expected: []expectedFile{
				{Name: "file.txt", Path: "/data/{proj}[2]/file.txt"},
				{Name: "subdir/deep.txt", Path: "/data/{proj}[2]/subdir/deep.txt"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fs := afero.NewMemMapFs()
			for _, dir := range tt.dirs {
				require.NoError(t, fs.MkdirAll(dir, 0o755))
			}
			for path, content := range tt.files {
				require.NoError(t, afero.WriteFile(fs, path, []byte(content), 0o644))
			}

			var logBuf testutil.SafeBuffer
			ls := logging.Options{
				Logout: &logBuf,
				Stdout: io.Discard,
				Stderr: io.Discard,
			}
			_ = ls.LogLevel.Set("info")

			prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

			files, err := prog.findElementsToProtect(t.Context(), &tt.job)
			require.NoError(t, err)
			require.Len(t, files, len(tt.expected))

			actual := make(map[string]string, len(files)) // map[name]path
			for _, f := range files {
				actual[f.Name] = f.Path

				require.NotContains(t, f.Path, "\\", "path %q contains escape backslash", f.Path)
				require.NotContains(t, f.Name, "\\", "name %q contains escape backslash", f.Name)
			}
			for _, exp := range tt.expected {
				path, ok := actual[exp.Name]

				require.True(t, ok, "expected file %q not found in results", exp.Name)
				require.Equal(t, exp.Path, path, "path mismatch for %q", exp.Name)
			}
		})
	}
}

// Expectation: The function should return the correct error when there's nothing to do.
func Test_Service_findElementsToProtect_NoFilesToProtect_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	_, err := prog.findElementsToProtect(t.Context(), job)

	require.ErrorIs(t, err, errNoFilesToProtect)
	require.Contains(t, logBuf.String(), "No files to protect")
}

// Expectation: Function should reject deep (/) glob patterns in recursive creation mode.
func Test_Service_findElementsToProtect_DeepGlobInRecursiveMode_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir: "/data/folder",
		markerPath: "/data/folder/_par2cron",
		par2Glob:   "**/*.txt",
		par2Mode:   schema.CreateRecursiveMode,
	}

	files, err := prog.findElementsToProtect(t.Context(), job)

	require.Nil(t, files)
	require.ErrorIs(t, err, schema.ErrUnsupportedGlob)
}

// Expectation: Function should reject glob patterns where the static prefix contains a symlink.
func Test_Service_findElementsToProtect_SymlinkInGlobPrefix_Error(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	target := filepath.Join(root, "real_folder")
	require.NoError(t, os.MkdirAll(filepath.Join(target, "sub"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(target, "sub", "file.txt"), []byte("content"), 0o600))

	workingDir := filepath.Join(root, "project")
	require.NoError(t, os.MkdirAll(workingDir, 0o750))
	require.NoError(t, os.Symlink(target, filepath.Join(workingDir, "link")))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "_par2cron"), []byte(""), 0o600))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(afero.NewOsFs(), logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir: workingDir,
		markerPath: filepath.Join(workingDir, "_par2cron"),
		par2Mode:   schema.CreateFolderMode,
		par2Glob:   "link/sub/*.txt",
	}

	files, err := prog.findElementsToProtect(t.Context(), job)

	require.Nil(t, files)
	require.ErrorIs(t, err, schema.ErrUnsupportedGlob)
	require.Contains(t, logBuf.String(), "symbolic link")
}

// Expectation: Symlinks found during globbing should be skipped with a warning, not cause an error.
func Test_Service_findElementsToProtect_SymlinkInGlobResults_Success(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	workingDir := filepath.Join(root, "project")
	require.NoError(t, os.MkdirAll(workingDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "real.txt"), []byte("content"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "_par2cron"), []byte(""), 0o600))

	target := filepath.Join(root, "other.txt")
	require.NoError(t, os.WriteFile(target, []byte("other"), 0o600))
	require.NoError(t, os.Symlink(target, filepath.Join(workingDir, "symlinked.txt")))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(afero.NewOsFs(), logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir: workingDir,
		markerPath: filepath.Join(workingDir, "_par2cron"),
		par2Mode:   schema.CreateFileMode,
		par2Glob:   "*.txt",
	}

	files, err := prog.findElementsToProtect(t.Context(), job)

	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, filepath.Join(workingDir, "real.txt"), files[0].Path)
	require.Equal(t, "real.txt", files[0].Name)

	require.Contains(t, logBuf.String(), "symbolic link was skipped")
}

// Expectation: WithNoFollow should not traverse into symlinked directories during deep glob expansion.
func Test_Service_findElementsToProtect_SymlinkDir_NoFollow_Success(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	workingDir := filepath.Join(root, "project")
	require.NoError(t, os.MkdirAll(filepath.Join(workingDir, "realdir"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "realdir", "real.txt"), []byte("content"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "_par2cron"), []byte(""), 0o600))

	// Create a symlinked directory with files that should not be traversed
	symlinkTarget := filepath.Join(root, "external")
	require.NoError(t, os.MkdirAll(symlinkTarget, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(symlinkTarget, "hidden.txt"), []byte("secret"), 0o600))

	require.NoError(t, os.Symlink(symlinkTarget, filepath.Join(workingDir, "linkeddir")))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(afero.NewOsFs(), logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir: workingDir,
		markerPath: filepath.Join(workingDir, "_par2cron"),
		par2Mode:   schema.CreateFolderMode,
		par2Glob:   "**/*.txt",
	}

	files, err := prog.findElementsToProtect(t.Context(), job)

	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, filepath.Join(workingDir, "realdir", "real.txt"), files[0].Path)
	require.Equal(t, "realdir/real.txt", files[0].Name)

	// Files inside the symlinked directory should never appear
	for _, f := range files {
		require.NotContains(t, f.Path, "hidden.txt")
		require.NotContains(t, f.Path, "linkeddir")
	}
}

// Expectation: The function should succeed in folder mode.
func Test_Service_createCombined_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file1.txt", []byte("content1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file2.txt", []byte("content2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file1.txt", Name: "file1.txt"},
		{Path: "/data/folder/file2.txt", Name: "file2.txt"},
	}

	err := prog.createCombined(t.Context(), job, files)
	require.NoError(t, err)

	require.Equal(t, 1, called)
	require.Equal(t, 1, strings.Count(logBuf.String(), "Succeeded to create PAR2"))
}

// Expectation: The function should return no error when a PAR2 already exists.
func Test_Service_createCombined_AlreadyExists_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("existing"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file.txt", Name: "file.txt"},
	}

	err := prog.createCombined(t.Context(), job, files)
	require.NoError(t, err)
	require.Contains(t, logBuf.String(), "Same-named PAR2 already exists in folder")
}

// Expectation: The function should succeed when both directories succeed.
func Test_Service_createNested_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder/sub1", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder/sub2", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/sub1/file1.txt", []byte("content1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/sub2/file2.txt", []byte("content2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateNestedMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/sub1/file1.txt", Name: "file1.txt"},
		{Path: "/data/folder/sub2/file2.txt", Name: "file2.txt"},
	}

	err := prog.createNested(t.Context(), job, files)
	require.NoError(t, err)

	require.Equal(t, 2, called)
	require.Equal(t, 2, strings.Count(logBuf.String(), "Succeeded to create PAR2"))
}

// Expectation: Files in the same directory should be grouped together and passed to runCreate as one call.
func Test_Service_createNested_Grouping_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder/sub1", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder/sub2", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/sub1/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/sub1/b.txt", []byte("b"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/sub2/c.txt", []byte("c"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	type runCall struct {
		workingDir string
		args       []string
	}
	var calls []runCall

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			calls = append(calls, runCall{workingDir: workingDir, args: append([]string{}, args...)})

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateNestedMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/sub1/a.txt", Name: "a.txt"},
		{Path: "/data/folder/sub1/b.txt", Name: "b.txt"},
		{Path: "/data/folder/sub2/c.txt", Name: "c.txt"},
	}

	err := prog.createNested(t.Context(), job, files)
	require.NoError(t, err)

	// Two groups: sub1 (2 files) and sub2 (1 file).
	require.Len(t, calls, 2)

	// First call should be for sub1 with both files grouped together.
	require.Equal(t, "/data/folder/sub1", calls[0].workingDir)
	require.Contains(t, calls[0].args, "/data/folder/sub1/a.txt")
	require.Contains(t, calls[0].args, "/data/folder/sub1/b.txt")

	// Second call should be for sub2 with one file.
	require.Equal(t, "/data/folder/sub2", calls[1].workingDir)
	require.Contains(t, calls[1].args, "/data/folder/sub2/c.txt")

	require.Equal(t, 2, strings.Count(logBuf.String(), "Succeeded to create PAR2"))
}

// Expectation: Files in deeply nested directories should be grouped by their immediate parent.
func Test_Service_createNested_DeepNesting_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder/a/b/c", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder/a/b/d", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/a/b/c/file1.txt", []byte("1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/a/b/c/file2.txt", []byte("2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/a/b/d/file3.txt", []byte("3"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/a/b/file4.txt", []byte("4"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	type runCall struct {
		workingDir string
		args       []string
	}
	var calls []runCall

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			calls = append(calls, runCall{workingDir: workingDir, args: append([]string{}, args...)})

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateNestedMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/a/b/c/file1.txt", Name: "file1.txt"},
		{Path: "/data/folder/a/b/c/file2.txt", Name: "file2.txt"},
		{Path: "/data/folder/a/b/d/file3.txt", Name: "file3.txt"},
		{Path: "/data/folder/a/b/file4.txt", Name: "file4.txt"},
	}

	err := prog.createNested(t.Context(), job, files)
	require.NoError(t, err)

	// Three groups: a/b/c (2 files), a/b/d (1 file), a/b (1 file).
	require.Len(t, calls, 3)

	// First call: a/b/c with two files grouped together.
	require.Equal(t, "/data/folder/a/b/c", calls[0].workingDir)
	require.Contains(t, calls[0].args, "/data/folder/a/b/c/file1.txt")
	require.Contains(t, calls[0].args, "/data/folder/a/b/c/file2.txt")

	// Second call: a/b/d with one file.
	require.Equal(t, "/data/folder/a/b/d", calls[1].workingDir)
	require.Contains(t, calls[1].args, "/data/folder/a/b/d/file3.txt")

	// Third call: a/b with one file.
	require.Equal(t, "/data/folder/a/b", calls[2].workingDir)
	require.Contains(t, calls[2].args, "/data/folder/a/b/file4.txt")

	require.Equal(t, 3, strings.Count(logBuf.String(), "Succeeded to create PAR2"))
}

// Expectation: The function should continue to second directory when first fails and return correct error.
func Test_Service_createNested_FirstFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder/sub1", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder/sub2", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/sub1/file1.txt", []byte("content1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/sub2/file2.txt", []byte("content2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			if called == 1 {
				return errors.New("test error")
			}

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateNestedMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/sub1/file1.txt", Name: "file1.txt"},
		{Path: "/data/folder/sub2/file2.txt", Name: "file2.txt"},
	}

	err := prog.createNested(t.Context(), job, files)
	require.Equal(t, 2, called)

	require.ErrorIs(t, err, errSubjobFailure)
	require.Contains(t, err.Error(), "1/2 failed")
	require.Equal(t, 1, strings.Count(logBuf.String(), "Succeeded to create PAR2"))
}

// Expectation: The function should not fail when a same-named PAR2 already exists.
func Test_Service_createNested_AlreadyExists_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder/sub", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/sub/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/sub/sub"+schema.Par2Extension, []byte("existing"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateNestedMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/sub/file.txt", Name: "file.txt"},
	}

	err := prog.createNested(t.Context(), job, files)
	require.NoError(t, err)
	require.Contains(t, logBuf.String(), "Same-named PAR2 already exists in folder")
}

// Expectation: The function should respect cancellation and return the correct error.
func Test_Service_createNested_CtxCancel_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder/sub", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/sub/file1.txt", []byte("content1"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateNestedMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/sub/file1.txt", Name: "file1.txt"},
	}

	err := prog.createNested(ctx, job, files)
	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: The function should succeed when both files succeed.
func Test_Service_createIndividual_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file1.txt", []byte("content1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file2.txt", []byte("content2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFileMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file1.txt", Name: "file1.txt"},
		{Path: "/data/folder/file2.txt", Name: "file2.txt"},
	}

	err := prog.createIndividual(t.Context(), job, files)
	require.NoError(t, err)

	require.Equal(t, 2, called)
	require.Equal(t, 2, strings.Count(logBuf.String(), "Succeeded to create PAR2"))
}

// Expectation: The function should continue to second file when first fails and return correct error.
func Test_Service_createIndividual_FirstFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file1.txt", []byte("content1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file2.txt", []byte("content2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			if called == 1 {
				return errors.New("test error")
			}

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFileMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file1.txt", Name: "file1.txt"},
		{Path: "/data/folder/file2.txt", Name: "file2.txt"},
	}

	err := prog.createIndividual(t.Context(), job, files)
	require.Equal(t, 2, called)

	require.ErrorIs(t, err, errSubjobFailure)
	require.Contains(t, err.Error(), "1/2 failed")
	require.Equal(t, 1, strings.Count(logBuf.String(), "Succeeded to create PAR2"))
}

// Expectation: The function should not fail when a same-named PAR2 already exists.
func Test_Service_createIndividual_AlreadyExists_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt"+schema.Par2Extension, []byte("existing"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFileMode,
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file.txt", Name: "file.txt"},
	}

	err := prog.createIndividual(t.Context(), job, files)
	require.NoError(t, err)
	require.Contains(t, logBuf.String(), "Same-named PAR2 already exists in folder")
}

// Expectation: The function should respect cancellation and return the correct error.
func Test_Service_createIndividual_CtxCancel_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file1.txt", []byte("content1"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFileMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file1.txt", Name: "file1.txt"},
	}

	err := prog.createIndividual(ctx, job, files)
	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: The function should run the creation without any failures.
func Test_Service_runCreate_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2data"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file.txt", Name: "file.txt"},
	}

	require.NoError(t, prog.runCreate(t.Context(), job, files))

	par2Exists, _ := afero.Exists(fs, job.par2Path)
	require.True(t, par2Exists)

	manifestExists, _ := afero.Exists(fs, job.manifestPath)
	require.True(t, manifestExists)
}

// Expectation: The function should create with run par2, verify par2 without failure.
func Test_Service_runCreate_PostVerification_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			if called == 1 {
				require.Equal(t, "create", args[0])
				require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("content"), 0o644))

				return nil
			} else {
				require.Equal(t, "verify", args[0])

				return nil
			}
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		par2Verify:   true,
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file.txt", Name: "file.txt"},
	}

	require.NoError(t, prog.runCreate(t.Context(), job, files))

	par2Exists, _ := afero.Exists(fs, job.par2Path)
	require.True(t, par2Exists)

	manifestExists, _ := afero.Exists(fs, job.manifestPath)
	require.True(t, manifestExists)

	require.Equal(t, 2, called)
}

// Expectation: The function should error on post-verification failure.
func Test_Service_runCreate_PostVerification_Failure_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			if called == 1 {
				require.Equal(t, "create", args[0])
				require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("content"), 0o644))

				return nil
			} else {
				require.Equal(t, "verify", args[0])

				return errors.New("verification failed")
			}
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		par2Verify:   true,
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file.txt", Name: "file.txt"},
	}

	require.ErrorContains(t, prog.runCreate(t.Context(), job, files), "verification failed")

	par2Exists, _ := afero.Exists(fs, job.par2Path)
	require.False(t, par2Exists)

	manifestExists, _ := afero.Exists(fs, job.manifestPath)
	require.False(t, manifestExists)

	require.Equal(t, 2, called)
}

// Expectation: The function should call par2 with correct arguments.
func Test_Service_runCreate_CorrectArgs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file2.txt", []byte("content2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var capturedCmd string
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			capturedCmd = cmd
			capturedArgs = slices.Clone(args)

			require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2data"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10", "-n5"},
		par2Glob:     "*",
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file.txt", Name: "file.txt"},
		{Path: "/data/folder/file2.txt", Name: "file2.txt"},
	}

	require.NoError(t, prog.runCreate(t.Context(), job, files))

	require.Equal(t, "par2", capturedCmd)
	require.Equal(t, []string{
		"create",
		"-r10",
		"-n5",
		"--",
		job.par2Path,
		"/data/folder/file.txt",
		"/data/folder/file2.txt",
	}, capturedArgs)
}

// Expectation: The function should call par2 with correct arguments in recursive mode.
func Test_Service_runCreate_CorrectArgs_RecursiveMode_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder/subfolder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/subfolder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	var capturedCmd string
	var capturedArgs []string
	var capturedWorkingDir string
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			capturedCmd = cmd
			capturedArgs = slices.Clone(args)
			capturedWorkingDir = workingDir

			require.NoError(t, afero.WriteFile(fs, "/data/folder/folder"+schema.Par2Extension, []byte("par2data"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateRecursiveMode,
		par2Name:     "folder" + schema.Par2Extension,
		par2Path:     "/data/folder/folder" + schema.Par2Extension,
		par2Args:     []string{"-r10", "-n5", "-R"},
		par2Glob:     "*",
		lockPath:     "/data/folder/folder" + schema.Par2Extension + schema.LockExtension,
		manifestName: "folder" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/folder" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file.txt", Name: "file.txt", IsDir: false},
		{Path: "/data/folder/subfolder", Name: "subfolder", IsDir: true},
	}

	require.NoError(t, prog.runCreate(t.Context(), job, files))

	require.Equal(t, "par2", capturedCmd)
	require.Equal(t, "/data/folder", capturedWorkingDir)
	require.Equal(t, []string{
		"create",
		"-r10",
		"-n5",
		"-R",
		"--",
		"/data/folder/folder" + schema.Par2Extension,
		"/data/folder/file.txt",
		"/data/folder/subfolder",
	}, capturedArgs)
}

// Expectation: The manifest should unmarshal correctly and contain the expected
// creation metadata, including mode, glob, args, timing, and elements with
// relative file names rather than full paths.
func Test_Service_runCreate_ManifestUnmarshal_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file2.txt", []byte("content2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2data"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*.txt",
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file.txt", Name: "file.txt"},
		{Path: "/data/folder/file2.txt", Name: "file2.txt"},
	}

	require.NoError(t, prog.runCreate(t.Context(), job, files))

	manifestData, err := afero.ReadFile(fs, job.manifestPath)
	require.NoError(t, err)

	var mf schema.Manifest
	require.NoError(t, json.Unmarshal(manifestData, &mf))

	require.Equal(t, schema.ProgramVersion, mf.ProgramVersion)
	require.Equal(t, schema.ManifestVersion, mf.ManifestVersion)
	require.Equal(t, job.par2Name, mf.Name)
	require.NotEmpty(t, mf.SHA256)

	require.NotNil(t, mf.Creation)
	require.Equal(t, schema.ProgramVersion, mf.Creation.ProgramVersion)
	require.Equal(t, schema.Par2Version, mf.Creation.Par2Version)
	require.Equal(t, schema.CreateFolderMode, mf.Creation.Mode)
	require.Equal(t, "*.txt", mf.Creation.Glob)
	require.Equal(t, []string{"-r10"}, mf.Creation.Args)
	require.False(t, mf.Creation.Time.IsZero())
	require.Greater(t, mf.Creation.Duration, time.Duration(0))

	require.Len(t, mf.Creation.Elements, 2)
	for _, elem := range mf.Creation.Elements {
		require.NotContains(t, elem.Name, "/data/folder")
		require.NotContains(t, elem.Path, string(os.PathSeparator)+"data"+string(os.PathSeparator))
	}

	expectedNames := []string{files[0].Name, files[1].Name}
	actualNames := make([]string, len(mf.Creation.Elements))
	for i, elem := range mf.Creation.Elements {
		actualNames[i] = elem.Name
	}
	require.ElementsMatch(t, expectedNames, actualNames)
}

// Expectation: The manifest should contain relative file names, not full paths.
func Test_Service_runCreate_ManifestContainsRelativePaths_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder2", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder2/file2.txt", []byte("content2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			require.NoError(t, afero.WriteFile(fs, "/data/folder2/test"+schema.Par2Extension, []byte("par2data"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder2",
		markerPath:   "/data/folder2/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder2/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder2/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder2/test" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder2/file.txt", Name: "file.txt"},
		{Path: "/data/folder2/file2.txt", Name: "file2.txt"},
	}

	require.NoError(t, prog.runCreate(t.Context(), job, files))

	manifestData, err := afero.ReadFile(fs, job.manifestPath)
	require.NoError(t, err)

	manifestStr := string(manifestData)
	require.NotContains(t, manifestStr, "data")
	require.NotContains(t, manifestStr, "folder2")
}

// Expectation: The function should cleanup on failure and return error.
func Test_Service_runCreate_Par2Fails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("partial"), 0o644))

			return testutil.CreateExitError(t, ctx, 5)
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file.txt", Name: "file.txt"},
	}

	err := prog.runCreate(t.Context(), job, files)

	require.Error(t, err)
	require.Contains(t, logBuf.String(), "Failed to create PAR2")

	par2Exists, _ := afero.Exists(fs, job.par2Path)
	require.False(t, par2Exists)

	manifestExists, _ := afero.Exists(fs, job.manifestPath)
	require.False(t, manifestExists)
}

// Expectation: The function should log warning if manifest write fails but still succeed.
func Test_Service_runCreate_ManifestWriteFails_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/data/folder/file.txt", []byte("content"), 0o644))

	fs := &testutil.FailingWriteFs{Fs: baseFs, FailSuffix: schema.ManifestExtension}

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			require.NoError(t, afero.WriteFile(baseFs, "/data/folder/test"+schema.Par2Extension, []byte("par2data"), 0o644))

			return nil
		},
	}
	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})
	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file.txt", Name: "file.txt"},
	}

	require.NoError(t, prog.runCreate(t.Context(), job, files))
	require.Contains(t, logBuf.String(), "Failed to write par2cron manifest")

	par2exists, _ := afero.Exists(fs, job.par2Path)
	require.True(t, par2exists)

	manifestExists, _ := afero.Exists(fs, job.manifestPath)
	require.False(t, manifestExists)
}

// Expectation: The function should respect context cancellation.
func Test_Service_runCreate_CtxCancel_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))

	ctx, cancel := context.WithCancel(t.Context())

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			cancel()

			return context.Canceled
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file.txt", Name: "file.txt"},
	}

	err := prog.runCreate(ctx, job, files)
	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: The function should run the creation and pack the result as a bundle.
func Test_Service_runCreate_Bundle_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2data"), 0o644))
			require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol00+01"+schema.Par2Extension, []byte("vol1"), 0o644))

			return nil
		},
	}

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
	bundler := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			packCalled = true

			require.Equal(t, setID, recoverySetID)
			require.NotEmpty(t, manifest.Bytes)
			require.NotEmpty(t, files)

			require.NoError(t, afero.WriteFile(fsys, bundlePath, []byte("bundledata"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, bundler, par2er)

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
		asBundle:     true,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file.txt", Name: "file.txt"},
	}

	require.NoError(t, prog.runCreate(t.Context(), job, files))
	require.True(t, packCalled)

	// Original PAR2 files should be cleaned up.
	indexExists, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension)
	require.False(t, indexExists)

	volExists, _ := afero.Exists(fs, "/data/folder/test.vol00+01"+schema.Par2Extension)
	require.False(t, volExists)

	// Bundle should exist.
	bundleExists, _ := afero.Exists(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension)
	require.True(t, bundleExists)

	// Job fields should have been updated to point to the bundle.
	require.Equal(t, "test"+schema.BundleExtension+schema.Par2Extension, job.par2Name)
	require.Equal(t, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, job.par2Path)

	// No standalone manifest file should exist (it's inside the bundle).
	manifestExists, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension)
	require.False(t, manifestExists)
}

// Expectation: The function should return an error when bundling fails after successful PAR2 creation.
func Test_Service_runCreate_Bundle_PackFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2data"), 0o644))
			require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol00+01"+schema.Par2Extension, []byte("vol1"), 0o644))

			return nil
		},
	}

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

	bundler := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			return errors.New("disk full")
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner, bundler, par2er)

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Mode:     schema.CreateFolderMode,
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
		asBundle:     true,
	}

	files := []schema.FsElement{
		{Path: "/data/folder/file.txt", Name: "file.txt"},
	}

	err := prog.runCreate(t.Context(), job, files)
	require.ErrorContains(t, err, "failed to bundle")
	require.Contains(t, logBuf.String(), "Failed to bundle created PAR2 files")

	// PAR2 files should be cleaned up by the deferred cleanupAfterFailure.
	par2Exists, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension)
	require.False(t, par2Exists)

	volExists, _ := afero.Exists(fs, "/data/folder/test.vol00+01"+schema.Par2Extension)
	require.False(t, volExists)

	// No bundle should exist.
	bundleExists, _ := afero.Exists(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension)
	require.False(t, bundleExists)

	// No standalone manifest should exist.
	manifestExists, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension)
	require.False(t, manifestExists)
}
