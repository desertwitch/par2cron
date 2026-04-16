package bundle

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"slices"

	"github.com/spf13/afero"
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

// computeIndexSize returns the total size of the index packet.
func computeIndexSize(manifest ManifestInput, files []FileInput) uint64 {
	manifestNameLen := uint64(len(manifest.Name))
	size := uint64(commonHeaderSize) + indexFixedSize + padTo4(manifestNameLen)

	for _, fi := range files {
		size += indexEntryFixedSize + padTo4(uint64(len(fi.Name)))
	}

	return size
}

// Pack creates a bundle at outputPath from the given recovery set ID, files and manifest.
func Pack(fsys afero.Fs, outputPath string, recoverySetID [16]byte, manifest ManifestInput, files []FileInput) error {
	f, err := fsys.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create: %w", err)
	}
	defer f.Close()

	// Copy the slice, in case the caller didn't do it.
	manifest.Bytes = slices.Clone(manifest.Bytes)

	// Compute index packet size.
	indexSize := computeIndexSize(manifest, files)

	// Write a placeholder since we don't know the offsets yet.
	placeholder := make([]byte, indexSize)
	if err := writeAll(f, placeholder); err != nil {
		return fmt.Errorf("failed to write index packet placeholder: %w", err)
	}

	// Write the file packets followed by their respective raw byte stream.
	entries := make([]IndexEntry, len(files))
	for i, fi := range files {
		entry, err := writeFileSegment(fsys, f, recoverySetID, fi)
		if err != nil {
			return fmt.Errorf("failed to write file segment: %w", err)
		}
		entries[i] = entry
	}

	// Write the manifest at the end of the file.
	manifestPacketOffset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get manifest packet position: %w", err)
	}
	if manifestPacketOffset < 0 {
		return fmt.Errorf("failed to get valid manifest packet offset: %d", manifestPacketOffset)
	}
	manifestEntry, err := writeManifestPacket(f, recoverySetID, manifest, uint64(manifestPacketOffset))
	if err != nil {
		return fmt.Errorf("failed to write manifest packet: %w", err)
	}

	// Now seek back and write the actual index packet with the offsets.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to start: %w", err)
	}
	if err := writeIndexPacket(f, recoverySetID, entries, manifestEntry); err != nil {
		return fmt.Errorf("failed to write index packet: %w", err)
	}

	// Sync the changes.
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}

	return nil
}

// UpdateManifest replaces the manifest with a new one. It truncates the
// file at the manifest packet offset, writes a new manifest packet, then
// rewrites the index packet with the updated manifest fields.
func (b *Bundle) UpdateManifest(manifest []byte) error {
	manifestPacketOffset := b.Index.ManifestPacketOffset

	// Check first if there's really a manifest packet at that offset.
	ch, _, err := readAndValidatePacket(b.f, int64(manifestPacketOffset), b.size) //nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to validate manifest packet: %w", err)
	}
	if ch.PacketType != PacketTypeManifest {
		return fmt.Errorf("expected manifest packet at offset %d, got %q", manifestPacketOffset, ch.PacketType)
	}

	// Truncate the file to drop the old manifest packet.
	if err := b.f.Truncate(int64(manifestPacketOffset)); err != nil { //nolint:gosec
		return fmt.Errorf("failed to truncate manifest packet: %w", err)
	}

	// Seek to the manifest offset to place the new manifest packet.
	if _, err := b.f.Seek(int64(manifestPacketOffset), io.SeekStart); err != nil { //nolint:gosec
		return fmt.Errorf("failed to seek to manifest offset: %w", err)
	}

	// Write the new manifest packet at the requested offset.
	newManifest := ManifestInput{
		Name:  b.Index.ManifestName,
		Bytes: slices.Clone(manifest),
	}
	mf, err := writeManifestPacket(b.f, b.Index.RecoverySetID, newManifest, manifestPacketOffset)
	if err != nil {
		return fmt.Errorf("failed to write manifest packet: %w", err)
	}

	// Update the index packet with the new manifest's information.
	b.Index.ManifestPacketOffset = mf.packetOffset
	b.Index.ManifestDataOffset = mf.dataOffset
	b.Index.ManifestDataLength = mf.dataLength
	b.Index.ManifestDataB3 = mf.dataB3
	b.Index.ManifestNameLen = mf.nameLen
	b.Index.ManifestName = mf.name

	// Seek back to start of file to write the new index packet.
	if _, err := b.f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to start: %w", err)
	}

	// Write the new index packet at offset 0.
	if err := writeIndexPacket(b.f, b.Index.RecoverySetID, b.Index.Entries, mf); err != nil {
		return fmt.Errorf("failed to write index packet: %w", err)
	}

	// Sync the changes.
	if err := b.f.Sync(); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}

	// Update the file size.
	fi, err := b.f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat: %w", err)
	}
	if fi.Size() < 0 {
		return fmt.Errorf("file size < 0 after update: %d", fi.Size())
	}
	b.size = fi.Size()

	return nil
}

// manifestWriteEntry holds manifest info needed by the index packet writer.
type manifestWriteEntry struct {
	packetOffset uint64
	dataOffset   uint64
	dataLength   uint64
	dataB3       [32]byte
	nameLen      uint64
	name         string
}

// writeCommonPacket writes a full packet: header + body.
func writeCommonPacket(w io.Writer, recoverySetID [16]byte, packetType [16]byte, body []byte) error {
	packetLength := uint64(commonHeaderSize) + uint64(len(body))
	packetChecksum := packetMD5(recoverySetID, packetType, body)

	if err := writeAll(w, Magic[:]); err != nil {
		return fmt.Errorf("failed to write magic bytes: %w", err)
	}
	if err := writeUint64LE(w, packetLength); err != nil {
		return fmt.Errorf("failed to write packet length: %w", err)
	}
	if err := writeAll(w, packetChecksum[:]); err != nil {
		return fmt.Errorf("failed to write packet checksum: %w", err)
	}
	if err := writeAll(w, recoverySetID[:]); err != nil {
		return fmt.Errorf("failed to write recovery set ID: %w", err)
	}
	if err := writeAll(w, packetType[:]); err != nil {
		return fmt.Errorf("failed to write packet type: %w", err)
	}
	if err := writeAll(w, body); err != nil {
		return fmt.Errorf("failed to write packet body: %w", err)
	}

	return nil
}

// writeIndexPacket writes the bundle's index packet (header + manifest ref + entry table).
func writeIndexPacket(w io.Writer, recoverySetID [16]byte, entries []IndexEntry, mf manifestWriteEntry) error {
	var body bytes.Buffer

	if err := writeUint64LE(&body, Version); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}

	// Write manifest information.
	if err := writeUint64LE(&body, mf.packetOffset); err != nil {
		return fmt.Errorf("failed to write manifest packet offset: %w", err)
	}
	if err := writeUint64LE(&body, mf.dataOffset); err != nil {
		return fmt.Errorf("failed to write manifest data offset: %w", err)
	}
	if err := writeUint64LE(&body, mf.dataLength); err != nil {
		return fmt.Errorf("failed to write manifest data length: %w", err)
	}
	if err := writeAll(&body, mf.dataB3[:]); err != nil {
		return fmt.Errorf("failed to write manifest data hash: %w", err)
	}
	if err := writeUint64LE(&body, mf.nameLen); err != nil {
		return fmt.Errorf("failed to write manifest name length: %w", err)
	}

	// Write manifest file name.
	manifestNameBytes := make([]byte, padTo4(mf.nameLen))
	copy(manifestNameBytes, mf.name)
	if err := writeAll(&body, manifestNameBytes); err != nil {
		return fmt.Errorf("failed to write manifest name: %w", err)
	}

	// Write file entries.
	if err := writeUint64LE(&body, uint64(len(entries))); err != nil {
		return fmt.Errorf("failed to write entry count: %w", err)
	}
	for _, e := range entries {
		// Write file entry information.
		if err := writeUint64LE(&body, e.PacketOffset); err != nil {
			return fmt.Errorf("failed to write entry packet offset: %w", err)
		}
		if err := writeUint64LE(&body, e.DataOffset); err != nil {
			return fmt.Errorf("failed to write entry data offset: %w", err)
		}
		if err := writeUint64LE(&body, e.DataLength); err != nil {
			return fmt.Errorf("failed to write entry data length: %w", err)
		}
		if err := writeAll(&body, e.DataB3[:]); err != nil {
			return fmt.Errorf("failed to write entry data hash: %w", err)
		}
		if err := writeUint64LE(&body, e.NameLen); err != nil {
			return fmt.Errorf("failed to write entry name length: %w", err)
		}

		// Write file entry name.
		nameBytes := make([]byte, padTo4(e.NameLen))
		copy(nameBytes, e.Name)
		if err := writeAll(&body, nameBytes); err != nil {
			return fmt.Errorf("failed to write entry name: %w", err)
		}
	}

	return writeCommonPacket(w, recoverySetID, PacketTypeIndex, body.Bytes())
}

// writeFileSegment writes a file packet plus the file's data and padding (usually none).
func writeFileSegment(fsys afero.Fs, w io.WriteSeeker, recoverySetID [16]byte, fi FileInput) (IndexEntry, error) {
	src, err := fsys.Open(fi.Path)
	if err != nil {
		return IndexEntry{}, fmt.Errorf("failed to open: %w", err)
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return IndexEntry{}, fmt.Errorf("failed to stat: %w", err)
	}
	if info.Size() < 0 {
		return IndexEntry{}, fmt.Errorf("failed to get valid file size: %d", info.Size())
	}

	// Calculate offsets.
	dataLen := uint64(info.Size()) //nolint:gosec
	nameLen := uint64(len(fi.Name))

	filePacketOffset, err := w.Seek(0, io.SeekCurrent)
	if err != nil {
		return IndexEntry{}, fmt.Errorf("failed to get file packet position: %w", err)
	}
	if filePacketOffset < 0 {
		return IndexEntry{}, fmt.Errorf("failed to get valid file packet offset: %d", filePacketOffset)
	}

	// Hash data stream to calculate checksum.
	dataB3, err := dataHashReader(src)
	if err != nil {
		return IndexEntry{}, fmt.Errorf("failed to hash data: %w", err)
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return IndexEntry{}, fmt.Errorf("failed to seek to start: %w", err)
	}

	filePacketLength := uint64(commonHeaderSize) + fileBodyPrefixSize + padTo4(nameLen)
	dataOffset := uint64(filePacketOffset) + filePacketLength

	// Write the file packet first.
	if err := writeFilePacket(w, recoverySetID, fi.Name, dataLen, dataB3); err != nil {
		return IndexEntry{}, fmt.Errorf("failed to write file packet: %w", err)
	}

	// Now write the data stream after the file packet.
	written, err := io.Copy(w, src)
	if err != nil {
		return IndexEntry{}, fmt.Errorf("failed to write file stream: %w", err)
	}
	if uint64(written) != dataLen { //nolint:gosec
		return IndexEntry{}, io.ErrShortWrite
	}
	if err := writeDataPadding(w, dataLen); err != nil {
		return IndexEntry{}, fmt.Errorf("failed to write file stream padding: %w", err)
	}

	return IndexEntry{
		PacketOffset: uint64(filePacketOffset),
		DataOffset:   dataOffset,
		DataLength:   dataLen,
		DataB3:       dataB3,
		NameLen:      nameLen,
		Name:         fi.Name,
	}, nil
}

// writeFilePacket writes a file packet (header + body prefix + padded name).
func writeFilePacket(w io.Writer, recoverySetID [16]byte, name string, dataLen uint64, dataB3 [32]byte) error {
	var body bytes.Buffer

	nameLen := uint64(len(name))
	paddedNameLen := padTo4(nameLen)

	if err := writeUint64LE(&body, dataLen); err != nil {
		return fmt.Errorf("failed to write data length: %w", err)
	}
	if err := writeAll(&body, dataB3[:]); err != nil {
		return fmt.Errorf("failed to write data hash: %w", err)
	}
	if err := writeUint64LE(&body, nameLen); err != nil {
		return fmt.Errorf("failed to write name length: %w", err)
	}

	nameBytes := make([]byte, paddedNameLen)
	copy(nameBytes, name)
	if err := writeAll(&body, nameBytes); err != nil {
		return fmt.Errorf("failed to write name: %w", err)
	}

	return writeCommonPacket(w, recoverySetID, PacketTypeFile, body.Bytes())
}

// writeManifestPacket writes the manifest packet.
func writeManifestPacket(w io.Writer, recoverySetID [16]byte, manifest ManifestInput, packetOffset uint64) (manifestWriteEntry, error) {
	var body bytes.Buffer

	dataLen := uint64(len(manifest.Bytes))
	dataB3 := dataHash(manifest.Bytes)
	nameLen := uint64(len(manifest.Name))
	paddedNameLen := padTo4(nameLen)

	dataOffset := packetOffset + commonHeaderSize + manifestBodyPrefixSize + paddedNameLen

	if err := writeUint64LE(&body, dataLen); err != nil {
		return manifestWriteEntry{}, fmt.Errorf("failed to write data length: %w", err)
	}
	if err := writeAll(&body, dataB3[:]); err != nil {
		return manifestWriteEntry{}, fmt.Errorf("failed to write data hash: %w", err)
	}
	if err := writeUint64LE(&body, nameLen); err != nil {
		return manifestWriteEntry{}, fmt.Errorf("failed to write name length: %w", err)
	}

	nameBytes := make([]byte, paddedNameLen)
	copy(nameBytes, manifest.Name)
	if err := writeAll(&body, nameBytes); err != nil {
		return manifestWriteEntry{}, fmt.Errorf("failed to write name: %w", err)
	}

	if err := writeAll(&body, manifest.Bytes); err != nil {
		return manifestWriteEntry{}, fmt.Errorf("failed to write manifest bytes: %w", err)
	}
	if err := writeDataPadding(&body, dataLen); err != nil {
		return manifestWriteEntry{}, fmt.Errorf("failed to write manifest length: %w", err)
	}

	if err := writeCommonPacket(w, recoverySetID, PacketTypeManifest, body.Bytes()); err != nil {
		return manifestWriteEntry{}, fmt.Errorf("failed to write packet: %w", err)
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

// writeDataPadding writes zero bytes to w to pad dataLen up to the next 4-byte boundary.
func writeDataPadding(w io.Writer, dataLen uint64) error {
	padLen := padTo4(dataLen) - dataLen
	if padLen == 0 {
		return nil
	}

	var pad [3]byte // max padding for 4-byte alignment is 3
	_, err := w.Write(pad[:padLen&3])

	return err //nolint:wrapcheck
}

// writeUint64LE writes v to w as 8 bytes in little-endian order.
func writeUint64LE(w io.Writer, v uint64) error {
	var buf [8]byte

	binary.LittleEndian.PutUint64(buf[:], v)
	_, err := w.Write(buf[:])

	return err //nolint:wrapcheck
}

// writeAll writes all of p to w, looping until done or an error occurs.
func writeAll(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if err != nil {
			return err //nolint:wrapcheck
		}

		if n == 0 {
			return io.ErrShortWrite
		}
		p = p[n:]
	}

	return nil
}
