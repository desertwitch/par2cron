package par2

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/afero"
)

var errUnsafePath = errors.New("unsafe path rejected")

// ParseFile parses a PAR2 file into a slice of []FileEntry.
func ParseFile(fsys afero.Fs, filename string) ([]FileEntry, error) {
	f, err := fsys.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open PAR2 file: %w", err)
	}
	defer f.Close()

	return Parse(f, true)
}

// ParseFileAsInfos parses a PAR2 file into a slice of []schema.FileInfo.
func ParseFileAsInfos(fsys afero.Fs, workingDir string, filename string) ([]schema.FileInfo, error) {
	f, err := fsys.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open PAR2 file: %w", err)
	}
	defer f.Close()

	e, err := Parse(f, true)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAR2 file: %w", err)
	}

	r, err := ToFileInfos(fsys, workingDir, e)
	if err != nil {
		return nil, fmt.Errorf("failed to convert PAR2 files: %w", err)
	}

	return r, nil
}

// ToFileInfos converts PAR2 file entries to a slice of [schema.FileInfo].
func ToFileInfos(fsys afero.Fs, baseDir string, entries []FileEntry) ([]schema.FileInfo, error) {
	files := make([]schema.FileInfo, 0, len(entries))

	for _, e := range entries {
		if filepath.IsAbs(e.Name) || strings.Contains(e.Name, "..") {
			return nil, fmt.Errorf("%w: %q", errUnsafePath, e.Name)
		}

		name := filepath.FromSlash(e.Name)
		path := filepath.Join(baseDir, name)

		rel, err := filepath.Rel(baseDir, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("%w: %q (escapes base)", errUnsafePath, e.Name)
		}

		pf := schema.FileInfo{
			Path: path,
			Name: name,
			Size: e.Size,
		}

		if fi, err := fsys.Stat(path); err == nil {
			pf.Mode = fi.Mode()
			pf.ModTime = fi.ModTime()
		}

		files = append(files, pf)
	}

	return files, nil
}
