package bundle

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/afero"
)

// Unpack extracts all files and the manifest to destDir on destFs.
func (b *Bundle) Unpack(destFs afero.Fs, destDir string) error {
	// Extract the file entries.
	for _, e := range b.Index.Entries {
		if err := writeToFile(destFs, filepath.Join(destDir, e.Name), func(w io.Writer) error {
			return b.ExtractEntry(e, w)
		}); err != nil {
			return fmt.Errorf("failed to extract %q: %w", e.Name, err)
		}
	}

	// Extract the manifest.
	if err := writeToFile(destFs, filepath.Join(destDir, b.Index.ManifestName), b.ExtractManifest); err != nil {
		return fmt.Errorf("failed to extract manifest: %w", err)
	}

	return nil
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
