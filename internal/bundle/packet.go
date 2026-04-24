package bundle

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

const (
	// commonHeaderSize is the fixed size of every packet header prefix.
	// magic(8) + packet_length(8) + packet_md5(16) + recovery_set_id(16) + packet_type(16) = 64.
	commonHeaderSize = 64

	// indexFixedSize is the fixed size of every index packet prefix.
	// version(8) + flags(8) + manifestPacketOffset(8) + manifestDataOffset(8) +
	// manifestDataLength(8) + manifestDataSHA256(32) + manifestNameLen(8) + entryCount(8) = 88.
	indexFixedSize = 88

	// IndexEntryFixedSize is the fixed size of every index packet entry prefix.
	// packetOffset(8) + dataOffset(8) + dataLength(8) + dataSHA256(32) + nameLen(8) = 64.
	indexEntryFixedSize = 64

	// FileBodyPrefixSize is the fixed size of every file packet body prefix.
	// dataLength(8) + dataSHA256(32) + nameLen(8) = 48.
	fileBodyPrefixSize = 48

	// ManifestBodyPrefixSize is the fixed size of every manifest packet body prefix.
	// dataLength(8) + dataSHA256(32) + nameLen(8) = 48.
	manifestBodyPrefixSize = 48

	// maxPacketBodyBytes is the maximum allowed body length for any of our packets.
	// This would have to be a serious bug though, as we only store metadata/manifests.
	maxPacketBodyBytes uint64 = 128 * 1024 * 1024 // 128 MiB
)

// CommonHeader is the 64-byte PAR2 packet header.
type CommonHeader struct {
	Magic         [8]byte
	PacketLength  uint64
	PacketMD5     [16]byte // md5(recovery_set_id || packet_type || body)
	RecoverySetID [16]byte
	PacketType    [16]byte
}

// IndexPacket is the index at the start of the bundle.
type IndexPacket struct {
	CommonHeader

	Version uint64
	Flags   uint64

	ManifestPacketOffset uint64
	ManifestDataOffset   uint64
	ManifestDataLength   uint64
	ManifestDataSHA256   [32]byte
	ManifestNameLen      uint64
	ManifestName         string

	EntryCount uint64
	Entries    []IndexEntry
}

// IndexEntry is one file entry in the index packet's file table.
type IndexEntry struct {
	PacketOffset uint64
	DataOffset   uint64
	DataLength   uint64
	DataSHA256   [32]byte
	NameLen      uint64
	Name         string
}

// isKnownPacketType returns if the packet is of a par2cron-specific type.
func isKnownPacketType(t [16]byte) bool {
	switch t {
	case PacketTypeIndex, PacketTypeFile, PacketTypeManifest:
		return true
	default:
		return false
	}
}

// packetMD5 computes md5(recovery_set_id || packet_type || body).
func packetMD5(recoverySetID [16]byte, packetType [16]byte, body []byte) [16]byte {
	totalLen := uint64(len(recoverySetID)) + uint64(len(packetType)) + uint64(len(body))
	input := make([]byte, 0, totalLen)

	input = append(input, recoverySetID[:]...)
	input = append(input, packetType[:]...)
	input = append(input, body...)

	return md5.Sum(input)
}

// readAndValidatePacket reads the common packet header at the given offset and
// validates magic bytes and packet length alignment. Then it reads the rest of
// the packet, validates MD5 and returns packet header, packet body or an error.
func readAndValidatePacket(r io.ReaderAt, offset, fileSize int64, checkMD5 bool) (CommonHeader, []byte, error) {
	// Bounds check: can we fit a header in the remaining file?
	if offset < 0 || fileSize < commonHeaderSize || offset > fileSize-commonHeaderSize {
		return CommonHeader{}, nil, io.ErrUnexpectedEOF
	}

	// Read the header.
	var hdrBuf [commonHeaderSize]byte
	if _, err := r.ReadAt(hdrBuf[:], offset); err != nil {
		return CommonHeader{}, nil, fmt.Errorf("failed to read header: %w", err)
	}
	var ch CommonHeader
	if err := binary.Read(bytes.NewReader(hdrBuf[:]), binary.LittleEndian, &ch); err != nil {
		return CommonHeader{}, nil, fmt.Errorf("failed to parse header: %w", err)
	}

	// Validate the packet.
	if !isKnownPacketType(ch.PacketType) {
		return CommonHeader{}, nil, errors.New("unknown packet type")
	}
	if ch.Magic != Magic {
		return CommonHeader{}, nil, errors.New("invalid magic bytes")
	}
	if ch.PacketLength < commonHeaderSize {
		return CommonHeader{}, nil, fmt.Errorf("invalid packet length %d", ch.PacketLength)
	}
	if !isAligned4(ch.PacketLength) {
		return CommonHeader{}, nil, fmt.Errorf("packet length %d not 4-byte aligned", ch.PacketLength)
	}

	// Safe: offset <= fileSize - 64
	bodyOffset := offset + commonHeaderSize

	// Bounds check: can we fit the body in the remaining file?
	bodyLen := ch.PacketLength - commonHeaderSize
	if bodyLen > uint64(fileSize-bodyOffset) { //nolint:gosec
		return CommonHeader{}, nil, fmt.Errorf("body length %d exceeds file size", ch.PacketLength)
	}

	// Memory allocation check: is it a sane packet length?
	if bodyLen > maxPacketBodyBytes {
		return CommonHeader{}, nil, fmt.Errorf("packet body too large: %d > %d", bodyLen, maxPacketBodyBytes)
	}

	// Read the body at its offset.
	body := make([]byte, bodyLen)
	if _, err := r.ReadAt(body, bodyOffset); err != nil {
		return CommonHeader{}, nil, fmt.Errorf("failed to read body: %w", err)
	}

	if checkMD5 {
		// Verify the packet checksum.
		if packetMD5(ch.RecoverySetID, ch.PacketType, body) != ch.PacketMD5 {
			return CommonHeader{}, nil, errors.New("invalid packet checksum")
		}
	}

	return ch, body, nil
}

// parseIndexPacket parses the type-specific header bytes of an index packet.
func parseIndexPacket(r *bytes.Reader, ch CommonHeader) (IndexPacket, error) {
	var mp IndexPacket
	mp.CommonHeader = ch

	if err := safeReadU64(r, &mp.Version, math.MaxInt64); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read version: %w", err)
	}
	if err := safeReadU64(r, &mp.Flags, math.MaxInt64); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read flags: %w", err)
	}
	if err := safeReadU64(r, &mp.ManifestPacketOffset, math.MaxInt64); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read manifest packet offset: %w", err)
	}
	if err := safeReadU64(r, &mp.ManifestDataOffset, math.MaxInt64); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read manifest data offset: %w", err)
	}
	if err := safeReadU64(r, &mp.ManifestDataLength, math.MaxInt64); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read manifest data length: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &mp.ManifestDataSHA256); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read manifest data hash: %w", err)
	}

	// Read manifest name.
	if err := safeReadU64(r, &mp.ManifestNameLen, math.MaxUint16); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read manifest name length: %w", err)
	}
	manifestNameBuf := make([]byte, padTo4(mp.ManifestNameLen))
	if _, err := io.ReadFull(r, manifestNameBuf); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read manifest name: %w", err)
	}
	mp.ManifestName = string(manifestNameBuf[:mp.ManifestNameLen])

	// Read file entries.
	if err := safeReadU64(r, &mp.EntryCount, math.MaxUint16); err != nil {
		return IndexPacket{}, fmt.Errorf("failed to read entry count: %w", err)
	}

	mp.Entries = make([]IndexEntry, mp.EntryCount)
	for i := range mp.EntryCount {
		if err := safeReadU64(r, &mp.Entries[i].PacketOffset, math.MaxInt64); err != nil {
			return IndexPacket{}, fmt.Errorf("failed to read entry packet offset: %w", err)
		}
		if err := safeReadU64(r, &mp.Entries[i].DataOffset, math.MaxInt64); err != nil {
			return IndexPacket{}, fmt.Errorf("failed to read entry data offset: %w", err)
		}
		if err := safeReadU64(r, &mp.Entries[i].DataLength, math.MaxInt64); err != nil {
			return IndexPacket{}, fmt.Errorf("failed to read entry data length: %w", err)
		}
		if err := binary.Read(r, binary.LittleEndian, &mp.Entries[i].DataSHA256); err != nil {
			return IndexPacket{}, fmt.Errorf("failed to read entry data hash: %w", err)
		}
		if err := safeReadU64(r, &mp.Entries[i].NameLen, math.MaxUint16); err != nil {
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

// FilePacket is the metadata for a single bundled PAR2 file.
type FilePacket struct {
	CommonHeader

	DataLength uint64
	DataSHA256 [32]byte
	NameLen    uint64
	Name       string

	packetOffset uint64 // derived from packet position (not index)
	dataOffset   uint64 // derived from packet position (not index)
}

// parseFilePacket parses the type-specific header bytes of a file packet.
func parseFilePacket(r *bytes.Reader, ch CommonHeader, packetOffset int64) (FilePacket, error) {
	var fp FilePacket
	fp.CommonHeader = ch

	if packetOffset < 0 {
		return FilePacket{}, errors.New("negative packet offset")
	}

	if err := safeReadU64(r, &fp.DataLength, math.MaxInt64); err != nil {
		return FilePacket{}, fmt.Errorf("failed to read data length: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &fp.DataSHA256); err != nil {
		return FilePacket{}, fmt.Errorf("failed to read data hash: %w", err)
	}
	if err := safeReadU64(r, &fp.NameLen, math.MaxUint16); err != nil {
		return FilePacket{}, fmt.Errorf("failed to read name length: %w", err)
	}

	// Read file name.
	nameBuf := make([]byte, padTo4(fp.NameLen))
	if _, err := io.ReadFull(r, nameBuf); err != nil {
		return FilePacket{}, fmt.Errorf("failed to read name: %w", err)
	}
	fp.Name = string(nameBuf[:fp.NameLen])

	// Record offsets.
	fp.packetOffset = uint64(packetOffset)

	if fp.packetOffset > math.MaxUint64-ch.PacketLength {
		return FilePacket{}, errors.New("packet offset/length overflow")
	}
	fp.dataOffset = fp.packetOffset + ch.PacketLength

	if fp.dataOffset > math.MaxUint64-fp.DataLength {
		return FilePacket{}, errors.New("data offset/length overflow")
	}

	return fp, nil
}

// ManifestPacket is the metadata for the JSON manifest.
type ManifestPacket struct {
	CommonHeader

	DataLength uint64
	DataSHA256 [32]byte
	NameLen    uint64
	Name       string

	packetOffset uint64 // derived from packet position (not index)
	dataOffset   uint64 // derived from packet position (not index)
}

// parseManifestPacket parses the type-specific header bytes of a manifest packet.
func parseManifestPacket(r *bytes.Reader, ch CommonHeader, packetOffset int64) (ManifestPacket, error) {
	var mp ManifestPacket
	mp.CommonHeader = ch

	if packetOffset < 0 {
		return ManifestPacket{}, errors.New("negative packet offset")
	}

	startRemaining := r.Len()

	if err := safeReadU64(r, &mp.DataLength, math.MaxInt64); err != nil {
		return ManifestPacket{}, fmt.Errorf("failed to read data length: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &mp.DataSHA256); err != nil {
		return ManifestPacket{}, fmt.Errorf("failed to read data hash: %w", err)
	}
	if err := safeReadU64(r, &mp.NameLen, math.MaxUint16); err != nil {
		return ManifestPacket{}, fmt.Errorf("failed to read name length: %w", err)
	}

	// Read manifest file name.
	nameBuf := make([]byte, padTo4(mp.NameLen))
	if _, err := io.ReadFull(r, nameBuf); err != nil {
		return ManifestPacket{}, fmt.Errorf("failed to read name: %w", err)
	}
	mp.Name = string(nameBuf[:mp.NameLen])

	// Record offsets.
	endRemaining := r.Len()
	if startRemaining < 0 || endRemaining < 0 || startRemaining < endRemaining {
		return ManifestPacket{}, errors.New("invalid buffer state")
	}

	headerBytesInBody := uint64(startRemaining - endRemaining) //nolint:gosec

	mp.packetOffset = uint64(packetOffset)
	if mp.packetOffset > math.MaxUint64-ch.PacketLength {
		return ManifestPacket{}, errors.New("packet offset/length overflow")
	}

	body := mp.packetOffset + commonHeaderSize
	if body > math.MaxUint64-headerBytesInBody {
		return ManifestPacket{}, errors.New("data offset/length overflow")
	}
	mp.dataOffset = body + headerBytesInBody

	if mp.dataOffset > math.MaxUint64-mp.DataLength {
		return ManifestPacket{}, errors.New("data offset/length overflow")
	}
	if mp.dataOffset+mp.DataLength > mp.packetOffset+ch.PacketLength {
		return ManifestPacket{}, errors.New("data extends beyond packet")
	}

	return mp, nil
}
