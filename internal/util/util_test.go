package util

import (
	"strings"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/schema"
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

// Expectation: The PAR2 base should be identified as case-insensitive.
func Test_IsPar2Base_ValidBase_Success(t *testing.T) {
	t.Parallel()

	require.True(t, IsPar2Base("/data/test"+schema.Par2Extension))
	require.True(t, IsPar2Base("/data/test"+strings.ToUpper(schema.Par2Extension)))
	require.True(t, IsPar2Base("test"+schema.Par2Extension))
}

// Expectation: The PAR2 volumes should not be identified as base.
func Test_IsPar2Base_VolumeFile_Success(t *testing.T) {
	t.Parallel()

	require.False(t, IsPar2Base("/data/test.vol01+02"+schema.Par2Extension))
	require.False(t, IsPar2Base("/data/test.vol10+20"+strings.ToUpper(schema.Par2Extension)))
}

// Expectation: Other unrelated files should not be identified as base.
func Test_IsPar2Base_NonPar2_Success(t *testing.T) {
	t.Parallel()

	require.False(t, IsPar2Base("/data/test.txt"))
	require.False(t, IsPar2Base("/data/test.par"))
	require.False(t, IsPar2Base("/data/test"))
}

// Expectation: The duration should be formatted to string with success.
func Test_FmtDur_Success(t *testing.T) {
	t.Parallel()

	result := FmtDur(90 * time.Minute)

	require.NotEmpty(t, result)
	require.NotEqual(t, "?", result)
}

// Expectation: The duration should be formatted to string with success.
func Test_FmtDur_Negative_Success(t *testing.T) {
	t.Parallel()

	result := FmtDur(-1)

	require.NotEmpty(t, result)
	require.NotEqual(t, "?", result)
}

// Expectation: The duration should be formatted to string with success.
func Test_FmtDur_ZeroDuration_Success(t *testing.T) {
	t.Parallel()

	result := FmtDur(0)

	require.NotEmpty(t, result)
	require.NotEqual(t, "?", result)
}
