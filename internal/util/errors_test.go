package util

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/stretchr/testify/require"
)

// Expectation: A pointer to an exit code should be returned on exit error.
func Test_AsExitCode_ExitError_Error(t *testing.T) {
	t.Parallel()

	exitErr := &exec.ExitError{}
	code := AsExitCode(exitErr)

	require.NotNil(t, code)
}

// Expectation: A nil pointer should be returned on non-exit error.
func Test_AsExitCode_NonExitError_Error(t *testing.T) {
	t.Parallel()

	regularErr := exec.ErrNotFound
	code := AsExitCode(regularErr)

	require.Nil(t, code)
}

// Expectation: The highest error should be returned.
func Test_HighestError_Table_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		errs     []error
		expected error
	}{
		{
			name:     "empty slice returns nil",
			errs:     []error{},
			expected: nil,
		},
		{
			name:     "slice with only nils returns nil",
			errs:     []error{nil, nil, nil},
			expected: nil,
		},
		{
			name:     "single error returns that error",
			errs:     []error{schema.ErrExitRepairable},
			expected: schema.ErrExitRepairable,
		},
		{
			name:     "returns highest priority error",
			errs:     []error{schema.ErrExitRepairable, schema.ErrExitBadInvocation, schema.ErrExitPartialFailure},
			expected: schema.ErrExitRepairable,
		},
		{
			name:     "skips nil errors and returns highest",
			errs:     []error{nil, schema.ErrExitRepairable, nil, schema.ErrExitUnrepairable, nil},
			expected: schema.ErrExitUnrepairable,
		},
		{
			name:     "returns first occurrence when multiple have same priority",
			errs:     []error{schema.ErrExitUnclassified, errors.New("another error")},
			expected: schema.ErrExitUnclassified,
		},
		{
			name:     "wrapped error is recognized",
			errs:     []error{schema.ErrExitRepairable, fmt.Errorf("wrapped: %w", schema.ErrExitUnrepairable)},
			expected: fmt.Errorf("wrapped: %w", schema.ErrExitUnrepairable),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := HighestError(tt.errs)
			if tt.expected == nil {
				require.NoError(t, result)
			} else {
				require.Error(t, result)
				require.Equal(t, schema.ExitCodeFor(tt.expected), schema.ExitCodeFor(result))
			}
		})
	}
}

// Expectation: The function should return true when all errors match the sentinel.
func Test_OnlyContains_AllMatch_Success(t *testing.T) {
	t.Parallel()

	err := errors.Join(schema.ErrFileIsLocked, schema.ErrFileIsLocked)
	require.True(t, OnlyContains(err, schema.ErrFileIsLocked))
}

// Expectation: The function should return false when some errors do not match the sentinel.
func Test_OnlyContains_SomeDontMatch_Success(t *testing.T) {
	t.Parallel()

	err := errors.Join(schema.ErrFileIsLocked, errors.New("other error"))
	require.False(t, OnlyContains(err, schema.ErrFileIsLocked))
}

// Expectation: The function should return false for non-joined errors.
func Test_OnlyContains_NonJoined_Success(t *testing.T) {
	t.Parallel()

	err := errors.New("single error")
	require.False(t, OnlyContains(err, schema.ErrFileIsLocked))
}

// Expectation: The function should return true for non-joined errors of sentinel type.
func Test_OnlyContains_NonJoined_Sentinel_Success(t *testing.T) {
	t.Parallel()

	err := schema.ErrFileIsLocked
	require.True(t, OnlyContains(err, schema.ErrFileIsLocked))
}

// Expectation: The function should return true for a single joined error that matches.
func Test_OnlyContains_SingleJoined_Success(t *testing.T) {
	t.Parallel()

	err := errors.Join(schema.ErrFileIsLocked)
	require.True(t, OnlyContains(err, schema.ErrFileIsLocked))
}

// Expectation: The function should return false when the overall error is nil.
func Test_OnlyContains_NilErr_Success(t *testing.T) {
	t.Parallel()

	require.False(t, OnlyContains(nil, schema.ErrFileIsLocked))
}
