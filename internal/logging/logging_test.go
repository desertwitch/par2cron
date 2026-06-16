package logging

import (
	"log/slog"
	"testing"

	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/stretchr/testify/require"
)

// Expectation: A non-JSON handler should be returned.
func Test_NewLogger_WantTint_Success(t *testing.T) {
	t.Parallel()

	ls := Options{
		Logout:   &testutil.SafeBuffer{},
		WantJSON: false,
	}
	_ = ls.LogLevel.Set("info")

	logger := NewLogger(ls)
	_, ok := logger.Handler().(*slog.JSONHandler)

	require.False(t, ok)
	require.NotNil(t, logger)
}

// Expectation: A JSON handler should be returned.
func Test_NewLogger_WantJSON_Success(t *testing.T) {
	t.Parallel()

	ls := Options{
		Logout:   &testutil.SafeBuffer{},
		WantJSON: true,
	}
	_ = ls.LogLevel.Set("info")

	logger := NewLogger(ls)
	_, ok := logger.Handler().(*slog.JSONHandler)

	require.True(t, ok)
	require.NotNil(t, logger)
}

// Expectation: All known log levels should return a log handler.
func Test_NewLogger_AllLevels_Success(t *testing.T) {
	t.Parallel()

	levels := []string{"debug", "info", "warn", "error"}

	for _, level := range levels {
		ls := Options{
			Logout: &testutil.SafeBuffer{},
		}
		err := ls.LogLevel.Set(level)

		require.NoError(t, err)

		handler := NewLogger(ls)
		require.NotNil(t, handler.Logger)
	}
}

// Expectation: With should return a new Logger with added attributes while preserving Options.
func Test_Logger_With_Success(t *testing.T) {
	t.Parallel()

	buf := &testutil.SafeBuffer{}
	ls := Options{
		Logout:   buf,
		WantJSON: true,
	}
	_ = ls.LogLevel.Set("info")

	logger := NewLogger(ls)
	childLogger := logger.With("key", "value", "count", 42)

	require.NotNil(t, childLogger)
	require.NotSame(t, logger, childLogger)
	require.Equal(t, logger.Options, childLogger.Options)

	childLogger.Info("test message")
	output := buf.String()
	require.Contains(t, output, "key")
	require.Contains(t, output, "value")
	require.Contains(t, output, "count")
	require.Contains(t, output, "42")
}

// Expectation: With should preserve Options but return a distinct Logger with nil seqHandler.
func Test_Logger_With_PreservesOptions(t *testing.T) {
	t.Parallel()

	buf := &testutil.SafeBuffer{}
	ls := Options{
		Logout:   buf,
		WantJSON: false,
	}

	_ = ls.LogLevel.Set("debug")
	logger := NewLogger(ls)
	child := logger.With("component", "test")

	require.Equal(t, logger.Options, child.Options)
	require.Nil(t, child.seqHandler)
}

// Expectation: NewLogger with SeqURL should produce a fanoutHandler.
func Test_NewLogger_WithSeqURL_ReturnsFanoutHandler(t *testing.T) {
	t.Parallel()

	ls := Options{
		Logout: &testutil.SafeBuffer{},
		SeqURL: "http://localhost:5341",
		SeqKey: "test-api-key",
	}

	_ = ls.LogLevel.Set("info")
	logger := NewLogger(ls)
	require.NotNil(t, logger)
	require.NotNil(t, logger.seqHandler)

	_, ok := logger.Handler().(*fanoutHandler)
	require.True(t, ok)
	logger.Close()
}

// Expectation: NewLogger with SeqURL but no SeqKey should still produce a fanoutHandler.
func Test_NewLogger_WithSeqURL_NoKey_ReturnsFanoutHandler(t *testing.T) {
	t.Parallel()

	ls := Options{
		Logout: &testutil.SafeBuffer{},
		SeqURL: "http://localhost:5341",
	}

	_ = ls.LogLevel.Set("info")
	logger := NewLogger(ls)
	require.NotNil(t, logger)
	require.NotNil(t, logger.seqHandler)

	_, ok := logger.Handler().(*fanoutHandler)
	require.True(t, ok)
	logger.Close()
}

// Expectation: Close should not panic when no Seq handler is configured.
func Test_Logger_Close_NoSeq_Success(t *testing.T) {
	t.Parallel()

	ls := Options{
		Logout:   &testutil.SafeBuffer{},
		WantJSON: false,
	}

	_ = ls.LogLevel.Set("info")
	logger := NewLogger(ls)

	require.NotPanics(t, func() {
		logger.Close()
	})
}
