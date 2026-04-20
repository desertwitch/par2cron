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

	"github.com/bmatcuk/doublestar/v4"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/afero"
)

func LstatIfPossible(fsys afero.Fs, name string) (fs.FileInfo, error) {
	if lstatter, ok := fsys.(afero.Lstater); ok {
		fi, lstat, err := lstatter.LstatIfPossible(name)

		if err != nil && lstat {
			return fi, fmt.Errorf("lstat: %w", err)
		}
		if err != nil && !lstat {
			return fi, fmt.Errorf("stat: %w", err)
		}

		return fi, nil
	}

	fi, err := fsys.Stat(name)
	if err != nil {
		return fi, fmt.Errorf("stat: %w", err)
	}

	return fi, nil
}

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

	err = syscall.Flock(int(f.Fd()), flags) //nolint:gosec
	if err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, schema.ErrFileIsLocked
		}

		return nil, fmt.Errorf("failed to flock: %w", err)
	}

	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:gosec
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

func WriteManifest(fsys afero.Fs, bundler schema.BundleHandler, path string, m *schema.Manifest, isBundle bool) error {
	// Update versions here, as we un- and re-marshalled to a possibly
	// new manifest format (adding new fields and dropping old fields).
	m.ProgramVersion = schema.ProgramVersion
	m.ManifestVersion = schema.ManifestVersion

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	if !isBundle {
		err = afero.WriteFile(fsys, path, data, UmaskFilePerm)
		if err != nil {
			return fmt.Errorf("failed to write: %w", err)
		}
	} else {
		b, err := bundler.Open(fsys, path)
		if err != nil {
			return fmt.Errorf("failed to open bundle: %w", err)
		}
		defer b.Close()

		if err := b.Update(data); err != nil {
			return fmt.Errorf("failed to update bundle: %w", err)
		}
	}

	return nil
}

var _ schema.FilesystemWalker = (*AferoWalker)(nil)

// AferoWalker is an adapter to turn the [afero.Walk] into a [filepath.WalkDir] signature.
type AferoWalker struct {
	Fs afero.Fs
}

func (w AferoWalker) Name() string { return "afero" }

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

var _ schema.FilesystemWalker = (*OSWalker)(nil)

// OSWalker is a wrapper structure for the native [filepath.WalkDir] function.
type OSWalker struct{}

func (w OSWalker) Name() string { return "os" }

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

func ShouldIgnorePath(fsys afero.Fs, path string, rootDir string) bool {
	dir := filepath.Dir(path)

	ignorePath := filepath.Join(dir, schema.IgnoreFile)
	if _, err := LstatIfPossible(fsys, ignorePath); err == nil {
		return true
	}

	for {
		ignoreAllPath := filepath.Join(dir, schema.IgnoreAllFile)
		if _, err := LstatIfPossible(fsys, ignoreAllPath); err == nil {
			return true
		}
		if dir == rootDir || dir == filepath.Dir(dir) {
			break
		}
		dir = filepath.Dir(dir)
	}

	return false
}

// Deprecated: IgnoreChecker is unused because it's much slower (more .Stat);
// keeping if needed later as it allows to [filepath.SkipDir] large subtrees.
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

		_, err := LstatIfPossible(c.fsys, ignorePath)
		c.hasIgnore = (err == nil)
		_, err = LstatIfPossible(c.fsys, ignoreAllPath)
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

func HasGlobSymlinks(fsys afero.Fs, workingDir string, pattern string) (string, bool) {
	patternPrefix, _ := doublestar.SplitPattern(pattern)

	patternPrefix = filepath.Clean(patternPrefix)
	workingDir = filepath.Clean(workingDir)

	dir := patternPrefix
	for dir != workingDir {
		fi, err := LstatIfPossible(fsys, dir)
		if err == nil {
			if fi.Mode()&fs.ModeSymlink != 0 {
				return dir, true
			}
		}

		if dir == filepath.Dir(dir) {
			break
		}
		dir = filepath.Dir(dir)
	}

	return "", false
}
