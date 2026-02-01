package par2

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"slices"
	"strconv"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/require"
)

var (
	sID = [16]byte{0x00}
	idA = [16]byte{0x01}
	idB = [16]byte{0x02}
	idC = [16]byte{0x03}

	realSeeds = []struct {
		file     string
		expected []string
	}{
		{"testdata/simple_par2cmdline.par2", []string{"test.txt"}},
		{"testdata/simple_multipar.par2", []string{"test.txt"}},
		{"testdata/simple_quickpar.par2", []string{"test.txt"}},
		{"testdata/simple_par2cmdlineturbo.par2", []string{"test.txt"}},
		{"testdata/recursive_par2cmdline.par2", []string{"shallow.txt", "test/test.txt"}},
		{"testdata/recursive_multipar.par2", []string{"Update_English.txt", "tool/ReadMe.txt"}},
		{"testdata/recursive_par2cmdlineturbo.par2", []string{"dir/test.txt", "test.txt"}},
		{"testdata/ns_unicode_par2cmdline.par2", []string{"emoji🎉.txt", "日本語.txt"}},
		{"testdata/ns_unicode_multipar.par2", []string{"emoji🎉.txt", "日本語.txt"}},
		{"testdata/ns_unicode_par2cmdlineturbo.par2", []string{"dir/ascii.txt", "dir/test.txt", "dir/日本語.txt", "emoji🎉.txt"}},
	}

	syntheticSeeds = [][]byte{
		// Valid spec: ASCII FileDesc
		slices.Concat(
			buildMainPacket(4096, [][16]byte{idA}, nil, sID),
			buildFileDescPacket("test.txt", 100, idA, sID),
		),
		slices.Concat(
			buildMainPacket(4096, [][16]byte{idA, idB}, nil, sID),
			buildFileDescPacket("a.txt", 50, idA, sID),
			buildFileDescPacket("b.txt", 75, idB, sID),
		),
		slices.Concat(
			buildMainPacket(4096, [][16]byte{idA}, nil, sID),
			buildFileDescPacket("a.txt", 50, idA, sID),
			buildUnicodePacket("a.txt", idA, sID),
		),

		// Valid spec: ASCII FileDesc + Unicode override
		slices.Concat(
			buildMainPacket(4096, [][16]byte{idA}, nil, sID),
			buildFileDescPacket("placeholder.txt", 50, idA, sID),
			buildUnicodePacket("日本語.txt", idA, sID)),
		slices.Concat(
			buildMainPacket(4096, [][16]byte{idB}, nil, sID),
			buildFileDescPacket("placeholder.txt", 50, idB, sID),
			buildUnicodePacket("🎉🎊🎁.txt", idB, sID),
		),
		slices.Concat(
			buildMainPacket(4096, [][16]byte{idC}, nil, sID),
			buildFileDescPacket("placeholder.txt", 50, idC, sID),
			buildUnicodePacket("mixed_αβγ_🚀.txt", idC, sID),
		),

		// Invalid spec, but done in most PAR2 software: UTF-8 in ASCII FileDesc
		slices.Concat(
			buildMainPacket(4096, [][16]byte{idA}, nil, sID),
			buildFileDescPacket("not_ascii_日本語.txt", 100, idA, sID),
		),
		slices.Concat(
			buildMainPacket(4096, [][16]byte{idA}, nil, sID),
			buildFileDescPacket("not_ascii_🎉🎊🎁.txt", 100, idA, sID),
		),
		slices.Concat(
			buildMainPacket(4096, [][16]byte{idA}, nil, sID),
			buildFileDescPacket("not_ascii_mixed_αβγ_🚀.txt", 100, idA, sID),
		),
	}
)

// ============================================================================
// Fuzz Test
// ============================================================================

func FuzzParse(f *testing.F) {
	// Synthetic PAR2 files constructed for testing
	for _, seed := range syntheticSeeds {
		f.Add(seed)
	}

	// Real PAR2 files from actual PAR2 software
	for _, r := range realSeeds {
		seed, err := os.ReadFile(r.file)
		require.NoError(f, err)
		f.Add(seed)
	}

	// A minimal/empty packet and nothing else
	f.Add([]byte{})
	f.Add([]byte("PAR2\x00PKT"))

	// A very small length packet and nothing else
	f.Add([]byte("PAR2\x00PKT\x00\x00\x00\x00\x00\x00\x00\x00"))

	f.Fuzz(func(t *testing.T, data []byte) {
		sets1, err1 := Parse(bytes.NewReader(data), false)
		sets2, err2 := Parse(bytes.NewReader(data), false)

		require.Equal(t, err1, err2, "non-deterministic error")
		require.Equal(t, sets1, sets2, "non-deterministic result")
	})
}

// ============================================================================
// Fuzz-Related Tests
// ============================================================================

func Test_Parse_RealSeeds_Success(t *testing.T) {
	t.Parallel()

	for _, tt := range realSeeds {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			f, err := os.Open(tt.file)
			require.NoError(t, err)
			defer f.Close()

			sets, err := Parse(f, true)
			require.NoError(t, err)

			require.Len(t, sets, 1)
			require.Len(t, sets[0].RecoverySet, len(tt.expected))

			for i, name := range tt.expected {
				require.Equal(t, name, sets[0].RecoverySet[i].Name, "entry %d", i)
			}
		})
	}
}

func Test_Parse_SyntheticSeeds_Success(t *testing.T) {
	t.Parallel()

	for i, seed := range syntheticSeeds {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()

			entries, err := Parse(bytes.NewReader(seed), true)

			require.NoError(t, err)
			require.NotEmpty(t, entries)
		})
	}
}

// ============================================================================
// Integration and Unit Tests
// ============================================================================

// Expectation: Parse should handle empty input gracefully.
func Test_Parse_EmptyInput_Success(t *testing.T) {
	t.Parallel()

	sets, err := Parse(bytes.NewReader([]byte{}), false)
	require.NoError(t, err)
	require.Empty(t, sets)
}

// Expectation: Parse should handle multiple sets in the same file.
func Test_Parse_MultipleSets_Success(t *testing.T) {
	t.Parallel()

	// Create two different sets with different setIDs
	set1Main := buildMainPacket(4096, [][16]byte{idA}, nil, idA)
	set1File := buildFileDescPacket("file1.txt", 100, idA, idA)

	set2Main := buildMainPacket(4096, [][16]byte{idB}, nil, idB)
	set2File := buildFileDescPacket("file2.txt", 200, idB, idB)

	combined := slices.Concat(set1Main, set1File, set2Main, set2File)

	sets, err := Parse(bytes.NewReader(combined), false)
	require.NoError(t, err)
	require.Len(t, sets, 2)
}

// Expectation: Parse should handle files with only stray packets.
func Test_Parse_OnlyStrayPackets_Success(t *testing.T) {
	t.Parallel()

	// File packet without corresponding main packet entry
	filePacket := buildFileDescPacket("stray.txt", 100, idC, sID)

	sets, err := Parse(bytes.NewReader(filePacket), false)
	require.NoError(t, err)
	require.Len(t, sets, 1)
	require.Empty(t, sets[0].RecoverySet)
	require.Empty(t, sets[0].NonRecoverySet)
	require.Len(t, sets[0].StrayPackets, 1)
}

// Expectation: Parse should handle missing file description packets.
func Test_Parse_MissingFileDescriptions_Success(t *testing.T) {
	t.Parallel()

	mainPacket := buildMainPacket(4096, [][16]byte{idA, idB}, [][16]byte{idC}, sID)

	sets, err := Parse(bytes.NewReader(mainPacket), false)
	require.NoError(t, err)
	require.Len(t, sets, 1)
	require.Len(t, sets[0].MissingRecoveryPackets, 2)
	require.Len(t, sets[0].MissingNonRecoveryPackets, 1)
}

// Expectation: Parse should properly categorize recovery and non-recovery files.
func Test_Parse_RecoveryAndNonRecovery_Success(t *testing.T) {
	t.Parallel()

	main := buildMainPacket(4096, [][16]byte{idA}, [][16]byte{idB}, sID)
	file1 := buildFileDescPacket("recovery.txt", 100, idA, sID)
	file2 := buildFileDescPacket("nonrecovery.txt", 200, idB, sID)

	combined := slices.Concat(main, file1, file2)

	sets, err := Parse(bytes.NewReader(combined), false)
	require.NoError(t, err)
	require.Len(t, sets, 1)
	require.Len(t, sets[0].RecoverySet, 1)
	require.Len(t, sets[0].NonRecoverySet, 1)
	require.Equal(t, "recovery.txt", sets[0].RecoverySet[0].Name)
	require.Equal(t, "nonrecovery.txt", sets[0].NonRecoverySet[0].Name)
}

// Expectation: Parse should override ASCII filename with unicode filename.
func Test_Parse_UnicodeOverride_Success(t *testing.T) {
	t.Parallel()

	main := buildMainPacket(4096, [][16]byte{idA}, nil, sID)
	ascii := buildFileDescPacket("placeholder.txt", 100, idA, sID)
	unicode := buildUnicodePacket("日本語.txt", idA, sID)

	combined := slices.Concat(main, ascii, unicode)

	sets, err := Parse(bytes.NewReader(combined), false)
	require.NoError(t, err)
	require.Len(t, sets, 1)
	require.Equal(t, "日本語.txt", sets[0].RecoverySet[0].Name)
	require.True(t, sets[0].RecoverySet[0].FromUnicode)
}

// Expectation: Parse should handle unicode packet without matching ASCII packet.
func Test_Parse_UnicodeWithoutASCII_Success(t *testing.T) {
	t.Parallel()

	main := buildMainPacket(4096, [][16]byte{idA}, nil, sID)
	unicode := buildUnicodePacket("orphan.txt", idB, sID)

	combined := slices.Concat(main, unicode)

	sets, err := Parse(bytes.NewReader(combined), false)
	require.NoError(t, err)
	require.Len(t, sets, 1)
	require.Empty(t, sets[0].RecoverySet) // idB not in recovery list
}

// Expectation: Parse should handle multiple unicode overrides.
func Test_Parse_MultipleUnicodeOverrides_Success(t *testing.T) {
	t.Parallel()

	main := buildMainPacket(4096, [][16]byte{idA, idB}, nil, sID)
	ascii1 := buildFileDescPacket("file1.txt", 100, idA, sID)
	ascii2 := buildFileDescPacket("file2.txt", 200, idB, sID)
	unicode1 := buildUnicodePacket("ファイル1.txt", idA, sID)
	unicode2 := buildUnicodePacket("ファイル2.txt", idB, sID)

	combined := slices.Concat(main, ascii1, ascii2, unicode1, unicode2)

	sets, err := Parse(bytes.NewReader(combined), false)
	require.NoError(t, err)
	require.Len(t, sets, 1)
	require.Len(t, sets[0].RecoverySet, 2)
	require.Equal(t, "ファイル1.txt", sets[0].RecoverySet[0].Name)
	require.Equal(t, "ファイル2.txt", sets[0].RecoverySet[1].Name)
}

// Expectation: readNextPacket should fail when parsePacketHeader returns error.
func Test_readNextPacket_ParseHeaderError_Error(t *testing.T) {
	t.Parallel()

	invalidHeader := make([]byte, 64)
	binary.LittleEndian.PutUint64(invalidHeader[8:16], 64) // length

	_, err := readNextPacket(bytes.NewReader(invalidHeader), false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid PAR2 magic bytes")
}

// Expectation: readNextPacket should handle EOF when reading header.
func Test_readNextPacket_HeaderEOF_Error(t *testing.T) {
	t.Parallel()

	emptyReader := bytes.NewReader([]byte{})

	_, err := readNextPacket(emptyReader, false)
	require.ErrorIs(t, err, io.EOF)
}

// Expectation: readNextPacket should handle partial header read.
func Test_readNextPacket_PartialHeader_Error(t *testing.T) {
	t.Parallel()

	partialHeader := make([]byte, 50)

	_, err := readNextPacket(bytes.NewReader(partialHeader), false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read packet header")
}

// Expectation: readNextPacket should handle EOF when reading body.
func Test_readNextPacket_BodyEOF_Error(t *testing.T) {
	t.Parallel()

	header := make([]byte, 64)
	copy(header[0:8], packetMagic)
	binary.LittleEndian.PutUint64(header[8:16], 164) // 64 + 100 bytes
	copy(header[48:64], mainType)

	hasher := md5.New()
	hasher.Write(header[packetHashOffset:])
	copy(header[16:32], hasher.Sum(nil))

	// No body despite 100 bytes claimed...

	_, err := readNextPacket(bytes.NewReader(header), false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read packet body")
}

// Expectation: readNextPacket should handle partial body read.
func Test_readNextPacket_PartialBody_Error(t *testing.T) {
	t.Parallel()

	header := make([]byte, 64)
	copy(header[0:8], packetMagic)
	binary.LittleEndian.PutUint64(header[8:16], 164) // 64 + 100 bytes
	copy(header[48:64], mainType)

	hasher := md5.New()
	hasher.Write(header[packetHashOffset:])
	copy(header[16:32], hasher.Sum(nil))

	partialBody := make([]byte, 50) // Only 50 of 100 bytes
	combined := slices.Concat(header, partialBody)

	_, err := readNextPacket(bytes.NewReader(combined), false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read packet body")
}

// Expectation: readNextPacket should return errSkipPacket for unknown packet types.
func Test_readNextPacket_UnknownPacketType_Success(t *testing.T) {
	t.Parallel()

	unknownType := []byte{'P', 'A', 'R', ' ', '2', '.', '0', 0x00, 'U', 'n', 'k', 'n', 'o', 'w', 'n', '!'}

	header := make([]byte, 64)
	copy(header[0:8], packetMagic)
	binary.LittleEndian.PutUint64(header[8:16], 68) // 64 + 4 bytes (aligned)
	copy(header[48:64], unknownType)

	body := make([]byte, 4)
	hasher := md5.New()
	hasher.Write(header[packetHashOffset:])
	hasher.Write(body)
	copy(header[16:32], hasher.Sum(nil))

	combined := slices.Concat(header, body)

	_, err := readNextPacket(bytes.NewReader(combined), false)
	require.ErrorIs(t, err, errSkipPacket)
}

// Expectation: readNextPacket should fail if seek fails when skipping unknown packets.
func Test_readNextPacket_SeekError_Error(t *testing.T) {
	t.Parallel()

	unknownType := []byte{'P', 'A', 'R', ' ', '2', '.', '0', 0x00, 'U', 'n', 'k', 'n', 'o', 'w', 'n', '!'}

	header := make([]byte, 64)
	copy(header[0:8], packetMagic)
	binary.LittleEndian.PutUint64(header[8:16], 1064) // 64 + 1000 bytes
	copy(header[48:64], unknownType)

	hasher := md5.New()
	hasher.Write(header[packetHashOffset:])
	copy(header[16:32], hasher.Sum(nil))

	// Use a non-seekable reader (io.LimitReader doesn't implement Seek)
	// Wrap in a struct that only implements Read
	nonSeekableReader := &nonSeekableReader{reader: bytes.NewReader(header)}

	_, err := readNextPacket(nonSeekableReader, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to skip packet body")
}

// Expectation: readNextPacket should handle packet length exceeding MaxInt64.
func Test_readNextPacket_LengthExceedsMaxInt64_Error(t *testing.T) {
	t.Parallel()

	header := make([]byte, 64)
	copy(header[0:8], packetMagic)
	binary.LittleEndian.PutUint64(header[8:16], uint64(1<<63)+100) // > MaxInt64
	copy(header[48:64], mainType)

	hasher := md5.New()
	hasher.Write(header[packetHashOffset:])
	copy(header[16:32], hasher.Sum(nil))

	_, err := readNextPacket(bytes.NewReader(header), false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds system capacity")
}

// Expectation: readNextPacket should handle negative body length.
func Test_readNextPacket_NegativeBodyLength_Error(t *testing.T) {
	t.Parallel()

	header := make([]byte, 64)
	copy(header[0:8], packetMagic)
	binary.LittleEndian.PutUint64(header[8:16], 32) // < header size (64)
	copy(header[48:64], mainType)

	hasher := md5.New()
	hasher.Write(header[packetHashOffset:])
	copy(header[16:32], hasher.Sum(nil))

	_, err := readNextPacket(bytes.NewReader(header), false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "packet length")
}

// Expectation: readNextPacket should reject packet with body length exceeding max size.
func Test_readNextPacket_ExceedingBodyLength_Error(t *testing.T) {
	t.Parallel()

	header := make([]byte, 64)
	copy(header[0:8], packetMagic)
	// Total packet size = header (64) + body (maxPacketSize + 4)
	binary.LittleEndian.PutUint64(header[8:16], uint64(64+maxPacketSize+4))
	copy(header[48:64], mainType)

	hasher := md5.New()
	hasher.Write(header[packetHashOffset:])
	copy(header[16:32], hasher.Sum(nil))

	_, err := readNextPacket(bytes.NewReader(header), false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid body length")
}

// Expectation: readNextPacket should reject packets with invalid length alignment.
func Test_readNextPacket_InvalidAlignment_Error(t *testing.T) {
	t.Parallel()

	packet := buildMainPacket(4096, [][16]byte{idA}, nil, sID)

	// Set packet length to non-multiple of 4
	binary.LittleEndian.PutUint64(packet[8:16], 65)

	_, err := readNextPacket(bytes.NewReader(packet), false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not aligned to 4 bytes")
}

func Test_readNextPacket_PacketAtMaxSize_Success(t *testing.T) {
	t.Parallel()

	body := make([]byte, maxPacketSize)
	packet := buildPacket(mainType, body, sID)

	_, err := readNextPacket(bytes.NewReader(packet), false)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "invalid body length")
}

// Expectation: readNextPacket should fail on MD5 checksum mismatch when checkMD5 is true.
func Test_readNextPacket_MD5ChecksumMismatch_Error(t *testing.T) {
	t.Parallel()

	packet := buildMainPacket(4096, [][16]byte{idA}, nil, sID)

	// Corrupt the MD5 hash in the header (bytes 16-32)
	packet[16] ^= 0xFF

	_, err := readNextPacket(bytes.NewReader(packet), true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to validate packet checksum")
}

// Expectation: parsePacketHeader should fail on truncated header.
func Test_parsePacketHeader_TruncatedHeader_Error(t *testing.T) {
	t.Parallel()

	shortHeader := make([]byte, 32) // Less than 64 bytes

	_, err := parsePacketHeader(shortHeader)
	require.Error(t, err)
	require.Contains(t, err.Error(), "packet header too short")
}

// Expectation: parsePacketHeader should correctly parse valid header.
func Test_parsePacketHeader_ValidHeader_Success(t *testing.T) {
	t.Parallel()

	header := make([]byte, 64)
	copy(header[0:8], packetMagic)
	binary.LittleEndian.PutUint64(header[8:16], 1024)
	copy(header[32:48], idA[:])
	copy(header[48:64], mainType)

	parsed, err := parsePacketHeader(header)
	require.NoError(t, err)
	require.Equal(t, uint64(1024), parsed.length)
	require.Equal(t, Hash(idA), parsed.setID)
}

// Expectation: parseMainPacketBody should reject invalid slice size alignment.
func Test_parseMainPacketBody_InvalidSliceSizeAlignment_Error(t *testing.T) {
	t.Parallel()

	body := make([]byte, 12)
	binary.LittleEndian.PutUint64(body[0:8], 4097) // Not multiple of 4
	binary.LittleEndian.PutUint32(body[8:12], 0)

	_, err := parseMainPacketBody(Hash{}, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "slice size")
}

// Expectation: parseMainPacketBody should fail on body too short.
func Test_parseMainPacketBody_BodyTooShort_Error(t *testing.T) {
	t.Parallel()

	shortBody := make([]byte, 8)

	_, err := parseMainPacketBody(Hash{}, shortBody)
	require.Error(t, err)
	require.Contains(t, err.Error(), "body too short")
}

// Expectation: parseMainPacketBody should handle zero recovery files.
func Test_parseMainPacketBody_ZeroRecoveryFiles_Success(t *testing.T) {
	t.Parallel()

	body := make([]byte, 12)
	binary.LittleEndian.PutUint64(body[0:8], 4096)
	binary.LittleEndian.PutUint32(body[8:12], 0)

	packet, err := parseMainPacketBody(Hash{}, body)
	require.NoError(t, err)
	require.Empty(t, packet.RecoveryIDs)
	require.Empty(t, packet.NonRecoveryIDs)
}

// Expectation: parseMainPacketBody should handle only recovery files.
func Test_parseMainPacketBody_OnlyRecoveryFiles_Success(t *testing.T) {
	t.Parallel()

	body := make([]byte, 12+16*2)
	binary.LittleEndian.PutUint64(body[0:8], 4096)
	binary.LittleEndian.PutUint32(body[8:12], 2)
	copy(body[12:28], idA[:])
	copy(body[28:44], idB[:])

	packet, err := parseMainPacketBody(Hash{}, body)
	require.NoError(t, err)
	require.Len(t, packet.RecoveryIDs, 2)
	require.Empty(t, packet.NonRecoveryIDs)
}

// Expectation: parseMainPacketBody should handle only non-recovery files.
func Test_parseMainPacketBody_OnlyNonRecoveryFiles_Success(t *testing.T) {
	t.Parallel()

	body := make([]byte, 12+16*2)
	binary.LittleEndian.PutUint64(body[0:8], 4096)
	binary.LittleEndian.PutUint32(body[8:12], 0)
	copy(body[12:28], idA[:])
	copy(body[28:44], idB[:])

	packet, err := parseMainPacketBody(Hash{}, body)
	require.NoError(t, err)
	require.Empty(t, packet.RecoveryIDs)
	require.Len(t, packet.NonRecoveryIDs, 2)
}

// Expectation: parseMainPacketBody should fail on insufficient bytes for recovery IDs.
func Test_parseMainPacketBody_InsufficientRecoveryBytes_Error(t *testing.T) {
	t.Parallel()

	body := make([]byte, 12+8) // not enough space for 2 files
	binary.LittleEndian.PutUint64(body[0:8], 4096)
	binary.LittleEndian.PutUint32(body[8:12], 2)

	_, err := parseMainPacketBody(Hash{}, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "bytes mismatch packet body")
}

// Expectation: parseMainPacketBody should fail on misaligned non-recovery IDs.
func Test_parseMainPacketBody_MisalignedNonRecovery_Error(t *testing.T) {
	t.Parallel()

	body := make([]byte, 12+8) // 8 bytes is not multiple of 16
	binary.LittleEndian.PutUint64(body[0:8], 4096)
	binary.LittleEndian.PutUint32(body[8:12], 0)

	_, err := parseMainPacketBody(Hash{}, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not aligned to 4 bytes")
}

// Expectation: parseFileDescriptionBody should fail on body too short.
func Test_parseFileDescriptionBody_BodyTooShort_Error(t *testing.T) {
	t.Parallel()

	shortBody := make([]byte, 50) // Less than fileDescSizeFixed+4 (60)

	_, err := parseFileDescriptionBody(Hash{}, shortBody)
	require.Error(t, err)
	require.Contains(t, err.Error(), "body too short")
}

// Expectation: parseFileDescriptionBody should handle filename with null terminator.
func Test_parseFileDescriptionBody_FilenameWithNull_Success(t *testing.T) {
	t.Parallel()

	body := make([]byte, 56+8)
	copy(body[0:16], idA[:])
	binary.LittleEndian.PutUint64(body[48:56], 100)
	copy(body[56:], []byte("test.txt\x00"))

	packet, err := parseFileDescriptionBody(Hash{}, body)
	require.NoError(t, err)
	require.Equal(t, "test.txt", packet.Name)
}

// Expectation: parseFileDescriptionBody should handle filename without null terminator.
func Test_parseFileDescriptionBody_FilenameNoNull_Success(t *testing.T) {
	t.Parallel()

	body := make([]byte, 56+8)
	copy(body[0:16], idA[:])
	binary.LittleEndian.PutUint64(body[48:56], 100)
	copy(body[56:], []byte("test.txt"))

	packet, err := parseFileDescriptionBody(Hash{}, body)
	require.NoError(t, err)
	require.Equal(t, "test.txt", packet.Name)
}

// Expectation: parseFileDescriptionBody should fail on empty filename.
func Test_parseFileDescriptionBody_EmptyFilename_Error(t *testing.T) {
	t.Parallel()

	body := make([]byte, 56+4)
	copy(body[0:16], idA[:])
	binary.LittleEndian.PutUint64(body[48:56], 100)
	body[56] = 0 // Null terminator immediately

	_, err := parseFileDescriptionBody(Hash{}, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty filename")
}

// Expectation: parseFileDescriptionBody should fail on filesize exceeding MaxInt64.
func Test_parseFileDescriptionBody_FilesizeTooLarge_Error(t *testing.T) {
	t.Parallel()

	body := make([]byte, 56+8)
	copy(body[0:16], idA[:])
	binary.LittleEndian.PutUint64(body[48:56], uint64(1<<63)) // MaxInt64 + 1
	copy(body[56:], []byte("test.txt"))

	_, err := parseFileDescriptionBody(Hash{}, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds system capacity")
}

// Expectation: parseFileDescriptionBody should handle long filenames correctly.
func Test_parseFileDescriptionBody_LongFilename_Success(t *testing.T) {
	t.Parallel()

	longName := string(bytes.Repeat([]byte("a"), 1000))
	nameBytes := []byte(longName)
	padding := (4 - (len(nameBytes) % 4)) % 4

	body := make([]byte, 56+len(nameBytes)+padding)
	copy(body[0:16], idA[:])
	binary.LittleEndian.PutUint64(body[48:56], 100)
	copy(body[56:], nameBytes)

	packet, err := parseFileDescriptionBody(Hash{}, body)
	require.NoError(t, err)
	require.Equal(t, longName, packet.Name)
}

// Expectation: parseFileDescriptionBody should handle all hashes correctly.
func Test_parseFileDescriptionBody_AllHashes_Success(t *testing.T) {
	t.Parallel()

	body := make([]byte, 56+8)
	copy(body[0:16], idA[:])
	copy(body[16:32], idB[:]) // HashFull
	copy(body[32:48], idC[:]) // Hash16k
	binary.LittleEndian.PutUint64(body[48:56], 100)
	copy(body[56:], []byte("test.txt"))

	packet, err := parseFileDescriptionBody(Hash{}, body)
	require.NoError(t, err)
	require.Equal(t, Hash(idA), packet.FileID)
	require.Equal(t, Hash(idB), packet.Hash)
	require.Equal(t, Hash(idC), packet.Hash16k)
}

// Expectation: parseFileDescriptionBody should handle filename at maximum allowed length.
func Test_parseFileDescriptionBody_FilenameAtMaxLength_Success(t *testing.T) {
	t.Parallel()

	// Account for 4-byte alignment padding
	// So use maxFilenameLength - 4 to ensure we have room for padding
	maxName := string(bytes.Repeat([]byte("a"), maxFilenameLength-4))
	nameBytes := []byte(maxName)
	padding := (4 - (len(nameBytes) % 4)) % 4

	body := make([]byte, 56+len(nameBytes)+padding)
	copy(body[0:16], idA[:])
	binary.LittleEndian.PutUint64(body[48:56], 100)
	copy(body[56:], nameBytes)

	packet, err := parseFileDescriptionBody(Hash{}, body)
	require.NoError(t, err)
	require.Equal(t, maxName, packet.Name)
}

// Expectation: parseFileDescriptionBody should fail when filename exactly exceeds max length.
func Test_parseFileDescriptionBody_FilenameExceedsMaxLength_Error(t *testing.T) {
	t.Parallel()

	// Create a filename that will exceed maxFilenameLength after padding
	tooLongName := string(bytes.Repeat([]byte("a"), maxFilenameLength))
	nameBytes := []byte(tooLongName)
	padding := (4 - (len(nameBytes) % 4)) % 4

	body := make([]byte, 56+len(nameBytes)+padding)
	copy(body[0:16], idA[:])
	binary.LittleEndian.PutUint64(body[48:56], 100)
	copy(body[56:], nameBytes)

	_, err := parseFileDescriptionBody(Hash{}, body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "filename exceeds maximum length")
}

// Expectation: parseFileDescriptionBody should handle zero-length file.
func Test_parseFileDescriptionBody_ZeroLengthFile_Success(t *testing.T) {
	t.Parallel()

	body := make([]byte, 56+12) // Enough for "empty.txt" + padding
	copy(body[0:16], idA[:])
	binary.LittleEndian.PutUint64(body[48:56], 0) // Zero file size
	copy(body[56:], []byte("empty.txt"))

	packet, err := parseFileDescriptionBody(Hash{}, body)
	require.NoError(t, err)
	require.Equal(t, int64(0), packet.Size)
	require.Equal(t, "empty.txt", packet.Name)
}

// Expectation: parseFileDescriptionBody should handle maximum int64 file size.
func Test_parseFileDescriptionBody_MaxInt64FileSize_Success(t *testing.T) {
	t.Parallel()

	body := make([]byte, 56+8)
	copy(body[0:16], idA[:])
	binary.LittleEndian.PutUint64(body[48:56], uint64(1<<63-1)) // MaxInt64
	copy(body[56:], []byte("big.txt"))

	packet, err := parseFileDescriptionBody(Hash{}, body)
	require.NoError(t, err)
	require.Equal(t, int64(1<<63-1), packet.Size)
	require.Equal(t, "big.txt", packet.Name)
}

// Expectation: parseFileDescriptionBody should handle filename with special characters.
func Test_parseFileDescriptionBody_FilenameSpecialChars_Success(t *testing.T) {
	t.Parallel()

	specialName := "file!@#$%^&*()_+-={}[]|\\:;\"'<>,.?~`.txt"
	nameBytes := []byte(specialName)
	padding := (4 - (len(nameBytes) % 4)) % 4

	body := make([]byte, 56+len(nameBytes)+padding)
	copy(body[0:16], idA[:])
	binary.LittleEndian.PutUint64(body[48:56], 100)
	copy(body[56:], nameBytes)

	packet, err := parseFileDescriptionBody(Hash{}, body)
	require.NoError(t, err)
	require.Equal(t, specialName, packet.Name)
}

// Expectation: parseUnicodeDescriptionBody should fail on body too short.
func Test_parseUnicodeDescriptionBody_BodyTooShort_Success(t *testing.T) {
	t.Parallel()

	shortBody := make([]byte, 16) // Less than HashSize+4 (20)

	_, err := parseUnicodeDescriptionBody(Hash{}, shortBody)
	require.ErrorIs(t, err, errSkipPacket)
}

// Expectation: parseUnicodeDescriptionBody should handle valid UTF-16LE.
func Test_parseUnicodeDescriptionBody_ValidUTF16_Success(t *testing.T) {
	t.Parallel()

	name := "test日本語.txt"
	u16 := utf16.Encode([]rune(name))

	body := make([]byte, 16+len(u16)*2+2) // +2 for null terminator
	copy(body[0:16], idA[:])
	for i, v := range u16 {
		binary.LittleEndian.PutUint16(body[16+i*2:], v)
	}

	packet, err := parseUnicodeDescriptionBody(Hash{}, body)
	require.NoError(t, err)
	require.Equal(t, name, packet.Name)
}

// Expectation: parseUnicodeDescriptionBody should handle emoji correctly.
func Test_parseUnicodeDescriptionBody_Emoji_Success(t *testing.T) {
	t.Parallel()

	name := "emoji🎉🎊.txt"
	u16 := utf16.Encode([]rune(name))

	body := make([]byte, 16+len(u16)*2+2)
	copy(body[0:16], idA[:])
	for i, v := range u16 {
		binary.LittleEndian.PutUint16(body[16+i*2:], v)
	}

	packet, err := parseUnicodeDescriptionBody(Hash{}, body)
	require.NoError(t, err)
	require.Equal(t, name, packet.Name)
}

// Expectation: parseUnicodeDescriptionBody should skip packet on decodeUTF16LE error (odd bytes).
func Test_parseUnicodeDescriptionBody_DecodeError_OddBytes_Success(t *testing.T) {
	t.Parallel()

	// Create body with odd number of UTF-16 bytes (will cause decodeUTF16LE to fail)
	body := make([]byte, 16+5) // 16 for FileID + 5 odd bytes
	copy(body[0:16], idA[:])
	copy(body[16:], []byte{0x41, 0x00, 0x42, 0x00, 0x43}) // Odd byte count

	_, err := parseUnicodeDescriptionBody(Hash{}, body)
	require.ErrorIs(t, err, errSkipPacket)
}

// Expectation: parseUnicodeDescriptionBody should skip packet on decodeUTF16LE error (all nulls).
func Test_parseUnicodeDescriptionBody_DecodeError_AllNulls_Success(t *testing.T) {
	t.Parallel()

	// Create body with all-null UTF-16 data (will cause decodeUTF16LE to fail)
	body := make([]byte, 16+8)
	copy(body[0:16], idA[:])
	// Rest is zeros (all nulls)

	_, err := parseUnicodeDescriptionBody(Hash{}, body)
	require.ErrorIs(t, err, errSkipPacket)
}

// Expectation: decodeUTF16LE should fail on odd number of bytes.
func Test_decodeUTF16LE_OddBytes_Error(t *testing.T) {
	t.Parallel()

	oddBytes := []byte{0x00, 0x00, 0x00} // 3 bytes

	_, err := decodeUTF16LE(oddBytes)
	require.Error(t, err)
	require.Contains(t, err.Error(), "odd number of bytes")
}

// Expectation: decodeUTF16LE should fail on all-null input.
func Test_decodeUTF16LE_AllNulls_Error(t *testing.T) {
	t.Parallel()

	nullBytes := []byte{0x00, 0x00, 0x00, 0x00}

	_, err := decodeUTF16LE(nullBytes)
	require.Error(t, err)
	require.Contains(t, err.Error(), "nothing left after trimming nulls")
}

// Expectation: decodeUTF16LE should handle null termination correctly.
func Test_decodeUTF16LE_NullTermination_Success(t *testing.T) {
	t.Parallel()

	// "AB" followed by null terminator and padding
	data := []byte{0x41, 0x00, 0x42, 0x00, 0x00, 0x00, 0x00, 0x00}

	result, err := decodeUTF16LE(data)
	require.NoError(t, err)
	require.Equal(t, "AB", result)
}

// Expectation: decodeUTF16LE should handle surrogate pairs correctly.
func Test_decodeUTF16LE_SurrogatePairs_Success(t *testing.T) {
	t.Parallel()

	// Emoji with surrogate pairs
	name := "🎉"
	u16 := utf16.Encode([]rune(name))

	data := make([]byte, len(u16)*2+2)
	for i, v := range u16 {
		binary.LittleEndian.PutUint16(data[i*2:], v)
	}

	result, err := decodeUTF16LE(data)
	require.NoError(t, err)
	require.Equal(t, name, result)
}

// Expectation: Sets should preserve order of sets.
func Test_setGrouper_Sets_PreservesOrder_Success(t *testing.T) {
	t.Parallel()

	grouper := &setGrouper{}
	grouper.groups = map[Hash]*setGroup{
		idA: {setID: idA},
		idB: {setID: idB},
		idC: {setID: idC},
	}
	grouper.order = []Hash{idC, idA, idB}

	sets := grouper.Sets()

	require.Len(t, sets, 3)
	require.Equal(t, Hash(idC), sets[0].SetID)
	require.Equal(t, Hash(idA), sets[1].SetID)
	require.Equal(t, Hash(idB), sets[2].SetID)
}

// Expectation: Sets should handle empty groups.
func Test_setGrouper_EmptyGroups_Success(t *testing.T) {
	t.Parallel()

	grouper := &setGrouper{}
	grouper.groups = map[Hash]*setGroup{}
	grouper.order = []Hash{}

	sets := grouper.Sets()

	require.Empty(t, sets)
}

// Expectation: asSets should handle set with no main packet.
func Test_setGrouper_NoMainPacket_Success(t *testing.T) {
	t.Parallel()

	grouper := &setGrouper{}
	grouper.groups = map[Hash]*setGroup{
		idA: {
			setID:             idA,
			recoveryIDs:       make(map[Hash]struct{}),
			nonRecoveryIDs:    make(map[Hash]struct{}),
			unfilteredASCII:   make(map[Hash]*FilePacket),
			unfilteredUnicode: make(map[Hash]*UnicodePacket),
		},
	}
	grouper.order = []Hash{idA}

	sets := grouper.Sets()

	require.Len(t, sets, 1)
	require.Nil(t, sets[0].MainPacket)
}

// ============================================================================
// Helper Functions for Tests
// ============================================================================

// nonSeekableReader wraps a reader and only implements Read, not Seek.
type nonSeekableReader struct {
	reader io.Reader
}

func (n *nonSeekableReader) Read(p []byte) (int, error) {
	return n.reader.Read(p)
}

func (n *nonSeekableReader) Seek(offset int64, whence int) (int64, error) {
	return 0, errors.New("seek not supported")
}

func buildMainPacket(sliceSize uint64, recoveryIDs [][16]byte, nonRecoveryIDs [][16]byte, setID [16]byte) []byte {
	bodyLen := 12 + len(recoveryIDs)*16 + len(nonRecoveryIDs)*16
	body := make([]byte, bodyLen)

	binary.LittleEndian.PutUint64(body[0:8], sliceSize)
	binary.LittleEndian.PutUint32(body[8:12], uint32(len(recoveryIDs))) //nolint:gosec

	offset := 12
	for _, id := range recoveryIDs {
		copy(body[offset:offset+16], id[:])
		offset += 16
	}
	for _, id := range nonRecoveryIDs {
		copy(body[offset:offset+16], id[:])
		offset += 16
	}

	return buildPacket(mainType, body, setID)
}

func buildPacket(packetType []byte, body []byte, setID [16]byte) []byte {
	const headerLen = 64
	totalSize := uint64(headerLen) + uint64(len(body))

	packet := make([]byte, totalSize) // Already zero'd.

	copy(packet[0:8], packetMagic)
	binary.LittleEndian.PutUint64(packet[8:16], totalSize)
	// hash at 16:32 - will be filled in later
	copy(packet[32:48], setID[:])
	copy(packet[48:64], packetType)
	copy(packet[64:], body)

	hasher := md5.New()
	hasher.Write(packet[packetHashOffset:]) // from setID to end of packet
	copy(packet[16:32], hasher.Sum(nil))

	return packet
}

func buildFileDescPacket(name string, size uint64, fileID [16]byte, setID [16]byte) []byte {
	nameBytes := []byte(name)
	contentLen := 56 + len(nameBytes)

	// 4-byte alignment
	padding := (4 - (contentLen % 4)) % 4
	totalSize := contentLen + padding

	body := make([]byte, totalSize) // Already zero'd.

	copy(body[0:16], fileID[:])
	// HashFull, Hash16k at 16:48 (zeros is fine)
	binary.LittleEndian.PutUint64(body[48:56], size)
	copy(body[56:], nameBytes)

	return buildPacket(fileDescType, body, setID)
}

func buildUnicodePacket(name string, fileID [16]byte, setID [16]byte) []byte {
	u16 := utf16.Encode([]rune(name))
	contentLen := 16 + len(u16)*2

	// 4-byte alignment
	padding := (4 - (contentLen % 4)) % 4
	totalSize := contentLen + padding

	body := make([]byte, totalSize) // Already zero'd.

	copy(body[0:16], fileID[:])
	for i, v := range u16 {
		binary.LittleEndian.PutUint16(body[16+i*2:], v)
	}

	return buildPacket(unicodeDescType, body, setID)
}
