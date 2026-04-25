package testutil

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
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

// Expectation: The mock par2 handler should call the provided function.
func Test_MockPar2Handler_ParseFile_WithFunc_Success(t *testing.T) {
	t.Parallel()

	called := false
	handler := &MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			called = true

			return &par2.File{}, nil
		},
	}

	result, err := handler.ParseFile(afero.NewMemMapFs(), "/data/test.par2", true)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, called)
}

// Expectation: The mock par2 handler should return the error from the function.
func Test_MockPar2Handler_ParseFile_WithFunc_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("parse error")
	handler := &MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return nil, expectedErr
		},
	}

	_, err := handler.ParseFile(afero.NewMemMapFs(), "/data/test.par2", true)

	require.ErrorIs(t, err, expectedErr)
}

// Expectation: The mock par2 handler should return an error when no function is provided.
func Test_MockPar2Handler_ParseFile_NoFunc_Error(t *testing.T) {
	t.Parallel()

	handler := &MockPar2Handler{}

	_, err := handler.ParseFile(afero.NewMemMapFs(), "/data/test.par2", true)

	require.Error(t, err)
	require.Contains(t, err.Error(), "not implemented")
}

// Expectation: The mock bundle handler should call the provided Open function.
func Test_MockBundleHandler_Open_WithFunc_Success(t *testing.T) {
	t.Parallel()

	called := false
	handler := &MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			called = true

			return &MockBundle{}, nil
		},
	}

	result, err := handler.Open(afero.NewMemMapFs(), "/data/test.par2")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, called)
}

// Expectation: The mock bundle handler should return the error from the Open function.
func Test_MockBundleHandler_Open_WithFunc_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("open error")
	handler := &MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return nil, expectedErr
		},
	}

	_, err := handler.Open(afero.NewMemMapFs(), "/data/test.par2")

	require.ErrorIs(t, err, expectedErr)
}

// Expectation: The mock bundle handler should return an error when no Open function is provided.
func Test_MockBundleHandler_Open_NoFunc_Error(t *testing.T) {
	t.Parallel()

	handler := &MockBundleHandler{}

	_, err := handler.Open(afero.NewMemMapFs(), "/data/test.par2")

	require.Error(t, err)
	require.Contains(t, err.Error(), "not implemented")
}

// Expectation: The mock bundle handler should call the provided Pack function.
func Test_MockBundleHandler_Pack_WithFunc_Success(t *testing.T) {
	t.Parallel()

	called := false
	handler := &MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			called = true

			return nil
		},
	}

	err := handler.Pack(afero.NewMemMapFs(), "/data/test.par2", [16]byte{}, bundle.ManifestInput{}, nil)

	require.NoError(t, err)
	require.True(t, called)
}

// Expectation: The mock bundle handler should return the error from the Pack function.
func Test_MockBundleHandler_Pack_WithFunc_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("pack error")
	handler := &MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			return expectedErr
		},
	}

	err := handler.Pack(afero.NewMemMapFs(), "/data/test.par2", [16]byte{}, bundle.ManifestInput{}, nil)

	require.ErrorIs(t, err, expectedErr)
}

// Expectation: The mock bundle handler should return an error when no Pack function is provided.
func Test_MockBundleHandler_Pack_NoFunc_Error(t *testing.T) {
	t.Parallel()

	handler := &MockBundleHandler{}

	err := handler.Pack(afero.NewMemMapFs(), "/data/test.par2", [16]byte{}, bundle.ManifestInput{}, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "not implemented")
}

// Expectation: The mock bundle should call the provided Close function.
func Test_MockBundle_Close_WithFunc_Success(t *testing.T) {
	t.Parallel()

	called := false
	b := &MockBundle{
		CloseFunc: func() error {
			called = true

			return nil
		},
	}

	require.NoError(t, b.Close())
	require.True(t, called)
}

// Expectation: The mock bundle should return nil when no Close function is provided.
func Test_MockBundle_Close_NoFunc_Success(t *testing.T) {
	t.Parallel()

	b := &MockBundle{}

	require.NoError(t, b.Close())
}

// Expectation: The mock bundle should call the provided Manifest function.
func Test_MockBundle_Manifest_WithFunc_Success(t *testing.T) {
	t.Parallel()

	expected := []byte(`{"name":"test"}`)
	b := &MockBundle{
		ManifestFunc: func() ([]byte, error) {
			return expected, nil
		},
	}

	data, err := b.Manifest()

	require.NoError(t, err)
	require.Equal(t, expected, data)
}

// Expectation: The mock bundle should return an error when no Manifest function is provided.
func Test_MockBundle_Manifest_NoFunc_Error(t *testing.T) {
	t.Parallel()

	b := &MockBundle{}

	_, err := b.Manifest()

	require.Error(t, err)
	require.Contains(t, err.Error(), "not implemented")
}

// Expectation: The mock bundle should call the provided Update function.
func Test_MockBundle_Update_WithFunc_Success(t *testing.T) {
	t.Parallel()

	var capturedData []byte
	b := &MockBundle{
		UpdateFunc: func(manifest []byte) error {
			capturedData = manifest

			return nil
		},
	}

	data := []byte(`{"name":"test"}`)
	require.NoError(t, b.Update(data))
	require.Equal(t, data, capturedData)
}

// Expectation: The mock bundle should return an error when no Update function is provided.
func Test_MockBundle_Update_NoFunc_Error(t *testing.T) {
	t.Parallel()

	b := &MockBundle{}

	err := b.Update([]byte("data"))

	require.Error(t, err)
	require.Contains(t, err.Error(), "not implemented")
}

// Expectation: The mock bundle should call the provided ValidateIndex function.
func Test_MockBundle_ValidateIndex_WithFunc_Success(t *testing.T) {
	t.Parallel()

	called := false
	b := &MockBundle{
		ValidateIndexFunc: func() error {
			called = true

			return nil
		},
	}

	require.NoError(t, b.ValidateIndex())
	require.True(t, called)
}

// Expectation: The mock bundle should return nil when no Validate function is provided.
func Test_MockBundle_Validate_NoFunc_Success(t *testing.T) {
	t.Parallel()

	b := &MockBundle{}

	require.NoError(t, b.Validate(true))
}

// Expectation: The mock bundle should return the error from the Validate function.
func Test_MockBundle_Validate_WithFunc_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("validation error")
	b := &MockBundle{
		ValidateFunc: func(bool) error {
			return expectedErr
		},
	}

	err := b.Validate(true)

	require.ErrorIs(t, err, expectedErr)
}

// Expectation: The mock bundle should return nil when no ValidateIndex function is provided.
func Test_MockBundle_ValidateIndex_NoFunc_Success(t *testing.T) {
	t.Parallel()

	b := &MockBundle{}

	require.NoError(t, b.ValidateIndex())
}

// Expectation: The mock bundle should return the error from the ValidateIndex function.
func Test_MockBundle_ValidateIndex_WithFunc_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("validation error")
	b := &MockBundle{
		ValidateIndexFunc: func() error {
			return expectedErr
		},
	}

	err := b.ValidateIndex()

	require.ErrorIs(t, err, expectedErr)
}

// Expectation: The mock bundle should return nil when no IsRebuilt value is provided.
func Test_MockBundle_IsRebuilt_NoValue_Success(t *testing.T) {
	t.Parallel()

	b := &MockBundle{}

	require.False(t, b.IsRebuilt())
}

// Expectation: The mock bundle should return the value from the IsRebuilt value.
func Test_MockBundle_IsRebuilt_WithValue_Error(t *testing.T) {
	t.Parallel()

	b := &MockBundle{
		IsRebuiltValue: new(true),
	}

	require.True(t, b.IsRebuilt())
}

// Expectation: The mock bundle should return nil when no Unpack function is provided.
func Test_MockBundle_Unpack_NoFunc_Success(t *testing.T) {
	t.Parallel()

	b := &MockBundle{}

	files, err := b.Unpack(afero.NewMemMapFs(), "/dest", true)
	require.ErrorContains(t, err, "not implemented")
	require.Nil(t, files)
}

// Expectation: The mock bundle should return the error from the Unpack function.
func Test_MockBundle_Unpack_WithFunc_Error(t *testing.T) {
	t.Parallel()

	b := &MockBundle{
		UnpackFunc: func(fsys afero.Fs, destDir string, strict bool) ([]string, error) {
			return []string{"a", "b"}, nil
		},
	}

	files, err := b.Unpack(afero.NewMemMapFs(), "/dest", true)
	require.NoError(t, err)
	require.Len(t, files, 2)
}
