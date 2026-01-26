package repair

import (
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: A new backup restorer should be created with the correct values and initialized state.
func Test_newBackupRestorer_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	log := slog.New(slog.DiscardHandler)

	restorer, err := newBackupRestorer(fs, log, dir)

	require.NoError(t, err)
	require.Equal(t, fs, restorer.fsys)
	require.Equal(t, log, restorer.log)
	require.Equal(t, dir, restorer.dir)
	require.NotNil(t, restorer.before)
}

// Expectation: Constructor should capture the current state of files with inodes.
func Test_newBackupRestorer_CapturesState_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/file1.txt", []byte("content1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/file2.txt", []byte("content2"), 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)

	require.NoError(t, err)
	require.Len(t, restorer.before, 2)
}

// Expectation: Constructor should handle empty directory.
func Test_newBackupRestorer_EmptyDir_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)

	require.NoError(t, err)
	require.Empty(t, restorer.before)
}

// Expectation: Constructor should return error when directory doesn't exist.
func Test_newBackupRestorer_MissingDir_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	log := slog.New(slog.DiscardHandler)

	_, err := newBackupRestorer(fs, log, "/nonexistent-dir-that-does-not-exist")

	require.ErrorContains(t, err, "failed to establish before-state")
}

// Expectation: Constructor should return error when reading directory fails.
func Test_newBackupRestorer_OpenError_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewOsFs()
	dir := t.TempDir()

	fs := &testutil.FailingOpenFs{
		Fs:          baseFs,
		FailPattern: dir,
	}

	log := slog.New(slog.DiscardHandler)
	_, err := newBackupRestorer(fs, log, dir)

	require.ErrorContains(t, err, "failed to establish before-state")
}

// Expectation: Restore should not touch pre-existing numbered backup files.
func Test_backupRestorer_Restore_UnchangedOlderBackup_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()

	// Create a pre-existing numbered backup file before initializing restorer
	require.NoError(t, afero.WriteFile(fs, dir+"/oldfile.txt.1", []byte("old backup"), 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)
	require.NoError(t, err)

	// Don't rename anything - the numbered file stays as-is

	require.NoError(t, restorer.Restore())

	// The old backup file should remain untouched
	exists, _ := afero.Exists(fs, dir+"/oldfile.txt.1")
	require.True(t, exists)

	// No oldfile.txt should be created
	exists, _ = afero.Exists(fs, dir+"/oldfile.txt")
	require.False(t, exists)
}

// Expectation: Restore should rename numbered backup files back to original names.
func Test_backupRestorer_Restore_RestoresRenamedFiles_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/file.txt", []byte("content"), 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)
	require.NoError(t, err)

	// Simulate external tool renaming file to numbered backup
	require.NoError(t, fs.Rename(dir+"/file.txt", dir+"/file.txt.1"))

	require.NoError(t, restorer.Restore())

	// File should be restored to original name
	exists, _ := afero.Exists(fs, dir+"/file.txt")
	require.True(t, exists)
	exists, _ = afero.Exists(fs, dir+"/file.txt.1")
	require.False(t, exists)
}

// Expectation: Restore should not touch files that weren't renamed.
func Test_backupRestorer_Restore_IgnoresUnchangedFiles_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/file.txt", []byte("content"), 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)
	require.NoError(t, err)

	require.NoError(t, restorer.Restore())

	// File should remain unchanged
	exists, _ := afero.Exists(fs, dir+"/file.txt")
	require.True(t, exists)
}

// Expectation: Restore should not restore files with different inodes.
func Test_backupRestorer_Restore_IgnoresNewFiles_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)
	require.NoError(t, err)

	// Create new numbered file after initialization
	require.NoError(t, afero.WriteFile(fs, dir+"/file.txt.1", []byte("new"), 0o644))

	require.NoError(t, restorer.Restore())

	// New file should remain as-is
	exists, _ := afero.Exists(fs, dir+"/file.txt.1")
	require.True(t, exists)
}

// Expectation: Restore should not restore files with size mismatch.
func Test_backupRestorer_Restore_SizeMismatch_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	originalPath := dir + "/file.txt"
	backupPath := dir + "/file.txt.1"
	require.NoError(t, afero.WriteFile(fs, originalPath, []byte("content"), 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)
	require.NoError(t, err)

	// Rename file
	require.NoError(t, fs.Rename(originalPath, backupPath))
	// Modify content (changing size) - must write to same inode
	f, err := fs.OpenFile(backupPath, 2, 0o644) // 2 = O_RDWR
	require.NoError(t, err)
	_, err = f.WriteString("modified content that is longer")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	require.NoError(t, restorer.Restore())

	// File should not be restored due to size mismatch
	exists, _ := afero.Exists(fs, backupPath)
	require.True(t, exists)
	exists, _ = afero.Exists(fs, originalPath)
	require.False(t, exists)
}

// Expectation: Restore should handle multiple renamed files.
func Test_backupRestorer_Restore_MultipleFiles_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/file1.txt", []byte("content1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/file2.txt", []byte("content2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/file3.txt", []byte("content3"), 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)
	require.NoError(t, err)

	// Rename all files
	require.NoError(t, fs.Rename(dir+"/file1.txt", dir+"/file1.txt.1"))
	require.NoError(t, fs.Rename(dir+"/file2.txt", dir+"/file2.txt.5"))
	require.NoError(t, fs.Rename(dir+"/file3.txt", dir+"/file3.txt.999"))

	require.NoError(t, restorer.Restore())

	// All files should be restored
	exists, _ := afero.Exists(fs, dir+"/file1.txt")
	require.True(t, exists)
	exists, _ = afero.Exists(fs, dir+"/file2.txt")
	require.True(t, exists)
	exists, _ = afero.Exists(fs, dir+"/file3.txt")
	require.True(t, exists)

	// Numbered versions should be gone
	exists, _ = afero.Exists(fs, dir+"/file1.txt.1")
	require.False(t, exists)
	exists, _ = afero.Exists(fs, dir+"/file2.txt.5")
	require.False(t, exists)
	exists, _ = afero.Exists(fs, dir+"/file3.txt.999")
	require.False(t, exists)
}

// Expectation: Restore should only process files in current directory, not subdirectories.
func Test_backupRestorer_Restore_IgnoresSubdirectories_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, fs.MkdirAll(dir+"/subdir", 0o755))
	require.NoError(t, afero.WriteFile(fs, dir+"/file.txt", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/subdir/file.txt", []byte("content"), 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)
	require.NoError(t, err)

	// Rename files in both locations
	require.NoError(t, fs.Rename(dir+"/file.txt", dir+"/file.txt.1"))
	require.NoError(t, fs.Rename(dir+"/subdir/file.txt", dir+"/subdir/file.txt.1"))

	require.NoError(t, restorer.Restore())

	// Current dir file should be restored
	exists, _ := afero.Exists(fs, dir+"/file.txt")
	require.True(t, exists)

	// Subdirectory file should remain renamed (not tracked)
	exists, _ = afero.Exists(fs, dir+"/subdir/file.txt.1")
	require.True(t, exists)
}

// Expectation: Restore should continue when file rename fails.
func Test_backupRestorer_Restore_RenameError_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(baseFs, dir+"/file.txt", []byte("content"), 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(baseFs, log, dir)
	require.NoError(t, err)

	// Rename file first
	require.NoError(t, baseFs.Rename(dir+"/file.txt", dir+"/file.txt.1"))

	// Wrap with failing fs after rename
	fs := &testutil.FailingRenameFs{
		Fs:          baseFs,
		FailPattern: "file.txt",
	}
	restorer.fsys = fs

	err = restorer.Restore()
	require.NoError(t, err)
}

// Expectation: Restore should handle multiple files when some renames fail.
func Test_backupRestorer_Restore_PartialRenameError_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(baseFs, dir+"/file1.txt", []byte("content1"), 0o644))
	require.NoError(t, afero.WriteFile(baseFs, dir+"/file2.txt", []byte("content2"), 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(baseFs, log, dir)
	require.NoError(t, err)

	// Rename both files
	require.NoError(t, baseFs.Rename(dir+"/file1.txt", dir+"/file1.txt.1"))
	require.NoError(t, baseFs.Rename(dir+"/file2.txt", dir+"/file2.txt.1"))

	// Wrap with failing fs - only file1 will fail
	fs := &testutil.FailingRenameFs{
		Fs:          baseFs,
		FailPattern: "file1.txt",
	}
	restorer.fsys = fs

	err = restorer.Restore()
	require.NoError(t, err)

	// file2 should be restored successfully
	exists, _ := afero.Exists(fs, dir+"/file2.txt")
	require.True(t, exists)
}

// Expectation: Restore should return error when after-state reading fails.
func Test_backupRestorer_Restore_PostReadDirError_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)
	require.NoError(t, err)

	// Remove directory to cause read error
	require.NoError(t, fs.RemoveAll(dir))

	err = restorer.Restore()

	require.ErrorContains(t, err, "failed to establish after-state")
}

// Expectation: getFilesWithInodes should capture all files with their inodes.
func Test_backupRestorer_getFilesWithInodes_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/file1.txt", []byte("content1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/file2.txt", []byte("content2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/file.txt.1", []byte("backup"), 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)
	require.NoError(t, err)

	files, err := restorer.getFilesWithInodes()

	require.NoError(t, err)
	require.Len(t, files, 3)
}

// Expectation: getFilesWithInodes should skip directories.
func Test_backupRestorer_getFilesWithInodes_SkipsDirs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, fs.MkdirAll(dir+"/subdir", 0o755))
	require.NoError(t, afero.WriteFile(fs, dir+"/file.txt", []byte("file"), 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)
	require.NoError(t, err)

	files, err := restorer.getFilesWithInodes()

	require.NoError(t, err)
	require.Len(t, files, 1)
}

// Expectation: getNumberedFilesWithInodes should only return files with numeric extensions.
func Test_backupRestorer_getNumberedFilesWithInodes_Pattern_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/file.txt.1", []byte("match"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/file.txt.999", []byte("match"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/file.txt", []byte("no match"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/file.txt.bak", []byte("no match"), 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)
	require.NoError(t, err)

	files, err := restorer.getNumberedFilesWithInodes()

	require.NoError(t, err)
	require.Len(t, files, 2)
}

// Expectation: getNumberedFilesWithInodes should skip directories.
func Test_backupRestorer_getNumberedFilesWithInodes_SkipsDirs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, fs.MkdirAll(dir+"/subdir.1", 0o755))
	require.NoError(t, afero.WriteFile(fs, dir+"/file.txt.1", []byte("file"), 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)
	require.NoError(t, err)

	files, err := restorer.getNumberedFilesWithInodes()

	require.NoError(t, err)
	require.Len(t, files, 1)
}

// Expectation: getInode should return the inode.
func Test_backupRestorer_getInode_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()

	require.NoError(t, afero.WriteFile(fs, dir+"/file.txt", []byte("file"), 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)
	require.NoError(t, err)

	inode, err := restorer.getInode(dir + "/file.txt")
	require.NoError(t, err)
	require.NotZero(t, inode)
}

// Expectation: getInode should return error when stat fails.
func Test_backupRestorer_getInode_StatError_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewOsFs()
	dir := t.TempDir()

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(baseFs, log, dir)
	require.NoError(t, err)

	fs := &testutil.FailingStatFs{
		Fs:          baseFs,
		FailPattern: "file.txt",
	}
	restorer.fsys = fs

	_, err = restorer.getInode(dir + "/file.txt")

	require.ErrorContains(t, err, "failed to stat")
}

// Expectation: getInode should return error when Sys() doesn't return syscall.Stat_t.
func Test_backupRestorer_getInode_NoSyscallStat_Error(t *testing.T) {
	t.Parallel()

	// Use MemMapFs which doesn't provide syscall.Stat_t
	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt", []byte("content"), 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer := &backupRestorer{
		fsys:   fs,
		log:    log,
		dir:    "/data",
		before: make(map[uint64]fileRecord),
	}

	_, err := restorer.getInode("/data/file.txt")

	require.ErrorIs(t, err, errNoSyscallStatT)
}

// Expectation: Restore should preserve file content after restoration.
func Test_backupRestorer_Restore_PreservesContent_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	originalContent := []byte("important content that must be preserved")
	require.NoError(t, afero.WriteFile(fs, dir+"/file.txt", originalContent, 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)
	require.NoError(t, err)

	// Rename file
	require.NoError(t, fs.Rename(dir+"/file.txt", dir+"/file.txt.1"))

	require.NoError(t, restorer.Restore())

	// Verify content is preserved
	content, err := afero.ReadFile(fs, dir+"/file.txt")
	require.NoError(t, err)
	require.Equal(t, originalContent, content)
}

// Expectation: Restore should not restore files with different inodes.
func Test_backupRestorer_Restore_DifferentInodeSameContent_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()

	content := []byte("same content")
	originalPath := filepath.Join(dir, "file.txt")
	newPath := filepath.Join(dir, "file.txt.1")

	require.NoError(t, afero.WriteFile(fs, originalPath, content, 0o644))

	log := slog.New(slog.DiscardHandler)
	restorer, err := newBackupRestorer(fs, log, dir)
	require.NoError(t, err)

	var originalInode uint64
	for inode := range restorer.before {
		originalInode = inode

		break
	}

	// Create the new file first, then delete the old one.
	// This forces the OS to use a different inode for file.txt.1
	require.NoError(t, afero.WriteFile(fs, newPath, content, 0o644))
	require.NoError(t, fs.Remove(originalPath))

	newInode, err := restorer.getInode(newPath)
	require.NoError(t, err)

	if originalInode == newInode {
		t.Fatalf("Filesystem still reused inode %d despite parallel existence.", originalInode)
	}

	require.NoError(t, restorer.Restore())

	exists, _ := afero.Exists(fs, newPath)
	require.True(t, exists, "The numbered file should still exist")
	exists, _ = afero.Exists(fs, originalPath)
	require.False(t, exists, "The original file should not have been restored")
}
