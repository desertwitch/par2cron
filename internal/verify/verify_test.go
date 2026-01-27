package verify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func createWithManifest(t *testing.T, fs afero.Fs, path string) {
	t.Helper()

	mf := schema.NewManifest(filepath.Base(path))

	mf.Creation = &schema.CreationManifest{}
	mf.Creation.Time = time.Now()

	by, err := json.Marshal(mf)
	require.NoError(t, err)

	require.NoError(t, fs.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, afero.WriteFile(fs, path+schema.Par2Extension, []byte("par2data"), 0o644))
	require.NoError(t, afero.WriteFile(fs, path+schema.Par2Extension+schema.ManifestExtension, by, 0o644))
}

// Expectation: Important constants should not have changed.
func Test_Constants_Success(t *testing.T) {
	t.Parallel()

	require.Equal(t, 0, prioNoManifest)
	require.Equal(t, 1, prioNoVerification)
	require.Equal(t, 2, prioNeedsRepair)
	require.Equal(t, 3, prioOther)
}

// Expectation: A new verification job should be returned with the correct values.
func Test_NewJob_Success(t *testing.T) {
	t.Parallel()

	args := Options{
		Par2Args: []string{"-v"},
	}

	mf := schema.NewManifest("test" + schema.Par2Extension)
	job := NewJob("/data/test"+schema.Par2Extension, args, mf)

	require.Equal(t, "/data", job.workingDir)
	require.Equal(t, "test"+schema.Par2Extension, job.par2Name)
	require.Equal(t, "/data/test"+schema.Par2Extension, job.par2Path)
	require.Equal(t, []string{"-v"}, job.par2Args)
	require.Equal(t, "test"+schema.Par2Extension+schema.ManifestExtension, job.manifestName)
	require.Equal(t, "/data/test"+schema.Par2Extension+schema.ManifestExtension, job.manifestPath)
	require.Equal(t, "/data/test"+schema.Par2Extension+schema.LockExtension, job.lockPath)

	require.Equal(t, mf, job.manifest)
}

// Expectation: The program should run the verification with the correct outcome.
func Test_Service_Verify_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/test")

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

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)
	args := Options{Par2Args: []string{"-v"}}
	require.NoError(t, prog.Verify(t.Context(), "/data", args))

	require.True(t, called)
	require.Contains(t, logBuf.String(), "Job completed with success")
}

// Expectation: A locked file should not fail the verification process.
func Test_Service_Verify_FileLocked_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/test")

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
	args := Options{Par2Args: []string{"-v"}}
	require.NoError(t, prog.Verify(t.Context(), "/data", args))

	require.True(t, called)
	require.Contains(t, logBuf.String(), "Job unavailable (will retry next run)")
}

// Expectation: The program should run the verification with the correct outcome.
func Test_Service_Verify_Generic_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/test")

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			return errors.New("generic error")
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	args := Options{Par2Args: []string{"-v"}}
	require.ErrorIs(t, prog.Verify(t.Context(), "/data", args), schema.ErrExitPartialFailure)

	require.Contains(t, logBuf.String(), "Job failure (will retry next run)")
}

// Expectation: The program should run the verification with the correct outcome.
func Test_Service_Verify_CorruptionDetected_Repairable_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/test")

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			return testutil.CreateExitError(t, ctx, schema.Par2ExitCodeRepairPossible)
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	args := Options{Par2Args: []string{"-v"}}
	require.ErrorIs(t, prog.Verify(t.Context(), "/data", args), schema.ErrExitRepairable)

	require.Contains(t, logBuf.String(), "Job completed with corruption detected")
}

// Expectation: The program should run the verification with the correct outcome.
func Test_Service_Verify_CorruptionDetected_Unrepairable_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/test")

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			return testutil.CreateExitError(t, ctx, schema.Par2ExitCodeRepairImpossible)
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	args := Options{Par2Args: []string{"-v"}}
	require.ErrorIs(t, prog.Verify(t.Context(), "/data", args), schema.ErrExitUnrepairable)

	require.Contains(t, logBuf.String(), "Job completed with corruption detected")
}

// Expectation: The program should run the verification with the correct outcome.
func Test_Service_Verify_MultipleJobs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/test")
	createWithManifest(t, fs, "/data/test2")

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
	args := Options{Par2Args: []string{"-v"}}
	require.NoError(t, prog.Verify(t.Context(), "/data", args))

	require.Equal(t, 2, called)
	require.Equal(t, 2, strings.Count(logBuf.String(), "Job completed with success"))
}

// Expectation: The program should run the verification with the correct outcome.
func Test_Service_Verify_MultipleJobs_OneFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/test")
	createWithManifest(t, fs, "/data/test2")

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
			}

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)
	args := Options{Par2Args: []string{"-v"}}
	require.ErrorIs(t, prog.Verify(t.Context(), "/data", args), schema.ErrExitPartialFailure)

	require.Equal(t, 2, called)
	require.Equal(t, 1, strings.Count(logBuf.String(), "Job completed with success"))
	require.Equal(t, 1, strings.Count(logBuf.String(), "Job failure (will retry next run)"))
}

// Expectation: The program should continue if an enumeration partial (non-fatal) failure occurs.
// Eventually though, an error must be returned so the user knows something went wrong (non-zero exit code).
func Test_Service_Verify_MultipleJobs_EnumerationFails_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	createWithManifest(t, baseFs, "/data/test1")
	createWithManifest(t, baseFs, "/data/test2")

	fs := &testutil.FailingOpenFs{
		Fs:          baseFs,
		FailPattern: "/data/test1" + schema.Par2Extension + schema.ManifestExtension,
	}

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

	args := Options{Par2Args: []string{"-v"}}
	err := prog.Verify(t.Context(), "/data", args)

	require.ErrorIs(t, err, schema.ErrExitPartialFailure)
	require.ErrorIs(t, err, schema.ErrNonFatal)
	require.Equal(t, 1, called)
	require.Equal(t, 1, strings.Count(logBuf.String(), "Job completed with success"))
	require.Equal(t, 1, strings.Count(logBuf.String(), "Failed to read par2cron manifest"))
}

// Expectation: The program should continue if an enumeration partial (non-fatal) failure occurs.
// Eventually though, an error must be returned so the user knows something went wrong (non-zero exit code).
func Test_Service_Verify_MultipleJobs_EnumerationFails_NoOtherJobs_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	createWithManifest(t, baseFs, "/data/test")

	fs := &testutil.FailingOpenFs{
		Fs:          baseFs,
		FailPattern: schema.ManifestExtension,
	}

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

	args := Options{Par2Args: []string{"-v"}}
	err := prog.Verify(t.Context(), "/data", args)

	require.ErrorIs(t, err, schema.ErrExitPartialFailure)
	require.ErrorIs(t, err, schema.ErrNonFatal)
	require.Equal(t, 0, called)
	require.Equal(t, 1, strings.Count(logBuf.String(), "Nothing to do"))
	require.Equal(t, 1, strings.Count(logBuf.String(), "Failed to read par2cron manifest"))
}

// Expectation: The program should run the verification with the correct outcome.
func Test_Service_Verify_MultipleJobs_ErrorOrdering_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/test")
	createWithManifest(t, fs, "/data/test2")

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
				return testutil.CreateExitError(t, ctx, schema.Par2ExitCodeRepairImpossible)
			}

			return errors.New("generic I/O error")
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)
	args := Options{Par2Args: []string{"-v"}}
	require.ErrorIs(t, prog.Verify(t.Context(), "/data", args), schema.ErrExitUnrepairable)

	require.Equal(t, 2, called)
}

// Expectation: The program should recognize when there's nothing to do.
func Test_Service_Verify_NoJobs_Success(t *testing.T) {
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

	args := Options{Par2Args: []string{"-v"}}
	require.NoError(t, prog.Verify(t.Context(), "/data", args))

	require.Contains(t, logBuf.String(), "Nothing to do")
}

// Expectation: The verification should respect a context cancellation.
func Test_Service_Verify_CtxCancel_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/test")

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

	args := Options{Par2Args: []string{"-v"}}
	err := prog.Verify(ctx, "/data", args)

	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: The correct job and its manifest should be returned.
func Test_Service_Enumerate_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/test")

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-v"}}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NotNil(t, jobs[0].manifest)
}

// Expectation: The correct job and a nil manifest should be returned on invalid manifest.
func Test_Service_Enumerate_InvalidManifest_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, []byte("invalid"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-v"}}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)

	require.Len(t, jobs, 1)
	require.Nil(t, jobs[0].manifest)
	require.Contains(t, logBuf.String(), "resetting manifest")

	// Should not be deleted yet (will be overwritten on success).
	manifestExists, _ := afero.Exists(fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension)
	require.True(t, manifestExists)
}

// Expectation: The relevant files should be returned as jobs, but not those without manifest.
func Test_Service_Enumerate_NoManifest_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/test")

	require.NoError(t, afero.WriteFile(fs, "/data/test2"+schema.Par2Extension, []byte("par2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-v"}}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NotNil(t, jobs[0].manifest)
	require.Equal(t, "/data/test"+schema.Par2Extension, jobs[0].par2Path)
}

// Expectation: A partial failure should be returned when reading the manifest fails.
func Test_Service_Enumerate_ReadManifestFailure_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	createWithManifest(t, baseFs, "/data/test")

	fs := &testutil.FailingOpenFs{
		Fs:          baseFs,
		FailPattern: schema.ManifestExtension,
	}

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-v"}}
	_, err := prog.Enumerate(t.Context(), "/data", args)

	require.ErrorIs(t, err, schema.ErrNonFatal)
	require.Contains(t, logBuf.String(), "Failed to read par2cron manifest")
}

// Expectation: An empty slice should be returned if no PAR2 files are found.
func Test_Service_Enumerate_NoJobs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/data/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-v"}}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Empty(t, jobs)
}

// Expectation: Elements in directories with an ignore file should be skipped non-recursively.
func Test_Service_Enumerate_IgnoreFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/ignored")
	createWithManifest(t, fs, "/data/subdir/notignored")

	require.NoError(t, afero.WriteFile(fs, "/data/"+schema.IgnoreFile, []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-v"}}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, "/data/subdir/notignored"+schema.Par2Extension, jobs[0].par2Path)
}

// Expectation: Elements in directories with an ignore-all file should be skipped recursively.
func Test_Service_Enumerate_IgnoreAllFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/normal/test")
	createWithManifest(t, fs, "/data/ignored/sub/test")

	require.NoError(t, afero.WriteFile(fs, "/data/ignored/"+schema.IgnoreAllFile, []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-v"}}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, "/data/normal/test"+schema.Par2Extension, jobs[0].par2Path)
}

// Expectation: Subdirectories with an ignore-all file should skip the entire directory tree.
func Test_Service_Enumerate_IgnoreAllFile_SkipsRecursive_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/test")
	createWithManifest(t, fs, "/data/subdir/test")
	createWithManifest(t, fs, "/data/ignored/test")
	createWithManifest(t, fs, "/data/ignored/subdir/nested")

	require.NoError(t, afero.WriteFile(fs, "/data/ignored/"+schema.IgnoreAllFile, []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-v"}}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 2)
	require.Equal(t, "/data/subdir/test"+schema.Par2Extension, jobs[0].par2Path)
	require.Equal(t, "/data/test"+schema.Par2Extension, jobs[1].par2Path)
}

// Expectation: PAR2 files without manifest should be included when --include-external is set.
func Test_Service_Enumerate_IncludeExternal_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/data/no-manifest"+schema.Par2Extension, []byte("par2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{
		Par2Args:        []string{"-v"},
		IncludeExternal: true,
	}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Nil(t, jobs[0].manifest)
}

// Expectation: PAR2 files without manifest should be skipped when --include-external is unset.
func Test_Service_Enumerate_IncludeExternal_Unset_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/with-manifest")
	require.NoError(t, afero.WriteFile(fs, "/data/no-manifest"+schema.Par2Extension, []byte("par2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{
		Par2Args:        []string{"-v"},
		IncludeExternal: false,
	}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, "/data/with-manifest"+schema.Par2Extension, jobs[0].par2Path)
	require.Contains(t, logBuf.String(), "skipping")
}

// Expectation: PAR2 files without creation manifest should be skipped when --skip-not-created is set.
func Test_Service_Enumerate_SkipNotCreated_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/with-creation")

	mfNoCreation := schema.NewManifest("no-creation" + schema.Par2Extension)
	mfNoCreation.Creation = nil
	mfNoCreationData, err := json.Marshal(mfNoCreation)
	require.NoError(t, err)
	require.NoError(t, afero.WriteFile(fs, "/data/no-creation"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/no-creation"+schema.Par2Extension+schema.ManifestExtension, mfNoCreationData, 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{
		Par2Args:       []string{"-v"},
		SkipNotCreated: true,
	}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, "/data/with-creation"+schema.Par2Extension, jobs[0].par2Path)
	require.Contains(t, logBuf.String(), "skipping; --skip-not-created")
}

// Expectation: PAR2 files with invalid manifest should be skipped when --skip-not-created is set.
func Test_Service_Enumerate_SkipNotCreated_InvalidManifest_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/data/invalid"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/invalid"+schema.Par2Extension+schema.ManifestExtension, []byte("invalid json"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{
		Par2Args:       []string{"-v"},
		SkipNotCreated: true,
	}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Empty(t, jobs)
	require.Contains(t, logBuf.String(), "skipping; --skip-not-created")
}

// Expectation: PAR2 files without creation manifest should be included when --skip-not-created is not set.
func Test_Service_Enumerate_NoSkipNotCreated_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	mfNoCreation := schema.NewManifest("no-creation" + schema.Par2Extension)
	mfNoCreation.Creation = nil
	mfNoCreationData, err := json.Marshal(mfNoCreation)
	require.NoError(t, err)
	require.NoError(t, afero.WriteFile(fs, "/data/no-creation"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/no-creation"+schema.Par2Extension+schema.ManifestExtension, mfNoCreationData, 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{
		Par2Args:       []string{"-v"},
		SkipNotCreated: false,
	}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NotNil(t, jobs[0].manifest)
	require.Nil(t, jobs[0].manifest.Creation)
}

// Expectation: Both --include-external=false and --skip-not-created can be used together.
func Test_Service_Enumerate_BothSkipOptions_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	// PAR2 without any manifest
	require.NoError(t, afero.WriteFile(fs, "/data/no-manifest"+schema.Par2Extension, []byte("par2"), 0o644))

	// PAR2 with manifest but no creation info
	mfNoCreation := schema.NewManifest("no-creation" + schema.Par2Extension)
	mfNoCreation.Creation = nil
	mfNoCreationData, err := json.Marshal(mfNoCreation)
	require.NoError(t, err)
	require.NoError(t, afero.WriteFile(fs, "/data/no-creation"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/no-creation"+schema.Par2Extension+schema.ManifestExtension, mfNoCreationData, 0o644))

	// PAR2 with full manifest
	createWithManifest(t, fs, "/data/full")

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{
		Par2Args:        []string{"-v"},
		IncludeExternal: false,
		SkipNotCreated:  true,
	}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, "/data/full"+schema.Par2Extension, jobs[0].par2Path)
}

// Expectation: Context cancellation should be respected during enumeration.
func Test_Service_Enumerate_CtxCancel_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	createWithManifest(t, fs, "/data/test")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-v"}}
	_, err := prog.Enumerate(ctx, "/data", args)

	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: The verification should pass and a manifest be created.
func Test_Service_RunVerify_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     []string{"-v"},
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		manifest:     nil,
	}

	require.NoError(t, prog.RunVerify(t.Context(), job, false))

	require.NotNil(t, job.manifest)
	require.NotNil(t, job.manifest.Verification)
	require.NotZero(t, job.manifest.Verification.Duration)
	require.Equal(t, schema.Par2ExitCodeSuccess, job.manifest.Verification.ExitCode)

	manifestExists, _ := afero.Exists(fs, job.manifestPath)
	require.True(t, manifestExists)
}

// Expectation: The verification should use the correct arguments.
func Test_Service_RunVerify_CorrectArgs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	runCmd := ""
	runArgs := []string{}
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			runCmd = cmd
			runArgs = append(runArgs, args...)

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     []string{"-v", "-q"},
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		manifest:     nil,
	}

	require.NoError(t, prog.RunVerify(t.Context(), job, false))

	require.Equal(t, "par2", runCmd)
	require.Equal(t, []string{
		"verify",
		"-v",
		"-q",
		"--",
		job.par2Path,
	}, runArgs)
}

// Expectation: The verification should not overwrite the creation manifest values.
func Test_Service_RunVerify_KeepCreateManifest_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte{}, 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     []string{"-v", "-q"},
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		manifest:     schema.NewManifest("test" + schema.Par2Extension),
	}
	job.manifest.SHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // ""
	job.manifest.Creation = &schema.CreationManifest{Time: time.Now()}

	require.NoError(t, prog.RunVerify(t.Context(), job, false))

	manifestData, err := afero.ReadFile(fs, job.manifestPath)
	require.NoError(t, err)

	mf := &schema.Manifest{}
	require.NoError(t, json.Unmarshal(manifestData, &mf))

	require.Equal(t, job.manifest.SHA256, mf.SHA256)
	require.Equal(t, job.manifest.Name, mf.Name)
	require.Equal(t, job.manifest.Verification.ExitCode, mf.Verification.ExitCode)

	require.True(t, job.manifest.Creation.Time.Equal(mf.Creation.Time))
	require.True(t, job.manifest.Verification.Time.Equal(mf.Verification.Time))
}

// Expectation: A hash mismatch should not fail the verification, but reset the manifest.
func Test_Service_RunVerify_HashMismatch_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.SHA256 = "wronghash"

	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     []string{"-v"},
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		manifest:     mf,
	}

	require.NoError(t, prog.RunVerify(t.Context(), job, false))

	require.NotEqual(t, "wronghash", job.manifest.SHA256)
	require.NotEqual(t, mf, job.manifest)

	require.Contains(t, logBuf.String(), "PAR2 changed since par2cron manifest creation")
}

// Expectation: A non-exit-code related error should return early that error.
func Test_Service_RunVerify_GenericError_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			return errors.New("test error")
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.SHA256 = "wronghash"

	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     []string{"-v"},
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		manifest:     mf,
	}

	require.ErrorContains(t, prog.RunVerify(t.Context(), job, false), "test error")
}

// Expectation: A manifest write error should fail the verification and return the error.
func Test_Service_RunVerify_ManifestWriteError_Error(t *testing.T) {
	t.Parallel()

	fs := &testutil.FailingWriteFs{Fs: afero.NewMemMapFs(), FailSuffix: schema.ManifestExtension}
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.SHA256 = "wronghash"

	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     []string{"-v"},
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		manifest:     mf,
	}

	require.ErrorContains(t, prog.RunVerify(t.Context(), job, false), "failed to write manifest")
}

// Expectation: The exit code should be parsed according to expectations.
func Test_Service_parseExitCode_CodeSuccess_Success(t *testing.T) {
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
	job := &Job{
		manifest: &schema.Manifest{
			Verification: &schema.VerificationManifest{},
		},
	}

	require.NoError(t, prog.parseExitCode(job, nil))

	require.Equal(t, 0, job.manifest.Verification.ExitCode)
	require.False(t, job.manifest.Verification.RepairNeeded)
	require.True(t, job.manifest.Verification.RepairPossible)
	require.Zero(t, job.manifest.Verification.CountCorrupted)
}

// Expectation: The exit code should be parsed according to expectations.
func Test_Service_parseExitCode_CodeRepairPossible_Success(t *testing.T) {
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
	job := &Job{
		manifest: &schema.Manifest{
			Verification: &schema.VerificationManifest{},
		},
	}

	err := testutil.CreateExitError(t, t.Context(), schema.Par2ExitCodeRepairPossible)
	require.NoError(t, prog.parseExitCode(job, err))

	require.Equal(t, schema.Par2ExitCodeRepairPossible, job.manifest.Verification.ExitCode)
	require.True(t, job.manifest.Verification.RepairNeeded)
	require.True(t, job.manifest.Verification.RepairPossible)
	require.Equal(t, 1, job.manifest.Verification.CountCorrupted)
}

// Expectation: The exit code should be parsed according to expectations.
func Test_Service_parseExitCode_CodeRepairImpossible_Success(t *testing.T) {
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
	job := &Job{
		manifest: &schema.Manifest{
			Verification: &schema.VerificationManifest{},
		},
	}

	err := testutil.CreateExitError(t, t.Context(), schema.Par2ExitCodeRepairImpossible)
	require.NoError(t, prog.parseExitCode(job, err))

	require.Equal(t, schema.Par2ExitCodeRepairImpossible, job.manifest.Verification.ExitCode)
	require.True(t, job.manifest.Verification.RepairNeeded)
	require.False(t, job.manifest.Verification.RepairPossible)
	require.Equal(t, 1, job.manifest.Verification.CountCorrupted)
}

// Expectation: The exit code should be parsed according to expectations.
func Test_Service_parseExitCode_UnhandledCode_Error(t *testing.T) {
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
	job := &Job{
		manifest: &schema.Manifest{
			Verification: &schema.VerificationManifest{},
		},
	}

	err := testutil.CreateExitError(t, t.Context(), 99)
	require.ErrorIs(t, prog.parseExitCode(job, err), err)

	require.Equal(t, 99, job.manifest.Verification.ExitCode)
}

// Expectation: A backlog warning should be thrown when the backlog is growing.
func Test_Service_considerBacklog_InsufficientCapacity_Success(t *testing.T) {
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

	jobs := []*Job{
		{
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 10 * time.Hour,
				},
			},
		},
	}

	args := Options{}
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")
	_ = args.RunInterval.Set("24h")

	prog.considerBacklog(jobs, args)

	require.Contains(t, logBuf.String(), "Backlog is growing indefinitely")
}

// Expectation: A backlog warning should not be thrown when no durations are known.
func Test_Service_considerBacklog_NoDuration_Success(t *testing.T) {
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

	jobs := []*Job{
		{
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{},
			},
		},
		{
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{},
			},
		},
	}

	args := Options{}
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")
	_ = args.RunInterval.Set("24h")

	prog.considerBacklog(jobs, args)

	require.NotContains(t, logBuf.String(), "Backlog is growing indefinitely")
}

// Expectation: A warning should be logged when the first job has unknown duration.
func Test_Service_considerDurations_FirstJobUnknownDuration_Success(t *testing.T) {
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

	jobs := []*Job{
		{
			par2Path: "/data/test" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 0,
				},
			},
		},
	}

	args := Options{}
	_ = args.MaxDuration.Set("1h")

	prog.considerDurations(jobs, args)

	require.Contains(t, logBuf.String(), "First job has (still) unknown duration")
}

// Expectation: A warning should be logged when the first job exceeds max duration.
func Test_Service_considerDurations_FirstJobExceedsDuration_Success(t *testing.T) {
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

	jobs := []*Job{
		{
			par2Path: "/data/test" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 2 * time.Hour,
				},
			},
		},
	}

	args := Options{}
	_ = args.MaxDuration.Set("1h")

	prog.considerDurations(jobs, args)

	require.Contains(t, logBuf.String(), "First job is estimated to exceed --duration")
}

// Expectation: A warning should be logged when subsequent jobs have unknown duration.
func Test_Service_considerDurations_SubsequentJobsUnknownDuration_Success(t *testing.T) {
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

	jobs := []*Job{
		{
			par2Path: "/data/test1" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 30 * time.Minute,
				},
			},
		},
		{
			par2Path: "/data/test2" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 0,
				},
			},
		},
	}

	args := Options{}
	_ = args.MaxDuration.Set("1h")

	prog.considerDurations(jobs, args)

	require.Contains(t, logBuf.String(), "Some jobs have a (still) unknown duration")
}

// Expectation: No warning should be logged when all jobs have known durations within limit.
func Test_Service_considerDurations_AllJobsKnownDuration_Success(t *testing.T) {
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

	jobs := []*Job{
		{
			par2Path: "/data/test1" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 30 * time.Minute,
				},
			},
		},
		{
			par2Path: "/data/test2" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 20 * time.Minute,
				},
			},
		},
	}

	args := Options{}
	_ = args.MaxDuration.Set("1h")

	prog.considerDurations(jobs, args)

	require.NotContains(t, logBuf.String(), "unknown duration")
	require.NotContains(t, logBuf.String(), "exceed --duration")
}

// Expectation: No warning should be logged when no max duration is set.
func Test_Service_considerDurations_NoMaxDuration_Success(t *testing.T) {
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

	jobs := []*Job{
		{
			par2Path: "/data/test" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 0,
				},
			},
		},
	}

	args := Options{}

	prog.considerDurations(jobs, args)

	require.NotContains(t, logBuf.String(), "unknown duration")
	require.NotContains(t, logBuf.String(), "exceed --duration")
}

// Expectation: Nothing should panic if no jobs are given to the function.
func Test_Service_considerDurations_NoJobs_Success(t *testing.T) {
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

	jobs := []*Job{}
	args := Options{}

	prog.considerDurations(jobs, args)
	require.Empty(t, logBuf.String())
}
