package util

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

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

	err := WriteManifest(fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, mf)

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

	err := WriteManifest(fsys, "/data/test"+schema.ManifestExtension, mf)
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

	err := WriteManifest(fsys, "/data/test"+schema.ManifestExtension, mf)
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

	err := WriteManifest(fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, mf)

	require.ErrorContains(t, err, "failed to write")

	exists, _ := afero.Exists(fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension)
	require.False(t, exists)
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
