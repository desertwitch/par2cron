package util

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/afero"
)

func AcquireLock(fsys afero.Fs, lockPath string, block bool) (func(), error) {
	if _, ok := fsys.(*afero.OsFs); !ok {
		return func() {}, nil
	}

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, UmaskFilePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to open: %w", err)
	}

	flags := syscall.LOCK_EX
	if !block {
		flags |= syscall.LOCK_NB
	}

	err = syscall.Flock(int(f.Fd()), flags)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, schema.ErrFileIsLocked
		}

		return nil, fmt.Errorf("failed to flock: %w", err)
	}

	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

func HashFile(fsys afero.Fs, filePath string) (string, error) {
	f, err := fsys.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to hash: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func WriteManifest(fsys afero.Fs, path string, m *schema.Manifest) error {
	m.ProgramVersion = schema.ProgramVersion
	m.ManifestVersion = schema.ManifestVersion

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	err = afero.WriteFile(fsys, path, data, UmaskFilePerm)
	if err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}

	return nil
}

// AferoWalker is an adapter to turn the [afero.Walk] into a [filepath.WalkDir] signature.
type AferoWalker struct {
	Fs afero.Fs
}

// WalkDir is a method that adapts [afero.Walk] into a [filepath.WalkDir] compatible signature.
func (w AferoWalker) WalkDir(root string, fn fs.WalkDirFunc) error {
	return afero.Walk(w.Fs, root, func(path string, info fs.FileInfo, err error) error { //nolint:wrapcheck
		var entry fs.DirEntry

		if info != nil {
			entry = fileInfoDirEntry{info}
		}

		return fn(path, entry, err)
	})
}

// OSWalker is a wrapper structure for the native [filepath.WalkDir] function.
type OSWalker struct{}

// WalkDir is a wrapper method for the native [filepath.WalkDir] function.
func (w OSWalker) WalkDir(root string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(root, fn) //nolint:wrapcheck
}

type fileInfoDirEntry struct {
	fs.FileInfo
}

func (fi fileInfoDirEntry) Type() fs.FileMode {
	return fi.Mode().Type()
}

func (fi fileInfoDirEntry) Info() (fs.FileInfo, error) {
	return fi.FileInfo, nil
}

func (fi fileInfoDirEntry) IsDir() bool {
	return fi.Mode().IsDir()
}

func (fi fileInfoDirEntry) Name() string {
	return fi.FileInfo.Name()
}

type IgnoreChecker struct {
	fsys afero.Fs

	lastVisited  string
	hasIgnore    bool
	hasIgnoreAll bool
}

func NewIgnoreChecker(fsys afero.Fs) *IgnoreChecker {
	return &IgnoreChecker{
		fsys: fsys,
	}
}

func (c *IgnoreChecker) ShouldSkip(path string, isDir bool) (bool, error) {
	if currentDir := filepath.Dir(path); currentDir != c.lastVisited {
		ignorePath := filepath.Join(currentDir, schema.IgnoreFile)
		ignoreAllPath := filepath.Join(currentDir, schema.IgnoreAllFile)

		_, err := c.fsys.Stat(ignorePath)
		c.hasIgnore = (err == nil)
		_, err = c.fsys.Stat(ignoreAllPath)
		c.hasIgnoreAll = (err == nil)

		c.lastVisited = currentDir
	}

	if isDir && c.hasIgnoreAll {
		return true, filepath.SkipDir
	} else if !isDir && (c.hasIgnore || c.hasIgnoreAll) {
		return true, nil
	}

	return false, nil
}
