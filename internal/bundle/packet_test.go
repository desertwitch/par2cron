package bundle

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"io"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func encodeCommonHeader(t *testing.T, ch CommonHeader) []byte {
	t.Helper()

	var buf bytes.Buffer
	require.NoError(t, binary.Write(&buf, binary.LittleEndian, ch))

	return buf.Bytes()
}

func writePaddedString(t *testing.T, w io.Writer, s string) {
	t.Helper()

	padded := make([]byte, padTo4(uint64(len(s))))
	copy(padded, s)
	require.NoError(t, writeAll(w, padded))
}

// Expectation: Known packet types should be recognized and arbitrary types rejected.
func Test_isKnownPacketType_Success(t *testing.T) {
	t.Parallel()

	require.True(t, isKnownPacketType(PacketTypeIndex))
	require.True(t, isKnownPacketType(PacketTypeFile))
	require.True(t, isKnownPacketType(PacketTypeManifest))
	require.False(t, isKnownPacketType([16]byte{'x'}))
}

// Expectation: packetMD5 should hash recovery set ID, packet type, and body in order.
func Test_packetMD5_Success(t *testing.T) {
	t.Parallel()

	body := []byte("body")
	got := packetMD5(testRecoverySetID, PacketTypeFile, body)

	want := md5.Sum(append(append(testRecoverySetID[:], PacketTypeFile[:]...), body...))

	require.Equal(t, want, got)
}

// Expectation: readAndValidatePacket should read back a valid packet header and body.
func Test_readAndValidatePacket_Success(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	body := []byte("name")
	require.NoError(t, writeCommonPacket(&buf, testRecoverySetID, PacketTypeFile, body))

	ch, gotBody, err := readAndValidatePacket(bytes.NewReader(buf.Bytes()), 0, int64(buf.Len()), true)

	require.NoError(t, err)
	require.Equal(t, Magic, ch.Magic)
	require.Equal(t, PacketTypeFile, ch.PacketType)
	require.Equal(t, body, gotBody)
}

// Expectation: readAndValidatePacket should reject invalid magic bytes.
func Test_readAndValidatePacket_InvalidMagic_Error(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeCommonPacket(&buf, testRecoverySetID, PacketTypeFile, []byte("name")))
	raw := buf.Bytes()
	raw[0] ^= 0xFF

	_, _, err := readAndValidatePacket(bytes.NewReader(raw), 0, int64(len(raw)), false)

	require.ErrorContains(t, err, "invalid magic bytes")
}

// Expectation: readAndValidatePacket should reject packets with invalid checksums when requested.
func Test_readAndValidatePacket_InvalidChecksum_Error(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeCommonPacket(&buf, testRecoverySetID, PacketTypeFile, []byte("name")))
	raw := buf.Bytes()
	raw[commonHeaderSize] ^= 0xFF

	_, _, err := readAndValidatePacket(bytes.NewReader(raw), 0, int64(len(raw)), true)

	require.ErrorContains(t, err, "invalid packet checksum")
}

// Expectation: readAndValidatePacket should reject packets shorter than the fixed header size.
func Test_readAndValidatePacket_InvalidPacketLength_Error(t *testing.T) {
	t.Parallel()

	ch := CommonHeader{
		Magic:         Magic,
		PacketLength:  commonHeaderSize - 4,
		RecoverySetID: testRecoverySetID,
		PacketType:    PacketTypeFile,
	}

	_, _, err := readAndValidatePacket(bytes.NewReader(encodeCommonHeader(t, ch)), 0, commonHeaderSize, false)

	require.ErrorContains(t, err, "invalid packet length")
}

// Expectation: readAndValidatePacket should reject packet lengths that are not 4-byte aligned.
func Test_readAndValidatePacket_NotAligned4_Error(t *testing.T) {
	t.Parallel()

	ch := CommonHeader{
		Magic:         Magic,
		PacketLength:  commonHeaderSize + 1,
		RecoverySetID: testRecoverySetID,
		PacketType:    PacketTypeFile,
	}

	_, _, err := readAndValidatePacket(bytes.NewReader(encodeCommonHeader(t, ch)), 0, int64(ch.PacketLength), false) //nolint:gosec

	require.ErrorContains(t, err, "not 4-byte aligned")
}

// Expectation: readAndValidatePacket should reject packets whose body size exceeds the implementation limit.
func Test_readAndValidatePacket_PacketTooLarge_Error(t *testing.T) {
	t.Parallel()

	ch := CommonHeader{
		Magic:         Magic,
		PacketLength:  commonHeaderSize + maxPacketBodyBytes + 4,
		RecoverySetID: testRecoverySetID,
		PacketType:    PacketTypeFile,
	}

	_, _, err := readAndValidatePacket(
		bytes.NewReader(encodeCommonHeader(t, ch)),
		0,
		int64(ch.PacketLength), //nolint:gosec
		false,
	)

	require.ErrorContains(t, err, "packet body too large")
}

// Expectation: readAndValidatePacket should reject packets whose declared body extends past the file size.
func Test_readAndValidatePacket_BodyExceedsFileSize_Error(t *testing.T) {
	t.Parallel()

	ch := CommonHeader{
		Magic:         Magic,
		PacketLength:  commonHeaderSize + 8,
		RecoverySetID: testRecoverySetID,
		PacketType:    PacketTypeFile,
	}

	_, _, err := readAndValidatePacket(
		bytes.NewReader(encodeCommonHeader(t, ch)),
		0,
		commonHeaderSize,
		false,
	)

	require.ErrorContains(t, err, "body length")
	require.ErrorContains(t, err, "exceeds file size")
}

// Expectation: parseIndexPacket should decode manifest metadata and file entries with padded names.
func Test_parseIndexPacket_Success(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	manifestHash := dataHash([]byte("manifest"))
	entryHash := dataHash([]byte("file"))

	require.NoError(t, writeUint64LE(&body, Version))
	require.NoError(t, writeUint64LE(&body, FlagIndexRebuilt))
	require.NoError(t, writeUint64LE(&body, 100))
	require.NoError(t, writeUint64LE(&body, 168))
	require.NoError(t, writeUint64LE(&body, 8))
	require.NoError(t, writeAll(&body, manifestHash[:]))
	require.NoError(t, writeUint64LE(&body, uint64(len("manifest.json"))))
	writePaddedString(t, &body, "manifest.json")
	require.NoError(t, writeUint64LE(&body, 1))
	require.NoError(t, writeUint64LE(&body, 200))
	require.NoError(t, writeUint64LE(&body, 264))
	require.NoError(t, writeUint64LE(&body, 4))
	require.NoError(t, writeAll(&body, entryHash[:]))
	require.NoError(t, writeUint64LE(&body, uint64(len("file.par2"))))
	writePaddedString(t, &body, "file.par2")

	ch := CommonHeader{
		Magic:         Magic,
		PacketLength:  uint64(commonHeaderSize + body.Len()), //nolint:gosec
		RecoverySetID: testRecoverySetID,
		PacketType:    PacketTypeIndex,
	}

	got, err := parseIndexPacket(bytes.NewReader(body.Bytes()), ch)

	require.NoError(t, err)
	require.Equal(t, Version, got.Version)
	require.Equal(t, FlagIndexRebuilt, got.Flags)
	require.Equal(t, "manifest.json", got.ManifestName)
	require.Len(t, got.Entries, 1)
	require.Equal(t, "file.par2", got.Entries[0].Name)
	require.Equal(t, uint64(264), got.Entries[0].DataOffset)
	require.Equal(t, entryHash, got.Entries[0].DataSHA256)
}

// Expectation: parseIndexPacket should reject manifest names above the supported length range.
func Test_parseIndexPacket_NameLenTooLarge_Error(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer

	require.NoError(t, writeUint64LE(&body, Version))
	require.NoError(t, writeUint64LE(&body, 0))
	require.NoError(t, writeUint64LE(&body, 1))
	require.NoError(t, writeUint64LE(&body, 2))
	require.NoError(t, writeUint64LE(&body, 3))
	require.NoError(t, writeAll(&body, make([]byte, sha256Size)))
	require.NoError(t, writeUint64LE(&body, math.MaxUint16+1))

	_, err := parseIndexPacket(bytes.NewReader(body.Bytes()), CommonHeader{PacketType: PacketTypeIndex})

	require.ErrorContains(t, err, "failed to read manifest name length")
}

// Expectation: parseFilePacket should decode name, sizes, and derived offsets from a valid body.
func Test_parseFilePacket_Success(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	hash := dataHash([]byte("abc"))
	name := "alpha.par2"

	require.NoError(t, writeUint64LE(&body, 3))
	require.NoError(t, writeAll(&body, hash[:]))
	require.NoError(t, writeUint64LE(&body, uint64(len(name))))
	writePaddedString(t, &body, name)

	ch := CommonHeader{
		PacketLength: commonHeaderSize + fileBodyPrefixSize + padTo4(uint64(len(name))),
		PacketType:   PacketTypeFile,
	}

	got, err := parseFilePacket(bytes.NewReader(body.Bytes()), ch, 128)

	require.NoError(t, err)
	require.Equal(t, name, got.Name)
	require.Equal(t, uint64(128), got.packetOffset)
	require.Equal(t, uint64(128)+ch.PacketLength, got.dataOffset)
	require.Equal(t, hash, got.DataSHA256)
}

// Expectation: parseFilePacket should reject negative packet offsets.
func Test_parseFilePacket_NegativeOffset_Error(t *testing.T) {
	t.Parallel()

	_, err := parseFilePacket(bytes.NewReader(nil), CommonHeader{PacketType: PacketTypeFile}, -1)

	require.ErrorContains(t, err, "negative packet offset")
}

// Expectation: parseFilePacket should reject packet offsets that overflow when combined with the packet length.
func Test_parseFilePacket_Overflow_Error(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer

	require.NoError(t, writeUint64LE(&body, 1))
	require.NoError(t, writeAll(&body, make([]byte, sha256Size)))
	require.NoError(t, writeUint64LE(&body, 0))

	_, err := parseFilePacket(bytes.NewReader(body.Bytes()), CommonHeader{
		PacketLength: math.MaxUint64,
		PacketType:   PacketTypeFile,
	}, math.MaxInt64)

	require.ErrorContains(t, err, "packet offset/length overflow")
}

// Expectation: parseFilePacket should reject data ranges whose data offset and length overflow uint64.
func Test_parseFilePacket_DataOffsetLengthOverflow_Error(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer

	require.NoError(t, writeUint64LE(&body, uint64(math.MaxInt64)))
	require.NoError(t, writeAll(&body, make([]byte, sha256Size)))
	require.NoError(t, writeUint64LE(&body, 0))

	_, err := parseFilePacket(bytes.NewReader(body.Bytes()), CommonHeader{
		PacketLength: commonHeaderSize,
		PacketType:   PacketTypeFile,
	}, math.MaxInt64)

	require.ErrorContains(t, err, "data offset/length overflow")
}

// Expectation: parseManifestPacket should decode in-packet manifest data offsets correctly.
func Test_parseManifestPacket_Success(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	hash := dataHash([]byte("abc"))
	name := "manifest.json"
	packetOffset := int64(512)
	packetLength := commonHeaderSize + manifestBodyPrefixSize + padTo4(uint64(len(name))) + padTo4(3)

	require.NoError(t, writeUint64LE(&body, 3))
	require.NoError(t, writeAll(&body, hash[:]))
	require.NoError(t, writeUint64LE(&body, uint64(len(name))))
	writePaddedString(t, &body, name)
	require.NoError(t, writeAll(&body, []byte("abc\x00")))

	got, err := parseManifestPacket(bytes.NewReader(body.Bytes()), CommonHeader{
		PacketLength: packetLength,
		PacketType:   PacketTypeManifest,
	}, packetOffset)

	require.NoError(t, err)
	require.Equal(t, name, got.Name)
	require.Equal(t, uint64(packetOffset), got.packetOffset)
	require.Equal(t, uint64(packetOffset)+commonHeaderSize+manifestBodyPrefixSize+padTo4(uint64(len(name))), got.dataOffset)
	require.Equal(t, uint64(3), got.DataLength)
}

// Expectation: parseManifestPacket should reject negative packet offsets.
func Test_parseManifestPacket_NegativeOffset_Error(t *testing.T) {
	t.Parallel()

	_, err := parseManifestPacket(bytes.NewReader(nil), CommonHeader{PacketType: PacketTypeManifest}, -1)

	require.ErrorContains(t, err, "negative packet offset")
}

// Expectation: parseManifestPacket should reject packet offsets that overflow when combined with the packet length.
func Test_parseManifestPacket_PacketOffsetLengthOverflow_Error(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer

	require.NoError(t, writeUint64LE(&body, 1))
	require.NoError(t, writeAll(&body, make([]byte, sha256Size)))
	require.NoError(t, writeUint64LE(&body, 0))

	_, err := parseManifestPacket(bytes.NewReader(body.Bytes()), CommonHeader{
		PacketLength: math.MaxUint64,
		PacketType:   PacketTypeManifest,
	}, math.MaxInt64)

	require.ErrorContains(t, err, "packet offset/length overflow")
}

// Expectation: parseManifestPacket should reject data ranges whose derived offsets overflow uint64.
func Test_parseManifestPacket_DataOffsetLengthOverflow_Error(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer

	require.NoError(t, writeUint64LE(&body, uint64(math.MaxInt64)))
	require.NoError(t, writeAll(&body, make([]byte, sha256Size)))
	require.NoError(t, writeUint64LE(&body, 0))

	_, err := parseManifestPacket(bytes.NewReader(body.Bytes()), CommonHeader{
		PacketLength: commonHeaderSize + manifestBodyPrefixSize,
		PacketType:   PacketTypeManifest,
	}, math.MaxInt64)

	require.ErrorContains(t, err, "data offset/length overflow")
}

// Expectation: parseManifestPacket should reject data ranges that extend beyond the packet length.
func Test_parseManifestPacket_DataExtendsBeyondPacket_Error(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	name := "manifest.json"

	require.NoError(t, writeUint64LE(&body, 8))
	require.NoError(t, writeAll(&body, make([]byte, sha256Size)))
	require.NoError(t, writeUint64LE(&body, uint64(len(name))))
	writePaddedString(t, &body, name)

	_, err := parseManifestPacket(bytes.NewReader(body.Bytes()), CommonHeader{
		PacketLength: commonHeaderSize + manifestBodyPrefixSize + padTo4(uint64(len(name))),
		PacketType:   PacketTypeManifest,
	}, 64)

	require.ErrorContains(t, err, "data extends beyond packet")
}
