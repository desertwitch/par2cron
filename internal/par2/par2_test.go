package par2

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"os"
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
		{"testdata/recursive_multipar.par2", []string{"tool/ReadMe.txt", "Update_English.txt"}},
		{"testdata/ns_unicode_par2cmdline.par2", []string{"日本語.txt", "emoji🎉.txt"}},
		{"testdata/ns_unicode_multipar.par2", []string{"日本語.txt", "emoji🎉.txt"}},
	}

	syntheticSeeds = [][]byte{
		// Valid spec: ASCII FileDesc
		buildFileDescPacket("test.txt", 100, idA),
		append(buildFileDescPacket("a.txt", 50, idA), buildFileDescPacket("b.txt", 75, idB)...),
		append(buildFileDescPacket("a.txt", 50, idA), buildUnicodePacket("a.txt", idA)...),

		// Valid spec: ASCII FileDesc + Unicode override
		append(buildFileDescPacket("placeholder.txt", 50, idA), buildUnicodePacket("日本語.txt", idA)...),
		append(buildFileDescPacket("placeholder.txt", 50, idB), buildUnicodePacket("🎉🎊🎁.txt", idB)...),
		append(buildFileDescPacket("placeholder.txt", 50, idC), buildUnicodePacket("mixed_αβγ_🚀.txt", idC)...),

		// Invalid spec, but done in most PAR2 software: UTF-8 in ASCII FileDesc
		buildFileDescPacket("not_ascii_日本語.txt", 100, idA),
		buildFileDescPacket("not_ascii_🎉🎊🎁.txt", 100, idA),
		buildFileDescPacket("not_ascii_mixed_αβγ_🚀.txt", 100, idA),
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

			entries, err := Parse(f, true)
			require.NoError(t, err)
			require.Len(t, entries, len(tt.expected))

			for i, name := range tt.expected {
				require.Equal(t, name, entries[i].Name, "entry %d", i)
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
			if err != nil {
				t.Errorf("seed %d failed: %v", i, err)
			}
			if len(entries) == 0 {
				t.Errorf("seed %d returned no entries", i)
			}
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
		if err != nil {
			f.Errorf("failed to read seed file %s: %v", r.file, err)

			continue
		}

		f.Add(content, true)
		f.Add(content, false)
	}

	// A minimal/empty packet and nothing else
	f.Add([]byte{}, false)
	f.Add([]byte("PAR2\x00PKT"), false)

	// A very small length packet and nothing else
	f.Add([]byte("PAR2\x00PKT\x00\x00\x00\x00\x00\x00\x00\x00"), false)

	f.Fuzz(func(t *testing.T, data []byte, checkMD5 bool) {
		entries, err := Parse(bytes.NewReader(data), checkMD5)
		if err != nil {
			// OK to error, just don't panic.
			return
		}

		// Re-parse and verify deterministic result
		entries2, err := Parse(bytes.NewReader(data), checkMD5)
		if err != nil {
			t.Fatal("second parse failed but first succeeded")
		}

		if len(entries) != len(entries2) {
			t.Fatal("non-deterministic entry count")
		}

		for i := range entries {
			if entries[i].Name != entries2[i].Name {
				t.Errorf("non-deterministic name at %d: %q vs %q", i, entries[i].Name, entries2[i].Name)
			}
			if entries[i].Size != entries2[i].Size {
				t.Errorf("non-deterministic size at %d", i)
			}
		}
	})
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
