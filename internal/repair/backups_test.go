package repair

import (
	"errors"
	"io"
	"os"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// bmNewManager creates a [backupManager] for the given protected file names,
// as they would be listed in the recovery set of the PAR2 file in dir.
func bmNewManager(t *testing.T, fs afero.Fs, dir string, names ...string) (*backupManager, *testutil.SafeBuffer) {
	t.Helper()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	par2Path := dir + "/test" + schema.Par2Extension
	job := &Job{workingDir: dir, par2Path: par2Path}

	man, err := newBackupManager(t.Context(), job, fs, mockPar2Files(t, par2Path, names...), logging.NewLogger(ls))
	require.NoError(t, err)

	return man, &logBuf
}

// bmStat returns the [syscall.Stat_t] of a path (without following symlinks).
func bmStat(t *testing.T, path string) *syscall.Stat_t {
	t.Helper()

	info, err := os.Lstat(path)
	require.NoError(t, err)

	st, ok := info.Sys().(*syscall.Stat_t)
	require.True(t, ok)

	return st
}

// bmReplace simulates par2 renaming an original file to a backup
// path and writing the reconstructed content at the original path.
func bmReplace(t *testing.T, fs afero.Fs, path, backupPath, content string) {
	t.Helper()

	require.NoError(t, fs.Rename(path, backupPath))
	require.NoError(t, afero.WriteFile(fs, path, []byte(content), 0o644))
	require.NoError(t, fs.Chmod(path, 0o644))
}

// bmForeignGID returns a group ID the current user is not a member of.
func bmForeignGID(t *testing.T) uint32 {
	t.Helper()

	groups, err := os.Getgroups()
	require.NoError(t, err)
	groups = append(groups, os.Getegid())

	for gid := 60000; ; gid++ {
		if !slices.Contains(groups, gid) {
			return uint32(gid)
		}
	}
}

// Expectation: The constructor should record the protected files of the PAR2 recovery set.
func Test_newBackupManager_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/b.txt", []byte("b"), 0o644))

	man, _ := bmNewManager(t, fs, dir, "a.txt", "b.txt")

	require.Len(t, man.snapshot, 2)
	require.Equal(t, bmStat(t, dir+"/a.txt").Ino, man.snapshot[dir+"/a.txt"].Ino)
	require.Equal(t, bmStat(t, dir+"/b.txt").Ino, man.snapshot[dir+"/b.txt"].Ino)
	require.Empty(t, man.backups)
}

// Expectation: The constructor should record protected files in subdirectories.
func Test_newBackupManager_Subdirectory_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, fs.MkdirAll(dir+"/sub/deep", 0o755))
	require.NoError(t, afero.WriteFile(fs, dir+"/sub/deep/a.txt", []byte("a"), 0o644))

	man, _ := bmNewManager(t, fs, dir, "sub/deep/a.txt")

	require.Len(t, man.snapshot, 1)
	require.Contains(t, man.snapshot, dir+"/sub/deep/a.txt")
}

// Expectation: The constructor should convert Windows separators in PAR2 names.
func Test_newBackupManager_WindowsSeparators_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, fs.MkdirAll(dir+"/sub", 0o755))
	require.NoError(t, afero.WriteFile(fs, dir+"/sub/a.txt", []byte("a"), 0o644))

	man, _ := bmNewManager(t, fs, dir, `sub\a.txt`)

	require.Len(t, man.snapshot, 1)
	require.Contains(t, man.snapshot, dir+"/sub/a.txt")
}

// Expectation: The constructor should record protected files of all sets in the PAR2 file.
func Test_newBackupManager_MultipleSets_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/b.txt", []byte("b"), 0o644))

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
			return &par2.File{Sets: []par2.Set{
				{RecoverySet: []par2.FilePacket{{Name: "a.txt"}}},
				{RecoverySet: []par2.FilePacket{{Name: "b.txt"}}},
			}}, nil
		},
	}
	log := logging.NewLogger(logging.Options{Logout: io.Discard, Stdout: io.Discard, Stderr: io.Discard})
	job := &Job{workingDir: dir, par2Path: dir + "/test" + schema.Par2Extension}

	man, err := newBackupManager(t.Context(), job, fs, par2er, log)

	require.NoError(t, err)
	require.Len(t, man.snapshot, 2)
}

// Expectation: The constructor should not record files of the non-recovery set.
func Test_newBackupManager_NonRecoverySet_Ignored(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/b.txt", []byte("b"), 0o644))

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
			return &par2.File{Sets: []par2.Set{{
				RecoverySet:    []par2.FilePacket{{Name: "a.txt"}},
				NonRecoverySet: []par2.FilePacket{{Name: "b.txt"}},
			}}}, nil
		},
	}
	log := logging.NewLogger(logging.Options{Logout: io.Discard, Stdout: io.Discard, Stderr: io.Discard})
	job := &Job{workingDir: dir, par2Path: dir + "/test" + schema.Par2Extension}

	man, err := newBackupManager(t.Context(), job, fs, par2er, log)

	require.NoError(t, err)
	require.Len(t, man.snapshot, 1)
	require.Contains(t, man.snapshot, dir+"/a.txt")
}

// Expectation: The constructor should return an error when the PAR2 file cannot be parsed.
func Test_newBackupManager_ParseError_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
			return nil, errors.New("simulated parse failure")
		},
	}
	log := logging.NewLogger(logging.Options{Logout: io.Discard, Stdout: io.Discard, Stderr: io.Discard})
	job := &Job{workingDir: dir, par2Path: dir + "/test" + schema.Par2Extension}

	man, err := newBackupManager(t.Context(), job, fs, par2er, log)

	require.ErrorContains(t, err, "failed to parse par2")
	require.Nil(t, man)
}

// Expectation: The constructor should skip protected files that do not exist.
func Test_newBackupManager_MissingFile_Skipped(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("a"), 0o644))

	man, logBuf := bmNewManager(t, fs, dir, "a.txt", "missing.txt")

	require.Len(t, man.snapshot, 1)
	require.NotContains(t, man.snapshot, dir+"/missing.txt")
	require.Contains(t, logBuf.String(), "Skipping missing PAR2-referenced file")
}

// Expectation: The constructor should skip PAR2 names escaping the working directory.
func Test_newBackupManager_ParentTraversal_Skipped(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	root := t.TempDir()
	dir := root + "/work"
	require.NoError(t, fs.MkdirAll(dir, 0o755))
	require.NoError(t, afero.WriteFile(fs, root+"/outside.txt", []byte("outside"), 0o644))

	man, logBuf := bmNewManager(t, fs, dir, "../outside.txt")

	require.Empty(t, man.snapshot)
	require.Contains(t, logBuf.String(), "Skipping invalid PAR2-referenced filename")
}

// Expectation: The constructor should skip absolute PAR2 names.
func Test_newBackupManager_AbsoluteName_Skipped(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("a"), 0o644))

	man, logBuf := bmNewManager(t, fs, dir, dir+"/a.txt")

	require.Empty(t, man.snapshot)
	require.Contains(t, logBuf.String(), "Skipping invalid PAR2-referenced filename")
}

// Expectation: The constructor should skip protected paths that are symlinks.
func Test_newBackupManager_Symlink_Skipped(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/target.txt", []byte("target"), 0o644))
	require.NoError(t, os.Symlink(dir+"/target.txt", dir+"/link.txt"))

	man, logBuf := bmNewManager(t, fs, dir, "link.txt")

	require.Empty(t, man.snapshot)
	require.Contains(t, logBuf.String(), "Skipping non-regular PAR2-referenced file")
}

// Expectation: The constructor should skip protected paths that are directories.
func Test_newBackupManager_Directory_Skipped(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, fs.MkdirAll(dir+"/sub", 0o755))

	man, logBuf := bmNewManager(t, fs, dir, "sub")

	require.Empty(t, man.snapshot)
	require.Contains(t, logBuf.String(), "Skipping non-regular PAR2-referenced file")
}

// Expectation: The constructor should skip protected files that cannot be accessed.
func Test_newBackupManager_StatError_Skipped(t *testing.T) {
	t.Parallel()

	fs := &testutil.FailingStatFs{Fs: afero.NewOsFs(), FailPattern: "denied.txt"}
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/denied.txt", []byte("denied"), 0o644))

	man, logBuf := bmNewManager(t, fs, dir, "a.txt", "denied.txt")

	require.Len(t, man.snapshot, 1)
	require.NotContains(t, man.snapshot, dir+"/denied.txt")
	require.Contains(t, logBuf.String(), "Skipping non-accessible PAR2-referenced file")
}

// Expectation: FindBackups should pair a renamed original with its replaced path.
func Test_backupManager_FindBackups_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))

	man, _ := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "repaired")

	man.FindBackups()

	require.Len(t, man.backups, 1)
	pair := man.backups[man.snapshot[dir+"/a.txt"]]
	require.Equal(t, dir+"/a.txt", pair.originalPath)
	require.Equal(t, dir+"/a.txt.1", pair.backupPath)
}

// Expectation: FindBackups should not pair files that par2 has not replaced.
func Test_backupManager_FindBackups_UntouchedFile_NotFound(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))

	man, logBuf := bmNewManager(t, fs, dir, "a.txt")

	man.FindBackups()

	require.Empty(t, man.backups)
	require.NotContains(t, logBuf.String(), "Replaced file has no identifiable backup")
}

// Expectation: FindBackups should find a backup placed after a pre-existing backup.
func Test_backupManager_FindBackups_ExistingBackup_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt.1", []byte("older backup"), 0o644))

	man, _ := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.2", "repaired")

	man.FindBackups()

	require.Len(t, man.backups, 1)
	require.Equal(t, dir+"/a.txt.2", man.backups[man.snapshot[dir+"/a.txt"]].backupPath)
}

// Expectation: FindBackups should find a backup behind a gap in the numbering.
func Test_backupManager_FindBackups_NumberingGap_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))

	man, _ := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.3", "repaired")

	man.FindBackups()

	require.Len(t, man.backups, 1)
	require.Equal(t, dir+"/a.txt.3", man.backups[man.snapshot[dir+"/a.txt"]].backupPath)
}

// Expectation: FindBackups should find a backup when par2 did not write a replacement.
func Test_backupManager_FindBackups_NoReplacement_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))

	man, _ := bmNewManager(t, fs, dir, "a.txt")
	require.NoError(t, fs.Rename(dir+"/a.txt", dir+"/a.txt.1"))

	man.FindBackups()

	require.Len(t, man.backups, 1)
	require.Equal(t, dir+"/a.txt.1", man.backups[man.snapshot[dir+"/a.txt"]].backupPath)
}

// Expectation: FindBackups should pair files with the same name in different directories correctly.
func Test_backupManager_FindBackups_Subdirectories_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, fs.MkdirAll(dir+"/sub1", 0o755))
	require.NoError(t, fs.MkdirAll(dir+"/sub2", 0o755))
	require.NoError(t, afero.WriteFile(fs, dir+"/sub1/a.txt", []byte("sub1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/sub2/a.txt", []byte("sub2"), 0o644))

	man, _ := bmNewManager(t, fs, dir, "sub1/a.txt", "sub2/a.txt")
	bmReplace(t, fs, dir+"/sub1/a.txt", dir+"/sub1/a.txt.1", "sub1 repaired")
	bmReplace(t, fs, dir+"/sub2/a.txt", dir+"/sub2/a.txt.1", "sub2 repaired")

	man.FindBackups()

	require.Len(t, man.backups, 2)
	require.Equal(t, dir+"/sub1/a.txt.1", man.backups[man.snapshot[dir+"/sub1/a.txt"]].backupPath)
	require.Equal(t, dir+"/sub2/a.txt.1", man.backups[man.snapshot[dir+"/sub2/a.txt"]].backupPath)
}

// Expectation: FindBackups should not pair a numbered file that is not the original inode.
func Test_backupManager_FindBackups_UnrelatedNumberedFile_NotFound(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))

	man, logBuf := bmNewManager(t, fs, dir, "a.txt")
	// Keep the original inode alive, so the new files cannot reuse its number.
	require.NoError(t, fs.Rename(dir+"/a.txt", dir+"/moved"))
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("replaced"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt.1", []byte("unrelated"), 0o644))

	man.FindBackups()

	require.Empty(t, man.backups)
	require.Contains(t, logBuf.String(), "Replaced file has no identifiable backup")
}

// Expectation: FindBackups should not pair the original inode under another file's numbered name.
func Test_backupManager_FindBackups_OtherBaseName_NotFound(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))

	man, logBuf := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/b.txt.1", "repaired")

	man.FindBackups()

	require.Empty(t, man.backups)
	require.Contains(t, logBuf.String(), "Replaced file has no identifiable backup")
}

// Expectation: FindBackups should discard pairs found by a previous call.
func Test_backupManager_FindBackups_ResetsPrevious_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))

	man, _ := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "repaired")
	man.FindBackups()
	require.Len(t, man.backups, 1)

	require.NoError(t, fs.Rename(dir+"/a.txt.1", dir+"/a.txt"))
	man.FindBackups()

	require.Empty(t, man.backups)
}

// Expectation: findNumbered should index regular numbered files by their inode.
func Test_backupManager_findNumbered_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt.1", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/b.txt.23", []byte("b"), 0o644))

	man := &backupManager{log: logging.NewLogger(logging.Options{Logout: io.Discard}), fsys: fs}

	found := man.findNumbered(dir)

	stA := bmStat(t, dir+"/a.txt.1")
	stB := bmStat(t, dir+"/b.txt.23")
	require.Len(t, found, 2)
	require.Equal(t, "a.txt.1", found[fileID{stA.Dev, stA.Ino}])
	require.Equal(t, "b.txt.23", found[fileID{stB.Dev, stB.Ino}])
}

// Expectation: findNumbered should not index files without a numeric extension.
func Test_backupManager_findNumbered_NotNumbered_Ignored(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/b.1a", []byte("b"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/c.", []byte("c"), 0o644))

	man := &backupManager{log: logging.NewLogger(logging.Options{Logout: io.Discard}), fsys: fs}

	found := man.findNumbered(dir)

	require.Empty(t, found)
}

// Expectation: findNumbered should not index numbered directories and symlinks.
func Test_backupManager_findNumbered_NonRegular_Ignored(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, fs.MkdirAll(dir+"/dir.1", 0o755))
	require.NoError(t, afero.WriteFile(fs, dir+"/target.txt", []byte("target"), 0o644))
	require.NoError(t, os.Symlink(dir+"/target.txt", dir+"/link.txt.2"))

	man := &backupManager{log: logging.NewLogger(logging.Options{Logout: io.Discard}), fsys: fs}

	found := man.findNumbered(dir)

	require.Empty(t, found)
}

// Expectation: findNumbered should return an empty index when the directory cannot be read.
func Test_backupManager_findNumbered_ReadError_Empty(t *testing.T) {
	t.Parallel()

	var logBuf testutil.SafeBuffer
	fs := afero.NewOsFs()
	man := &backupManager{log: logging.NewLogger(logging.Options{Logout: &logBuf}), fsys: fs}

	found := man.findNumbered(t.TempDir() + "/nonexistent")

	require.NotNil(t, found)
	require.Empty(t, found)
	require.Contains(t, logBuf.String(), "Failed to read directory for backup lookup")
}

// Expectation: isBackupName should accept the original name followed by a numeric extension.
func Test_isBackupName_NumericSuffix_True(t *testing.T) {
	t.Parallel()

	require.True(t, isBackupName("a.txt.1", "a.txt"))
	require.True(t, isBackupName("a.txt.123", "a.txt"))
}

// Expectation: isBackupName should reject the original name without a numeric extension.
func Test_isBackupName_MissingSuffix_False(t *testing.T) {
	t.Parallel()

	require.False(t, isBackupName("a.txt", "a.txt"))
	require.False(t, isBackupName("a.txt.", "a.txt"))
}

// Expectation: isBackupName should reject extensions containing non-digit characters.
func Test_isBackupName_NonDigitSuffix_False(t *testing.T) {
	t.Parallel()

	require.False(t, isBackupName("a.txt.1a", "a.txt"))
	require.False(t, isBackupName("a.txt.bak", "a.txt"))
	require.False(t, isBackupName("a.txt.-1", "a.txt"))
}

// Expectation: isBackupName should reject numbered names of other files.
func Test_isBackupName_OtherBaseName_False(t *testing.T) {
	t.Parallel()

	require.False(t, isBackupName("b.txt.1", "a.txt"))
	require.False(t, isBackupName("aa.txt.1", "a.txt"))
}

// Expectation: isBackupName should reject names where the original is only a prefix.
func Test_isBackupName_OriginalPrefixOnly_False(t *testing.T) {
	t.Parallel()

	require.False(t, isBackupName("a.txt.1", "a"))
}

// Expectation: Purge should remove the backup and keep the repaired file.
func Test_backupManager_Purge_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))

	man, _ := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "repaired")
	man.FindBackups()

	man.Purge()

	backupExists, _ := afero.Exists(fs, dir+"/a.txt.1")
	require.False(t, backupExists)

	content, err := afero.ReadFile(fs, dir+"/a.txt")
	require.NoError(t, err)
	require.Equal(t, "repaired", string(content))
	require.Empty(t, man.backups)
}

// Expectation: Purge should not remove pre-existing numbered files.
func Test_backupManager_Purge_ExistingBackup_Kept(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt.1", []byte("older backup"), 0o644))

	man, _ := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.2", "repaired")
	man.FindBackups()

	man.Purge()

	olderExists, _ := afero.Exists(fs, dir+"/a.txt.1")
	require.True(t, olderExists)

	backupExists, _ := afero.Exists(fs, dir+"/a.txt.2")
	require.False(t, backupExists)
}

// Expectation: Purge should keep a backup that was replaced since FindBackups.
func Test_backupManager_Purge_BackupChanged_Kept(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))

	man, logBuf := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "repaired")
	man.FindBackups()

	// Keep the original inode alive, so the new file cannot reuse its number.
	require.NoError(t, fs.Rename(dir+"/a.txt.1", dir+"/moved"))
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt.1", []byte("other file"), 0o644))

	man.Purge()

	content, err := afero.ReadFile(fs, dir+"/a.txt.1")
	require.NoError(t, err)
	require.Equal(t, "other file", string(content))
	require.Len(t, man.backups, 1)
	require.Contains(t, logBuf.String(), "not purging backup")
}

// Expectation: Purge should keep the backup when no repaired file exists.
func Test_backupManager_Purge_NoReplacement_Kept(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))

	man, logBuf := bmNewManager(t, fs, dir, "a.txt")
	require.NoError(t, fs.Rename(dir+"/a.txt", dir+"/a.txt.1"))
	man.FindBackups()

	man.Purge()

	backupExists, _ := afero.Exists(fs, dir+"/a.txt.1")
	require.True(t, backupExists)
	require.Contains(t, logBuf.String(), "No valid repaired file (not purging backup)")
}

// Expectation: Purge should keep the backup when the repaired file is empty.
func Test_backupManager_Purge_EmptyReplacement_Kept(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))

	man, _ := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "")
	man.FindBackups()

	man.Purge()

	backupExists, _ := afero.Exists(fs, dir+"/a.txt.1")
	require.True(t, backupExists)
}

// Expectation: Purge should keep the pair when removing the backup fails.
func Test_backupManager_Purge_RemoveError_Kept(t *testing.T) {
	t.Parallel()

	fs := &testutil.FailingRemoveFs{Fs: afero.NewOsFs(), FailSuffix: ".1"}
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))

	man, logBuf := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "repaired")
	man.FindBackups()

	man.Purge()

	backupExists, _ := afero.Exists(fs, dir+"/a.txt.1")
	require.True(t, backupExists)
	require.Len(t, man.backups, 1)
	require.Contains(t, logBuf.String(), "Failed to purge backup file")
}

// Expectation: Restore should move the backup back over the original path.
func Test_backupManager_Restore_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))
	ino := bmStat(t, dir+"/a.txt").Ino

	man, _ := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "corrupt")
	man.FindBackups()

	man.Restore()

	content, err := afero.ReadFile(fs, dir+"/a.txt")
	require.NoError(t, err)
	require.Equal(t, "original", string(content))
	require.Equal(t, ino, bmStat(t, dir+"/a.txt").Ino)

	backupExists, _ := afero.Exists(fs, dir+"/a.txt.1")
	require.False(t, backupExists)
	require.Empty(t, man.backups)
}

// Expectation: Restore should restore the backup when par2 did not write a replacement.
func Test_backupManager_Restore_NoReplacement_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))

	man, _ := bmNewManager(t, fs, dir, "a.txt")
	require.NoError(t, fs.Rename(dir+"/a.txt", dir+"/a.txt.1"))
	man.FindBackups()

	man.Restore()

	content, err := afero.ReadFile(fs, dir+"/a.txt")
	require.NoError(t, err)
	require.Equal(t, "original", string(content))
}

// Expectation: Restore should not move a backup that was replaced since FindBackups.
func Test_backupManager_Restore_BackupChanged_NotRestored(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))

	man, logBuf := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "corrupt")
	man.FindBackups()

	// Keep the original inode alive, so the new file cannot reuse its number.
	require.NoError(t, fs.Rename(dir+"/a.txt.1", dir+"/moved"))
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt.1", []byte("other file"), 0o644))

	man.Restore()

	content, err := afero.ReadFile(fs, dir+"/a.txt")
	require.NoError(t, err)
	require.Equal(t, "corrupt", string(content))

	backupContent, err := afero.ReadFile(fs, dir+"/a.txt.1")
	require.NoError(t, err)
	require.Equal(t, "other file", string(backupContent))
	require.Contains(t, logBuf.String(), "not restoring backup")
}

// Expectation: Restore should keep the pair when renaming the backup fails.
func Test_backupManager_Restore_RenameError_Kept(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(baseFs, dir+"/a.txt", []byte("original"), 0o644))

	fs := &testutil.FailingRenameFs{Fs: baseFs, FailPattern: ".1"}
	man, logBuf := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, baseFs, dir+"/a.txt", dir+"/a.txt.1", "corrupt")
	man.FindBackups()

	man.Restore()

	backupExists, _ := afero.Exists(fs, dir+"/a.txt.1")
	require.True(t, backupExists)
	require.Len(t, man.backups, 1)
	require.Contains(t, logBuf.String(), "Failed to restore backup file")
}

// Expectation: RestoreAttrs should apply the pre-repair mode to the repaired file.
func Test_backupManager_RestoreAttrs_Mode_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o600))
	require.NoError(t, fs.Chmod(dir+"/a.txt", 0o600))

	man, _ := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "repaired")
	man.FindBackups()

	man.RestoreAttrs()

	info, err := fs.Stat(dir + "/a.txt")
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// Expectation: RestoreAttrs should apply the pre-repair modification time to the repaired file.
func Test_backupManager_RestoreAttrs_ModTime_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	mtime := time.Date(2020, 1, 2, 3, 4, 5, 6000, time.UTC)
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))
	require.NoError(t, fs.Chtimes(dir+"/a.txt", mtime, mtime))

	man, _ := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "repaired")
	man.FindBackups()

	man.RestoreAttrs()

	info, err := fs.Stat(dir + "/a.txt")
	require.NoError(t, err)
	require.True(t, mtime.Equal(info.ModTime()), "want %v, got %v", mtime, info.ModTime())
}

// Expectation: RestoreAttrs should restore ownership and then mode (incl. setuid) when running as root.
func Test_backupManager_RestoreAttrs_Ownership_Success(t *testing.T) {
	t.Parallel()

	if os.Geteuid() != 0 {
		t.Skip("restoring ownership requires root")
	}

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))
	require.NoError(t, fs.Chown(dir+"/a.txt", 65534, 65534))
	require.NoError(t, fs.Chmod(dir+"/a.txt", os.ModeSetuid|0o750))

	man, _ := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "repaired")
	man.FindBackups()

	man.RestoreAttrs()

	st := bmStat(t, dir+"/a.txt")
	require.Equal(t, uint32(65534), st.Uid)
	require.Equal(t, uint32(65534), st.Gid)

	// The mode is applied after chown(2), so the setuid bit survives.
	info, err := fs.Stat(dir + "/a.txt")
	require.NoError(t, err)
	require.Equal(t, os.ModeSetuid|0o750, info.Mode()&(os.ModeSetuid|os.ModePerm))
}

// Expectation: RestoreAttrs should not restore ownership or mode when not running as root.
func Test_backupManager_RestoreAttrs_OwnershipNonRoot_DifferentOwner_Success(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("ownership is always restored when running as root")
	}

	fs := afero.NewOsFs()
	dir := t.TempDir()
	mtime := time.Date(2020, 1, 2, 3, 4, 5, 6000, time.UTC)
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o600))
	require.NoError(t, fs.Chmod(dir+"/a.txt", 0o600))
	require.NoError(t, fs.Chtimes(dir+"/a.txt", mtime, mtime))

	man, logBuf := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "repaired")
	man.FindBackups()

	// Pretend the original was owned by someone else, which a non-root
	// test cannot set up for real (and which it could not restore either).
	man.snapshot[dir+"/a.txt"].Uid++

	man.RestoreAttrs()

	require.Equal(t, uint32(os.Geteuid()), bmStat(t, dir+"/a.txt").Uid) //nolint:gosec

	// The mode stays at the repaired file's default, as the original
	// permission bits were set for a different owner.
	info, err := fs.Stat(dir + "/a.txt")
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), info.Mode().Perm())

	// Times do not depend on ownership and are still restored.
	require.True(t, mtime.Equal(info.ModTime()), "want %v, got %v", mtime, info.ModTime())

	require.Contains(t, logBuf.String(), "Failed to restore ownership of repaired file")
	require.Contains(t, logBuf.String(), "Not restoring mode of repaired file (ownership differs)")
}

// Expectation: RestoreAttrs should not restore the mode when the group differs.
func Test_backupManager_RestoreAttrs_OwnershipNonRoot_DifferentGroup_Success(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("ownership is always restored when running as root")
	}

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o640))
	require.NoError(t, fs.Chmod(dir+"/a.txt", 0o640))

	man, logBuf := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "repaired")
	man.FindBackups()

	// Pretend the original belonged to another group (same owner).
	man.snapshot[dir+"/a.txt"].Gid = bmForeignGID(t)

	man.RestoreAttrs()

	info, err := fs.Stat(dir + "/a.txt")
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), info.Mode().Perm())
	require.Contains(t, logBuf.String(), "Failed to restore ownership of repaired file")
	require.Contains(t, logBuf.String(), "Not restoring mode of repaired file (ownership differs)")
}

// Expectation: RestoreAttrs should restore the mode (incl. setuid) when not running as root but ownership matches.
func Test_backupManager_RestoreAttrs_OwnershipNonRoot_SameOwnerRestoresMode_Success(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("tests the non-root path")
	}

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o600))
	require.NoError(t, fs.Chmod(dir+"/a.txt", os.ModeSetuid|0o700))

	man, logBuf := bmNewManager(t, fs, dir, "a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "repaired")
	man.FindBackups()

	man.RestoreAttrs()

	st := bmStat(t, dir+"/a.txt")
	require.Equal(t, uint32(os.Geteuid()), st.Uid) //nolint:gosec
	require.Equal(t, uint32(os.Getegid()), st.Gid) //nolint:gosec

	info, err := fs.Stat(dir + "/a.txt")
	require.NoError(t, err)
	require.Equal(t, os.ModeSetuid|0o700, info.Mode()&(os.ModeSetuid|os.ModePerm))

	require.NotContains(t, logBuf.String(), "Failed to restore ownership of repaired file")
	require.NotContains(t, logBuf.String(), "Not restoring mode of repaired file")
}

// Expectation: RestoreAttrs should not change files that par2 has not replaced.
func Test_backupManager_RestoreAttrs_UntouchedFile_Unchanged(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))
	require.NoError(t, afero.WriteFile(fs, dir+"/b.txt", []byte("untouched"), 0o644))

	man, _ := bmNewManager(t, fs, dir, "a.txt", "b.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "repaired")
	require.NoError(t, fs.Chmod(dir+"/b.txt", 0o640))
	man.FindBackups()

	man.RestoreAttrs()

	info, err := fs.Stat(dir + "/b.txt")
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o640), info.Mode().Perm())
}

// Expectation: RestoreAttrs should skip pairs without a repaired file.
func Test_backupManager_RestoreAttrs_NoReplacement_Skipped(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))

	man, logBuf := bmNewManager(t, fs, dir, "a.txt")
	require.NoError(t, fs.Rename(dir+"/a.txt", dir+"/a.txt.1"))
	man.FindBackups()

	man.RestoreAttrs()

	exists, _ := afero.Exists(fs, dir+"/a.txt")
	require.False(t, exists)
	require.Contains(t, logBuf.String(), "No valid repaired file (not restoring attributes)")
}

// Expectation: backupIntact should accept a backup that still is the original inode.
func Test_backupManager_backupIntact_SameInode_True(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt.1", []byte("backup"), 0o644))

	man := &backupManager{log: logging.NewLogger(logging.Options{Logout: io.Discard}), fsys: fs}
	pair := backupPair{originalPath: dir + "/a.txt", backupPath: dir + "/a.txt.1"}

	require.True(t, man.backupIntact(pair, bmStat(t, dir+"/a.txt.1")))
}

// Expectation: backupIntact should reject a backup that no longer exists.
func Test_backupManager_backupIntact_Missing_False(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt.1", []byte("backup"), 0o644))
	ss := bmStat(t, dir+"/a.txt.1")
	require.NoError(t, fs.Remove(dir+"/a.txt.1"))

	man := &backupManager{log: logging.NewLogger(logging.Options{Logout: io.Discard}), fsys: fs}
	pair := backupPair{originalPath: dir + "/a.txt", backupPath: dir + "/a.txt.1"}

	require.False(t, man.backupIntact(pair, ss))
}

// Expectation: backupIntact should reject a backup path that holds a different inode.
func Test_backupManager_backupIntact_DifferentInode_False(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt.1", []byte("backup"), 0o644))
	ss := bmStat(t, dir+"/a.txt.1")
	// Keep the original inode alive, so the new file cannot reuse its number.
	require.NoError(t, fs.Rename(dir+"/a.txt.1", dir+"/moved"))
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt.1", []byte("other"), 0o644))

	man := &backupManager{log: logging.NewLogger(logging.Options{Logout: io.Discard}), fsys: fs}
	pair := backupPair{originalPath: dir + "/a.txt", backupPath: dir + "/a.txt.1"}

	require.False(t, man.backupIntact(pair, ss))
}

// Expectation: checkReplacement should accept a new non-empty file at the original path.
func Test_backupManager_checkReplacement_NewFile_True(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))
	ss := bmStat(t, dir+"/a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "repaired")

	man := &backupManager{log: logging.NewLogger(logging.Options{Logout: io.Discard}), fsys: fs}
	pair := backupPair{originalPath: dir + "/a.txt", backupPath: dir + "/a.txt.1"}

	st, ok := man.checkReplacement(pair, ss)

	require.True(t, ok)
	require.Equal(t, bmStat(t, dir+"/a.txt").Ino, st.Ino)
}

// Expectation: checkReplacement should reject an original path that still holds the original inode.
func Test_backupManager_checkReplacement_SameInode_False(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))

	man := &backupManager{log: logging.NewLogger(logging.Options{Logout: io.Discard}), fsys: fs}
	pair := backupPair{originalPath: dir + "/a.txt", backupPath: dir + "/a.txt.1"}

	_, ok := man.checkReplacement(pair, bmStat(t, dir+"/a.txt"))

	require.False(t, ok)
}

// Expectation: checkReplacement should reject a missing original path.
func Test_backupManager_checkReplacement_Missing_False(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))
	ss := bmStat(t, dir+"/a.txt")
	require.NoError(t, fs.Rename(dir+"/a.txt", dir+"/a.txt.1"))

	man := &backupManager{log: logging.NewLogger(logging.Options{Logout: io.Discard}), fsys: fs}
	pair := backupPair{originalPath: dir + "/a.txt", backupPath: dir + "/a.txt.1"}

	_, ok := man.checkReplacement(pair, ss)

	require.False(t, ok)
}

// Expectation: checkReplacement should reject an empty file at the original path.
func Test_backupManager_checkReplacement_Empty_False(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))
	ss := bmStat(t, dir+"/a.txt")
	bmReplace(t, fs, dir+"/a.txt", dir+"/a.txt.1", "")

	man := &backupManager{log: logging.NewLogger(logging.Options{Logout: io.Discard}), fsys: fs}
	pair := backupPair{originalPath: dir + "/a.txt", backupPath: dir + "/a.txt.1"}

	_, ok := man.checkReplacement(pair, ss)

	require.False(t, ok)
}

// Expectation: checkReplacement should reject a directory at the original path.
func Test_backupManager_checkReplacement_Directory_False(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	dir := t.TempDir()
	require.NoError(t, afero.WriteFile(fs, dir+"/a.txt", []byte("original"), 0o644))
	ss := bmStat(t, dir+"/a.txt")
	require.NoError(t, fs.Rename(dir+"/a.txt", dir+"/a.txt.1"))
	require.NoError(t, fs.MkdirAll(dir+"/a.txt", 0o755))

	man := &backupManager{log: logging.NewLogger(logging.Options{Logout: io.Discard}), fsys: fs}
	pair := backupPair{originalPath: dir + "/a.txt", backupPath: dir + "/a.txt.1"}

	_, ok := man.checkReplacement(pair, ss)

	require.False(t, ok)
}

// Expectation: unixToFileMode should keep the permission bits.
func Test_unixToFileMode_Permissions_Success(t *testing.T) {
	t.Parallel()

	require.Equal(t, os.FileMode(0o644), unixToFileMode(0o644))
	require.Equal(t, os.FileMode(0o600), unixToFileMode(0o600))
}

// Expectation: unixToFileMode should drop the file type bits.
func Test_unixToFileMode_FileType_Dropped(t *testing.T) {
	t.Parallel()

	require.Equal(t, os.FileMode(0o644), unixToFileMode(syscall.S_IFREG|0o644))
}

// Expectation: unixToFileMode should convert the setuid bit.
func Test_unixToFileMode_Setuid_Success(t *testing.T) {
	t.Parallel()

	require.Equal(t, os.ModeSetuid|0o755, unixToFileMode(syscall.S_ISUID|0o755))
}

// Expectation: unixToFileMode should convert the setgid bit.
func Test_unixToFileMode_Setgid_Success(t *testing.T) {
	t.Parallel()

	require.Equal(t, os.ModeSetgid|0o755, unixToFileMode(syscall.S_ISGID|0o755))
}

// Expectation: unixToFileMode should convert the sticky bit.
func Test_unixToFileMode_Sticky_Success(t *testing.T) {
	t.Parallel()

	require.Equal(t, os.ModeSticky|0o755, unixToFileMode(syscall.S_ISVTX|0o755))
}

// Expectation: unixToFileMode should convert all special bits together.
func Test_unixToFileMode_AllSpecialBits_Success(t *testing.T) {
	t.Parallel()

	want := os.ModeSetuid | os.ModeSetgid | os.ModeSticky | 0o777
	require.Equal(t, want, unixToFileMode(syscall.S_ISUID|syscall.S_ISGID|syscall.S_ISVTX|0o777))
}
