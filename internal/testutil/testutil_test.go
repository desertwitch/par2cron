package testutil

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: The buffer should safely handle concurrent writes.
func Test_SafeBuffer_Write_Concurrent_Success(t *testing.T) {
	t.Parallel()

	sb := &SafeBuffer{}

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			_, _ = sb.Write([]byte("test"))
		})
	}
	wg.Wait()

	require.Equal(t, 400, sb.Len())
}

// Expectation: The buffer should return the correct string.
func Test_SafeBuffer_String_Success(t *testing.T) {
	t.Parallel()

	sb := &SafeBuffer{}

	_, _ = sb.Write([]byte("hello"))

	require.Equal(t, "hello", sb.String())
}

// Expectation: The buffer should return the correct bytes.
func Test_SafeBuffer_Bytes_Success(t *testing.T) {
	t.Parallel()

	sb := &SafeBuffer{}

	_, _ = sb.Write([]byte("hello"))

	require.Equal(t, "hello", string(sb.Bytes()))
}

// Expectation: The buffer should reset correctly.
func Test_SafeBuffer_Reset_Success(t *testing.T) {
	t.Parallel()

	sb := &SafeBuffer{}
	_, _ = sb.Write([]byte("hello"))

	sb.Reset()

	require.Equal(t, 0, sb.Len())
	require.Empty(t, sb.String())
}

// Expectation: The buffer should return the correct length.
func Test_SafeBuffer_Len_Success(t *testing.T) {
	t.Parallel()

	sb := &SafeBuffer{}

	_, _ = sb.Write([]byte("hello"))

	require.Equal(t, 5, sb.Len())
}

// Expectation: The mock runner should call the provided function.
func Test_MockRunner_Run_WithFunc_Success(t *testing.T) {
	t.Parallel()

	called := false
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			called = true

			return nil
		},
	}

	err := runner.Run(t.Context(), "test", nil, "/tmp", nil, nil)

	require.NoError(t, err)
	require.True(t, called)
}

// Expectation: The mock runner should return the error from the function.
func Test_MockRunner_Run_WithFunc_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("test error")
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
			return expectedErr
		},
	}

	err := runner.Run(t.Context(), "test", nil, "/tmp", nil, nil)

	require.ErrorIs(t, err, expectedErr)
}

// Expectation: The mock runner should return nil when no function is provided.
func Test_MockRunner_Run_NoFunc_Success(t *testing.T) {
	t.Parallel()

	runner := &MockRunner{}

	err := runner.Run(t.Context(), "test", nil, "/tmp", nil, nil)

	require.NoError(t, err)
}

// Expectation: The failing fs should fail to open files matching the specified pattern.
func Test_FailingOpenFs_Open_MatchingPattern_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(baseFs, "/data/test.txt", []byte("content"), 0o644))

	fs := &FailingOpenFs{
		Fs:          baseFs,
		FailPattern: ".txt",
	}

	_, err := fs.Open("/data/test.txt")

	require.Error(t, err)
	require.Contains(t, err.Error(), "simulated open failure")
}

// Expectation: The failing fs should successfully open files not matching the specified pattern.
func Test_FailingOpenFs_Open_NonMatchingPattern_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(baseFs, "/data/test.log", []byte("content"), 0o644))

	fs := &FailingOpenFs{
		Fs:          baseFs,
		FailPattern: ".txt",
	}

	f, err := fs.Open("/data/test.log")

	require.NoError(t, err)
	require.NotNil(t, f)
	require.NoError(t, f.Close())
}

// Expectation: The failing fs should match patterns anywhere in the path.
func Test_FailingOpenFs_Open_PatternInPath_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(baseFs, "/data/secret/file.log", []byte("content"), 0o644))

	fs := &FailingOpenFs{
		Fs:          baseFs,
		FailPattern: "secret",
	}

	_, err := fs.Open("/data/secret/file.log")

	require.Error(t, err)
	require.Contains(t, err.Error(), "simulated open failure")
}

// Expectation: The failing fs should fail to stat files matching the specified pattern.
func Test_FailingStatFs_Stat_MatchingPattern_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(baseFs, "/data/test.txt", []byte("content"), 0o644))

	fs := &FailingStatFs{
		Fs:          baseFs,
		FailPattern: ".txt",
	}

	_, err := fs.Stat("/data/test.txt")

	require.Error(t, err)
	require.Contains(t, err.Error(), "stat failed")
}

// Expectation: The failing fs should successfully stat files not matching the specified pattern.
func Test_FailingStatFs_Stat_NonMatchingPattern_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(baseFs, "/data/test.log", []byte("content"), 0o644))

	fs := &FailingStatFs{
		Fs:          baseFs,
		FailPattern: ".txt",
	}

	info, err := fs.Stat("/data/test.log")

	require.NoError(t, err)
	require.NotNil(t, info)
	require.Equal(t, "test.log", info.Name())
}

// Expectation: The failing fs should match patterns anywhere in the path.
func Test_FailingStatFs_Stat_PatternInPath_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(baseFs, "/data/secret/file.log", []byte("content"), 0o644))

	fs := &FailingStatFs{
		Fs:          baseFs,
		FailPattern: "secret",
	}

	_, err := fs.Stat("/data/secret/file.log")

	require.Error(t, err)
	require.Contains(t, err.Error(), "stat failed")
}

// Expectation: The failing fs should fail for directories matching the pattern.
func Test_FailingStatFs_Stat_DirectoryMatchingPattern_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data/test.d", 0o755))

	fs := &FailingStatFs{
		Fs:          baseFs,
		FailPattern: ".d",
	}

	_, err := fs.Stat("/data/test.d")

	require.Error(t, err)
	require.Contains(t, err.Error(), "stat failed")
}

// Expectation: The failing fs should successfully stat directories not matching the pattern.
func Test_FailingStatFs_Stat_DirectoryNonMatchingPattern_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data/testdir", 0o755))

	fs := &FailingStatFs{
		Fs:          baseFs,
		FailPattern: ".d",
	}

	info, err := fs.Stat("/data/testdir")

	require.NoError(t, err)
	require.NotNil(t, info)
	require.True(t, info.IsDir())
}

// Expectation: The failing fs should return error even if underlying fs would fail.
func Test_FailingStatFs_Stat_UnderlyingFailure_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()

	fs := &FailingStatFs{
		Fs:          baseFs,
		FailPattern: "nonexistent",
	}

	_, err := fs.Stat("/data/nonexistent.txt")

	require.Error(t, err)
	require.Contains(t, err.Error(), "stat failed")
}

// Expectation: The failing fs should pass through errors from underlying fs when pattern doesn't match.
func Test_FailingStatFs_Stat_UnderlyingError_Passthrough_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()

	fs := &FailingStatFs{
		Fs:          baseFs,
		FailPattern: ".txt",
	}

	_, err := fs.Stat("/data/nonexistent.log")

	require.Error(t, err)
	require.NotContains(t, err.Error(), "stat failed")
}

// Expectation: The failing fs should fail to rename files matching the pattern in oldname.
func Test_FailingRenameFs_Rename_MatchingOldname_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(baseFs, "/data/test.txt", []byte("content"), 0o644))

	fs := &FailingRenameFs{
		Fs:          baseFs,
		FailPattern: "test.txt",
	}

	err := fs.Rename("/data/test.txt", "/data/renamed.txt")

	require.Error(t, err)
	require.Contains(t, err.Error(), "rename failed")
}

// Expectation: The failing fs should fail to rename files matching the pattern in newname.
func Test_FailingRenameFs_Rename_MatchingNewname_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(baseFs, "/data/source.txt", []byte("content"), 0o644))

	fs := &FailingRenameFs{
		Fs:          baseFs,
		FailPattern: "target.txt",
	}

	err := fs.Rename("/data/source.txt", "/data/target.txt")

	require.Error(t, err)
	require.Contains(t, err.Error(), "rename failed")
}

// Expectation: The failing fs should successfully rename files not matching the pattern.
func Test_FailingRenameFs_Rename_NonMatchingPattern_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(baseFs, "/data/source.txt", []byte("content"), 0o644))

	fs := &FailingRenameFs{
		Fs:          baseFs,
		FailPattern: "nomatch",
	}

	err := fs.Rename("/data/source.txt", "/data/target.txt")

	require.NoError(t, err)

	exists, err := afero.Exists(fs, "/data/target.txt")
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = afero.Exists(fs, "/data/source.txt")
	require.NoError(t, err)
	require.False(t, exists)
}

// Expectation: The failing fs should match patterns anywhere in the path.
func Test_FailingRenameFs_Rename_PatternInPath_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data/secret", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/data/secret/file.txt", []byte("content"), 0o644))

	fs := &FailingRenameFs{
		Fs:          baseFs,
		FailPattern: "secret",
	}

	err := fs.Rename("/data/secret/file.txt", "/data/other/file.txt")

	require.Error(t, err)
	require.Contains(t, err.Error(), "rename failed")
}

// Expectation: The failing fs should fail to remove files with the specified suffix.
func Test_FailingRemoveFs_Remove_MatchingSuffix_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(baseFs, "/test.txt", []byte("content"), 0o644))

	fs := &FailingRemoveFs{
		Fs:         baseFs,
		FailSuffix: ".txt",
	}

	err := fs.Remove("/test.txt")

	require.Error(t, err)
	require.Contains(t, err.Error(), "permission denied")
}

// Expectation: The failing fs should successfully remove files without the specified suffix.
func Test_FailingRemoveFs_Remove_NonMatchingSuffix_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(baseFs, "/test.log", []byte("content"), 0o644))

	fs := &FailingRemoveFs{
		Fs:         baseFs,
		FailSuffix: ".txt",
	}

	err := fs.Remove("/test.log")

	require.NoError(t, err)
}

// Expectation: The failing fs should fail to create files with the specified suffix.
func Test_FailingWriteFs_Create_MatchingSuffix_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()

	fs := &FailingWriteFs{
		Fs:         baseFs,
		FailSuffix: ".txt",
	}

	_, err := fs.Create("/test.txt")

	require.Error(t, err)
	require.Contains(t, err.Error(), "permission denied")
}

// Expectation: The failing fs should successfully create files without the specified suffix.
func Test_FailingWriteFs_Create_NonMatchingSuffix_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()

	fs := &FailingWriteFs{
		Fs:         baseFs,
		FailSuffix: ".txt",
	}

	f, err := fs.Create("/test.log")

	require.NoError(t, err)
	require.NotNil(t, f)
	require.NoError(t, f.Close())
}

// Expectation: The failing fs should fail to open files for writing with the specified suffix.
func Test_FailingWriteFs_OpenFile_WriteMatchingSuffix_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()

	fs := &FailingWriteFs{
		Fs:         baseFs,
		FailSuffix: ".txt",
	}

	_, err := fs.OpenFile("/test.txt", os.O_CREATE|os.O_WRONLY, 0o644)

	require.Error(t, err)
	require.Contains(t, err.Error(), "permission denied")
}

// Expectation: The failing fs should successfully open files for reading with the specified suffix.
func Test_FailingWriteFs_OpenFile_ReadMatchingSuffix_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(baseFs, "/test.txt", []byte("content"), 0o644))

	fs := &FailingWriteFs{
		Fs:         baseFs,
		FailSuffix: ".txt",
	}

	f, err := fs.OpenFile("/test.txt", os.O_RDONLY, 0o644)

	require.NoError(t, err)
	require.NotNil(t, f)
	require.NoError(t, f.Close())
}

// Expectation: The function should create an exit error with the specified code.
func Test_CreateExitError_NonZero_Error(t *testing.T) {
	t.Parallel()

	err := CreateExitError(t, t.Context(), 1)

	require.Error(t, err)
}

// Expectation: The function should return nil for exit code 0.
func Test_CreateExitError_Zero_Success(t *testing.T) {
	t.Parallel()

	err := CreateExitError(t, t.Context(), 0)

	require.NoError(t, err)
}

// Expectation: The function should panic for negative exit codes.
func Test_CreateExitError_NegativeCode_Panic(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		require.Error(t, CreateExitError(t, t.Context(), -1))
	})
}
