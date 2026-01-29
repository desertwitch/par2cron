package par2

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"os"
	"slices"
	"strconv"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/require"
)

var (
	idA = [16]byte{0x01}
	idB = [16]byte{0x02}
	idC = [16]byte{0x03}

	realSeeds = []struct {
		file     string
		expected []string
	}{
		{"testdata/simple_par2cmdline.par2", []string{"test.txt"}},
		{"testdata/simple_multipar.par2", []string{"test.txt"}},
		{"testdata/recursive_par2cmdline.par2", []string{"shallow.txt", "test/test.txt"}},
		{"testdata/recursive_multipar.par2", []string{"Update_English.txt", "tool/ReadMe.txt"}},
		{"testdata/ns_unicode_par2cmdline.par2", []string{"emoji🎉.txt", "日本語.txt"}},
		{"testdata/ns_unicode_multipar.par2", []string{"emoji🎉.txt", "日本語.txt"}},
	}

	syntheticSeeds = [][]byte{
		// Valid spec: ASCII FileDesc
		slices.Concat(buildMainPacket(4096, [][16]byte{idA}, nil), buildFileDescPacket("test.txt", 100, idA)),
		slices.Concat(buildMainPacket(4096, [][16]byte{idA, idB}, nil), buildFileDescPacket("a.txt", 50, idA), buildFileDescPacket("b.txt", 75, idB)),
		slices.Concat(buildMainPacket(4096, [][16]byte{idA}, nil), buildFileDescPacket("a.txt", 50, idA), buildUnicodePacket("a.txt", idA)),

		// Valid spec: ASCII FileDesc + Unicode override
		slices.Concat(buildMainPacket(4096, [][16]byte{idA}, nil), buildFileDescPacket("placeholder.txt", 50, idA), buildUnicodePacket("日本語.txt", idA)),
		slices.Concat(buildMainPacket(4096, [][16]byte{idB}, nil), buildFileDescPacket("placeholder.txt", 50, idB), buildUnicodePacket("🎉🎊🎁.txt", idB)),
		slices.Concat(buildMainPacket(4096, [][16]byte{idC}, nil), buildFileDescPacket("placeholder.txt", 50, idC), buildUnicodePacket("mixed_αβγ_🚀.txt", idC)),

		// Invalid spec, but done in most PAR2 software: UTF-8 in ASCII FileDesc
		slices.Concat(buildMainPacket(4096, [][16]byte{idA}, nil), buildFileDescPacket("not_ascii_日本語.txt", 100, idA)),
		slices.Concat(buildMainPacket(4096, [][16]byte{idA}, nil), buildFileDescPacket("not_ascii_🎉🎊🎁.txt", 100, idA)),
		slices.Concat(buildMainPacket(4096, [][16]byte{idA}, nil), buildFileDescPacket("not_ascii_mixed_αβγ_🚀.txt", 100, idA)),
	}
)

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

func FuzzParse(f *testing.F) {
	// Synthetic PAR2 files constructed for testing
	for _, seed := range syntheticSeeds {
		f.Add(seed, true)
		f.Add(seed, false)
	}

	// Real PAR2 files from actual PAR2 software
	for _, r := range realSeeds {
		content, err := os.ReadFile(r.file)
		require.NoError(f, err)

		f.Add(content, true)
		f.Add(content, false)
	}

	// A minimal/empty packet and nothing else
	f.Add([]byte{}, false)
	f.Add([]byte("PAR2\x00PKT"), false)

	// A very small length packet and nothing else
	f.Add([]byte("PAR2\x00PKT\x00\x00\x00\x00\x00\x00\x00\x00"), false)

	f.Fuzz(func(t *testing.T, data []byte, checkMD5 bool) {
		sets1, err := Parse(bytes.NewReader(data), checkMD5)
		if err != nil {
			return
		}

		sets2, err := Parse(bytes.NewReader(data), checkMD5)
		require.NoError(t, err, "non-deterministic parse success (second failed)")

		require.Equal(t, sets1, sets2, "non-deterministic output for same input")
	})
}

func buildMainPacket(sliceSize uint64, recoveryIDs [][16]byte, nonRecoveryIDs [][16]byte) []byte {
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

	return buildPacket(mainType, body)
}

func buildPacket(packetType []byte, body []byte) []byte {
	const headerLen = 64
	totalSize := uint64(headerLen) + uint64(len(body))

	packet := make([]byte, totalSize) // Already zero'd.

	copy(packet[0:8], packetMagic)
	binary.LittleEndian.PutUint64(packet[8:16], totalSize)
	// hash at 16:32 - will be filled in later
	// setID at 32:48 - zeros fine
	copy(packet[48:64], packetType)
	copy(packet[64:], body)

	hasher := md5.New()
	hasher.Write(packet[32:]) // from setID to end of packet
	copy(packet[16:32], hasher.Sum(nil))

	return packet
}

func buildFileDescPacket(name string, size uint64, fileID [16]byte) []byte {
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

	return buildPacket(fileDescType, body)
}

func buildUnicodePacket(name string, fileID [16]byte) []byte {
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

	return buildPacket(unicodeDescType, body)
}
