package repair

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: A new repair job should be returned with the correct values.
func Test_NewRepairJob_Success(t *testing.T) {
	t.Parallel()

	args := Options{
		Par2Args:     []string{"-v"},
		Par2Verify:   true,
		PurgeBackups: true,
	}

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	job := NewRepairJob("/data/test"+schema.Par2Extension, args, mf)

	require.Equal(t, "/data", job.workingDir)
	require.Equal(t, "test"+schema.Par2Extension, job.par2Name)
	require.Equal(t, "/data/test"+schema.Par2Extension, job.par2Path)
	require.Equal(t, []string{"-v"}, job.par2Args)
	require.True(t, job.par2Verify)
	require.Equal(t, "test"+schema.Par2Extension+schema.ManifestExtension, job.manifestName)
	require.Equal(t, "/data/test"+schema.Par2Extension+schema.ManifestExtension, job.manifestPath)
	require.Equal(t, "/data/test"+schema.Par2Extension+schema.LockExtension, job.lockPath)
	require.True(t, job.purgeBackups)

	require.Equal(t, mf, job.manifest)
}

// Expectation: NewRepairJob should clone args to prevent external modification.
func Test_NewRepairJob_ArgsCloned_Success(t *testing.T) {
	t.Parallel()

	originalArgs := []string{"-v", "-q"}
	args := Options{
		Par2Args:   originalArgs,
		Par2Verify: true,
	}

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	job := NewRepairJob("/data/test"+schema.Par2Extension, args, mf)

	// Modify original args
	originalArgs[0] = "modified"

	// Job args should not be affected
	require.Equal(t, []string{"-v", "-q"}, job.par2Args)
}

// Expectation: The program should run the repair with the correct outcome.
func Test_Service_Repair_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}
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

	var called bool
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called = true

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)
	args := Options{Par2Args: []string{"-v"}}
	require.NoError(t, prog.Repair(t.Context(), "/data", args))

	require.True(t, called)
	require.Contains(t, logBuf.String(), "Job completed with success")
}

// Expectation: A locked file should not fail the repair process.
func Test_Service_Repair_FileLocked_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}
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

	var called bool
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called = true

			return schema.ErrFileIsLocked
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)
	args := Options{Par2Args: []string{"-v"}}
	require.NoError(t, prog.Repair(t.Context(), "/data", args))

	require.True(t, called)
	require.Contains(t, logBuf.String(), "Job unavailable (will retry next run)")
}

// Expectation: The program should run the repair with the correct outcome on generic error.
func Test_Service_Repair_Generic_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}
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

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			return errors.New("generic error")
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	args := Options{Par2Args: []string{"-v"}}
	require.ErrorIs(t, prog.Repair(t.Context(), "/data", args), schema.ErrExitPartialFailure)

	require.Contains(t, logBuf.String(), "Job failure (will retry next run)")
}

// Expectation: The program should run the repair with multiple jobs successfully.
func Test_Service_Repair_MultipleJobs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/test2"+schema.Par2Extension, []byte("par2data"), 0o644))

	for _, name := range []string{"test", "test2"} {
		mf := schema.NewManifest(t.Context(), name+schema.Par2Extension)
		mf.Verification = &schema.VerificationManifest{
			RepairNeeded:   true,
			RepairPossible: true,
		}
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
	require.NoError(t, prog.Repair(t.Context(), "/data", args))

	require.Equal(t, 2, called)
	require.Equal(t, 2, strings.Count(logBuf.String(), "Job completed with success"))
}

// Expectation: The program should handle multiple jobs with one failing.
func Test_Service_Repair_MultipleJobs_OneFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/test2"+schema.Par2Extension, []byte("par2data"), 0o644))

	for _, name := range []string{"test", "test2"} {
		mf := schema.NewManifest(t.Context(), name+schema.Par2Extension)
		mf.Verification = &schema.VerificationManifest{
			RepairNeeded:   true,
			RepairPossible: true,
		}
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
	require.ErrorIs(t, prog.Repair(t.Context(), "/data", args), schema.ErrExitPartialFailure)

	require.Equal(t, 2, called)
	require.Equal(t, 1, strings.Count(logBuf.String(), "Job completed with success"))
	require.Equal(t, 1, strings.Count(logBuf.String(), "Job failure (will retry next run)"))
}

// Expectation: The program should continue if an enumeration partial (non-fatal) failure occurs.
func Test_Service_Repair_MultipleJobs_EnumerationFails_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/data/test1"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(baseFs, "/data/test2"+schema.Par2Extension, []byte("par2"), 0o644))

	for _, name := range []string{"test1", "test2"} {
		mf := schema.NewManifest(t.Context(), name+schema.Par2Extension)
		mf.Verification = &schema.VerificationManifest{
			RepairNeeded:   true,
			RepairPossible: true,
		}
		mfData, err := json.Marshal(mf)
		require.NoError(t, err)
		require.NoError(t, afero.WriteFile(baseFs, "/data/"+name+schema.Par2Extension+schema.ManifestExtension, mfData, 0o644))
	}

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
	err := prog.Repair(t.Context(), "/data", args)

	require.ErrorIs(t, err, schema.ErrExitPartialFailure)
	require.ErrorIs(t, err, schema.ErrNonFatal)
	require.Equal(t, 1, called)
	require.Equal(t, 1, strings.Count(logBuf.String(), "Job completed with success"))
	require.Equal(t, 1, strings.Count(logBuf.String(), "Failed to read par2cron manifest"))
}

// Expectation: The program should recognize when there's nothing to do.
func Test_Service_Repair_NoJobs_Success(t *testing.T) {
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
	require.NoError(t, prog.Repair(t.Context(), "/data", args))

	require.Contains(t, logBuf.String(), "Nothing to do")
}

// Expectation: The repair should respect a context cancellation.
func Test_Service_Repair_CtxCancel_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}
	mfData, err := json.Marshal(mf)
	require.NoError(t, err)
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, mfData, 0o644))

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
	err = prog.Repair(ctx, "/data", args)

	require.ErrorIs(t, err, context.Canceled)
}

// Expectation: The repair should respect max duration deadline.
func Test_Service_Repair_MaxDuration_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/test2"+schema.Par2Extension, []byte("par2data"), 0o644))

	for _, name := range []string{"test", "test2"} {
		mf := schema.NewManifest(t.Context(), name+schema.Par2Extension)
		mf.Verification = &schema.VerificationManifest{
			RepairNeeded:   true,
			RepairPossible: true,
		}
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
	_ = ls.LogLevel.Set("info")

	var called int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called++

			time.Sleep(50 * time.Millisecond)

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	args := Options{Par2Args: []string{"-v"}}
	_ = args.MaxDuration.Set("1ms")

	require.NoError(t, prog.Repair(t.Context(), "/data", args))

	require.GreaterOrEqual(t, called, 1)
	require.Contains(t, logBuf.String(), "Exceeded the --duration budget")
}

// Expectation: The correct job should be returned when manifest indicates repair is needed and possible.
func Test_Service_Enumerate_RepairNeeded_RepairPossible_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-v"}}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NotNil(t, jobs[0].manifest)
}

// Expectation: No job should be returned when repair is not needed.
func Test_Service_Enumerate_RepairNotNeeded_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   false,
		RepairPossible: true,
	}

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-v"}}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Empty(t, jobs)
	require.Contains(t, logBuf.String(), "Not a candidate for repair")
}

// Expectation: No job should be returned when repair is impossible without --attempt-unrepairables.
func Test_Service_Enumerate_RepairNeeded_RepairImpossible_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: false,
	}

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{
		Par2Args:             []string{"-v"},
		AttemptUnrepairables: false,
	}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Empty(t, jobs)
	require.Contains(t, logBuf.String(), "Not a candidate for repair")
}

// Expectation: A job should be returned when the min-tested count is met.
func Test_Service_Enumerate_RepairNeeded_MinTestedCount_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		CountCorrupted: 1,
		RepairNeeded:   true,
		RepairPossible: true,
	}

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{
		Par2Args:       []string{"-v"},
		MinTestedCount: 1,
	}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
}

// Expectation: No job should be returned when the min-tested count is not met.
func Test_Service_Enumerate_RepairNeeded_MinTestedCount_NotMet_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		CountCorrupted: 1,
		RepairNeeded:   true,
		RepairPossible: true,
	}

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{
		Par2Args:       []string{"-v"},
		MinTestedCount: 2,
	}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Empty(t, jobs)
	require.Contains(t, logBuf.String(), "Not a candidate for repair")
}

// Expectation: Job should be returned when repair is impossible but --attempt-unrepairables is set.
func Test_Service_Enumerate_AttemptUnrepairables_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: false,
	}

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{
		Par2Args:             []string{"-v"},
		AttemptUnrepairables: true,
	}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
}

// Expectation: No job should be returned when there's no verification manifest.
func Test_Service_Enumerate_NoVerificationManifest_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = nil

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-v"}}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Empty(t, jobs)
	require.Contains(t, logBuf.String(), "No verification manifest")
}

// Expectation: No job should be returned when there's no par2cron manifest file.
func Test_Service_Enumerate_NoManifestFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

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
	require.Empty(t, jobs)
	require.Contains(t, logBuf.String(), "Failed to find par2cron manifest")
}

// Expectation: No job should be returned when manifest is invalid JSON.
func Test_Service_Enumerate_InvalidManifest_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, []byte("invalid json"), 0o644))

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
	require.Empty(t, jobs)
	require.Contains(t, logBuf.String(), "Failed to unmarshal par2cron manifest")
}

// Expectation: No job should be returned when --skip-not-created is set and no creation manifest exists.
func Test_Service_Enumerate_SkipNotCreated_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Creation = nil
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{
		Par2Args:       []string{"-v"},
		SkipNotCreated: true,
	}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Empty(t, jobs)
	require.Contains(t, logBuf.String(), "No creation manifest")
}

// Expectation: A partial failure should be returned when reading the manifest fails.
func Test_Service_Enumerate_ReadManifestFailure_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}
	mfData, err := json.Marshal(mf)
	require.NoError(t, err)
	require.NoError(t, afero.WriteFile(baseFs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, mfData, 0o644))

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
	_, err = prog.Enumerate(t.Context(), "/data", args)

	require.ErrorIs(t, err, schema.ErrNonFatal)
	require.Contains(t, logBuf.String(), "Failed to read par2cron manifest")
}

// Expectation: An empty slice should be returned if no PAR2 files are found.
func Test_Service_Enumerate_NoJobs_Success(t *testing.T) {
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

	args := Options{Par2Args: []string{"-v"}}
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Empty(t, jobs)
}

// Expectation: Elements in directories with an ignore file should be skipped non-recursively.
func Test_Service_Enumerate_IgnoreFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/data/"+schema.IgnoreFile, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/ignored"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/subdir/notignored"+schema.Par2Extension, []byte("par2"), 0o644))

	// Create manifests for both PAR2 files
	for _, path := range []string{"/data/ignored", "/data/subdir/notignored"} {
		mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
		mf.Verification = &schema.VerificationManifest{
			RepairNeeded:   true,
			RepairPossible: true,
		}
		mfData, err := json.Marshal(mf)
		require.NoError(t, err)
		require.NoError(t, afero.WriteFile(fs, path+schema.Par2Extension+schema.ManifestExtension, mfData, 0o644))
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
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, "/data/subdir/notignored"+schema.Par2Extension, jobs[0].par2Path)
}

// Expectation: Elements in directories with an ignore-all file should be skipped recursively.
func Test_Service_Enumerate_IgnoreAllFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/data/normal/test"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/ignored/"+schema.IgnoreAllFile, []byte(""), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/ignored/sub/test"+schema.Par2Extension, []byte("par2"), 0o644))

	// Create manifests for all PAR2 files
	for _, path := range []string{"/data/normal/test", "/data/ignored/test", "/data/ignored/sub/test"} {
		mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
		mf.Verification = &schema.VerificationManifest{
			RepairNeeded:   true,
			RepairPossible: true,
		}
		mfData, err := json.Marshal(mf)
		require.NoError(t, err)
		require.NoError(t, afero.WriteFile(fs, path+schema.Par2Extension+schema.ManifestExtension, mfData, 0o644))
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
	jobs, err := prog.Enumerate(t.Context(), "/data", args)

	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, "/data/normal/test"+schema.Par2Extension, jobs[0].par2Path)
}

// Expectation: Context cancellation should be respected during enumeration.
func Test_Service_Enumerate_CtxCancel_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

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

// Expectation: The repair should pass and a manifest be updated.
func Test_Service_runRepair_Success(t *testing.T) {
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
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}

	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     []string{"-v"},
		par2Verify:   false,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		lockPath:     "/data/test" + schema.Par2Extension + schema.LockExtension,
		manifest:     mf,
	}

	require.NoError(t, prog.runRepair(t.Context(), job))

	require.NotNil(t, job.manifest.Repair)
	require.NotZero(t, job.manifest.Repair.Duration)
	require.Equal(t, schema.Par2ExitCodeSuccess, job.manifest.Repair.ExitCode)
	require.Equal(t, 1, job.manifest.Repair.Count)

	manifestExists, _ := afero.Exists(fs, job.manifestPath)
	require.True(t, manifestExists)
}

// Expectation: The repair should pass and the backup files be purged after.
func Test_Service_runRepair_PurgeBackups_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test.txt", []byte("original file"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/other.txt", []byte("original file"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2 file"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/old.file.1", []byte("unrelated file"), 0o644))

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

			require.NoError(t, afero.WriteFile(fs, "/data/test.txt.1", []byte("backup file"), 0o644))
			require.NoError(t, afero.WriteFile(fs, "/data/other.txt.2", []byte("backup file"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}

	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     []string{"-v"},
		par2Verify:   false,
		purgeBackups: true,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		lockPath:     "/data/test" + schema.Par2Extension + schema.LockExtension,
		manifest:     mf,
	}

	require.NoError(t, prog.runRepair(t.Context(), job))

	require.NotNil(t, job.manifest.Repair)
	require.NotZero(t, job.manifest.Repair.Duration)
	require.Equal(t, schema.Par2ExitCodeSuccess, job.manifest.Repair.ExitCode)
	require.Equal(t, 1, job.manifest.Repair.Count)

	require.Equal(t, 1, called)

	manifestExists, _ := afero.Exists(fs, job.manifestPath)
	require.True(t, manifestExists)

	backup1Exists, _ := afero.Exists(fs, "/data/test.txt.1")
	require.False(t, backup1Exists)

	backup2Exists, _ := afero.Exists(fs, "/data/other.txt.2")
	require.False(t, backup2Exists)

	backup3Exists, _ := afero.Exists(fs, "/data/old.file.1")
	require.True(t, backup3Exists)
}

// Expectation: The repair should use the correct arguments.
func Test_Service_runRepair_CorrectArgs_Success(t *testing.T) {
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

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}

	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     []string{"-v", "-q"},
		par2Verify:   false,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		lockPath:     "/data/test" + schema.Par2Extension + schema.LockExtension,
		manifest:     mf,
	}

	require.NoError(t, prog.runRepair(t.Context(), job))

	require.Equal(t, "par2", runCmd)
	require.Equal(t, []string{
		"repair",
		"-v",
		"-q",
		"--",
		job.par2Path,
	}, runArgs)
}

// Expectation: The repair count should increment on subsequent repairs.
func Test_Service_runRepair_IncrementCount_Success(t *testing.T) {
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
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}
	mf.Repair = &schema.RepairManifest{
		Count: 5,
	}

	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     []string{"-v"},
		par2Verify:   false,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		lockPath:     "/data/test" + schema.Par2Extension + schema.LockExtension,
		manifest:     mf,
	}

	require.NoError(t, prog.runRepair(t.Context(), job))

	require.Equal(t, 6, job.manifest.Repair.Count)
}

// Expectation: A non-exit-code related error should return early that error.
func Test_Service_runRepair_GenericError_Error(t *testing.T) {
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
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			return errors.New("test error")
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}

	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     []string{"-v"},
		par2Verify:   false,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		lockPath:     "/data/test" + schema.Par2Extension + schema.LockExtension,
		manifest:     mf,
	}

	require.ErrorContains(t, prog.runRepair(t.Context(), job), "test error")
	require.Contains(t, logBuf.String(), "Failed to repair PAR2")
}

// Expectation: An exit-code related error should return early that error.
func Test_Service_runRepair_NonZeroExitCode_Error(t *testing.T) {
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
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			return testutil.CreateExitError(t, ctx, 1)
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}

	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     []string{"-v"},
		par2Verify:   false,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		lockPath:     "/data/test" + schema.Par2Extension + schema.LockExtension,
		manifest:     mf,
	}

	require.ErrorContains(t, prog.runRepair(t.Context(), job), "1")
	require.Contains(t, logBuf.String(), "Failed to repair PAR2")
}

// Expectation: A manifest write error should log a warning but not fail the repair.
func Test_Service_runRepair_ManifestWriteError_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/data/test"+schema.Par2Extension, []byte("par2data"), 0o644))

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
			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}

	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     []string{"-v"},
		par2Verify:   false,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		lockPath:     "/data/test" + schema.Par2Extension + schema.LockExtension,
		manifest:     mf,
	}

	require.NoError(t, prog.runRepair(t.Context(), job))
	require.Contains(t, logBuf.String(), "Failed to write par2cron manifest")
}

// Expectation: The repair should trigger verification when par2Verify is true.
func Test_Service_runRepair_WithVerify_Success(t *testing.T) {
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
	_ = ls.LogLevel.Set("info")

	var calls []string
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			if len(args) > 0 {
				calls = append(calls, args[0]) // "repair" or "verify"
			}

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}

	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     []string{"-v"},
		par2Verify:   true,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		lockPath:     "/data/test" + schema.Par2Extension + schema.LockExtension,
		manifest:     mf,
	}

	require.NoError(t, prog.runRepair(t.Context(), job))

	require.Len(t, calls, 2)
	require.Equal(t, "repair", calls[0])
	require.Equal(t, "verify", calls[1])
}

// Expectation: The repair should fail if verification after repair fails.
func Test_Service_runRepair_VerifyFails_Error(t *testing.T) {
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
	_ = ls.LogLevel.Set("info")

	var callCount int
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			callCount++
			if callCount == 2 {
				// Fail on verify
				return errors.New("verify failed")
			}

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}

	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     []string{"-v"},
		par2Verify:   true,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		lockPath:     "/data/test" + schema.Par2Extension + schema.LockExtension,
		manifest:     mf,
	}

	require.ErrorContains(t, prog.runRepair(t.Context(), job), "failed to verify par2")
}

// Expectation: Args should be stored in the repair manifest.
func Test_Service_runRepair_StoresArgs_Success(t *testing.T) {
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
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}

	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     []string{"-v", "-q", "--custom"},
		par2Verify:   false,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		lockPath:     "/data/test" + schema.Par2Extension + schema.LockExtension,
		manifest:     mf,
	}

	require.NoError(t, prog.runRepair(t.Context(), job))

	require.Equal(t, []string{"-v", "-q", "--custom"}, job.manifest.Repair.Args)
}

// Expectation: Modifying original par2Args should not affect stored manifest args.
func Test_Service_runRepair_ArgsCloned_Success(t *testing.T) {
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
	_ = ls.LogLevel.Set("info")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), runner)

	mf := schema.NewManifest(t.Context(), "test"+schema.Par2Extension)
	mf.Verification = &schema.VerificationManifest{
		RepairNeeded:   true,
		RepairPossible: true,
	}

	originalArgs := []string{"-v", "-q"}
	job := &Job{
		workingDir:   "/data",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/test" + schema.Par2Extension,
		par2Args:     originalArgs,
		par2Verify:   false,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/test" + schema.Par2Extension + schema.ManifestExtension,
		lockPath:     "/data/test" + schema.Par2Extension + schema.LockExtension,
		manifest:     mf,
	}

	require.NoError(t, prog.runRepair(t.Context(), job))

	// Modify original args
	originalArgs[0] = "modified"

	// Stored args should not be affected
	require.Equal(t, []string{"-v", "-q"}, job.manifest.Repair.Args)
}
