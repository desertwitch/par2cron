//nolint:mnd
package bundle

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"slices"
)

// FileInput describes a file to pack into a bundle.
type FileInput struct {
	Name string // Filename stored in the bundle
	Path string // Path on disk to the file read from
}

// ManifestInput describes a manifest to pack into a bundle.
type ManifestInput struct {
	Name  string
	Bytes []byte
}

// computeMainSize returns the total size of the main packet.
func computeMainSize(manifest ManifestInput, files []FileInput) uint64 {
	// common(48) + version(8) + manifest_packet_offset(8) + manifest_data_offset(8) +
	// manifest_data_length(8) + manifest_data_b3(32) + manifest_name_len(8) +
	// manifest_name(padded) + entry_count(8).
	manifestNameLen := uint64(len(manifest.Name))
	size := uint64(120) + 8 + padTo4(manifestNameLen)

	for _, fi := range files {
		// packet_offset(8) + data_offset(8) + data_length(8) + data_b3(32) + name_len(8) + name(padded)
		size += 8 + 8 + 8 + 32 + 8 + padTo4(uint64(len(fi.Name)))
	}

	return size
}

// Pack creates a bundle at outputPath from the given files and manifest JSON.
// Layout: [main packet][file packets...][manifest packet].
func Pack(outputPath string, manifest ManifestInput, files []FileInput) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create bundle file: %w", err)
	}
	defer f.Close()

	// Just in case someone passes in without copying.
	manifest.Bytes = slices.Clone(manifest.Bytes)

	// Compute main packet size so we know where file packets start.
	mainSize := computeMainSize(manifest, files)

	// Write a zeroed placeholder for the main packet.
	placeholder := make([]byte, mainSize)
	if err := writeAll(f, placeholder); err != nil {
		return fmt.Errorf("failed to write main packet placeholder: %w", err)
	}

	// Write file packets, collecting entries and offets.
	entries := make([]MainEntry, len(files))
	for i, fi := range files {
		packetOffset, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return fmt.Errorf("failed to get file packet position: %w", err)
		}
		entry, err := writeFilePacket(f, fi, uint64(packetOffset))
		if err != nil {
			return fmt.Errorf("failed to write file packet: %w", err)
		}

		entries[i] = entry
	}

	// Write manifest packet, at the end of the file.
	manifestPacketOffset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get manifest packet position: %w", err)
	}
	manifestEntry, err := writeManifestPacket(f, manifest, uint64(manifestPacketOffset))
	if err != nil {
		return fmt.Errorf("failed to write manifest packet: %w", err)
	}

	// Seek back and write the real main packet (we now know the offsets).
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to start: %w", err)
	}
	if err := writeMainPacket(f, entries, manifestEntry); err != nil {
		return fmt.Errorf("failed to write main packet: %w", err)
	}

	return f.Sync()
}

// manifestWriteEntry holds manifest info needed by the main packet writer.
type manifestWriteEntry struct {
	packetOffset uint64
	dataOffset   uint64
	dataLength   uint64
	dataB3       [32]byte
	nameLen      uint64
	name         string
}

// UpdateManifest replaces the manifest with a new one. It truncates the
// file at the manifest packet offset, writes a new manifest packet, then
// rewrites the main packet header with the updated manifest's fields.
func (b *Bundle) UpdateManifest(manifest []byte) error {
	manifestPacketOffset := b.Main.ManifestPacketOffset

	// Verify that the manifest packet really sits at that offset.
	ch, _, err := readAndValidateHeader(b.f, int64(manifestPacketOffset))
	if err != nil {
		return fmt.Errorf("failed to validate manifest packet: %w", err)
	}
	if ch.PacketType != PacketTypeManifest {
		return fmt.Errorf("expected manifest packet at offset %d, got 0x%02x", manifestPacketOffset, ch.PacketType)
	}

	// Now remove the old manifest packet, we established it sits at that offset.
	if err := b.f.Truncate(int64(manifestPacketOffset)); err != nil {
		return fmt.Errorf("failed to truncate manifest packet: %w", err)
	}

	// Write the new manifest at the manifest offset.
	if _, err := b.f.Seek(int64(manifestPacketOffset), io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to manifest offset: %w", err)
	}

	newManifest := ManifestInput{
		Name:  b.Main.ManifestName,
		Bytes: slices.Clone(manifest),
	}

	mf, err := writeManifestPacket(b.f, newManifest, manifestPacketOffset)
	if err != nil {
		return fmt.Errorf("failed to write manifest packet: %w", err)
	}

	// Update and write the main packet at the begin of the file.
	b.Main.ManifestPacketOffset = mf.packetOffset
	b.Main.ManifestDataOffset = mf.dataOffset
	b.Main.ManifestDataLength = mf.dataLength
	b.Main.ManifestDataB3 = mf.dataB3
	b.Main.ManifestNameLen = mf.nameLen
	b.Main.ManifestName = mf.name

	if _, err := b.f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to start: %w", err)
	}

	mfEntry := manifestWriteEntry{
		packetOffset: b.Main.ManifestPacketOffset,
		dataOffset:   b.Main.ManifestDataOffset,
		dataLength:   b.Main.ManifestDataLength,
		dataB3:       b.Main.ManifestDataB3,
		nameLen:      b.Main.ManifestNameLen,
		name:         b.Main.ManifestName,
	}

	if err := writeMainPacket(b.f, b.Main.Entries, mfEntry); err != nil {
		return fmt.Errorf("failed to write main packet: %w", err)
	}

	return b.f.Sync()
}

// writeMainPacket writes the main packet at the current position.
func writeMainPacket(w io.Writer, entries []MainEntry, mf manifestWriteEntry) error {
	var typeSpecific bytes.Buffer

	// Construct type-specific parts first, so we know the size.
	if err := writeUint64LE(&typeSpecific, Version); err != nil {
		return err
	}
	if err := writeUint64LE(&typeSpecific, mf.packetOffset); err != nil {
		return err
	}
	if err := writeUint64LE(&typeSpecific, mf.dataOffset); err != nil {
		return err
	}
	if err := writeUint64LE(&typeSpecific, mf.dataLength); err != nil {
		return err
	}
	if err := writeAll(&typeSpecific, mf.dataB3[:]); err != nil {
		return err
	}
	if err := writeUint64LE(&typeSpecific, mf.nameLen); err != nil {
		return err
	}

	manifestNameBytes := make([]byte, padTo4(mf.nameLen))
	copy(manifestNameBytes, mf.name)
	if err := writeAll(&typeSpecific, manifestNameBytes); err != nil {
		return err
	}

	if err := writeUint64LE(&typeSpecific, uint64(len(entries))); err != nil {
		return err
	}
	for _, e := range entries {
		if err := writeUint64LE(&typeSpecific, e.PacketOffset); err != nil {
			return err
		}
		if err := writeUint64LE(&typeSpecific, e.DataOffset); err != nil {
			return err
		}
		if err := writeUint64LE(&typeSpecific, e.DataLength); err != nil {
			return err
		}
		if err := writeAll(&typeSpecific, e.DataB3[:]); err != nil {
			return err
		}
		if err := writeUint64LE(&typeSpecific, e.NameLen); err != nil {
			return err
		}

		nameBytes := make([]byte, padTo4(e.NameLen))
		copy(nameBytes, e.Name)
		if err := writeAll(&typeSpecific, nameBytes); err != nil {
			return err
		}
	}

	// Calculate the sizes and the validation checksum.
	typeSpecificBytes := typeSpecific.Bytes()
	packetLength := uint64(CommonHeaderSize) + uint64(len(typeSpecificBytes))
	headerLength := packetLength
	headerChecksum := headerMD5(PacketTypeMain, packetLength, headerLength, typeSpecificBytes)

	// Now write the common header and the pre-constructed type-specific parts.
	if err := writeAll(w, Magic[:]); err != nil {
		return err
	}
	if err := writeUint64LE(w, PacketTypeMain); err != nil {
		return err
	}
	if err := writeUint64LE(w, packetLength); err != nil {
		return err
	}
	if err := writeUint64LE(w, headerLength); err != nil {
		return err
	}
	if err := writeAll(w, headerChecksum[:]); err != nil {
		return err
	}

	return writeAll(w, typeSpecificBytes)
}

// writeFilePacket writes a file packet, streaming data from disk.
// Returns the MainEntry for the main packet's entry table.
func writeFilePacket(w io.WriteSeeker, fi FileInput, packetOffset uint64) (MainEntry, error) {
	src, err := os.Open(fi.Path)
	if err != nil {
		return MainEntry{}, fmt.Errorf("failed to open: %w", err)
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return MainEntry{}, fmt.Errorf("failed to stat: %w", err)
	}

	// Get the sizes to calculate the header fields.
	dataLen := uint64(info.Size())
	nameLen := uint64(len(fi.Name))
	paddedNameLen := padTo4(nameLen)

	// header_length = 48 (common) + 8 (data_length) + 32 (data_b3) + 8 (name_len) + padded_name.
	headerLength := uint64(CommonHeaderSize) + 8 + 32 + 8 + paddedNameLen
	packetLength := headerLength + padTo4(dataLen)
	dataOffset := packetOffset + headerLength

	// Hash the actual file data with a streamed approach.
	dataB3, err := dataHashReader(src)
	if err != nil {
		return MainEntry{}, fmt.Errorf("failed to hash data: %w", err)
	}

	// Seek source back to start for writing.
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return MainEntry{}, fmt.Errorf("failed to seek to start: %w", err)
	}

	// Build type-specific packet part for MD5.
	var typeSpecific bytes.Buffer
	if err := writeUint64LE(&typeSpecific, dataLen); err != nil {
		return MainEntry{}, err
	}
	if err := writeAll(&typeSpecific, dataB3[:]); err != nil {
		return MainEntry{}, err
	}
	if err := writeUint64LE(&typeSpecific, nameLen); err != nil {
		return MainEntry{}, err
	}
	nameBytes := make([]byte, paddedNameLen)
	copy(nameBytes, fi.Name)
	if err := writeAll(&typeSpecific, nameBytes); err != nil {
		return MainEntry{}, err
	}

	typeSpecificBytes := typeSpecific.Bytes()
	headerChecksum := headerMD5(PacketTypeFile, packetLength, headerLength, typeSpecificBytes)

	// Write common header.
	if err := writeAll(w, Magic[:]); err != nil {
		return MainEntry{}, err
	}
	if err := writeUint64LE(w, PacketTypeFile); err != nil {
		return MainEntry{}, err
	}
	if err := writeUint64LE(w, packetLength); err != nil {
		return MainEntry{}, err
	}
	if err := writeUint64LE(w, headerLength); err != nil {
		return MainEntry{}, err
	}
	if err := writeAll(w, headerChecksum[:]); err != nil {
		return MainEntry{}, err
	}

	// Write type-specific packet part.
	if err := writeAll(w, typeSpecificBytes); err != nil {
		return MainEntry{}, err
	}

	// Stream file data into bundle.
	if _, err := io.Copy(w, src); err != nil {
		return MainEntry{}, err
	}
	if err := writeDataPadding(w, dataLen); err != nil {
		return MainEntry{}, err
	}

	return MainEntry{
		PacketOffset: packetOffset,
		DataOffset:   dataOffset,
		DataLength:   dataLen,
		DataB3:       dataB3,
		NameLen:      nameLen,
		Name:         fi.Name,
	}, nil
}

// writeManifestPacket writes the manifest packet.
// Manifest data is small enough to hold in memory (it's JSON metadata).
func writeManifestPacket(w io.Writer, manifest ManifestInput, packetOffset uint64) (manifestWriteEntry, error) {
	dataLen := uint64(len(manifest.Bytes))
	dataB3 := dataHash(manifest.Bytes)
	nameLen := uint64(len(manifest.Name))
	paddedNameLen := padTo4(nameLen)

	// header_length = 48 (common) + 8 (data_length) + 32 (data_b3) + 8 (name_len) + padded_name.
	headerLength := uint64(CommonHeaderSize) + 8 + 32 + 8 + paddedNameLen
	packetLength := headerLength + padTo4(dataLen)
	dataOffset := packetOffset + headerLength

	// Build type-specific packet part for MD5.
	var typeSpecific bytes.Buffer
	if err := writeUint64LE(&typeSpecific, dataLen); err != nil {
		return manifestWriteEntry{}, err
	}
	if err := writeAll(&typeSpecific, dataB3[:]); err != nil {
		return manifestWriteEntry{}, err
	}
	if err := writeUint64LE(&typeSpecific, nameLen); err != nil {
		return manifestWriteEntry{}, err
	}
	nameBytes := make([]byte, paddedNameLen)
	copy(nameBytes, manifest.Name)
	if err := writeAll(&typeSpecific, nameBytes); err != nil {
		return manifestWriteEntry{}, err
	}

	typeSpecificBytes := typeSpecific.Bytes()
	headerChecksum := headerMD5(PacketTypeManifest, packetLength, headerLength, typeSpecificBytes)

	// Write common header.
	if err := writeAll(w, Magic[:]); err != nil {
		return manifestWriteEntry{}, err
	}
	if err := writeUint64LE(w, PacketTypeManifest); err != nil {
		return manifestWriteEntry{}, err
	}

	if err := writeUint64LE(w, packetLength); err != nil {
		return manifestWriteEntry{}, err
	}
	if err := writeUint64LE(w, headerLength); err != nil {
		return manifestWriteEntry{}, err
	}
	if err := writeAll(w, headerChecksum[:]); err != nil {
		return manifestWriteEntry{}, err
	}

	// Write type-specific packet part.
	if err := writeAll(w, typeSpecificBytes); err != nil {
		return manifestWriteEntry{}, err
	}

	// Write manifest.
	if err := writeAll(w, manifest.Bytes); err != nil {
		return manifestWriteEntry{}, err
	}
	if err := writeDataPadding(w, dataLen); err != nil {
		return manifestWriteEntry{}, err
	}

	return manifestWriteEntry{
		packetOffset: packetOffset,
		dataOffset:   dataOffset,
		dataLength:   dataLen,
		dataB3:       dataB3,
		nameLen:      nameLen,
		name:         manifest.Name,
	}, nil
}

func writeDataPadding(w io.Writer, dataLen uint64) error {
	padLen := padTo4(dataLen) - dataLen
	if padLen == 0 {
		return nil
	}

	err := writeAll(w, make([]byte, padLen))

	return err
}

func writeUint64LE(w io.Writer, v uint64) error {
	return binary.Write(w, binary.LittleEndian, v)
}

func writeAll(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		p = p[n:]
	}

	return nil
}
