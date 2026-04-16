//go:generate go run ../../tool/generate-bundle
//nolint:funcorder
package bundle

import (
	"bytes"
	"errors"
	"fmt"
	"os"

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

// readIndexPacket reads and validates the index packet at offset 0.
func (b *Bundle) readIndexPacket() error {
	ch, body, err := readAndValidatePacket(b.f, 0, b.size)
	if err != nil {
		return fmt.Errorf("failed to read packet: %w", err)
	}

	if ch.PacketType != PacketTypeIndex {
		return fmt.Errorf("expected index packet at offset 0, got %q", ch.PacketType)
	}

	mp, err := parseIndexPacket(bytes.NewReader(body), ch)
	if err != nil {
		return fmt.Errorf("failed to parse packet: %w", err)
	}

	b.Index = mp

	return nil
}

// Manifest reads, verifies and returns the manifest bytes.
func (b *Bundle) Manifest() ([]byte, error) {
	var buf bytes.Buffer

	if err := b.ExtractManifest(&buf); err != nil {
		return nil, fmt.Errorf("failed to extract: %w", err)
	}

	return buf.Bytes(), nil
}

// Close closes the bundle file.
func (b *Bundle) Close() error {
	return b.f.Close() //nolint:wrapcheck
}
