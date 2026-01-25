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
