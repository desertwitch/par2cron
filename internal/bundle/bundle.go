package bundle

import (
	"errors"
)

// Magic identifies PAR2 packets.
var Magic = [8]byte{'P', 'A', 'R', '2', 0, 'P', 'K', 'T'}

var (
	PacketTypeIndex    = [16]byte{'P', '2', 'C', 'R', ' ', 'B', 'u', 'n', 'd', 'l', 'e', 'I', 'n', 'd', 'x'}
	PacketTypeFile     = [16]byte{'P', '2', 'C', 'R', ' ', 'B', 'u', 'n', 'd', 'l', 'e', 'F', 'i', 'l', 'e'}
	PacketTypeManifest = [16]byte{'P', '2', 'C', 'R', ' ', 'B', 'u', 'n', 'd', 'l', 'e', 'M', 'f', 's', 't'}
)

const (
	// Version is the current format version.
	Version uint64 = 1

	// commonHeaderSize is the fixed size of every packet header prefix.
	// magic(8) + packet_length(8) + packet_md5(16) + recovery_set_id(16) + packet_type(16) = 64.
	commonHeaderSize = 64

	// indexFixedSize is the fixed size of every index packet prefix.
	// version(8) + manifestPacketOffset(8) + manifestDataOffset(8) +
	// manifestDataLength(8) + manifestDataB3(32) + manifestNameLen(8) + entryCount(8) = 80.
	indexFixedSize = 80

	// IndexEntryFixedSize is the fixed size of every index packet entry prefix.
	// packetOffset(8) + dataOffset(8) + dataLength(8) + dataB3(32) + nameLen(8) = 64.
	indexEntryFixedSize = 64

	// FileBodyPrefixSize is the fixed size of every file packet body prefix.
	// dataLength(8) + dataB3(32) + nameLen(8) = 48.
	fileBodyPrefixSize = 48

	// ManifestBodyPrefixSize is the fixed size of every manifest packet body prefix.
	// dataLength(8) + dataB3(32) + nameLen(8) = 48.
	manifestBodyPrefixSize = 48
)

var (
	ErrInvalidMagic      = errors.New("invalid magic bytes")
	ErrInvalidChecksum   = errors.New("packet md5 mismatch")
	ErrDataCorrupt       = errors.New("data hash mismatch")
	ErrNotFound          = errors.New("entry not found")
	ErrUnknownPacketType = errors.New("unknown packet type")
)

// CommonHeader is the 64-byte PAR2 packet header.
type CommonHeader struct {
	Magic         [8]byte
	PacketLength  uint64
	PacketMD5     [16]byte // md5(recovery_set_id || packet_type || body)
	RecoverySetID [16]byte
	PacketType    [16]byte
}

// IndexEntry is one entry in the index packet's file table.
type IndexEntry struct {
	PacketOffset uint64
	DataOffset   uint64
	DataLength   uint64
	DataB3       [32]byte
	NameLen      uint64
	Name         string
}

// IndexPacket is the index at the start of the bundle.
type IndexPacket struct {
	CommonHeader

	Version uint64

	ManifestPacketOffset uint64
	ManifestDataOffset   uint64
	ManifestDataLength   uint64
	ManifestDataB3       [32]byte
	ManifestNameLen      uint64
	ManifestName         string

	EntryCount uint64
	Entries    []IndexEntry
}

// FilePacket wraps a single par2 file.
type FilePacket struct {
	CommonHeader

	DataLength uint64
	DataB3     [32]byte
	NameLen    uint64
	Name       string

	packetOffset uint64 // derived from packet position (not index)
	dataOffset   uint64 // derived from packet position (not index)
}

// ManifestPacket holds the JSON manifest.
type ManifestPacket struct {
	CommonHeader

	DataLength uint64
	DataB3     [32]byte
	NameLen    uint64
	Name       string

	packetOffset uint64 // derived from packet position (not index)
	dataOffset   uint64 // derived from packet position (not index)
}
