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

// Expectation: The function should perform according to the table's expectations.
func Test_OnlyContains_Table(t *testing.T) {
	t.Parallel()

	sentinel := schema.ErrFileIsLocked
	other := schema.ErrManifestMismatch
	errSubjob := errors.New("subjob failed")

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error returns false",
			err:      nil,
			expected: false,
		},
		{
			name:     "bare sentinel matches",
			err:      sentinel,
			expected: true,
		},
		{
			name:     "bare non-sentinel does not match",
			err:      other,
			expected: false,
		},
		{
			name:     "single-wrapped sentinel matches",
			err:      fmt.Errorf("context: %w", sentinel),
			expected: true,
		},
		{
			name:     "single-wrapped non-sentinel does not match",
			err:      fmt.Errorf("context: %w", other),
			expected: false,
		},
		{
			name:     "joined single sentinel matches",
			err:      errors.Join(sentinel),
			expected: true,
		},
		{
			name:     "joined all-sentinel matches",
			err:      errors.Join(sentinel, sentinel, sentinel),
			expected: true,
		},
		{
			name:     "joined mixed does not match",
			err:      errors.Join(sentinel, other),
			expected: false,
		},
		{
			name:     "joined all-non-sentinel does not match",
			err:      errors.Join(other, other),
			expected: false,
		},
		{
			name:     "single-wrap around joined mixed does not match",
			err:      fmt.Errorf("failed to create par2: %w", errors.Join(sentinel, other)),
			expected: false,
		},
		{
			name:     "single-wrap around joined all-sentinel matches",
			err:      fmt.Errorf("failed to create par2: %w", errors.Join(sentinel, sentinel)),
			expected: true,
		},
		{
			name:     "double-wrapped joined mixed does not match",
			err:      fmt.Errorf("outer: %w", fmt.Errorf("par2: %w", errors.Join(sentinel, other))),
			expected: false,
		},
		{
			name:     "double-wrapped joined all-sentinel matches",
			err:      fmt.Errorf("outer: %w", fmt.Errorf("par2: %w", errors.Join(sentinel, sentinel))),
			expected: true,
		},
		{
			name:     "nested join all-sentinel matches",
			err:      errors.Join(errors.Join(sentinel, sentinel), errors.Join(sentinel, sentinel)),
			expected: true,
		},
		{
			name:     "nested join with one deep non-sentinel does not match",
			err:      errors.Join(errors.Join(sentinel, sentinel), errors.Join(sentinel, other)),
			expected: false,
		},
		{
			name:     "join of single-wrapped sentinels matches",
			err:      errors.Join(fmt.Errorf("file A: %w", sentinel), fmt.Errorf("file B: %w", sentinel)),
			expected: true,
		},
		{
			name:     "join of single-wrapped mixed does not match",
			err:      errors.Join(fmt.Errorf("file A: %w", sentinel), fmt.Errorf("file B: %w", other)),
			expected: false,
		},
		{
			name: "subjob failure plus locked join behind single-wrap does not match",
			err: fmt.Errorf("failed to create par2: %w",
				fmt.Errorf("%w: context: %w", errSubjob, errors.Join(sentinel, sentinel)),
			),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, OnlyContains(tt.err, sentinel))
		})
	}
}
