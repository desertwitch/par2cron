package util

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/stretchr/testify/require"
)

// Expectation: NewResultTracker should initialize with zero counts.
func Test_NewResultTracker_ZeroValues_Success(t *testing.T) {
	t.Parallel()

	tracker := NewResultTracker(slog.Default())
	require.Equal(t, 0, tracker.Success)
	require.Equal(t, 0, tracker.Skipped)
	require.Equal(t, 0, tracker.Error)
}

// Expectation: PrintCompletionInfo should log the correct totals.
func Test_ResultTracker_PrintCompletionInfo_Success(t *testing.T) {
	t.Parallel()

	var buf testutil.SafeBuffer
	log := slog.New(slog.NewTextHandler(&buf, nil))

	tracker := NewResultTracker(log)
	tracker.Success = 5
	tracker.Skipped = 2
	tracker.Error = 1

	tracker.PrintCompletionInfo(8)

	output := buf.String()
	require.Contains(t, output, "Operation complete (8/8 jobs processed)")
	require.Contains(t, output, "successCount=5")
	require.Contains(t, output, "skipCount=2")
	require.Contains(t, output, "errorCount=1")
	require.Contains(t, output, "processedCount=8")
	require.Contains(t, output, "selectedCount=8")
}

// Expectation: PrintCompletionInfo should handle zero counts.
func Test_ResultTracker_PrintCompletionInfo_ZeroCounts_Success(t *testing.T) {
	t.Parallel()

	var buf testutil.SafeBuffer
	log := slog.New(slog.NewTextHandler(&buf, nil))

	tracker := NewResultTracker(log)
	tracker.PrintCompletionInfo(5)

	output := buf.String()
	require.Contains(t, output, "Operation complete (0/5 jobs processed)")
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
