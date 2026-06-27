package util

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/stretchr/testify/require"
)

// Expectation: The runner should be creatable with a valid cgroup file option.
func Test_NewCtxRunner_WithCgroup_Success(t *testing.T) {
	t.Parallel()

	cgroupFile := filepath.Join(t.TempDir(), "cgroup.procs")
	require.NoError(t, os.WriteFile(cgroupFile, nil, 0o600))

	runner, err := NewCtxRunner(WithCgroup(cgroupFile))
	require.NoError(t, err)
	require.NotNil(t, runner.CgroupFile)

	require.NoError(t, runner.Close())
}

// Expectation: The runner should return an error when the cgroup path does not exist.
func Test_NewCtxRunner_WithCgroup_InvalidPath_Error(t *testing.T) {
	t.Parallel()

	runner, err := NewCtxRunner(WithCgroup(filepath.Join(t.TempDir(), "nonexistent")))
	require.Error(t, err)
	require.Nil(t, runner)
}

// Expectation: The runner should be creatable without any options.
func Test_NewCtxRunner_NoOptions_Success(t *testing.T) {
	t.Parallel()

	runner, err := NewCtxRunner()
	require.NoError(t, err)
	require.NotNil(t, runner)
	require.Nil(t, runner.CgroupFile)
}

// Expectation: WithCgroup should clean traversal sequences from the path.
func Test_WithCgroup_PathCleaning(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cgroupFile := filepath.Join(dir, "cgroup.procs")
	require.NoError(t, os.WriteFile(cgroupFile, nil, 0o600))

	dirtyPath := filepath.Join(dir, "..", filepath.Base(dir), "cgroup.procs")
	runner, err := NewCtxRunner(WithCgroup(dirtyPath))
	require.NoError(t, err)
	require.NotNil(t, runner.CgroupFile)

	require.NoError(t, runner.Close())
}

// Expectation: The runner should be able to run a basic command.
func Test_CtxRunner_Run_Success(t *testing.T) {
	t.Parallel()

	runner := CtxRunner{}

	err := runner.Run(t.Context(), "echo", []string{"test"}, "/tmp", io.Discard, io.Discard)

	require.NoError(t, err)
}

// Expectation: The runner should be respect the set working directory.
func Test_CtxRunner_Run_WorkingDir(t *testing.T) {
	t.Parallel()

	runner := CtxRunner{}
	var stdout testutil.SafeBuffer
	workingDir := "/tmp"

	err := runner.Run(
		t.Context(),
		"pwd",
		nil,
		workingDir,
		&stdout,
		io.Discard,
	)

	require.NoError(t, err)

	got := strings.TrimSpace(stdout.String())
	require.Equal(t, workingDir, got)
}

// Expectation: The runner should respect a cancellation and return the correct error.
func Test_CtxRunner_Run_CtxCancel_BeforeRun_Error(t *testing.T) {
	t.Parallel()

	runner := CtxRunner{}

	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond)

	err := runner.Run(ctx, "sleep", []string{"10"}, "/tmp", io.Discard, io.Discard)

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

// Expectation: The runner should respect a cancellation and return the correct error.
func Test_CtxRunner_Run_CtxCancel_AfterRun_Error(t *testing.T) {
	t.Parallel()

	runner := CtxRunner{}

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	err := runner.Run(ctx, "sleep", []string{"10"}, "/tmp", io.Discard, io.Discard)

	require.ErrorContains(t, err, "interrupt")
}

// Expectation: The runner should return an error when the binary is not found.
func Test_CtxRunner_Run_InvalidCommand_Error(t *testing.T) {
	t.Parallel()

	runner := CtxRunner{}

	err := runner.Run(t.Context(), "nonexistentcommand12345", []string{}, "/tmp", io.Discard, io.Discard)

	require.Error(t, err)
}

// Expectation: Close should be safe to call when no cgroup file is set.
func Test_CtxRunner_Close_NilCgroupFile(t *testing.T) {
	t.Parallel()

	runner := &CtxRunner{}
	require.NoError(t, runner.Close())
}

// Expectation: Close should close the underlying cgroup file descriptor.
func Test_CtxRunner_Close_ClosesCgroupFile(t *testing.T) {
	t.Parallel()

	cgroupFile := filepath.Join(t.TempDir(), "cgroup.procs")
	require.NoError(t, os.WriteFile(cgroupFile, nil, 0o600))

	runner, err := NewCtxRunner(WithCgroup(cgroupFile))
	require.NoError(t, err)

	fd := runner.CgroupFile.Fd()
	require.NoError(t, runner.Close())

	// Writing to a closed fd should fail.
	_, err = syscall.Write(int(fd), []byte("test"))
	require.Error(t, err)
}
