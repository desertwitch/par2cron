package par2

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"unicode/utf16"
)

var (
	// PAR2 magic bytes: "PAR2\0PKT".
	packetMagic = []byte{'P', 'A', 'R', '2', 0x00, 'P', 'K', 'T'}

	// File Description packet type: "PAR 2.0\0FileDesc".
	fileDescType = []byte{'P', 'A', 'R', ' ', '2', '.', '0', 0x00, 'F', 'i', 'l', 'e', 'D', 'e', 's', 'c'}

	// Unicode Filename packet type: "PAR 2.0\0UniFileN".
	unicodeDescType = []byte{'P', 'A', 'R', ' ', '2', '.', '0', 0x00, 'U', 'n', 'i', 'F', 'i', 'l', 'e', 'N'}
)

const (
	fileIDSize        = 16 // Size of the FileID field
	fileDescSizeFixed = 56 // FileID(16) + HashFull(16) + Hash16k(16) + Length(8)

	maxFilenameLength = 65535    // Sane filename length
	maxPacketSize     = 10 << 20 // Sane packet size (10 MB)

	packetHashOffset = 32 // Offset where MD5 hash verification starts
	packetHeaderSize = 64 // Total header size in bytes
)

var (
	errChecksumMismatch = errors.New("packet checksum mismatch")
	errFilenameTooLong  = errors.New("filename exceeds maximum length")
	errInvalidAlignment = errors.New("packet length not aligned to 4 bytes")
	errInvalidMagic     = errors.New("invalid PAR2 magic bytes")
	errInvalidPacket    = errors.New("invalid packet structure")
	errInvalidUnicode   = errors.New("invalid unicode data")
	errSkipPacket       = errors.New("skip this packet")
)

// FileEntry represents a file protected by the PAR2 set.
type FileEntry struct {
	FileID    [16]byte // The ID of the file (MD5)
	Name      string   // Filename of the file
	Size      int64    // Size of the file
	Hash      [16]byte // MD5 hash of entire file
	Hash16    [16]byte // MD5 hash of first 16KB
	isUnicode bool     // Name is Unicode (or ASCII)
}

// UnicodeFileEntry represents a unicode filename for a file ID.
type UnicodeFileEntry struct {
	FileID [16]byte // The ID of the file (MD5)
	Name   string   // Unicode-name of the file
}

// Parse reads PAR2 data from an io.ReadSeeker and extracts file entries.
func Parse(r io.ReadSeeker, checkMD5 bool) ([]FileEntry, error) {
	files := &struct {
		ASCII   []*FileEntry
		Unicode []*UnicodeFileEntry
	}{}

	for {
		entry, err := readNextPacket(r, checkMD5)
		if errors.Is(err, io.EOF) {
			break
		}
		if errors.Is(err, errSkipPacket) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read packet: %w", err)
		}

		switch e := entry.(type) {
		case *FileEntry:
			files.ASCII = append(files.ASCII, e)
		case *UnicodeFileEntry:
			files.Unicode = append(files.Unicode, e)
		}
	}

	asciiByID := make(map[[16]byte]*FileEntry, len(files.ASCII))
	for _, e := range files.ASCII {
		asciiByID[e.FileID] = e
	}

	for _, ue := range files.Unicode {
		if e, ok := asciiByID[ue.FileID]; ok {
			e.Name = ue.Name
			e.isUnicode = true
		}
	}

	results := make([]FileEntry, 0, len(files.ASCII))
	for _, e := range files.ASCII {
		results = append(results, *e)
	}

	return results, nil
}

// readNextPacket reads packets of interest from the PAR2.
func readNextPacket(r io.ReadSeeker, checkMD5 bool) (any, error) {
	// Read the 64-byte header
	headerBytes := make([]byte, packetHeaderSize)
	if _, err := io.ReadFull(r, headerBytes); err != nil {
		return nil, fmt.Errorf("failed to read packet header: %w", err)
	}

	// Parse header fields
	header, err := parsePacketHeader(headerBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse packet header: %w", err)
	}

	// Validate magic bytes
	if !bytes.Equal(header.magic[:], packetMagic) {
		return nil, fmt.Errorf("%w: not a valid PAR2 packet", errInvalidMagic)
	}

	// Validate 4-byte alignment (per spec)
	if header.length%4 != 0 {
		return nil, fmt.Errorf("%w: length=%d", errInvalidAlignment, header.length)
	}

	// Validate and calculate the packet length
	if header.length < uint64(packetHeaderSize) {
		return nil, fmt.Errorf("%w: packet length %d smaller than header", errInvalidPacket, header.length)
	}
	if header.length > math.MaxInt64 {
		return nil, fmt.Errorf("%w: packet length %d exceeds system capacity", errInvalidPacket, header.length)
	}
	bodyLen := int64(header.length) - int64(packetHeaderSize)

	// Validate that the body has a sane length
	// The packets we care about should be smaller than that
	if bodyLen < 0 || bodyLen > maxPacketSize {
		return nil, fmt.Errorf("%w: invalid body length (%d bytes)", errInvalidPacket, bodyLen)
	}

	// Read the body only for packets we care about, skip the others.
	switch {
	case bytes.Equal(header.packetType[:], fileDescType):
	case bytes.Equal(header.packetType[:], unicodeDescType):
	default:
		// Advance the reading pointer to the end of the body.
		if _, err := r.Seek(bodyLen, io.SeekCurrent); err != nil {
			return nil, fmt.Errorf("failed to skip packet body: %w", err)
		}

		return nil, errSkipPacket
	}

	// Read the body into memory
	bodyBytes := make([]byte, bodyLen)
	if _, err := io.ReadFull(r, bodyBytes); err != nil {
		return nil, fmt.Errorf("failed to read packet body: %w", err)
	}

	if checkMD5 {
		// Verify the packet checksum
		if err := verifyPacketChecksum(header, headerBytes, bodyBytes); err != nil {
			return nil, fmt.Errorf("failed to check packet checksum: %w", err)
		}
	}

	// Parse the packets we care about, skip the others
	switch {
	case bytes.Equal(header.packetType[:], fileDescType):
		return parseFileDescriptionBody(bodyBytes)
	case bytes.Equal(header.packetType[:], unicodeDescType):
		return parseUnicodeDescriptionBody(bodyBytes)
	default:
		return nil, errSkipPacket
	}
}

// packetHeader represents the 64-byte header of every PAR2 packet.
type packetHeader struct {
	magic      [8]byte  // Magic sequence for identification as PAR2 packet
	length     uint64   // Length of the entire packet
	hash       [16]byte // MD5 hash of packet (from setID to end of body)
	setID      [16]byte // Recovery set ID
	packetType [16]byte // Packet type (says which kind of packet it is)
}

// parsePacketHeader returns the header of a PAR2 packet.
func parsePacketHeader(data []byte) (*packetHeader, error) {
	if len(data) < packetHeaderSize {
		return nil, fmt.Errorf("%w: header too short", errInvalidPacket)
	}

	h := &packetHeader{}
	copy(h.magic[:], data[0:8])
	h.length = binary.LittleEndian.Uint64(data[8:16])
	copy(h.hash[:], data[16:32])
	copy(h.setID[:], data[32:48])
	copy(h.packetType[:], data[48:64])

	return h, nil
}

// verifyPacketChecksum verifies the MD5 hash of the packet.
// Per spec: From first byte of recovery set ID to last byte of body.
func verifyPacketChecksum(header *packetHeader, headerBytes, bodyBytes []byte) error {
	hasher := md5.New()

	// Hash from setID (offset 32) to end of header
	hasher.Write(headerBytes[packetHashOffset:])

	// Hash the reset (until end of body of the packet)
	hasher.Write(bodyBytes)

	var computed [16]byte
	copy(computed[:], hasher.Sum(nil))

	if computed != header.hash {
		return fmt.Errorf("%w: expected %x, got %x", errChecksumMismatch, header.hash, computed)
	}

	return nil
}

// parseFileDescriptionBody parses the body of a file description packet.
func parseFileDescriptionBody(body []byte) (*FileEntry, error) {
	// File description body layout:
	// - FileID:   16 bytes (MD5 hash identifier)
	// - HashFull: 16 bytes (MD5 hash of entire file)
	// - Hash16k:  16 bytes (MD5 hash of first 16KB)
	// - Length:   8 bytes  (file size)
	// - Name:     variable (null-padded to multiple of 4)

	if len(body) <= fileDescSizeFixed {
		// Technically also not enough space for any kind of filename.
		return nil, fmt.Errorf("%w: body too short for FileDesc", errInvalidPacket)
	}

	var fileID [16]byte
	var hashFull [16]byte
	var hash16k [16]byte
	copy(fileID[:], body[0:16])
	copy(hashFull[:], body[16:32])
	copy(hash16k[:], body[32:48])

	length := binary.LittleEndian.Uint64(body[48:56])

	nameBytes := body[fileDescSizeFixed:]
	if len(nameBytes) > maxFilenameLength {
		return nil, fmt.Errorf("%w: length=%d", errFilenameTooLong, len(nameBytes))
	}

	// Walk to null byte (per spec: padded with 1-3 zero bytes to reach multiple of 4)
	// Note: If filename length is exact multiple of 4, there may be no null termination
	var name string
	if before0, _, ok := bytes.Cut(nameBytes, []byte{0}); ok {
		name = string(before0)
	} else {
		name = string(nameBytes)
	}

	// This should not be possible with our length validation, but to be safe.
	if name == "" {
		return nil, fmt.Errorf("%w: empty filename", errInvalidPacket)
	}

	// This should not be possible, but a bad implementation could write it.
	if length > uint64(math.MaxInt64) {
		return nil, fmt.Errorf("%w: filesize %d exceeds system capacity", errInvalidPacket, length)
	}

	return &FileEntry{
		Name:   name,
		Size:   int64(length),
		FileID: fileID,
		Hash:   hashFull,
		Hash16: hash16k,
	}, nil
}

// parseUnicodeDescriptionBody parses a Unicode Filename packet body.
// Layout: FileID (16 bytes) + Unicode name (variable, UTF-16LE, padded to 4 bytes).
func parseUnicodeDescriptionBody(body []byte) (*UnicodeFileEntry, error) {
	if len(body) < fileIDSize {
		return nil, fmt.Errorf("%w: body too short for Unicode filename", errInvalidPacket)
	}

	uf := &UnicodeFileEntry{}
	copy(uf.FileID[:], body[0:fileIDSize])

	nameBytes := body[fileIDSize:]
	if len(nameBytes) == 0 {
		return nil, fmt.Errorf("%w: empty unicode filename", errInvalidPacket)
	}

	name, err := decodeUTF16LE(nameBytes)
	if err != nil {
		return nil, err
	}

	uf.Name = name

	return uf, nil
}

// decodeUTF16LE decodes a UTF-16 little-endian byte slice to a Go string.
// Handles null padding (per spec: padded to 4-byte alignment).
//
//nolint:mnd
func decodeUTF16LE(b []byte) (string, error) {
	if len(b)%2 != 0 {
		return "", fmt.Errorf("%w: odd number of bytes for UTF-16", errInvalidUnicode)
	}

	// Pairs of two bytes for UTF16
	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(b[i*2:])
	}

	// Trim null terminators (may have 0-1 null uint16 values for padding)
	for len(u16) > 0 && u16[len(u16)-1] == 0 {
		u16 = u16[:len(u16)-1]
	}

	if len(u16) == 0 {
		return "", fmt.Errorf("%w: empty string after trimming nulls", errInvalidUnicode)
	}

	runes := utf16.Decode(u16)

	return string(runes), nil
}
