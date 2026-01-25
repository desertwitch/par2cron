package schema

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Expectation: Important constants should not have changed.
func Test_Constants_Success(t *testing.T) {
	t.Parallel()

	require.Equal(t, 0, ExitCodeSuccess)
	require.Equal(t, 1, ExitCodePartialFailure)
	require.Equal(t, 2, ExitCodeBadInvocation)
	require.Equal(t, 3, ExitCodeRepairable)
	require.Equal(t, 4, ExitCodeUnrepairable)
	require.Equal(t, 5, ExitCodeUnclassified)

	require.Equal(t, 0, Par2ExitCodeSuccess)
	require.Equal(t, 1, Par2ExitCodeRepairPossible)
	require.Equal(t, 2, Par2ExitCodeRepairImpossible)

	require.Equal(t, ".par2", Par2Extension)
	require.Equal(t, ".lock", LockExtension)
	require.Equal(t, ".json", ManifestExtension)

	require.Equal(t, ".par2cron-ignore", IgnoreFile)
	require.Equal(t, ".par2cron-ignore-all", IgnoreAllFile)

	require.Equal(t, "file", CreateFileMode)
	require.Equal(t, "folder", CreateFolderMode)
}
