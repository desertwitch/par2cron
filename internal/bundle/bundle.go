//go:generate go run ../../tool/generate-bundle
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

var ErrDataCorrupt = errors.New("data hash mismatch")

// Bundle is an opened bundle file with a parsed index packet.
type Bundle struct {
	f     afero.File // os.O_RDWR
	size  int64      // guaranteed > 0
	Index IndexPacket
}

// Open opens a bundle file and reads the index packet.
func Open(fsys afero.Fs, bundlePath string) (*Bundle, error) {
	f, err := fsys.OpenFile(bundlePath, os.O_RDWR, 0)
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

// Close closes the bundle file.
func (b *Bundle) Close() error {
	return b.f.Close() //nolint:wrapcheck
}

// Manifest reads, verifies and returns the manifest bytes.
func (b *Bundle) Manifest() ([]byte, error) {
	var buf bytes.Buffer

	if err := b.ExtractManifest(&buf); err != nil {
		return nil, fmt.Errorf("failed to extract: %w", err)
	}

	return buf.Bytes(), nil
}

// Validate checks that every packet referenced by the index is present and
// well-formed at the expected offset, and that the manifest data passes BLAKE3
// integrity checks. If strict is true, it additionally verifies that each file
// packet's data stream begins with a PAR2 magic byte sequence. This option
// should be used carefully though, as there may be non-compliant PAR2 writers.
func (b *Bundle) Validate(strict bool) error {
	// Validate manifest packet.
	ch, _, err := readAndValidatePacket(b.f, int64(b.Index.ManifestPacketOffset), b.size) //nolint:gosec
	if err != nil {
		return fmt.Errorf("manifest packet at offset %d: %w", b.Index.ManifestPacketOffset, err)
	}
	if ch.PacketType != PacketTypeManifest {
		return fmt.Errorf("manifest packet at offset %d: expected manifest type, got %q", b.Index.ManifestPacketOffset, ch.PacketType)
	}
	if b.Index.ManifestPacketOffset+ch.PacketLength != uint64(b.size) { //nolint:gosec
		return fmt.Errorf("manifest packet does not end at EOF: ends at %d, file size %d",
			b.Index.ManifestPacketOffset+ch.PacketLength, b.size)
	}

	// Validate manifest integrity (it's part of our packet).
	_, err = b.Manifest()
	if err != nil {
		return fmt.Errorf("manifest data: %w", err)
	}

	// Validate file packets.
	for i, entry := range b.Index.Entries {
		ch, _, err := readAndValidatePacket(b.f, int64(entry.PacketOffset), b.size) //nolint:gosec
		if err != nil {
			return fmt.Errorf("file packet %d (%q) at offset %d: %w", i, entry.Name, entry.PacketOffset, err)
		}
		if ch.PacketType != PacketTypeFile {
			return fmt.Errorf("file packet %d (%q) at offset %d: expected file type, got %q", i, entry.Name, entry.PacketOffset, ch.PacketType)
		}

		// Validate file stream starts on magic byte (it's not part of our packet).
		if strict {
			var magic [8]byte
			if _, err := b.f.ReadAt(magic[:], int64(entry.DataOffset)); err != nil { //nolint:gosec
				return fmt.Errorf("file packet %d (%q): failed to read data magic at offset %d: %w", i, entry.Name, entry.DataOffset, err)
			}
			if magic != Magic {
				return fmt.Errorf("file packet %d (%q): data at offset %d does not start with PAR2 magic", i, entry.Name, entry.DataOffset)
			}
		}
	}

	return nil
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
