package par2

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"unicode/utf16"
)

var (
	// PAR2 magic bytes: "PAR2\0PKT".
	packetMagic = []byte{'P', 'A', 'R', '2', 0x00, 'P', 'K', 'T'}

	// Main packet type: "PAR 2.0\0Main\0\0\0\0".
	mainType = []byte{'P', 'A', 'R', ' ', '2', '.', '0', 0x00, 'M', 'a', 'i', 'n', 0x00, 0x00, 0x00, 0x00}

	// File Description packet type: "PAR 2.0\0FileDesc".
	fileDescType = []byte{'P', 'A', 'R', ' ', '2', '.', '0', 0x00, 'F', 'i', 'l', 'e', 'D', 'e', 's', 'c'}

	// Unicode Filename packet type: "PAR 2.0\0UniFileN".
	unicodeDescType = []byte{'P', 'A', 'R', ' ', '2', '.', '0', 0x00, 'U', 'n', 'i', 'F', 'i', 'l', 'e', 'N'}
)

const (
	mainSizeFixed     = 12 // SliceSize(8) + NumFiles (4)
	fileDescSizeFixed = 56 // FileID(16) + HashFull(16) + Hash16k(16) + Length(8)

	maxSets           = 10       // Sane amount of sets
	maxPacketSize     = 10 << 20 // Sane packet size (10 MB)
	maxFilenameLength = 65535    // Sane filename length

	packetHashOffset = 32 // Starting offset for MD5 hashing
	packetHeaderSize = 64 // Total header size of a packet in bytes
)

var (
	errChecksumMismatch = errors.New("packet checksum mismatch")
	errFilenameTooLong  = errors.New("filename exceeds maximum length")
	errInvalidAlignment = errors.New("packet length not aligned to 4 bytes")
	errInvalidMagic     = errors.New("invalid PAR2 magic bytes")
	errInvalidPacket    = errors.New("invalid packet structure")
	errInvalidUnicode   = errors.New("invalid unicode data")
	errTooManySets      = errors.New("too many sets in archive")
	errSkipPacket       = errors.New("skip this packet")
)

// Parse reads PAR2 data and returns a slice of [Set] in the order they appeared.
func Parse(r io.ReadSeeker, checkMD5 bool) ([]Set, error) {
	groups := make(map[Hash]*setGroup)

	var order []Hash
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

		var setID Hash
		switch e := entry.(type) {
		case *MainPacket:
			setID = e.SetID
		case *FilePacket:
			setID = e.SetID
		case *UnicodePacket:
			setID = e.SetID
		}

		if _, exists := groups[setID]; !exists {
			if len(groups) >= maxSets {
				return nil, fmt.Errorf("%w: len=%d", errTooManySets, len(groups))
			}
			groups[setID] = &setGroup{
				setID:             setID,
				recoveryIDs:       make(map[Hash]struct{}),
				nonRecoveryIDs:    make(map[Hash]struct{}),
				unfilteredASCII:   make(map[Hash]*FilePacket),
				unfilteredUnicode: make(map[Hash]*UnicodePacket),
			}
			order = append(order, setID)
		}
		group := groups[setID]

		switch e := entry.(type) {
		case *MainPacket:
			group.mainPacket = e
			for _, v := range e.RecoveryIDs {
				group.recoveryIDs[v] = struct{}{}
			}
			for _, v := range e.NonRecoveryIDs {
				group.nonRecoveryIDs[v] = struct{}{}
			}
		case *FilePacket:
			group.unfilteredASCII[e.FileID] = e
		case *UnicodePacket:
			group.unfilteredUnicode[e.FileID] = e
		}
	}

	return toSets(groups, order), nil
}

type setGroup struct {
	setID             Hash                    // Dataset ID
	mainPacket        *MainPacket             // Main packet (can be nil)
	recoveryIDs       map[Hash]struct{}       // Protected (recovery) IDs
	nonRecoveryIDs    map[Hash]struct{}       // Auxiliary (non-recovery) IDs
	unfilteredASCII   map[Hash]*FilePacket    // File description packets
	unfilteredUnicode map[Hash]*UnicodePacket // Unicode override packets
}

// toSets processes a map[Hash]*setGroup to a slice of [Set].
// It preserves the order in the provided as argument slice of set IDs.
func toSets(groups map[Hash]*setGroup, order []Hash) []Set {
	results := make([]Set, 0, len(order))

	for _, id := range order {
		group := groups[id]

		for _, ue := range group.unfilteredUnicode {
			if fe, ok := group.unfilteredASCII[ue.FileID]; ok {
				fe.Name = ue.Name
				fe.FromUnicode = true
			}
		}

		recoveryList := make([]FilePacket, 0, len(group.recoveryIDs))
		nonRecoveryList := make([]FilePacket, 0, len(group.nonRecoveryIDs))
		strayList := make([]FilePacket, 0)

		for _, fe := range group.unfilteredASCII {
			if _, ok := group.recoveryIDs[fe.FileID]; ok {
				recoveryList = append(recoveryList, *fe)
			} else if _, ok := group.nonRecoveryIDs[fe.FileID]; ok {
				nonRecoveryList = append(nonRecoveryList, *fe)
			} else {
				strayList = append(strayList, *fe)
			}
		}

		recoveryMissing := []Hash{}
		for id := range group.recoveryIDs {
			if _, ok := group.unfilteredASCII[id]; !ok {
				recoveryMissing = append(recoveryMissing, id)
			}
		}

		nonRecoveryMissing := []Hash{}
		for id := range group.nonRecoveryIDs {
			if _, ok := group.unfilteredASCII[id]; !ok {
				nonRecoveryMissing = append(nonRecoveryMissing, id)
			}
		}

		sortFilePackets(recoveryList)
		sortFilePackets(nonRecoveryList)
		sortFilePackets(strayList)

		sortFileIDs(recoveryMissing)
		sortFileIDs(nonRecoveryMissing)

		var mainPacket *MainPacket
		if group.mainPacket != nil {
			mainPacket = &MainPacket{
				SetID:          group.mainPacket.SetID,
				SliceSize:      group.mainPacket.SliceSize,
				RecoveryIDs:    slices.Clone(group.mainPacket.RecoveryIDs),
				NonRecoveryIDs: slices.Clone(group.mainPacket.NonRecoveryIDs),
			}
		}

		results = append(results, Set{
			SetID:          id,
			MainPacket:     mainPacket,
			RecoverySet:    recoveryList,
			NonRecoverySet: nonRecoveryList,

			StrayPackets:              strayList,
			MissingRecoveryPackets:    recoveryMissing,
			MissingNonRecoveryPackets: nonRecoveryMissing,
		})
	}

	return results
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
	case bytes.Equal(header.packetType[:], mainType):
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
			return nil, fmt.Errorf("failed to validate packet checksum: %w", err)
		}
	}

	// Parse the packets we care about, skip the others
	switch {
	case bytes.Equal(header.packetType[:], mainType):
		return parseMainPacketBody(header.setID, bodyBytes)
	case bytes.Equal(header.packetType[:], fileDescType):
		return parseFileDescriptionBody(header.setID, bodyBytes)
	case bytes.Equal(header.packetType[:], unicodeDescType):
		return parseUnicodeDescriptionBody(header.setID, bodyBytes)
	default:
		return nil, errSkipPacket
	}
}

// packetHeader represents the 64-byte header of every PAR2 packet.
type packetHeader struct {
	magic      [8]byte // Magic sequence for identification as PAR2 packet
	length     uint64  // Length of the entire packet
	hash       Hash    // MD5 hash of packet (from setID to end of body)
	setID      Hash    // Recovery set ID
	packetType Hash    // Packet type (says which kind of packet it is)
}

// parsePacketHeader returns the header of a PAR2 packet.
func parsePacketHeader(data []byte) (*packetHeader, error) {
	if len(data) < packetHeaderSize {
		return nil, fmt.Errorf("%w: packet header too short", errInvalidPacket)
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

	var computed Hash
	copy(computed[:], hasher.Sum(nil))

	if computed != header.hash {
		return fmt.Errorf("%w: expected %x, got %x", errChecksumMismatch, header.hash, computed)
	}

	return nil
}

// parseMainPacketBody parses the body of a PAR2 main packet.
func parseMainPacketBody(setID Hash, body []byte) (*MainPacket, error) {
	// Main packet body layout:
	// - Slice Size:    8 bytes
	// - Num Files:     4 bytes
	// - Recovery IDs:  16 bytes * number of files
	// - Non-Rec IDs:   Remaining bytes (multiple of 16)

	if len(body) < mainSizeFixed {
		return nil, fmt.Errorf("%w: body too short for main packet", errInvalidPacket)
	}

	sliceSize := binary.LittleEndian.Uint64(body[0:8])
	numRecoveryFiles := binary.LittleEndian.Uint32(body[8:12])

	// Check the slice size alignment (needs to be a multiple of 4)
	if sliceSize%4 != 0 {
		return nil, fmt.Errorf("%w: slice size %d not multiple of 4", errInvalidAlignment, sliceSize)
	}

	// Check expected size for recovery IDs vs. actual bytes in body
	if uint64(numRecoveryFiles)*HashSize > uint64(len(body))-uint64(mainSizeFixed) {
		return nil, fmt.Errorf("%w: claimed bytes mismatch packet body", errInvalidPacket)
	}

	// Now parse the recovery file IDs
	recoveryIDs := make([]Hash, numRecoveryFiles)

	curr := mainSizeFixed // Start after fixed-width fields
	for i := range numRecoveryFiles {
		recoveryIDs[i] = Hash(body[curr : curr+HashSize])
		curr += HashSize
	}

	// The rest of the packet is non-recovery file IDs
	remaining := len(body) - curr
	if remaining%HashSize != 0 {
		return nil, fmt.Errorf("%w: non-recovery section size %d", errInvalidAlignment, remaining)
	}

	numNonRecoveryFiles := remaining / HashSize
	nonRecoveryIDs := make([]Hash, numNonRecoveryFiles)

	for i := range numNonRecoveryFiles {
		nonRecoveryIDs[i] = Hash(body[curr : curr+HashSize])
		curr += HashSize
	}

	return &MainPacket{
		SetID:          setID,
		SliceSize:      sliceSize,
		RecoveryIDs:    recoveryIDs,
		NonRecoveryIDs: nonRecoveryIDs,
	}, nil
}

// parseFileDescriptionBody parses the body of a file description packet.
func parseFileDescriptionBody(setID Hash, body []byte) (*FilePacket, error) {
	// File description body layout:
	// - FileID:   16 bytes (MD5 hash identifier)
	// - HashFull: 16 bytes (MD5 hash of entire file)
	// - Hash16k:  16 bytes (MD5 hash of first 16KB)
	// - Length:   8 bytes  (file size)
	// - Name:     variable (null-padded to multiple of 4)

	if len(body) < fileDescSizeFixed+4 {
		return nil, fmt.Errorf("%w: body too short for file packet", errInvalidPacket)
	}

	fileID := Hash(body[0:16])
	hashFull := Hash(body[16:32])
	hash16k := Hash(body[32:48])
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

	// This should not be possible, but a bad implementation could write it.
	if name == "" {
		return nil, fmt.Errorf("%w: empty filename", errInvalidPacket)
	}

	// This should not be possible, but a bad implementation could write it.
	if length > uint64(math.MaxInt64) {
		return nil, fmt.Errorf("%w: filesize %d exceeds system capacity", errInvalidPacket, length)
	}

	return &FilePacket{
		SetID:  setID,
		Name:   name,
		Size:   int64(length),
		FileID: fileID,
		Hash:   hashFull,
		Hash16: hash16k,
	}, nil
}

// parseUnicodeDescriptionBody parses a unicode filename packet body.
func parseUnicodeDescriptionBody(setID Hash, body []byte) (*UnicodePacket, error) {
	// Unicode file description body layout:
	// - FileID:       16 bytes (MD5 hash identifier)
	// - Unicode name: variable (null-padded to multiple of 4)

	if len(body) < HashSize+4 {
		// We are not so strict with the unicode packets, and just skip it.
		return nil, errSkipPacket
	}

	decodedName, err := decodeUTF16LE(body[HashSize:])
	if err != nil {
		// We are not so strict with the unicode packets, and just skip it.
		return nil, errSkipPacket
	}

	return &UnicodePacket{
		SetID:  setID,
		FileID: Hash(body[:HashSize]),
		Name:   decodedName,
	}, nil
}

// decodeUTF16LE decodes a UTF-16 little-endian byte slice to a Go string.
// It handles null padding per specification: padded to a 4-byte alignment.
func decodeUTF16LE(b []byte) (string, error) {
	if len(b)%2 != 0 {
		return "", fmt.Errorf("%w: odd number of bytes for UTF-16", errInvalidUnicode)
	}

	// Check against a too long filename
	if len(b) > maxFilenameLength*2 {
		return "", fmt.Errorf("%w: len=%d", errFilenameTooLong, len(b))
	}

	// Pairs of two bytes for UTF16
	u16 := make([]uint16, len(b)/2) //nolint:mnd
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(b[i*2:])
	}

	for i, v := range u16 {
		if v == 0 {
			u16 = u16[:i]

			break
		}
	}

	if len(u16) == 0 {
		return "", fmt.Errorf("%w: nothing left after trimming nulls", errInvalidUnicode)
	}

	runes := utf16.Decode(u16)

	return string(runes), nil
}
