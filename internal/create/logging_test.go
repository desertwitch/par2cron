package create

import (
	"io"
	"testing"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: The debug message should be emitted to expectation.
func Test_Service_debugArgsModified_Add_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	prog.debugArgsModified("-r", "20", []string{}, []string{"-r20"}, false, "/data/_par2cron")

	require.Contains(t, logBuf.String(), "Added argument")
}

// Expectation: The debug message should be emitted to expectation.
func Test_Service_debugArgsModified_Replace_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	prog.debugArgsModified("-r", "20", "-r10", "-r20", true, "/data/_par2cron")

	require.Contains(t, logBuf.String(), "Replaced argument")
}
