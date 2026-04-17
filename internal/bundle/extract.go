package bundle

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

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

// Unpack extracts all file entries and the manifest to destDir. It attempts
// to extract every entry rather than stopping at the first error. If strict
// is true, files that fail integrity checks are removed; otherwise they are
// kept on disk for potential reusing. Errors are joined in the return value.
func (b *Bundle) Unpack(fsys afero.Fs, destDir string, strict bool) error {
	var errs []error

	for _, e := range b.Index.Entries {
		if err := extractToFile(fsys, filepath.Join(destDir, e.Name), func(w io.Writer) error {
			return b.ExtractEntry(e, w)
		}, strict); err != nil {
			errs = append(errs, fmt.Errorf("%q: %w", e.Name, err))
		}
	}

	if err := extractToFile(fsys, filepath.Join(destDir, b.Index.ManifestName),
		b.ExtractManifest, strict); err != nil {
		errs = append(errs, fmt.Errorf("manifest: %w", err))
	}

	return errors.Join(errs...)
}

// ExtractEntry extracts a single entry from the bundle to a destination writer.
func (b *Bundle) ExtractEntry(e IndexEntry, w io.Writer) error {
	sr := io.NewSectionReader(b.f, int64(e.DataOffset), int64(e.DataLength)) //nolint:gosec
	expectedHash := e.DataB3

	hash, err := dataHashReader(io.TeeReader(sr, w))
	if err != nil {
		return fmt.Errorf("failed to io: %w", err)
	}

	if hash != expectedHash {
		return fmt.Errorf("failed to validate: %w", ErrDataCorrupt)
	}

	return nil
}

// ExtractManifest extracts the manifest from the bundle to a destination writer.
func (b *Bundle) ExtractManifest(w io.Writer) error {
	sr := io.NewSectionReader(b.f, int64(b.Index.ManifestDataOffset), int64(b.Index.ManifestDataLength)) //nolint:gosec
	expectedHash := b.Index.ManifestDataB3

	hash, err := dataHashReader(io.TeeReader(sr, w))
	if err != nil {
		return fmt.Errorf("failed to io: %w", err)
	}

	if hash != expectedHash {
		return fmt.Errorf("failed to validate: %w", ErrDataCorrupt)
	}

	return nil
}
