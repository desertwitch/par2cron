package util

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/stretchr/testify/require"
)

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
