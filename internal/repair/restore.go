package repair

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"syscall"

	"github.com/spf13/afero"
)

var errNoSyscallStatT = errors.New("no syscall.Stat_t")

type fileRecord struct {
	path string
	size int64
}

type backupRestorer struct {
	fsys afero.Fs
	log  *slog.Logger

	dir    string
	before map[uint64]fileRecord // map[inode]fileRecord
}

func newBackupRestorer(fsys afero.Fs, log *slog.Logger, dir string) (*backupRestorer, error) {
	r := &backupRestorer{
		fsys: fsys,
		log:  log,
		dir:  dir,
	}

	files, err := r.getFilesWithInodes()
	if err != nil {
		return nil, fmt.Errorf("failed to establish before-state: %w", err)
	}
	r.before = files

	return r, nil
}

func (r *backupRestorer) Restore() error {
	after, err := r.getNumberedFilesWithInodes()
	if err != nil {
		return fmt.Errorf("failed to establish after-state: %w", err)
	}

	for inode, currentRecord := range after {
		beforeRecord, existed := r.before[inode]
		if !existed {
			// The inode did not exist before (it's not our backup file).
			continue
		}

		if currentRecord.path == beforeRecord.path {
			// It's an unchanged older backup file (it was there before).
			continue
		}

		if currentRecord.size != beforeRecord.size {
			r.log.Warn("Backup mismatches before-size (not restoring backup)",
				"path", currentRecord.path,
				"beforeSize", beforeRecord.size,
				"afterSize", currentRecord.size)

			continue
		}

		if err := r.fsys.Rename(currentRecord.path, beforeRecord.path); err != nil {
			r.log.Warn("Failed to restore backup file (needs manual restoration)",
				"path", currentRecord.path,
				"error", err)

			continue
		}

		r.log.Debug("Restored pre-repair state from backup file",
			"path", beforeRecord.path)
	}

	return nil
}

func (r *backupRestorer) getFilesWithInodes() (map[uint64]fileRecord, error) {
	files := make(map[uint64]fileRecord)

	entries, err := afero.ReadDir(r.fsys, r.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(r.dir, entry.Name())
		inode, err := r.getInode(path)
		if err != nil {
			r.log.Debug("Failed to get inode", "path", path, "error", err)

			continue
		}

		files[inode] = fileRecord{
			path: path,
			size: entry.Size(),
		}
	}

	return files, nil
}

func (r *backupRestorer) getNumberedFilesWithInodes() (map[uint64]fileRecord, error) {
	files := make(map[uint64]fileRecord)

	entries, err := afero.ReadDir(r.fsys, r.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !numberedFilePattern.MatchString(entry.Name()) {
			continue
		}

		path := filepath.Join(r.dir, entry.Name())
		inode, err := r.getInode(path)
		if err != nil {
			r.log.Debug("Failed to get inode", "path", path, "error", err)

			continue
		}

		files[inode] = fileRecord{
			path: path,
			size: entry.Size(),
		}
	}

	return files, nil
}

func (r *backupRestorer) getInode(path string) (uint64, error) {
	info, err := r.fsys.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("failed to stat: %w", err)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errNoSyscallStatT
	}

	return stat.Ino, nil
}
