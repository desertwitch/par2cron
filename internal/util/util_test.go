package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Expectation: NewResultTracker should initialize with zero counts.
func Test_NewResultTracker_ZeroValues_Success(t *testing.T) {
	t.Parallel()

	tracker := NewResultTracker()
	require.Equal(t, 0, tracker.Selected)
	require.Equal(t, 0, tracker.Success)
	require.Equal(t, 0, tracker.Skipped)
	require.Equal(t, 0, tracker.Error)
}
