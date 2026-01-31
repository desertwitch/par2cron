package repair

import (
	"log/slog"
	"testing"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: A new backup purger should be created with the correct values and initialized state.
func Test_newBackupPurger_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}

	purger, err := newBackupPurger(fs, log, "/data")

	require.NoError(t, err)
	require.Equal(t, fs, purger.fsys)
	require.Equal(t, log, purger.log)
	require.Equal(t, "/data", purger.dir)
	require.NotNil(t, purger.before)
}

// Expectation: Constructor should capture the current state of numbered files.
func Test_newBackupPurger_CapturesState_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.1", []byte("backup1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.2", []byte("backup2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt", []byte("original"), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")

	require.NoError(t, err)
	require.Len(t, purger.before, 2)
	require.Contains(t, purger.before, "/data/file.txt.1")
	require.Contains(t, purger.before, "/data/file.txt.2")
}

// Expectation: Constructor should handle empty directory.
func Test_newBackupPurger_EmptyDir_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")

	require.NoError(t, err)
	require.Empty(t, purger.before)
}

// Expectation: Constructor should return error when directory doesn't exist.
func Test_newBackupPurger_MissingDir_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}

	_, err := newBackupPurger(fs, log, "/nonexistent")

	require.ErrorContains(t, err, "failed to establish before-state")
}

// Expectation: Constructor should return error when reading directory fails.
func Test_newBackupPurger_OpenError_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data", 0o755))

	fs := &testutil.FailingOpenFs{
		Fs:          baseFs,
		FailPattern: "/data",
	}

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	_, err := newBackupPurger(fs, log, "/data")

	require.ErrorContains(t, err, "failed to establish before-state")
}

// Expectation: Purge should remove new backup files with valid originals.
func Test_backupPurger_Purge_RemovesNewBackups_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt", []byte("original"), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	// Create new backup file after initialization
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.1", []byte("backup"), 0o644))

	require.NoError(t, purger.Purge())

	// Backup file should be removed
	exists, _ := afero.Exists(fs, "/data/file.txt.1")
	require.False(t, exists)

	// Original should remain
	exists, _ = afero.Exists(fs, "/data/file.txt")
	require.True(t, exists)
}

// Expectation: Purge should not remove backup files that existed at construction time.
func Test_backupPurger_Purge_KeepsOldBackups_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt", []byte("original"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.1", []byte("old backup"), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	require.NoError(t, purger.Purge())

	// Old backup should still exist
	exists, _ := afero.Exists(fs, "/data/file.txt.1")
	require.True(t, exists)
}

// Expectation: Purge should not remove backups without valid originals.
func Test_backupPurger_Purge_KeepsOrphans_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	// Create backup without original
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.1", []byte("orphan"), 0o644))

	require.NoError(t, purger.Purge())

	// Orphan backup should remain
	exists, _ := afero.Exists(fs, "/data/file.txt.1")
	require.True(t, exists)
}

// Expectation: Purge should not remove backups with empty original files.
func Test_backupPurger_Purge_KeepsEmptyOriginal_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt", []byte(""), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	// Create backup with empty original
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.1", []byte("backup"), 0o644))

	require.NoError(t, purger.Purge())

	// Backup should remain (original is empty)
	exists, _ := afero.Exists(fs, "/data/file.txt.1")
	require.True(t, exists)
}

// Expectation: Purge should handle multiple numbered extensions correctly.
func Test_backupPurger_Purge_MultipleExtensions_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt", []byte("original"), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	// Create multiple backup files
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.1", []byte("backup1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.2", []byte("backup2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.123", []byte("backup123"), 0o644))

	require.NoError(t, purger.Purge())

	// All backups should be removed
	exists, _ := afero.Exists(fs, "/data/file.txt.1")
	require.False(t, exists)
	exists, _ = afero.Exists(fs, "/data/file.txt.2")
	require.False(t, exists)
	exists, _ = afero.Exists(fs, "/data/file.txt.123")
	require.False(t, exists)
}

// Expectation: Purge should succeed when removing multiple valid backups.
func Test_backupPurger_Purge_MultipleValidBackups_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file1.txt", []byte("original1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file2.txt", []byte("original2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file3.txt", []byte("original3"), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	// Create multiple new backups
	require.NoError(t, afero.WriteFile(fs, "/data/file1.txt.1", []byte("backup1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file2.txt.5", []byte("backup2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file3.txt.999", []byte("backup3"), 0o644))

	require.NoError(t, purger.Purge())

	// All backups should be removed
	exists, _ := afero.Exists(fs, "/data/file1.txt.1")
	require.False(t, exists)
	exists, _ = afero.Exists(fs, "/data/file2.txt.5")
	require.False(t, exists)
	exists, _ = afero.Exists(fs, "/data/file3.txt.999")
	require.False(t, exists)

	// Originals should remain
	exists, _ = afero.Exists(fs, "/data/file1.txt")
	require.True(t, exists)
	exists, _ = afero.Exists(fs, "/data/file2.txt")
	require.True(t, exists)
	exists, _ = afero.Exists(fs, "/data/file3.txt")
	require.True(t, exists)
}

// Expectation: Purge should only process files in the current directory, not subdirectories.
func Test_backupPurger_Purge_IgnoresSubdirectories_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/subdir", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt", []byte("original"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/subdir/file.txt", []byte("original"), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	// Create backups in both current dir and subdirectory
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.1", []byte("backup"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/subdir/file.txt.1", []byte("backup"), 0o644))

	require.NoError(t, purger.Purge())

	// Current dir backup should be removed
	exists, _ := afero.Exists(fs, "/data/file.txt.1")
	require.False(t, exists)

	// Subdirectory backup should remain (not processed)
	exists, _ = afero.Exists(fs, "/data/subdir/file.txt.1")
	require.True(t, exists)
}

// Expectation: Purge should continue when file removal fails.
func Test_backupPurger_Purge_RemoveError_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/data/file.txt", []byte("original"), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(baseFs, log, "/data")
	require.NoError(t, err)

	// Wrap with failing fs after construction
	fs := &testutil.FailingRemoveFs{
		Fs:         baseFs,
		FailSuffix: ".1",
	}
	purger.fsys = fs

	// Create backup that will fail to remove
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.1", []byte("backup"), 0o644))

	err = purger.Purge()
	require.NoError(t, err)
}

// Expectation: Purge should handle multiple backups when some removals fail.
func Test_backupPurger_Purge_PartialRemoveError_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/data/file1.txt", []byte("original"), 0o644))
	require.NoError(t, afero.WriteFile(baseFs, "/data/file2.txt", []byte("original"), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(baseFs, log, "/data")
	require.NoError(t, err)

	// Wrap with failing fs after construction
	fs := &testutil.FailingRemoveFs{
		Fs:         baseFs,
		FailSuffix: "file1.txt.1",
	}
	purger.fsys = fs

	// Create backups - one will fail to remove
	require.NoError(t, afero.WriteFile(fs, "/data/file1.txt.1", []byte("backup"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file2.txt.1", []byte("backup"), 0o644))

	err = purger.Purge()
	require.NoError(t, err)

	exists, err := afero.Exists(fs, "/data/file2.txt.1")
	require.NoError(t, err)
	require.False(t, exists)
}

// Expectation: Purge should return error when after-state reading fails.
func Test_backupPurger_Purge_PostReadDirError_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	// Remove directory to cause read error
	require.NoError(t, fs.RemoveAll("/data"))

	err = purger.Purge()

	require.ErrorContains(t, err, "failed to establish after-state")
}

// Expectation: Purge should continue when an element has a stat failure.
func Test_backupPurger_Purge_StatError_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/data/file.txt", []byte("original"), 0o644))
	require.NoError(t, afero.WriteFile(baseFs, "/data/file2.txt", []byte("original"), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(baseFs, log, "/data")
	require.NoError(t, err)

	// Wrap with failing fs after construction
	fs := &testutil.FailingStatFs{
		Fs:          baseFs,
		FailPattern: "file.txt",
	}
	purger.fsys = fs

	// Create backup
	require.NoError(t, afero.WriteFile(baseFs, "/data/file.txt.1", []byte("backup"), 0o644))
	require.NoError(t, afero.WriteFile(baseFs, "/data/file2.txt.1", []byte("backup"), 0o644))

	err = purger.Purge()
	require.NoError(t, err)

	// Backup 1 should not be removed (stat failure)
	exists, _ := afero.Exists(baseFs, "/data/file.txt.1")
	require.True(t, exists)

	// Backup 2 should be removed (no stat failure)
	exists, _ = afero.Exists(baseFs, "/data/file2.txt.1")
	require.False(t, exists)
}

// Expectation: getNumberExtensions should only match files ending with numeric extensions.
func Test_backupPurger_getNumberExtensions_Pattern_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.1", []byte("match"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.999", []byte("match"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt", []byte("no match"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.bak", []byte("no match"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file.1.txt", []byte("no match"), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	files, err := purger.getNumberExtensions()

	require.NoError(t, err)
	require.Len(t, files, 2)
	require.Contains(t, files, "/data/file.txt.1")
	require.Contains(t, files, "/data/file.txt.999")
}

// Expectation: getNumberExtensions should skip directories.
func Test_backupPurger_getNumberExtensions_SkipsDirs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/subdir.1", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.1", []byte("file"), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	files, err := purger.getNumberExtensions()

	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Contains(t, files, "/data/file.txt.1")
}

// Expectation: getNumberExtensions should only process current directory, not subdirectories.
func Test_backupPurger_getNumberExtensions_CurrentDirOnly_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/subdir", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.1", []byte("current"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/subdir/file.txt.1", []byte("sub"), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	files, err := purger.getNumberExtensions()

	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Contains(t, files, "/data/file.txt.1")
	require.NotContains(t, files, "/data/subdir/file.txt.1")
}

// Expectation: getNumberExtensions should match multi-level numeric extensions like .1.1 or .2.2.2.
func Test_backupPurger_getNumberExtensions_MultiLevelNumeric_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.1", []byte("match"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.1.1", []byte("no match - ends in .1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.2.2.2", []byte("no match - ends in .2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/archive.tar.gz.5", []byte("match"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt", []byte("no match"), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	files, err := purger.getNumberExtensions()

	require.NoError(t, err)
	require.Len(t, files, 4)
	require.Contains(t, files, "/data/file.txt.1")
	require.Contains(t, files, "/data/file.txt.1.1")
	require.Contains(t, files, "/data/file.txt.2.2.2")
	require.Contains(t, files, "/data/archive.tar.gz.5")
}

// Expectation: hasValidOriginal should return true for existing non-empty original.
func Test_backupPurger_hasValidOriginal_Valid_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt", []byte("content"), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	valid, err := purger.hasValidOriginal("/data/file.txt.1")

	require.NoError(t, err)
	require.True(t, valid)
}

// Expectation: hasValidOriginal should return false for empty original.
func Test_backupPurger_hasValidOriginal_EmptyOriginal_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt", []byte(""), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	valid, err := purger.hasValidOriginal("/data/file.txt.1")

	require.NoError(t, err)
	require.False(t, valid)
}

// Expectation: hasValidOriginal should return false for missing original.
func Test_backupPurger_hasValidOriginal_MissingOriginal_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	valid, err := purger.hasValidOriginal("/data/file.txt.1")

	require.NoError(t, err)
	require.False(t, valid)
}

// Expectation: hasValidOriginal should return false for files without numeric extension.
func Test_backupPurger_hasValidOriginal_NoNumericExt_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	valid, err := purger.hasValidOriginal("/data/file.txt")

	require.NoError(t, err)
	require.False(t, valid)
}

// Expectation: hasValidOriginal should handle multiple numeric extensions correctly.
func Test_backupPurger_hasValidOriginal_MultipleExtensions_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt", []byte("content"), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	// Test single digit
	valid, err := purger.hasValidOriginal("/data/file.txt.1")
	require.NoError(t, err)
	require.True(t, valid)

	// Test multiple digits
	valid, err = purger.hasValidOriginal("/data/file.txt.12345")
	require.NoError(t, err)
	require.True(t, valid)
}

// Expectation: hasValidOriginal should strip only the final numeric extension from multi-level files.
func Test_backupPurger_hasValidOriginal_MultiLevelNumeric_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.1", []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.1.1", []byte("content"), 0o644))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(fs, log, "/data")
	require.NoError(t, err)

	// For file.txt.1.1, original should be file.txt.1
	valid, err := purger.hasValidOriginal("/data/file.txt.1.1")
	require.NoError(t, err)
	require.True(t, valid)

	// For file.txt.2.2.2, original should be file.txt.2.2
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt.2.2", []byte("content"), 0o644))
	valid, err = purger.hasValidOriginal("/data/file.txt.2.2.2")
	require.NoError(t, err)
	require.True(t, valid)
}

// Expectation: hasValidOriginal should return error when stat fails.
func Test_backupPurger_hasValidOriginal_StatError_Error(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data", 0o755))

	log := &logging.Logger{Logger: slog.New(slog.DiscardHandler), Options: logging.Options{}}
	purger, err := newBackupPurger(baseFs, log, "/data")
	require.NoError(t, err)

	// Wrap with failing fs after construction
	fs := &testutil.FailingStatFs{
		Fs:          baseFs,
		FailPattern: "file.txt",
	}
	purger.fsys = fs

	_, err = purger.hasValidOriginal("/data/file.txt.1")

	require.ErrorContains(t, err, "failed to stat")
}
