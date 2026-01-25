package create

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/flags"
	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: The correct paths should be derived from the [createConfig].
func Test_NewJob_Success(t *testing.T) {
	t.Parallel()

	cfg := MarkerConfig{
		Par2Mode:   &flags.CreateMode{Raw: schema.CreateFolderMode, Value: schema.CreateFolderMode},
		Par2Name:   util.Ptr("test" + schema.Par2Extension),
		Par2Args:   &[]string{"-r10", "-n5"},
		Par2Glob:   util.Ptr("*.txt"),
		Par2Verify: util.Ptr(true),
		HideFiles:  util.Ptr(false),
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
}

// Expectation: The correct paths should be derived from the [createConfig].
func Test_NewJob_HideFiles_Success(t *testing.T) {
	t.Parallel()

	cfg := MarkerConfig{
		Par2Mode:   &flags.CreateMode{Raw: schema.CreateFolderMode, Value: schema.CreateFolderMode},
		Par2Name:   util.Ptr("test" + schema.Par2Extension),
		Par2Args:   &[]string{"-r10", "-n5"},
		Par2Glob:   util.Ptr("*.txt"),
		Par2Verify: util.Ptr(true),
		HideFiles:  util.Ptr(true),
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

	prog := NewService(fs, logging.NewLogger(ls), runner)

	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	require.NoError(t, prog.Create(t.Context(), "/data", args))

	require.True(t, called)
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

	prog := NewService(fs, logging.NewLogger(ls), runner)

	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	require.NoError(t, prog.Create(t.Context(), "/data", args))

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

	prog := NewService(fs, logging.NewLogger(ls), runner)

	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	require.ErrorIs(t, prog.Create(t.Context(), "/data", args), schema.ErrExitPartialFailure)

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

	prog := NewService(fs, logging.NewLogger(ls), runner)

	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	require.NoError(t, prog.Create(t.Context(), "/data", args))

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

	prog := NewService(fs, logging.NewLogger(ls), runner)

	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	require.ErrorIs(t, prog.Create(t.Context(), "/data", args), schema.ErrExitPartialFailure)

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

	prog := NewService(fs, logging.NewLogger(ls), runner)
	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}

	err := prog.Create(t.Context(), "/data", args)
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

	prog := NewService(fs, logging.NewLogger(ls), runner)
	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}

	err := prog.Create(t.Context(), "/data", args)
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-r10"}}
	require.NoError(t, prog.Create(t.Context(), "/data", args))
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-r10"}}
	err := prog.Create(ctx, "/data", args)

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

	prog := NewService(fs, logging.NewLogger(ls), runner)
	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	require.NoError(t, args.MaxDuration.Set("10ms"))

	err := prog.Create(t.Context(), "/data", args)
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

	prog := NewService(fs, logging.NewLogger(ls), runner)
	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	require.NoError(t, args.MaxDuration.Set("10s"))

	err := prog.Create(t.Context(), "/data", args)
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

	prog := NewService(fs, logging.NewLogger(ls), runner)
	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}
	require.NoError(t, args.MaxDuration.Set("10ms"))

	err := prog.Create(t.Context(), "/data", args)
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

	prog := NewService(fs, logging.NewLogger(ls), runner)
	args := Options{Par2Args: []string{"-r10"}, Par2Glob: "*"}

	err := prog.Create(t.Context(), "/data", args)
	require.NoError(t, err)

	require.Equal(t, 2, called)
	require.Equal(t, 2, strings.Count(logBuf.String(), "Job completed with success"))
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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
func Test_Service_findFilesToProtect_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

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

	files, err := prog.findFilesToProtect(t.Context(), job)

	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "/data/folder/file.txt", files[0].Path)
	require.Equal(t, "file.txt", files[0].Name)
}

// Expectation: The function should return the correct error when there's nothing to do.
func Test_Service_findFilesToProtect_NoFilesToProtect_Error(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

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

	_, err := prog.findFilesToProtect(t.Context(), job)

	require.ErrorIs(t, err, errNoFilesToProtect)
	require.Contains(t, logBuf.String(), "No files to protect")
}

// Expectation: The function should succeed in folder mode.
func Test_Service_createFolderMode_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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

	files := []schema.ProtectedFile{
		{Path: "/data/folder/file1.txt", Name: "file1.txt"},
		{Path: "/data/folder/file2.txt", Name: "file2.txt"},
	}

	err := prog.createFolderMode(t.Context(), job, files)
	require.NoError(t, err)

	require.Equal(t, 1, called)
	require.Equal(t, 1, strings.Count(logBuf.String(), "Succeeded to create PAR2"))
}

// Expectation: The function should return the correct error when a PAR2 already exists.
func Test_Service_createFolderMode_AlreadyExists_Error(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

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

	files := []schema.ProtectedFile{
		{Path: "/data/folder/file.txt", Name: "file.txt"},
	}

	err := prog.createFolderMode(t.Context(), job, files)
	require.ErrorIs(t, err, schema.ErrAlreadyExists)
	require.Contains(t, logBuf.String(), "Same-named PAR2 already exists in folder")
}

// Expectation: The function should succeed when both files succeed.
func Test_Service_createFileMode_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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

	files := []schema.ProtectedFile{
		{Path: "/data/folder/file1.txt", Name: "file1.txt"},
		{Path: "/data/folder/file2.txt", Name: "file2.txt"},
	}

	err := prog.createFileMode(t.Context(), job, files)
	require.NoError(t, err)

	require.Equal(t, 2, called)
	require.Equal(t, 2, strings.Count(logBuf.String(), "Succeeded to create PAR2"))
}

// Expectation: The function should continue to second file when first fails and return correct error.
func Test_Service_createFileMode_FirstFails_Error(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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

	files := []schema.ProtectedFile{
		{Path: "/data/folder/file1.txt", Name: "file1.txt"},
		{Path: "/data/folder/file2.txt", Name: "file2.txt"},
	}

	err := prog.createFileMode(t.Context(), job, files)
	require.Equal(t, 2, called)

	require.ErrorIs(t, err, errSubjobFailure)
	require.Contains(t, err.Error(), "1/2 failed")
	require.Equal(t, 1, strings.Count(logBuf.String(), "Succeeded to create PAR2"))
}

// Expectation: The function should not fail when a same-named PAR2 already exists.
func Test_Service_createFileMode_AlreadyExists_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

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

	files := []schema.ProtectedFile{
		{Path: "/data/folder/file.txt", Name: "file.txt"},
	}

	err := prog.createFileMode(t.Context(), job, files)
	require.NoError(t, err)
	require.Contains(t, logBuf.String(), "Same-named PAR2 already exists in folder")
}

// Expectation: The function should respect cancellation and return the correct error.
func Test_Service_createFileMode_CtxCancel_Error(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

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

	files := []schema.ProtectedFile{
		{Path: "/data/folder/file1.txt", Name: "file1.txt"},
	}

	err := prog.createFileMode(ctx, job, files)
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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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

	files := []schema.ProtectedFile{
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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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

	files := []schema.ProtectedFile{
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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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

	files := []schema.ProtectedFile{
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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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

	files := []schema.ProtectedFile{
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

// Expectation: The manifest should contain relative file names, not full paths.
func Test_Service_runCreate_ManifestContainsRelativePaths_Success(t *testing.T) {
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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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

	files := []schema.ProtectedFile{
		{Path: "/data/folder/file.txt", Name: "file.txt"},
		{Path: "/data/folder/file2.txt", Name: "file2.txt"},
	}

	require.NoError(t, prog.runCreate(t.Context(), job, files))

	manifestData, err := afero.ReadFile(fs, job.manifestPath)
	require.NoError(t, err)

	manifestStr := string(manifestData)
	require.NotContains(t, manifestStr, "data")
	require.NotContains(t, manifestStr, "folder")
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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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

	files := []schema.ProtectedFile{
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
	prog := NewService(fs, logging.NewLogger(ls), runner)
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

	files := []schema.ProtectedFile{
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

	prog := NewService(fs, logging.NewLogger(ls), runner)

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

	files := []schema.ProtectedFile{
		{Path: "/data/folder/file.txt", Name: "file.txt"},
	}

	err := prog.runCreate(ctx, job, files)
	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: All relevant files should be removed, but others not.
func Test_Service_cleanupAfterFailure_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension+schema.LockExtension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol01+02"+schema.Par2Extension, []byte("vol"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/existing"+schema.Par2Extension, []byte("par2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
	}

	prog.cleanupAfterFailure(t.Context(), job)

	exists1, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension)
	require.False(t, exists1)

	exists2, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension)
	require.False(t, exists2)

	exists3, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension+schema.LockExtension)
	require.False(t, exists3)

	exists4, _ := afero.Exists(fs, "/data/folder/test.vol01+02"+schema.Par2Extension)
	require.False(t, exists4)

	exists5, _ := afero.Exists(fs, "/data/folder/existing"+schema.Par2Extension)
	require.True(t, exists5)
}

// Expectation: Non-failing files should be removed regardless of failure.
func Test_Service_cleanupAfterFailure_OneFails_Error(t *testing.T) {
	t.Parallel()

	fs := &testutil.FailingRemoveFs{Fs: afero.NewMemMapFs(), FailSuffix: schema.LockExtension}

	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension+schema.LockExtension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol01+02"+schema.Par2Extension, []byte("vol"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/existing"+schema.Par2Extension, []byte("par2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
	}

	prog.cleanupAfterFailure(t.Context(), job)

	exists1, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension)
	require.False(t, exists1)

	exists2, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension)
	require.False(t, exists2)

	exists3, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension+schema.LockExtension)
	require.True(t, exists3)

	exists4, _ := afero.Exists(fs, "/data/folder/test.vol01+02"+schema.Par2Extension)
	require.False(t, exists4)

	exists5, _ := afero.Exists(fs, "/data/folder/existing"+schema.Par2Extension)
	require.True(t, exists5)

	require.Contains(t, logBuf.String(), "Failed to cleanup a file after failure")
}
