package bundle

import (
	"errors"
)

// Magic identifies P2CB packets.
var Magic = [8]byte{'P', '2', 'C', 'B', 0, 'P', 'K', 'T'}

const (
	PacketTypeMain     uint64 = 0x01
	PacketTypeFile     uint64 = 0x02
	PacketTypeManifest uint64 = 0x03

	// CommonHeaderSize is the fixed size of every packet header prefix.
	// magic(8) + packet_type(8) + packet_length(8) + header_length(8) + header_md5(16).
	CommonHeaderSize = 48

	// Version is the current format version.
	Version uint64 = 1
)

var (
	ErrInvalidMagic    = errors.New("bundle: invalid magic")
	ErrInvalidChecksum = errors.New("bundle: header md5 mismatch")
	ErrDataCorrupt     = errors.New("bundle: data hash mismatch")
	ErrNotFound        = errors.New("bundle: entry not found")
)

// CommonHeader is the 48-byte prefix shared by all packets.
type CommonHeader struct {
	Magic        [8]byte
	PacketType   uint64
	PacketLength uint64
	HeaderLength uint64
	HeaderMD5    [16]byte // md5 of bytes 8..header_length with this field omitted
}

// MainEntry is one entry in the main packet's file table.
type MainEntry struct {
	PacketOffset uint64
	DataOffset   uint64
	DataLength   uint64
	DataB3       [32]byte
	NameLen      uint64
	Name         string
}

// MainPacket is the index at the start of the bundle.
type MainPacket struct {
	CommonHeader

	Version uint64

	ManifestPacketOffset uint64
	ManifestDataOffset   uint64
	ManifestDataLength   uint64
	ManifestDataB3       [32]byte
	ManifestNameLen      uint64
	ManifestName         string

	EntryCount uint64
	Entries    []MainEntry
}

// FilePacket wraps a single par2 file.
type FilePacket struct {
	CommonHeader

	DataLength uint64
	DataB3     [32]byte
	NameLen    uint64
	Name       string

	// packetOffset/dataOffset are derived positions only filled in by Scan().
	// If not using Scan() the main packet should be consulted for the offsets.
	packetOffset uint64
	dataOffset   uint64
}

// ManifestPacket holds the JSON manifest.
type ManifestPacket struct {
	CommonHeader

	DataLength uint64
	DataB3     [32]byte
	NameLen    uint64
	Name       string

	// packetOffset/dataOffset are derived positions only filled in by Scan().
	// If not using Scan() the main packet should be consulted for the offsets.
	packetOffset uint64
	dataOffset   uint64
}
