package par2

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: Hash.MarshalJSON should correctly encode hash to hex string.
func Test_Hash_MarshalJSON_Success(t *testing.T) {
	t.Parallel()

	hash := Hash(idA)

	data, err := json.Marshal(&hash)
	require.NoError(t, err)

	expected := `"01000000000000000000000000000000"`
	require.Equal(t, expected, string(data))
}

// Expectation: Hash.MarshalJSON should handle zero hash.
func Test_Hash_MarshalJSON_ZeroHash_Success(t *testing.T) {
	t.Parallel()

	hash := Hash{}

	data, err := json.Marshal(&hash)
	require.NoError(t, err)

	expected := `"00000000000000000000000000000000"`
	require.Equal(t, expected, string(data))
}

// Expectation: Hash.MarshalJSON should handle all bytes set.
func Test_Hash_MarshalJSON_AllBytesSet_Success(t *testing.T) {
	t.Parallel()

	hash := Hash{}
	for i := range hash {
		hash[i] = 0xFF
	}

	data, err := json.Marshal(&hash)
	require.NoError(t, err)

	expected := `"ffffffffffffffffffffffffffffffff"`
	require.Equal(t, expected, string(data))
}

// Expectation: Hash.MarshalJSON should produce valid JSON in struct.
func Test_Hash_MarshalJSON_InStruct_Success(t *testing.T) {
	t.Parallel()

	type TestStruct struct {
		ID   *Hash  `json:"id"`
		Name string `json:"name"`
	}

	id := Hash(idA)
	s := TestStruct{
		ID:   &id,
		Name: "test",
	}

	data, err := json.Marshal(s)
	require.NoError(t, err)

	var result TestStruct
	require.NoError(t, json.Unmarshal(data, &result))
	require.NotNil(t, result.ID)
	require.Equal(t, *s.ID, *result.ID)
	require.Equal(t, s.Name, result.Name)
}

// Expectation: Hash.UnmarshalJSON should correctly decode hex string to hash.
func Test_Hash_UnmarshalJSON_Success(t *testing.T) {
	t.Parallel()

	data := []byte(`"01000000000000000000000000000000"`)

	var hash Hash
	err := json.Unmarshal(data, &hash)
	require.NoError(t, err)
	require.Equal(t, Hash(idA), hash)
}

// Expectation: Hash.UnmarshalJSON should handle zero hash.
func Test_Hash_UnmarshalJSON_ZeroHash_Success(t *testing.T) {
	t.Parallel()

	data := []byte(`"00000000000000000000000000000000"`)

	var hash Hash
	err := json.Unmarshal(data, &hash)
	require.NoError(t, err)
	require.Equal(t, Hash{}, hash)
}

// Expectation: Hash.UnmarshalJSON should handle all bytes set.
func Test_Hash_UnmarshalJSON_AllBytesSet_Success(t *testing.T) {
	t.Parallel()

	data := []byte(`"ffffffffffffffffffffffffffffffff"`)

	var hash Hash
	err := json.Unmarshal(data, &hash)
	require.NoError(t, err)

	expected := Hash{}
	for i := range expected {
		expected[i] = 0xFF
	}
	require.Equal(t, expected, hash)
}

// Expectation: Hash.UnmarshalJSON should fail on invalid JSON.
func Test_Hash_UnmarshalJSON_InvalidJSON_Error(t *testing.T) {
	t.Parallel()

	data := []byte(`{invalid json}`)

	var hash Hash
	err := json.Unmarshal(data, &hash)
	require.Error(t, err)
}

// Expectation: Hash.UnmarshalJSON should fail on invalid hex string.
func Test_Hash_UnmarshalJSON_InvalidHex_Error(t *testing.T) {
	t.Parallel()

	data := []byte(`"gggggggggggggggggggggggggggggggg"`)

	var hash Hash
	err := json.Unmarshal(data, &hash)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to decode to hex")
}

// Expectation: Hash.UnmarshalJSON should fail on wrong length (too short).
func Test_Hash_UnmarshalJSON_TooShort_Error(t *testing.T) {
	t.Parallel()

	data := []byte(`"0100000000000000"`)

	var hash Hash
	err := json.Unmarshal(data, &hash)
	require.Error(t, err)
	require.ErrorIs(t, err, errUnexpectedLength)
	require.Contains(t, err.Error(), "expected 16 bytes")
}

// Expectation: Hash.UnmarshalJSON should fail on wrong length (too long).
func Test_Hash_UnmarshalJSON_TooLong_Error(t *testing.T) {
	t.Parallel()

	data := []byte(`"0100000000000000000000000000000000000000"`)

	var hash Hash
	err := json.Unmarshal(data, &hash)
	require.Error(t, err)
	require.ErrorIs(t, err, errUnexpectedLength)
	require.Contains(t, err.Error(), "expected 16 bytes")
}

// Expectation: Hash.UnmarshalJSON should fail on non-string JSON type.
func Test_Hash_UnmarshalJSON_NonString_Error(t *testing.T) {
	t.Parallel()

	data := []byte(`123`)

	var hash Hash
	err := json.Unmarshal(data, &hash)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal")
}

// Expectation: Hash.UnmarshalJSON should handle uppercase hex.
func Test_Hash_UnmarshalJSON_UppercaseHex_Success(t *testing.T) {
	t.Parallel()

	data := []byte(`"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"`)

	var hash Hash
	err := json.Unmarshal(data, &hash)
	require.NoError(t, err)

	expected := Hash{}
	for i := range expected {
		expected[i] = 0xFF
	}
	require.Equal(t, expected, hash)
}

// Expectation: Hash.UnmarshalJSON should round-trip with MarshalJSON.
func Test_Hash_JSON_RoundTrip_Success(t *testing.T) {
	t.Parallel()

	original := Hash(idB)

	// Marshal
	data, err := json.Marshal(&original)
	require.NoError(t, err)

	// Unmarshal
	var decoded Hash
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	require.Equal(t, original, decoded)
}

// Expectation: Hash.UnmarshalJSON should handle mixed case hex.
func Test_Hash_UnmarshalJSON_MixedCaseHex_Success(t *testing.T) {
	t.Parallel()

	data := []byte(`"AaBbCcDdEeFf00112233445566778899"`)

	var hash Hash
	err := json.Unmarshal(data, &hash)
	require.NoError(t, err)

	expected := Hash{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99}
	require.Equal(t, expected, hash)
}

// Expectation: ParseFile should successfully parse a valid PAR2 file.
func Test_ParseFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	packet := buildMainPacket(4096, [][16]byte{idA}, nil, sID)
	require.NoError(t, afero.WriteFile(fs, "/test.par2", packet, 0o644))

	before := time.Now()
	archive, err := ParseFile(fs, "/test.par2")
	after := time.Now()

	require.NoError(t, err)
	require.NotNil(t, archive)
	require.Len(t, archive.Sets, 1)
	require.True(t, archive.Time.After(before) || archive.Time.Equal(before))
	require.True(t, archive.Time.Before(after) || archive.Time.Equal(after))
}

// Expectation: ParseFile should fail when file doesn't exist.
func Test_ParseFile_FileNotFound_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	_, err := ParseFile(fs, "/nonexistent.par2")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to open PAR2 file")
}

// Expectation: ParseFile should fail on invalid PAR2 content.
func Test_ParseFile_InvalidPAR2_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/invalid.par2", []byte("not a par2 file"), 0o644))

	_, err := ParseFile(fs, "/invalid.par2")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse PAR2")
}

// Expectation: ParseFile should parse real PAR2 files correctly.
func Test_ParseFile_RealFile_Success(t *testing.T) {
	t.Parallel()

	for _, tt := range realSeeds {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			fs := afero.NewOsFs()

			archive, err := ParseFile(fs, tt.file)
			require.NoError(t, err)
			require.NotNil(t, archive)
			require.Len(t, archive.Sets, 1)
			require.Len(t, archive.Sets[0].RecoverySet, len(tt.expected))
		})
	}
}

// Expectation: ParseFile should handle empty PAR2 file.
func Test_ParseFile_EmptyFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/empty.par2", []byte{}, 0o644))

	archive, err := ParseFile(fs, "/empty.par2")
	require.NoError(t, err)
	require.NotNil(t, archive)
	require.Empty(t, archive.Sets)
}

// Expectation: ParseFile should handle multiple sets in one file.
func Test_ParseFile_MultipleSets_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	set1 := buildMainPacket(4096, [][16]byte{idA}, nil, idA)
	set2 := buildMainPacket(4096, [][16]byte{idB}, nil, idB)
	combined := slices.Concat(set1, set2)

	require.NoError(t, afero.WriteFile(fs, "/multi.par2", combined, 0o644))

	archive, err := ParseFile(fs, "/multi.par2")
	require.NoError(t, err)
	require.NotNil(t, archive)
	require.Len(t, archive.Sets, 2)
}

// Expectation: ParseFileToArchivePtr should successfully parse a valid PAR2 file.
func Test_ParseFileToArchivePtr_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	validPar2 := loadTestPar2(t)
	require.NoError(t, afero.WriteFile(fs, "/test.par2", validPar2, 0o644))

	var archive *Archive
	var logMessages []string

	ParseFileToArchivePtr(&archive, fs, "/test.par2", func(msg string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf(msg, args...))
	})

	require.NotNil(t, archive)
	require.Empty(t, logMessages)
	require.NotNil(t, archive.Sets)
	require.Len(t, archive.Sets, 1)
}

// Expectation: ParseFileToArchivePtr should set target to nil on parse error.
func Test_ParseFileToArchivePtr_ParseError_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/invalid.par2", []byte("not a par2 file"), 0o644))

	var archive *Archive
	var logMessages []string

	ParseFileToArchivePtr(&archive, fs, "/invalid.par2", func(msg string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf("%s %v", msg, args))
	})

	require.Nil(t, archive)
	require.Len(t, logMessages, 1)
	require.Contains(t, logMessages[0], "Failed to parse PAR2")
}

// Expectation: ParseFileToArchivePtr should set target to nil when file doesn't exist.
func Test_ParseFileToArchivePtr_FileNotFound_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var archive *Archive
	var logMessages []string

	ParseFileToArchivePtr(&archive, fs, "/nonexistent.par2", func(msg string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf("%s %v", msg, args))
	})

	require.Nil(t, archive)
	require.Len(t, logMessages, 1)
	require.Contains(t, logMessages[0], "Failed to parse PAR2")
}

// Expectation: ParseFileToArchivePtr should handle panic and set target to nil.
func Test_ParseFileToArchivePtr_Panic_Success(t *testing.T) {
	t.Parallel()

	panicFs := &panicingFs{}

	var archive *Archive
	var logMessages []string

	ParseFileToArchivePtr(&archive, panicFs, "/test.par2", func(msg string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf("%s %v", msg, args))
	})

	require.Nil(t, archive)
	require.Len(t, logMessages, 1)
	require.Contains(t, logMessages[0], "Panic while parsing PAR2")
	require.Contains(t, logMessages[0], "test panic")
}

// Expectation: ParseFileToArchivePtr should log panic with stack trace.
func Test_ParseFileToArchivePtr_PanicWithStackTrace_Success(t *testing.T) {
	t.Parallel()

	panicFs := &panicingFs{}

	var archive *Archive
	var logMessages []string
	var logArgs [][]any

	ParseFileToArchivePtr(&archive, panicFs, "/test.par2", func(msg string, args ...any) {
		logMessages = append(logMessages, msg)
		logArgs = append(logArgs, args)
	})

	require.Nil(t, archive)
	require.Len(t, logMessages, 1)
	require.Contains(t, logMessages[0], "Panic while parsing PAR2")

	require.NotEmpty(t, logArgs)
	found := false
	for _, arg := range logArgs[0] {
		if str, ok := arg.(string); ok && strings.Contains(str, "goroutine") {
			found = true

			break
		}
	}
	require.True(t, found, "Stack trace should be included in log args")
}

// Expectation: ParseFileToArchivePtr should handle concurrent calls safely.
func Test_ParseFileToArchivePtr_Concurrent_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	validPar2 := loadTestPar2(t)
	require.NoError(t, afero.WriteFile(fs, "/test.par2", validPar2, 0o644))

	var archive1, archive2, archive3 *Archive

	done := make(chan bool, 3)

	go func() {
		ParseFileToArchivePtr(&archive1, fs, "/test.par2", func(msg string, args ...any) {})
		done <- true
	}()

	go func() {
		ParseFileToArchivePtr(&archive2, fs, "/test.par2", func(msg string, args ...any) {})
		done <- true
	}()

	go func() {
		ParseFileToArchivePtr(&archive3, fs, "/test.par2", func(msg string, args ...any) {})
		done <- true
	}()

	<-done
	<-done
	<-done

	require.NotNil(t, archive1)
	require.NotNil(t, archive2)
	require.NotNil(t, archive3)
	require.Equal(t, archive1.Sets, archive2.Sets, archive3.Sets)
}

// Expectation: ParseFileToArchivePtr should overwrite existing archive pointer.
func Test_ParseFileToArchivePtr_OverwritesExisting_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	validPar2 := loadTestPar2(t)
	require.NoError(t, afero.WriteFile(fs, "/test.par2", validPar2, 0o644))

	oldArchive := &Archive{}
	archive := oldArchive

	ParseFileToArchivePtr(&archive, fs, "/test.par2", func(msg string, args ...any) {})

	require.NotNil(t, archive)
	require.NotEqual(t, oldArchive, archive, "Should have replaced the old archive")
}

// Expectation: ParseFileToArchivePtr should set to nil when parsing fails on previously valid archive.
func Test_ParseFileToArchivePtr_ReplacesWithNilOnError_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/invalid.par2", []byte("invalid"), 0o644))

	archive := &Archive{}

	ParseFileToArchivePtr(&archive, fs, "/invalid.par2", func(msg string, args ...any) {})

	require.Nil(t, archive, "Should have replaced archive with nil on error")
}

// Expectation: ParseFileToArchivePtr should log error details correctly.
func Test_ParseFileToArchivePtr_LogsErrorDetails_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/invalid.par2", []byte("invalid"), 0o644))

	var archive *Archive
	var logMessage string
	var logArgs []any

	ParseFileToArchivePtr(&archive, fs, "/invalid.par2", func(msg string, args ...any) {
		logMessage = msg
		logArgs = args
	})

	require.Equal(t, "Failed to parse PAR2 for par2cron manifest (will retry next run)", logMessage)
	require.Len(t, logArgs, 2)
	require.Equal(t, "error", logArgs[0])

	err, ok := logArgs[1].(error)
	require.True(t, ok)
	require.Error(t, err)
}

// Expectation: ParseFileToArchivePtr should complete synchronously.
func Test_ParseFileToArchivePtr_Synchronous_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	validPar2 := loadTestPar2(t)
	require.NoError(t, afero.WriteFile(fs, "/test.par2", validPar2, 0o644))

	var archive *Archive
	var completed bool

	ParseFileToArchivePtr(&archive, fs, "/test.par2", func(msg string, args ...any) {})
	completed = true

	require.True(t, completed, "ParseFileToArchivePtr should block until completion")
	require.NotNil(t, archive)
}

// Expectation: ParseFileToArchivePtr should handle empty PAR2 file.
func Test_ParseFileToArchivePtr_EmptyFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/empty.par2", []byte{}, 0o644))

	var archive *Archive

	ParseFileToArchivePtr(&archive, fs, "/empty.par2", func(msg string, args ...any) {})

	require.NotNil(t, archive)
	require.Empty(t, archive.Sets)
}

// Expectation: ParseFileToArchivePtr should parse real PAR2 file correctly.
func Test_ParseFileToArchivePtr_RealFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()

	var archive *Archive
	var logMessages []string

	ParseFileToArchivePtr(&archive, fs, "testdata/simple_par2cmdline.par2", func(msg string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf(msg, args...))
	})

	require.NotNil(t, archive)
	require.Empty(t, logMessages)
	require.Len(t, archive.Sets, 1)
	require.Len(t, archive.Sets[0].RecoverySet, 1)
	require.Equal(t, "test.txt", archive.Sets[0].RecoverySet[0].Name)
}

// Expectation: ParseFileToArchivePtr should handle panic with custom message.
func Test_ParseFileToArchivePtr_PanicWithCustomMessage_Success(t *testing.T) {
	t.Parallel()

	customPanicFs := &customPanicFs{msg: "custom panic message"}

	var archive *Archive
	var logMessages []string

	ParseFileToArchivePtr(&archive, customPanicFs, "/test.par2", func(msg string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf("%s %v", msg, args))
	})

	require.Nil(t, archive)
	require.Len(t, logMessages, 1)
	require.Contains(t, logMessages[0], "Panic while parsing PAR2")
	require.Contains(t, logMessages[0], "custom panic message")
}

// panicingFs is a filesystem that panics when Open is called.
type panicingFs struct {
	afero.Fs
}

func (p *panicingFs) Open(name string) (afero.File, error) {
	panic("test panic")
}

// Expectation: sortFilePackets should sort by name first.
func Test_sortFilePackets_SortByName_Success(t *testing.T) {
	t.Parallel()

	packets := []FilePacket{
		{Name: "zebra.txt", FileID: idA},
		{Name: "apple.txt", FileID: idB},
		{Name: "middle.txt", FileID: idC},
	}

	sortFilePackets(packets)

	require.Equal(t, "apple.txt", packets[0].Name)
	require.Equal(t, "middle.txt", packets[1].Name)
	require.Equal(t, "zebra.txt", packets[2].Name)
}

// Expectation: sortFilePackets should sort by FileID when names are equal.
func Test_sortFilePackets_SortByFileID_Success(t *testing.T) {
	t.Parallel()

	packets := []FilePacket{
		{Name: "same.txt", FileID: idC},
		{Name: "same.txt", FileID: idA},
		{Name: "same.txt", FileID: idB},
	}

	sortFilePackets(packets)

	require.Equal(t, Hash(idA), packets[0].FileID)
	require.Equal(t, Hash(idB), packets[1].FileID)
	require.Equal(t, Hash(idC), packets[2].FileID)
}

// Expectation: sortFilePackets should handle empty slice.
func Test_sortFilePackets_EmptySlice_Success(t *testing.T) {
	t.Parallel()

	packets := []FilePacket{}

	sortFilePackets(packets)

	require.Empty(t, packets)
}

// Expectation: sortFilePackets should handle single element.
func Test_sortFilePackets_SingleElement_Success(t *testing.T) {
	t.Parallel()

	packets := []FilePacket{
		{Name: "single.txt", FileID: idA},
	}

	sortFilePackets(packets)

	require.Len(t, packets, 1)
	require.Equal(t, "single.txt", packets[0].Name)
}

// Expectation: sortFilePackets should handle already sorted slice.
func Test_sortFilePackets_AlreadySorted_Success(t *testing.T) {
	t.Parallel()

	packets := []FilePacket{
		{Name: "a.txt", FileID: idA},
		{Name: "b.txt", FileID: idA},
		{Name: "c.txt", FileID: idA},
	}

	sortFilePackets(packets)

	require.Equal(t, "a.txt", packets[0].Name)
	require.Equal(t, "b.txt", packets[1].Name)
	require.Equal(t, "c.txt", packets[2].Name)
}

// Expectation: sortFilePackets should handle reverse sorted slice.
func Test_sortFilePackets_ReverseSorted_Success(t *testing.T) {
	t.Parallel()

	packets := []FilePacket{
		{Name: "z.txt", FileID: idA},
		{Name: "m.txt", FileID: idA},
		{Name: "a.txt", FileID: idA},
	}

	sortFilePackets(packets)

	require.Equal(t, "a.txt", packets[0].Name)
	require.Equal(t, "m.txt", packets[1].Name)
	require.Equal(t, "z.txt", packets[2].Name)
}

// Expectation: sortFilePackets should be stable for equal elements.
func Test_sortFilePackets_StableSort_Success(t *testing.T) {
	t.Parallel()

	packets := []FilePacket{
		{Name: "same.txt", FileID: idA, Size: 100},
		{Name: "same.txt", FileID: idA, Size: 200},
		{Name: "same.txt", FileID: idA, Size: 300},
	}

	sortFilePackets(packets)

	// All should remain in original order since name and FileID are identical
	require.Equal(t, int64(100), packets[0].Size)
	require.Equal(t, int64(200), packets[1].Size)
	require.Equal(t, int64(300), packets[2].Size)
}

// Expectation: sortFilePackets should handle case-sensitive sorting.
func Test_sortFilePackets_CaseSensitive_Success(t *testing.T) {
	t.Parallel()

	packets := []FilePacket{
		{Name: "Zebra.txt", FileID: idA},
		{Name: "apple.txt", FileID: idA},
		{Name: "Apple.txt", FileID: idA},
	}

	sortFilePackets(packets)

	// Uppercase letters come before lowercase in ASCII
	require.Equal(t, "Apple.txt", packets[0].Name)
	require.Equal(t, "Zebra.txt", packets[1].Name)
	require.Equal(t, "apple.txt", packets[2].Name)
}

// Expectation: sortFilePackets should handle special characters.
func Test_sortFilePackets_SpecialCharacters_Success(t *testing.T) {
	t.Parallel()

	packets := []FilePacket{
		{Name: "file_3.txt", FileID: idA},
		{Name: "file-1.txt", FileID: idA},
		{Name: "file.2.txt", FileID: idA},
	}

	sortFilePackets(packets)

	require.Equal(t, "file-1.txt", packets[0].Name)
	require.Equal(t, "file.2.txt", packets[1].Name)
	require.Equal(t, "file_3.txt", packets[2].Name)
}

// Expectation: sortFileIDs should sort hashes in ascending order.
func Test_sortFileIDs_AscendingOrder_Success(t *testing.T) {
	t.Parallel()

	hashes := []Hash{idC, idA, idB}

	sortFileIDs(hashes)

	require.Equal(t, Hash(idA), hashes[0])
	require.Equal(t, Hash(idB), hashes[1])
	require.Equal(t, Hash(idC), hashes[2])
}

// Expectation: sortFileIDs should handle empty slice.
func Test_sortFileIDs_EmptySlice_Success(t *testing.T) {
	t.Parallel()

	hashes := []Hash{}

	sortFileIDs(hashes)

	require.Empty(t, hashes)
}

// Expectation: sortFileIDs should handle single element.
func Test_sortFileIDs_SingleElement_Success(t *testing.T) {
	t.Parallel()

	hashes := []Hash{idA}

	sortFileIDs(hashes)

	require.Len(t, hashes, 1)
	require.Equal(t, Hash(idA), hashes[0])
}

// Expectation: sortFileIDs should handle already sorted slice.
func Test_sortFileIDs_AlreadySorted_Success(t *testing.T) {
	t.Parallel()

	hashes := []Hash{idA, idB, idC}

	sortFileIDs(hashes)

	require.Equal(t, Hash(idA), hashes[0])
	require.Equal(t, Hash(idB), hashes[1])
	require.Equal(t, Hash(idC), hashes[2])
}

// Expectation: sortFileIDs should handle reverse sorted slice.
func Test_sortFileIDs_ReverseSorted_Success(t *testing.T) {
	t.Parallel()

	hashes := []Hash{idC, idB, idA}

	sortFileIDs(hashes)

	require.Equal(t, Hash(idA), hashes[0])
	require.Equal(t, Hash(idB), hashes[1])
	require.Equal(t, Hash(idC), hashes[2])
}

// Expectation: sortFileIDs should handle duplicate hashes.
func Test_sortFileIDs_Duplicates_Success(t *testing.T) {
	t.Parallel()

	hashes := []Hash{idB, idA, idB, idA}

	sortFileIDs(hashes)

	require.Equal(t, Hash(idA), hashes[0])
	require.Equal(t, Hash(idA), hashes[1])
	require.Equal(t, Hash(idB), hashes[2])
	require.Equal(t, Hash(idB), hashes[3])
}

// Expectation: sortFileIDs should handle zero hashes.
func Test_sortFileIDs_ZeroHashes_Success(t *testing.T) {
	t.Parallel()

	zero := Hash{}
	hashes := []Hash{idA, zero, idB}

	sortFileIDs(hashes)

	require.Equal(t, zero, hashes[0])
	require.Equal(t, Hash(idA), hashes[1])
	require.Equal(t, Hash(idB), hashes[2])
}

// Expectation: sortFileIDs should sort by byte comparison.
func Test_sortFileIDs_ByteComparison_Success(t *testing.T) {
	t.Parallel()

	hash1 := Hash{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
	hash2 := Hash{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02}
	hash3 := Hash{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00}

	hashes := []Hash{hash3, hash1, hash2}

	sortFileIDs(hashes)

	require.Equal(t, hash1, hashes[0])
	require.Equal(t, hash2, hashes[1])
	require.Equal(t, hash3, hashes[2])
}

// Expectation: sortFileIDs should handle all bytes set to maximum.
func Test_sortFileIDs_MaxBytes_Success(t *testing.T) {
	t.Parallel()

	maxHash := Hash{}
	for i := range maxHash {
		maxHash[i] = 0xFF
	}

	hashes := []Hash{idA, maxHash, idB}

	sortFileIDs(hashes)

	require.Equal(t, Hash(idA), hashes[0])
	require.Equal(t, Hash(idB), hashes[1])
	require.Equal(t, maxHash, hashes[2])
}

// customPanicFs is a filesystem that panics with a custom message.
type customPanicFs struct {
	afero.Fs

	msg string
}

func (p *customPanicFs) Open(name string) (afero.File, error) {
	panic(p.msg)
}

func loadTestPar2(t *testing.T) []byte {
	t.Helper()

	data, err := os.ReadFile("testdata/simple_par2cmdline.par2")
	require.NoError(t, err)

	return data
}
