package util

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: LstatIfPossible should fall back to Stat when the filesystem does not implement Lstater.
func Test_LstatIfPossible_NoLstater_FallsBackToStat_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fsys, "/testfile.txt", []byte("hello"), 0o644))

	fi, err := LstatIfPossible(fsys, "/testfile.txt")

	require.NoError(t, err)
	require.Equal(t, "testfile.txt", fi.Name())
}

// Expectation: LstatIfPossible should return a stat-prefixed error when the filesystem does not implement Lstater and the file does not exist.
func Test_LstatIfPossible_NoLstater_FileNotFound_ReturnsStatError(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	_, err := LstatIfPossible(fsys, "/nonexistent.txt")

	require.Error(t, err)
	require.Contains(t, err.Error(), "stat:")
}

// Expectation: LstatIfPossible should use Lstat when the filesystem implements Lstater.
func Test_LstatIfPossible_WithLstater_UsesLstat_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewOsFs()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile.txt")
	require.NoError(t, afero.WriteFile(fsys, filePath, []byte("hello"), 0o644))

	fi, err := LstatIfPossible(fsys, filePath)

	require.NoError(t, err)
	require.Equal(t, "testfile.txt", fi.Name())
}

// Expectation: LstatIfPossible should return an lstat-prefixed error when the filesystem implements Lstater and the file does not exist.
func Test_LstatIfPossible_WithLstater_FileNotFound_ReturnsLstatError(t *testing.T) {
	t.Parallel()

	fsys := afero.NewOsFs()
	_, err := LstatIfPossible(fsys, "/nonexistent_path_that_should_not_exist.txt")

	require.Error(t, err)
	require.Contains(t, err.Error(), "lstat:")
}

// Expectation: LstatIfPossible should return symlink info (not target info) when the filesystem implements Lstater.
func Test_LstatIfPossible_WithLstater_Symlink_ReturnsSymlinkInfo(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target.txt")
	linkPath := filepath.Join(tmpDir, "link.txt")
	require.NoError(t, os.WriteFile(targetPath, []byte("hello"), 0o600))
	require.NoError(t, os.Symlink(targetPath, linkPath))
	fsys := afero.NewOsFs()

	fi, err := LstatIfPossible(fsys, linkPath)

	require.NoError(t, err)
	require.NotZero(t, fi.Mode()&fs.ModeSymlink, "expected symlink mode bit to be set")
}

// Expectation: The function should hash the file as requested.
func Test_HashFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/data/test.txt", []byte("hello"), 0o644))

	hash, err := HashFile(fs, "/data/test.txt")

	require.NoError(t, err)
	require.NotEmpty(t, hash)
	require.Len(t, hash, 64)
}

// Expectation: The hashes should be the same over multiple runs.
func Test_HashFile_ConsistentHashes_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/data/test.txt", []byte("content"), 0o644))

	hash1, err1 := HashFile(fs, "/data/test.txt")
	hash2, err2 := HashFile(fs, "/data/test.txt")

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.Equal(t, hash1, hash2)
}

// Expectation: An error should be returned if the file to be hashed is not found.
func Test_HashFile_NotFound_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	_, err := HashFile(fs, "/data/nonexistent.txt")

	require.Error(t, err)
}

// Expectation: The manifest should be written out as JSON without error.
func Test_WriteManifest_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.SHA256 = "abc123"

	err := WriteManifest(fs, &BundleHandler{}, "/data/test"+schema.Par2Extension+schema.ManifestExtension, mf, false)

	require.NoError(t, err)

	exists, _ := afero.Exists(fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension)
	require.True(t, exists)

	by, _ := afero.ReadFile(fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension)
	require.True(t, json.Valid(by))
}

// Expectation: The manifest version should be updated to current schema version on write.
func Test_WriteManifest_UpdatesManifestVersion_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/data", 0o755))

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.ManifestVersion = "0" // simulate old version

	err := WriteManifest(fsys, &BundleHandler{}, "/data/test"+schema.ManifestExtension, mf, false)
	require.NoError(t, err)

	by, err := afero.ReadFile(fsys, "/data/test"+schema.ManifestExtension)
	require.NoError(t, err)

	var written schema.Manifest
	require.NoError(t, json.Unmarshal(by, &written))
	require.Equal(t, schema.ManifestVersion, written.ManifestVersion)
}

// Expectation: The program version should be updated on write.
func Test_WriteManifest_UpdatesProgramVersion_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/data", 0o755))

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.ProgramVersion = "0.0.0" // simulate old version

	err := WriteManifest(fsys, &BundleHandler{}, "/data/test"+schema.ManifestExtension, mf, false)
	require.NoError(t, err)

	by, err := afero.ReadFile(fsys, "/data/test"+schema.ManifestExtension)
	require.NoError(t, err)

	var written schema.Manifest
	require.NoError(t, json.Unmarshal(by, &written))
	require.Equal(t, schema.ProgramVersion, written.ProgramVersion)
}

// Expectation: A write failure should fail the function and return an error.
func Test_WriteManifest_WriteFails_Error(t *testing.T) {
	t.Parallel()

	fs := &testutil.FailingWriteFs{Fs: afero.NewMemMapFs(), FailSuffix: schema.ManifestExtension}

	require.NoError(t, fs.MkdirAll("/data", 0o755))

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.SHA256 = "abc123"

	err := WriteManifest(fs, &BundleHandler{}, "/data/test"+schema.Par2Extension+schema.ManifestExtension, mf, false)

	require.ErrorContains(t, err, "failed to write")

	exists, _ := afero.Exists(fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension)
	require.False(t, exists)
}

// Expectation: The manifest should be written into the bundle via Open and Update.
func Test_WriteManifest_Bundle_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	mf := schema.NewManifest("test" + schema.BundleExtension + schema.Par2Extension)
	mf.SHA256 = "abc123"

	var updateCalled bool
	var capturedData []byte
	mockBundle := &testutil.MockBundle{
		UpdateFunc: func(data []byte) error {
			updateCalled = true
			capturedData = data

			return nil
		},
		CloseFunc: func() error {
			return nil
		},
	}

	bundler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			require.Equal(t, "/data/test"+schema.BundleExtension+schema.Par2Extension, bundlePath)

			return mockBundle, nil
		},
	}

	err := WriteManifest(fs, bundler, "/data/test"+schema.BundleExtension+schema.Par2Extension, mf, true)

	require.NoError(t, err)
	require.True(t, updateCalled)

	var written schema.Manifest
	require.NoError(t, json.Unmarshal(capturedData, &written))
	require.Equal(t, "abc123", written.SHA256)
	require.Equal(t, schema.ProgramVersion, written.ProgramVersion)
	require.Equal(t, schema.ManifestVersion, written.ManifestVersion)
}

// Expectation: The manifest version should be updated to current schema version when writing to a bundle.
func Test_WriteManifest_Bundle_UpdatesVersions_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	mf := schema.NewManifest("test" + schema.BundleExtension + schema.Par2Extension)
	mf.ProgramVersion = "0.0.0"
	mf.ManifestVersion = "0"

	var capturedData []byte
	mockBundle := &testutil.MockBundle{
		UpdateFunc: func(data []byte) error {
			capturedData = data

			return nil
		},
		CloseFunc: func() error {
			return nil
		},
	}

	bundler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	err := WriteManifest(fs, bundler, "/data/test"+schema.BundleExtension+schema.Par2Extension, mf, true)
	require.NoError(t, err)

	var written schema.Manifest
	require.NoError(t, json.Unmarshal(capturedData, &written))
	require.Equal(t, schema.ProgramVersion, written.ProgramVersion)
	require.Equal(t, schema.ManifestVersion, written.ManifestVersion)
}

// Expectation: An error should be returned when the bundle cannot be opened.
func Test_WriteManifest_Bundle_OpenFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	mf := schema.NewManifest("test" + schema.BundleExtension + schema.Par2Extension)

	bundler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return nil, errors.New("corrupt bundle")
		},
	}

	err := WriteManifest(fs, bundler, "/data/test"+schema.BundleExtension+schema.Par2Extension, mf, true)

	require.ErrorContains(t, err, "failed to open bundle")
}

// Expectation: An error should be returned when the bundle update fails.
func Test_WriteManifest_Bundle_UpdateFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	mf := schema.NewManifest("test" + schema.BundleExtension + schema.Par2Extension)

	mockBundle := &testutil.MockBundle{
		UpdateFunc: func(data []byte) error {
			return errors.New("write failed")
		},
		CloseFunc: func() error {
			return nil
		},
	}

	bundler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	err := WriteManifest(fs, bundler, "/data/test"+schema.BundleExtension+schema.Par2Extension, mf, true)

	require.ErrorContains(t, err, "failed to update bundle")
}

// Expectation: Close should be called even when Update succeeds.
func Test_WriteManifest_Bundle_CloseCalled_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	mf := schema.NewManifest("test" + schema.BundleExtension + schema.Par2Extension)

	var closeCalled bool
	mockBundle := &testutil.MockBundle{
		UpdateFunc: func(data []byte) error {
			return nil
		},
		CloseFunc: func() error {
			closeCalled = true

			return nil
		},
	}

	bundler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	require.NoError(t, WriteManifest(fs, bundler, "/data/test"+schema.BundleExtension+schema.Par2Extension, mf, true))
	require.True(t, closeCalled)
}

// Expectation: Close should be called even when Update fails.
func Test_WriteManifest_Bundle_CloseCalledOnUpdateFailure_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	mf := schema.NewManifest("test" + schema.BundleExtension + schema.Par2Extension)

	var closeCalled bool
	mockBundle := &testutil.MockBundle{
		UpdateFunc: func(data []byte) error {
			return errors.New("update failed")
		},
		CloseFunc: func() error {
			closeCalled = true

			return nil
		},
	}

	bundler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	require.Error(t, WriteManifest(fs, bundler, "/data/test"+schema.BundleExtension+schema.Par2Extension, mf, true))
	require.True(t, closeCalled)
}

// Expectation: No standalone file should be written to disk when writing to a bundle.
func Test_WriteManifest_Bundle_NoStandaloneFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	mf := schema.NewManifest("test" + schema.BundleExtension + schema.Par2Extension)

	mockBundle := &testutil.MockBundle{
		UpdateFunc: func(data []byte) error { return nil },
		CloseFunc:  func() error { return nil },
	}

	bundler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	require.NoError(t, WriteManifest(fs, bundler, "/data/test"+schema.BundleExtension+schema.Par2Extension, mf, true))

	// No file should be written to disk - the manifest lives inside the bundle.
	entries, err := afero.ReadDir(fs, "/data")
	require.NoError(t, err)
	require.Empty(t, entries)
}

// Expectation: The walker should visit all files and directories.
func Test_AferoWalker_WalkDir_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root/subdir", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/root/file1.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fsys, "/root/subdir/file2.txt", []byte("content"), 0o644))

	walker := AferoWalker{Fs: fsys}

	var visited []string
	err := walker.WalkDir("/root", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		visited = append(visited, path)

		return nil
	})

	require.NoError(t, err)
	require.Contains(t, visited, "/root")
	require.Contains(t, visited, "/root/file1.txt")
	require.Contains(t, visited, "/root/subdir")
	require.Contains(t, visited, "/root/subdir/file2.txt")
}

// Expectation: The walker should provide correct DirEntry information.
func Test_AferoWalker_WalkDir_DirEntry_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root/subdir", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/root/file.txt", []byte("content"), 0o644))

	walker := AferoWalker{Fs: fsys}

	entries := make(map[string]fs.DirEntry)
	err := walker.WalkDir("/root", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		entries[path] = d

		return nil
	})

	require.NoError(t, err)

	require.True(t, entries["/root"].IsDir())
	require.True(t, entries["/root/subdir"].IsDir())
	require.False(t, entries["/root/file.txt"].IsDir())
	require.NotNil(t, entries["/root/file.txt"].Type())

	require.Equal(t, "file.txt", entries["/root/file.txt"].Name())

	info, err := entries["/root/file.txt"].Info()
	require.NoError(t, err)
	require.Equal(t, int64(7), info.Size())
}

// Expectation: The walker should propagate errors from the walk function.
func Test_AferoWalker_WalkDir_Error(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fsys, "/root/file.txt", []byte("content"), 0o644))

	walker := AferoWalker{Fs: fsys}

	expectedErr := fs.ErrPermission
	err := walker.WalkDir("/root", func(path string, d fs.DirEntry, err error) error {
		if d != nil && !d.IsDir() {
			return expectedErr
		}

		return nil
	})

	require.ErrorIs(t, err, expectedErr)
}

// Expectation: The function should not skip a path when no ignore files exist.
func Test_ShouldIgnorePath_NoIgnoreFiles_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root/subdir", 0o755))

	skip := ShouldIgnorePath(fsys, "/root/subdir/file.txt", "/root")

	require.False(t, skip)
}

// Expectation: The function should skip a path when ignore file exists in the same directory.
func Test_ShouldIgnorePath_IgnoreFile_SkipsPath_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root/subdir", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/root/subdir/"+schema.IgnoreFile, []byte{}, 0o644))

	skip := ShouldIgnorePath(fsys, "/root/subdir/file.txt", "/root")

	require.True(t, skip)
}

// Expectation: The function should not skip a path when ignore file exists only in a parent directory.
func Test_ShouldIgnorePath_IgnoreFile_InParent_DoesNotSkip_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root/subdir", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/root/"+schema.IgnoreFile, []byte{}, 0o644))

	skip := ShouldIgnorePath(fsys, "/root/subdir/file.txt", "/root")

	require.False(t, skip)
}

// Expectation: The function should skip a path when ignore-all file exists in the same directory.
func Test_ShouldIgnorePath_IgnoreAllFile_SameDir_SkipsPath_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root/subdir", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/root/subdir/"+schema.IgnoreAllFile, []byte{}, 0o644))

	skip := ShouldIgnorePath(fsys, "/root/subdir/file.txt", "/root")

	require.True(t, skip)
}

// Expectation: The function should skip a path when ignore-all file exists in a parent directory.
func Test_ShouldIgnorePath_IgnoreAllFile_InParent_SkipsPath_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root/subdir/deep", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/root/"+schema.IgnoreAllFile, []byte{}, 0o644))

	skip := ShouldIgnorePath(fsys, "/root/subdir/deep/file.txt", "/root")

	require.True(t, skip)
}

// Expectation: The function should skip a path when ignore-all file exists in an intermediate directory.
func Test_ShouldIgnorePath_IgnoreAllFile_InIntermediate_SkipsPath_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root/mid/subdir", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/root/mid/"+schema.IgnoreAllFile, []byte{}, 0o644))

	skip := ShouldIgnorePath(fsys, "/root/mid/subdir/file.txt", "/root")

	require.True(t, skip)
}

// Expectation: The function should not walk above the root directory.
func Test_ShouldIgnorePath_StopsAtRoot_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root/subdir", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/"+schema.IgnoreAllFile, []byte{}, 0o644))

	skip := ShouldIgnorePath(fsys, "/root/subdir/file.txt", "/root")

	require.False(t, skip)
}

// Expectation: The function should handle a path directly in the root directory with ignore-all.
func Test_ShouldIgnorePath_PathInRoot_IgnoreAll_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/root/"+schema.IgnoreAllFile, []byte{}, 0o644))

	skip := ShouldIgnorePath(fsys, "/root/file.txt", "/root")

	require.True(t, skip)
}

// Expectation: The function should handle a path directly in the root directory with ignore file.
func Test_ShouldIgnorePath_PathInRoot_IgnoreFile_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/root/"+schema.IgnoreFile, []byte{}, 0o644))

	skip := ShouldIgnorePath(fsys, "/root/file.txt", "/root")

	require.True(t, skip)
}

// Expectation: The checker should not skip files when no ignore files exist.
func Test_IgnoreChecker_ShouldSkip_NoIgnoreFiles_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/root/file.txt", []byte("content"), 0o644))

	checker := NewIgnoreChecker(fsys)

	skip, err := checker.ShouldSkip("/root/file.txt", false)

	require.NoError(t, err)
	require.False(t, skip)
}

// Expectation: The checker should skip files when ignore file exists.
func Test_IgnoreChecker_ShouldSkip_IgnoreFile_SkipsFile_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/root/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fsys, "/root/"+schema.IgnoreFile, []byte{}, 0o644))

	checker := NewIgnoreChecker(fsys)

	skip, err := checker.ShouldSkip("/root/file.txt", false)

	require.NoError(t, err)
	require.True(t, skip)
}

// Expectation: The checker should not skip directories when only ignore file exists.
func Test_IgnoreChecker_ShouldSkip_IgnoreFile_DoesNotSkipDir_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root/subdir", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/root/"+schema.IgnoreFile, []byte{}, 0o644))

	checker := NewIgnoreChecker(fsys)

	skip, err := checker.ShouldSkip("/root/subdir", true)

	require.NoError(t, err)
	require.False(t, skip)
}

// Expectation: The checker should skip files when ignore-all file exists.
func Test_IgnoreChecker_ShouldSkip_IgnoreAllFile_SkipsFile_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/root/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fsys, "/root/"+schema.IgnoreAllFile, []byte{}, 0o644))

	checker := NewIgnoreChecker(fsys)

	skip, err := checker.ShouldSkip("/root/file.txt", false)

	require.NoError(t, err)
	require.True(t, skip)
}

// Expectation: The checker should skip directories with SkipDir when ignore-all file exists.
func Test_IgnoreChecker_ShouldSkip_IgnoreAllFile_SkipsDir_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root/subdir", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/root/"+schema.IgnoreAllFile, []byte{}, 0o644))

	checker := NewIgnoreChecker(fsys)

	skip, err := checker.ShouldSkip("/root/subdir", true)

	require.True(t, skip)
	require.ErrorIs(t, err, filepath.SkipDir)
}

// Expectation: The checker should cache ignore status for the same directory.
func Test_IgnoreChecker_ShouldSkip_CachesDirectory_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/root/file1.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fsys, "/root/file2.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fsys, "/root/"+schema.IgnoreFile, []byte{}, 0o644))

	checker := NewIgnoreChecker(fsys)

	skip1, _ := checker.ShouldSkip("/root/file1.txt", false)
	skip2, _ := checker.ShouldSkip("/root/file2.txt", false)

	require.True(t, skip1)
	require.True(t, skip2)
	require.Equal(t, "/root", checker.lastVisited)
}

// Expectation: The checker should update cache when directory changes.
func Test_IgnoreChecker_ShouldSkip_UpdatesCacheOnDirChange_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/root/dir1", 0o755))
	require.NoError(t, fsys.MkdirAll("/root/dir2", 0o755))
	require.NoError(t, afero.WriteFile(fsys, "/root/dir1/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fsys, "/root/dir2/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fsys, "/root/dir1/"+schema.IgnoreFile, []byte{}, 0o644))

	checker := NewIgnoreChecker(fsys)

	skip1, _ := checker.ShouldSkip("/root/dir1/file.txt", false)
	skip2, _ := checker.ShouldSkip("/root/dir2/file.txt", false)

	require.True(t, skip1)
	require.False(t, skip2)
}

// Expectation: No available lstat should pass all table tests.
func Test_HasGlobSymlinks_MemMapFs_NoLstat_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		workingDir string
		dirs       []string
		pattern    string
		want       bool
	}{
		{
			name:       "simple file pattern no symlinks",
			workingDir: "/project",
			dirs:       []string{"/project/src"},
			pattern:    "/project/src/*.go",
			want:       false,
		},
		{
			name:       "deep glob no symlinks",
			workingDir: "/project",
			dirs:       []string{"/project/src/pkg/internal"},
			pattern:    "/project/src/**/*.go",
			want:       false,
		},
		{
			name:       "dot pattern",
			workingDir: "/project",
			dirs:       []string{"/project"},
			pattern:    "/project/./*.go",
			want:       false,
		},
		{
			name:       "pattern base is dot",
			workingDir: "/project",
			dirs:       []string{"/project"},
			pattern:    "/project/*.go",
			want:       false,
		},
		{
			name:       "deeply nested dirs no symlinks",
			workingDir: "/project",
			dirs:       []string{"/project/a/b/c/d/e"},
			pattern:    "/project/a/b/c/d/e/*.txt",
			want:       false,
		},
		{
			name:       "double star at root",
			workingDir: "/project",
			dirs:       []string{"/project/foo"},
			pattern:    "/project/**/*.go",
			want:       false,
		},
		{
			name:       "nonexistent intermediate dir",
			workingDir: "/project",
			dirs:       []string{"/project"},
			pattern:    "/project/nonexistent/deep/path/**/*.go",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fsys := afero.NewMemMapFs()
			for _, d := range tt.dirs {
				require.NoError(t, fsys.MkdirAll(d, 0o755))
			}
			_, got := HasGlobSymlinks(fsys, tt.workingDir, tt.pattern)
			require.Equal(t, tt.want, got)
		})
	}
}

// Expectation: Available lstat should match symlinks in the pattern base.
func Test_HasGlobSymlinks_OsFs_HasLstat_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setup      func(t *testing.T, root string, mkDir func(path string), symlink func(target, path string))
		workingDir string
		pattern    string // relative to workingDir, will be prefixed in runner
		want       bool
		wantPath   string // relative to workingDir
	}{
		{
			name: "no symlinks shallow glob",
			setup: func(t *testing.T, root string, mkDir func(string), symlink func(string, string)) {
				t.Helper()
				// src/
				mkDir("src")
			},
			workingDir: "project",
			pattern:    "src/*.go",
			want:       false,
			wantPath:   "",
		},
		{
			name: "no symlinks deep glob",
			setup: func(t *testing.T, root string, mkDir func(string), symlink func(string, string)) {
				t.Helper()
				// src/pkg/internal/
				mkDir("src/pkg/internal")
			},
			workingDir: "project",
			pattern:    "src/**/*.go",
			want:       false,
			wantPath:   "",
		},
		{
			name: "symlink as direct pattern base",
			setup: func(t *testing.T, root string, mkDir func(string), symlink func(string, string)) {
				t.Helper()
				// <link>
				target := filepath.Join(root, "real_project")
				require.NoError(t, os.MkdirAll(target, 0o750))
				symlink(target, "link")
			},
			workingDir: "project",
			pattern:    "link/*.go",
			want:       true,
			wantPath:   "link",
		},
		{
			name: "symlink as beginning dir in longer pattern",
			setup: func(t *testing.T, root string, mkDir func(string), symlink func(string, string)) {
				t.Helper()
				// <link>/sub/
				target := filepath.Join(root, "real_project")
				require.NoError(t, os.MkdirAll(filepath.Join(target, "sub"), 0o750))
				symlink(target, "link")
			},
			workingDir: "project",
			pattern:    "link/sub/*.go",
			want:       true,
			wantPath:   "link",
		},
		{
			name: "symlink as intermediate dir in longer pattern",
			setup: func(t *testing.T, root string, mkDir func(string), symlink func(string, string)) {
				t.Helper()
				// a/<link>
				target := filepath.Join(root, "real_project")
				require.NoError(t, os.MkdirAll(target, 0o750))
				mkDir("a")
				symlink(target, "a/link")
			},
			workingDir: "project",
			pattern:    "a/link/*.go",
			want:       true,
			wantPath:   "a/link",
		},
		{
			name: "symlink deep in path with double star",
			setup: func(t *testing.T, root string, mkDir func(string), symlink func(string, string)) {
				t.Helper()
				// a/b/<link>
				target := filepath.Join(root, "real_project")
				require.NoError(t, os.MkdirAll(target, 0o750))
				mkDir("a/b")
				symlink(target, "a/b/link")
			},
			workingDir: "project",
			pattern:    "a/b/link/**/*.go",
			want:       true,
			wantPath:   "a/b/link",
		},
		{
			name: "symlink only in non-pattern part (workingDir itself)",
			setup: func(t *testing.T, root string, mkDir func(string), symlink func(string, string)) {
				t.Helper()
				// src/ (workingDir is a symlink)
				target := filepath.Join(root, "real_project")
				require.NoError(t, os.MkdirAll(filepath.Join(target, "src"), 0o750))
				require.NoError(t, os.Symlink(target, filepath.Join(root, "project_link")))
			},
			workingDir: "project_link",
			pattern:    "src/*.go",
			want:       false,
			wantPath:   "",
		},
		{
			name: "no symlinks deeply nested double star",
			setup: func(t *testing.T, root string, mkDir func(string), symlink func(string, string)) {
				t.Helper()
				// a/b/c/d/
				mkDir("a/b/c/d")
			},
			workingDir: "project",
			pattern:    "a/b/c/d/**/*.go",
			want:       false,
			wantPath:   "",
		},
		{
			name: "symlink at first level of pattern",
			setup: func(t *testing.T, root string, mkDir func(string), symlink func(string, string)) {
				t.Helper()
				// <vendor>/deep/nested/
				target := filepath.Join(root, "real_project")
				require.NoError(t, os.MkdirAll(filepath.Join(target, "deep", "nested"), 0o750))
				symlink(target, "vendor")
			},
			workingDir: "project",
			pattern:    "vendor/deep/nested/**/*.go",
			want:       true,
			wantPath:   "vendor",
		},
		{
			name: "regular dir then symlink then regular dir",
			setup: func(t *testing.T, root string, mkDir func(string), symlink func(string, string)) {
				t.Helper()
				// a/<sym>/child/
				target := filepath.Join(root, "real_project")
				require.NoError(t, os.MkdirAll(filepath.Join(target, "child"), 0o750))
				mkDir("a")
				symlink(target, "a/sym")
			},
			workingDir: "project",
			pattern:    "a/sym/child/*.go",
			want:       true,
			wantPath:   "a/sym",
		},
		{
			name: "pattern base is dot",
			setup: func(t *testing.T, root string, mkDir func(string), symlink func(string, string)) {
				t.Helper()
			},
			workingDir: "project",
			pattern:    "*.go",
			want:       false,
			wantPath:   "",
		},
		{
			name: "nonexistent path in pattern",
			setup: func(t *testing.T, root string, mkDir func(string), symlink func(string, string)) {
				t.Helper()
			},
			workingDir: "project",
			pattern:    "does/not/exist/**/*.go",
			want:       false,
			wantPath:   "",
		},
		{
			name: "multiple symlinks in path finds deepest",
			setup: func(t *testing.T, root string, mkDir func(string), symlink func(string, string)) {
				t.Helper()
				// <sym1>/sub/<sym2>
				require.NoError(t, os.MkdirAll(filepath.Join(root, "target1", "sub"), 0o750))
				require.NoError(t, os.MkdirAll(filepath.Join(root, "target2"), 0o750))
				symlink(filepath.Join(root, "target1"), "sym1")
				require.NoError(t, os.Symlink(filepath.Join(root, "target2"), filepath.Join(root, "target1", "sub", "sym2")))
			},
			workingDir: "project",
			pattern:    "sym1/sub/sym2/**/*.go",
			want:       true,
			wantPath:   "sym1/sub/sym2",
		},
		{
			name: "escaped meta chars in path portion",
			setup: func(t *testing.T, root string, mkDir func(string), symlink func(string, string)) {
				t.Helper()
				// a/b[1]/c*d/
				mkDir("a/b[1]/c*d")
			},
			workingDir: "project",
			pattern:    "a/b\\[1\\]/c\\*d/*.go",
			want:       false,
			wantPath:   "",
		},
		{
			name: "symlink with escaped meta chars in path",
			setup: func(t *testing.T, root string, mkDir func(string), symlink func(string, string)) {
				t.Helper()
				// a/<link{1}>
				target := filepath.Join(root, "real_project")
				require.NoError(t, os.MkdirAll(target, 0o750))
				mkDir("a")
				symlink(target, "a/link{1}")
			},
			workingDir: "project",
			pattern:    "a/link\\{1\\}/*.go",
			want:       true,
			wantPath:   "a/link{1}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			workingDir := filepath.Join(root, tt.workingDir)

			mkDir := func(path string) {
				require.NoError(t, os.MkdirAll(filepath.Join(workingDir, path), 0o750))
			}
			symlink := func(target, path string) {
				require.NoError(t, os.Symlink(target, filepath.Join(workingDir, path)))
			}

			// Ensure workingDir exists before setup (unless the test creates it as a symlink)
			if tt.name != "symlink only in non-pattern part (workingDir itself)" {
				mkDir(".")
			}

			tt.setup(t, root, mkDir, symlink)

			// Prefix the pattern with workingDir to make it absolute
			pattern := workingDir + "/" + tt.pattern

			fsys := afero.NewOsFs()
			gotPath, got := HasGlobSymlinks(fsys, workingDir, pattern)
			require.Equal(t, tt.want, got)

			if tt.wantPath != "" {
				relPath, err := filepath.Rel(workingDir, gotPath)
				require.NoError(t, err)
				require.Equal(t, tt.wantPath, relPath)
			} else {
				require.Empty(t, gotPath)
			}
		})
	}
}
