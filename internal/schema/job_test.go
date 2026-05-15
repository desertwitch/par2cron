package schema

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Expectation: Important constants should not have changed.
func Test_MetaVersion_Constant_Success(t *testing.T) {
	t.Parallel()

	require.Equal(t, uint8(1), MetaVersion)
}

// Expectation: A new job meta without manifest only contains base metadata.
func Test_NewJobMeta_NilManifest_Success(t *testing.T) {
	t.Parallel()

	meta := NewJobMeta("test"+Par2Extension, nil, false)

	require.Equal(t, "test"+Par2Extension, meta.Par2Path)
	require.Equal(t, MetaVersion, meta.MetaVersion)
	require.False(t, meta.IsBundle)

	require.False(t, meta.HasManifest)
	require.False(t, meta.HasCreation)
	require.False(t, meta.HasVerification)

	require.False(t, meta.Walked)
	require.False(t, meta.RepairNeeded)
	require.False(t, meta.RepairPossible)
	require.Zero(t, meta.CountCorrupted)
	require.Zero(t, meta.VerifyTime)
	require.Zero(t, meta.VerifyDuration)
}

// Expectation: Bundle flag should be copied into the job meta.
func Test_NewJobMeta_IsBundle_Success(t *testing.T) {
	t.Parallel()

	meta := NewJobMeta("bundle"+Par2Extension, nil, true)

	require.True(t, meta.IsBundle)
	require.Equal(t, "bundle"+Par2Extension, meta.Par2Path)
	require.Equal(t, MetaVersion, meta.MetaVersion)
}

// Expectation: A manifest without creation or verification only marks HasManifest.
func Test_NewJobMeta_ManifestOnly_Success(t *testing.T) {
	t.Parallel()

	mf := NewManifest("test" + Par2Extension)

	meta := NewJobMeta("test"+Par2Extension, mf, false)

	require.True(t, meta.HasManifest)
	require.False(t, meta.HasCreation)
	require.False(t, meta.HasVerification)

	require.Zero(t, meta.VerifyTime)
	require.Zero(t, meta.VerifyDuration)
	require.False(t, meta.RepairNeeded)
	require.False(t, meta.RepairPossible)
	require.Zero(t, meta.CountCorrupted)
}

// Expectation: Creation metadata should be detected when present.
func Test_NewJobMeta_WithCreation_Success(t *testing.T) {
	t.Parallel()

	mf := NewManifest("test" + Par2Extension)
	mf.Creation = NewCreationManifest()

	meta := NewJobMeta("test"+Par2Extension, mf, false)

	require.True(t, meta.HasManifest)
	require.True(t, meta.HasCreation)
	require.False(t, meta.HasVerification)

	require.Zero(t, meta.VerifyTime)
	require.Zero(t, meta.VerifyDuration)
	require.False(t, meta.RepairNeeded)
	require.False(t, meta.RepairPossible)
	require.Zero(t, meta.CountCorrupted)
}

// Expectation: Verification metadata should be copied when present.
func Test_NewJobMeta_WithVerification_Success(t *testing.T) {
	t.Parallel()

	verifyTime := time.Date(2023, 10, 1, 10, 0, 0, 0, time.UTC)
	verifyDuration := 5 * time.Second

	mf := NewManifest("test" + Par2Extension)
	mf.Verification = NewVerificationManifest()
	mf.Verification.Time = verifyTime
	mf.Verification.Duration = verifyDuration
	mf.Verification.RepairNeeded = true
	mf.Verification.RepairPossible = true
	mf.Verification.CountCorrupted = 3

	meta := NewJobMeta("test"+Par2Extension, mf, false)

	require.True(t, meta.HasManifest)
	require.False(t, meta.HasCreation)
	require.True(t, meta.HasVerification)

	require.Equal(t, verifyTime, meta.VerifyTime)
	require.Equal(t, verifyDuration, meta.VerifyDuration)
	require.True(t, meta.RepairNeeded)
	require.True(t, meta.RepairPossible)
	require.Equal(t, 3, meta.CountCorrupted)
}

// Expectation: Creation and verification metadata can both be detected.
func Test_NewJobMeta_WithCreationAndVerification_Success(t *testing.T) {
	t.Parallel()

	verifyTime := time.Date(2023, 10, 1, 10, 0, 0, 0, time.UTC)
	verifyDuration := 250 * time.Millisecond

	mf := NewManifest("test" + Par2Extension)
	mf.Creation = NewCreationManifest()
	mf.Verification = NewVerificationManifest()
	mf.Verification.Time = verifyTime
	mf.Verification.Duration = verifyDuration
	mf.Verification.RepairNeeded = true
	mf.Verification.RepairPossible = false
	mf.Verification.CountCorrupted = 7

	meta := NewJobMeta("test"+Par2Extension, mf, true)

	require.Equal(t, "test"+Par2Extension, meta.Par2Path)
	require.Equal(t, MetaVersion, meta.MetaVersion)
	require.True(t, meta.IsBundle)

	require.True(t, meta.HasManifest)
	require.True(t, meta.HasCreation)
	require.True(t, meta.HasVerification)

	require.Equal(t, verifyTime, meta.VerifyTime)
	require.Equal(t, verifyDuration, meta.VerifyDuration)
	require.True(t, meta.RepairNeeded)
	require.False(t, meta.RepairPossible)
	require.Equal(t, 7, meta.CountCorrupted)
}
