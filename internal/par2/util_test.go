package par2

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: MainPacket.Copy should return nil for nil receiver.
func Test_MainPacket_Copy_NilReceiver_Success(t *testing.T) {
	t.Parallel()

	var m *MainPacket
	cpy := m.Copy()

	require.Nil(t, cpy)
}

// Expectation: MainPacket.Copy should create a deep copy.
func Test_MainPacket_Copy_DeepCopy_Success(t *testing.T) {
	t.Parallel()

	original := &MainPacket{
		SetID:          Hash(sID),
		SliceSize:      4096,
		RecoveryIDs:    []Hash{Hash(idA), Hash(idB)},
		NonRecoveryIDs: []Hash{Hash(idC)},
	}

	cpy := original.Copy()

	require.NotNil(t, cpy)
	require.NotSame(t, original, cpy)
	require.Equal(t, original, cpy)

	// Modify original and verify copy is unaffected
	original.RecoveryIDs[0] = Hash(idC)
	require.NotEqual(t, original, cpy)
}

// Expectation: MainPacket.Copy should handle empty slices.
func Test_MainPacket_Copy_EmptySlices_Success(t *testing.T) {
	t.Parallel()

	original := &MainPacket{
		SetID:          Hash(sID),
		SliceSize:      4096,
		RecoveryIDs:    []Hash{},
		NonRecoveryIDs: []Hash{},
	}

	cpy := original.Copy()

	require.NotNil(t, cpy)
	require.NotSame(t, original, cpy)
	require.Equal(t, original, cpy)
}

// Expectation: MainPacket.Copy should handle nil slices.
func Test_MainPacket_Copy_NilSlices_Success(t *testing.T) {
	t.Parallel()

	original := &MainPacket{
		SetID:          Hash(sID),
		SliceSize:      4096,
		RecoveryIDs:    nil,
		NonRecoveryIDs: nil,
	}

	cpy := original.Copy()

	require.NotNil(t, cpy)
	require.NotSame(t, original, cpy)
	require.Equal(t, original, cpy)
}

// Expectation: MainPacket.Equal should return true for both nil.
func Test_MainPacket_Equal_BothNil_Success(t *testing.T) {
	t.Parallel()

	var a, b *MainPacket

	require.True(t, a.Equal(b))
}

// Expectation: MainPacket.Equal should return false when receiver is nil.
func Test_MainPacket_Equal_ReceiverNil_Success(t *testing.T) {
	t.Parallel()

	var a *MainPacket
	b := &MainPacket{SetID: Hash(sID), SliceSize: 4096}

	require.False(t, a.Equal(b))
}

// Expectation: MainPacket.Equal should return false when other is nil.
func Test_MainPacket_Equal_OtherNil_Success(t *testing.T) {
	t.Parallel()

	a := &MainPacket{SetID: Hash(sID), SliceSize: 4096}
	var b *MainPacket

	require.False(t, a.Equal(b))
}

// Expectation: MainPacket.Equal should return true for identical packets.
func Test_MainPacket_Equal_Identical_Success(t *testing.T) {
	t.Parallel()

	a := &MainPacket{
		SetID:          Hash(sID),
		SliceSize:      4096,
		RecoveryIDs:    []Hash{Hash(idA), Hash(idB)},
		NonRecoveryIDs: []Hash{Hash(idC)},
	}
	b := &MainPacket{
		SetID:          Hash(sID),
		SliceSize:      4096,
		RecoveryIDs:    []Hash{Hash(idA), Hash(idB)},
		NonRecoveryIDs: []Hash{Hash(idC)},
	}

	require.NotSame(t, a, b)
	require.True(t, a.Equal(b))
}

// Expectation: MainPacket.Equal should return false for different SetID.
func Test_MainPacket_Equal_DifferentSetID_Success(t *testing.T) {
	t.Parallel()

	a := &MainPacket{SetID: Hash(idA), SliceSize: 4096}
	b := &MainPacket{SetID: Hash(idB), SliceSize: 4096}

	require.False(t, a.Equal(b))
}

// Expectation: MainPacket.Equal should return false for different SliceSize.
func Test_MainPacket_Equal_DifferentSliceSize_Success(t *testing.T) {
	t.Parallel()

	a := &MainPacket{SetID: Hash(sID), SliceSize: 4096}
	b := &MainPacket{SetID: Hash(sID), SliceSize: 8192}

	require.False(t, a.Equal(b))
}

// Expectation: MainPacket.Equal should return false for different RecoveryIDs.
func Test_MainPacket_Equal_DifferentRecoveryIDs_Success(t *testing.T) {
	t.Parallel()

	a := &MainPacket{SetID: Hash(sID), SliceSize: 4096, RecoveryIDs: []Hash{Hash(idA)}}
	b := &MainPacket{SetID: Hash(sID), SliceSize: 4096, RecoveryIDs: []Hash{Hash(idB)}}

	require.False(t, a.Equal(b))
}

// Expectation: MainPacket.Equal should return false for different NonRecoveryIDs.
func Test_MainPacket_Equal_DifferentNonRecoveryIDs_Success(t *testing.T) {
	t.Parallel()

	a := &MainPacket{SetID: Hash(sID), SliceSize: 4096, NonRecoveryIDs: []Hash{Hash(idA)}}
	b := &MainPacket{SetID: Hash(sID), SliceSize: 4096, NonRecoveryIDs: []Hash{Hash(idB)}}

	require.False(t, a.Equal(b))
}

// Expectation: MainPacket.Equal should return false for different RecoveryIDs length.
func Test_MainPacket_Equal_DifferentRecoveryIDsLength_Success(t *testing.T) {
	t.Parallel()

	a := &MainPacket{SetID: Hash(sID), SliceSize: 4096, RecoveryIDs: []Hash{Hash(idA)}}
	b := &MainPacket{SetID: Hash(sID), SliceSize: 4096, RecoveryIDs: []Hash{Hash(idA), Hash(idB)}}

	require.False(t, a.Equal(b))
}

// Expectation: MainPacket.Equal should handle empty vs nil slices.
func Test_MainPacket_Equal_EmptyVsNilSlices_Success(t *testing.T) {
	t.Parallel()

	a := &MainPacket{SetID: Hash(sID), SliceSize: 4096, RecoveryIDs: []Hash{}}
	b := &MainPacket{SetID: Hash(sID), SliceSize: 4096, RecoveryIDs: nil}

	// slices.Equal treats nil and empty as equal
	require.True(t, a.Equal(b))
}

// Expectation: ParserPanicError.Error should return formatted message.
func Test_ParserPanicError_Error_Success(t *testing.T) {
	t.Parallel()

	err := &ParserPanicError{
		Value: "test panic",
		Stack: []byte("stack trace"),
	}

	require.Equal(t, "parser panic: test panic", err.Error())
}

// Expectation: ParserPanicError.Error should handle different panic types.
func Test_ParserPanicError_Error_DifferentTypes_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    any
		expected string
	}{
		{"string", "panic message", "parser panic: panic message"},
		{"int", 42, "parser panic: 42"},
		{"error", errors.New("an error"), "parser panic: an error"},
		{"nil", nil, "parser panic: <nil>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := &ParserPanicError{Value: tt.value}
			require.Equal(t, tt.expected, err.Error())
		})
	}
}

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

	f, err := ParseFile(fs, "/test.par2", false)

	require.NoError(t, err)
	require.NotNil(t, f)
	require.Len(t, f.Sets, 1)
}

// Expectation: ParseFile should fail when file doesn't exist.
func Test_ParseFile_FileNotFound_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	_, err := ParseFile(fs, "/nonexistent.par2", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to open PAR2 file")
}

// Expectation: ParseFile should produce an empty [File] (no readable packets).
func Test_ParseFile_InvalidPAR2_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/invalid.par2", []byte("not a par2 file"), 0o644))

	f, err := ParseFile(fs, "/invalid.par2", false)
	require.NoError(t, err)
	require.Empty(t, f.Sets)
}

// Expectation: ParseFile should parse real PAR2 files correctly.
func Test_ParseFile_RealFile_Success(t *testing.T) {
	t.Parallel()

	for _, tt := range realSeeds {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			fs := afero.NewOsFs()

			f, err := ParseFile(fs, tt.file, false)
			require.NoError(t, err)
			require.NotNil(t, f)
			require.Len(t, f.Sets, 1)
			require.Len(t, f.Sets[0].RecoverySet, len(tt.expected))
		})
	}
}

// Expectation: ParseFile should handle empty PAR2 file.
func Test_ParseFile_EmptyFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/empty.par2", []byte{}, 0o644))

	f, err := ParseFile(fs, "/empty.par2", false)
	require.NoError(t, err)
	require.NotNil(t, f)
	require.Empty(t, f.Sets)
}

// Expectation: ParseFile should handle multiple sets in one file.
func Test_ParseFile_MultipleSets_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	set1 := buildMainPacket(4096, [][16]byte{idA}, nil, idA)
	set2 := buildMainPacket(4096, [][16]byte{idB}, nil, idB)
	combined := slices.Concat(set1, set2)

	require.NoError(t, afero.WriteFile(fs, "/multi.par2", combined, 0o644))

	f, err := ParseFile(fs, "/multi.par2", false)
	require.NoError(t, err)

	require.NotNil(t, f)
	require.Len(t, f.Sets, 2)
}

// Expectation: ParseFile should set correct filename from path.
func Test_ParseFile_CorrectFilename_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	packet := buildMainPacket(4096, [][16]byte{idA}, nil, sID)
	require.NoError(t, afero.WriteFile(fs, "/some/deep/path/myfile.par2", packet, 0o644))

	f, err := ParseFile(fs, "/some/deep/path/myfile.par2", false)
	require.NoError(t, err)

	require.Equal(t, "myfile.par2", f.Name)
}

// Expectation: ParseFileSet should parse index and volume files.
func Test_ParseFileSet_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	mainPacket := buildMainPacket(4096, [][16]byte{idA, idB}, nil, sID)
	fileA := buildFileDescPacket("a.txt", 100, idA, sID)
	fileB := buildFileDescPacket("b.txt", 200, idB, sID)

	indexData := slices.Concat(mainPacket, fileA)
	require.NoError(t, afero.WriteFile(fs, "/archive.par2", indexData, 0o644))

	volData := slices.Concat(mainPacket, fileB)
	require.NoError(t, afero.WriteFile(fs, "/archive.vol00+01.par2", volData, 0o644))

	result, err := ParseFileSet(fs, "/archive.par2", false)
	require.NoError(t, err)

	require.NotNil(t, result)
	require.Len(t, result.Files, 2)
	require.Len(t, result.SetsMerged, 1)
	require.Len(t, result.SetsMerged[0].RecoverySet, 2)
}

// Expectation: ParseFileSet should fail when no files can be parsed.
func Test_ParseFileSet_NoParseableFiles_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	_, err := ParseFileSet(fs, "/notexist.par2", false)
	require.Error(t, err)
	require.ErrorIs(t, err, errFileCorrupted)
}

// Expectation: ParseFileSet should return empty unparseable files.
func Test_ParseFileSet_SomeUnparseableFiles_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	mainPacket := buildMainPacket(4096, [][16]byte{idA}, nil, sID)
	fileA := buildFileDescPacket("a.txt", 100, idA, sID)

	indexData := slices.Concat(mainPacket, fileA)
	require.NoError(t, afero.WriteFile(fs, "/archive.par2", indexData, 0o644))

	require.NoError(t, afero.WriteFile(fs, "/archive.vol00+01.par2", []byte("garbage"), 0o644))

	result, err := ParseFileSet(fs, "/archive.par2", false)
	require.NoError(t, err)

	require.NotNil(t, result)
	require.Len(t, result.Files, 2)
	require.Empty(t, result.Files[1].Sets)
}

// Expectation: ParseFileSet should handle only volume files existing.
func Test_ParseFileSet_NoIndexButVolumeFiles_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	mainPacket := buildMainPacket(4096, [][16]byte{idA}, nil, sID)
	fileA := buildFileDescPacket("a.txt", 100, idA, sID)
	volData := slices.Concat(mainPacket, fileA)

	require.NoError(t, afero.WriteFile(fs, "/archive.vol00+01.par2", volData, 0o644))

	result, err := ParseFileSet(fs, "/archive.par2", false)
	require.NoError(t, err)

	require.NotNil(t, result)
	require.Len(t, result.Files, 1)
	require.NotEmpty(t, result.SetsMerged)
}

// Expectation: ParseFileSet should not include index file twice.
func Test_ParseFileSet_NoDuplicateIndex_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	mainPacket := buildMainPacket(4096, [][16]byte{idA}, nil, sID)
	fileA := buildFileDescPacket("a.txt", 100, idA, sID)
	indexData := slices.Concat(mainPacket, fileA)

	require.NoError(t, afero.WriteFile(fs, "/archive.par2", indexData, 0o644))

	result, err := ParseFileSet(fs, "/archive.par2", false)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Files, 1)
}

// Expectation: ParseFileSet should merge multiple volume files.
func Test_ParseFileSet_MergesMultipleVolumes_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	mainPacket := buildMainPacket(4096, [][16]byte{idA, idB, idC}, nil, sID)
	fileA := buildFileDescPacket("a.txt", 100, idA, sID)
	fileB := buildFileDescPacket("b.txt", 200, idB, sID)
	fileC := buildFileDescPacket("c.txt", 300, idC, sID)

	indexData := slices.Concat(mainPacket, fileA)
	require.NoError(t, afero.WriteFile(fs, "/archive.par2", indexData, 0o644))

	vol1Data := slices.Concat(mainPacket, fileB)
	require.NoError(t, afero.WriteFile(fs, "/archive.vol00+01.par2", vol1Data, 0o644))

	vol2Data := slices.Concat(mainPacket, fileC)
	require.NoError(t, afero.WriteFile(fs, "/archive.vol01+02.par2", vol2Data, 0o644))

	result, err := ParseFileSet(fs, "/archive.par2", false)
	require.NoError(t, err)

	require.NotNil(t, result)
	require.Len(t, result.Files, 3)
	require.Len(t, result.SetsMerged, 1)
	require.Len(t, result.SetsMerged[0].RecoverySet, 3)
}

// Expectation: ParseFileSet should handle conflicting main packets.
func Test_ParseFileSet_ConflictingMainPackets_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	mainPacket1 := buildMainPacket(4096, [][16]byte{idA}, nil, sID)
	mainPacket2 := buildMainPacket(8192, [][16]byte{idA}, nil, sID)

	require.NoError(t, afero.WriteFile(fs, "/archive.par2", mainPacket1, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/archive.vol00+01.par2", mainPacket2, 0o644))

	_, err := ParseFileSet(fs, "/archive.par2", false)
	require.ErrorIs(t, err, errUnresolvableConflict)
}

// Expectation: ParseFileSet should handle empty index file with valid volumes.
func Test_ParseFileSet_EmptyIndexValidVolumes_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/archive.par2", []byte{}, 0o644))

	mainPacket := buildMainPacket(4096, [][16]byte{idA}, nil, sID)
	fileA := buildFileDescPacket("a.txt", 100, idA, sID)
	volData := slices.Concat(mainPacket, fileA)
	require.NoError(t, afero.WriteFile(fs, "/archive.vol00+01.par2", volData, 0o644))

	result, err := ParseFileSet(fs, "/archive.par2", false)
	require.NoError(t, err)

	require.NotNil(t, result)
	require.Len(t, result.Files, 2)
	require.NotEmpty(t, result.SetsMerged)
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
