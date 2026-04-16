package bundle

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/afero"
)

// Bundle is an opened bundle file with a parsed index packet.
type Bundle struct {
	fsys  afero.Fs
	f     afero.File
	Index IndexPacket
}

// Open opens a bundle file and reads the index packet.
func Open(fsys afero.Fs, path string) (*Bundle, error) {
	f, err := fsys.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open: %w", err)
	}

	b := &Bundle{fsys: fsys, f: f}
	if err := b.readIndexPacket(); err != nil {
		_ = f.Close()

		return nil, fmt.Errorf("failed to read index packet: %w", err)
	}

	return b, nil
}

// Unpack extracts all files and the manifest to destDir.
func (b *Bundle) Unpack(destDir string) error {
	for _, entry := range b.Index.Entries {
		if err := b.extractFile(entry.Name, destDir); err != nil {
			return fmt.Errorf("failed to extract file: %w", err)
		}
	}

	if err := b.extractManifest(destDir); err != nil {
		return fmt.Errorf("failed to extract manifest: %w", err)
	}

	return nil
}

// Close closes the bundle file.
func (b *Bundle) Close() error {
	return b.f.Close() //nolint:wrapcheck
}

// readAndValidatePacket reads the common packet header at the given offset and
// validates magic bytes and packet length alignment. Then it reads the rest of
// the packet, validates MD5 and returns packet header, packet body or an error.
func readAndValidatePacket(r io.ReadSeeker, offset int64) (CommonHeader, []byte, error) {
	if _, err := r.Seek(offset, io.SeekStart); err != nil {
		return CommonHeader{}, nil, fmt.Errorf("failed to seek to header: %w", err)
	}

	// Read the packet header.
	var ch CommonHeader
	if err := binary.Read(r, binary.LittleEndian, &ch); err != nil {
		return CommonHeader{}, nil, fmt.Errorf("failed to read common header: %w", err)
	}

	// Drop packets we do not know.
	if !isKnownPacketType(ch.PacketType) {
		return CommonHeader{}, nil, fmt.Errorf("%w: %x", ErrUnknownPacketType, ch.PacketType)
	}

	// Validate the packets we do know.
	if ch.Magic != Magic {
		return CommonHeader{}, nil, fmt.Errorf("invalid magic bytes: %w", ErrInvalidMagic)
	}
	if ch.PacketLength < commonHeaderSize {
		return CommonHeader{}, nil, fmt.Errorf("invalid packet length %d", ch.PacketLength)
	}
	if !isAligned4(ch.PacketLength) {
		return CommonHeader{}, nil, fmt.Errorf("packet length %d is not 4-byte aligned", ch.PacketLength)
	}

	// Validate the packet length (as it's not covered by MD5).
	cur, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return CommonHeader{}, nil, fmt.Errorf("failed to get current offset: %w", err)
	}
	end, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return CommonHeader{}, nil, fmt.Errorf("failed to seek to end: %w", err)
	}
	if _, err := r.Seek(cur, io.SeekStart); err != nil {
		return CommonHeader{}, nil, fmt.Errorf("failed to restore offset: %w", err)
	}
	if cur < 0 || end < 0 || end < cur {
		return CommonHeader{}, nil, errors.New("invalid stream state")
	}
	bodyLen := ch.PacketLength - commonHeaderSize
	if uint64(end-cur) < bodyLen { //nolint:gosec
		return CommonHeader{}, nil, fmt.Errorf("packet length %d exceeds available bytes", ch.PacketLength)
	}

	// Read the packet body.
	body := make([]byte, ch.PacketLength-commonHeaderSize)
	if _, err := io.ReadFull(r, body); err != nil {
		return CommonHeader{}, nil, fmt.Errorf("failed to read packet body: %w", err)
	}

	// Verify the packet MD5.
	if packetMD5(ch.RecoverySetID, ch.PacketType, body) != ch.PacketMD5 {
		return CommonHeader{}, nil, fmt.Errorf("packet checksum mismatch: %w", ErrInvalidChecksum)
	}

	return ch, body, nil
}

// readIndexPacket reads and validates the index packet at offset 0.
func (b *Bundle) readIndexPacket() error {
	ch, body, err := readAndValidatePacket(b.f, 0)
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

// findIndexEntry looks up a file entry in the index packet by name.
func (b *Bundle) findIndexEntry(name string) (*IndexEntry, error) {
	for i := range b.Index.Entries {
		if b.Index.Entries[i].Name == name {
			return &b.Index.Entries[i], nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
}

// readFile returns a SectionReader over the file's data in the bundle,
// along with the expected BLAKE3 hash for verification of the read data.
func (b *Bundle) readFile(name string) (*io.SectionReader, [32]byte, error) {
	entry, err := b.findIndexEntry(name)
	if err != nil {
		return nil, [32]byte{},
			fmt.Errorf("failed to find in index: %w", err)
	}

	sr := io.NewSectionReader(b.f, int64(entry.DataOffset), int64(entry.DataLength)) //nolint:gosec

	return sr, entry.DataB3, nil
}

// readManifest returns a SectionReader over the manifest data,
// along with the expected BLAKE3 hash for verification of the read data.
func (b *Bundle) readManifest() (*io.SectionReader, [32]byte) {
	sr := io.NewSectionReader(b.f, int64(b.Index.ManifestDataOffset), int64(b.Index.ManifestDataLength)) //nolint:gosec

	return sr, b.Index.ManifestDataB3
}

// Scan scans the bundle for app-specific packets by PAR2 magic, ignoring the
// index packet index. Use this as a fallback if the index packet is corrupt.
func Scan(fsys afero.Fs, path string) ([]FilePacket, *ManifestPacket, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat: %w", err)
	}
	fileSize := fi.Size()

	var files []FilePacket
	var manifest *ManifestPacket
	offset := int64(0)

	for offset+commonHeaderSize <= fileSize {
		ch, body, err := readAndValidatePacket(f, offset)
		if err != nil {
			offset++

			continue
		}

		switch ch.PacketType {
		case PacketTypeFile:
			fp, err := parseFilePacket(bytes.NewReader(body), ch, offset)
			if err != nil {
				offset++

				continue
			}
			files = append(files, fp)

		case PacketTypeManifest:
			mp, err := parseManifestPacket(bytes.NewReader(body), ch, offset)
			if err != nil {
				offset++

				continue
			}
			manifest = &mp
		}

		offset += int64(ch.PacketLength) //nolint:gosec
	}

	return files, manifest, nil
}
