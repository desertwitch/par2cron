package list

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Expectation: The status names should match the documented output values.
func Test_jobStatus_String_Success(t *testing.T) {
	t.Parallel()

	require.Equal(t, "unrepairable", statusUnrepairable.String())
	require.Equal(t, "repairable", statusRepairable.String())
	require.Equal(t, "unverified", statusUnverified.String())
	require.Equal(t, "healthy", statusHealthy.String())
}

// Expectation: The status constants should be ordered by severity (most urgent first).
func Test_jobStatus_SeverityOrder_Success(t *testing.T) {
	t.Parallel()

	require.Less(t, statusUnrepairable, statusRepairable)
	require.Less(t, statusRepairable, statusUnverified)
	require.Less(t, statusUnverified, statusHealthy)
}

// Expectation: statusOf should return unverified for a job without a manifest.
func Test_statusOf_NoManifest_Success(t *testing.T) {
	t.Parallel()

	m := newTestMeta("/data/test.par2")

	require.Equal(t, statusUnverified, statusOf(m))
}

// Expectation: statusOf should return unverified for a job without verification data.
func Test_statusOf_NoVerification_Success(t *testing.T) {
	t.Parallel()

	m := newTestMeta("/data/test.par2")
	m.HasManifest = true

	require.Equal(t, statusUnverified, statusOf(m))
}

// Expectation: statusOf should return unrepairable when repair is needed but not possible.
func Test_statusOf_Unrepairable_Success(t *testing.T) {
	t.Parallel()

	m := newTestMeta("/data/test.par2")
	m.HasManifest = true
	m.HasVerification = true
	m.RepairNeeded = true
	m.RepairPossible = false

	require.Equal(t, statusUnrepairable, statusOf(m))
}

// Expectation: statusOf should return repairable when repair is needed and possible.
func Test_statusOf_Repairable_Success(t *testing.T) {
	t.Parallel()

	m := newTestMeta("/data/test.par2")
	m.HasManifest = true
	m.HasVerification = true
	m.RepairNeeded = true
	m.RepairPossible = true

	require.Equal(t, statusRepairable, statusOf(m))
}

// Expectation: statusOf should return healthy for a verified job without corruption.
func Test_statusOf_Healthy_Success(t *testing.T) {
	t.Parallel()

	m := newTestMeta("/data/test.par2")
	m.HasManifest = true
	m.HasVerification = true

	require.Equal(t, statusHealthy, statusOf(m))
}
