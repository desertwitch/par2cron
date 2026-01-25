package schema

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// Expectation: The correct exit code should be returned.
func Test_ExitCodeFor_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "nil error returns success",
			err:      nil,
			expected: ExitCodeSuccess,
		},
		{
			name:     "ErrExitBadInvocation returns bad invocation code",
			err:      ErrExitBadInvocation,
			expected: ExitCodeBadInvocation,
		},
		{
			name:     "ErrExitUnrepairable returns unrepairable code",
			err:      ErrExitUnrepairable,
			expected: ExitCodeUnrepairable,
		},
		{
			name:     "ErrExitPartialFailure returns partial failure code",
			err:      ErrExitPartialFailure,
			expected: ExitCodePartialFailure,
		},
		{
			name:     "ErrExitRepairable returns repairable code",
			err:      ErrExitRepairable,
			expected: ExitCodeRepairable,
		},
		{
			name:     "multiple known errors returns highest error",
			err:      fmt.Errorf("wrapped: %w: %w", ErrExitPartialFailure, ErrExitBadInvocation),
			expected: ExitCodeBadInvocation,
		},
		{
			name:     "unknown error returns unclassified error code",
			err:      errors.New("some random error"),
			expected: ExitCodeUnclassified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ExitCodeFor(tt.err)
			require.Equal(t, tt.expected, result)
		})
	}
}
