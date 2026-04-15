package bundle

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Bundle is an opened bundle file with a parsed main packet.
type Bundle struct {
	f    *os.File
	Main MainPacket
}

// Open opens a bundle file and reads the main packet.
func Open(path string) (*Bundle, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open: %w", err)
	}

	b := &Bundle{f: f}
	if err := b.readMainPacket(); err != nil {
		_ = f.Close()

		return nil, fmt.Errorf("failed to read main packet: %w", err)
	}

	return b, nil
}

// Unpack extracts all files and the manifest to destDir.
func (b *Bundle) Unpack(destDir string) error {
	for _, entry := range b.Main.Entries {
		if err := b.extractFile(entry.Name, destDir); err != nil {
			return err
		}
	}

	return b.extractManifest(destDir)
}

// Close closes the bundle file.
func (b *Bundle) Close() error {
	return b.f.Close()
}

// readAndValidateHeader reads the common header and its type-specific bytes
// at the given offset, validates alignment, bounds, and MD5 checksum.
func readAndValidateHeader(r io.ReadSeeker, offset int64) (CommonHeader, []byte, error) {
	if _, err := r.Seek(offset, io.SeekStart); err != nil {
		return CommonHeader{}, nil, fmt.Errorf("failed to seek to header: %w", err)
	}

	var ch CommonHeader
	if err := binary.Read(r, binary.LittleEndian, &ch); err != nil {
		return CommonHeader{}, nil, fmt.Errorf("failed to read common header: %w", err)
	}
	if ch.Magic != Magic {
		return CommonHeader{}, nil, fmt.Errorf("invalid magic bytes: %w", ErrInvalidMagic)
	}
	if ch.HeaderLength < CommonHeaderSize || ch.HeaderLength > ch.PacketLength {
		return CommonHeader{}, nil, fmt.Errorf("invalid header length %d", ch.HeaderLength)
	}
	if !isAligned4(ch.HeaderLength) {
		return CommonHeader{}, nil, fmt.Errorf("header length %d is not 4-byte aligned", ch.HeaderLength)
	}
	if !isAligned4(ch.PacketLength) {
		return CommonHeader{}, nil, fmt.Errorf("packet length %d is not 4-byte aligned", ch.PacketLength)
	}

	typeSpecific := make([]byte, ch.HeaderLength-CommonHeaderSize)
	if _, err := io.ReadFull(r, typeSpecific); err != nil {
		return CommonHeader{}, nil, fmt.Errorf("failed to read type-specific header: %w", err)
	}
	if headerMD5(ch.PacketType, ch.PacketLength, ch.HeaderLength, typeSpecific) != ch.HeaderMD5 {
		return CommonHeader{}, nil, fmt.Errorf("header checksum mismatch: %w", ErrInvalidChecksum)
	}

	return ch, typeSpecific, nil
}

// readMainPacket reads and validates the main packet at offset 0.
func (b *Bundle) readMainPacket() error {
	ch, typeSpecific, err := readAndValidateHeader(b.f, 0)
	if err != nil {
		return fmt.Errorf("failed to read packet: %w", err)
	}

	if ch.PacketType != PacketTypeMain {
		return fmt.Errorf("expected main packet type 0x01, got 0x%02x", ch.PacketType)
	}
	if ch.PacketLength != ch.HeaderLength {
		return fmt.Errorf("main packet length %d must equal header length %d", ch.PacketLength, ch.HeaderLength)
	}

	mp, err := parseMainPacket(bytes.NewReader(typeSpecific), ch)
	if err != nil {
		return fmt.Errorf("failed to parse packet: %w", err)
	}

	b.Main = mp

	return nil
}

// findEntry looks up a file entry in the main packet by name.
func (b *Bundle) findEntry(name string) (*MainEntry, error) {
	for i := range b.Main.Entries {
		if b.Main.Entries[i].Name == name {
			return &b.Main.Entries[i], nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
}

// readFile returns a SectionReader over the file's data in the bundle,
// along with the expected BLAKE3 hash for verification of the read data.
func (b *Bundle) readFile(name string) (*io.SectionReader, [32]byte, error) {
	entry, err := b.findEntry(name)
	if err != nil {
		return nil, [32]byte{},
			fmt.Errorf("failed to find file entry in main packet: %w", err)
	}

	sr := io.NewSectionReader(b.f, int64(entry.DataOffset), int64(entry.DataLength))

	return sr, entry.DataB3, nil
}

// readManifest returns a SectionReader over the manifest data,
// along with the expected BLAKE3 hash for verification of the read data.
func (b *Bundle) readManifest() (*io.SectionReader, [32]byte, error) {
	sr := io.NewSectionReader(b.f, int64(b.Main.ManifestDataOffset), int64(b.Main.ManifestDataLength))

	return sr, b.Main.ManifestDataB3, nil
}

// Scan scans the bundle for P2CR packets by magic byte, ignoring the main
// packet index. Use this as a fallback if the main packet is corrupt.
func Scan(path string) ([]FilePacket, *ManifestPacket, error) {
	f, err := os.Open(path)
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

	for offset+CommonHeaderSize <= fileSize {
		ch, typeSpecific, err := readAndValidateHeader(f, offset)
		if err != nil {
			offset++

			continue
		}

		// Skip over packets where the content is missing (truncated file)
		if offset+int64(ch.PacketLength) > fileSize {
			offset++

			continue
		}

		switch ch.PacketType {
		case PacketTypeFile:
			fp, err := parseFilePacket(bytes.NewReader(typeSpecific), ch, offset)
			if err != nil {
				offset++

				continue
			}
			files = append(files, fp)

		case PacketTypeManifest:
			mp, err := parseManifestPacket(bytes.NewReader(typeSpecific), ch, offset)
			if err != nil {
				offset++

				continue
			}
			manifest = &mp
		}

		offset += int64(ch.PacketLength)
	}

	return files, manifest, nil
}
