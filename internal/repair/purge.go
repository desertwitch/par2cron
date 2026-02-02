package repair

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/spf13/afero"
)

var numberedFilePattern = regexp.MustCompile(`\.\d+$`)

type backupPurger struct {
	fsys afero.Fs
	log  *logging.Logger

	dir    string
	before map[string]struct{}
}

func newBackupPurger(fsys afero.Fs, log *logging.Logger, dir string) (*backupPurger, error) {
	p := &backupPurger{
		fsys: fsys,
		log:  log,
		dir:  dir,
	}

	files, err := p.getNumberExtensions()
	if err != nil {
		return nil, fmt.Errorf("failed to establish before-state: %w", err)
	}
	p.before = files

	return p, nil
}

func (p *backupPurger) Purge() error {
	after, err := p.getNumberExtensions()
	if err != nil {
		return fmt.Errorf("failed to establish after-state: %w", err)
	}

	for f := range after {
		if _, existed := p.before[f]; !existed {
			valid, err := p.hasValidOriginal(f)
			if err != nil {
				p.log.Warn("Failed to check for original file (not purging backup)",
					"path", f, "error", err)

				continue
			}
			if valid {
				if err := p.fsys.Remove(f); err != nil {
					p.log.Warn("Failed to purge backup file (needs manual removal)",
						"path", f, "error", err)

					continue
				}

				p.log.Debug("Purged backup file", "path", f)
			}
		}
	}

	return nil
}

func (p *backupPurger) getNumberExtensions() (map[string]struct{}, error) {
	files := make(map[string]struct{})

	entries, err := afero.ReadDir(p.fsys, p.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && numberedFilePattern.MatchString(entry.Name()) {
			files[filepath.Join(p.dir, entry.Name())] = struct{}{}
		}
	}

	return files, nil
}

func (p *backupPurger) hasValidOriginal(backupPath string) (bool, error) {
	originalPath := numberedFilePattern.ReplaceAllString(backupPath, "")

	if originalPath == backupPath {
		p.log.Warn("Same-path original and backup file (not purging backup)",
			"path", backupPath)

		return false, nil
	}

	info, err := p.fsys.Stat(originalPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			p.log.Warn("No original file found (not purging backup)",
				"path", backupPath)

			return false, nil
		}

		return false, fmt.Errorf("failed to stat: %w", err)
	}

	if info.Size() <= 0 {
		p.log.Warn("Invalid original file size (not purging backup)",
			"path", backupPath)

		return false, nil
	}

	return true, nil
}
