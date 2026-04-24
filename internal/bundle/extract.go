package bundle

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// Unpack extracts all bundled files and the manifest to destDir. It attempts to
// extract every file rather than stopping at the first error, returning the
// paths that were successfully written alongside any errors. If strict is true,
// files that fail integrity checks are removed; otherwise they are kept on disk
// (returning ErrDataCorrupt).
func (b *Bundle) Unpack(fsys afero.Fs, destDir string, strict bool) ([]string, error) {
	var errs []error
	var extractedPaths []string

	for _, e := range b.Index.Entries {
		path, err := safePath(destDir, e.Name)
		if err != nil {
			errs = append(errs, err)

			continue
		}
		err = extractToFile(fsys, path, func(w io.Writer) error {
			return b.ExtractEntry(e, w)
		}, strict)
		if err != nil {
			errs = append(errs, fmt.Errorf("%q: %w", e.Name, err))
		}
		if err == nil || (!strict && errors.Is(err, ErrDataCorrupt)) {
			extractedPaths = append(extractedPaths, path)
		}
	}

	path, err := safePath(destDir, b.Index.ManifestName)
	if err != nil {
		errs = append(errs, err)
	} else {
		err := extractToFile(fsys, path,
			b.ExtractManifest, strict)
		if err != nil {
			errs = append(errs, fmt.Errorf("manifest: %w", err))
		}
		if err == nil || (!strict && errors.Is(err, ErrDataCorrupt)) {
			extractedPaths = append(extractedPaths, path)
		}
	}

	return extractedPaths, errors.Join(errs...)
}

// ExtractEntry copies a file packet's data stream to w and verifies it against
// its SHA256 hash. If an error is returned, the written data may be partial or
// corrupt. If the transfer is complete but corrupt, ErrDataCorrupt is returned.
func (b *Bundle) ExtractEntry(e IndexEntry, w io.Writer) error {
	sr := io.NewSectionReader(b.f, int64(e.DataOffset), int64(e.DataLength)) //nolint:gosec
	expectedHash := e.DataSHA256

	hash, err := dataHashReader(io.TeeReader(sr, w))
	if err != nil {
		return fmt.Errorf("failed to io: %w", err)
	}

	if hash != expectedHash {
		return fmt.Errorf("failed to validate: %w", ErrDataCorrupt)
	}

	return nil
}

// ExtractManifest copies the manifest's data stream to w and verifies it
// against its SHA256 hash. If an error is returned, written data may be partial
// or corrupt. If transfer is complete but corrupt, ErrDataCorrupt is returned.
func (b *Bundle) ExtractManifest(w io.Writer) error {
	sr := io.NewSectionReader(b.f, int64(b.Index.ManifestDataOffset), int64(b.Index.ManifestDataLength)) //nolint:gosec
	expectedHash := b.Index.ManifestDataSHA256

	hash, err := dataHashReader(io.TeeReader(sr, w))
	if err != nil {
		return fmt.Errorf("failed to io: %w", err)
	}

	if hash != expectedHash {
		return fmt.Errorf("failed to validate: %w", ErrDataCorrupt)
	}

	return nil
}

// extractToFile writes the output of extract to path. On I/O errors the file is
// always removed. On ErrDataCorrupt the file is kept on disk unless strict is true.
func extractToFile(fsys afero.Fs, path string, extract func(io.Writer) error, strict bool) error {
	out, err := fsys.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o666) //nolint:mnd
	if err != nil {
		return fmt.Errorf("failed to create: %w", err)
	}

	if err := extract(out); err != nil {
		_ = out.Close()

		if strict || !errors.Is(err, ErrDataCorrupt) {
			_ = fsys.Remove(path)
		}

		return fmt.Errorf("failed to extract: %w", err)
	}

	if err := out.Sync(); err != nil {
		_ = out.Close()
		_ = fsys.Remove(path)

		return fmt.Errorf("failed to sync: %w", err)
	}

	if err := out.Close(); err != nil {
		_ = fsys.Remove(path)

		return fmt.Errorf("failed to close: %w", err)
	}

	return nil
}

func safePath(destDir, name string) (string, error) {
	rel, err := filepath.Rel(destDir, filepath.Join(destDir, name))
	if err != nil {
		return "", fmt.Errorf("invalid path %q: %w", name, err)
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid path %q: escapes destination directory", name)
	}

	return filepath.Join(destDir, rel), nil
}
