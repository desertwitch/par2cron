package repair

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/spf13/afero"
)

var backupPattern = regexp.MustCompile(`\.\d+$`)

type backupPair struct {
	originalPath string
	backupPath   string
}

type backupManager struct {
	log  *logging.Logger
	fsys afero.Fs

	backups  map[*syscall.Stat_t]backupPair
	snapshot map[string]*syscall.Stat_t
}

// newBackupManager returns a new [backupManager].
// It records the pre-repair state as part of the construction.
func newBackupManager(ctx context.Context, job *Job, fsys afero.Fs, par2er schema.Par2Handler, log *logging.Logger) (*backupManager, error) {
	p2, err := par2er.ParseFile(ctx, fsys, job.par2Path, true)
	if err != nil {
		return nil, fmt.Errorf("failed to parse par2: %w", err)
	}

	man := &backupManager{log: log.With("component", "backupManager"), fsys: fsys}
	man.backups = make(map[*syscall.Stat_t]backupPair)
	man.snapshot = make(map[string]*syscall.Stat_t)

	for _, set := range p2.Sets {
		for _, rset := range set.RecoverySet {
			path, err := util.SanitizePar2Path(rset.Name)
			if err == nil {
				path, err = util.JailedJoinPath(job.workingDir, path)
			}
			if err != nil {
				log.Debug("Skipping invalid PAR2-referenced filename", "name", rset.Name, "error", err)

				continue
			}

			info, err := util.LstatIfPossible(fsys, path)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					log.Debug("Skipping missing PAR2-referenced file", "name", rset.Name, "path", path)

					continue
				}

				log.Warn("Skipping non-accessible PAR2-referenced file", "name", rset.Name, "path", path, "error", err)

				continue
			}

			if !info.Mode().IsRegular() {
				log.Debug("Skipping non-regular PAR2-referenced file", "name", rset.Name, "path", path, "mode", info.Mode().String())

				continue
			}

			st, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				log.Debug("Skipping PAR2-referenced file without Stat_t", "name", rset.Name, "path", path)

				continue
			}

			man.snapshot[path] = st
		}
	}

	return man, nil
}

type fileID struct{ dev, ino uint64 }

// FindBackups stores in the [backupManager] the found backup files after a
// PAR2 repair operation has run (it should be called regardless of outcome).
func (man *backupManager) FindBackups() {
	index := make(map[string]map[fileID]string) // dir -> (dev,ino) -> numbered name
	man.backups = make(map[*syscall.Stat_t]backupPair)

	for path, ss := range man.snapshot {
		if info, err := util.LstatIfPossible(man.fsys, path); err == nil {
			if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Dev == ss.Dev && st.Ino == ss.Ino {
				continue // Par2 hasn't replaced the file at all, it's still the same inode.
			}
		}

		dir, origName := filepath.Split(path)

		entries, ok := index[dir]
		if !ok {
			entries = man.findNumbered(dir)
			index[dir] = entries
		}

		numberedName, ok := entries[fileID{ss.Dev, ss.Ino}]
		if !ok || !isBackupName(numberedName, origName) {
			man.log.Warn("Replaced file has no identifiable backup "+
				"(filesystem may not preserve inodes on rename)", "path", path)

			continue
		}

		backupPath := filepath.Join(dir, numberedName)
		man.backups[ss] = backupPair{originalPath: path, backupPath: backupPath}
	}
}

// findNumbered indexes a directory for files matching [backupPattern].
func (man *backupManager) findNumbered(dir string) map[fileID]string {
	found := make(map[fileID]string)

	infos, err := afero.ReadDir(man.fsys, dir)
	if err != nil {
		man.log.Warn("Failed to read directory for backup lookup", "path", dir, "error", err)

		return found
	}

	for _, info := range infos {
		if !info.Mode().IsRegular() || !backupPattern.MatchString(info.Name()) {
			continue
		}

		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			found[fileID{st.Dev, st.Ino}] = info.Name()
		}
	}

	return found
}

// isBackupName checks if numberedName is a valid backup name for origName.
func isBackupName(numberedName, origName string) bool {
	suffix, ok := strings.CutPrefix(numberedName, origName+".")
	if !ok || suffix == "" {
		return false
	}

	for _, r := range suffix {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// Purge removes the backups of successfully repaired files.
func (man *backupManager) Purge() {
	for ss, pair := range man.backups {
		if !man.backupIntact(pair, ss) {
			man.log.Warn("No valid backup file (not purging backup)", "path", pair.backupPath)

			continue
		}

		if _, ok := man.checkReplacement(pair, ss); !ok {
			man.log.Warn("No valid repaired file (not purging backup)", "path", pair.originalPath)

			continue
		}

		if err := man.fsys.Remove(pair.backupPath); err != nil {
			man.log.Warn("Failed to purge backup file (needs manual deletion)", "path", pair.backupPath, "error", err)

			continue
		}

		delete(man.backups, ss)
		man.log.Debug("Purged backup file", "path", pair.backupPath)
	}
}

// Restore moves the backups back over their original paths after a failed repair.
func (man *backupManager) Restore() {
	for ss, pair := range man.backups {
		if !man.backupIntact(pair, ss) {
			man.log.Warn("No valid backup file (not restoring backup)", "path", pair.backupPath)

			continue
		}

		if err := man.fsys.Rename(pair.backupPath, pair.originalPath); err != nil {
			man.log.Warn("Failed to restore backup file (needs manual restore)",
				"path", pair.originalPath, "backup", pair.backupPath, "error", err)

			continue
		}

		delete(man.backups, ss)
		man.log.Info("Restored backup file", "backup", pair.backupPath, "path", pair.originalPath)
	}
}

// RestoreAttrs applies the pre-repair attributes to the repaired files.
// Ownership is only restored when running as root; mode and times always.
func (man *backupManager) RestoreAttrs() {
	isRoot := os.Geteuid() == 0

	for ss, pair := range man.backups {
		st, ok := man.checkReplacement(pair, ss)
		if !ok {
			man.log.Warn("No valid repaired file (not restoring attributes)", "path", pair.originalPath)

			continue
		}

		path := pair.originalPath
		ownerMatches := st.Uid == ss.Uid && st.Gid == ss.Gid

		// Ownership first, as chown(2) clears setuid/setgid bits.
		if !ownerMatches {
			if isRoot {
				if err := man.fsys.Chown(path, int(ss.Uid), int(ss.Gid)); err != nil {
					man.log.Warn("Failed to restore ownership of repaired file", "path", path, "error", err)
				} else {
					ownerMatches = true
				}
			} else {
				man.log.Warn("Ownership of repaired file differs (cannot restore; not root)",
					"path", path, "wantUid", ss.Uid, "wantGid", ss.Gid, "haveUid", st.Uid, "haveGid", st.Gid)
			}
		}

		// Permission bits only make sense for the owner and group they were set for.
		if ownerMatches {
			if err := man.fsys.Chmod(path, unixToFileMode(ss.Mode)); err != nil {
				man.log.Warn("Failed to restore mode of repaired file", "path", path, "error", err)
			}
		} else {
			man.log.Warn("Not restoring mode of repaired file (ownership differs)", "path", path)
		}

		atime := time.Unix(ss.Atim.Unix())
		mtime := time.Unix(ss.Mtim.Unix())

		if err := man.fsys.Chtimes(path, atime, mtime); err != nil {
			man.log.Warn("Failed to restore times of repaired file", "path", path, "error", err)
		}

		man.log.Debug("Restored pre-repair attributes on repaired file", "path", path)
	}
}

// backupIntact re-checks that the backup still is the original inode.
// Other steps (e.g. the par2 re-verify) run between FindBackups() and
// the actions, so this guards against anything having changed in between.
func (man *backupManager) backupIntact(pair backupPair, ss *syscall.Stat_t) bool {
	info, err := util.LstatIfPossible(man.fsys, pair.backupPath)
	if err != nil {
		man.log.Warn("Failed to stat backup file", "path", pair.backupPath, "error", err)

		return false
	}

	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || st.Dev != ss.Dev || st.Ino != ss.Ino {
		man.log.Warn("Backup file changed since repair (not touching it)", "path", pair.backupPath)

		return false
	}

	return true
}

// checkReplacement returns the file par2 wrote at the original path, if it is
// a non-empty regular file that is not the original (backed-up) inode.
func (man *backupManager) checkReplacement(pair backupPair, ss *syscall.Stat_t) (*syscall.Stat_t, bool) {
	info, err := util.LstatIfPossible(man.fsys, pair.originalPath)
	if err != nil {
		man.log.Warn("Failed to stat repaired file", "path", pair.originalPath, "error", err)

		return nil, false
	}

	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Size() <= 0 || (st.Dev == ss.Dev && st.Ino == ss.Ino) {
		man.log.Warn("Unexpected repaired file", "path", pair.originalPath)

		return nil, false
	}

	return st, true
}

// unixToFileMode converts a [syscall.Stat_t.Mode] to a [fs.FileMode].
func unixToFileMode(m uint32) fs.FileMode {
	mode := fs.FileMode(m & 0o777) //nolint:mnd

	if m&syscall.S_ISUID != 0 {
		mode |= fs.ModeSetuid
	}
	if m&syscall.S_ISGID != 0 {
		mode |= fs.ModeSetgid
	}
	if m&syscall.S_ISVTX != 0 {
		mode |= fs.ModeSticky
	}

	return mode
}
