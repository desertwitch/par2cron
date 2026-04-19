package bundle

import (
	"bytes"
	"fmt"
	"io"
	"slices"
	"strings"
)

// Scan scans the bundle for app-specific packets by PAR2 magic bytes, ignoring
// index packet metadata. Use this as fallback if the index packet is corrupt.
func Scan(r io.ReaderAt, size int64) ([]FilePacket, *ManifestPacket) {
	var files []FilePacket
	var manifest *ManifestPacket

	for offset := int64(0); offset+commonHeaderSize <= size; {
		ch, body, err := readAndValidatePacket(r, offset, size)
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
		found, err := findNextMagic(r, offset+1, size)
		if err != nil {
			break // No more magic sequences found.
		}

		offset = found
	}

	return files, manifest
}

// findNextMagic scans r for the next occurrence of magic bytes starting at
// from. Returns the offset of the match or an error if none is found.
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

// reconstructIndex builds an IndexPacket from scanned packets. The resulting
// packet is always equal or smaller than the original, since it can only
// contain fewer or equal entries (so is generally safe to put at offset 0).
func reconstructIndex(manifest *ManifestPacket, files []FilePacket) IndexPacket {
	slices.SortFunc(files, func(a, b FilePacket) int {
		return strings.Compare(a.Name, b.Name)
	})

	entries := make([]IndexEntry, len(files))
	for i, fp := range files {
		entries[i] = IndexEntry{
			PacketOffset: fp.packetOffset,
			DataOffset:   fp.dataOffset,
			DataLength:   fp.DataLength,
			DataB3:       fp.DataB3,
			NameLen:      fp.NameLen,
			Name:         fp.Name,
		}
	}

	return IndexPacket{
		CommonHeader: CommonHeader{
			RecoverySetID: manifest.RecoverySetID,
		},
		Version:              Version,
		Flags:                FlagIndexRebuilt,
		ManifestPacketOffset: manifest.packetOffset,
		ManifestDataOffset:   manifest.dataOffset,
		ManifestDataLength:   manifest.DataLength,
		ManifestDataB3:       manifest.DataB3,
		ManifestNameLen:      manifest.NameLen,
		ManifestName:         manifest.Name,
		EntryCount:           uint64(len(entries)),
		Entries:              entries,
	}
}
