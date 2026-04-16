package bundle

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

var Magic = [8]byte{'P', 'A', 'R', '2', 0, 'P', 'K', 'T'}

var (
	PacketTypeIndex    = [16]byte{'P', '2', 'C', 'R', ' ', 'B', 'u', 'n', 'd', 'l', 'e', 'I', 'n', 'd', 'x'}
	PacketTypeFile     = [16]byte{'P', '2', 'C', 'R', ' ', 'B', 'u', 'n', 'd', 'l', 'e', 'F', 'i', 'l', 'e'}
	PacketTypeManifest = [16]byte{'P', '2', 'C', 'R', ' ', 'B', 'u', 'n', 'd', 'l', 'e', 'M', 'f', 's', 't'}
)

const (
	// Version is the current format version.
	Version uint64 = 1
)

var (
	ErrInvalidMagic      = errors.New("invalid magic bytes")
	ErrInvalidChecksum   = errors.New("packet md5 mismatch")
	ErrDataCorrupt       = errors.New("data hash mismatch")
	ErrNotFound          = errors.New("entry not found")
	ErrUnknownPacketType = errors.New("unknown packet type")
)

// Bundle is an opened bundle file with a parsed index packet.
type Bundle struct {
	f     afero.File // os.O_RDWR
	size  int64      // guaranteed > 0
	Index IndexPacket
}

// Open opens a bundle file and reads the index packet.
func Open(fsys afero.Fs, path string) (*Bundle, error) {
	f, err := fsys.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open: %w", err)
	}

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
	}
	if fi.Size() < 0 {
		return nil, fmt.Errorf("file size < 0: %d", fi.Size())
	}

	b := &Bundle{f: f, size: fi.Size()}
	if err := b.readIndexPacket(); err != nil {
		_ = f.Close()

		return nil, fmt.Errorf("failed to read index packet: %w", err)
	}

	return b, nil
}

// Manifest reads, verifies and returns the manifest bytes.
func (b *Bundle) Manifest() ([]byte, error) {
	var buf bytes.Buffer

	if err := b.ExtractManifest(&buf); err != nil {
		return nil, fmt.Errorf("failed to extract: %w", err)
	}

	return buf.Bytes(), nil
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

// Close closes the bundle file.
func (b *Bundle) Close() error {
	return b.f.Close() //nolint:wrapcheck
}
