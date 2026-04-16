package bundle

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// parseIndexPacket parses the type-specific header bytes of an index packet.
func parseIndexPacket(r *bytes.Reader, ch CommonHeader) (IndexPacket, error) {
	var mp IndexPacket
	mp.CommonHeader = ch

	if err := binary.Read(r, binary.LittleEndian, &mp.Version); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read version: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &mp.ManifestPacketOffset); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read manifest packet offset: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &mp.ManifestDataOffset); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read manifest data offset: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &mp.ManifestDataLength); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read manifest data length: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &mp.ManifestDataB3); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read manifest data hash: %w", err)
	}

	// Read manifest name.
	if err := binary.Read(r, binary.LittleEndian, &mp.ManifestNameLen); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read manifest name length: %w", err)
	}
	manifestNameBuf := make([]byte, padTo4(mp.ManifestNameLen))
	if _, err := io.ReadFull(r, manifestNameBuf); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read manifest name: %w", err)
	}
	mp.ManifestName = string(manifestNameBuf[:mp.ManifestNameLen])

	// Read file entries.
	if err := binary.Read(r, binary.LittleEndian, &mp.EntryCount); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read entry count: %w", err)
	}

	mp.Entries = make([]IndexEntry, mp.EntryCount)
	for i := range mp.EntryCount {
		if err := binary.Read(r, binary.LittleEndian, &mp.Entries[i].PacketOffset); err != nil {
			return IndexPacket{}, fmt.Errorf("failed to read entry packet offset: %w", err)
		}
		if err := binary.Read(r, binary.LittleEndian, &mp.Entries[i].DataOffset); err != nil {
			return IndexPacket{}, fmt.Errorf("failed to read entry data offset: %w", err)
		}
		if err := binary.Read(r, binary.LittleEndian, &mp.Entries[i].DataLength); err != nil {
			return IndexPacket{}, fmt.Errorf("failed to read entry data length: %w", err)
		}
		if err := binary.Read(r, binary.LittleEndian, &mp.Entries[i].DataB3); err != nil {
			return IndexPacket{}, fmt.Errorf("failed to read entry data hash: %w", err)
		}
		if err := binary.Read(r, binary.LittleEndian, &mp.Entries[i].NameLen); err != nil {
			return IndexPacket{}, fmt.Errorf("failed to read entry name length: %w", err)
		}

		// Read entry name (= file name).
		nameBuf := make([]byte, padTo4(mp.Entries[i].NameLen))
		if _, err := io.ReadFull(r, nameBuf); err != nil {
			return IndexPacket{}, fmt.Errorf("failed to read entry name: %w", err)
		}
		mp.Entries[i].Name = string(nameBuf[:mp.Entries[i].NameLen])
	}

	return mp, nil
}

// parseFilePacket parses the type-specific header bytes of a file packet.
func parseFilePacket(r *bytes.Reader, ch CommonHeader, packetOffset int64) (FilePacket, error) {
	var fp FilePacket
	fp.CommonHeader = ch

	if err := binary.Read(r, binary.LittleEndian, &fp.DataLength); err != nil {
		return FilePacket{}, fmt.Errorf("failed to read data length: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &fp.DataB3); err != nil {
		return FilePacket{}, fmt.Errorf("failed to read data hash: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &fp.NameLen); err != nil {
		return FilePacket{}, fmt.Errorf("failed to read name length: %w", err)
	}

	// Read file name.
	nameBuf := make([]byte, padTo4(fp.NameLen))
	if _, err := io.ReadFull(r, nameBuf); err != nil {
		return FilePacket{}, fmt.Errorf("failed to read name: %w", err)
	}
	fp.Name = string(nameBuf[:fp.NameLen])

	// Record offsets.
	fp.packetOffset = uint64(packetOffset) //nolint:gosec
	fp.dataOffset = fp.packetOffset + ch.PacketLength

	// Validate data length.
	if fp.dataOffset+fp.DataLength < fp.dataOffset {
		return FilePacket{}, errors.New("invalid packet data length")
	}

	return fp, nil
}

// parseManifestPacket parses the type-specific header bytes of a manifest packet.
func parseManifestPacket(r *bytes.Reader, ch CommonHeader, packetOffset int64) (ManifestPacket, error) {
	var mp ManifestPacket
	mp.CommonHeader = ch
	startLen := r.Len()

	if err := binary.Read(r, binary.LittleEndian, &mp.DataLength); err != nil {
		return ManifestPacket{}, fmt.Errorf("failed to read data length: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &mp.DataB3); err != nil {
		return ManifestPacket{}, fmt.Errorf("failed to read data hash: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &mp.NameLen); err != nil {
		return ManifestPacket{}, fmt.Errorf("failed to read name length: %w", err)
	}

	// Read manifest file name.
	nameBuf := make([]byte, padTo4(mp.NameLen))
	if _, err := io.ReadFull(r, nameBuf); err != nil {
		return ManifestPacket{}, fmt.Errorf("failed to read name: %w", err)
	}
	mp.Name = string(nameBuf[:mp.NameLen])

	// Just in case...
	endLen := r.Len()
	if startLen < 0 || endLen < 0 || startLen < endLen {
		return ManifestPacket{}, errors.New("invalid buffer state")
	}

	// Record offsets.
	headerBytesInBody := uint64(startLen - endLen)                              //nolint:gosec
	mp.dataOffset = uint64(packetOffset) + commonHeaderSize + headerBytesInBody //nolint:gosec
	mp.packetOffset = uint64(packetOffset)                                      //nolint:gosec

	// Validate data length.
	if mp.dataOffset+mp.DataLength > mp.packetOffset+ch.PacketLength ||
		mp.dataOffset+mp.DataLength < mp.packetOffset {
		return ManifestPacket{}, errors.New("invalid packet data length")
	}

	return mp, nil
}
