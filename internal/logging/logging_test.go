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
