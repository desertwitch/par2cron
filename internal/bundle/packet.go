package bundle

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// parseMainPacket parses the type-specific header bytes of a main packet.
func parseMainPacket(r *bytes.Reader, ch CommonHeader) (MainPacket, error) {
	var mp MainPacket
	mp.CommonHeader = ch

	if err := binary.Read(r, binary.LittleEndian, &mp.Version); err != nil {
		return MainPacket{}, err
	}
	if err := binary.Read(r, binary.LittleEndian, &mp.ManifestPacketOffset); err != nil {
		return MainPacket{}, err
	}
	if err := binary.Read(r, binary.LittleEndian, &mp.ManifestDataOffset); err != nil {
		return MainPacket{}, err
	}
	if err := binary.Read(r, binary.LittleEndian, &mp.ManifestDataLength); err != nil {
		return MainPacket{}, err
	}
	if err := binary.Read(r, binary.LittleEndian, &mp.ManifestDataB3); err != nil {
		return MainPacket{}, err
	}
	if err := binary.Read(r, binary.LittleEndian, &mp.ManifestNameLen); err != nil {
		return MainPacket{}, err
	}

	manifestNameBuf := make([]byte, padTo4(mp.ManifestNameLen))
	if _, err := io.ReadFull(r, manifestNameBuf); err != nil {
		return MainPacket{}, err
	}
	mp.ManifestName = string(manifestNameBuf[:mp.ManifestNameLen])

	if err := binary.Read(r, binary.LittleEndian, &mp.EntryCount); err != nil {
		return MainPacket{}, err
	}
	mp.Entries = make([]MainEntry, mp.EntryCount)
	for i := uint64(0); i < mp.EntryCount; i++ {
		if err := binary.Read(r, binary.LittleEndian, &mp.Entries[i].PacketOffset); err != nil {
			return MainPacket{}, err
		}
		if err := binary.Read(r, binary.LittleEndian, &mp.Entries[i].DataOffset); err != nil {
			return MainPacket{}, err
		}
		if err := binary.Read(r, binary.LittleEndian, &mp.Entries[i].DataLength); err != nil {
			return MainPacket{}, err
		}
		if err := binary.Read(r, binary.LittleEndian, &mp.Entries[i].DataB3); err != nil {
			return MainPacket{}, err
		}
		if err := binary.Read(r, binary.LittleEndian, &mp.Entries[i].NameLen); err != nil {
			return MainPacket{}, err
		}

		paddedLen := padTo4(mp.Entries[i].NameLen)
		nameBuf := make([]byte, paddedLen)
		if _, err := io.ReadFull(r, nameBuf); err != nil {
			return MainPacket{}, err
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
		return FilePacket{}, err
	}
	if err := binary.Read(r, binary.LittleEndian, &fp.DataB3); err != nil {
		return FilePacket{}, err
	}
	if err := binary.Read(r, binary.LittleEndian, &fp.NameLen); err != nil {
		return FilePacket{}, err
	}

	nameBuf := make([]byte, padTo4(fp.NameLen))
	if _, err := io.ReadFull(r, nameBuf); err != nil {
		return FilePacket{}, err
	}
	fp.Name = string(nameBuf[:fp.NameLen])

	fp.packetOffset = uint64(packetOffset)
	fp.dataOffset = uint64(packetOffset) + ch.HeaderLength
	if fp.dataOffset+fp.DataLength > fp.packetOffset+ch.PacketLength {
		return FilePacket{}, fmt.Errorf("bundle: invalid file packet data bounds")
	}

	return fp, nil
}

// parseManifestPacket parses the type-specific header bytes of a manifest packet.
func parseManifestPacket(r *bytes.Reader, ch CommonHeader, packetOffset int64) (ManifestPacket, error) {
	var mp ManifestPacket
	mp.CommonHeader = ch

	if err := binary.Read(r, binary.LittleEndian, &mp.DataLength); err != nil {
		return ManifestPacket{}, err
	}
	if err := binary.Read(r, binary.LittleEndian, &mp.DataB3); err != nil {
		return ManifestPacket{}, err
	}
	if err := binary.Read(r, binary.LittleEndian, &mp.NameLen); err != nil {
		return ManifestPacket{}, err
	}

	nameBuf := make([]byte, padTo4(mp.NameLen))
	if _, err := io.ReadFull(r, nameBuf); err != nil {
		return ManifestPacket{}, err
	}
	mp.Name = string(nameBuf[:mp.NameLen])

	mp.packetOffset = uint64(packetOffset)
	mp.dataOffset = uint64(packetOffset) + ch.HeaderLength
	if mp.dataOffset+mp.DataLength > mp.packetOffset+ch.PacketLength {
		return ManifestPacket{}, fmt.Errorf("bundle: invalid manifest packet data bounds")
	}

	return mp, nil
}
