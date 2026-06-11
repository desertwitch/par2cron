package testutil

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
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

// Expectation: The mock par2 handler should call the provided Parse function.
func Test_MockPar2Handler_Parse_WithFunc_Success(t *testing.T) {
	t.Parallel()

	called := false
	handler := &MockPar2Handler{
		ParseFunc: func(r io.ReadSeeker, checkMD5 bool) ([]par2.Set, error) {
			called = true

			return []par2.Set{{}}, nil
		},
	}

	result, err := handler.Parse(t.Context(), bytes.NewReader([]byte("data")), true)

	require.NoError(t, err)
	require.Len(t, result, 1)
	require.True(t, called)
}

// Expectation: The mock par2 handler should return the error from the Parse function.
func Test_MockPar2Handler_Parse_WithFunc_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("parse error")
	handler := &MockPar2Handler{
		ParseFunc: func(r io.ReadSeeker, checkMD5 bool) ([]par2.Set, error) {
			return nil, expectedErr
		},
	}

	_, err := handler.Parse(t.Context(), bytes.NewReader([]byte("data")), true)

	require.ErrorIs(t, err, expectedErr)
}

// Expectation: The mock par2 handler should return an error when no Parse function is provided.
func Test_MockPar2Handler_Parse_NoFunc_Error(t *testing.T) {
	t.Parallel()

	handler := &MockPar2Handler{}

	_, err := handler.Parse(t.Context(), bytes.NewReader([]byte("data")), true)

	require.Error(t, err)
	require.Contains(t, err.Error(), "not implemented")
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

	result, err := handler.ParseFile(t.Context(), afero.NewMemMapFs(), "/data/test.par2", true)

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

	_, err := handler.ParseFile(t.Context(), afero.NewMemMapFs(), "/data/test.par2", true)

	require.ErrorIs(t, err, expectedErr)
}

// Expectation: The mock par2 handler should return an error when no function is provided.
func Test_MockPar2Handler_ParseFile_NoFunc_Error(t *testing.T) {
	t.Parallel()

	handler := &MockPar2Handler{}

	_, err := handler.ParseFile(t.Context(), afero.NewMemMapFs(), "/data/test.par2", true)

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

	result, err := handler.Open(t.Context(), afero.NewMemMapFs(), "/data/test.par2")

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

	_, err := handler.Open(t.Context(), afero.NewMemMapFs(), "/data/test.par2")

	require.ErrorIs(t, err, expectedErr)
}

// Expectation: The mock bundle handler should return an error when no Open function is provided.
func Test_MockBundleHandler_Open_NoFunc_Error(t *testing.T) {
	t.Parallel()

	handler := &MockBundleHandler{}

	_, err := handler.Open(t.Context(), afero.NewMemMapFs(), "/data/test.par2")

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

	err := handler.Pack(t.Context(), afero.NewMemMapFs(), "/data/test.par2", [16]byte{}, bundle.ManifestInput{}, nil)

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

	err := handler.Pack(t.Context(), afero.NewMemMapFs(), "/data/test.par2", [16]byte{}, bundle.ManifestInput{}, nil)

	require.ErrorIs(t, err, expectedErr)
}

// Expectation: The mock bundle handler should return an error when no Pack function is provided.
func Test_MockBundleHandler_Pack_NoFunc_Error(t *testing.T) {
	t.Parallel()

	handler := &MockBundleHandler{}

	err := handler.Pack(t.Context(), afero.NewMemMapFs(), "/data/test.par2", [16]byte{}, bundle.ManifestInput{}, nil)

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

	data, err := b.Manifest(t.Context())

	require.NoError(t, err)
	require.Equal(t, expected, data)
}

// Expectation: The mock bundle should return an error when no Manifest function is provided.
func Test_MockBundle_Manifest_NoFunc_Error(t *testing.T) {
	t.Parallel()

	b := &MockBundle{}

	_, err := b.Manifest(t.Context())

	require.Error(t, err)
	require.Contains(t, err.Error(), "not implemented")
}

// Expectation: The mock bundle should return "" when no ManifestName value is provided.
func Test_MockBundle_ManifestName_NoValue_Success(t *testing.T) {
	t.Parallel()

	b := &MockBundle{}

	require.Empty(t, b.ManifestName())
}

// Expectation: The mock bundle should return the value from the ManifestName value.
func Test_MockBundle_ManifestName_WithValue_Error(t *testing.T) {
	t.Parallel()

	b := &MockBundle{
		ManifestNameValue: new("test.json"),
	}

	require.Equal(t, "test.json", b.ManifestName())
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

	require.NoError(t, b.Validate(t.Context(), true))
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

	err := b.Validate(t.Context(), true)

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

	files, err := b.Unpack(t.Context(), afero.NewMemMapFs(), "/dest", true)
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

	files, err := b.Unpack(t.Context(), afero.NewMemMapFs(), "/dest", true)
	require.NoError(t, err)
	require.Len(t, files, 2)
}

// Expectation: The mock bundle should return the provided MarshalJSON value.
func Test_MockBundle_MarshalJSON_WithValue_Success(t *testing.T) {
	t.Parallel()

	expected := []byte(`{"mock":true}`)
	b := &MockBundle{
		MarshalJSONValue: expected,
	}

	data, err := b.MarshalJSON()

	require.NoError(t, err)
	require.Equal(t, expected, data)
}

// Expectation: The mock bundle should return an error when no MarshalJSON value is provided.
func Test_MockBundle_MarshalJSON_NoValue_Error(t *testing.T) {
	t.Parallel()

	b := &MockBundle{}

	_, err := b.MarshalJSON()

	require.Error(t, err)
	require.Contains(t, err.Error(), "not implemented")
}

// Expectation: The mock bundle should call the provided Entries function.
func Test_MockBundle_Entries_WithFunc_Success(t *testing.T) {
	t.Parallel()

	expected := []bundle.IndexEntry{
		{Name: "file1.txt", DataLength: 100},
		{Name: "file2.txt", DataLength: 200},
	}
	b := &MockBundle{
		EntriesFunc: func() []bundle.IndexEntry {
			return expected
		},
	}

	result := b.Entries()

	require.Equal(t, expected, result)
}

// Expectation: The mock bundle should return an empty slice when no Entries function is provided.
func Test_MockBundle_Entries_NoFunc_Success(t *testing.T) {
	t.Parallel()

	b := &MockBundle{}

	result := b.Entries()

	require.Empty(t, result)
}

// Expectation: The mock bundle should call the provided ExtractEntry function.
func Test_MockBundle_ExtractEntry_WithFunc_Success(t *testing.T) {
	t.Parallel()

	called := false
	b := &MockBundle{
		ExtractEntryFunc: func(e bundle.IndexEntry, w io.Writer) error {
			called = true
			_, err := w.Write([]byte("extracted"))

			return err
		},
	}

	var buf bytes.Buffer
	err := b.ExtractEntry(t.Context(), bundle.IndexEntry{Name: "test.txt"}, &buf)

	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, "extracted", buf.String())
}

// Expectation: The mock bundle should return the error from the ExtractEntry function.
func Test_MockBundle_ExtractEntry_WithFunc_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("extract error")
	b := &MockBundle{
		ExtractEntryFunc: func(e bundle.IndexEntry, w io.Writer) error {
			return expectedErr
		},
	}

	err := b.ExtractEntry(t.Context(), bundle.IndexEntry{Name: "test.txt"}, &bytes.Buffer{})

	require.ErrorIs(t, err, expectedErr)
}

// Expectation: The mock bundle should return an error when no ExtractEntry function is provided.
func Test_MockBundle_ExtractEntry_NoFunc_Error(t *testing.T) {
	t.Parallel()

	b := &MockBundle{}

	err := b.ExtractEntry(t.Context(), bundle.IndexEntry{Name: "test.txt"}, &bytes.Buffer{})

	require.Error(t, err)
	require.Contains(t, err.Error(), "not implemented")
}

// Expectation: The fake dir entry should return the correct name.
func Test_FakeDirEntry_Name_Success(t *testing.T) {
	t.Parallel()

	entry := FakeDirEntry{EntryName: "test.txt"}

	require.Equal(t, "test.txt", entry.Name())
}

// Expectation: The fake dir entry should return false for IsDir.
func Test_FakeDirEntry_IsDir_Success(t *testing.T) {
	t.Parallel()

	entry := FakeDirEntry{EntryName: "test.txt"}

	require.False(t, entry.IsDir())
}

// Expectation: The fake dir entry should return zero for Type.
func Test_FakeDirEntry_Type_Success(t *testing.T) {
	t.Parallel()

	entry := FakeDirEntry{EntryName: "test.txt"}

	require.Equal(t, fs.FileMode(0), entry.Type())
}

// Expectation: The fake dir entry should return nil for Info.
func Test_FakeDirEntry_Info_Success(t *testing.T) {
	t.Parallel()

	entry := FakeDirEntry{EntryName: "test.txt"}

	info, err := entry.Info()

	require.NoError(t, err)
	require.Nil(t, info)
}

// Expectation: The fake walker should return the correct name.
func Test_FakeWalker_Name_Success(t *testing.T) {
	t.Parallel()

	walker := &FakeWalker{}

	require.Equal(t, "fake", walker.Name())
}

// Expectation: The fake walker should visit all entries with correct paths.
func Test_FakeWalker_WalkDir_AllEntries_Success(t *testing.T) {
	t.Parallel()

	walker := &FakeWalker{
		Entries: []FakeDirEntry{
			{EntryName: "a.txt"},
			{EntryName: "b.txt"},
			{EntryName: "c.txt"},
		},
	}

	var visited []string
	err := walker.WalkDir("/root", func(path string, d fs.DirEntry, err error) error {
		visited = append(visited, path)

		return nil
	})

	require.NoError(t, err)
	require.Equal(t, []string{"/root/a.txt", "/root/b.txt", "/root/c.txt"}, visited)
}

// Expectation: The fake walker should pass the correct dir entry to the callback.
func Test_FakeWalker_WalkDir_CorrectDirEntry_Success(t *testing.T) {
	t.Parallel()

	walker := &FakeWalker{
		Entries: []FakeDirEntry{
			{EntryName: "test.txt"},
		},
	}

	err := walker.WalkDir("/root", func(path string, d fs.DirEntry, err error) error {
		require.Equal(t, "test.txt", d.Name())

		return nil
	})

	require.NoError(t, err)
}

// Expectation: The fake walker should handle an empty entry list.
func Test_FakeWalker_WalkDir_NoEntries_Success(t *testing.T) {
	t.Parallel()

	walker := &FakeWalker{}

	called := false
	err := walker.WalkDir("/root", func(path string, d fs.DirEntry, err error) error {
		called = true

		return nil
	})

	require.NoError(t, err)
	require.False(t, called)
}

// Expectation: The fake walker should stop and return the error when the callback returns an error.
func Test_FakeWalker_WalkDir_CallbackError_Error(t *testing.T) {
	t.Parallel()

	walker := &FakeWalker{
		Entries: []FakeDirEntry{
			{EntryName: "a.txt"},
			{EntryName: "b.txt"},
		},
	}

	expectedErr := errors.New("stop walking")
	var visited []string
	err := walker.WalkDir("/root", func(path string, d fs.DirEntry, err error) error {
		visited = append(visited, path)

		return expectedErr
	})

	require.ErrorIs(t, err, expectedErr)
	require.Len(t, visited, 1)
}

// Expectation: The fake walker should pass nil error to the callback.
func Test_FakeWalker_WalkDir_NilError_Success(t *testing.T) {
	t.Parallel()

	walker := &FakeWalker{
		Entries: []FakeDirEntry{
			{EntryName: "test.txt"},
		},
	}

	err := walker.WalkDir("/root", func(path string, d fs.DirEntry, err error) error {
		require.NoError(t, err)

		return nil
	})

	require.NoError(t, err)
}

// Expectation: The mock cache handler should call the provided NewCache function.
func Test_MockCacheHandler_NewCache_WithFunc_Success(t *testing.T) {
	t.Parallel()

	called := false
	expectedCache := &MockCache{}
	handler := &MockCacheHandler{
		NewCacheFunc: func(fsys afero.Fs, cacheDir string, cacheName string) schema.Cache {
			called = true

			return expectedCache
		},
	}

	result := handler.NewCache(afero.NewMemMapFs(), "/cache", "test")

	require.True(t, called)
	require.Equal(t, expectedCache, result)
}

// Expectation: The mock cache handler should return a default MockCache when no function is provided.
func Test_MockCacheHandler_NewCache_NoFunc_Success(t *testing.T) {
	t.Parallel()

	handler := &MockCacheHandler{}

	result := handler.NewCache(afero.NewMemMapFs(), "/cache", "test")

	require.NotNil(t, result)
	require.IsType(t, &MockCache{}, result)
}

// Expectation: The mock cache handler should pass the correct arguments to the function.
func Test_MockCacheHandler_NewCache_PassesArguments_Success(t *testing.T) {
	t.Parallel()

	var capturedDir, capturedName string
	handler := &MockCacheHandler{
		NewCacheFunc: func(fsys afero.Fs, cacheDir string, cacheName string) schema.Cache {
			capturedDir = cacheDir
			capturedName = cacheName

			return &MockCache{}
		},
	}

	handler.NewCache(afero.NewMemMapFs(), "/data/cache", "my-cache")

	require.Equal(t, "/data/cache", capturedDir)
	require.Equal(t, "my-cache", capturedName)
}

// Expectation: The mock cache should call the provided All function.
func Test_MockCache_All_WithFunc_Success(t *testing.T) {
	t.Parallel()

	expected := []*schema.JobMeta{
		{Par2Path: "/a.par2"},
		{Par2Path: "/b.par2"},
	}
	c := &MockCache{
		AllFunc: func() []*schema.JobMeta {
			return expected
		},
	}

	result := c.All()

	require.Equal(t, expected, result)
}

// Expectation: The mock cache should return nil when no All function is provided.
func Test_MockCache_All_NoFunc_Success(t *testing.T) {
	t.Parallel()

	c := &MockCache{}

	require.Nil(t, c.All())
}

// Expectation: The mock cache should call the provided Get function.
func Test_MockCache_Get_WithFunc_Success(t *testing.T) {
	t.Parallel()

	expected := &schema.JobMeta{Par2Path: "/a.par2"}
	c := &MockCache{
		GetFunc: func(key string) (*schema.JobMeta, bool) {
			return expected, true
		},
	}

	result, ok := c.Get("/a.par2")

	require.True(t, ok)
	require.Equal(t, expected, result)
}

// Expectation: The mock cache should return nil and false when no Get function is provided.
func Test_MockCache_Get_NoFunc_Success(t *testing.T) {
	t.Parallel()

	c := &MockCache{}

	result, ok := c.Get("/a.par2")

	require.False(t, ok)
	require.Nil(t, result)
}

// Expectation: The mock cache should pass the correct key to the Get function.
func Test_MockCache_Get_PassesKey_Success(t *testing.T) {
	t.Parallel()

	var capturedKey string
	c := &MockCache{
		GetFunc: func(key string) (*schema.JobMeta, bool) {
			capturedKey = key

			return nil, false
		},
	}

	c.Get("/data/test.par2")

	require.Equal(t, "/data/test.par2", capturedKey)
}

// Expectation: The mock cache should call the provided Len function.
func Test_MockCache_Len_WithFunc_Success(t *testing.T) {
	t.Parallel()

	c := &MockCache{
		LenFunc: func() int {
			return 42
		},
	}

	require.Equal(t, 42, c.Len())
}

// Expectation: The mock cache should return zero when no Len function is provided.
func Test_MockCache_Len_NoFunc_Success(t *testing.T) {
	t.Parallel()

	c := &MockCache{}

	require.Equal(t, 0, c.Len())
}

// Expectation: The mock cache should call the provided Load function.
func Test_MockCache_Load_WithFunc_Success(t *testing.T) {
	t.Parallel()

	called := false
	c := &MockCache{
		LoadFunc: func() error {
			called = true

			return nil
		},
	}

	require.NoError(t, c.Load())
	require.True(t, called)
}

// Expectation: The mock cache should return the error from the Load function.
func Test_MockCache_Load_WithFunc_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("load error")
	c := &MockCache{
		LoadFunc: func() error {
			return expectedErr
		},
	}

	err := c.Load()

	require.ErrorIs(t, err, expectedErr)
}

// Expectation: The mock cache should return nil when no Load function is provided.
func Test_MockCache_Load_NoFunc_Success(t *testing.T) {
	t.Parallel()

	c := &MockCache{}

	require.NoError(t, c.Load())
}

// Expectation: The mock cache should call the provided Save function.
func Test_MockCache_Save_WithFunc_Success(t *testing.T) {
	t.Parallel()

	called := false
	c := &MockCache{
		SaveFunc: func() error {
			called = true

			return nil
		},
	}

	require.NoError(t, c.Save())
	require.True(t, called)
}

// Expectation: The mock cache should return the error from the Save function.
func Test_MockCache_Save_WithFunc_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("save error")
	c := &MockCache{
		SaveFunc: func() error {
			return expectedErr
		},
	}

	err := c.Save()

	require.ErrorIs(t, err, expectedErr)
}

// Expectation: The mock cache should return nil when no Save function is provided.
func Test_MockCache_Save_NoFunc_Success(t *testing.T) {
	t.Parallel()

	c := &MockCache{}

	require.NoError(t, c.Save())
}

// Expectation: The mock cache should call the provided Set function.
func Test_MockCache_Set_WithFunc_Success(t *testing.T) {
	t.Parallel()

	var capturedKey string
	var capturedMeta *schema.JobMeta
	c := &MockCache{
		SetFunc: func(key string, meta *schema.JobMeta) {
			capturedKey = key
			capturedMeta = meta
		},
	}

	meta := &schema.JobMeta{Par2Path: "/a.par2"}
	c.Set("/a.par2", meta)

	require.Equal(t, "/a.par2", capturedKey)
	require.Equal(t, meta, capturedMeta)
}

// Expectation: The mock cache should not panic when no Set function is provided.
func Test_MockCache_Set_NoFunc_Success(t *testing.T) {
	t.Parallel()

	c := &MockCache{}

	require.NotPanics(t, func() {
		c.Set("/a.par2", &schema.JobMeta{Par2Path: "/a.par2"})
	})
}

// Expectation: The mock cache should call the provided ResetWalked function.
func Test_MockCache_ResetWalked_WithFunc_Success(t *testing.T) {
	t.Parallel()

	called := false
	c := &MockCache{
		ResetWalkedFunc: func() {
			called = true
		},
	}

	c.ResetWalked()

	require.True(t, called)
}

// Expectation: The mock cache should not panic when no ResetWalked function is provided.
func Test_MockCache_ResetWalked_NoFunc_Success(t *testing.T) {
	t.Parallel()

	c := &MockCache{}

	require.NotPanics(t, func() { c.ResetWalked() })
}

// Expectation: The mock cache should call the provided PruneUnwalked function.
func Test_MockCache_PruneUnwalked_WithFunc_Success(t *testing.T) {
	t.Parallel()

	c := &MockCache{
		PruneUnwalkedFunc: func() int {
			return 5
		},
	}

	require.Equal(t, 5, c.PruneUnwalked())
}

// Expectation: The mock cache should return zero when no PruneUnwalked function is provided.
func Test_MockCache_PruneUnwalked_NoFunc_Success(t *testing.T) {
	t.Parallel()

	c := &MockCache{}

	require.Equal(t, 0, c.PruneUnwalked())
}
