package bundle

import (
	"bytes"
	"fmt"
	"io"

	"github.com/spf13/afero"
)

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
	if fi.Size() < 0 {
		return nil, nil, fmt.Errorf("file size < 0: %d", fi.Size())
	}
	fileSize := fi.Size()

	var files []FilePacket
	var manifest *ManifestPacket

	for offset := int64(0); offset+commonHeaderSize <= fileSize; {
		ch, body, err := readAndValidatePacket(f, offset, fileSize)
		if err == nil {
			switch ch.PacketType {
			case PacketTypeFile:
				if fp, err := parseFilePacket(bytes.NewReader(body), ch, offset); err == nil {
					files = append(files, fp)
					offset += int64(ch.PacketLength) //nolint:gosec

					continue
				}

			case PacketTypeManifest:
				if mp, err := parseManifestPacket(bytes.NewReader(body), ch, offset); err == nil {
					manifest = &mp
					offset += int64(ch.PacketLength) //nolint:gosec

					continue
				}
			}
		}

		// Invalid packet, scan forward for next magic sequence.
		found, err := findNextMagic(f, offset+1, fileSize)
		if err != nil {
			break // No more magic sequences found.
		}

		offset = found
	}

	return files, manifest, nil
}

// findNextMagic scans r for the next occurrence of Magic starting at from.
// Returns the offset of the match or an error if none is found.
func findNextMagic(r io.ReaderAt, from, fileSize int64) (int64, error) {
	const bufSize = 16 * 1024

	buf := make([]byte, bufSize)
	magicLen := int64(len(Magic))

	for off := from; off+magicLen <= fileSize; {
		// Read a chunk.
		readLen := min(int64(bufSize), fileSize-off)
		n, err := r.ReadAt(buf[:readLen], off)
		if n == 0 && err != nil {
			return 0, fmt.Errorf("failed to io: %w", err)
		}

		// Search for magic in the chunk.
		if idx := bytes.Index(buf[:n], Magic[:]); idx >= 0 {
			return off + int64(idx), nil
		}

		// Advance, but back up by len(Magic)-1 so we don't miss
		// a magic sequence that straddles the buffer boundary.
		off += int64(n) - magicLen + 1
	}

	return 0, io.EOF
}
