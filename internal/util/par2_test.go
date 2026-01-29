package util

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: ParsePar2To should successfully parse a valid PAR2 file.
func Test_ParsePar2To_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	validPar2 := loadTestPar2(t)
	require.NoError(t, afero.WriteFile(fs, "/test.par2", validPar2, 0o644))

	var archive *par2.Archive
	var logMessages []string

	ParsePar2To(&archive, fs, "/test.par2", func(msg string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf(msg, args...))
	})

	require.NotNil(t, archive)
	require.Empty(t, logMessages)
	require.NotNil(t, archive.Sets)
	require.Len(t, archive.Sets, 1)
}

// Expectation: ParsePar2To should set target to nil on parse error.
func Test_ParsePar2To_ParseError_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/invalid.par2", []byte("not a par2 file"), 0o644))

	var archive *par2.Archive
	var logMessages []string

	ParsePar2To(&archive, fs, "/invalid.par2", func(msg string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf("%s %v", msg, args))
	})

	require.Nil(t, archive)
	require.Len(t, logMessages, 1)
	require.Contains(t, logMessages[0], "Failed to parse created PAR2")
}

// Expectation: ParsePar2To should set target to nil when file doesn't exist.
func Test_ParsePar2To_FileNotFound_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var archive *par2.Archive
	var logMessages []string

	ParsePar2To(&archive, fs, "/nonexistent.par2", func(msg string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf("%s %v", msg, args))
	})

	require.Nil(t, archive)
	require.Len(t, logMessages, 1)
	require.Contains(t, logMessages[0], "Failed to parse created PAR2")
}

// Expectation: ParsePar2To should handle panic and set target to nil.
func Test_ParsePar2To_Panic_Success(t *testing.T) {
	t.Parallel()

	panicFs := &panicingFs{}

	var archive *par2.Archive
	var logMessages []string

	ParsePar2To(&archive, panicFs, "/test.par2", func(msg string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf("%s %v", msg, args))
	})

	require.Nil(t, archive)
	require.Len(t, logMessages, 1)
	require.Contains(t, logMessages[0], "Panic parsing PAR2")
	require.Contains(t, logMessages[0], "test panic")
}

// Expectation: ParsePar2To should log panic with stack trace.
func Test_ParsePar2To_PanicWithStackTrace_Success(t *testing.T) {
	t.Parallel()

	panicFs := &panicingFs{}

	var archive *par2.Archive
	var logMessages []string
	var logArgs [][]any

	ParsePar2To(&archive, panicFs, "/test.par2", func(msg string, args ...any) {
		logMessages = append(logMessages, msg)
		logArgs = append(logArgs, args)
	})

	require.Nil(t, archive)
	require.Len(t, logMessages, 1)
	require.Contains(t, logMessages[0], "Panic parsing PAR2")

	require.NotEmpty(t, logArgs)
	found := false
	for _, arg := range logArgs[0] {
		if str, ok := arg.(string); ok && strings.Contains(str, "goroutine") {
			found = true

			break
		}
	}
	require.True(t, found, "Stack trace should be included in log args")
}

// Expectation: ParsePar2To should handle concurrent calls safely.
func Test_ParsePar2To_Concurrent_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	validPar2 := loadTestPar2(t)
	require.NoError(t, afero.WriteFile(fs, "/test.par2", validPar2, 0o644))

	var archive1, archive2, archive3 *par2.Archive

	done := make(chan bool, 3)

	go func() {
		ParsePar2To(&archive1, fs, "/test.par2", func(msg string, args ...any) {})
		done <- true
	}()

	go func() {
		ParsePar2To(&archive2, fs, "/test.par2", func(msg string, args ...any) {})
		done <- true
	}()

	go func() {
		ParsePar2To(&archive3, fs, "/test.par2", func(msg string, args ...any) {})
		done <- true
	}()

	<-done
	<-done
	<-done

	require.NotNil(t, archive1)
	require.NotNil(t, archive2)
	require.NotNil(t, archive3)
}

// Expectation: ParsePar2To should overwrite existing archive pointer.
func Test_ParsePar2To_OverwritesExisting_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	validPar2 := loadTestPar2(t)
	require.NoError(t, afero.WriteFile(fs, "/test.par2", validPar2, 0o644))

	oldArchive := &par2.Archive{}
	archive := oldArchive

	ParsePar2To(&archive, fs, "/test.par2", func(msg string, args ...any) {})

	require.NotNil(t, archive)
	require.NotEqual(t, oldArchive, archive, "Should have replaced the old archive")
}

// Expectation: ParsePar2To should set to nil when parsing fails on previously valid archive.
func Test_ParsePar2To_ReplacesWithNilOnError_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/invalid.par2", []byte("invalid"), 0o644))

	archive := &par2.Archive{}

	ParsePar2To(&archive, fs, "/invalid.par2", func(msg string, args ...any) {})

	require.Nil(t, archive, "Should have replaced archive with nil on error")
}

// Expectation: ParsePar2To should log error details correctly.
func Test_ParsePar2To_LogsErrorDetails_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/invalid.par2", []byte("invalid"), 0o644))

	var archive *par2.Archive
	var logMessage string
	var logArgs []any

	ParsePar2To(&archive, fs, "/invalid.par2", func(msg string, args ...any) {
		logMessage = msg
		logArgs = args
	})

	require.Equal(t, "Failed to parse created PAR2 for par2cron manifest", logMessage)
	require.Len(t, logArgs, 2)
	require.Equal(t, "error", logArgs[0])

	err, ok := logArgs[1].(error)
	require.True(t, ok)
	require.Error(t, err)
}

// Expectation: ParsePar2To should complete synchronously.
func Test_ParsePar2To_Synchronous_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	validPar2 := loadTestPar2(t)
	require.NoError(t, afero.WriteFile(fs, "/test.par2", validPar2, 0o644))

	var archive *par2.Archive
	var completed bool

	ParsePar2To(&archive, fs, "/test.par2", func(msg string, args ...any) {})
	completed = true

	require.True(t, completed, "ParsePar2To should block until completion")
	require.NotNil(t, archive)
}

// Expectation: ParsePar2To should handle empty PAR2 file.
func Test_ParsePar2To_EmptyFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/empty.par2", []byte{}, 0o644))

	var archive *par2.Archive

	ParsePar2To(&archive, fs, "/empty.par2", func(msg string, args ...any) {})

	require.NotNil(t, archive)
	require.Empty(t, archive.Sets)
}

// Expectation: ParsePar2To should parse real PAR2 file correctly.
func Test_ParsePar2To_RealFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()

	var archive *par2.Archive
	var logMessages []string

	ParsePar2To(&archive, fs, "testdata/simple_par2cmdline.par2", func(msg string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf(msg, args...))
	})

	require.NotNil(t, archive)
	require.Empty(t, logMessages)
	require.Len(t, archive.Sets, 1)
	require.Len(t, archive.Sets[0].RecoverySet, 1)
	require.Equal(t, "test.txt", archive.Sets[0].RecoverySet[0].Name)
}

// Expectation: ParsePar2To should handle panic with custom message.
func Test_ParsePar2To_PanicWithCustomMessage_Success(t *testing.T) {
	t.Parallel()

	customPanicFs := &customPanicFs{msg: "custom panic message"}

	var archive *par2.Archive
	var logMessages []string

	ParsePar2To(&archive, customPanicFs, "/test.par2", func(msg string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf("%s %v", msg, args))
	})

	require.Nil(t, archive)
	require.Len(t, logMessages, 1)
	require.Contains(t, logMessages[0], "Panic parsing PAR2")
	require.Contains(t, logMessages[0], "custom panic message")
}

// panicingFs is a filesystem that panics when Open is called.
type panicingFs struct {
	afero.Fs
}

func (p *panicingFs) Open(name string) (afero.File, error) {
	panic("test panic")
}

// customPanicFs is a filesystem that panics with a custom message.
type customPanicFs struct {
	afero.Fs

	msg string
}

func (p *customPanicFs) Open(name string) (afero.File, error) {
	panic(p.msg)
}

func loadTestPar2(t *testing.T) []byte {
	t.Helper()

	data, err := os.ReadFile("testdata/simple_par2cmdline.par2")
	require.NoError(t, err)

	return data
}
